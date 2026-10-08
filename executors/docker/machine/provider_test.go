//go:build !integration

package machine

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	logrus_test "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	docker_executor "gitlab.com/gitlab-org/gitlab-runner/executors/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/log/test"
)

var (
	_ common.ExecutorProvider              = (*machineProvider)(nil)
	_ common.ManagedExecutorProvider       = (*machineProvider)(nil)
	_ common.ShutdownAwareExecutorProvider = (*machineProvider)(nil)
	_ common.WarmupReporter                = (*machineProvider)(nil)
)

var machineDefaultConfig = &common.RunnerConfig{
	RunnerSettings: common.RunnerSettings{
		Machine: &common.DockerMachine{
			MachineName: "%s",
			IdleTime:    5,
		},
	},
}

var machineCreateFail = &common.RunnerConfig{
	RunnerSettings: common.RunnerSettings{
		Machine: &common.DockerMachine{
			MachineName: "create-fail-%s",
			IdleTime:    5,
		},
	},
}

var machineProvisionFail = &common.RunnerConfig{
	RunnerSettings: common.RunnerSettings{
		Machine: &common.DockerMachine{
			MachineName: "provision-fail-%s",
			IdleTime:    5,
		},
	},
}

var machineSecondFail = &common.RunnerConfig{
	RunnerSettings: common.RunnerSettings{
		Machine: &common.DockerMachine{
			MachineName: "second-fail-%s",
			IdleTime:    5,
		},
	},
}

var machineNoConnect = &common.RunnerConfig{
	RunnerSettings: common.RunnerSettings{
		Machine: &common.DockerMachine{
			MachineName: "no-connect-%s",
			IdleTime:    5,
		},
	},
}

func createMachineConfigWithLimit(t *testing.T, idleCount int, idleTime int, limit int) *common.RunnerConfig {
	conf := &common.RunnerConfig{
		Limit: limit,
		RunnerSettings: common.RunnerSettings{
			Machine: &common.DockerMachine{
				MachineName: "test-machine-%s",
				IdleCount:   idleCount,
				IdleTime:    idleTime,
			},
		},
	}
	err := conf.RunnerSettings.Machine.CompilePeriods()
	require.NoError(t, err)
	return conf
}

func createMachineConfig(t *testing.T, idleCount int, idleTime int) *common.RunnerConfig {
	return createMachineConfigWithLimit(t, idleCount, idleTime, 0)
}

type testMachine struct {
	machines []string
	second   bool

	Created chan bool
	Removed chan bool
	Stopped chan bool

	forceRemoveCalls int

	mutex sync.Mutex
}

func (m *testMachine) Create(ctx context.Context, driver, name string, _ docker.MachineOutput, opts ...string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if strings.Contains(name, "second-fail") {
		if !m.second {
			m.second = true
			return errors.New("failed to create")
		}
	} else if strings.Contains(name, "create-fail") || strings.Contains(name, "provision-fail") {
		return errors.New("failed to create")
	}
	m.machines = append(m.machines, name)
	m.Created <- true

	return nil
}

func (m *testMachine) UpdateLabels(ctx context.Context, name string, labels map[string]string) error {
	return nil
}

func (m *testMachine) Provision(ctx context.Context, name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if strings.Contains(name, "provision-fail") || strings.Contains(name, "second-fail") {
		return errors.New("failed to provision")
	}
	m.machines = append(m.machines, name)
	return nil
}

func (m *testMachine) Stop(ctx context.Context, name string) error {
	m.Stopped <- true

	return nil
}

func (m *testMachine) Remove(ctx context.Context, name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if name == "remove-fail" {
		return errors.New("failed to remove")
	}
	var machines []string
	for _, machine := range m.machines {
		if machine != name {
			machines = append(machines, machine)
		}
	}
	m.machines = machines
	m.Removed <- true

	return nil
}

func (m *testMachine) ForceRemove(ctx context.Context, name string) error {
	m.mutex.Lock()
	m.forceRemoveCalls++
	m.mutex.Unlock()
	return m.Remove(ctx, name)
}

func (m *testMachine) Exist(ctx context.Context, name string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return slices.Contains(m.machines, name)
}

func (m *testMachine) List() (machines []string, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// A copy, as docker-machine returns a new list on every call. Callers append to the
	// result, which would otherwise write into the backing array that Create appends to.
	return slices.Clone(m.machines), nil
}

func (m *testMachine) CanConnect(ctx context.Context, name string, skipCache bool) bool {
	return !strings.Contains(name, "no-can-connect")
}

func (m *testMachine) Credentials(ctx context.Context, name string) (dc docker.Credentials, err error) {
	if strings.Contains(name, "no-connect") {
		err = errors.New("failed to connect")
	}
	return
}

// Inspect returns an empty MachineInfo by default. Tests that need to
// exercise the target-label discovery path should swap in a mock that
// returns a populated MachineInfo.
func (m *testMachine) Inspect(name string) (docker.MachineInfo, error) {
	return docker.MachineInfo{}, nil
}

func countIdleMachines(p *machineProvider) (count int) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for _, details := range p.details {
		details.Lock()
		if details.State == machineStateIdle {
			count++
		}
		details.Unlock()
	}
	return
}

func assertIdleMachines(t *testing.T, p *machineProvider, expected int, msgAndArgs ...any) {
	var idle int
	for range 10 {
		idle = countIdleMachines(p)

		if expected == idle {
			return
		}

		time.Sleep(50 * time.Microsecond)
	}

	result := fmt.Sprintf("should have %d idle, but has %d", expected, idle)
	assert.Fail(t, result, msgAndArgs...)
}

func countTotalMachines(p *machineProvider) (count int) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for _, details := range p.details {
		if details.State != machineStateRemoving {
			count++
		}
	}
	return
}

func assertTotalMachines(t *testing.T, p *machineProvider, expected int, msgAndArgs ...any) {
	var total int
	for range 10 {
		total = countTotalMachines(p)

		if expected == total {
			return
		}

		time.Sleep(50 * time.Microsecond)
	}

	result := fmt.Sprintf("should have %d total, but has %d", expected, total)
	assert.Fail(t, result, msgAndArgs...)
}

func testMachineProvider(machine ...string) (*machineProvider, *testMachine) {
	t := &testMachine{
		machines: machine,
		Created:  make(chan bool, 10),
		Removed:  make(chan bool, 10),
		Stopped:  make(chan bool, 10),
	}
	p := newMachineProvider(docker_executor.NewProvider())
	p.machine = t
	return p, t
}

// newMockMachine wraps docker.NewMockMachine with a default Inspect
// expectation pre-registered so individual tests don't have to know about
// the post-Create Inspect call in createWithGrowthCapacity. Tests that
// care about the Inspect return value can layer their own expectation on
// top with a more specific matcher.
func newMockMachine(t *testing.T) *docker.MockMachine {
	t.Helper()
	m := docker.NewMockMachine(t)
	m.On("Inspect", mock.Anything).Return(docker.MachineInfo{}, nil).Maybe()
	return m
}

func TestMachineDetails(t *testing.T) {
	p, _ := testMachineProvider()
	m1 := p.machineDetails("test", false)
	assert.NotNil(t, m1, "returns a new machine")
	assert.Equal(t, machineStateIdle, m1.State)
	assert.Equal(t, 1, m1.UsedCount)

	m2 := p.machineDetails("test", false)
	assert.Equal(t, m1, m2, "returns the same machine")

	m3 := p.machineDetails("test", true)
	assert.Equal(t, machineStateAcquired, m3.State, "acquires machine")

	m4 := p.machineDetails("test", true)
	assert.Nil(t, m4, "fails to return re-acquired machine")

	m5 := p.machineDetails("test", false)
	assert.Equal(t, m1, m5, "returns acquired machine")
	assert.Equal(t, machineStateAcquired, m5.State, "machine is acquired")
}

