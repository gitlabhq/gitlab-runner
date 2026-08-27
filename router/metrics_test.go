//go:build !integration

package router

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/network"
	"gitlab.com/gitlab-org/gitlab-runner/router/rpc"
)

const getJobDurationMetricName = "gitlab_runner_job_router_get_job_duration_seconds"

// gatherMetric returns every series a collector emits for the named metric.
// Going through a registry exercises the collector's Describe/Collect wiring
// instead of reading the metric vectors directly.
func gatherMetric(t *testing.T, c prometheus.Collector, metricName string) []*dto.Metric {
	t.Helper()

	reg := prometheus.NewPedanticRegistry()
	require.NoError(t, reg.Register(c))
	families, err := reg.Gather()
	require.NoError(t, err)

	for _, mf := range families {
		if mf.GetName() == metricName {
			return mf.GetMetric()
		}
	}
	return nil
}

// labelsOf returns a series' labels as a map.
func labelsOf(m *dto.Metric) map[string]string {
	labels := map[string]string{}
	for _, lp := range m.GetLabel() {
		labels[lp.GetName()] = lp.GetValue()
	}
	return labels
}

// getJobSeries returns the GetJob observation counts keyed by
// "<runner>/<system_id>/<result>", so tests assert on the label values and not
// just on the number of observations.
func getJobSeries(t *testing.T, rc *Client) map[string]uint64 {
	t.Helper()

	series := map[string]uint64{}
	for _, m := range gatherMetric(t, rc, getJobDurationMetricName) {
		labels := labelsOf(m)
		require.Len(t, labels, 3, "unexpected labels on %s: %v", getJobDurationMetricName, labels)
		key := labels["runner"] + "/" + labels["system_id"] + "/" + labels["result"]
		series[key] = m.GetHistogram().GetSampleCount()
	}
	return series
}

// getJobKey is the getJobSeries key a config emits for the given result.
func getJobKey(config common.RunnerConfig, result getJobResult) string {
	return config.RunnerCredentials.ShortDescription() + "/" + config.SystemID + "/" + string(result)
}

func fallbackCount(rc *Client, reason fallbackReason) float64 {
	return testutil.ToFloat64(rc.metrics.fallbacks.WithLabelValues(string(reason)))
}

// setRouterDiscovery injects a cached discovery result under the client mutex,
// keeping the same locking discipline as the production code paths.
func setRouterDiscovery(rc *Client, disco *common.RouterDiscovery, expiresAt time.Time) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.disco = disco
	rc.discoExpiresAt = expiresAt
}

func TestMetrics_DiscoveryCacheHitMiss(t *testing.T) {
	rc, gitLabURL := setupWithRouter(t, false)
	config := newConfig(gitLabURL)
	sessionInfo := &common.SessionInfo{}

	// First request: the cache is empty, so discovery is a miss.
	rc.RequestJob(t.Context(), config, sessionInfo)
	assert.EqualValues(t, 1, testutil.ToFloat64(rc.metrics.discoveryCacheEvents.WithLabelValues(cacheMiss)))
	assert.EqualValues(t, 0, testutil.ToFloat64(rc.metrics.discoveryCacheEvents.WithLabelValues(cacheHit)))

	// Second request: served from the cached discovery, so it is a hit.
	rc.RequestJob(t.Context(), config, sessionInfo)
	assert.EqualValues(t, 1, testutil.ToFloat64(rc.metrics.discoveryCacheEvents.WithLabelValues(cacheMiss)))
	assert.EqualValues(t, 1, testutil.ToFloat64(rc.metrics.discoveryCacheEvents.WithLabelValues(cacheHit)))
}

func TestMetrics_GetJobDurationObserved(t *testing.T) {
	rc, gitLabURL := setupWithRouter(t, false)
	config := newConfig(gitLabURL)
	sessionInfo := &common.SessionInfo{}

	assert.Empty(t, getJobSeries(t, rc))

	// Each request that reaches the router records one GetJob observation,
	// attributed to the runner that issued it.
	rc.RequestJob(t.Context(), config, sessionInfo)
	rc.RequestJob(t.Context(), config, sessionInfo)
	assert.Equal(t, map[string]uint64{getJobKey(config, getJobResultJob): 2}, getJobSeries(t, rc))
}

func TestMetrics_GetJobDurationObservedOnFailure(t *testing.T) {
	rc, config, routerSrv, _ := setupRouterWithRailsFallback(t)
	routerSrv.failCode.Store(int32(codes.Unavailable))
	sessionInfo := &common.SessionInfo{}

	// The router is reached but fails; the duration is still observed.
	rc.RequestJob(t.Context(), config, sessionInfo)
	assert.Equal(t, map[string]uint64{getJobKey(config, getJobResultError): 1}, getJobSeries(t, rc))
}

