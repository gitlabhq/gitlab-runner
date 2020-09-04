package network

import (
	"context"
	"sync"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/trace"
)

type clientJobTrace struct {
	client         common.Network
	config         common.RunnerConfig
	jobCredentials *common.JobCredentials
	id             int
	cancelFunc     context.CancelFunc

	buffer *trace.Buffer

	lock          sync.RWMutex
	state         common.JobState
	failureReason common.JobFailureReason
	finished      chan bool

	sentTrace int
	sentTime  time.Time

	updateInterval    time.Duration
	forceSendInterval time.Duration
	maxTracePatchSize int

	failuresCollector common.FailuresCollector
}

func (c *clientJobTrace) Success() {
	c.complete(nil, "")
}

func (c *clientJobTrace) complete(err error, failureReason common.JobFailureReason) {
	c.lock.Lock()

	if c.state != common.Running {
		c.lock.Unlock()
		return
	}

	if err == nil {
		c.state = common.Success
	} else {
		c.setFailure(failureReason)
	}

	c.lock.Unlock()
	c.finish()
}

func (c *clientJobTrace) Fail(err error, failureReason common.JobFailureReason) {
	c.complete(err, failureReason)
}

func (c *clientJobTrace) Write(data []byte) (n int, err error) {
	return c.buffer.Write(data)
}

func (c *clientJobTrace) SetMasked(masked []string) {
	c.buffer.SetMasked(masked)
}

func (c *clientJobTrace) SetCancelFunc(cancelFunc context.CancelFunc) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.cancelFunc = cancelFunc
}

func (c *clientJobTrace) Cancel() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.cancelFunc == nil {
		return false
	}

	c.cancelFunc()
	return true
}

func (c *clientJobTrace) SetFailuresCollector(fc common.FailuresCollector) {
	c.failuresCollector = fc
}

func (c *clientJobTrace) IsStdout() bool {
	return false
}

func (c *clientJobTrace) setFailure(reason common.JobFailureReason) {
	c.state = common.Failed
	c.failureReason = reason
	if c.failuresCollector != nil {
		c.failuresCollector.RecordFailure(reason, c.config.ShortDescription())
	}
}

func (c *clientJobTrace) start() {
	c.finished = make(chan bool)
	c.state = common.Running
	c.setupLogLimit()
	go c.watch()
}

func (c *clientJobTrace) ensureAllTraceSent() {
	for c.anyTraceToSend() {
		switch c.sendPatch() {
		case common.PatchSucceeded:
			// we continue sending till we succeed
			continue
		case common.PatchAbort:
			return
		case common.PatchNotFound:
			return
		case common.PatchRangeMismatch:
			time.Sleep(c.getUpdateInterval())
		case common.PatchFailed:
			time.Sleep(c.getUpdateInterval())
		}
	}
}

func (c *clientJobTrace) finalUpdate() {
	// On final-update we want the Runner to fallback
	// to default interval and make Rails to override it
	c.setUpdateInterval(common.DefaultUpdateInterval)

	for {
		// Before sending update to ensure that trace is sent
		// as `sendUpdate()` can force Runner to rewind trace
		c.ensureAllTraceSent()

		switch c.sendUpdate() {
		case common.UpdateSucceeded:
			return
		case common.UpdateAbort:
			return
		case common.UpdateNotFound:
			return
		case common.UpdateAcceptedButNotCompleted:
			time.Sleep(c.getUpdateInterval())
		case common.UpdateTraceValidationFailed:
			time.Sleep(c.getUpdateInterval())
		case common.UpdateFailed:
			time.Sleep(c.getUpdateInterval())
		}
	}
}

func (c *clientJobTrace) finish() {
	c.buffer.Finish()
	c.finished <- true
	c.finalUpdate()
	c.buffer.Close()
}

// incrementalUpdate returns a flag if jobs is supposed
// to be running, or whether it should be finished
func (c *clientJobTrace) incrementalUpdate() bool {
	patchState := c.sendPatch()

	if patchState == common.PatchSucceeded {
		// We try to additionally touch job to check
		// it might be required if no content was send
		// for longer period of time.
		// This is needed to discover if it should be aborted
		touchState := c.touchJob()

		// Try to abort job
		if touchState == common.UpdateAbort && c.abort() {
			return false
		}
	} else if patchState == common.PatchAbort && c.abort() {
		return false
	}

	return true
}