func TestMachineFindFree(t *testing.T) {
	p, tm := testMachineProvider("no-can-connect")
	d1 := p.findFreeMachine(false)
	assert.Nil(t, d1, "no machines, return nil")

	d2 := p.findFreeMachine(false, "machine1")
	assert.NotNil(t, d2, "acquire one machine")

	d3 := p.findFreeMachine(false, "machine1")
	assert.Nil(t, d3, "fail to acquire that machine")

	d4 := p.findFreeMachine(false, "machine1", "machine2")
	assert.NotNil(t, d4, "acquire a new machine")
	assert.NotEqual(t, d2, d4, "and it's a different machine")

	assert.Len(t, tm.machines, 1, "has one machine")
	d5 := p.findFreeMachine(false, "machine1", "no-can-connect")
	assert.Nil(t, d5, "fails to acquire machine to which he can't connect")
}

func TestMachineCreationAndRemoval(t *testing.T) {
	provisionRetryInterval = 0

	p, _ := testMachineProvider()
	d, errCh := p.create(machineDefaultConfig, machineStateUsed, false, nil)
	assert.NotNil(t, d)
	assert.NoError(t, <-errCh)
	assert.Equal(t, machineStateUsed, d.State)
	assert.Equal(t, 0, d.UsedCount)
	assert.NotNil(t, p.details[d.Name])

	err := p.remove(d.Name)
	assert.NoError(t, err)
	assert.Equal(t, machineStateRemoving, d.State)

	d2, errCh := p.create(machineProvisionFail, machineStateUsed, false, nil)
	assert.NotNil(t, d2)
	assert.Error(t, <-errCh, "Fails, because it fails to provision machine")
	assert.Equal(t, machineStateRemoving, d2.State)

	d3, errCh := p.create(machineCreateFail, machineStateUsed, false, nil)
	assert.NotNil(t, d3)
	assert.Error(t, <-errCh)
	assert.Equal(t, machineStateRemoving, d3.State)
}

func TestMachineRemovalWithoutStop(t *testing.T) {
	provisionRetryInterval = 0

	tests := map[string]struct {
		removeWithoutStop bool
		expectStops       int
	}{
		"default stops before removing":      {removeWithoutStop: false, expectStops: 1},
		"RemoveWithoutStop removes directly": {removeWithoutStop: true, expectStops: 0},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			config := createMachineConfig(t, 0, 0)
			config.Machine.RemoveWithoutStop = tt.removeWithoutStop

			p, m := testMachineProvider()
			d, errCh := p.create(config, machineStateIdle, false, nil)
			require.NoError(t, <-errCh)

			require.NoError(t, p.remove(d.Name))
			<-d.inFlight
			<-m.Removed

			assert.Len(t, m.Stopped, tt.expectStops)
		})
	}
}

func TestMachineCreationRegistersHeartbeat(t *testing.T) {
	provisionRetryInterval = 0

	config := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{Token: "heartbeat-runner"},
		RunnerSettings: common.RunnerSettings{
			Machine: &common.DockerMachine{
				MachineName:       "%s",
				IdleTime:          5,
				HeartbeatInterval: 30 * time.Minute,
			},
		},
	}

	p, _ := testMachineProvider()
	d, errCh := p.create(config, machineStateIdle, false, nil)
	require.NoError(t, <-errCh)

	h := p.heartbeaterFor(config)
	h.lock.Lock()
	defer h.lock.Unlock()
	assert.Contains(t, h.lastBeat, d.Name, "created machine is tracked without waiting for Acquire")
}

func TestMachineRemovalForgetsHeartbeat(t *testing.T) {
	provisionRetryInterval = 0
	origInterval := removeRetryInterval
	removeRetryInterval = time.Millisecond
	defer func() { removeRetryInterval = origInterval }()

	config := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{Token: "heartbeat-runner"},
		RunnerSettings: common.RunnerSettings{
			Machine: &common.DockerMachine{
				MachineName:       "%s",
				IdleTime:          5,
				HeartbeatInterval: 30 * time.Minute,
			},
		},
	}

	tracked := func(p *machineProvider, name string) bool {
		h := p.heartbeaterFor(config)
		h.lock.Lock()
		defer h.lock.Unlock()
		_, ok := h.lastBeat[name]
		return ok
	}

	t.Run("removed", func(t *testing.T) {
		p, _ := testMachineProvider()
		d, errCh := p.create(config, machineStateIdle, false, nil)
		require.NoError(t, <-errCh)
		require.True(t, tracked(p, d.Name))

		require.NoError(t, p.remove(d.Name))
		<-d.inFlight
		assert.False(t, tracked(p, d.Name))
	})

	t.Run("gave up", func(t *testing.T) {
		p, _ := testMachineProvider("remove-fail")
		p.heartbeaterFor(config).beat("remove-fail", 30*time.Minute)
		require.True(t, tracked(p, "remove-fail"))

		d := &machineDetails{Name: "remove-fail", State: machineStateRemoving, maxRemovalAttempts: 2}
		p.finalizeRemoval(d)
		assert.False(t, tracked(p, "remove-fail"), "a VM the manager gave up on must not keep looking manager-owned")
	})
}

func TestMachineUse(t *testing.T) {
	provisionRetryInterval = 0

	p, _ := testMachineProvider("machine1")

	d1, err := p.useMachine(machineDefaultConfig)
	assert.NotNil(t, d1)
	assert.NoError(t, err)
	assert.Equal(t, machineStateAcquired, d1.State)
	assert.Equal(t, "machine1", d1.Name, "finds a free machine1")

	d2, err := p.useMachine(machineDefaultConfig)
	assert.NotNil(t, d2)
	assert.NoError(t, err)
	assert.Equal(t, machineStateAcquired, d2.State)
	assert.NotEqual(t, "machine1", d2.Name, "creates a new machine")

	_, err = p.useMachine(machineProvisionFail)
	assert.Error(t, err, "fails to create a new machine")
}

func TestMachineReuse(t *testing.T) {
	// Create a machine with an idle state. Then try to create additional ones.
	// while the creation of the all subsequent ones are blocked use the first one,
	// making sure that the whole process works
	machineGrowthConfig := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Machine: &common.DockerMachine{
				MachineName:   "growth-temp-%s",
				MaxGrowthRate: 1,
				IdleTime:      5,
			},
		},
	}

	p := newMachineProvider(docker_executor.NewProvider())

	machineMock := newMockMachine(t)
	p.machine = machineMock

	var blockCreatingMachineWg sync.WaitGroup
	blockCreatingMachineWg.Add(1)

	var createdMachineDetails *machineDetails

	machineMock.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	machineMock.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			// Free the previously created machine after the useMachine call has already happened.
			// The useMachine call tries to create a new machine at first because a free one doesn't exist
			// however it's blocked by blockCreatingMachineWg, thus it's waiting for us to release a new one.
			// If useMachine never returns because it can't find a machine then that's a bug.
			time.AfterFunc(time.Second, func() {
				p.Release(machineGrowthConfig, createdMachineDetails)
			})

			blockCreatingMachineWg.Wait()
		}).
		Return(nil).
		Once()

	machineMock.On("CanConnect", mock.Anything, mock.Anything, mock.Anything).
		Return(true).
		Once()

	createdMachineDetails, errCh := p.create(machineGrowthConfig, machineStateUsed, false, nil)
	require.NotNil(t, createdMachineDetails)
	require.NoError(t, <-errCh)

	machineMock.On("List").Return([]string{createdMachineDetails.Name}, nil)

	usedMachineDetails, err := p.useMachine(machineGrowthConfig)
	require.NoError(t, err)
	require.Equal(t, createdMachineDetails, usedMachineDetails)
}

