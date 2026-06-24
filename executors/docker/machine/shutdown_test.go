//go:build !integration

package machine

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	docker_executor "gitlab.com/gitlab-org/gitlab-runner/executors/docker"
	"gitlab.com/gitlab-org/gitlab-runner/log/test"
)

const drainWaitingMsg = "Waiting for in-flight operation to settle"

// awaitDrainParked blocks until drain logs drainWaitingMsg, i.e. it has
// parked on the in-flight channel. It uses t.Errorf, never t.FailNow,
// so simulation goroutines can call it.
func awaitDrainParked(t *testing.T, hook *logrustest.Hook) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range hook.AllEntries() {
			if strings.Contains(e.Message, drainWaitingMsg) {
				return
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Errorf("timed out waiting for drain to park (log %q)", drainWaitingMsg)
}

func TestMachineProvider_Shutdown_NoDrainConfig(t *testing.T) {
	p := newMachineProvider(docker_executor.NewProvider())

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	runWithLogCheck(t, "No shutdown drain config, skipping machine pool drain", func() {
		p.Shutdown(ctx, nil)
	})
}

func TestMachineProvider_Shutdown_DrainDisabled(t *testing.T) {
	p := newMachineProvider(docker_executor.NewProvider())

	config := &common.Config{
		Machine: &common.MachineConfig{
			ShutdownDrain: &common.DockerMachineShutdownDrain{
				Enabled:      false,
				Concurrency:  3,
				MaxRetries:   3,
				RetryBackoff: 5 * time.Second,
			},
		},
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	runWithLogCheck(t, "Shutdown drain is disabled, skipping machine pool drain", func() {
		p.Shutdown(ctx, config)
	})
}

func runWithLogCheck(t *testing.T, expectedLastMessage string, fn func()) {
	hook, cleanup := test.NewHook()
	defer cleanup()

	oldLevel := logrus.GetLevel()
	defer func() {
		logrus.SetLevel(oldLevel)
	}()
	logrus.SetLevel(logrus.DebugLevel)

	fn()

	entry := hook.LastEntry()
	if assert.NotNil(t, entry) {
		assert.Equal(t, expectedLastMessage, entry.Message)
	}
}

func TestMachineProvider_Shutdown_NoMachines(t *testing.T) {
	p := newMachineProvider(docker_executor.NewProvider())

	config := &common.Config{
		Machine: &common.MachineConfig{
			ShutdownDrain: &common.DockerMachineShutdownDrain{
				Enabled:      true,
				Concurrency:  3,
				MaxRetries:   3,
				RetryBackoff: 100 * time.Millisecond,
			},
		},
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	p.Shutdown(ctx, config)
}

func TestMachineProvider_Shutdown_DrainsMachines(t *testing.T) {
	machine := newMockMachine(t)

	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine

	config := &common.Config{
		Machine: &common.MachineConfig{
			ShutdownDrain: &common.DockerMachineShutdownDrain{
				Enabled:      true,
				Concurrency:  3,
				MaxRetries:   3,
				RetryBackoff: 100 * time.Millisecond,
			},
		},
	}

	for i := range 5 {
		name := fmt.Sprintf("test-machine-%d", i)
		p.details[name] = &machineDetails{
			Name:    name,
			State:   machineStateIdle,
			Created: time.Now(),
		}
	}

	machine.EXPECT().Exist(mock.Anything, mock.Anything).Return(true)
	machine.EXPECT().ForceRemove(mock.Anything, mock.Anything).Return(nil).Times(5)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	p.Shutdown(ctx, config)

	assert.Empty(t, p.details)
}

func TestMachineProvider_Shutdown_ConcurrencyLimit(t *testing.T) {
	machine := newMockMachine(t)

	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine

	concurrency := 2

	config := &common.Config{
		Machine: &common.MachineConfig{
			ShutdownDrain: &common.DockerMachineShutdownDrain{
				Enabled:      true,
				Concurrency:  concurrency,
				MaxRetries:   1,
				RetryBackoff: 10 * time.Millisecond,
			},
		},
	}

	numMachines := 10
	for i := range numMachines {
		name := fmt.Sprintf("test-machine-%d", i)
		p.details[name] = &machineDetails{
			Name:    name,
			State:   machineStateIdle,
			Created: time.Now(),
		}
	}

	var maxConcurrent int32
	var currentConcurrent int32

	machine.EXPECT().Exist(mock.Anything, mock.Anything).Return(true)
	machine.EXPECT().ForceRemove(mock.Anything, mock.Anything).Run(func(ctx context.Context, name string) {
		current := atomic.AddInt32(&currentConcurrent, 1)
		for {
			old := atomic.LoadInt32(&maxConcurrent)
			if current <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, current) {
				break
			}
		}

		time.Sleep(100 * time.Millisecond)

		atomic.AddInt32(&currentConcurrent, -1)
	}).Return(nil).Times(numMachines)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	p.Shutdown(ctx, config)

	assert.LessOrEqual(t, atomic.LoadInt32(&maxConcurrent), int32(concurrency))
}

func TestMachineProvider_Shutdown_RetryOnFailure(t *testing.T) {
	machine := newMockMachine(t)

	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine

	config := &common.Config{
		Machine: &common.MachineConfig{
			ShutdownDrain: &common.DockerMachineShutdownDrain{
				Enabled:      true,
				Concurrency:  1,
				MaxRetries:   3,
				RetryBackoff: 10 * time.Millisecond,
			},
		},
	}

	p.details["test-machine"] = &machineDetails{
		Name:    "test-machine",
		State:   machineStateIdle,
		Created: time.Now(),
	}

	machine.EXPECT().Exist(mock.Anything, mock.Anything).Return(true)
	machine.EXPECT().ForceRemove(mock.Anything, mock.Anything).Return(assert.AnError).Times(3)
	machine.EXPECT().ForceRemove(mock.Anything, mock.Anything).Return(nil).Once()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	p.Shutdown(ctx, config)
}

func TestMachineProvider_Shutdown_Timeout(t *testing.T) {
	t.Parallel()

	machine := newMockMachine(t)

	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine

	config := &common.Config{
		Machine: &common.MachineConfig{
			ShutdownDrain: &common.DockerMachineShutdownDrain{
				Enabled:      true,
				Concurrency:  1,
				MaxRetries:   3,
				RetryBackoff: 10 * time.Millisecond,
			},
		},
	}

	for i := range 10 {
		name := fmt.Sprintf("test-machine-%d", i)
		p.details[name] = &machineDetails{
			Name:    name,
			State:   machineStateIdle,
			Created: time.Now(),
		}
	}

	machine.EXPECT().Exist(mock.Anything, mock.Anything).Return(true)
	machine.EXPECT().ForceRemove(mock.Anything, mock.Anything).Run(func(ctx context.Context, name string) {
		time.Sleep(50 * time.Millisecond)
	}).Return(nil)

	// Use context timeout to simulate global shutdown_timeout
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	hook, cleanup := test.NewHook()
	defer cleanup()

	start := time.Now()
	p.Shutdown(ctx, config)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 2*time.Second)

	for _, entry := range hook.Entries {
		if strings.Contains(entry.Message, "Drain operation cancelled or timed out") {
			return
		}
	}

	t.Error("missing log entry about canceling the drain operation")
}

func TestMachineProvider_Shutdown_DrainsAllMachineStates(t *testing.T) {
	machine := newMockMachine(t)

	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine

	config := &common.Config{
		Machine: &common.MachineConfig{
			ShutdownDrain: &common.DockerMachineShutdownDrain{
				Enabled:      true,
				Concurrency:  3,
				MaxRetries:   3,
				RetryBackoff: 10 * time.Millisecond,
			},
		},
	}

	p.details["idle-machine"] = &machineDetails{
		Name:    "idle-machine",
		State:   machineStateIdle,
		Created: time.Now(),
	}
	p.details["used-machine"] = &machineDetails{
		Name:    "used-machine",
		State:   machineStateUsed,
		Created: time.Now(),
	}
	p.details["acquired-machine"] = &machineDetails{
		Name:    "acquired-machine",
		State:   machineStateAcquired,
		Created: time.Now(),
	}

	machine.EXPECT().Exist(mock.Anything, mock.Anything).Return(true).Times(3)
	machine.EXPECT().ForceRemove(mock.Anything, mock.Anything).Return(nil).Times(3)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	p.Shutdown(ctx, config)

	assert.Empty(t, p.details)
}

func TestMachineProvider_Shutdown_WaitsForInFlightRemoval(t *testing.T) {
	// state=Removing means another path already called m.remove and
	// finalizeRemoval is running in a goroutine. Drain must wait for it
	// to clear the entry, not skip and count it as success.
	machine := newMockMachine(t)
	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine

	hook, removeHook := test.NewHook()
	defer removeHook()
	logrus.AddHook(hook)

	config := &common.Config{
		Machine: &common.MachineConfig{
			ShutdownDrain: &common.DockerMachineShutdownDrain{
				Enabled:      true,
				Concurrency:  3,
				MaxRetries:   3,
				RetryBackoff: 10 * time.Millisecond,
			},
		},
	}

	p.details["idle-machine"] = &machineDetails{
		Name:    "idle-machine",
		State:   machineStateIdle,
		Created: time.Now(),
	}
	removalDone := make(chan struct{})
	p.details["in-flight-removal"] = &machineDetails{
		Name:     "in-flight-removal",
		State:    machineStateRemoving,
		Created:  time.Now(),
		inFlight: removalDone,
	}

	machine.EXPECT().Exist(mock.Anything, "idle-machine").Return(true).Once()
	machine.EXPECT().ForceRemove(mock.Anything, "idle-machine").Return(nil).Once()
	// in-flight-removal must not hit ForceRemove. Drain waits for the
	// simulated finalizeRemoval to delete the entry.

	// Simulate finalizeRemoval: once drain parks, delete the entry then
	// close the channel (the production order).
	go func() {
		awaitDrainParked(t, hook)
		p.lock.Lock()
		delete(p.details, "in-flight-removal")
		p.lock.Unlock()
		close(removalDone)
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p.Shutdown(ctx, config)

	assert.Empty(t, p.details, "idle drained directly, in-flight-removal drained by waiting")
}

func TestMachineProvider_Shutdown_WaitsForInFlightCreate(t *testing.T) {
	// state=Creating means docker-machine create is mid-flight and the
	// on-disk Driver state is incomplete (e.g. ResolvedZone not yet
	// written for regional MIG creates). Removing now would build a
	// delete with stale coordinates that GCP rejects, so drain must wait
	// for the create to finish.
	machine := newMockMachine(t)
	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine

	hook, removeHook := test.NewHook()
	defer removeHook()
	logrus.AddHook(hook)

	config := &common.Config{
		Machine: &common.MachineConfig{
			ShutdownDrain: &common.DockerMachineShutdownDrain{
				Enabled:      true,
				Concurrency:  1,
				MaxRetries:   3,
				RetryBackoff: 10 * time.Millisecond,
			},
		},
	}

	createDone := make(chan struct{})
	d := &machineDetails{
		Name:     "creating-then-idle",
		State:    machineStateCreating,
		Created:  time.Now(),
		inFlight: createDone,
	}
	p.details["creating-then-idle"] = d

	// Create completes once drain parks: state goes to Idle and the
	// deferred close fires. Drain then takes the normal path.
	go func() {
		awaitDrainParked(t, hook)
		d.Lock()
		d.State = machineStateIdle
		d.Unlock()
		close(createDone)
	}()

	machine.EXPECT().Exist(mock.Anything, "creating-then-idle").Return(true).Once()
	machine.EXPECT().ForceRemove(mock.Anything, "creating-then-idle").Return(nil).Once()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p.Shutdown(ctx, config)

	assert.Empty(t, p.details, "entry drained after create completed")
}

func TestMachineProvider_Shutdown_TransientStateTimeout(t *testing.T) {
	// When the in-flight operation never completes (cloud API hung,
	// runner exits before the goroutine finished), drain must report a
	// failure instead of counting it as success.
	for _, tc := range []struct {
		name  string
		state machineState
	}{
		{"creating never settles", machineStateCreating},
		{"removing never settles", machineStateRemoving},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := newMockMachine(t)
			p := newMachineProvider(docker_executor.NewProvider())
			p.machine = machine

			hook, removeHook := test.NewHook()
			defer removeHook()
			logrus.AddHook(hook)

			config := &common.Config{
				Machine: &common.MachineConfig{
					ShutdownDrain: &common.DockerMachineShutdownDrain{
						Enabled:      true,
						Concurrency:  1,
						MaxRetries:   0,
						RetryBackoff: 10 * time.Millisecond,
					},
				},
			}
			// inFlight set but never closed: the goroutine hangs forever,
			// so drain must time out on ctx.Done().
			p.details["stuck"] = &machineDetails{
				Name:     "stuck",
				State:    tc.state,
				Created:  time.Now(),
				inFlight: make(chan struct{}),
			}
			// No Exist / ForceRemove: drain should be waiting, not removing.

			ctx, cancel := context.WithTimeout(t.Context(), 400*time.Millisecond)
			defer cancel()
			p.Shutdown(ctx, config)

			assert.Len(t, p.details, 1, "stuck entry remains; drain reports failure rather than removing")

			var sawTimeoutWarning, sawFailureSummary bool
			for _, e := range hook.AllEntries() {
				if strings.Contains(e.Message, "did not settle before shutdown timeout") {
					sawTimeoutWarning = true
				}
				if strings.Contains(e.Message, "drain completed") && e.Data["failed"] != nil && fmt.Sprint(e.Data["failed"]) != "0" {
					sawFailureSummary = true
				}
			}
			assert.True(t, sawTimeoutWarning, "should log a timeout warning")
			assert.True(t, sawFailureSummary, "drain summary should count it as failed")
		})
	}
}

