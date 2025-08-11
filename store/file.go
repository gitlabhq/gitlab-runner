package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var errEmptyStorePath = fmt.Errorf("file store path is required")

type fileStoreProvider struct {
	sync.Mutex

	m map[string]*jobFileStore
}

var fileProviderInstance = &fileStoreProvider{}

func FileProvider() common.JobStoreProvider {
	return fileProviderInstance
}

func (f *fileStoreProvider) Name() string {
	return "file"
}

// Get returns a JobStore instance for the given RunnerConfig. One store instance is created per store path.
// This way, the same store instance can be shared between multiple builds.
// The store provider allows for variable expansion in the path.
// Since the expansion happens very early on in the lifecycle of the Runner only a handful of variables are available.
// CI_* and any environment variable that is set in the `env` section of the config.toml
func (f *fileStoreProvider) Get(config *common.RunnerConfig) (common.JobStore, error) {
	f.Lock()
	defer f.Unlock()

	if f.m == nil {
		f.m = make(map[string]*jobFileStore)
	}

	if config.Store.File == nil || config.Store.File.Path == nil || *config.Store.File.Path == "" {
		return nil, errEmptyStorePath
	}

	storePath := config.GetVariables().ExpandValue(*config.Store.File.Path)
	if store, ok := f.m[storePath]; ok {
		return store, nil
	}

	if err := os.MkdirAll(storePath, 0700); err != nil {
		return nil, err
	}

	f.m[storePath] = newJobFileStore(storePath, config.Store, config.Log())
	return f.m[storePath], nil
}

// jobFileStore is a file-based implementation of the JobStore interface.
// It's a simple store that saves jobs to disk in a directory. As such it has
// the limitation that it does not support more than one Runner Manager instance using the same store path
// this means that deployments that use multiple runners on the same machine should use a different path for each runner.
// This store is synchronized with a mutex to prevent concurrent access to the file system.
// A single instance per store path should be used.
type jobFileStore struct {
	codec     jobCodec
	storePath string

	// mu is used to protect the file store from concurrent file system access
	mu sync.Mutex

	newJobScanner func() (jobScanner, error)

	canResumeFilter jobFilter
	canDeleteFilter jobFilter

	logger logrus.FieldLogger
}

// newJobFileStore creates a new jobFileStore instance.
// The store config must have a non-empty path.
// The StoreConfig variable is guaranteed to be non-nil.
func newJobFileStore(storePath string, storeConfig *common.StoreConfig, logger logrus.FieldLogger) *jobFileStore {
	// if we decide to introduce more codecs
	// we could introduce a setting in the config
	// it could also be made a parameter to store constructors
	codec := gobJobCodec{}
	return &jobFileStore{
		codec:     codec,
		storePath: storePath,

		newJobScanner: func() (jobScanner, error) {
			return newFileStoreJobScanner(storePath, codec)
		},

		canResumeFilter: newCanResumeJobFilter(storeConfig),
		canDeleteFilter: newCanDeleteJobFilter(storeConfig),

		logger: logger.WithField("store", FileProvider().Name()),
	}
}

// Request returns the next job that can be resumed.
func (f *jobFileStore) Request() (*common.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.logger.Debugf("[FT-DEBUG] Request: Scanning store path: %s", f.storePath)
	
	scanner, err := f.newJobScanner()
	if err != nil {
		f.logger.WithError(err).Errorln("[FT-DEBUG] Request: Failed to create job scanner")
		return nil, err
	}

	jobCount := 0
	deletedCount := 0
	for scanner.Scan() {
		jobCount++
		job, err := scanner.Job()
		if err != nil {
			// We can't decode the file, so we skip it. Likely corrupted.
			f.logger.WithError(err).Warningln("[FT-DEBUG] Request: Error decoding job file")
			continue
		}

		f.logger.WithFields(logrus.Fields{
			"job_id": job.ID,
			"job_token": job.Token[:8] + "...",
			"health_check": job.State.GetHealthCheckAt(),
			"started_at": job.State.GetStartedAt(),
			"stage": job.State.GetStage(),
			"build_state": job.State.GetBuildState(),
		}).Debugln("[FT-DEBUG] Request: Found job in store")

		if f.canDeleteFilter(job) {
			f.logger.WithField("job_id", job.ID).Infoln("[FT-DEBUG] Request: Job marked for deletion (expired/unhealthy)")
			if err := f.removeJob(job); err != nil {
				f.logger.WithError(err).Errorln("[FT-DEBUG] Request: Failed to remove job")
				return nil, err
			}
			deletedCount++
		} else if f.canResumeFilter(job) {
			f.logger.WithField("job_id", job.ID).Infoln("[FT-DEBUG] Request: Found resumable job")
			return job, nil
		} else {
			f.logger.WithField("job_id", job.ID).Debugln("[FT-DEBUG] Request: Job not resumable yet")
		}
	}

	f.logger.Debugf("[FT-DEBUG] Request: Scanned %d jobs, deleted %d, none resumable", jobCount, deletedCount)
	return nil, nil
}