func TestMachineReuseWithContention(t *testing.T) {
	// Create machines while trying to reuse them with a contention.
	// Make sure that there are no deadlocks, data races and that machines
	// are provided to the caller.
	machineGrowthConfig := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Machine: &common.DockerMachine{
				MachineName:   "growth-temp-%s",
				MaxGrowthRate: 10,
				IdleTime:      5,
			},
		},
	}

	p := newMachineProvider(docker_executor.NewProvider())

	machineMock := newMockMachine(t)
	p.machine = machineMock

	var listLock sync.Mutex
	list := make([]string, 0)

	listCall := machineMock.On("List").Return([]string{}, nil)
	machineMock.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			name := args.String(2)

			listLock.Lock()
			list = append(list, name)
			listCopy := make([]string, len(list))
			copy(listCopy, list)
			listCall.Return(listCopy, nil).Maybe()
			listLock.Unlock()
		}).
		Return(nil)

	machineMock.On("CanConnect", mock.Anything, mock.Anything, mock.Anything).
		Return(true).
		Maybe()

	const N = 500
	var wg sync.WaitGroup
	wg.Add(N)

	startCh := make(chan struct{})
	for range N {
		go func() {
			defer wg.Done()
			<-startCh

			usedMachineDetails, err := p.useMachine(machineGrowthConfig)
			require.NoError(t, err)
			require.NotNil(t, usedMachineDetails)
			p.Release(machineGrowthConfig, usedMachineDetails)
		}()
	}

	close(startCh)
	wg.Wait()

	listLock.Lock()
	defer listLock.Unlock()
	assert.NotEmpty(t, list)
}

func TestMachineTestRetry(t *testing.T) {
	provisionRetryInterval = 0

	p, _ := testMachineProvider()
	_, err := p.useMachine(machineSecondFail)
	assert.Error(t, err, "fails to create a new machine")

	p, _ = testMachineProvider()
	d1, err := p.retryUseMachine(machineSecondFail)
	assert.NoError(t, err, "after replying the same test scenario and using retry it succeeds")
	assert.Equal(t, machineStateAcquired, d1.State)
}

func TestMachineAcquireGrowthCapacity(t *testing.T) {
	p, _ := testMachineProvider()

	tests := map[string]struct {
		maxGrowthCapacity int
		concurrency       int

		expectedMaxConcurrentCalls int
	}{
		"growth capacity 3 concurrency 1": {
			maxGrowthCapacity: 3,
			concurrency:       1,

			expectedMaxConcurrentCalls: 1,
		},
		"growth capacity 3 concurrency 15": {
			maxGrowthCapacity: 3,
			concurrency:       15,

			expectedMaxConcurrentCalls: 3,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			machineMock := newMockMachine(t)
			p.machine = machineMock

			var wg sync.WaitGroup
			var concurrentCalls, maxConcurrentCalls int32
			var maxConcurrentCallsLock sync.Mutex

			machineMock.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(mock.Arguments) {
					defer atomic.AddInt32(&concurrentCalls, -1)
					cc := atomic.AddInt32(&concurrentCalls, 1)

					maxConcurrentCallsLock.Lock()
					if cc > maxConcurrentCalls {
						maxConcurrentCalls = cc
					}
					maxConcurrentCallsLock.Unlock()

					// simulate a network call in order to allow some goroutines to get in
					// line, otherwise we will never get past 1 concurrent call
					time.Sleep(300 * time.Millisecond)
				}).
				Return(nil)

			signal := make(chan struct{})
			for i := 0; i < tt.concurrency; i++ {
				wg.Go(func() {
					<-signal
					_, errCh := p.create(&common.RunnerConfig{
						Limit:             tt.concurrency,
						RunnerCredentials: common.RunnerCredentials{},
						RunnerSettings: common.RunnerSettings{
							Machine: &common.DockerMachine{
								IdleCount:     tt.concurrency,
								MachineName:   "test",
								MaxGrowthRate: tt.maxGrowthCapacity,
							},
						},
					}, machineStateIdle, false, nil)

					<-errCh
				})
			}

			// wait for all goroutines to fire up and line up for the signal in order to have a fair race
			// to the Create method
			close(signal)
			wg.Wait()

			assert.Equal(t, tt.expectedMaxConcurrentCalls, int(maxConcurrentCalls))
		})
	}
}

func TestMachineAcquireAndRelease(t *testing.T) {
	p, _ := testMachineProvider("test-machine")

	d1, err := p.Acquire(machineDefaultConfig)
	assert.NoError(t, err)
	assert.NotNil(t, d1, "acquires machine")

	d2, _ := p.Acquire(machineDefaultConfig)
	assert.Nil(t, d2, "fails to acquire a machine")

	p.Release(machineDefaultConfig, d1)

	d3, err := p.Acquire(machineDefaultConfig)
	assert.NoError(t, err)
	assert.Equal(t, d1, d3, "acquires released machine")
}

func TestMachineOnDemandMode(t *testing.T) {
	p, _ := testMachineProvider()

	config := createMachineConfig(t, 0, 1)
	_, err := p.Acquire(config)
	assert.NoError(t, err)
}

func TestMachineOnDemandModeWithLimit(t *testing.T) {
	p, _ := testMachineProvider()

	limit := 2
	config := createMachineConfigWithLimit(t, 0, 1, limit)

	for range limit {
		data, err := p.Acquire(config)
		assert.NoError(t, err)
		assert.Nil(t, data)

		_, nd, err := p.Use(config, data)
		assert.NoError(t, err)
		assert.NotNil(t, nd)
	}

	data, err := p.Acquire(config)
	assert.Error(t, err, "it should fail with message that currently there's no free machines")
	assert.Nil(t, data)
}

func TestMachinePreCreateMode(t *testing.T) {
	p, m := testMachineProvider()

	config := createMachineConfig(t, 1, 5)
	d, err := p.Acquire(config)
	assert.Error(t, err, "it should fail with message that currently there's no free machines")
	assert.Nil(t, d)

	<-m.Created
	assertIdleMachines(t, p, 1, "it should contain exactly one machine")

	d, err = p.Acquire(config)
	assert.NoError(t, err, "it should be ready to process builds")
	assertIdleMachines(t, p, 0, "it should acquire the free node")
	p.Release(config, d)
	assertIdleMachines(t, p, 1, "after releasing it should have one free node")

	config = createMachineConfig(t, 2, 5)
	d, err = p.Acquire(config)
	assert.NoError(t, err)
	p.Release(config, d)

	<-m.Created
	assertIdleMachines(t, p, 2, "it should start creating a second machine")

	config = createMachineConfig(t, 1, 0)
	config.Limit = 1
	d, err = p.Acquire(config)
	assert.NoError(t, err)
	p.Release(config, d)

	<-m.Stopped
	<-m.Removed
	assertIdleMachines(t, p, 1, "it should downscale to single machine")

	d, err = p.Acquire(config)
	assert.NoError(t, err, "we should acquire single machine")

	_, err = p.Acquire(config)
	assert.Error(t, err, "it should fail with message that currently there's no free machines")
	p.Release(config, d)
	assertIdleMachines(t, p, 1, "it should leave one idle")
}

