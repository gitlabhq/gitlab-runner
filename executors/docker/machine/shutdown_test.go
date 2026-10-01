//go:build !integration

package machine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	docker_executor "gitlab.com/gitlab-org/gitlab-runner/executors/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
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

func drainConfig(concurrency, maxRetries int) *common.Config {
	return &common.Config{
		Machine: &common.MachineConfig{
			ShutdownDrain: &common.DockerMachineShutdownDrain{
				Enabled:      true,
				Early:        true,
				Concurrency:  concurrency,
				MaxRetries:   maxRetries,
				RetryBackoff: time.Millisecond,
			},
		},
	}
}

func withShortDrainPoll(t *testing.T) {
	t.Helper()
	old := shutdownDrainPollInterval
	shutdownDrainPollInterval = 10 * time.Millisecond
	t.Cleanup(func() { shutdownDrainPollInterval = old })
}

func trackedState(p *machineProvider, name string) (machineState, bool) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	d, ok := p.details[name]
	if !ok {
		return 0, false
	}
	d.Lock()
	defer d.Unlock()
	return d.State, true
}

func assertMachineState(t *testing.T, p *machineProvider, name string, want machineState) {
	t.Helper()
	got, ok := trackedState(p, name)
	if assert.True(t, ok, "%s should still be tracked", name) {
		assert.Equal(t, want, got, "%s state", name)
	}
}

func assertMachineGone(t *testing.T, p *machineProvider, name string) {
	t.Helper()
	assert.Eventually(t, func() bool {
		_, ok := trackedState(p, name)
		return !ok
	}, 5*time.Second, time.Millisecond, "%s should have been drained", name)
}

func TestMachineProvider_BeginShutdown_DrainsIdleWhileJobsRun(t *testing.T) {
	withShortDrainPoll(t)

	machine := newMockMachine(t)
	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine

	hook, removeHook := test.NewHook()
	defer removeHook()

	createDone := make(chan struct{})
	for _, d := range []*machineDetails{
		{Name: "idle-1", State: machineStateIdle},
		{Name: "idle-2", State: machineStateIdle},
		{Name: "used", State: machineStateUsed},
		{Name: "acquired", State: machineStateAcquired},
		{Name: "creating", State: machineStateCreating, inFlight: createDone},
	} {
		d.Created = time.Now()
		p.details[d.Name] = d
	}

	// The mock fails the test on any name not listed here.
	for _, name := range []string{"idle-1", "idle-2", "creating", "used", "acquired"} {
		machine.EXPECT().Exist(mock.Anything, name).Return(true).Once()
		machine.EXPECT().ForceRemove(mock.Anything, name).Return(nil).Once()
	}

	p.BeginShutdown(drainConfig(3, 1))

	assertMachineGone(t, p, "idle-1")
	assertMachineGone(t, p, "idle-2")
	time.Sleep(5 * shutdownDrainPollInterval)
	assertMachineState(t, p, "used", machineStateUsed)
	assertMachineState(t, p, "acquired", machineStateAcquired)
	assertMachineState(t, p, "creating", machineStateCreating)

	// The create finishes: a later pass picks the machine up as Idle.
	d := p.details["creating"]
	d.Lock()
	d.State = machineStateIdle
	d.Unlock()
	close(createDone)
	assertMachineGone(t, p, "creating")

	// A job finishes and releases its machine to the pool.
	p.Release(nil, p.details["used"])
	assertMachineGone(t, p, "used")
	assertMachineState(t, p, "acquired", machineStateAcquired)

	// Shutdown takes whatever is left, in any state.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p.Shutdown(ctx, drainConfig(3, 1))

	assert.Empty(t, p.details)

	var idlePasses int
	var summary *logrus.Entry
	for _, e := range hook.AllEntries() {
		switch e.Message {
		case "Draining idle machines":
			idlePasses++
		case "Machine pool drain completed":
			summary = e
		}
	}
	assert.Equal(t, 3, idlePasses, "one pass per batch of idle machines")
	if assert.NotNil(t, summary, "one summary for the whole drain") {
		assert.Equal(t, 5, summary.Data["success"])
		assert.Equal(t, 0, summary.Data["failed"])
		assert.Equal(t, 0, summary.Data["remaining"])
	}
}