func TestMachineProvider_Shutdown_TransientStateWithoutSignal(t *testing.T) {
	// A transient-state entry with no inFlight channel, e.g. loaded from
	// disk before any goroutine attached. Drain must not block on a nil
	// channel. It falls through to the normal path and logs a warning.
	machine := newMockMachine(t)
	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine

	hook, removeHook := test.NewHook()
	defer removeHook()
	logrus.AddHook(hook)

	config := &common.Config{
		Machine: &common.MachineConfig{
			ShutdownDrain: &common.DockerMachineShutdownDrain{
				Enabled:      true,
				Concurrency:  1,
				MaxRetries:   0,
				RetryBackoff: 10 * time.Millisecond,
			},
		},
	}
	p.details["orphan-creating"] = &machineDetails{
		Name:    "orphan-creating",
		State:   machineStateCreating,
		Created: time.Now(),
		// inFlight: nil
	}

	machine.EXPECT().Exist(mock.Anything, "orphan-creating").Return(true).Once()
	machine.EXPECT().ForceRemove(mock.Anything, "orphan-creating").Return(nil).Once()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p.Shutdown(ctx, config)

	assert.Empty(t, p.details)

	var sawFallbackWarning bool
	for _, e := range hook.AllEntries() {
		if strings.Contains(e.Message, "Transient state without completion signal") {
			sawFallbackWarning = true
		}
	}
	assert.True(t, sawFallbackWarning, "should warn that the entry had no completion signal")
}

