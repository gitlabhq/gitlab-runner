package helpers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob" // Needed to register the Azure driver
	_ "gocloud.dev/blob/s3blob"    // Needed to register the AWS S3 driver
	"mvdan.cc/sh/v3/shell"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/meter"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
	"gitlab.com/gitlab-org/gitlab-runner/log"
)

type CacheArchiverCommand struct {
	fileArchiver
	retryHelper
	meter.TransferMeterCommand

	File                   string   `long:"file" description:"The path to file"`
	AlternateFile          string   `long:"alternate-file" description:"(temporary) Alternate local cache file path (e.g. unhashed name) to rename to --file if --file does not exist"`
	URL                    string   `long:"url" description:"URL of remote cache resource (pre-signed URL)"`
	CheckURL               string   `long:"check-url" description:"(temporary) Pre-signed HEAD URL to check whether the primary cache object already exists"`
	GoCloudURL             string   `long:"gocloud-url" description:"Go Cloud URL of remote cache resource (requires credentials)"`
	Timeout                int      `long:"timeout" description:"Overall timeout for cache uploading request (in minutes)"`
	Headers                []string `long:"header" description:"HTTP headers to send with PUT request (in form of 'key:value')"`
	Metadata               metadata `long:"metadata" env:"CACHE_METADATA" description:"Metadata for the cache artifact (JSON encoded key-value-pairs, e.g. '{\"foo\":\"bar\",\"blerp\":\"blip\"}')"`
	CompressionLevel       string   `long:"compression-level" env:"CACHE_COMPRESSION_LEVEL" description:"Compression level (fastest, fast, default, slow, slowest)"`
	CompressionFormat      string   `long:"compression-format" env:"CACHE_COMPRESSION_FORMAT" description:"Compression format (zip, tarzstd)"`
	MaxUploadedArchiveSize int64    `long:"max-uploaded-archive-size" env:"CACHE_MAX_UPLOADED_ARCHIVE_SIZE" description:"Limit the size of the cache archive being uploaded to cloud storage, in bytes."`
	EnvFile                string   `long:"env-file" description:"Filename containing environment variables to read"`

	// Transfer options (all backends: presigned S3, GoCloud S3/Azure/GCS).
	TransferBufferSize int `long:"transfer-buffer-size" env:"CACHE_TRANSFER_BUFFER_SIZE" description:"Buffer size in bytes for streaming cache upload/download (default 4 MiB)"`
	ChunkSize          int `long:"chunk-size" env:"CACHE_CHUNK_SIZE" description:"Part/chunk size in bytes for GoCloud upload when FF_USE_PARALLEL_CACHE_TRANSFER is enabled (default 16 MiB)"`
	Concurrency        int `long:"concurrency" env:"CACHE_CONCURRENCY" description:"Concurrent parts for GoCloud multipart upload when FF_USE_PARALLEL_CACHE_TRANSFER is enabled (default 16; otherwise 1)"`

	client *CacheClient
	mux    *blob.URLMux
}

func NewCacheArchiverCommand() cli.Command {
	return common.NewCommand(
		"cache-archiver",
		"create and upload cache artifacts (internal)",
		&CacheArchiverCommand{
			retryHelper: retryHelper{
				Retry:     2,
				RetryTime: time.Second,
			},
			TransferBufferSize: defaultCacheTransferBufferSize,
			ChunkSize:          defaultCacheChunkSize,
			Concurrency:        defaultCacheConcurrency,
		},
	)
}

type metadata map[string]string

func (m *metadata) UnmarshalFlag(raw string) error {
	return json.Unmarshal([]byte(raw), m)
}

func (c *CacheArchiverCommand) getClient() *CacheClient {
	if c.client == nil {
		c.client = NewCacheClient(c.Timeout)
	}

	return c.client
}

