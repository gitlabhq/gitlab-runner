//go:build !integration

package machine

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	docker_executor "gitlab.com/gitlab-org/gitlab-runner/executors/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

// idleCount sums the Idle counters across every target bucket. The
// bucketed return type reflects the fact that machines created under
// different [runners.machine.Target] configs land in different gauge
// series; tests that don't care about the bucketing dimension just
// want the total.
func idleCount(buckets map[targetLabels]*machinesData) int {
	var total int
	for _, b := range buckets {
		total += b.Idle
	}
	return total
}

func TestIfMachineProviderExposesCollectInterface(t *testing.T) {
	var provider common.ExecutorProvider = &machineProvider{}
	collector, ok := provider.(prometheus.Collector)
	assert.True(t, ok)
	assert.NotNil(t, collector)
}

func TestCollectDetailsIgnoresLastSeenAge(t *testing.T) {
	provider := newMachineProvider(docker_executor.NewProvider())
	assert.Equal(t, 0, idleCount(provider.collectDetails()))

	details := provider.machineDetails("test", false)
	assert.Equal(t, 1, idleCount(provider.collectDetails()))

	details.LastSeen = time.Now().Add(-3 * time.Hour)
	assert.Equal(t, 1, idleCount(provider.collectDetails()))
}

// TestCollectBucketsByTarget verifies that the machine-states gauge
// emits one (state, target_*) series per distinct target tuple, so an
// operator can break down idle/used/etc by zone, region, project, and
// machine type.
func TestCollectBucketsByTarget(t *testing.T) {
	provider := newMachineProvider(docker_executor.NewProvider())

	east := targetLabels{
		zone: "us-east1-d", region: "us-east1", project: "p1",
		machineType: "n2d-standard-2",
	}
	west := targetLabels{
		zone: "us-central1-a", region: "us-central1", project: "p2",
		machineType: "t2a-standard-2",
	}

	// Two idle machines in east, one used in west. Bypass create() and
	// poke targets directly so the test stays focused on the collector.
	now := time.Now()
	for _, spec := range []struct {
		name    string
		targets targetLabels
		state   machineState
	}{
		{"east-idle-a", east, machineStateIdle},
		{"east-idle-b", east, machineStateIdle},
		{"west-used-a", west, machineStateUsed},
	} {
		d := provider.machineDetails(spec.name, false)
		d.Lock()
		d.targets = spec.targets
		d.State = spec.state
		d.LastSeen = now
		d.Unlock()
	}

	buckets := provider.collectDetails()
	require.Len(t, buckets, 2, "expected one bucket per distinct target tuple")
	require.Contains(t, buckets, east)
	require.Contains(t, buckets, west)
	assert.Equal(t, 2, buckets[east].Idle)
	assert.Equal(t, 0, buckets[east].Used)
	assert.Equal(t, 0, buckets[west].Idle)
	assert.Equal(t, 1, buckets[west].Used)

	// End-to-end: drive the prometheus.Collector path and assert that
	// every state sample carries the bucket's target_* label values.
	ch := make(chan prometheus.Metric, 64)
	provider.Collect(ch)
	close(ch)

	type sample struct {
		labels map[string]string
		value  float64
	}
	var stateSamples []sample
	for metric := range ch {
		desc := metric.Desc().String()
		if !strings.Contains(desc, "gitlab_runner_autoscaling_machine_states") {
			continue
		}
		var pb dto.Metric
		require.NoError(t, metric.Write(&pb))
		labels := map[string]string{}
		for _, lp := range pb.Label {
			labels[lp.GetName()] = lp.GetValue()
		}
		stateSamples = append(stateSamples, sample{labels: labels, value: pb.GetGauge().GetValue()})
	}

	// 6 states × 2 buckets.
	require.Len(t, stateSamples, 12)

	find := func(targetZone, state string) (sample, bool) {
		for _, s := range stateSamples {
			if s.labels["target_zone"] == targetZone && s.labels["state"] == state {
				return s, true
			}
		}
		return sample{}, false
	}

	eastIdle, ok := find("us-east1-d", "idle")
	require.True(t, ok)
	assert.Equal(t, float64(2), eastIdle.value)
	assert.Equal(t, "us-east1", eastIdle.labels["target_region"])
	assert.Equal(t, "p1", eastIdle.labels["target_project"])
	assert.Equal(t, "n2d-standard-2", eastIdle.labels["target_machine_type"])

	westUsed, ok := find("us-central1-a", "used")
	require.True(t, ok)
	assert.Equal(t, float64(1), westUsed.value)
	assert.Equal(t, "t2a-standard-2", westUsed.labels["target_machine_type"])
}

// TestCollectEmitsZeroSamplesWithoutMachines locks down the invariant
// that scrapers see per-state zero samples on a freshly started
// provider, even before the first machine is registered.
func TestCollectEmitsZeroSamplesWithoutMachines(t *testing.T) {
	provider := newMachineProvider(docker_executor.NewProvider())

	ch := make(chan prometheus.Metric, 64)
	provider.Collect(ch)
	close(ch)

	var count int
	for metric := range ch {
		if strings.Contains(metric.Desc().String(), "gitlab_runner_autoscaling_machine_states") {
			count++
		}
	}
	assert.Equal(t, 6, count, "expected one zero sample per state")
}