func TestMachineProvider_Shutdown_WaitsForCreateThenRemoveHandoff(t *testing.T) {
	// Create-failure path: createWithGrowthCapacity fails and calls
	// m.remove in the same goroutine, replacing the inFlight channel
	// before the create goroutine's deferred close fires. Drain captured
	// the original channel, wakes on its close, re-reads, sees Removing
	// with the new channel, and waits on that.
	machine := newMockMachine(t)
	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine

	hook, removeHook := test.NewHook()
	defer removeHook()
	logrus.AddHook(hook)

	config := &common.Config{
		Machine: &common.MachineConfig{
			ShutdownDrain: &common.DockerMachineShutdownDrain{
				Enabled:      true,
				Concurrency:  1,
				MaxRetries:   0,
				RetryBackoff: 10 * time.Millisecond,
			},
		},
	}

	createDone := make(chan struct{})
	removalDone := make(chan struct{})
	d := &machineDetails{
		Name:     "create-then-remove",
		State:    machineStateCreating,
		Created:  time.Now(),
		inFlight: createDone,
	}
	p.details["create-then-remove"] = d

	// Once drain parks: create "fails", so swap to Removing and install
	// the removal channel before closing createDone (production order:
	// remove runs before the create goroutine's deferred close). Then
	// the removal finishes: delete the entry and close removalDone.
	go func() {
		awaitDrainParked(t, hook)
		d.Lock()
		d.State = machineStateRemoving
		d.inFlight = removalDone
		d.Unlock()
		close(createDone)

		p.lock.Lock()
		delete(p.details, "create-then-remove")
		p.lock.Unlock()
		close(removalDone)
	}()

	// Drain never calls ForceRemove: the in-flight removal handles it.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p.Shutdown(ctx, config)

	assert.Empty(t, p.details, "removal goroutine cleared the entry; drain detected via map check")
}

