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
	s.logger.Debugln("[FT-DEBUG] RequestJob: Starting job request")
	s.logger.Debugf("[FT-DEBUG] RequestJob: Store type=%T, Store configured=%v", s.store, s.store != nil)
	
	job, err := s.store.Request()
	if err != nil {
		s.logger.WithError(err).Errorln("[FT-DEBUG] RequestJob: Failed to request job from store")
	} else {
		s.logger.Debugf("[FT-DEBUG] RequestJob: Store request completed, job=%v", job != nil)
	}

	if job != nil {
		s.logger.WithFields(logrus.Fields{
			"job_id": job.ID,
			"job_token": job.Token[:8] + "...",
			"resumed": job.State.IsResumed(),
			"stage": job.State.GetStage(),
			"build_state": job.State.GetBuildState(),
			"sent_trace": job.State.GetSentTrace(),
			"started_at": job.State.GetStartedAt(),
		}).Infoln("[FT-DEBUG] RequestJob: Restored job from store")
		
		s.job = job
		s.updateJobStore(JobStoreUpdate{ev: JobStoreUpdateResume})
		return s.job, true
	}

	s.logger.Debugln("[FT-DEBUG] RequestJob: No job in store, requesting from network")
	jobResponse, healthy := s.nw.RequestJob(ctx, *s.config, sessionInfo)
	
	s.logger.WithFields(logrus.Fields{
		"healthy": healthy,
		"job_received": jobResponse != nil,
	}).Debugln("[FT-DEBUG] RequestJob: Network request completed")
	
	if jobResponse == nil {
		s.logger.Debugln("[FT-DEBUG] RequestJob: No job received from network")
		return nil, healthy
	}

	s.job = NewJob(jobResponse)
	s.logger.WithFields(logrus.Fields{
		"job_id": s.job.ID,
		"job_token": s.job.Token[:8] + "...",
	}).Infoln("[FT-DEBUG] RequestJob: New job received from network")
	
	return s.job, healthy
}

func (s *StatefulJobManager) ProcessJob(credentials *JobCredentials) (JobTrace, error) {
	s.logger.WithFields(logrus.Fields{
		"job_id": credentials.ID,
		"sent_trace_offset": s.job.State.GetSentTrace(),
		"is_resumed": s.job.State.IsResumed(),
	}).Debugln("[FT-DEBUG] ProcessJob: Starting job processing")
	
	trace, err := s.traceProvider(s, *s.config, credentials, s.job.State.GetSentTrace())
	if err != nil {
		s.logger.WithError(err).Errorln("[FT-DEBUG] ProcessJob: Failed to create trace")
		return nil, err
	}

	s.logger.Debugln("[FT-DEBUG] ProcessJob: Starting trace")
	trace.Start()

	if s.job.State.IsResumed() {
		s.logger.Infoln("[FT-DEBUG] ProcessJob: Job is resumed, disabling trace initially")
		trace.Disable()
	}

	healthInterval := s.config.Store.GetHealthInterval()
	s.logger.Debugf("[FT-DEBUG] ProcessJob: Starting health update routine with interval=%v", healthInterval)
	go s.updateJobHealth(healthInterval, trace.Done())

	return trace, nil
}

func (s *StatefulJobManager) updateJobHealth(healthInterval time.Duration, done <-chan struct{}) {
	t := time.NewTicker(healthInterval)
	defer t.Stop()

	s.logger.Debugf("[FT-DEBUG] updateJobHealth: Started health update routine, interval=%v", healthInterval)
	updateCount := 0
	
	for {
		select {
		case <-t.C:
			updateCount++
			s.logger.Debugf("[FT-DEBUG] updateJobHealth: Health tick #%d", updateCount)
			s.updateJobStore(JobStoreUpdate{ev: JobStoreUpdateHealth})
		case <-done:
			s.logger.Debugf("[FT-DEBUG] updateJobHealth: Done signal received, stopping health updates after %d ticks", updateCount)
			return
		}
	}
}

func (s *StatefulJobManager) UpdateJob(jobCredentials *JobCredentials, jobInfo UpdateJobInfo) UpdateJobResult {
	s.logger.WithFields(logrus.Fields{
		"job_id": jobCredentials.ID,
		"job_state": jobInfo.State,
		"failure_reason": jobInfo.FailureReason,
	}).Debugln("[FT-DEBUG] UpdateJob: Updating job status")
	
	if jobInfo.State == Failed || jobInfo.State == Success {
		s.logger.WithField("state", jobInfo.State).Infoln("[FT-DEBUG] UpdateJob: Job finished, removing from store")
		s.updateJobStore(JobStoreUpdate{ev: JobStoreUpdateRemove})
	}

	result := s.nw.UpdateJob(*s.config, jobCredentials, jobInfo)
	s.logger.WithField("update_state", result.State).Debugln("[FT-DEBUG] UpdateJob: Network update completed")
	
	return result
}

