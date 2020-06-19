package network

import (
	"context"
	"sync"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/trace"
)

const (
	emptyRemoteTraceUpdateInterval = 0
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

	updateInterval      time.Duration
	forceSendInterval   time.Duration
	finishRetryInterval time.Duration
	maxTracePatchSize   int

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
		switch c.sendUpdate() {
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
	c.buffer.Finish()
	c.finished <- true
	c.finalTraceUpdate()
	c.finalStatusUpdate()
	c.buffer.Close()
}

func (c *clientJobTrace) incrementalUpdate() common.UpdateState {
	state := c.sendPatch()
	if state != common.UpdateSucceeded {
		return state
	}

	return c.touchJob()
}

func (c *clientJobTrace) anyTraceToSend() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.buffer.Size() != c.sentTrace
}

func (c *clientJobTrace) sendPatch() common.UpdateState {
	c.lock.RLock()
	content, err := c.buffer.Bytes(c.sentTrace, c.maxTracePatchSize)
	sentTrace := c.sentTrace
	c.lock.RUnlock()

	if err != nil {
		return common.UpdateFailed
	}

	if len(content) == 0 {
		return common.UpdateSucceeded
	}

	result := c.client.PatchTrace(c.config, c.jobCredentials, content, sentTrace)

	c.setUpdateInterval(result.NewUpdateInterval)

	if result.State == common.UpdateSucceeded || result.State == common.UpdateRangeMismatch {
		c.lock.Lock()
		c.sentTime = time.Now()
		c.sentTrace = result.SentOffset
		c.lock.Unlock()
	}

	return result.State
}

func (c *clientJobTrace) setUpdateInterval(newUpdateInterval time.Duration) {
	if newUpdateInterval <= time.Duration(emptyRemoteTraceUpdateInterval) {
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	c.updateInterval = newUpdateInterval
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

	status := c.client.UpdateJob(c.config, c.jobCredentials, jobInfo)

	if status == common.UpdateSucceeded {
		c.lock.Lock()
		c.sentTime = time.Now()
		c.lock.Unlock()
	}

	return status
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

	status := c.client.UpdateJob(c.config, c.jobCredentials, jobInfo)

	if status == common.UpdateSucceeded {
		c.lock.Lock()
		c.sentTime = time.Now()
		c.lock.Unlock()
	}

	return status
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
		client:              client,
		config:              config,
		buffer:              buffer,
		jobCredentials:      jobCredentials,
		id:                  jobCredentials.ID,
		maxTracePatchSize:   common.DefaultTracePatchLimit,
		updateInterval:      common.DefaultTraceUpdateInterval,
		forceSendInterval:   common.TraceForceSendInterval,
		finishRetryInterval: common.TraceFinishRetryInterval,
	}, nil
}