func TestCreationMetrics(t *testing.T) {
	machineConfig := &common.MachineConfig{
		CreationDurationSecondsBuckets:      []float64{1, 10},
		CreationPhaseDurationSecondsBuckets: []float64{1, 10},
	}

	provider := newMachineProvider(docker_executor.NewProvider())
	provider.Init(&common.Config{Machine: machineConfig})
	reg := prometheus.NewPedanticRegistry()
	require.NoError(t, reg.Register(provider))
	assert.Equal(t, 0, testutil.CollectAndCount(provider.creationHistogram))

	config := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Machine:      &common.DockerMachine{MachineName: "phase-test-%s", IdleTime: 5, MachineDriver: "google"},
			FeatureFlags: map[string]bool{featureflags.DockerMachineJSONOutput: true},
		},
		GlobalMachineConfig: machineConfig,
	}

	machineMock := docker.NewMockMachine(t)
	provider.machine = machineMock
	machineMock.On("Inspect", mock.Anything).Return(docker.MachineInfo{
		DriverName: "google", Zone: "us-east1-d", Project: "p1", MachineType: "n2d-standard-2",
	}, nil)

	t0 := time.Now()
	machineMock.On("Create", mock.Anything, "google", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			onOutput := args.Get(3).(docker.MachineOutput).OnEntry
			onOutput(phaseEntry(t0, "driver_create"))
			onOutput(phaseEntry(t0.Add(15*time.Second), "wait_ssh"))
			onOutput(phaseEntry(t0.Add(20*time.Second), "provision"))
		}).
		Return(nil).Once()
	machineMock.On("Create", mock.Anything, "google", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			args.Get(3).(docker.MachineOutput).OnEntry(phaseEntry(t0, "wait_ssh"))
		}).
		Return(errors.New("timeout")).Once()
	machineMock.On("Exist", mock.Anything, mock.Anything).Return(false).Maybe()
	machineMock.On("Remove", mock.Anything, mock.Anything).Return(nil).Maybe()
	machineMock.On("Stop", mock.Anything, mock.Anything).Return(nil).Maybe()

	_, errCh := provider.create(config, machineStateIdle, false, nil)
	require.NoError(t, <-errCh)
	_, errCh = provider.create(config, machineStateIdle, false, nil)
	require.Error(t, <-errCh)

	_, err := reg.Gather()
	require.NoError(t, err)

	targets := []string{"us-east1-d", "us-east1", "p1", "n2d-standard-2"}
	histogram := func(vec *prometheus.HistogramVec, labels ...string) *dto.Histogram {
		var pb dto.Metric
		require.NoError(t, vec.WithLabelValues(labels...).(prometheus.Metric).Write(&pb))
		return pb.Histogram
	}
	phase := func(name, result string) *dto.Histogram {
		return histogram(provider.creationPhaseHistogram, append([]string{name, result}, targets...)...)
	}

	creation := histogram(provider.creationHistogram, targets...)
	assert.EqualValues(t, 1, creation.GetSampleCount())
	assert.Len(t, creation.Bucket, 2, "configured buckets are used")
	assert.Equal(t, 1, testutil.CollectAndCount(provider.creationHistogram))

	assert.EqualValues(t, 1, histogram(provider.failedCreationHistogram, targets...).GetSampleCount())

	assert.InDelta(t, 15, phase("driver_create", "success").GetSampleSum(), 0.01)
	assert.InDelta(t, 5, phase("wait_ssh", "success").GetSampleSum(), 0.01)
	assert.EqualValues(t, 1, phase("provision", "success").GetSampleCount())
	assert.EqualValues(t, 1, phase("wait_ssh", "failure").GetSampleCount())
	assert.Equal(t, 4, testutil.CollectAndCount(provider.creationPhaseHistogram))
}

func TestCreationPhaseMetricsDisabled(t *testing.T) {
	provider := newMachineProvider(docker_executor.NewProvider())
	provider.Init(&common.Config{Machine: &common.MachineConfig{DisableCreationPhaseMetrics: true}})
	reg := prometheus.NewPedanticRegistry()
	require.NoError(t, reg.Register(provider))
	assert.Nil(t, provider.creationPhaseHistogram)

	config := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Machine:      &common.DockerMachine{MachineName: "phase-off-%s", IdleTime: 5, MachineDriver: "google"},
			FeatureFlags: map[string]bool{featureflags.DockerMachineJSONOutput: true},
		},
		GlobalMachineConfig: &common.MachineConfig{DisableCreationPhaseMetrics: true},
	}

	machineMock := newMockMachine(t)
	provider.machine = machineMock
	machineMock.On("Create", mock.Anything, "google", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			args.Get(3).(docker.MachineOutput).OnEntry(phaseEntry(time.Now(), "wait_ssh"))
		}).
		Return(nil).Once()

	_, errCh := provider.create(config, machineStateIdle, false, nil)
	require.NoError(t, <-errCh)

	families, err := reg.Gather()
	require.NoError(t, err)
	for _, f := range families {
		assert.NotEqual(t, "gitlab_runner_autoscaling_machine_creation_phase_duration_seconds", f.GetName())
	}
}