func TestMachineLimitMax(t *testing.T) {
	p, _ := testMachineProvider()

	config := createMachineConfig(t, 10, 5)
	config.Limit = 5

	d, err := p.Acquire(config)
	assert.Error(t, err, "it should fail with message that currently there's no free machines")
	assert.Nil(t, d)
	assertIdleMachines(t, p, 5, "it should contain exactly a maximum of 5 nodes")

	config.Limit = 8
	d, err = p.Acquire(config)
	assert.NoError(t, err)
	p.Release(config, d)
	assertIdleMachines(t, p, 8, "it should upscale to 8 nodes")

	config.Limit = 2
	d, err = p.Acquire(config)
	assert.NoError(t, err)
	p.Release(config, d)
	assertIdleMachines(t, p, 2, "it should downscale to 2 nodes")
}

func TestMachineMaxBuildsForExistingMachines(t *testing.T) {
	provisionRetryInterval = 0

	p, _ := testMachineProvider("remove-fail")
	config := createMachineConfig(t, 1, 5)
	config.Machine.MaxBuilds = 1
	d, err := p.Acquire(config)
	assert.Error(t, err)
	assert.Nil(t, d)
}

func TestMachineMaxBuilds(t *testing.T) {
	config := createMachineConfig(t, 1, 5)
	p, _ := testMachineProvider(newMachineName(config))
	config.Machine.MaxBuilds = 2 // by default we set it to 1
	d, err := p.Acquire(config)
	assert.NoError(t, err)
	assert.NotNil(t, d)

	_, nd, err := p.Use(config, d)
	assert.NoError(t, err)
	assert.Nil(t, nd, "we passed the data, we should not get the data now")

	p.Release(config, d)

	dd := d.(*machineDetails)
	assert.Equal(t, machineStateRemoving, dd.State, "the machine should be removed due to too many builds")
	assert.Equal(t, "Too many builds", dd.Reason, "the machine should be removed due to too many builds")
}

func TestMachineIdleLimits(t *testing.T) {
	p, _ := testMachineProvider()

	config := createMachineConfig(t, 2, 1)
	d, errCh := p.create(config, machineStateIdle, false, nil)
	assert.NoError(t, <-errCh, "machine creation should not fail")

	time.Sleep(time.Second)

	d2, err := p.Acquire(config)
	p.Release(config, d2)
	assert.NoError(t, err)
	assert.Equal(t, machineStateIdle, d.State, "machine should not be removed, because is still in idle time")

	config = createMachineConfig(t, 2, 0)
	d3, err := p.Acquire(config)
	p.Release(config, d3)
	assert.NoError(t, err)
	assert.Equal(t, machineStateIdle, d.State, "machine should not be removed, because no more than two idle")

	config = createMachineConfig(t, 0, 0)
	d4, err := p.Acquire(config)
	p.Release(config, d4)
	assert.NoError(t, err)

	assert.Equal(t, machineStateRemoving, d.State, "machine should be removed, because there are no idle")
	assert.Equal(t, "too many idle machines", d.Reason)
}

func TestMachineUseOnDemand(t *testing.T) {
	provisionRetryInterval = 0

	p, _ := testMachineProvider()

	_, nd, err := p.Use(machineDefaultConfig, nil)
	assert.NoError(t, err, "it create a new machine")
	assert.NotNil(t, nd)
	assertTotalMachines(t, p, 1, "it creates one machine")

	_, nd2, err := p.Use(machineDefaultConfig, nil)
	assert.NoError(t, err, "it create a new machine")
	assert.NotNil(t, nd2)
	assertTotalMachines(t, p, 2, "it creates two machines")

	_, _, err = p.Use(machineProvisionFail, nil)
	assert.Error(t, err, "fail to create a new machine")
	assertTotalMachines(t, p, 2, "it fails to create a third machine")

	_, _, err = p.Use(machineNoConnect, nil)
	assert.Error(t, err, "fail to create a new machine on connect")
	assertTotalMachines(t, p, 3, "it fails on no-connect, but we leave the machine created")
}

func TestMachineReleaseIfInvalidDataArePassed(t *testing.T) {
	p, _ := testMachineProvider()

	_, nd, err := p.Use(machineDefaultConfig, nil)
	assert.NoError(t, err, "it create a new machine")
	assert.NotNil(t, nd)
	assertTotalMachines(t, p, 1, "it creates one machine")

	p.Release(nil, nd)
}

func TestMachineCreationIfFailedToConnect(t *testing.T) {
	p, _ := testMachineProvider()

	_, nd, err := p.Use(machineNoConnect, nil)
	assert.Error(t, err, "it create a new machine")
	assert.Nil(t, nd)
}

func vanishedCount(p *machineProvider) float64 {
	return testutil.ToFloat64(p.totalActions.WithLabelValues(actionLabels("vanished", targetLabels{})...))
}

func TestAcquirePrunesVanishedIdleMachine(t *testing.T) {
	p, _ := testMachineProvider()
	config := createMachineConfig(t, 0, 3600)
	name := newMachineName(config)
	p.machineDetails(name, false)

	for range vanishedAfterMisses - 1 {
		_, err := p.Acquire(config)
		require.NoError(t, err)
		assert.Equal(t, 1, idleCount(p.collectDetails()))
	}

	_, err := p.Acquire(config)
	require.NoError(t, err)
	assert.Equal(t, 0, idleCount(p.collectDetails()))
	assert.NotContains(t, p.details, name)
	assert.Equal(t, float64(1), vanishedCount(p))
}

func TestAcquireKeepsIdleMachineListedAgain(t *testing.T) {
	p, m := testMachineProvider()
	config := createMachineConfig(t, 0, 3600)
	name := newMachineName(config)
	d := p.machineDetails(name, false)
	d.LastSeen = time.Now().Add(-time.Hour)

	for range vanishedAfterMisses - 1 {
		_, err := p.Acquire(config)
		require.NoError(t, err)
	}
	assert.Equal(t, vanishedAfterMisses-1, d.missedListings)

	m.machines = []string{name}
	data, err := p.Acquire(config)
	require.NoError(t, err)
	assert.Equal(t, d, data)
	assert.Equal(t, 0, d.missedListings)
	assert.WithinDuration(t, time.Now(), d.LastSeen, time.Minute)
	p.Release(config, data)

	m.machines = nil
	for range vanishedAfterMisses - 1 {
		_, err := p.Acquire(config)
		require.NoError(t, err)
	}
	assert.Contains(t, p.details, name)
	assert.Equal(t, float64(0), vanishedCount(p))
}

func TestAcquireOnlyPrunesIdleMachines(t *testing.T) {
	for _, state := range []machineState{machineStateCreating, machineStateAcquired, machineStateUsed, machineStateRemoving} {
		t.Run(state.String(), func(t *testing.T) {
			p, _ := testMachineProvider()
			config := createMachineConfig(t, 0, 3600)
			name := newMachineName(config)
			p.details[name] = &machineDetails{Name: name, State: state}

			for range vanishedAfterMisses + 1 {
				_, err := p.Acquire(config)
				require.NoError(t, err)
			}
			require.Contains(t, p.details, name)
			assert.Equal(t, 0, p.details[name].missedListings)
		})
	}
}

func TestAcquireDoesNotPruneOtherRunnersMachines(t *testing.T) {
	p, _ := testMachineProvider()
	config := createMachineConfig(t, 0, 3600)
	other := createMachineConfig(t, 0, 3600)
	other.Machine.MachineName = "other-%s"
	p.machineDetails(newMachineName(other), false)

	for range vanishedAfterMisses + 1 {
		_, err := p.Acquire(config)
		require.NoError(t, err)
	}
	assert.Equal(t, 1, idleCount(p.collectDetails()))
	assert.Equal(t, float64(0), vanishedCount(p))

	for range vanishedAfterMisses {
		_, err := p.Acquire(other)
		require.NoError(t, err)
	}
	assert.Equal(t, 0, idleCount(p.collectDetails()))
	assert.Equal(t, float64(1), vanishedCount(p))
}