func TestMachineProvider_Shutdown_TransientStateClosedSignalDoesNotSpin(t *testing.T) {
	// An entry stuck in a transient state whose inFlight channel is
	// already closed and never replaced (e.g. a create that errored
	// before its goroutine could transition state). A closed channel is
	// always ready, so re-selecting on it would busy-spin until the
	// deadline. waitForDrainableState must notice the channel is
	// unchanged, warn, and fall through to the normal drain.
	machine := newMockMachine(t)
	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine

	hook, removeHook := test.NewHook()
	defer removeHook()
	logrus.AddHook(hook)

	config := &common.Config{
		Machine: &common.MachineConfig{
			ShutdownDrain: &common.DockerMachineShutdownDrain{
				Enabled:      true,
				Concurrency:  1,
				MaxRetries:   0,
				RetryBackoff: 10 * time.Millisecond,
			},
		},
	}

	closed := make(chan struct{})
	close(closed)
	p.details["stuck-closed"] = &machineDetails{
		Name:     "stuck-closed",
		State:    machineStateCreating,
		Created:  time.Now(),
		inFlight: closed, // already closed, state never changes
	}

	machine.EXPECT().Exist(mock.Anything, "stuck-closed").Return(true).Once()
	machine.EXPECT().ForceRemove(mock.Anything, "stuck-closed").Return(nil).Once()

	// ctx is generous: the test asserts drain returns via ForceRemove,
	// not by hitting the deadline. The watchdog below bounds it.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		p.Shutdown(ctx, config)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown did not return promptly; waitForDrainableState likely spun on the closed channel")
	}

	assert.Empty(t, p.details)
	var sawSpinGuardWarning bool
	for _, e := range hook.AllEntries() {
		if strings.Contains(e.Message, "left machine in a transient state") {
			sawSpinGuardWarning = true
		}
	}
	assert.True(t, sawSpinGuardWarning, "should warn that the signal fired but state stayed transient")
}

