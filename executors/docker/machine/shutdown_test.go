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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/log/test"
)

func TestMachineProvider_Shutdown_NoDrainConfig(t *testing.T) {
	p := newMachineProvider()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	runWithLogCheck(t, "No shutdown drain config, skipping machine pool drain", func() {
		p.Shutdown(ctx, nil)
	})
}

func TestMachineProvider_Shutdown_DrainDisabled(t *testing.T) {
	p := newMachineProvider()

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
	p := newMachineProvider()

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
	machine := docker.NewMockMachine(t)

	p := newMachineProvider()
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
	machine := docker.NewMockMachine(t)

	p := newMachineProvider()
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
	machine := docker.NewMockMachine(t)

	p := newMachineProvider()
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

	machine := docker.NewMockMachine(t)

	p := newMachineProvider()
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
	machine := docker.NewMockMachine(t)

	p := newMachineProvider()
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
	p.details["creating-machine"] = &machineDetails{
		Name:    "creating-machine",
		State:   machineStateCreating,
		Created: time.Now(),
	}

	// All machines should be removed regardless of state
	machine.EXPECT().Exist(mock.Anything, mock.Anything).Return(true).Times(3)
	machine.EXPECT().ForceRemove(mock.Anything, mock.Anything).Return(nil).Times(3)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	p.Shutdown(ctx, config)

	assert.Empty(t, p.details)
}

func TestMachineProvider_CollectAllMachines(t *testing.T) {
	p := newMachineProvider()

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