func TestIntermediateMachineList(t *testing.T) {
	p, _ := testMachineProvider()
	p.details = machinesDetails{
		"machine1": &machineDetails{
			Name:  "machine1",
			State: machineStateIdle,
		},
		"machine2": &machineDetails{
			Name:  "machine2",
			State: machineStateCreating,
		},
		"machine3": &machineDetails{
			Name:  "machine3",
			State: machineStateCreating,
		},
	}

	expectedIntermediateMachines := []string{"machine3"}

	intermediateMachine := p.intermediateMachineList([]string{"machine1", "machine2"})
	assert.Equal(t, expectedIntermediateMachines, intermediateMachine)
}

func TestMachineOptionsWithName(t *testing.T) {
	provisionRetryInterval = 0

	p := newMachineProvider(docker_executor.NewProvider())
	machineMock := newMockMachine(t)
	p.machine = machineMock

	config := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Machine: &common.DockerMachine{
				MachineName:            "test-machine-%s",
				IdleTime:               5,
				MachineOptions:         []string{"--option1=value1", "--option2=value2"},
				MachineOptionsWithName: []string{"--name-option=%s", "--another-name-option=%s-suffix"},
			},
		},
	}

	var capturedOpts []string
	machineMock.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			for i := 4; i < len(args); i++ {
				capturedOpts = append(capturedOpts, args.String(i))
			}
		}).
		Return(nil).
		Once()

	d, errCh := p.create(config, machineStateIdle, false, nil)
	require.NotNil(t, d)
	require.NoError(t, <-errCh)

	expectedOpts := []string{
		"--option1=value1",
		"--option2=value2",
		fmt.Sprintf("--name-option=%s", d.Name),
		fmt.Sprintf("--another-name-option=%s-suffix", d.Name),
	}
	assert.Equal(t, expectedOpts, capturedOpts)
}

func TestMachineOptionsExpandEnv(t *testing.T) {
	provisionRetryInterval = 0
	t.Setenv("TEST_HOSTNAME", "gitlab-runner-p-s-1-86bfc8b89b-hkh9c")
	t.Setenv("TEST_NOTE", "100%")
	t.Setenv("GITLAB_RUNNER_MACHINE_NAME", "from-the-environment")

	options := []string{
		"google-label=runner_manager_name:$TEST_HOSTNAME",
		"google-label=runner_manager_zone:${TEST_ZONE_UNSET}",
		"google-label=runner_manager_deployment:k8s",
		"google-label=runner_instance_name:${GITLAB_RUNNER_MACHINE_NAME}",
	}
	optionsWithName := []string{
		"google-label=note:${TEST_NOTE},machine:%s",
	}

	tests := map[string]struct {
		ff   bool
		want func(name string) []string
	}{
		"expanded when enabled": {
			ff: true,
			want: func(name string) []string {
				return []string{
					"google-label=runner_manager_name:gitlab-runner-p-s-1-86bfc8b89b-hkh9c",
					"google-label=runner_manager_zone:",
					"google-label=runner_manager_deployment:k8s",
					"google-label=runner_instance_name:" + name,
					"google-label=note:100%,machine:" + name,
				}
			},
		},
		"untouched when disabled": {
			ff: false,
			want: func(name string) []string {
				return append(append([]string(nil), options...), "google-label=note:${TEST_NOTE},machine:"+name)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p := newMachineProvider(docker_executor.NewProvider())
			machineMock := newMockMachine(t)
			p.machine = machineMock

			config := &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Machine: &common.DockerMachine{
						MachineName:            "test-machine-%s",
						IdleTime:               5,
						MachineOptions:         append([]string(nil), options...),
						MachineOptionsWithName: append([]string(nil), optionsWithName...),
					},
					FeatureFlags: map[string]bool{featureflags.MachineOptionsExpandEnv: tt.ff},
				},
			}

			var captured []string
			machineMock.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					for i := 4; i < len(args); i++ {
						captured = append(captured, args.String(i))
					}
				}).
				Return(nil).
				Once()

			d, errCh := p.create(config, machineStateIdle, false, nil)
			require.NoError(t, <-errCh)

			assert.Equal(t, tt.want(d.Name), captured)
			assert.Equal(t, options, config.Machine.MachineOptions, "the config's options are not mutated")
		})
	}
}

func TestMachineOptionsWithNameEmpty(t *testing.T) {
	provisionRetryInterval = 0

	p := newMachineProvider(docker_executor.NewProvider())
	machineMock := newMockMachine(t)
	p.machine = machineMock

	config := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Machine: &common.DockerMachine{
				MachineName:    "test-machine-%s",
				IdleTime:       5,
				MachineOptions: []string{"--option1=value1"},
			},
		},
	}

	var capturedOpts []string
	machineMock.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			for i := 4; i < len(args); i++ {
				capturedOpts = append(capturedOpts, args.String(i))
			}
		}).
		Return(nil).
		Once()

	d, errCh := p.create(config, machineStateIdle, false, nil)
	require.NotNil(t, d)
	require.NoError(t, <-errCh)

	expectedOpts := []string{"--option1=value1"}
	assert.Equal(t, expectedOpts, capturedOpts)
}

func TestFinalizeRemoval_StopsAfterMaxAttempts(t *testing.T) {
	origInterval := removeRetryInterval
	removeRetryInterval = time.Millisecond
	defer func() {
		removeRetryInterval = origInterval
	}()

	p, tm := testMachineProvider("remove-fail")
	const name = "remove-fail"

	p.lock.Lock()
	p.details = map[string]*machineDetails{
		name: {Name: name, State: machineStateRemoving, maxRemovalAttempts: 3},
	}
	p.lock.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		p.finalizeRemoval(p.details[name])
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("finalizeRemoval did not return within the bound")
	}

	p.lock.Lock()
	_, stillPresent := p.details[name]
	p.lock.Unlock()
	assert.False(t, stillPresent, "local state must be cleared on give-up so the IdleCount slot is freed")

	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	assert.Equal(t, 1, tm.forceRemoveCalls, "ForceRemove must run once on give-up to purge the local store JSON")
}

func TestUpdateAutoscalingLimits(t *testing.T) {
	provider := newMachineProvider(nil)
	config := &common.RunnerConfig{}
	config.Token = "test-token-1234567890"
	config.Name = "test-runner"
	config.SystemID = "s_c2d22f638c25"
	config.Machine = &common.DockerMachine{
		IdleCount:       40,
		IdleCountMin:    5,
		IdleScaleFactor: 0.5,
		MaxGrowthRate:   30,
	}

	provider.updateAutoscalingLimits(config, &machinesData{Used: 20}, nil)

	expected := fmt.Sprintf(`
# HELP gitlab_runner_autoscaling_idle_target The number of available machines the autoscaler aims to keep after applying IdleScaleFactor and IdleCountMin.
# TYPE gitlab_runner_autoscaling_idle_target gauge
gitlab_runner_autoscaling_idle_target{executor="docker+machine",runner="%[1]s",runner_name="test-runner",system_id="s_c2d22f638c25"} 10
# HELP gitlab_runner_autoscaling_max_growth_rate The maximum number of machines that can be in the creation state at once (0 = unlimited).
# TYPE gitlab_runner_autoscaling_max_growth_rate gauge
gitlab_runner_autoscaling_max_growth_rate{executor="docker+machine",runner="%[1]s",runner_name="test-runner",system_id="s_c2d22f638c25"} 30
`, config.ShortDescription())
	err := testutil.CollectAndCompare(provider, strings.NewReader(expected),
		"gitlab_runner_autoscaling_max_growth_rate", "gitlab_runner_autoscaling_idle_target")
	assert.NoError(t, err)
}