func TestMachineProvider_BeginShutdown_RefusesAcquire(t *testing.T) {
	p, _ := testMachineProvider("test-machine")
	p.details["test-machine"] = &machineDetails{Name: "test-machine", State: machineStateIdle, Created: time.Now()}

	p.BeginShutdown(drainConfig(1, 0))
	t.Cleanup(func() {
		p.drain.cancel()
		<-p.drain.done
	})

	data, err := p.Acquire(machineDefaultConfig)
	assert.Nil(t, data)
	var noFree *common.NoFreeExecutorError
	assert.ErrorAs(t, err, &noFree)
}

func TestMachineProvider_BeginShutdown_NotEarly(t *testing.T) {
	for name, config := range map[string]*common.Config{
		"no config":   nil,
		"not early":   {Machine: &common.MachineConfig{ShutdownDrain: &common.DockerMachineShutdownDrain{Enabled: true}}},
		"not enabled": {Machine: &common.MachineConfig{ShutdownDrain: &common.DockerMachineShutdownDrain{Early: true}}},
	} {
		t.Run(name, func(t *testing.T) {
			p, _ := testMachineProvider("test-machine")

			hook, removeHook := test.NewHook()
			defer removeHook()

			p.BeginShutdown(config)

			data, err := p.Acquire(machineDefaultConfig)
			assert.NoError(t, err)
			assert.NotNil(t, data, "Acquire still hands out machines")
			assert.Nil(t, p.drain, "no drain was started")
			assert.Empty(t, hook.AllEntries(), "nothing to say when the early drain is off")
		})
	}
}

func TestClaimIdle(t *testing.T) {
	for _, tc := range []struct {
		name       string
		state      machineState
		pendingUse bool
		want       bool
	}{
		{name: "idle", state: machineStateIdle, want: true},
		{name: "idle pending use", state: machineStateIdle, pendingUse: true, want: false},
		{name: "acquired", state: machineStateAcquired, want: false},
		{name: "used", state: machineStateUsed, want: false},
		{name: "creating", state: machineStateCreating, want: false},
		{name: "removing", state: machineStateRemoving, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &machineDetails{Name: "m", State: tc.state, pendingUse: tc.pendingUse}
			assert.Equal(t, tc.want, claimIdle(d))
			if tc.want {
				assert.Equal(t, machineStateRemoving, d.State)
				assert.Equal(t, drainReason, d.Reason)
			} else {
				assert.Equal(t, tc.state, d.State, "state left untouched")
			}
		})
	}
}

func TestCreateAndAcquireMachine_ClearsPendingUse(t *testing.T) {
	p, _ := testMachineProvider()

	details, err := p.createAndAcquireMachine(machineDefaultConfig)
	require.NoError(t, err)
	require.NotNil(t, details)
	assert.Equal(t, machineStateAcquired, details.State)
	assert.False(t, details.pendingUse)
}

func TestMachineProvider_BeginShutdown_FailedMachineRetriedInFinalPassOnly(t *testing.T) {
	withShortDrainPoll(t)

	machine := newMockMachine(t)
	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine

	hook, removeHook := test.NewHook()
	defer removeHook()

	p.details["flaky"] = &machineDetails{Name: "flaky", State: machineStateIdle, Created: time.Now()}

	machine.EXPECT().Exist(mock.Anything, "flaky").Return(true)
	// MaxRetries 1: two attempts in the idle pass, none until the final pass.
	machine.EXPECT().ForceRemove(mock.Anything, "flaky").Return(assert.AnError).Times(2)
	machine.EXPECT().ForceRemove(mock.Anything, "flaky").Return(nil).Once()

	p.BeginShutdown(drainConfig(1, 1))

	assert.Eventually(t, func() bool {
		return p.drain.hasFailed("flaky")
	}, 5*time.Second, time.Millisecond)
	time.Sleep(5 * shutdownDrainPollInterval)
	assertMachineState(t, p, "flaky", machineStateRemoving)
	machine.AssertNumberOfCalls(t, "ForceRemove", 2)
	var idlePasses int
	for _, e := range hook.AllEntries() {
		if e.Message == "Draining idle machines" {
			idlePasses++
		}
	}
	assert.Equal(t, 1, idlePasses, "a failed machine is not announced again every pass")

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p.Shutdown(ctx, drainConfig(1, 1))

	assert.Empty(t, p.details)
	machine.AssertNumberOfCalls(t, "ForceRemove", 3)

	for _, e := range hook.AllEntries() {
		if e.Message == "Machine pool drain completed" {
			assert.Equal(t, 1, e.Data["success"])
			assert.Equal(t, 0, e.Data["failed"], "a machine removed on retry does not count as failed")
			return
		}
	}
	t.Error("missing drain summary")
}