func (c *CacheArchiverCommand) upload(_ int) error {
	file, err := os.Open(c.File)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	fi, err := file.Stat()
	if err != nil {
		return err
	}

	rc := meter.NewReader(
		file,
		c.TransferMeterFrequency,
		meter.LabelledRateFormat(os.Stdout, "Uploading cache", fi.Size()),
	)
	defer rc.Close()

	if c.GoCloudURL != "" {
		logrus.Infoln("Using GoCloud URL for cache upload")
		return c.handleGoCloudURL(rc)
	}
	logrus.Infoln("Using presigned URL for cache upload")
	return c.handlePresignedURL(fi, rc)
}

func (c *CacheArchiverCommand) handlePresignedURL(fi os.FileInfo, file io.ReadCloser) error {
	logrus.Infoln("Uploading", filepath.Base(c.File), "to", url_helpers.CleanURL(c.URL))

	// Use a buffered body so the HTTP client reads in larger chunks (improves S3 upload throughput).
	body := struct {
		io.Reader
		io.Closer
	}{bufio.NewReaderSize(file, c.TransferBufferSize), file}
	req, err := http.NewRequest(http.MethodPut, c.URL, body)
	if err != nil {
		return retryableErr{err: err}
	}

	c.setHeaders(req, fi)
	req.ContentLength = fi.Size()

	resp, err := c.getClient().Do(req)
	if err != nil {
		return retryableErr{err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	return retryOnServerError(resp)
}

func (c *CacheArchiverCommand) handleGoCloudURL(file io.Reader) error {
	logrus.Infoln("Uploading", filepath.Base(c.File), "to", url_helpers.CleanURL(c.GoCloudURL))

	if c.mux == nil {
		c.mux = blob.DefaultURLMux()
	}

	ctx, cancelWrite := context.WithCancel(context.Background())
	defer cancelWrite()

	u, err := url.Parse(c.GoCloudURL)
	if err != nil {
		return err
	}

	err = loadEnvFile(c.EnvFile)
	if err != nil {
		return err
	}

	objectName := strings.TrimLeft(u.Path, "/")
	if objectName == "" {
		return fmt.Errorf("no object name provided")
	}

	b, err := c.mux.OpenBucket(ctx, c.GoCloudURL)
	if err != nil {
		return err
	}
	defer b.Close()

	writerOpts := &blob.WriterOptions{
		Metadata:       c.Metadata,
		BufferSize:     c.ChunkSize,
		MaxConcurrency: c.Concurrency,
	}
	ffLogger := logrus.WithField("name", featureflags.UseParallelCacheTransfer)
	if !featureflags.IsOn(ffLogger, os.Getenv(featureflags.UseParallelCacheTransfer)) {
		writerOpts.MaxConcurrency = 1
	}

	writer, err := b.NewWriter(ctx, objectName, writerOpts)
	if err != nil {
		return err
	}

	buf := make([]byte, c.TransferBufferSize)
	if _, err = io.CopyBuffer(writer, file, buf); err != nil {
		cancelWrite()
		if writerErr := writer.Close(); writerErr != nil {
			logrus.WithError(writerErr).Error("error closing Go cloud upload after copy failure")
		}
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	return nil
}

func (c *CacheArchiverCommand) createZipFile(filename string) (int64, error) {
	err := os.MkdirAll(filepath.Dir(filename), 0o700)
	if err != nil {
		return 0, err
	}

	f, err := os.CreateTemp(filepath.Dir(filename), "archive_")
	if err != nil {
		return 0, err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	logrus.Debugln("Temporary file:", f.Name())

	switch strings.ToLower(c.CompressionFormat) {
	case string(spec.ArtifactFormatTarZstd):
		c.CompressionFormat = string(spec.ArtifactFormatTarZstd)
	default:
		c.CompressionFormat = string(spec.ArtifactFormatZip)
	}

	archiver, err := archive.NewArchiver(archive.Format(c.CompressionFormat), f, c.wd, GetCompressionLevel(c.CompressionLevel))
	if err != nil {
		return 0, err
	}

	// Create archive
	err = archiver.Archive(context.Background(), c.files)
	if err != nil {
		return 0, err
	}

	info, err := f.Stat()
	if err != nil {
		return 0, err
	}

	err = f.Close()
	if err != nil {
		return 0, err
	}

	return info.Size(), os.Rename(f.Name(), filename)
}

func (c *CacheArchiverCommand) tryRenameAlternateFile() {
	if c.AlternateFile == "" || c.AlternateFile == c.File {
		return
	}

	_, err := os.Stat(c.File)
	if err == nil {
		logrus.Debugln("Primary cache file already exists locally, skipping rename from alternate")
		return
	}
	if !errors.Is(err, fs.ErrNotExist) {
		logrus.WithError(err).Warningln("Failed to stat primary cache file")
		return
	}

	if _, err := os.Stat(c.AlternateFile); err != nil {
		logrus.Debugln("Alternate cache file not found locally, nothing to rename")
		return
	}

	if err := os.MkdirAll(filepath.Dir(c.File), 0o700); err != nil {
		logrus.WithError(err).Warningln("Failed to create directory for cache file rename")
		return
	}

	if err := os.Rename(c.AlternateFile, c.File); err != nil {
		logrus.WithError(err).Warningln("Failed to rename alternate cache file to primary")
		return
	}

	logrus.Infoln("Renamed alternate cache file to primary")
}

func (c *CacheArchiverCommand) Execute(*cli.Context) {
	log.SetRunnerFormatter()

	c.normalizeArgs()
	c.tryRenameAlternateFile()
	if err := validateCacheTransferTuning(c.TransferBufferSize, c.ChunkSize, c.Concurrency); err != nil {
		logrus.Fatalln(err)
	}

	// Enumerate files
	err := c.enumerate()
	if err != nil {
		logrus.Fatalln(err)
	}

	// Skip upload if no files were found
	if len(c.files) == 0 {
		logrus.Warningln("No files to cache.")
		return
	}

	// Check if list of files changed
	if !c.isFileChanged(c.File) {
		if c.AlternateFile != c.File {
			// AlternateFile is set (FF_HASH_CACHE_KEYS compatibility mode): the primary
			// archive may have been downloaded from the alternate URL by the extractor,
			// meaning the primary remote URL does not yet have an object. Upload the
			// existing archive to ensure the primary URL is populated.
			// This handles both transition directions:
			//   FF false→true: primary=hashed, alternate=unhashed
			//   FF true→false: primary=unhashed, alternate=hashed
			c.uploadExistingArchiveIfNeeded()
			return
		}
		logrus.Infoln("Archive is up to date!")
		return
	}

	// Create archive
	size, err := c.createZipFile(c.File)
	if err != nil {
		logrus.Fatalln(err)
	}

	err = writeCacheMetadataFile(c.File, c.Metadata)
	if err != nil {
		logrus.Fatalln(err)
	}

	c.uploadArchiveIfNeeded(size)
}

func (c *CacheArchiverCommand) normalizeArgs() {
	if c.File == "" {
		logrus.Fatalln("Missing --file")
	}

	if c.TransferBufferSize == 0 {
		c.TransferBufferSize = defaultCacheTransferBufferSize
	}
	if c.ChunkSize == 0 {
		c.ChunkSize = defaultCacheChunkSize
	}
	if c.Concurrency == 0 {
		c.Concurrency = defaultCacheConcurrency
	}

	for idx := range c.Paths {
		if path, err := shell.Expand(c.Paths[idx], nil); err != nil {
			logrus.Warnf("invalid path %q: %v", path, err)
		} else {
			c.Paths[idx] = path
		}
	}

	for idx := range c.Exclude {
		if path, err := shell.Expand(c.Exclude[idx], nil); err != nil {
			logrus.Warnf("invalid path %q: %v", path, err)
		} else {
			c.Exclude[idx] = path
		}
	}
}

// uploadExistingArchiveIfNeeded uploads the local cache archive to the primary remote URL
// if the archive exists locally and the primary remote does not yet have an object.
func (c *CacheArchiverCommand) uploadExistingArchiveIfNeeded() {
	fi, err := os.Stat(c.File)
	if err != nil {
		return
	}
	if c.primaryRemoteExists() {
		logrus.Infoln("Primary cache already exists remotely, skipping upload")
	} else {
		logrus.Infoln("Primary cache does not exist remotely, uploading existing archive")
		c.uploadArchiveIfNeeded(fi.Size())
	}
}

// primaryRemoteExists reports whether the primary remote cache object already exists.
// Returns true only when the object is confirmed present; returns false on any error or absence.
func (c *CacheArchiverCommand) primaryRemoteExists() bool {
	if c.GoCloudURL != "" {
		return c.primaryGoCloudExists()
	}
	if c.CheckURL != "" {
		return c.primaryPresignedExists()
	}
	return false
}

func (c *CacheArchiverCommand) primaryPresignedExists() bool {
	resp, err := c.getClient().Head(c.CheckURL)
	if err != nil {
		logrus.WithError(err).Warningln("Failed to check primary cache existence via HEAD request, assuming absent")
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	exists := resp.StatusCode == http.StatusOK
	logrus.WithField("status", resp.StatusCode).Debugln("Primary cache HEAD request completed")
	return exists
}

func (c *CacheArchiverCommand) primaryGoCloudExists() bool {
	if c.mux == nil {
		c.mux = blob.DefaultURLMux()
	}

	ctx := context.Background()

	if err := loadEnvFile(c.EnvFile); err != nil {
		return false
	}

	u, err := url.Parse(c.GoCloudURL)
	if err != nil {
		return false
	}

	objectName := strings.TrimLeft(u.Path, "/")
	if objectName == "" {
		return false
	}

	b, err := c.mux.OpenBucket(ctx, c.GoCloudURL)
	if err != nil {
		return false
	}
	defer b.Close()

	_, err = b.Attributes(ctx, objectName)
	if err != nil {
		logrus.WithField("object", objectName).Debugln("Primary cache object not found in remote storage")
		return false
	}
	logrus.WithField("object", objectName).Debugln("Primary cache object found in remote storage")
	return true
}

func (c *CacheArchiverCommand) uploadArchiveIfNeeded(size int64) {
	if c.URL == "" && c.GoCloudURL == "" {
		logrus.Infoln(
			"No URL provided, cache will not be uploaded to shared cache server. " +
				"Cache will be stored only locally.")
		return
	}

	if c.MaxUploadedArchiveSize != 0 && size > c.MaxUploadedArchiveSize {
		logrus.Infoln(fmt.Sprintf("Cache archive size (%d) is too big (Limit is set to %d). "+
			"Cache will be stored only locally.", size, c.MaxUploadedArchiveSize))
		return
	}

	err := c.doRetry(c.upload)
	if err != nil {
		logrus.Fatalln(err)
	}
}

func (c *CacheArchiverCommand) setHeaders(req *http.Request, fi os.FileInfo) {
	for k, v := range split(c.Headers) {
		req.Header.Set(strings.TrimSpace(k), strings.TrimSpace(v))
	}

	// Set default headers. But don't override custom Content-Type.
	if req.Header.Get(common.ContentType) == "" {
		req.Header.Set(common.ContentType, "application/octet-stream")
	}
	req.Header.Set("Last-Modified", fi.ModTime().UTC().Format(http.TimeFormat))
}

func split(raw []string) map[string]string {
	const sep = ":"

	data := make(map[string]string, len(raw))

	for _, s := range raw {
		k, v, ok := strings.Cut(s, sep)
		if !ok {
			continue
		}
		data[k] = v
	}

	return data
}