func TestMachineProvider_Shutdown_InFlightRemovalGaveUp(t *testing.T) {
	// state=Removing with a finalizeRemoval that exhausts its retries:
	// it sets removalGaveUp, drops the entry, and closes its channel.
	// Drain must count this as a failed drain (VM may be orphaned), not
	// read the cleared map and report success.
	machine := newMockMachine(t)
	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine

	hook, removeHook := test.NewHook()
	defer removeHook()
	logrus.AddHook(hook)

	config := &common.Config{
		Machine: &common.MachineConfig{
			ShutdownDrain: &common.DockerMachineShutdownDrain{
				Enabled:      true,
				Concurrency:  1,
				MaxRetries:   0,
				RetryBackoff: 10 * time.Millisecond,
			},
		},
	}

	removalDone := make(chan struct{})
	d := &machineDetails{
		Name:     "gave-up-removal",
		State:    machineStateRemoving,
		Created:  time.Now(),
		inFlight: removalDone,
	}
	p.details["gave-up-removal"] = d

	// Simulate finalizeRemoval giving up: set removalGaveUp before the
	// delete (production order, so drain observes both), then close.
	// Drain never ForceRemoves it.
	go func() {
		awaitDrainParked(t, hook)
		d.Lock()
		d.removalGaveUp = true
		d.Unlock()
		p.lock.Lock()
		delete(p.details, "gave-up-removal")
		p.lock.Unlock()
		close(removalDone)
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p.Shutdown(ctx, config)

	assert.Empty(t, p.details)

	var sawGaveUpWarning, sawFailureSummary bool
	for _, e := range hook.AllEntries() {
		if strings.Contains(e.Message, "In-flight removal gave up") {
			sawGaveUpWarning = true
		}
		if strings.Contains(e.Message, "drain completed") && fmt.Sprint(e.Data["failed"]) == "1" {
			sawFailureSummary = true
		}
	}
	assert.True(t, sawGaveUpWarning, "should warn that the in-flight removal gave up")
	assert.True(t, sawFailureSummary, "give-up must be counted as a failed drain, not a success")
}

func TestMachineProvider_CollectAllMachines(t *testing.T) {
	p := newMachineProvider(docker_executor.NewProvider())

	states := []struct {
		name  string
		state machineState
	}{
		{"idle-1", machineStateIdle},
		{"idle-2", machineStateIdle},
		{"used", machineStateUsed},
		{"creating", machineStateCreating},
		{"acquired", machineStateAcquired},
		{"removing", machineStateRemoving},
	}

	for _, s := range states {
		p.details[s.name] = &machineDetails{
			Name:  s.name,
			State: s.state,
		}
	}

	machines := p.collectAllMachines()

	assert.Len(t, machines, 6)
}

func TestDockerMachineShutdownDrain_GetConcurrency(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config   common.DockerMachineShutdownDrain
		expected int
	}{
		"zero uses default": {
			config:   common.DockerMachineShutdownDrain{Concurrency: 0},
			expected: 3,
		},
		"custom value": {
			config:   common.DockerMachineShutdownDrain{Concurrency: 10},
			expected: 10,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.GetConcurrency())
		})
	}
}