// List returns all jobs in the store.
func (f *jobFileStore) List() ([]*common.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	scanner, err := f.newJobScanner()
	if err != nil {
		return nil, err
	}

	var jobs []*common.Job
	for scanner.Scan() {
		job, err := scanner.Job()
		if err != nil {
			// We can't decode the file, so we skip it. Likely corrupted.
			f.logger.WithError(err).Warningln("Error decoding job file on List")
			continue
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// Update saves or updates a job in the store.
// The Update method uses a common method of writing the data to a temporary file and then renaming it to the final file.
// This works on POSIX compliant file systems and is atomic but is not guaranteed to work on all file systems, especially
// networked file systems. The method is used to prevent data corruption in case of a crash.
// This is why the file store combines this technique with reading from both the actual and temporary file in the scanner.
func (f *jobFileStore) Update(job *common.Job) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.logger.WithFields(logrus.Fields{
		"job_id": job.ID,
		"file_path": f.filePath(job),
		"health_check": job.State.GetHealthCheckAt(),
		"stage": job.State.GetStage(),
		"build_state": job.State.GetBuildState(),
		"sent_trace": job.State.GetSentTrace(),
		"is_resumed": job.State.IsResumed(),
	}).Debugln("[FT-DEBUG] Update: Updating job in store")

	tmpPath := f.filePathTmp(job)
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		f.logger.WithError(err).Errorln("[FT-DEBUG] Update: Failed to open temp file")
		return err
	}
	defer file.Close()

	if err := f.codec.Encode(file, job); err != nil {
		f.logger.WithError(err).Errorln("[FT-DEBUG] Update: Failed to encode job")
		return err
	}

	if err := file.Sync(); err != nil {
		f.logger.WithError(err).Errorln("[FT-DEBUG] Update: Failed to sync file")
		return err
	}

	if err := file.Close(); err != nil {
		f.logger.WithError(err).Errorln("[FT-DEBUG] Update: Failed to close temp file")
		return err
	}

	finalPath := f.filePath(job)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		f.logger.WithError(err).Errorln("[FT-DEBUG] Update: Failed to rename temp file to final")
		return err
	}

	f.logger.Debugln("[FT-DEBUG] Update: Job updated successfully in store")
	return nil
}

// Remove removes a job from the store.
func (f *jobFileStore) Remove(job *common.Job) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.logger.WithField("job_id", job.ID).Infoln("[FT-DEBUG] Remove: Removing job from store")
	return f.removeJob(job)
}

func (f *jobFileStore) removeJob(job *common.Job) error {
	var errs []error

	files := []string{f.filePath(job), f.filePathTmp(job)}
	f.logger.WithFields(logrus.Fields{
		"job_id": job.ID,
		"files": files,
	}).Debugln("[FT-DEBUG] removeJob: Attempting to remove job files")

	for _, file := range files {
		err := os.Remove(file)
		if err == nil {
			f.logger.Debugf("[FT-DEBUG] removeJob: Successfully removed file: %s", file)
			continue
		}
		if os.IsNotExist(err) {
			f.logger.Debugf("[FT-DEBUG] removeJob: File already removed: %s", file)
			continue
		}

		f.logger.WithError(err).Errorf("[FT-DEBUG] removeJob: Failed to remove file: %s", file)
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		f.logger.WithField("errors", errs).Errorln("[FT-DEBUG] removeJob: Failed to remove all job files")
	} else {
		f.logger.Debugln("[FT-DEBUG] removeJob: Job files removed successfully")
	}

	return errors.Join(errs...)
}

func (f *jobFileStore) filePath(job *common.Job) string {
	return filepath.Join(f.storePath, fmt.Sprintf("%v.state", job.ID))
}

func (f *jobFileStore) filePathTmp(job *common.Job) string {
	return f.filePath(job) + ".tmp"
}

type jobScanner interface {
	Scan() bool
	Job() (*common.Job, error)
}

// fileStoreJobScanner is a jobScanner implementation that scans files in a directory.
// It reads the files in the directory in order and decodes them into jobs.
// The files are sorted alphabetically. This way the regular file is read before the temp file and jobs are always processed in order.
// If the first one fails we will fall back to the temp file. Users of the scanner can decide what to do in case of an error.
type fileStoreJobScanner struct {
	decoder jobDecoder
	files   []string

	idx int
}

func newFileStoreJobScanner(storePath string, decoder jobDecoder) (*fileStoreJobScanner, error) {
	logrus.Debugf("[FT-DEBUG] newFileStoreJobScanner: Reading store path: %s", storePath)
	
	dirEntries, err := os.ReadDir(storePath)
	if err != nil {
		logrus.WithError(err).Errorf("[FT-DEBUG] newFileStoreJobScanner: Failed to read store path: %s", storePath)
		return nil, fmt.Errorf("job scanner reading store path: %w", err)
	}

	var files []string
	for _, dirEntry := range dirEntries {
		files = append(files, filepath.Join(storePath, dirEntry.Name()))
	}

	sort.Strings(files)
	
	logrus.Debugf("[FT-DEBUG] newFileStoreJobScanner: Found %d files in store: %v", len(files), files)

	return &fileStoreJobScanner{
		decoder: decoder,
		files:   files,
		idx:     -1,
	}, nil
}

func (f *fileStoreJobScanner) Scan() bool {
	f.idx++

	return f.idx < len(f.files)
}

func (f *fileStoreJobScanner) Job() (*common.Job, error) {
	file, err := os.OpenFile(f.files[f.idx], os.O_RDONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return f.decoder.Decode(file)
}
