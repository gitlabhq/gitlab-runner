//go:build !integration

package machine

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
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

func createMachineConfig(t *testing.T, idleCount int, idleTime int) *common.RunnerConfig {
	conf := &common.RunnerConfig{
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

type testMachine struct {
	machines []string
	second   bool

	Created chan bool
	Removed chan bool
	Stopped chan bool

	mutex sync.Mutex
}

func (m *testMachine) Create(driver, name string, opts ...string) error {
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

func (m *testMachine) Provision(name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if strings.Contains(name, "provision-fail") || strings.Contains(name, "second-fail") {
		return errors.New("failed to provision")
	}
	m.machines = append(m.machines, name)
	return nil
}

func (m *testMachine) Stop(name string, timeout time.Duration) error {
	m.Stopped <- true

	return nil
}

func (m *testMachine) Remove(name string) error {
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

func (m *testMachine) Exist(name string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, machine := range m.machines {
		if machine == name {
			return true
		}
	}
	return false
}

func (m *testMachine) List() (machines []string, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.machines, nil
}

func (m *testMachine) CanConnect(name string, skipCache bool) bool {
	return !strings.Contains(name, "no-can-connect")
}

func (m *testMachine) Credentials(name string) (dc docker.Credentials, err error) {
	if strings.Contains(name, "no-connect") {
		err = errors.New("failed to connect")
	}
	return
}

func countIdleMachines(p *machineProvider) (count int) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for _, details := range p.details {
		if details.State == machineStateIdle {
			count++
		}
	}
	return
}

func assertIdleMachines(t *testing.T, p *machineProvider, expected int, msgAndArgs ...interface{}) {
	var idle int
	for i := 0; i < 10; i++ {
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

func assertTotalMachines(t *testing.T, p *machineProvider, expected int, msgAndArgs ...interface{}) {
	var total int
	for i := 0; i < 10; i++ {
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
	p := newMachineProvider("docker+machine", "docker")
	p.machine = t
	return p, t
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
	d, errCh := p.create(machineDefaultConfig, machineStateUsed)
	assert.NotNil(t, d)
	assert.NoError(t, <-errCh)
	assert.Equal(t, machineStateUsed, d.State)
	assert.Equal(t, 0, d.UsedCount)
	assert.NotNil(t, p.details[d.Name])

	err := p.remove(d.Name)
	assert.NoError(t, err)
	assert.Equal(t, machineStateRemoving, d.State)

	d2, errCh := p.create(machineProvisionFail, machineStateUsed)
	assert.NotNil(t, d2)
	assert.Error(t, <-errCh, "Fails, because it fails to provision machine")
	assert.Equal(t, machineStateRemoving, d2.State)

	d3, errCh := p.create(machineCreateFail, machineStateUsed)
	assert.NotNil(t, d3)
	assert.Error(t, <-errCh)
	assert.Equal(t, machineStateRemoving, d3.State)
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

	p := newMachineProvider("docker+machine", "docker")

	machineMock := &docker.MockMachine{}
	defer machineMock.AssertExpectations(t)
	p.machine = machineMock

	var blockCreatingMachineWg sync.WaitGroup
	blockCreatingMachineWg.Add(1)

	var createdMachineDetails *machineDetails

	machineMock.On("Create", mock.Anything, mock.Anything).
		Return(nil).
		Once()

	machineMock.On("Create", mock.Anything, mock.Anything).
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

	machineMock.On("CanConnect", mock.Anything, mock.Anything).
		Return(true).
		Once()

	createdMachineDetails, errCh := p.create(machineGrowthConfig, machineStateUsed)
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

	p := newMachineProvider("docker+machine", "docker")

	machineMock := &docker.MockMachine{}
	defer machineMock.AssertExpectations(t)
	p.machine = machineMock

	var listLock sync.Mutex
	list := make([]string, 0)

	listCall := machineMock.On("List").Return([]string{}, nil)
	machineMock.On("Create", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			name := args.String(1)

			listLock.Lock()
			list = append(list, name)
			listCopy := make([]string, len(list))
			copy(listCopy, list)
			listCall.Return(listCopy, nil).Maybe()
			listLock.Unlock()
		}).
		Return(nil)

	machineMock.On("CanConnect", mock.Anything, mock.Anything).
		Return(true).
		Maybe()

	const N = 500
	var wg sync.WaitGroup
	wg.Add(N)

	startCh := make(chan struct{})
	for i := 0; i < N; i++ {
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
			machineMock := docker.MockMachine{}
			defer machineMock.AssertExpectations(t)
			p.machine = &machineMock

			var wg sync.WaitGroup
			var concurrentCalls, maxConcurrentCalls int32
			var maxConcurrentCallsLock sync.Mutex

			machineMock.On("Create", mock.Anything, mock.Anything).
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
				wg.Add(1)
				go func() {
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
					}, machineStateIdle)

					<-errCh
					wg.Done()
				}()
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
	d, errCh := p.create(config, machineStateIdle)
	assert.NoError(t, <-errCh, "machine creation should not fail")

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
