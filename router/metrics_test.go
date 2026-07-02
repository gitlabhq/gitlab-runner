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

// histogramSampleCount returns the number of observations recorded in a Histogram.
func histogramSampleCount(t *testing.T, h prometheus.Histogram) uint64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, h.(prometheus.Metric).Write(&m))
	return m.GetHistogram().GetSampleCount()
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

	assert.Zero(t, histogramSampleCount(t, rc.metrics.getJobDuration))

	// Each request that reaches the router records one GetJob observation.
	rc.RequestJob(t.Context(), config, sessionInfo)
	rc.RequestJob(t.Context(), config, sessionInfo)
	assert.EqualValues(t, 2, histogramSampleCount(t, rc.metrics.getJobDuration))
}

func TestMetrics_GetJobDurationObservedOnFailure(t *testing.T) {
	rc, config, routerSrv, _ := setupRouterWithRailsFallback(t)
	routerSrv.failCode.Store(int32(codes.Unavailable))
	sessionInfo := &common.SessionInfo{}

	// The router is reached but fails; the duration is still observed.
	rc.RequestJob(t.Context(), config, sessionInfo)
	assert.EqualValues(t, 1, histogramSampleCount(t, rc.metrics.getJobDuration))
}

func TestMetrics_Fallback_BreakerTrippedAndOpen(t *testing.T) {
	rc, config, routerSrv, _ := setupRouterWithRailsFallback(t)
	now := time.Now()
	installTestBreaker(rc, &now)
	routerSrv.failCode.Store(int32(codes.Unavailable))
	sessionInfo := &common.SessionInfo{}

	// Failures within the grace window are transient: no fallback yet.
	for i := 0; i < routerBreakerFailureThreshold+1; i++ {
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