func TestUpdateAutoscalingLimits_StaleConfigDropped(t *testing.T) {
	provider := newMachineProvider(nil)
	provider.autoscalingLimits["gone"] = autoscalingLimits{
		maxGrowthRate: 30,
		idleTarget:    10,
		updatedAt:     time.Now().Add(-autoscalingLimitsTTL - time.Minute),
	}

	err := testutil.CollectAndCompare(provider, strings.NewReader(""),
		"gitlab_runner_autoscaling_max_growth_rate", "gitlab_runner_autoscaling_idle_target")
	assert.NoError(t, err)
	assert.NotContains(t, provider.autoscalingLimits, "gone")
}

func warmupMachineConfig(t *testing.T, token, namePrefix string) *common.RunnerConfig {
	config := &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{Token: token},
		RunnerSettings: common.RunnerSettings{
			Machine: &common.DockerMachine{
				MachineName:        namePrefix + "-%s",
				IdleCount:          4,
				IdleCountMin:       1,
				IdleScaleFactor:    0.9,
				IdleTime:           600,
				MaxGrowthRate:      1,
				WarmupIdleFraction: 0.5,
			},
		},
	}
	require.NoError(t, config.Machine.CompilePeriods())
	return config
}

func warmupStateFor(p *machineProvider, config *common.RunnerConfig) *warmupState {
	p.warmupLock.Lock()
	defer p.warmupLock.Unlock()
	return p.warmups[config.ShortDescription()]
}

func TestMachineWarmupPerRunner(t *testing.T) {
	p, m := testMachineProvider()
	runnerA := warmupMachineConfig(t, "token-a-1234567890", "warm-a")
	runnerB := warmupMachineConfig(t, "token-b-1234567890", "warm-b")
	runners := []*common.RunnerConfig{runnerA, runnerB}

	assert.Empty(t, p.warmups)
	ready, pending := p.WarmupReady(runners)
	assert.False(t, ready)
	assert.Equal(t, "warm-up: not started (token-a-1), not started (token-b-1)", pending)

	before := time.Now()
	_, err := p.Acquire(runnerA)
	assert.Error(t, err)

	stateA := warmupStateFor(p, runnerA)
	require.NotNil(t, stateA)
	assert.False(t, stateA.startedAt.Before(before))
	assert.True(t, stateA.burst)
	assert.Nil(t, warmupStateFor(p, runnerB))

	ready, pending = p.WarmupReady(runners)
	assert.False(t, ready)
	assert.Equal(t, "warm-up: booted 0/2 (token-a-1), not started (token-b-1)", pending)

	<-m.Created
	<-m.Created
	assertIdleMachines(t, p, 2)

	d, err := p.Acquire(runnerA)
	require.NoError(t, err)
	p.Release(runnerA, d)

	ready, pending = p.WarmupReady([]*common.RunnerConfig{runnerA})
	assert.True(t, ready, pending)
	ready, pending = p.WarmupReady(runners)
	assert.False(t, ready)
	assert.Equal(t, "warm-up: not started (token-b-1)", pending)

	_, err = p.Acquire(runnerB)
	assert.Error(t, err)
	require.NotNil(t, warmupStateFor(p, runnerB))
	assert.True(t, warmupStateFor(p, runnerA).ready)

	expected := fmt.Sprintf(`
# HELP gitlab_runner_autoscaling_idle_target The number of available machines the autoscaler aims to keep after applying IdleScaleFactor and IdleCountMin.
# TYPE gitlab_runner_autoscaling_idle_target gauge
gitlab_runner_autoscaling_idle_target{executor="docker+machine",runner="%[1]s",runner_name="",system_id="unknown"} 2
gitlab_runner_autoscaling_idle_target{executor="docker+machine",runner="%[2]s",runner_name="",system_id="unknown"} 2
# HELP gitlab_runner_autoscaling_max_growth_rate The maximum number of machines that can be in the creation state at once (0 = unlimited).
# TYPE gitlab_runner_autoscaling_max_growth_rate gauge
gitlab_runner_autoscaling_max_growth_rate{executor="docker+machine",runner="%[1]s",runner_name="",system_id="unknown"} 2
gitlab_runner_autoscaling_max_growth_rate{executor="docker+machine",runner="%[2]s",runner_name="",system_id="unknown"} 2
# HELP gitlab_runner_autoscaling_warmup 1 while the runner is in warm-up, 0 otherwise.
# TYPE gitlab_runner_autoscaling_warmup gauge
gitlab_runner_autoscaling_warmup{executor="docker+machine",runner="%[1]s",runner_name="",system_id="unknown"} 1
gitlab_runner_autoscaling_warmup{executor="docker+machine",runner="%[2]s",runner_name="",system_id="unknown"} 1
`, runnerA.ShortDescription(), runnerB.ShortDescription())
	err = testutil.CollectAndCompare(p, strings.NewReader(expected),
		"gitlab_runner_autoscaling_max_growth_rate", "gitlab_runner_autoscaling_idle_target", "gitlab_runner_autoscaling_warmup")
	assert.NoError(t, err)
}

func warmupLogEntries(hook *logrus_test.Hook, message string) []*logrus.Entry {
	var out []*logrus.Entry
	for _, e := range hook.AllEntries() {
		if e.Message == message {
			out = append(out, e)
		}
	}
	return out
}

func warmupGaugesExpected(config *common.RunnerConfig, idleTarget, maxGrowthRate, warmup, timedOut, readySeconds int) string {
	out := fmt.Sprintf(`
# HELP gitlab_runner_autoscaling_idle_target The number of available machines the autoscaler aims to keep after applying IdleScaleFactor and IdleCountMin.
# TYPE gitlab_runner_autoscaling_idle_target gauge
gitlab_runner_autoscaling_idle_target{executor="docker+machine",runner="%[1]s",runner_name="",system_id="unknown"} %[2]d
# HELP gitlab_runner_autoscaling_max_growth_rate The maximum number of machines that can be in the creation state at once (0 = unlimited).
# TYPE gitlab_runner_autoscaling_max_growth_rate gauge
gitlab_runner_autoscaling_max_growth_rate{executor="docker+machine",runner="%[1]s",runner_name="",system_id="unknown"} %[3]d
# HELP gitlab_runner_autoscaling_warmup 1 while the runner is in warm-up, 0 otherwise.
# TYPE gitlab_runner_autoscaling_warmup gauge
gitlab_runner_autoscaling_warmup{executor="docker+machine",runner="%[1]s",runner_name="",system_id="unknown"} %[4]d
# HELP gitlab_runner_autoscaling_warmup_timed_out 1 if warm-up ended before the pool reached the ready threshold, 0 otherwise.
# TYPE gitlab_runner_autoscaling_warmup_timed_out gauge
gitlab_runner_autoscaling_warmup_timed_out{executor="docker+machine",runner="%[1]s",runner_name="",system_id="unknown"} %[5]d
`, config.ShortDescription(), idleTarget, maxGrowthRate, warmup, timedOut)
	if readySeconds > 0 {
		out += fmt.Sprintf(`
# HELP gitlab_runner_autoscaling_warmup_ready_seconds Seconds from the start of warm-up until the pool reached the ready threshold. Absent until it does.
# TYPE gitlab_runner_autoscaling_warmup_ready_seconds gauge
gitlab_runner_autoscaling_warmup_ready_seconds{executor="docker+machine",runner="%[1]s",runner_name="",system_id="unknown"} %[2]d
`, config.ShortDescription(), readySeconds)
	}
	return out
}

