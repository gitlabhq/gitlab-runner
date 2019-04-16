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
	maxTracePatchSize   int

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

func (c *clientJobTrace) finalTraceUpdate() {
	for c.anyTraceToSend() {
		switch c.sendPatch() {
		case common.UpdateSucceeded:
			// we continue sending till we succeed
			continue
		case common.UpdateAbort:
			return
		case common.UpdateNotFound:
			return
		case common.UpdateRangeMismatch:
			time.Sleep(c.finishRetryInterval)
		case common.UpdateFailed:
			time.Sleep(c.finishRetryInterval)
		}
	}
}

func (c *clientJobTrace) finalStatusUpdate() {
	for {
		switch c.sendUpdate(true) {
		case common.UpdateSucceeded:
			return
		case common.UpdateAbort:
			return
		case common.UpdateNotFound:
			return
		case common.UpdateRangeMismatch:
			return
		case common.UpdateFailed:
			time.Sleep(c.finishRetryInterval)
		}
	}
}

func (c *clientJobTrace) finish() {
	c.buffer.Close()
	c.finished <- true
	c.finalTraceUpdate()
	c.finalStatusUpdate()
}

func (c *clientJobTrace) incrementalUpdate() common.UpdateState {
	state := c.sendPatch()
	if state != common.UpdateSucceeded {
		return state
	}

	return c.sendUpdate(false)
}

func (c *clientJobTrace) anyTraceToSend() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return len(c.buffer.Bytes()) != c.sentTrace
}

func (c *clientJobTrace) sendPatch() common.UpdateState {
	c.lock.RLock()
	trace := c.buffer.Bytes()
	sentTrace := c.sentTrace
	c.lock.RUnlock()

	if len(trace) == sentTrace {
		return common.UpdateSucceeded
	}

	// we send at most `maxTracePatchSize` in single patch
	content := trace[sentTrace:]
	if len(content) > c.maxTracePatchSize {
		content = content[:c.maxTracePatchSize]
	}

	sentOffset, state := c.client.PatchTrace(
		c.config, c.jobCredentials, content, sentTrace)

	if state == common.UpdateSucceeded || state == common.UpdateRangeMismatch {
		c.lock.Lock()
		c.sentTime = time.Now()
		c.sentTrace = sentOffset
		c.lock.Unlock()
	}

	return state
}

func (c *clientJobTrace) sendUpdate(force bool) common.UpdateState {
	c.lock.RLock()
	state := c.state
	shouldUpdateState := c.state != c.sentState
	shouldRefresh := time.Since(c.sentTime) > c.forceSendInterval
	c.lock.RUnlock()

	if !force && !shouldUpdateState && !shouldRefresh {
		return common.UpdateSucceeded
	}

	jobInfo := common.UpdateJobInfo{
		ID:            c.id,
		State:         state,
		FailureReason: c.failureReason,
	}

	status := c.client.UpdateJob(c.config, c.jobCredentials, jobInfo)

	if status == common.UpdateSucceeded {
		c.lock.Lock()
		c.sentTime = time.Now()
		c.sentState = state
		c.lock.Unlock()
	}

	return status
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
	bytesLimit := c.config.OutputLimit * 1024 // convert to bytes
	if bytesLimit == 0 {
		bytesLimit = common.DefaultOutputLimit
	}

	c.buffer.SetLimit(bytesLimit)
}

func newJobTrace(client common.Network, config common.RunnerConfig, jobCredentials *common.JobCredentials) *clientJobTrace {
	return &clientJobTrace{
		client:              client,
		config:              config,
		buffer:              trace.New(),
		jobCredentials:      jobCredentials,
		id:                  jobCredentials.ID,
		maxTracePatchSize:   common.DefaultTracePatchLimit,
		updateInterval:      common.UpdateInterval,
		forceSendInterval:   common.ForceTraceSentInterval,
		finishRetryInterval: common.UpdateRetryInterval,
	}
}