func TestMachineProvider_Shutdown_TimeoutAbortsEarlyDrain(t *testing.T) {
	machine := newMockMachine(t)
	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine

	hook, removeHook := test.NewHook()
	defer removeHook()

	for i := range 5 {
		name := fmt.Sprintf("idle-%d", i)
		p.details[name] = &machineDetails{Name: name, State: machineStateIdle, Created: time.Now()}
	}

	started := make(chan struct{}, 1)
	machine.EXPECT().Exist(mock.Anything, mock.Anything).Return(true)
	machine.EXPECT().ForceRemove(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, _ string) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return ctx.Err()
	})

	p.BeginShutdown(drainConfig(1, 0))
	<-started

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		p.Shutdown(ctx, drainConfig(1, 0))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown did not return after its context expired")
	}

	assert.Len(t, p.details, 5, "nothing was removed")
	var sawCancel bool
	for _, e := range hook.AllEntries() {
		if e.Message == "Drain operation cancelled or timed out" {
			sawCancel = true
		}
	}
	assert.True(t, sawCancel)
}

func TestMachineProvider_Drain_ForgetsHeartbeat(t *testing.T) {
	machine := newMockMachine(t)
	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine

	rec := newTouchRecorder()
	h := newHeartbeater(rec.touch, make(chan struct{}, 1))
	p.heartbeaters = map[string]*heartbeater{"token": h}
	h.beat("idle", time.Hour)
	h.lock.Lock()
	_, tracked := h.lastBeat["idle"]
	h.lock.Unlock()
	require.True(t, tracked)

	p.details["idle"] = &machineDetails{Name: "idle", State: machineStateIdle, Created: time.Now()}
	machine.EXPECT().Exist(mock.Anything, "idle").Return(true).Once()
	machine.EXPECT().ForceRemove(mock.Anything, "idle").Return(nil).Once()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p.Shutdown(ctx, drainConfig(1, 0))

	h.lock.Lock()
	_, tracked = h.lastBeat["idle"]
	h.lock.Unlock()
	assert.False(t, tracked, "drained machine must not be heartbeaten any more")
}

func TestMachineProvider_CollectIdleMachines(t *testing.T) {
	p := newMachineProvider(docker_executor.NewProvider())
	for name, state := range map[string]machineState{
		"idle-1":   machineStateIdle,
		"idle-2":   machineStateIdle,
		"used":     machineStateUsed,
		"creating": machineStateCreating,
		"acquired": machineStateAcquired,
		"removing": machineStateRemoving,
		"failed":   machineStateIdle,
		"pending":  machineStateIdle,
	} {
		p.details[name] = &machineDetails{Name: name, State: state}
	}

	p.details["pending"].pendingUse = true
	d := &shutdownDrain{failed: map[string]struct{}{"failed": {}}}

	names := []string{}
	for _, d := range p.collectIdleMachines(d) {
		names = append(names, d.Name)
	}
	assert.ElementsMatch(t, []string{"idle-1", "idle-2"}, names)
}

func TestMachineProvider_Shutdown_WithoutBeginShutdownSkipsIdlePasses(t *testing.T) {
	for range 50 {
		machine := newMockMachine(t)
		p := newMachineProvider(docker_executor.NewProvider())
		p.machine = machine
		p.details["idle"] = &machineDetails{Name: "idle", State: machineStateIdle, Created: time.Now()}
		machine.EXPECT().Exist(mock.Anything, "idle").Return(true).Once()
		machine.EXPECT().ForceRemove(mock.Anything, "idle").Return(nil).Once()

		hook, removeHook := test.NewHook()

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		p.Shutdown(ctx, drainConfig(1, 0))
		cancel()

		for _, e := range hook.AllEntries() {
			assert.NotEqual(t, "Draining idle machines", e.Message)
		}
		removeHook()
	}
}

type inspectHookMachine struct {
	*testMachine
	once      sync.Once
	onInspect func()
}

func (m *inspectHookMachine) Inspect(name string) (docker.MachineInfo, error) {
	m.once.Do(m.onInspect)
	return m.testMachine.Inspect(name)
}

