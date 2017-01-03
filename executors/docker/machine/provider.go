package machine

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers/docker"
)

type machineProviderStatistics struct {
	Created int
	Used    int
	Removed int
}

type machineProvider struct {
	name        string
	machine     docker_helpers.Machine
	details     machinesDetails
	lock        sync.RWMutex
	acquireLock sync.Mutex
	statistics  machineProviderStatistics
	// provider stores a real executor that is used to start run the builds
	provider common.ExecutorProvider

	machinesDataDesc       *prometheus.Desc
	providerStatisticsDesc *prometheus.Desc
}

func (m *machineProvider) machineDetails(name string, acquire bool) *machineDetails {
	m.lock.Lock()
	defer m.lock.Unlock()

	details, ok := m.details[name]
	if !ok {
		details = &machineDetails{
			Name:      name,
			Created:   time.Now(),
			Used:      time.Now(),
			LastSeen:  time.Now(),
			UsedCount: 1, // any machine that we find we mark as already used
			State:     machineStateIdle,
		}
		m.details[name] = details
	}

	if acquire {
		if details.isUsed() {
			return nil
		}
		details.State = machineStateAcquired
	}

	return details
}

func (m *machineProvider) create(config *common.RunnerConfig, state machineState) (details *machineDetails, errCh chan error) {
	name := newMachineName(config)
	details = m.machineDetails(name, true)
	details.State = machineStateCreating
	details.UsedCount = 0
	details.RetryCount = 0
	details.LastSeen = time.Now()
	errCh = make(chan error, 1)

	// Create machine asynchronously
	go func() {
		started := time.Now()
		err := m.machine.Create(config.Machine.MachineDriver, details.Name, config.Machine.MachineOptions...)
		for i := 0; i < 3 && err != nil; i++ {
			details.RetryCount++
			logrus.WithField("name", details.Name).WithError(err).
				Warningln("Machine creation failed, trying to provision")
			time.Sleep(provisionRetryInterval)
			err = m.machine.Provision(details.Name)
		}

		if err != nil {
			logrus.WithField("name", details.Name).
				WithField("time", time.Since(started)).
				WithError(err).
				Errorln("Machine creation failed")
			m.remove(details.Name, "Failed to create")
		} else {
			details.State = state
			details.Used = time.Now()
			logrus.WithField("time", time.Since(started)).
				WithField("name", details.Name).
				WithField("now", time.Now()).
				WithField("retries", details.RetryCount).
				Infoln("Machine created")
			m.statistics.Created++
		}
		errCh <- err
	}()
	return
}

func (m *machineProvider) findFreeMachine(machines ...string) (details *machineDetails) {
	// Enumerate all machines in reverse order, to always take the newest machines first
	for idx := range machines {
		name := machines[len(machines)-idx-1]
		details := m.machineDetails(name, true)
		if details == nil {
			continue
		}

		// Check if node is running
		canConnect := m.machine.CanConnect(name)
		if !canConnect {
			m.remove(name, "machine is unavailable")
			continue
		}
		return details
	}

	return nil
}

func (m *machineProvider) useMachine(config *common.RunnerConfig) (details *machineDetails, err error) {
	machines, err := m.loadMachines(config)
	if err != nil {
		return
	}
	details = m.findFreeMachine(machines...)
	if details == nil {
		var errCh chan error
		details, errCh = m.create(config, machineStateAcquired)
		err = <-errCh
	}
	return
}

func (m *machineProvider) retryUseMachine(config *common.RunnerConfig) (details *machineDetails, err error) {
	// Try to find a machine
	for i := 0; i < 3; i++ {
		details, err = m.useMachine(config)
		if err == nil {
			break
		}
		time.Sleep(provisionRetryInterval)
	}
	return
}

func (m *machineProvider) finalizeRemoval(details *machineDetails) {
	for {
		if !m.machine.Exist(details.Name) {
			logrus.WithField("name", details.Name).
				WithField("created", time.Since(details.Created)).
				WithField("used", time.Since(details.Used)).
				WithField("reason", details.Reason).
				Warningln("Skipping machine removal, because it doesn't exist")
			break
		}

		err := m.machine.Remove(details.Name)
		if err == nil {
			break
		}
		time.Sleep(30 * time.Second)
		logrus.WithField("name", details.Name).
			WithField("created", time.Since(details.Created)).
			WithField("used", time.Since(details.Used)).
			WithField("reason", details.Reason).
			Warningln("Retrying removal")
		details.RetryCount++
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.details, details.Name)

	logrus.WithField("name", details.Name).
		WithField("created", time.Since(details.Created)).
		WithField("used", time.Since(details.Used)).
		WithField("reason", details.Reason).
		WithField("now", time.Now()).
		WithField("retries", details.RetryCount).
		Infoln("Machine removed")

	m.statistics.Removed++
}

func (m *machineProvider) remove(machineName string, reason ...interface{}) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	details, _ := m.details[machineName]
	if details == nil {
		return errors.New("Machine not found")
	}

	details.Reason = fmt.Sprint(reason...)
	details.State = machineStateRemoving
	details.RetryCount = 0
	logrus.WithField("name", machineName).
		WithField("created", time.Since(details.Created)).
		WithField("used", time.Since(details.Used)).
		WithField("reason", details.Reason).
		WithField("now", time.Now()).
		Warningln("Removing machine")
	details.Used = time.Now()
	details.writeDebugInformation()

	go m.finalizeRemoval(details)
	return nil
}