func (c *clientJobTrace) anyTraceToSend() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.buffer.Size() != c.sentTrace
}

func (c *clientJobTrace) sendPatch() common.PatchState {
	c.lock.RLock()
	content, err := c.buffer.Bytes(c.sentTrace, c.maxTracePatchSize)
	sentTrace := c.sentTrace
	c.lock.RUnlock()

	if err != nil {
		return common.PatchFailed
	}

	if len(content) == 0 {
		return common.PatchSucceeded
	}

	result := c.client.PatchTrace(c.config, c.jobCredentials, content, sentTrace)

	c.setUpdateInterval(result.NewUpdateInterval)

	if result.State == common.PatchSucceeded || result.State == common.PatchRangeMismatch {
		c.lock.Lock()
		c.sentTime = time.Now()
		c.sentTrace = result.SentOffset
		c.lock.Unlock()
	}

	return result.State
}

func (c *clientJobTrace) setUpdateInterval(newUpdateInterval time.Duration) {
	if newUpdateInterval <= 0 {
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	c.updateInterval = newUpdateInterval

	// Let's hope that this never happens,
	// but if server behaves bogus do not have too long interval
	if c.updateInterval > common.MaxUpdateInterval {
		c.updateInterval = common.MaxUpdateInterval
	}
}

// Update Coordinator that the job is still running.
func (c *clientJobTrace) touchJob() common.UpdateState {
	c.lock.RLock()
	shouldRefresh := time.Since(c.sentTime) > c.forceSendInterval
	c.lock.RUnlock()

	if !shouldRefresh {
		return common.UpdateSucceeded
	}

	jobInfo := common.UpdateJobInfo{
		ID:    c.id,
		State: common.Running,
	}

	result := c.client.UpdateJob(c.config, c.jobCredentials, jobInfo)

	c.setUpdateInterval(result.NewUpdateInterval)

	if result.State == common.UpdateSucceeded {
		c.lock.Lock()
		c.sentTime = time.Now()
		c.lock.Unlock()
	}

	return result.State
}

func (c *clientJobTrace) sendUpdate() common.UpdateState {
	c.lock.RLock()
	state := c.state
	c.lock.RUnlock()

	jobInfo := common.UpdateJobInfo{
		ID:            c.id,
		State:         state,
		FailureReason: c.failureReason,
	}

	result := c.client.UpdateJob(c.config, c.jobCredentials, jobInfo)

	c.setUpdateInterval(result.NewUpdateInterval)

	if result.State == common.UpdateSucceeded {
		c.lock.Lock()
		c.sentTime = time.Now()
		c.lock.Unlock()
	} else if result.State == common.UpdateTraceValidationFailed {
		c.lock.Lock()
		c.sentTime = time.Now()
		c.sentTrace = 0
		c.lock.Unlock()
	}

	return result.State
}

func (c *clientJobTrace) abort() bool {
	cancelled := c.Cancel()
	c.SetCancelFunc(nil)
	return cancelled
}

func (c *clientJobTrace) watch() {
	for {
		select {
		case <-time.After(c.getUpdateInterval()):
			if !c.incrementalUpdate() {
				// job is no longer running, wait for finish
				<-c.finished
				return
			}

		case <-c.finished:
			return
		}
	}
}

func (c *clientJobTrace) getUpdateInterval() time.Duration {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.updateInterval
}

func (c *clientJobTrace) setupLogLimit() {
	bytesLimit := c.config.OutputLimit * 1024 // convert to bytes
	if bytesLimit == 0 {
		bytesLimit = common.DefaultTraceOutputLimit
	}

	c.buffer.SetLimit(bytesLimit)
}

func newJobTrace(
	client common.Network,
	config common.RunnerConfig,
	jobCredentials *common.JobCredentials,
) (*clientJobTrace, error) {
	buffer, err := trace.New()
	if err != nil {
		return nil, err
	}

	return &clientJobTrace{
		client:            client,
		config:            config,
		buffer:            buffer,
		jobCredentials:    jobCredentials,
		id:                jobCredentials.ID,
		maxTracePatchSize: common.DefaultTracePatchLimit,
		updateInterval:    common.DefaultUpdateInterval,
		forceSendInterval: common.TraceForceSendInterval,
	}, nil
}
