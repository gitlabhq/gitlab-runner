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
	sentState common.JobState

	updateInterval      time.Duration
	forceSendInterval   time.Duration
	finishRetryInterval time.Duration

	failuresCollector common.FailuresCollector
}

func (c *clientJobTrace) Success() {
	c.Fail(nil, common.NoneFailure)
}

func (c *clientJobTrace) Fail(err error, failureReason common.JobFailureReason) {
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

func (c *clientJobTrace) Write(data []byte) (n int, err error) {
	return c.buffer.Write(data)
}

func (c *clientJobTrace) SetMasked(masked []string) {
	c.buffer.SetMasked(masked)
}

func (c *clientJobTrace) SetCancelFunc(cancelFunc context.CancelFunc) {
	c.cancelFunc = cancelFunc
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
	c.sentState = common.Running
	c.setupLogLimit()
	go c.watch()
}

func (c *clientJobTrace) finish() {
	c.buffer.Close()
	c.finished <- true

	// Do final upload of job trace
	for {
		if c.fullUpdate() != common.UpdateFailed {
			return
		}
		time.Sleep(c.finishRetryInterval)
	}
}

func (c *clientJobTrace) incrementalUpdate() common.UpdateState {
	c.lock.RLock()
	state := c.state
	trace := c.buffer.Bytes()
	c.lock.RUnlock()

	if c.sentTrace != len(trace) {
		result := c.sendPatch(trace)
		if result != common.UpdateSucceeded {
			return result
		}
	}

	if c.sentState != state || time.Since(c.sentTime) > c.forceSendInterval {
		if state == common.Running { // we should only follow-up with Running!
			result := c.sendUpdate(state)
			if result != common.UpdateSucceeded {
				return result
			}
		}
	}

	return common.UpdateSucceeded
}

func (c *clientJobTrace) sendPatch(trace []byte) common.UpdateState {
	tracePatch, err := newTracePatch(trace, c.sentTrace)
	if err != nil {
		c.config.Log().Errorln("Error while creating a tracePatch", err.Error())
	}

	update := c.client.PatchTrace(c.config, c.jobCredentials, tracePatch)
	if update == common.UpdateNotFound {
		return update
	}

	if update == common.UpdateRangeMismatch {
		update = c.resendPatch(c.jobCredentials.ID, c.config, c.jobCredentials, tracePatch)
	}

	if update == common.UpdateSucceeded {
		c.sentTrace = tracePatch.totalSize
		c.sentTime = time.Now()
	}

	return update
}

func (c *clientJobTrace) resendPatch(id int, config common.RunnerConfig, jobCredentials *common.JobCredentials, tracePatch common.JobTracePatch) (update common.UpdateState) {
	if !tracePatch.ValidateRange() {
		config.Log().Warningln(id, "Full job update is needed")
		fullTrace := string(c.buffer.Bytes())

		jobInfo := common.UpdateJobInfo{
			ID:            c.id,
			State:         c.state,
			Trace:         &fullTrace,
			FailureReason: c.failureReason,
		}

		return c.client.UpdateJob(c.config, jobCredentials, jobInfo)
	}

	config.Log().Warningln(id, "Resending trace patch due to range mismatch")

	update = c.client.PatchTrace(config, jobCredentials, tracePatch)
	if update == common.UpdateRangeMismatch {
		config.Log().Errorln(id, "Appending trace to coordinator...", "failed due to range mismatch")

		return common.UpdateFailed
	}

	return
}

func (c *clientJobTrace) sendUpdate(state common.JobState) common.UpdateState {
	jobInfo := common.UpdateJobInfo{
		ID:            c.id,
		State:         state,
		FailureReason: c.failureReason,
	}

	status := c.client.UpdateJob(c.config, c.jobCredentials, jobInfo)
	if status == common.UpdateSucceeded {
		c.sentState = state
		c.sentTime = time.Now()
	}

	return status
}

func (c *clientJobTrace) fullUpdate() common.UpdateState {
	c.lock.RLock()
	state := c.state
	trace := c.buffer.Bytes()
	c.lock.RUnlock()

	if c.sentTrace != len(trace) {
		c.sendPatch(trace) // we don't care about sendPatch() result, in the worst case we will re-send the trace
	}

	jobInfo := common.UpdateJobInfo{
		ID:            c.id,
		State:         state,
		FailureReason: c.failureReason,
	}

	if c.sentTrace != len(trace) {
		traceString := string(trace)
		jobInfo.Trace = &traceString
	}

	update := c.client.UpdateJob(c.config, c.jobCredentials, jobInfo)
	if update == common.UpdateSucceeded {
		c.sentTrace = len(trace)
		c.sentState = state
		c.sentTime = time.Now()
	}

	return update
}

func (c *clientJobTrace) abort() bool {
	if c.cancelFunc != nil {
		c.cancelFunc()
		c.cancelFunc = nil
		return true
	}
	return false
}

func (c *clientJobTrace) watch() {
	for {
		select {
		case <-time.After(c.updateInterval):
			state := c.incrementalUpdate()
			if state == common.UpdateAbort && c.abort() {
				<-c.finished
				return
			}
			break

		case <-c.finished:
			return
		}
	}
}

func (c *clientJobTrace) setupLogLimit() {
	bytesLimit := c.config.OutputLimit
	if bytesLimit == 0 {
		bytesLimit = common.DefaultOutputLimit
	}
	// configuration values are expressed in KB
	bytesLimit *= 1024

	c.buffer.SetLimit(bytesLimit)
}

func newJobTrace(client common.Network, config common.RunnerConfig, jobCredentials *common.JobCredentials) *clientJobTrace {
	return &clientJobTrace{
		client:              client,
		config:              config,
		buffer:              trace.New(),
		jobCredentials:      jobCredentials,
		id:                  jobCredentials.ID,
		updateInterval:      common.UpdateInterval,
		forceSendInterval:   common.ForceTraceSentInterval,
		finishRetryInterval: common.UpdateRetryInterval,
	}
}
