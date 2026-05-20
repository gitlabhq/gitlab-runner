//go:build !integration

package machine

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	docker_executor "gitlab.com/gitlab-org/gitlab-runner/executors/docker"
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

func TestMachineProviderDeadInterval(t *testing.T) {
	provider := newMachineProvider(docker_executor.NewProvider())
	assert.Equal(t, 0, idleCount(provider.collectDetails()))

	details := provider.machineDetails("test", false)
	assert.Equal(t, 1, idleCount(provider.collectDetails()))

	details.LastSeen = time.Now().Add(-(machineDeadInterval * time.Second))
	assert.Equal(t, 0, idleCount(provider.collectDetails()))
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
