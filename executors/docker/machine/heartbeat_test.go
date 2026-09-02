//go:build !integration

package machine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

type touchRecorder struct {
	lock  sync.Mutex
	calls map[string]int
	errs  map[string]error
	done  chan struct{}
}

func newTouchRecorder() *touchRecorder {
	return &touchRecorder{
		calls: map[string]int{},
		errs:  map[string]error{},
		done:  make(chan struct{}, 100),
	}
}

func (r *touchRecorder) touch(_ context.Context, name string, _ string) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.calls[name]++
	r.done <- struct{}{}
	return r.errs[name]
}

func (r *touchRecorder) callCount(name string) int {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.calls[name]
}

func (r *touchRecorder) waitForCall(t *testing.T) {
	select {
	case <-r.done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for touch call")
	}
}

// testClock is mutex-guarded because the beat goroutine reads it via
// nowFn after waitForCall returns.
type testClock struct {
	lock sync.Mutex
	t    time.Time
}

func (c *testClock) now() time.Time {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.t
}

func (c *testClock) advance(d time.Duration) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.t = c.t.Add(d)
}

func newTestHeartbeater(touch func(context.Context, string, string) error) (*heartbeater, *testClock) {
	clock := &testClock{t: time.Now()}
	h := newHeartbeater(touch, make(chan struct{}, 1))
	h.nowFn = clock.now
	h.randFn = func(n int64) int64 { return n }
	return h, clock
}

func TestHeartbeaterBeatsWhenDue(t *testing.T) {
	rec := newTouchRecorder()
	h, clock := newTestHeartbeater(rec.touch)

	h.beat("m1", time.Minute)
	rec.waitForCall(t)
	assert.Equal(t, 1, rec.callCount("m1"))

	h.beat("m1", time.Minute)
	assert.Equal(t, 1, rec.callCount("m1"))

	clock.advance(2 * time.Minute)
	h.beat("m1", time.Minute)
	rec.waitForCall(t)
	assert.Equal(t, 2, rec.callCount("m1"))
}

func TestHeartbeaterDisabledInterval(t *testing.T) {
	rec := newTouchRecorder()
	h, _ := newTestHeartbeater(rec.touch)

	h.beat("m1", 0)
	assert.Equal(t, 0, rec.callCount("m1"))
}

func TestHeartbeaterFailureCountsAsBeat(t *testing.T) {
	rec := newTouchRecorder()
	rec.errs["m1"] = assert.AnError
	h, _ := newTestHeartbeater(rec.touch)

	h.beat("m1", time.Minute)
	rec.waitForCall(t)
	require.Equal(t, 1, rec.callCount("m1"))

	h.beat("m1", time.Minute)
	assert.Equal(t, 1, rec.callCount("m1"))
}

func TestHeartbeaterFirstBeatJitter(t *testing.T) {
	rec := newTouchRecorder()
	h, clock := newTestHeartbeater(rec.touch)
	h.randFn = func(n int64) int64 { return 0 }

	h.beat("m1", time.Minute)
	assert.Equal(t, 0, rec.callCount("m1"))

	clock.advance(2 * time.Minute)
	h.beat("m1", time.Minute)
	rec.waitForCall(t)
	assert.Equal(t, 1, rec.callCount("m1"))
}

func TestHeartbeaterPrune(t *testing.T) {
	rec := newTouchRecorder()
	h, _ := newTestHeartbeater(rec.touch)

	h.beat("m1", time.Minute)
	rec.waitForCall(t)
	h.beat("m2", time.Minute)
	rec.waitForCall(t)

	h.prune([]string{"m2"})

	assert.Equal(t, 1, rec.callCount("m1"))
	assert.Equal(t, 1, rec.callCount("m2"))

	h.lock.Lock()
	_, m1Kept := h.lastBeat["m1"]
	_, m2Kept := h.lastBeat["m2"]
	h.lock.Unlock()
	assert.False(t, m1Kept)
	assert.True(t, m2Kept)
}

func TestHeartbeaterPerRunnerIsolation(t *testing.T) {
	p := newMachineProvider(nil)

	configFor := func(token string) *common.RunnerConfig {
		return &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{Token: token},
			RunnerSettings: common.RunnerSettings{
				Machine: &common.DockerMachine{},
			},
		}
	}

	h1 := p.heartbeaterFor(configFor("runner-one"))
	h2 := p.heartbeaterFor(configFor("runner-two"))

	require.NotSame(t, h1, h2)
	assert.Same(t, p.heartbeaterFor(configFor("runner-one")), h1)
	assert.Equal(t, h1.sem, h2.sem)

	h1.lastBeat["m1"] = time.Now()
	h2.lastBeat["m2"] = time.Now()
	h1.prune(nil)

	assert.Empty(t, h1.lastBeat)
	assert.Len(t, h2.lastBeat, 1)
}

func TestHeartbeaterBoundsTouchContext(t *testing.T) {
	deadlineSet := make(chan bool, 1)
	touch := func(ctx context.Context, _ string, _ string) error {
		_, ok := ctx.Deadline()
		deadlineSet <- ok
		return nil
	}
	h, _ := newTestHeartbeater(touch)

	h.beat("m1", time.Minute)
	select {
	case ok := <-deadlineSet:
		assert.True(t, ok, "touch context must carry a deadline")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for touch call")
	}
}

func TestHeartbeaterTicksWhileAcquireIsPaused(t *testing.T) {
	rec := newTouchRecorder()
	h, clock := newTestHeartbeater(rec.touch)
	h.tickPeriod = 10 * time.Millisecond

	h.beat("m1", time.Minute)
	rec.waitForCall(t)
	require.Equal(t, 1, rec.callCount("m1"))

	clock.advance(90 * time.Second)
	rec.waitForCall(t)
	assert.GreaterOrEqual(t, rec.callCount("m1"), 2)
}

func TestHeartbeaterDisablesOnUnsupportedDriver(t *testing.T) {
	t.Parallel()

	rec := newTouchRecorder()
	rec.errs["m1"] = docker.ErrLabelsNotSupported
	h, clock := newTestHeartbeater(rec.touch)

	h.beat("m1", time.Minute)
	rec.waitForCall(t)
	require.Equal(t, 1, rec.callCount("m1"))

	// No further beats for any machine once the driver is known not to
	// support labels.
	clock.advance(2 * time.Minute)
	h.beat("m1", time.Minute)
	h.beat("m2", time.Minute)
	assert.Equal(t, 1, rec.callCount("m1"))
	assert.Equal(t, 0, rec.callCount("m2"))
}
