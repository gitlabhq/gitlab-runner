package common

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

type JobManager interface {
	UpdateJob(jobCredentials *JobCredentials, jobInfo UpdateJobInfo) UpdateJobResult
	PatchTrace(jobCredentials *JobCredentials, content []byte, startOffset int, debugModeEnabled bool) PatchTraceResult
	RequestJob(ctx context.Context, sessionInfo *SessionInfo) (*Job, bool)
	ProcessJob(buildCredentials *JobCredentials) (JobTrace, error)
}

// JobTraceProviderFunc is required to facilitate creating a new instance of JobTrace.
// We need the interface to avoid circular dependencies and to allow for mocking.
type JobTraceProviderFunc func(jobManager JobManager, config RunnerConfig, jobCredentials *JobCredentials, startOffset int64) (JobTrace, error)

type StatefulJobManager struct {
	nw            Network
	store         JobStore
	traceProvider JobTraceProviderFunc
	config        *RunnerConfig
	logger        logrus.FieldLogger

	job *Job
}

func NewStatefulJobManager(nw Network, store JobStore, traceProvider JobTraceProviderFunc, config *RunnerConfig) *StatefulJobManager {
	return &StatefulJobManager{
		nw:            nw,
		store:         store,
		traceProvider: traceProvider,
		config:        config,
		logger:        config.Log(),
	}
}

// RequestJob requests a job from the store or the network. This method call associates the manager with a job.
// It must be called first. Calling other methods before RequestJob will result in a panic.
// The second return value will always be true if a job was restored from storage. Otherwise, if a job was
// requested from the network, the second return value will be passed through from the network.
func (s *StatefulJobManager) RequestJob(ctx context.Context, sessionInfo *SessionInfo) (*Job, bool) {
	job, err := s.store.Request()
	if err != nil {
		s.logger.WithError(err).Errorln("Failed to request job from store")
	}

	if job != nil {
		s.job = job
		s.updateJobStore(JobStoreUpdate{ev: JobStoreUpdateResume})
		return s.job, true
	}

	jobResponse, healthy := s.nw.RequestJob(ctx, *s.config, sessionInfo)
	if jobResponse == nil {
		return nil, healthy
	}

	s.job = NewJob(jobResponse)
	return s.job, healthy
}

func (s *StatefulJobManager) ProcessJob(credentials *JobCredentials) (JobTrace, error) {
	trace, err := s.traceProvider(s, *s.config, credentials, s.job.State.GetSentTrace())
	if err != nil {
		return nil, err
	}

	trace.Start()

	if s.job.State.IsResumed() {
		trace.Disable()
	}

	go s.updateJobHealth(trace, s.config.Store.GetHealthInterval())

	return trace, nil
}

func (s *StatefulJobManager) updateJobHealth(trace JobTrace, healthInterval time.Duration) {
	t := time.NewTicker(healthInterval)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			s.updateJobStore(JobStoreUpdate{ev: JobStoreUpdateHealth})
		case <-trace.Done():
			return
		}
	}
}

func (s *StatefulJobManager) UpdateJob(jobCredentials *JobCredentials, jobInfo UpdateJobInfo) UpdateJobResult {
	if jobInfo.State == Failed || jobInfo.State == Success {
		s.updateJobStore(JobStoreUpdate{ev: JobStoreUpdateRemove})
	}

	return s.nw.UpdateJob(*s.config, jobCredentials, jobInfo)
}

func (s *StatefulJobManager) PatchTrace(jobCredentials *JobCredentials, content []byte, startOffset int, debugModeEnabled bool) PatchTraceResult {
	patchResult := s.nw.PatchTrace(*s.config, jobCredentials, content, startOffset, debugModeEnabled)
	if patchResult.State == PatchSucceeded || patchResult.State == PatchRangeMismatch {
		s.updateJobStore(JobStoreUpdate{ev: JobStoreUpdateTrace, sentTrace: int64(patchResult.SentOffset)})
	} else {
		s.updateJobStore(JobStoreUpdate{ev: JobStoreUpdateHealth})
	}

	return patchResult
}

func (s *StatefulJobManager) updateJobStore(update JobStoreUpdate) {
	s.log().
		WithField("update", update).
		Debugln("Processing job event")

	if update.ev == JobStoreUpdateRemove {
		if err := s.store.Remove(s.job); err != nil {
			s.log().WithError(err).Errorln("Failed to remove job")
		}

		return
	}

	switch update.ev {
	case JobStoreUpdateHealth:
	case JobStoreUpdateTrace:
		s.job.State.SetSentTrace(update.sentTrace)
	case JobStoreUpdateResume:
		s.job.State.Resume()
	}

	// consider each update as a health check too
	s.job.State.UpdateHealth()

	if err := s.store.Update(s.job); err != nil {
		s.log().WithError(err).Errorln("Failed to update job")
	}
}

func (s *StatefulJobManager) log() *logrus.Entry {
	if s.job == nil {
		return s.logger.WithField("job", "nil")
	}

	return s.logger.WithField("job", s.job.ID)
}

type Job struct {
	*JobResponse
	State *JobRuntimeState
}

func NewJob(response *JobResponse) *Job {
	return &Job{
		JobResponse: response,
		State:       NewJobRuntimeState(),
	}
}