func TestDockerMachineShutdownDrain_GetMaxRetries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   common.DockerMachineShutdownDrain
		expected int
	}{
		{
			name:     "zero uses default",
			config:   common.DockerMachineShutdownDrain{MaxRetries: 0},
			expected: 3,
		},
		{
			name:     "custom value",
			config:   common.DockerMachineShutdownDrain{MaxRetries: 5},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.GetMaxRetries())
		})
	}
}

func TestDockerMachineShutdownDrain_GetRetryBackoff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   common.DockerMachineShutdownDrain
		expected time.Duration
	}{
		{
			name:     "zero uses default",
			config:   common.DockerMachineShutdownDrain{RetryBackoff: 0},
			expected: 5 * time.Second,
		},
		{
			name:     "custom value",
			config:   common.DockerMachineShutdownDrain{RetryBackoff: 10 * time.Second},
			expected: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.GetRetryBackoff())
		})
	}
}

func TestDockerMachineShutdownDrain_IsEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   common.DockerMachineShutdownDrain
		expected bool
	}{
		{
			name:     "disabled",
			config:   common.DockerMachineShutdownDrain{Enabled: false},
			expected: false,
		},
		{
			name:     "enabled",
			config:   common.DockerMachineShutdownDrain{Enabled: true},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.IsEnabled())
		})
	}
}
