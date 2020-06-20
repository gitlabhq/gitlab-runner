package machine

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

type machineProvider struct {
	name        string
	machine     docker.Machine
	details     machinesDetails
	lock        sync.RWMutex
	acquireLock sync.Mutex
	// provider stores a real executor that is used to start run the builds
	provider common.ExecutorProvider

	stuckRemoveLock sync.Mutex

	// metrics
	totalActions      *prometheus.CounterVec
	currentStatesDesc *prometheus.Desc
	creationHistogram prometheus.Histogram
}

func (m *machineProvider) machineDetails(name string, acquire bool) *machineDetails {
	m.lock.Lock()
	defer m.lock.Unlock()

	details, ok := m.details[name]
	if !ok {
		now := time.Now()
		details = &machineDetails{
			Name:      name,
			Created:   now,
			Used:      now,
			LastSeen:  now,
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

func (m *machineProvider) create(config *common.RunnerConfig, state machineState) (*machineDetails, chan error) {
	name := newMachineName(config)
	details := m.machineDetails(name, true)
	details.State = machineStateCreating
	details.UsedCount = 0
	details.RetryCount = 0
	details.LastSeen = time.Now()
	errCh := make(chan error, 1)

	// Create machine asynchronously
	go func() {
		started := time.Now()
		err := m.machine.Create(config.Machine.MachineDriver, details.Name, config.Machine.MachineOptions...)
		for i := 0; i < 3 && err != nil; i++ {
			details.RetryCount++
			logrus.WithField("name", details.Name).
				WithError(err).
				Warningln("Machine creation failed, trying to provision")
			time.Sleep(provisionRetryInterval)
			err = m.machine.Provision(details.Name)
		}

		if err != nil {
			logrus.WithField("name", details.Name).
				WithField("time", time.Since(started)).
				WithError(err).
				Errorln("Machine creation failed")
			_ = m.remove(details.Name, "Failed to create")
		} else {
			details.State = state
			details.Used = time.Now()
			creationTime := time.Since(started)
			logrus.WithField("duration", creationTime).
				WithField("name", details.Name).
				WithField("now", time.Now()).
				WithField("retries", details.RetryCount).
				Infoln("Machine created")
			m.totalActions.WithLabelValues("created").Inc()
			m.creationHistogram.Observe(creationTime.Seconds())
		}
		errCh <- err
	}()

	return details, errCh
}

func (m *machineProvider) findFreeMachine(skipCache bool, machines ...string) (details *machineDetails) {
	// Enumerate all machines in reverse order, to always take the newest machines first
	for idx := range machines {
		name := machines[len(machines)-idx-1]
		details := m.machineDetails(name, true)
		if details == nil {
			continue
		}

		// Check if node is running
		canConnect := m.machine.CanConnect(name, skipCache)
		if !canConnect {
			_ = m.remove(name, "machine is unavailable")
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
	details = m.findFreeMachine(true, machines...)
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

func (m *machineProvider) removeMachine(details *machineDetails) (err error) {
	if !m.machine.Exist(details.Name) {
		details.logger().
			Warningln("Skipping machine removal, because it doesn't exist")
		return nil
	}

	// This code limits amount of removal of stuck machines to one machine per interval
	if details.isStuckOnRemove() {
		m.stuckRemoveLock.Lock()
		defer m.stuckRemoveLock.Unlock()
	}

	details.logger().
		Warningln("Stopping machine")
	err = m.machine.Stop(details.Name, machineStopCommandTimeout)
	if err != nil {
		details.logger().
			WithError(err).
			Warningln("Error while stopping machine")
	}

	details.logger().
		Warningln("Removing machine")
	err = m.machine.Remove(details.Name)
	if err != nil {
		details.RetryCount++
		time.Sleep(removeRetryInterval)
		return err
	}

	return nil
}

func (m *machineProvider) finalizeRemoval(details *machineDetails) {
	for {
		err := m.removeMachine(details)
		if err == nil {
			break
		}
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.details, details.Name)

	details.logger().
		WithField("now", time.Now()).
		WithField("retries", details.RetryCount).
		Infoln("Machine removed")

	m.totalActions.WithLabelValues("removed").Inc()
}

func (m *machineProvider) remove(machineName string, reason ...interface{}) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	details := m.details[machineName]
	if details == nil {
		return errors.New("machine not found")
	}

	details.Reason = fmt.Sprint(reason...)
	details.State = machineStateRemoving
	details.RetryCount = 0

	details.logger().
		WithField("now", time.Now()).
		Warningln("Requesting machine removal")

	details.Used = time.Now()
	details.writeDebugInformation()

	go m.finalizeRemoval(details)
	return nil
}

func (m *machineProvider) updateMachine(
	config *common.RunnerConfig,
	data *machinesData,
	details *machineDetails,
) error {
	if details.State != machineStateIdle {
		return nil
	}

	if config.Machine.MaxBuilds > 0 && details.UsedCount >= config.Machine.MaxBuilds {
		// Limit number of builds
		return errors.New("too many builds")
	}

	if data.Total() >= config.Limit && config.Limit > 0 {
		// Limit maximum number of machines
		return errors.New("too many machines")
	}

	if time.Since(details.Used) > time.Second*time.Duration(config.Machine.GetIdleTime()) {
		if data.Idle >= config.Machine.GetIdleCount() {
			// Remove machine that are way over the idle time
			return errors.New("too many idle machines")
		}
	}
	return nil
}

func (m *machineProvider) updateMachines(
	machines []string,
	config *common.RunnerConfig,
) (data machinesData, validMachines []string) {
	data.Runner = config.ShortDescription()
	validMachines = make([]string, 0, len(machines))

	for _, name := range machines {
		details := m.machineDetails(name, false)
		details.LastSeen = time.Now()

		err := m.updateMachine(config, &data, details)
		if err == nil {
			validMachines = append(validMachines, name)
		} else {
			_ = m.remove(details.Name, err)
		}

		data.Add(details)
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

// intermediateMachineList returns a list of machines that might not yet be
// persisted on disk, these machines are the ones between being virtually
// created, and `docker-machine create` getting executed we populate this data
// set to overcome the race conditions related to not-full set of machines
// returned by `docker-machine ls -q`
func (m *machineProvider) intermediateMachineList(excludedMachines []string) []string {
	var excludedSet map[string]struct{}
	var intermediateMachines []string

	m.lock.Lock()
	defer m.lock.Unlock()

	for _, details := range m.details {
		if details.isPersistedOnDisk() {
			continue
		}

		// lazy init set, as most of times we don't create new machines
		if excludedSet == nil {
			excludedSet = make(map[string]struct{}, len(excludedMachines))
			for _, excludedMachine := range excludedMachines {
				excludedSet[excludedMachine] = struct{}{}
			}
		}

		if _, ok := excludedSet[details.Name]; ok {
			continue
		}

		intermediateMachines = append(intermediateMachines, details.Name)
	}

	return intermediateMachines
}

func (m *machineProvider) loadMachines(config *common.RunnerConfig) (machines []string, err error) {
	machines, err = m.machine.List()
	if err != nil {
		return nil, err
	}

	machines = append(machines, m.intermediateMachineList(machines)...)
	machines = filterMachineList(machines, machineFilter(config))
	return
}

func (m *machineProvider) Acquire(config *common.RunnerConfig) (common.ExecutorData, error) {
	if config.Machine == nil || config.Machine.MachineName == "" {
		return nil, fmt.Errorf("missing Machine options")
	}

	// Lock updating machines, because two Acquires can be run at the same time
	m.acquireLock.Lock()
	defer m.acquireLock.Unlock()

	machines, err := m.loadMachines(config)
	if err != nil {
		return nil, err
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
	details := m.findFreeMachine(false, validMachines...)
	if details != nil {
		return details, nil
	}

	// If we have a free machines we can process a build
	if config.Machine.GetIdleCount() != 0 && machinesData.Idle == 0 {
		err = errors.New("no free machines that can process builds")
	}
	return nil, err
}

//nolint:nakedret
func (m *machineProvider) Use(
	config *common.RunnerConfig,
	data common.ExecutorData,
) (newConfig common.RunnerConfig, newData common.ExecutorData, err error) {
	// Find a new machine
	details, _ := data.(*machineDetails)
	if details == nil || !details.canBeUsed() || !m.machine.CanConnect(details.Name, true) {
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
		newData = nil
		return
	}

	// Create shallow copy of config and store in it docker credentials
	newConfig = *config
	newConfig.Docker = &common.DockerConfig{}
	if config.Docker != nil {
		*newConfig.Docker = *config.Docker
	}
	newConfig.Docker.Credentials = dc

	// Mark machine as used
	details.State = machineStateUsed
	details.Used = time.Now()
	details.UsedCount++
	m.totalActions.WithLabelValues("used").Inc()
	return
}

func (m *machineProvider) Release(config *common.RunnerConfig, data common.ExecutorData) {
	// Release machine
	details, ok := data.(*machineDetails)
	if ok {
		// Mark last used time when is Used
		if details.State == machineStateUsed {
			details.Used = time.Now()
		}

		// Remove machine if we already used it
		if config != nil && config.Machine != nil &&
			config.Machine.MaxBuilds > 0 && details.UsedCount >= config.Machine.MaxBuilds {
			err := m.remove(details.Name, "Too many builds")
			if err == nil {
				return
			}
		}
		details.State = machineStateIdle
	}
}

func (m *machineProvider) CanCreate() bool {
	return m.provider.CanCreate()
}

func (m *machineProvider) GetFeatures(features *common.FeaturesInfo) error {
	return m.provider.GetFeatures(features)
}

func (m *machineProvider) GetDefaultShell() string {
	return m.provider.GetDefaultShell()
}

func (m *machineProvider) Create() common.Executor {
	return &machineExecutor{
		provider: m,
	}
}

func newMachineProvider(name, executor string) *machineProvider {
	provider := common.GetExecutorProvider(executor)
	if provider == nil {
		logrus.Panicln("Missing", executor)
	}

	return &machineProvider{
		name:     name,
		details:  make(machinesDetails),
		machine:  docker.NewMachineCommand(),
		provider: provider,
		totalActions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gitlab_runner_autoscaling_actions_total",
				Help: "The total number of actions executed by the provider.",
				ConstLabels: prometheus.Labels{
					"executor": name,
				},
			},
			[]string{"action"},
		),
		currentStatesDesc: prometheus.NewDesc(
			"gitlab_runner_autoscaling_machine_states",
			"The current number of machines per state in this provider.",
			[]string{"state"},
			prometheus.Labels{
				"executor": name,
			},
		),
		creationHistogram: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "gitlab_runner_autoscaling_machine_creation_duration_seconds",
				Help:    "Histogram of machine creation time.",
				Buckets: prometheus.ExponentialBuckets(30, 1.25, 10),
				ConstLabels: prometheus.Labels{
					"executor": name,
				},
			},
		),
	}
}
