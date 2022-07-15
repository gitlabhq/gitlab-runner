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
	runners     runnersDetails
	lock        sync.RWMutex
	acquireLock sync.Mutex
	// provider stores a real executor that is used to start run the builds
	provider common.ExecutorProvider

	stuckRemoveLock sync.Mutex

	// metrics
	totalActions            *prometheus.CounterVec
	currentStatesDesc       *prometheus.Desc
	creationHistogram       prometheus.Histogram
	stoppingHistogram       prometheus.Histogram
	removalHistogram        prometheus.Histogram
	failedCreationHistogram prometheus.Histogram
}

func (m *machineProvider) machineDetails(name string, acquire bool) *machineDetails {
	details := m.ensureDetails(name)
	if acquire {
		details = m.tryAcquireMachineDetails(details)
	}

	return details
}

func (m *machineProvider) ensureDetails(name string) *machineDetails {
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

	return details
}

var errNoConfig = errors.New("no runner config specified")

func (m *machineProvider) runnerMachinesCoordinator(config *common.RunnerConfig) (*runnerMachinesCoordinator, error) {
	if config == nil {
		return nil, errNoConfig
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	details, ok := m.runners[config.GetToken()]
	if !ok {
		details = newRunnerMachinesCoordinator()
		m.runners[config.GetToken()] = details
	}

	return details, nil
}

func (m *machineProvider) create(config *common.RunnerConfig, state machineState) (*machineDetails, chan error) {
	name := newMachineName(config)
	details := m.machineDetails(name, true)
	m.lock.Lock()
	details.State = machineStateCreating
	details.UsedCount = 0
	details.RetryCount = 0
	details.LastSeen = time.Now()
	m.lock.Unlock()
	errCh := make(chan error, 1)

	// Create machine with the required configuration asynchronously
	coordinator, err := m.runnerMachinesCoordinator(config)
	if err != nil {
		errCh <- err
		return nil, errCh
	}

	go coordinator.waitForGrowthCapacity(config.Machine.MaxGrowthRate, func() {
		m.createWithGrowthCapacity(coordinator, config, details, state, errCh)
	})

	return details, errCh
}

func (m *machineProvider) createWithGrowthCapacity(
	coordinator *runnerMachinesCoordinator,
	config *common.RunnerConfig,
	details *machineDetails,
	state machineState,
	errCh chan error,
) {
	logger := logrus.WithField("name", details.Name)
	started := time.Now()

	err := m.machine.Create(config.Machine.MachineDriver, details.Name, config.Machine.MachineOptions...)
	if err != nil {
		logger.WithField("time", time.Since(started)).
			WithError(err).
			Errorln("Machine creation failed")
		m.totalActions.WithLabelValues("creation-failed").Inc()
		m.failedCreationHistogram.Observe(time.Since(started).Seconds())
		_ = m.remove(details.Name, "Failed to create")
	} else {
		m.lock.Lock()
		details.State = state
		details.Used = time.Now()
		m.lock.Unlock()
		creationTime := time.Since(started)
		m.lock.RLock()
		logger.WithField("duration", creationTime).
			WithField("now", time.Now()).
			WithField("retries", details.RetryCount).
			Infoln("Machine created")
		m.lock.RUnlock()
		m.totalActions.WithLabelValues("created").Inc()
		m.creationHistogram.Observe(creationTime.Seconds())

		// Signal that a new machine is available. When there's contention, there's no guarantee between the
		// ordering of reading from errCh and the availability check.
		coordinator.addAvailableMachine()
	}
	errCh <- err
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

func (m *machineProvider) findFreeExistingMachine(config *common.RunnerConfig) (*machineDetails, error) {
	machines, err := m.loadMachines(config)
	if err != nil {
		return nil, err
	}

	return m.findFreeMachine(true, machines...), nil
}

func (m *machineProvider) useMachine(config *common.RunnerConfig) (*machineDetails, error) {
	details, err := m.findFreeExistingMachine(config)
	if err != nil || details != nil {
		return details, err
	}

	return m.createAndAcquireMachine(config)
}

func (m *machineProvider) createAndAcquireMachine(config *common.RunnerConfig) (*machineDetails, error) {
	coordinator, err := m.runnerMachinesCoordinator(config)
	if err != nil {
		return nil, err
	}

	newDetails, errCh := m.create(config, machineStateIdle)
	// Use either a free machine, or the created machine; whichever comes first. There's no guarantee that the created
	// machine can be used by us because between the time the machine is created, and the acquisition of the machine,
	// another goroutine may have found it via findFreeMachine and acquired it.
	var details *machineDetails
	for details == nil && err == nil {
		select {
		case err = <-errCh:
			if err != nil {
				return nil, err
			}

			details = m.tryAcquireMachineDetails(newDetails)
		case <-coordinator.availableMachineSignal():
			// Even though the signal is fired and we are *almost* sure that
			// there's a machine available, let's use the getAvailableMachine
			// method so that the internal counter is synchonized with what
			// we are actually doing and so that we can be sure that no other
			// goroutine that didn't accept the signal and instead used the ticker
			// hasn't already snatched a machine
			details, err = m.tryGetFreeExistingMachineFromCoordinator(config, coordinator)
		case <-time.After(time.Second):
			details, err = m.tryGetFreeExistingMachineFromCoordinator(config, coordinator)
		}
	}

	return details, err
}

func (m *machineProvider) tryGetFreeExistingMachineFromCoordinator(
	config *common.RunnerConfig,
	coordinator *runnerMachinesCoordinator,
) (*machineDetails, error) {
	if coordinator.getAvailableMachine() {
		return m.findFreeExistingMachine(config)
	}

	return nil, nil
}

func (m *machineProvider) tryAcquireMachineDetails(details *machineDetails) *machineDetails {
	m.lock.Lock()
	defer m.lock.Unlock()

	if details.isUsed() {
		return nil
	}

	details.State = machineStateAcquired
	return details
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

	details.logger().Warningln("Stopping machine")
	err = runHistogramCountedOperation(m.stoppingHistogram, func() error {
		return m.machine.Stop(details.Name, machineStopCommandTimeout)
	})
	if err != nil {
		details.logger().
			WithError(err).
			Warningln("Error while stopping machine")
	}

	details.logger().Warningln("Removing machine")
	err = runHistogramCountedOperation(m.removalHistogram, func() error {
		return m.machine.Remove(details.Name)
	})
	if err != nil {
		details.RetryCount++
		time.Sleep(removeRetryInterval)
		return err
	}

	return nil
}

func runHistogramCountedOperation(histogram prometheus.Histogram, operation func() error) error {
	startedAt := time.Now()
	err := operation()
	histogram.Observe(time.Since(startedAt).Seconds())

	return err
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

func (m *machineProvider) updateMachines(
	machines []string,
	config *common.RunnerConfig,
) (data machinesData, validMachines []string) {
	data.Runner = config.ShortDescription()
	validMachines = make([]string, 0, len(machines))

	for _, name := range machines {
		details := m.machineDetails(name, false)
		details.LastSeen = time.Now()

		reason := shouldRemoveIdle(config, &data, details)
		if reason == dontRemoveIdleMachine {
			validMachines = append(validMachines, name)
		} else {
			_ = m.remove(details.Name, reason)
		}

		data.Add(details)
	}
	return
}

// createMachines starts goroutines that are creating the new machines.
// Limiting strategy is used to ensure the autoscaling parameters are respected.
func (m *machineProvider) createMachines(config *common.RunnerConfig, data *machinesData) {
	for {
		if !canCreateIdle(config, data) {
			return
		}

		// Create a new machine and mark it as Idle
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

	logger := logrus.WithFields(machinesData.Fields()).
		WithField("runner", config.ShortDescription()).
		WithField("idleCountMin", config.Machine.GetIdleCountMin()).
		WithField("idleCount", config.Machine.GetIdleCount()).
		WithField("idleScaleFactor", config.Machine.GetIdleScaleFactor()).
		WithField("maxMachines", config.Limit).
		WithField("maxMachineCreate", config.Machine.MaxGrowthRate)

	logger.WithField("time", time.Now()).Debugln("Docker Machine Details")
	machinesData.writeDebugInformation()

	// Try to find a free machine
	details := m.findFreeMachine(false, validMachines...)
	if details != nil {
		return details, nil
	}

	if config.Machine.GetIdleCount() == 0 {
		logger.Info("IdleCount is set to 0 so the machine will be created on demand in job context")
	} else if machinesData.Idle == 0 {
		return nil, &common.NoFreeExecutorError{Message: "no free machines that can process builds"}
	}

	return nil, nil
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
	if !ok {
		return
	}

	m.lock.Lock()
	// Mark last used time when is Used
	if details.State == machineStateUsed {
		details.Used = time.Now()
	}
	m.lock.Unlock()

	// Remove machine if we already used it
	if config != nil && config.Machine != nil &&
		config.Machine.MaxBuilds > 0 && details.UsedCount >= config.Machine.MaxBuilds {
		err := m.remove(details.Name, "Too many builds")
		if err == nil {
			return
		}
	}
	m.lock.Lock()
	details.State = machineStateIdle
	m.lock.Unlock()

	// Signal pending builds that a new machine is available.
	if err := m.signalRelease(config); err != nil {
		return
	}
}

func (m *machineProvider) signalRelease(config *common.RunnerConfig) error {
	coordinator, err := m.runnerMachinesCoordinator(config)
	if err != nil && err != errNoConfig {
		return err
	}

	if err != errNoConfig && coordinator != nil {
		coordinator.addAvailableMachine()
	}

	return nil
}

func (m *machineProvider) CanCreate() bool {
	return m.provider.CanCreate()
}

func (m *machineProvider) GetFeatures(features *common.FeaturesInfo) error {
	return m.provider.GetFeatures(features)
}

func (m *machineProvider) GetConfigInfo(input *common.RunnerConfig, output *common.ConfigInfo) {
	m.provider.GetConfigInfo(input, output)
}

func (m *machineProvider) GetDefaultShell() string {
	return m.provider.GetDefaultShell()
}

func (m *machineProvider) Create() common.Executor {
	return &machineExecutor{
		provider: m,
	}
}

//nolint:funlen
func newMachineProvider(name, executor string) *machineProvider {
	provider := common.GetExecutorProvider(executor)
	if provider == nil {
		logrus.Panicln("Missing", executor)
	}

	return &machineProvider{
		name:     name,
		details:  make(machinesDetails),
		runners:  make(runnersDetails),
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
		stoppingHistogram: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "gitlab_runner_autoscaling_machine_stopping_duration_seconds",
				Help:    "Histogram of machine stopping time.",
				Buckets: []float64{1, 3, 5, 10, 30, 50, 60, 80, 90, 120},
				ConstLabels: prometheus.Labels{
					"executor": name,
				},
			},
		),
		removalHistogram: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "gitlab_runner_autoscaling_machine_removal_duration_seconds",
				Help:    "Histogram of machine removal time.",
				Buckets: []float64{1, 3, 5, 10, 30, 50, 60, 80, 90, 120},
				ConstLabels: prometheus.Labels{
					"executor": name,
				},
			},
		),
		failedCreationHistogram: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "gitlab_runner_autoscaling_machine_failed_creation_duration_seconds",
				Help:    "Histogram of machine failed creation timings",
				Buckets: []float64{1, 3, 5, 10, 30, 50, 60, 80, 90, 120},
				ConstLabels: prometheus.Labels{
					"executor": name,
				},
			},
		),
	}
}