func (m *machineProvider) updateMachine(config *common.RunnerConfig, data *machinesData, details *machineDetails) error {
	if details.State != machineStateIdle {
		return nil
	}

	if config.Machine.MaxBuilds > 0 && details.UsedCount >= config.Machine.MaxBuilds {
		// Limit number of builds
		return errors.New("Too many builds")
	}

	if data.Total() >= config.Limit && config.Limit > 0 {
		// Limit maximum number of machines
		return errors.New("Too many machines")
	}

	if time.Since(details.Used) > time.Second*time.Duration(config.Machine.GetIdleTime()) {
		if data.Idle >= config.Machine.GetIdleCount() {
			// Remove machine that are way over the idle time
			return errors.New("Too many idle machines")
		}
	}
	return nil
}

func (m *machineProvider) updateMachines(machines []string, config *common.RunnerConfig) (data machinesData, validMachines []string) {
	data.Runner = config.ShortDescription()
	validMachines = make([]string, 0, len(machines))

	for _, name := range machines {
		details := m.machineDetails(name, false)
		details.LastSeen = time.Now()

		err := m.updateMachine(config, &data, details)
		if err == nil {
			validMachines = append(validMachines, name)
		} else {
			m.remove(details.Name, err)
		}

		data.Add(details.State)
	}
	return
}

func (m *machineProvider) createMachines(config *common.RunnerConfig, data *machinesData) {
	// Create a new machines and mark them as Idle
	for {
		if data.Available() >= config.Machine.GetIdleCount() {
			// Limit maximum number of idle machines
			break
		}
		if data.Total() >= config.Limit && config.Limit > 0 {
			// Limit maximum number of machines
			break
		}
		m.create(config, machineStateIdle)
		data.Creating++
	}
}

func (m *machineProvider) loadMachines(config *common.RunnerConfig) (machines []string, err error) {
	machines, err = m.machine.List()
	if err != nil {
		return nil, err
	}

	machines = filterMachineList(machines, machineFilter(config))
	return
}

func (m *machineProvider) Acquire(config *common.RunnerConfig) (data common.ExecutorData, err error) {
	if config.Machine == nil || config.Machine.MachineName == "" {
		err = fmt.Errorf("Missing Machine options")
		return
	}

	// Lock updating machines, because two Acquires can be run at the same time
	m.acquireLock.Lock()
	defer m.acquireLock.Unlock()

	machines, err := m.loadMachines(config)
	if err != nil {
		return
	}

	// Update a list of currently configured machines
	machinesData, validMachines := m.updateMachines(machines, config)

	// Pre-create machines
	m.createMachines(config, &machinesData)

	logrus.WithFields(machinesData.Fields()).
		WithField("runner", config.ShortDescription()).
		WithField("minIdleCount", config.Machine.GetIdleCount()).
		WithField("maxMachines", config.Limit).
		WithField("time", time.Now()).
		Debugln("Docker Machine Details")
	machinesData.writeDebugInformation()

	// Try to find a free machine
	details := m.findFreeMachine(validMachines...)
	if details != nil {
		data = details
		return
	}

	// If we have a free machines we can process a build
	if config.Machine.GetIdleCount() != 0 && machinesData.Idle == 0 {
		err = errors.New("No free machines that can process builds")
	}
	return
}

func (m *machineProvider) Use(config *common.RunnerConfig, data common.ExecutorData) (newConfig common.RunnerConfig, newData common.ExecutorData, err error) {
	// Find a new machine
	details, _ := data.(*machineDetails)
	if details == nil || !details.canBeUsed() || !m.machine.CanConnect(details.Name) {
		details, err = m.retryUseMachine(config)
		if err != nil {
			return
		}

		// Return details only if this is a new instance
		newData = details
	}

	// Get machine credentials
	dc, err := m.machine.Credentials(details.Name)
	if err != nil {
		if newData != nil {
			m.Release(config, newData)
		}
		return
	}

	// Create shallow copy of config and store in it docker credentials
	newConfig = *config
	newConfig.Docker = &common.DockerConfig{}
	if config.Docker != nil {
		*newConfig.Docker = *config.Docker
	}
	newConfig.Docker.DockerCredentials = dc

	// Mark machine as used
	details.State = machineStateUsed
	details.Used = time.Now()
	details.UsedCount++
	m.statistics.Used++
	return
}

func (m *machineProvider) Release(config *common.RunnerConfig, data common.ExecutorData) error {
	// Release machine
	details, ok := data.(*machineDetails)
	if ok {
		// Mark last used time when is Used
		if details.State == machineStateUsed {
			details.Used = time.Now()
		}

		// Remove machine if we already used it
		if config.Machine.MaxBuilds > 0 && details.UsedCount >= config.Machine.MaxBuilds {
			err := m.remove(details.Name, "Too many builds")
			if err == nil {
				return nil
			}
		}
		details.State = machineStateIdle
	}
	return nil
}

func (m *machineProvider) CanCreate() bool {
	return m.provider.CanCreate()
}

func (m *machineProvider) GetFeatures(features *common.FeaturesInfo) {
	m.provider.GetFeatures(features)
}

func (m *machineProvider) Create() common.Executor {
	return &machineExecutor{
		provider: m,
	}
}

func newMachineProvider(name, executor string) *machineProvider {
	provider := common.GetExecutor(executor)
	if provider == nil {
		logrus.Panicln("Missing", executor)
	}

	return &machineProvider{
		name:     name,
		details:  make(machinesDetails),
		machine:  docker_helpers.NewMachineCommand(),
		provider: provider,
	}
}