var warmupGaugeNames = []string{
	"gitlab_runner_autoscaling_max_growth_rate", "gitlab_runner_autoscaling_idle_target",
	"gitlab_runner_autoscaling_warmup", "gitlab_runner_autoscaling_warmup_timed_out",
	"gitlab_runner_autoscaling_warmup_ready_seconds",
}

func TestMachineWarmupEndsAfterReady(t *testing.T) {
	hook, cleanup := test.NewHook()
	defer cleanup()

	p, m := testMachineProvider()
	config := warmupMachineConfig(t, "token-a-1234567890", "warm-a")

	_, err := p.Acquire(config)
	assert.Error(t, err)
	<-m.Created
	<-m.Created
	assertIdleMachines(t, p, 2)

	d1, err := p.Acquire(config)
	require.NoError(t, err)
	defer p.Release(config, d1)

	state := warmupStateFor(p, config)
	assert.True(t, state.ready)

	p.warmupLock.Lock()
	state.deadline = time.Now()
	state.readyAfter = 90 * time.Second
	p.warmupLock.Unlock()

	d2, err := p.Acquire(config)
	require.NoError(t, err)
	defer p.Release(config, d2)

	assert.True(t, state.ended)
	assert.False(t, state.timedOut)
	ended := warmupLogEntries(hook, "Warm-up ended")
	require.Len(t, ended, 1)
	assert.Equal(t, logrus.InfoLevel, ended[0].Level)

	assert.NoError(t, testutil.CollectAndCompare(p, strings.NewReader(warmupGaugesExpected(config, 1, 1, 0, 0, 90)), warmupGaugeNames...))
}

func TestMachineWarmupTimesOut(t *testing.T) {
	hook, cleanup := test.NewHook()
	defer cleanup()

	p, m := testMachineProvider()
	config := warmupMachineConfig(t, "token-a-1234567890", "warm-a")

	_, err := p.Acquire(config)
	assert.Error(t, err)

	state := warmupStateFor(p, config)
	p.warmupLock.Lock()
	state.deadline = time.Now()
	p.warmupLock.Unlock()

	d, _ := p.Acquire(config)
	if d != nil {
		defer p.Release(config, d)
	}

	assert.True(t, state.ended)
	assert.True(t, state.timedOut)
	ready, pending := p.WarmupReady([]*common.RunnerConfig{config})
	assert.True(t, ready, pending)

	ended := warmupLogEntries(hook, "Warm-up ended before the pool reached the ready threshold")
	require.Len(t, ended, 1)
	assert.Equal(t, logrus.WarnLevel, ended[0].Level)
	assert.Equal(t, 0, ended[0].Data["booted"])
	assert.Equal(t, 2, ended[0].Data["ready_machines"])

	assert.NoError(t, testutil.CollectAndCompare(p, strings.NewReader(warmupGaugesExpected(config, 1, 1, 0, 1, 0)), warmupGaugeNames...))

	<-m.Created
	<-m.Created
}

func TestMachineWarmupProgressLog(t *testing.T) {
	hook, cleanup := test.NewHook()
	defer cleanup()

	p, m := testMachineProvider()
	config := warmupMachineConfig(t, "token-a-1234567890", "warm-a")

	acquireAndRelease := func() {
		d, _ := p.Acquire(config)
		if d != nil {
			p.Release(config, d)
		}
	}

	acquireAndRelease()
	acquireAndRelease()
	assert.Empty(t, warmupLogEntries(hook, "Warm-up in progress"))

	state := warmupStateFor(p, config)
	p.warmupLock.Lock()
	state.startedAt = time.Now().Add(-2 * time.Minute)
	state.deadline = state.startedAt.Add(config.Machine.GetWarmupDuration())
	state.lastProgressLog = state.startedAt
	p.warmupLock.Unlock()

	<-m.Created
	<-m.Created
	assertIdleMachines(t, p, 2)

	acquireAndRelease()
	acquireAndRelease()

	progress := warmupLogEntries(hook, "Warm-up in progress")
	require.Len(t, progress, 1)
	assert.Equal(t, 2, progress[0].Data["booted"])
	assert.Equal(t, 2, progress[0].Data["idle"])
	assert.Equal(t, 2, progress[0].Data["ready_machines"])
	assert.Equal(t, 2, progress[0].Data["idle_target"])
	assert.Equal(t, true, progress[0].Data["ready"])
	assert.Equal(t, true, progress[0].Data["burst"])
	assert.Equal(t, 120.0, progress[0].Data["elapsed_s"])
	assert.Equal(t, 480.0, progress[0].Data["remaining_s"])
}

func TestMachineWarmupReadyIsMonotonic(t *testing.T) {
	p, m := testMachineProvider()
	config := warmupMachineConfig(t, "token-a-1234567890", "warm-a")

	_, err := p.Acquire(config)
	assert.Error(t, err)
	<-m.Created
	<-m.Created
	assertIdleMachines(t, p, 2)

	d1, err := p.Acquire(config)
	require.NoError(t, err)
	assert.True(t, warmupStateFor(p, config).ready)

	d2, err := p.Acquire(config)
	require.NoError(t, err)
	_, err = p.Acquire(config)
	assert.Error(t, err)

	ready, pending := p.WarmupReady([]*common.RunnerConfig{config})
	assert.True(t, ready, pending)

	p.Release(config, d1)
	p.Release(config, d2)
}

func TestMachineWarmupReadyCountsMachinesInUse(t *testing.T) {
	p, m := testMachineProvider()
	config := warmupMachineConfig(t, "token-a-1234567890", "warm-a")

	_, err := p.Acquire(config)
	assert.Error(t, err)
	<-m.Created
	<-m.Created
	assertIdleMachines(t, p, 2)

	d1, err := p.Acquire(config)
	require.NoError(t, err)
	d2, err := p.Acquire(config)
	require.NoError(t, err)
	defer p.Release(config, d1)
	defer p.Release(config, d2)

	state := warmupStateFor(p, config)
	p.warmupLock.Lock()
	state.ready = false
	p.warmupLock.Unlock()

	_, err = p.Acquire(config)
	assert.Error(t, err)
	assert.Equal(t, 2, state.booted)
	assert.True(t, state.ready)
}

func TestMachineWarmupCreationErrorDropsBurst(t *testing.T) {
	p, _ := testMachineProvider()
	config := warmupMachineConfig(t, "token-a-1234567890", "create-fail")

	_, err := p.Acquire(config)
	assert.Error(t, err)

	state := warmupStateFor(p, config)
	require.NotNil(t, state)
	require.Eventually(t, func() bool {
		p.warmupLock.Lock()
		defer p.warmupLock.Unlock()
		return !state.burst
	}, 5*time.Second, 10*time.Millisecond)

	assert.Equal(t, &warmupLimits{burst: false}, p.warmupLimitsFor(config, time.Now()))
	assert.Equal(t, config.Machine.MaxGrowthRate, maxGrowthRate(config, p.warmupLimitsFor(config, time.Now())))
}

func TestMachineWarmupDisabledByDefault(t *testing.T) {
	p, m := testMachineProvider()
	config := createMachineConfig(t, 1, 5)

	_, err := p.Acquire(config)
	assert.Error(t, err)
	<-m.Created

	assert.Empty(t, p.warmups)
	assert.Nil(t, p.warmupLimitsFor(config, time.Now()))
	ready, pending := p.WarmupReady([]*common.RunnerConfig{config})
	assert.True(t, ready)
	assert.Empty(t, pending)
}