func TestMetrics_GetJobDurationPartitionedByRunner(t *testing.T) {
	rc, gitLabURL := setupWithRouter(t, false)
	sessionInfo := &common.SessionInfo{}

	first := newConfig(gitLabURL)
	second := newConfig(gitLabURL)
	second.Token = "glrt-456456456"
	second.SystemID = "s_second_system_id"

	rc.RequestJob(t.Context(), first, sessionInfo)
	rc.RequestJob(t.Context(), second, sessionInfo)
	rc.RequestJob(t.Context(), second, sessionInfo)

	// Each configured runner entry gets its own series, so latency can be broken
	// down per shard instead of collapsing into one series per runner process.
	assert.Equal(t, map[string]uint64{
		getJobKey(first, getJobResultJob):  1,
		getJobKey(second, getJobResultJob): 2,
	}, getJobSeries(t, rc))
}

// TestMetrics_GetJobDurationPartitionedByResult covers the label that keeps the
// histogram interpretable: a no-job long-poll waits out GitLab's CI polling window
// and would otherwise dominate the latency tail of every other outcome.
func TestMetrics_GetJobDurationPartitionedByResult(t *testing.T) {
	rc, config, routerSrv, railsCalls := setupRouterWithRailsFallback(t)
	sessionInfo := &common.SessionInfo{}

	// A poll that returns a job.
	rc.RequestJob(t.Context(), config, sessionInfo)

	// Polls that return no job.
	routerSrv.noJob.Store(true)
	rc.RequestJob(t.Context(), config, sessionInfo)
	rc.RequestJob(t.Context(), config, sessionInfo)

	// A poll whose RPC fails. One failure stays under the breaker threshold, so the
	// request is not retried against Rails and the observation is not duplicated.
	routerSrv.noJob.Store(false)
	routerSrv.failCode.Store(int32(codes.Unavailable))
	rc.RequestJob(t.Context(), config, sessionInfo)

	assert.Equal(t, map[string]uint64{
		getJobKey(config, getJobResultJob):   1,
		getJobKey(config, getJobResultNoJob): 2,
		getJobKey(config, getJobResultError): 1,
	}, getJobSeries(t, rc))
	assert.Zero(t, railsCalls.Load(), "no fallback expected")
}

func TestMetrics_Fallback_BreakerTrippedAndOpen(t *testing.T) {
	rc, config, routerSrv, _ := setupRouterWithRailsFallback(t)
	now := time.Now()
	installTestBreaker(rc, &now)
	routerSrv.failCode.Store(int32(codes.Unavailable))
	sessionInfo := &common.SessionInfo{}

	// Failures within the grace window are transient: no fallback yet.
	for range routerBreakerFailureThreshold + 1 {
		rc.RequestJob(t.Context(), config, sessionInfo)
	}
	assert.Zero(t, fallbackCount(rc, fallbackBreakerTripped))
	assert.Zero(t, fallbackCount(rc, fallbackBreakerOpen))

	// Crossing the grace window trips the breaker and falls back once.
	now = now.Add(routerBreakerFailureGrace + time.Second)
	rc.RequestJob(t.Context(), config, sessionInfo)
	assert.EqualValues(t, 1, fallbackCount(rc, fallbackBreakerTripped))

	// While open, further requests fall back without contacting the router.
	rc.RequestJob(t.Context(), config, sessionInfo)
	assert.EqualValues(t, 1, fallbackCount(rc, fallbackBreakerOpen))
	assert.EqualValues(t, 1, fallbackCount(rc, fallbackBreakerTripped))
}

func TestMetrics_Fallback_RouterDisabled(t *testing.T) {
	rc, config, _, _ := setupRouterWithRailsFallback(t)
	sessionInfo := &common.SessionInfo{}

	// Point the client at a router that reports itself disabled.
	disabledSrv := startDisabledRouter(t)
	setRouterDiscovery(rc, &common.RouterDiscovery{ServerURL: disabledSrv}, time.Now().Add(discoveryTTL))

	rc.RequestJob(t.Context(), config, sessionInfo)
	assert.EqualValues(t, 1, fallbackCount(rc, fallbackRouterDisabled))
}

func TestMetrics_Fallback_NoDiscovery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v4/runners/router/discovery", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})
	mux.HandleFunc("POST /api/v4/jobs/request", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte(fakeJobResponse))
		assert.NoError(t, err)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	certDir := t.TempDir()
	rc := NewClient(network.NewGitLabClient(network.WithCertificateDirectory(certDir)), certDir, "runner-test")
	t.Cleanup(rc.Shutdown)
	config := newConfig(server.URL)
	sessionInfo := &common.SessionInfo{}

	rc.RequestJob(t.Context(), config, sessionInfo)
	assert.EqualValues(t, 1, fallbackCount(rc, fallbackNoDiscovery))
}

// startDisabledRouter starts a gRPC router that always reports the Job Router as
// disabled, and returns its grpc:// URL.
func startDisabledRouter(t *testing.T) string {
	t.Helper()
	routerSrv := &mockRouterServer{t: t, disabled: true}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })
	r := grpc.NewServer()
	rpc.RegisterJobRouterServer(r, routerSrv)
	go func() { assert.NoError(t, r.Serve(l)) }()
	t.Cleanup(r.GracefulStop)
	return fmt.Sprintf("grpc://%s", l.Addr())
}