func TestMachineProvider_EarlyDrain_DoesNotClaimMachineBeingCreated(t *testing.T) {
	for name, pendingUse := range map[string]bool{
		"for Use":  true,
		"for idle": false,
	} {
		t.Run(name, func(t *testing.T) {
			p, tm := testMachineProvider()
			d := &shutdownDrain{
				config: drainConfig(1, 0).Machine.ShutdownDrain,
				logger: logrus.WithField("test", t.Name()),
				failed: map[string]struct{}{},
			}
			p.machine = &inspectHookMachine{
				testMachine: tm,
				onInspect: func() {
					p.drainPass(t.Context(), d, p.collectIdleMachines(d), true)
				},
			}

			var details *machineDetails
			var errCh chan error
			require.NotPanics(t, func() {
				details, errCh = p.create(machineDefaultConfig, machineStateIdle, pendingUse, nil)
			})
			require.NotNil(t, details)
			require.NoError(t, <-errCh)

			assert.Equal(t, 0, d.removed, "an idle pass drained a machine that was being created")
			_, tracked := trackedState(p, details.Name)
			assert.True(t, tracked, "created machine should still be tracked")
		})
	}
}

func TestMachineProvider_EarlyDrain_FailedMachineNotHandedToJobs(t *testing.T) {
	withShortDrainPoll(t)

	p, _ := testMachineProvider("remove-fail")
	p.details["remove-fail"] = &machineDetails{Name: "remove-fail", State: machineStateIdle, Created: time.Now()}

	p.BeginShutdown(drainConfig(1, 0))
	t.Cleanup(func() {
		p.drain.cancel()
		<-p.drain.done
	})

	require.Eventually(t, func() bool {
		return p.drain.hasFailed("remove-fail")
	}, 5*time.Second, time.Millisecond)

	assert.Nil(t, p.findFreeMachine(true, "remove-fail"),
		"a machine whose drain removal failed must not be handed to a job")
}

func TestMachineProvider_EarlyDrain_TimeoutDuringBackoffCountsAsFailed(t *testing.T) {
	machine := newMockMachine(t)
	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine
	p.details["m"] = &machineDetails{Name: "m", State: machineStateIdle, Created: time.Now()}

	hook, removeHook := test.NewHook()
	defer removeHook()

	firstAttempt := make(chan struct{})
	machine.EXPECT().Exist(mock.Anything, "m").Return(true)
	machine.EXPECT().ForceRemove(mock.Anything, "m").RunAndReturn(func(context.Context, string) error {
		close(firstAttempt)
		return assert.AnError
	}).Once()

	config := drainConfig(1, 3)
	config.Machine.ShutdownDrain.RetryBackoff = time.Hour
	p.BeginShutdown(config)
	<-firstAttempt

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	p.Shutdown(ctx, config)

	for _, e := range hook.AllEntries() {
		if e.Message == "Machine pool drain completed" {
			assert.Equal(t, 1, e.Data["failed"], "a machine cancelled mid-retry was not removed")
			return
		}
	}
	t.Error("missing drain summary")
}

type shutdownDuringAcquireMachine struct {
	*testMachine
	p *machineProvider
}

func (m *shutdownDuringAcquireMachine) List() ([]string, error) {
	m.p.BeginShutdown(drainConfig(1, 0))
	return m.testMachine.List()
}

func TestMachineProvider_Acquire_DoesNotCreateMachinesOnceShutdownBegins(t *testing.T) {
	withShortDrainPoll(t)

	p, tm := testMachineProvider()
	p.machine = &shutdownDuringAcquireMachine{testMachine: tm, p: p}
	t.Cleanup(func() {
		p.drain.cancel()
		<-p.drain.done
	})

	config := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Machine: &common.DockerMachine{MachineName: "%s", IdleTime: 5, IdleCount: 3},
		},
	}
	_, _ = p.Acquire(config)

	p.lock.RLock()
	defer p.lock.RUnlock()
	assert.Empty(t, p.details, "Acquire created idle machines after the shutdown began")
}

func TestMachineProvider_Shutdown_FinishesEarlyDrainWhateverTheConfigSays(t *testing.T) {
	machine := newMockMachine(t)
	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = machine
	p.details["used"] = &machineDetails{Name: "used", State: machineStateUsed, Created: time.Now()}
	machine.EXPECT().Exist(mock.Anything, "used").Return(true).Once()
	machine.EXPECT().ForceRemove(mock.Anything, "used").Return(nil).Once()

	p.BeginShutdown(drainConfig(1, 0))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	p.Shutdown(ctx, nil)

	assert.Empty(t, p.details, "the running drain got its final pass")
}