func (s *StatefulJobManager) PatchTrace(jobCredentials *JobCredentials, content []byte, startOffset int, debugModeEnabled bool) PatchTraceResult {
	s.logger.WithFields(logrus.Fields{
		"job_id": jobCredentials.ID,
		"start_offset": startOffset,
		"content_size": len(content),
		"debug_mode": debugModeEnabled,
	}).Debugln("[FT-DEBUG] PatchTrace: Patching trace")
	
	patchResult := s.nw.PatchTrace(*s.config, jobCredentials, content, startOffset, debugModeEnabled)
	
	s.logger.WithFields(logrus.Fields{
		"patch_state": patchResult.State,
		"sent_offset": patchResult.SentOffset,
	}).Debugln("[FT-DEBUG] PatchTrace: Patch result received")
	
	if patchResult.State == PatchSucceeded || patchResult.State == PatchRangeMismatch {
		s.logger.Debugf("[FT-DEBUG] PatchTrace: Updating trace offset to %d", patchResult.SentOffset)
		s.updateJobStore(JobStoreUpdate{ev: JobStoreUpdateTrace, sentTrace: int64(patchResult.SentOffset)})
	} else {
		s.logger.Debugln("[FT-DEBUG] PatchTrace: Patch failed, updating health only")
		s.updateJobStore(JobStoreUpdate{ev: JobStoreUpdateHealth})
	}

	return patchResult
}

func (s *StatefulJobManager) updateJobStore(update JobStoreUpdate) {
	s.log().
		WithField("update", update).
		Debugln("[FT-DEBUG] updateJobStore: Processing job event")

	if update.ev == JobStoreUpdateRemove {
		s.log().Infoln("[FT-DEBUG] updateJobStore: Removing job from store")
		if err := s.store.Remove(s.job); err != nil {
			s.log().WithError(err).Errorln("[FT-DEBUG] updateJobStore: Failed to remove job")
		} else {
			s.log().Infoln("[FT-DEBUG] updateJobStore: Job removed successfully")
		}
		return
	}

	s.log().Debugf("[FT-DEBUG] updateJobStore: Processing update event: %v", update.ev)
	
	switch update.ev {
	case JobStoreUpdateHealth:
		s.log().Debugln("[FT-DEBUG] updateJobStore: Health update")
	case JobStoreUpdateTrace:
		s.log().Debugf("[FT-DEBUG] updateJobStore: Trace update, setting sent trace to %d", update.sentTrace)
		s.job.State.SetSentTrace(update.sentTrace)
	case JobStoreUpdateResume:
		s.log().Infoln("[FT-DEBUG] updateJobStore: Resume update")
		s.job.State.Resume()
	}

	// consider each update as a health check too
	s.job.State.UpdateHealth()
	
	s.log().WithFields(logrus.Fields{
		"health_check_time": s.job.State.GetHealthCheckAt(),
		"resumed": s.job.State.IsResumed(),
		"stage": s.job.State.GetStage(),
		"build_state": s.job.State.GetBuildState(),
		"sent_trace": s.job.State.GetSentTrace(),
	}).Debugln("[FT-DEBUG] updateJobStore: Current job state before store update")

	if err := s.store.Update(s.job); err != nil {
		s.log().WithError(err).Errorln("[FT-DEBUG] updateJobStore: Failed to update job in store")
	} else {
		s.log().Debugln("[FT-DEBUG] updateJobStore: Job updated successfully in store")
	}
}

func (s *StatefulJobManager) log() logrus.FieldLogger {
	if s.job == nil {
		return s.logger
	}

	return s.logger.WithField("job", s.job.ID)
}

type JobStateToEncoded[T any] interface {
	ToEncoded() (T, error)
}

type JobStateFromEncoded[T any] interface {
	FromEncoded() (T, error)
}

type Job struct {
	*JobResponse
	State *JobRuntimeState
}

var _ JobStateToEncoded[*EncodedJob] = (*Job)(nil)

type EncodedJob struct {
	*JobResponse
	State *EncodedJobRuntimeState
}

var _ JobStateFromEncoded[*Job] = (*EncodedJob)(nil)

func (j *Job) ToEncoded() (*EncodedJob, error) {
	job := &EncodedJob{JobResponse: j.JobResponse}

	state, err := j.State.ToEncoded()
	if err != nil {
		return nil, err
	}
	job.State = state

	return job, nil
}

func (from *EncodedJob) FromEncoded() (*Job, error) {
	job := NewJob(from.JobResponse)
	var err error
	job.State, err = from.State.FromEncoded()

	return job, err
}

func NewJob(response *JobResponse) *Job {
	return &Job{
		JobResponse: response,
		State:       NewJobRuntimeState(),
	}
}