func TestMachineWarmupLimitBelowTarget(t *testing.T) {
	hook, cleanup := test.NewHook()
	defer cleanup()

	p, m := testMachineProvider()
	config := warmupMachineConfig(t, "token-a-1234567890", "warm-a")
	config.Machine.WarmupIdleFraction = 1 // target 4, threshold 4
	config.Limit = 2

	_, err := p.Acquire(config)
	assert.Error(t, err)

	warnings := warmupLogEntries(hook, "Warm-up idle target is above the runner limit; the ready threshold is capped at the limit")
	require.Len(t, warnings, 1)
	assert.Equal(t, logrus.WarnLevel, warnings[0].Level)
	assert.Equal(t, 2, warnings[0].Data["limit"])
	assert.Equal(t, 4, warnings[0].Data["idle_target"])
	assert.Equal(t, 2, warnings[0].Data["ready_machines"])

	<-m.Created
	<-m.Created
	assertIdleMachines(t, p, 2)

	d, err := p.Acquire(config)
	require.NoError(t, err)
	defer p.Release(config, d)

	ready, pending := p.WarmupReady([]*common.RunnerConfig{config})
	assert.True(t, ready, pending)
}

func TestMachineWarmupUnacquiredRunnerReadyAfterDuration(t *testing.T) {
	p, _ := testMachineProvider()
	config := warmupMachineConfig(t, "", "warm-a")
	config.Machine.WarmupDuration = 20 * time.Millisecond

	ready, pending := p.WarmupReady([]*common.RunnerConfig{config})
	assert.False(t, ready)
	assert.Equal(t, "warm-up: not started ()", pending)

	time.Sleep(100 * time.Millisecond)

	ready, pending = p.WarmupReady([]*common.RunnerConfig{config})
	assert.True(t, ready, pending)
}

func TestMachineWarmupReadyEndsWarmupWithoutAcquire(t *testing.T) {
	hook, cleanup := test.NewHook()
	defer cleanup()

	p, m := testMachineProvider()
	config := warmupMachineConfig(t, "token-a-1234567890", "warm-a")

	_, err := p.Acquire(config)
	assert.Error(t, err)

	state := warmupStateFor(p, config)
	p.warmupLock.Lock()
	state.deadline = time.Now()
	p.warmupLock.Unlock()

	ready, pending := p.WarmupReady([]*common.RunnerConfig{config})
	assert.True(t, ready, pending)
	assert.True(t, state.ended)
	assert.True(t, state.timedOut)
	require.Len(t, warmupLogEntries(hook, "Warm-up ended before the pool reached the ready threshold"), 1)

	<-m.Created
	<-m.Created
}

func TestMachineWarmupCreationErrorIgnoredWhenGrowthUnlimited(t *testing.T) {
	hook, cleanup := test.NewHook()
	defer cleanup()

	p, _ := testMachineProvider()
	config := warmupMachineConfig(t, "token-a-1234567890", "create-fail")
	config.Machine.MaxGrowthRate = 0

	_, err := p.Acquire(config)
	assert.Error(t, err)

	state := warmupStateFor(p, config)
	require.NotNil(t, state)
	time.Sleep(50 * time.Millisecond)
	assert.True(t, state.burst)
	assert.Empty(t, warmupLogEntries(hook, "Machine creation failed during warm-up, dropping the creation burst"))
}

func TestMachineWarmupPrunesStaleRunners(t *testing.T) {
	p, m := testMachineProvider()
	gone := warmupMachineConfig(t, "token-gone-1234567890", "warm-gone")
	config := warmupMachineConfig(t, "token-a-1234567890", "warm-a")

	_, _ = p.Acquire(gone)
	state := warmupStateFor(p, gone)
	p.warmupLock.Lock()
	state.ended = true
	state.lastAcquire = time.Now().Add(-autoscalingLimitsTTL - time.Minute)
	p.warmupLock.Unlock()

	_, _ = p.Acquire(config)
	assert.Nil(t, warmupStateFor(p, gone))
	assert.NotNil(t, warmupStateFor(p, config))

	for range 4 {
		<-m.Created
	}
}

func TestMachineWarmupGaugesOnlyWhenEnabled(t *testing.T) {
	p, m := testMachineProvider()
	config := createMachineConfig(t, 1, 5)
	config.Token = "token-a-1234567890"

	_, err := p.Acquire(config)
	assert.Error(t, err)
	<-m.Created

	err = testutil.CollectAndCompare(p, strings.NewReader(""),
		"gitlab_runner_autoscaling_warmup", "gitlab_runner_autoscaling_warmup_timed_out", "gitlab_runner_autoscaling_warmup_ready_seconds")
	assert.NoError(t, err)
}

func TestMachineWarmupGaugesFollowReadyWithoutAcquire(t *testing.T) {
	p, m := testMachineProvider()
	config := warmupMachineConfig(t, "token-a-1234567890", "warm-a")

	_, err := p.Acquire(config)
	assert.Error(t, err)
	<-m.Created
	<-m.Created

	state := warmupStateFor(p, config)
	p.warmupLock.Lock()
	state.ready = false
	state.deadline = time.Now()
	p.warmupLock.Unlock()

	ready, _ := p.WarmupReady([]*common.RunnerConfig{config})
	require.True(t, ready)

	assert.NoError(t, testutil.CollectAndCompare(p, strings.NewReader(`
# HELP gitlab_runner_autoscaling_warmup 1 while the runner is in warm-up, 0 otherwise.
# TYPE gitlab_runner_autoscaling_warmup gauge
gitlab_runner_autoscaling_warmup{executor="docker+machine",runner="token-a-1",runner_name="",system_id="unknown"} 0
# HELP gitlab_runner_autoscaling_warmup_timed_out 1 if warm-up ended before the pool reached the ready threshold, 0 otherwise.
# TYPE gitlab_runner_autoscaling_warmup_timed_out gauge
gitlab_runner_autoscaling_warmup_timed_out{executor="docker+machine",runner="token-a-1",runner_name="",system_id="unknown"} 1
`), "gitlab_runner_autoscaling_warmup", "gitlab_runner_autoscaling_warmup_timed_out"))
}

func TestMachineWarmupPrunedRunnerDoesNotRestartWarmup(t *testing.T) {
	hook, cleanup := test.NewHook()
	defer cleanup()

	p, m := testMachineProvider()
	busy := warmupMachineConfig(t, "token-busy-1234567890", "warm-busy")
	other := warmupMachineConfig(t, "token-other-1234567890", "warm-other")

	_, _ = p.Acquire(busy)
	_, _ = p.Acquire(other)

	state := warmupStateFor(p, busy)
	p.warmupLock.Lock()
	state.deadline = time.Now().Add(-time.Second)
	p.warmupLock.Unlock()
	_, _ = p.Acquire(busy)
	require.True(t, state.ended)
	p.warmupLock.Lock()
	state.lastAcquire = time.Now().Add(-autoscalingLimitsTTL - time.Minute)
	p.warmupLock.Unlock()
	p.startedAt = time.Now().Add(-30 * time.Minute)

	_, _ = p.Acquire(other)
	_, _ = p.Acquire(busy)

	assert.Nil(t, p.warmupLimitsFor(busy, time.Now()))
	started := 0
	for _, e := range warmupLogEntries(hook, "Warm-up started") {
		if e.Data["runner"] == busy.ShortDescription() {
			started++
		}
	}
	assert.Equal(t, 1, started)

	for range 4 {
		<-m.Created
	}
}
