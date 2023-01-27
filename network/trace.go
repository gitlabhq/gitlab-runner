package network

import (
	"context"
	"sync"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/trace"
)

type clientJobTrace struct {
	client         common.Network
	config         common.RunnerConfig
	jobCredentials *common.JobCredentials
	id             int64
	cancelFunc     context.CancelFunc
	abortFunc      context.CancelFunc

	debugModeEnabled bool

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
	exitCode          int
}

func (c *clientJobTrace) Success() {
	c.complete(nil, common.JobFailureData{})
}

func (c *clientJobTrace) complete(err error, failureData common.JobFailureData) {
	c.lock.Lock()

	if c.state != common.Running {
		c.lock.Unlock()
		return
	}

	if err == nil {
		c.state = common.Success
	} else {
		c.setFailure(failureData)
	}

	c.lock.Unlock()
	c.finish()
}

func (c *clientJobTrace) Fail(err error, failureData common.JobFailureData) {
	c.complete(err, failureData)
}

func (c *clientJobTrace) Write(data []byte) (n int, err error) {
	return c.buffer.Write(data)
}

func (c *clientJobTrace) SetMasked(opts common.MaskOptions) {
	c.buffer.SetMasked(opts)
}

func (c *clientJobTrace) checksum() string {
	return c.buffer.Checksum()
}

func (c *clientJobTrace) bytesize() int {
	return c.buffer.Size()
}

// SetCancelFunc sets the function to be called by Cancel(). The function
// provided here should cancel the execution of any stages that are not
// absolutely required, whilst allowing for stages such as `after_script` to
// proceed.
func (c *clientJobTrace) SetCancelFunc(cancelFunc context.CancelFunc) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.cancelFunc = cancelFunc
}

// Cancel consumes the function set by SetCancelFunc.
func (c *clientJobTrace) Cancel() bool {
	c.lock.RLock()
	cancelFunc := c.cancelFunc
	c.lock.RUnlock()

	if cancelFunc == nil {
		return false
	}

	c.SetCancelFunc(nil)
	cancelFunc()
	return true
}

// SetAbortFunc sets the function to be called by Abort(). The function
// provided here should abort the execution of all stages.
func (c *clientJobTrace) SetAbortFunc(cancelFunc context.CancelFunc) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.abortFunc = cancelFunc
}

// Abort consumes function set by SetAbortFunc
// The abort always have much higher importance than Cancel
// as abort interrupts the execution, thus cancel is never
// called after the Abort
func (c *clientJobTrace) Abort() bool {
	c.lock.RLock()
	abortFunc := c.abortFunc
	c.lock.RUnlock()

	if abortFunc == nil {
		return false
	}

	c.SetCancelFunc(nil)
	c.SetAbortFunc(nil)

	abortFunc()
	return true
}

func (c *clientJobTrace) SetFailuresCollector(fc common.FailuresCollector) {
	c.failuresCollector = fc
}

func (c *clientJobTrace) IsStdout() bool {
	return false
}

func (c *clientJobTrace) setFailure(data common.JobFailureData) {
	c.state = common.Failed
	c.failureReason = data.Reason
	c.exitCode = data.ExitCode
	if c.failuresCollector != nil {
		c.failuresCollector.RecordFailure(data.Reason, c.config.ShortDescription())
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
		switch c.sendPatch().State {
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
	patchResult := c.sendPatch()
	if patchResult.CancelRequested {
		c.Cancel()
	}

	switch patchResult.State {
	case common.PatchSucceeded:
		// We try to additionally touch job to check
		// it might be required if no content was send
		// for longer period of time.
		// This is needed to discover if it should be aborted
		touchResult := c.touchJob()
		if touchResult.CancelRequested {
			c.Cancel()
		}

		if touchResult.State == common.UpdateAbort {
			c.Abort()
			return false
		}
	case common.PatchAbort:
		c.Abort()
		return false
	}

	return true
}

func (c *clientJobTrace) anyTraceToSend() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.buffer.Size() != c.sentTrace
}

func (c *clientJobTrace) sendPatch() common.PatchTraceResult {
	c.lock.RLock()
	content, err := c.buffer.Bytes(c.sentTrace, c.maxTracePatchSize)
	sentTrace := c.sentTrace
	c.lock.RUnlock()

	if err != nil {
		return common.PatchTraceResult{State: common.PatchFailed}
	}

	if len(content) == 0 {
		return common.PatchTraceResult{State: common.PatchSucceeded}
	}

	result := c.client.PatchTrace(c.config, c.jobCredentials, content, sentTrace, c.debugModeEnabled)

	c.setUpdateInterval(result.NewUpdateInterval)

	if result.State == common.PatchSucceeded || result.State == common.PatchRangeMismatch {
		c.lock.Lock()
		c.sentTime = time.Now()
		c.sentTrace = result.SentOffset
		c.lock.Unlock()
	}

	return result
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

	if c.config.IsFeatureFlagOn(featureflags.UseDynamicTraceForceSendInterval) {
		c.forceSendInterval = c.updateInterval * common.TraceForceSendUpdateIntervalMultiplier

		if c.forceSendInterval < common.MinTraceForceSendInterval {
			c.forceSendInterval = common.MinTraceForceSendInterval
		}
		if c.forceSendInterval > common.MaxTraceForceSendInterval {
			c.forceSendInterval = common.MaxTraceForceSendInterval
		}
	}
}

// Update Coordinator that the job is still running.
func (c *clientJobTrace) touchJob() common.UpdateJobResult {
	c.lock.RLock()
	shouldRefresh := time.Since(c.sentTime) > c.forceSendInterval
	c.lock.RUnlock()

	if !shouldRefresh {
		return common.UpdateJobResult{State: common.UpdateSucceeded}
	}

	jobInfo := common.UpdateJobInfo{
		ID:    c.id,
		State: common.Running,
		Output: common.JobTraceOutput{
			Checksum: c.checksum(),
			Bytesize: c.bytesize(),
		},
	}

	result := c.client.UpdateJob(c.config, c.jobCredentials, jobInfo)

	c.setUpdateInterval(result.NewUpdateInterval)

	if result.State == common.UpdateSucceeded {
		c.lock.Lock()
		c.sentTime = time.Now()
		c.lock.Unlock()
	}

	return result
}

func (c *clientJobTrace) sendUpdate() common.UpdateState {
	c.lock.RLock()
	state := c.state
	c.lock.RUnlock()

	jobInfo := common.UpdateJobInfo{
		ID:            c.id,
		State:         state,
		FailureReason: c.failureReason,
		Output: common.JobTraceOutput{
			Checksum: c.checksum(),
			Bytesize: c.bytesize(),
		},
		ExitCode: c.exitCode,
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

func (c *clientJobTrace) IsMaskingURLParams() bool {
	return c.config.IsFeatureFlagOn(featureflags.UseImprovedURLMasking)
}

func (c *clientJobTrace) SetDebugModeEnabled(isEnabled bool) {
	c.debugModeEnabled = isEnabled
}

func newJobTrace(
	client common.Network,
	config common.RunnerConfig,
	jobCredentials *common.JobCredentials,
) (*clientJobTrace, error) {
	buffer, err := trace.New(trace.WithURLParamMasking(config.IsFeatureFlagOn(featureflags.UseImprovedURLMasking)))
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
		forceSendInterval: common.MinTraceForceSendInterval,
	}, nil
}
