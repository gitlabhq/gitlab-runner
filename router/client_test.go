//go:build !integration

package router

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/certificate"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/circuitbreaker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/network"
	"gitlab.com/gitlab-org/gitlab-runner/router/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	fakeJobResponse   = `{"id":1223}`
	responseRequestID = "123-123-123"
)

func TestRequestJob_HappyPath(t *testing.T) {
	doTest := func(t *testing.T, withTLS bool) {
		rc, gitLabURL := setupWithRouter(t, withTLS)
		config := newConfig(gitLabURL)
		sessionInfo := &common.SessionInfo{}
		// Request 1
		job, healthy := rc.RequestJob(t.Context(), config, sessionInfo)
		require.True(t, healthy)
		require.NotNil(t, job)
		assert.EqualValues(t, 1223, job.ID) // just ensure we get what the server sent without testing the whole object as it may change.
		assert.Equal(t, responseRequestID, job.JobRequestCorrelationID)

		firstConn := rc.factory.currentConn
		assert.Zero(t, rc.factory.currentConn.numUsers)
		assert.Empty(t, rc.factory.closingConns)
		assert.Same(t, firstConn, rc.factory.currentConn)

		// Request 2 - should reuse the same connection without issues
		job, healthy = rc.RequestJob(t.Context(), config, sessionInfo)
		require.True(t, healthy)
		require.NotNil(t, job)
		assert.EqualValues(t, 1223, job.ID)
		assert.Equal(t, responseRequestID, job.JobRequestCorrelationID)

		assert.Zero(t, rc.factory.currentConn.numUsers)
		assert.Empty(t, rc.factory.closingConns)
		assert.Same(t, firstConn, rc.factory.currentConn)

		// Request 3 - should use a new connection since we have a new token
		config.Token = "glrt-new-token!"
		job, healthy = rc.RequestJob(t.Context(), config, sessionInfo)
		require.True(t, healthy)
		require.NotNil(t, job)
		assert.Zero(t, rc.factory.currentConn.numUsers)
		assert.Empty(t, rc.factory.closingConns)
		assert.NotSame(t, firstConn, rc.factory.currentConn) // we've used a new gRPC client connection
	}
	t.Run("no TLS", func(t *testing.T) {
		doTest(t, false)
	})
	t.Run("TLS", func(t *testing.T) {
		doTest(t, true)
	})
}

func TestRequestJob_FeatureFlagOff(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v4/jobs/request", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte(fakeJobResponse))
		assert.NoError(t, err)
	})
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Failf(t, "unexpected call", "%s %s", req.Method, req.URL)
		w.WriteHeader(http.StatusNotImplemented)
	}))
	server := httptest.NewServer(mux)
	defer server.Close()

	certDir := t.TempDir()
	rc := NewClient(
		network.NewGitLabClient(
			network.WithCertificateDirectory(certDir),
		),
		certDir,
		"runner-test",
	)
	defer rc.Shutdown()
	config := newConfig(server.URL)
	config.RunnerSettings.FeatureFlags = nil
	sessionInfo := &common.SessionInfo{}
	job, healthy := rc.RequestJob(t.Context(), config, sessionInfo)
	require.True(t, healthy)
	require.NotNil(t, job)
	assert.EqualValues(t, 1223, job.ID)
}

func TestRequestJob_NoRouter(t *testing.T) {
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
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { // matches everything else
		assert.Failf(t, "unexpected call", "%s %s", req.Method, req.URL)
		w.WriteHeader(http.StatusNotImplemented)
	}))
	server := httptest.NewServer(mux)
	defer server.Close()

	certDir := t.TempDir()
	rc := NewClient(
		network.NewGitLabClient(
			network.WithCertificateDirectory(certDir),
		),
		certDir,
		"runner-test",
	)
	defer rc.Shutdown()
	config := newConfig(server.URL)
	sessionInfo := &common.SessionInfo{}
	// Request 1
	job, healthy := rc.RequestJob(t.Context(), config, sessionInfo)
	require.True(t, healthy)
	require.NotNil(t, job)
	assert.EqualValues(t, 1223, job.ID) // just ensure we get what the server sent without testing the whole object as it may change.
	// Request 2 - should reuse the same connection without issues
	job, healthy = rc.RequestJob(t.Context(), config, sessionInfo)
	require.True(t, healthy)
	require.NotNil(t, job)
	assert.EqualValues(t, 1223, job.ID)
}

func TestRequestJob_JobRouterDisabled_FallsBackAndRediscovers(t *testing.T) {
	routerSrv := &mockRouterServer{t: t, disabled: true}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })
	r := grpc.NewServer()
	rpc.RegisterJobRouterServer(r, routerSrv)
	go func() { assert.NoError(t, r.Serve(l)) }()
	t.Cleanup(r.GracefulStop)

	discoveryJSON, err := json.Marshal(&common.RouterDiscovery{
		ServerURL: fmt.Sprintf("grpc://%s", l.Addr()),
	})
	require.NoError(t, err)

	var discoveryCalls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v4/runners/router/discovery", func(w http.ResponseWriter, req *http.Request) {
		// First discovery hands out the router; once it is disabled, discovery returns 501.
		if discoveryCalls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write(discoveryJSON)
			assert.NoError(t, writeErr)
			return
		}
		w.WriteHeader(http.StatusNotImplemented)
	})
	mux.HandleFunc("POST /api/v4/jobs/request", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, writeErr := w.Write([]byte(fakeJobResponse))
		assert.NoError(t, writeErr)
	})
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Failf(t, "unexpected call", "%s %s", req.Method, req.URL)
		w.WriteHeader(http.StatusNotImplemented)
	}))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	certDir := t.TempDir()
	rc := NewClient(
		network.NewGitLabClient(network.WithCertificateDirectory(certDir)),
		certDir,
		"runner-test",
	)
	t.Cleanup(rc.Shutdown)
	config := newConfig(server.URL)
	sessionInfo := &common.SessionInfo{}

	// Request 1: routes through the router, gets Unimplemented, and falls back to a direct job request.
	job, healthy := rc.RequestJob(t.Context(), config, sessionInfo)
	require.True(t, healthy)
	require.NotNil(t, job)
	assert.EqualValues(t, 1223, job.ID)
	assert.EqualValues(t, 1, routerSrv.calls.Load())
	// The discovery cache was invalidated, so the next request will re-discover.
	assert.True(t, rc.discoExpiresAt.IsZero())

	// Request 2: re-discovers, finds the router gone (501), and polls GitLab directly.
	job, healthy = rc.RequestJob(t.Context(), config, sessionInfo)
	require.True(t, healthy)
	require.NotNil(t, job)
	assert.EqualValues(t, 1223, job.ID)
	// The router was not used again.
	assert.EqualValues(t, 1, routerSrv.calls.Load())
	assert.EqualValues(t, 2, discoveryCalls.Load())
}

// installTestBreaker replaces the client's breaker with one matching production
// (threshold, open timeout, failure grace) but driven by a controllable clock,
// so tests can advance time to cross the grace and cooldown windows.
func installTestBreaker(rc *Client, now *time.Time) {
	rc.breaker = circuitbreaker.New(
		routerBreakerFailureThreshold,
		routerBreakerOpenTimeout,
		circuitbreaker.WithFailureGrace(routerBreakerFailureGrace),
		circuitbreaker.WithClock(func() time.Time { return *now }),
	)
}

func TestRequestJob_SustainedRouterFailure_TripsAndFallsBack(t *testing.T) {
	rc, config, routerSrv, railsCalls := setupRouterWithRailsFallback(t)
	now := time.Now()
	installTestBreaker(rc, &now)
	routerSrv.failCode.Store(int32(codes.Unavailable))
	sessionInfo := &common.SessionInfo{}

	// Failures within the grace window are transient: the runner stays healthy,
	// keeps polling the router, and does not fall back to Rails - even once the
	// failure count has passed the threshold.
	for i := 0; i < routerBreakerFailureThreshold+1; i++ {
		job, healthy := rc.RequestJob(t.Context(), config, sessionInfo)
		assert.Nil(t, job)
		assert.True(t, healthy)
	}
	assert.EqualValues(t, 0, railsCalls.Load(), "no fallback within the grace window")

	// Once the failures span the grace window, the next poll trips and falls back.
	now = now.Add(routerBreakerFailureGrace + time.Second)
	job, healthy := rc.RequestJob(t.Context(), config, sessionInfo)
	require.True(t, healthy)
	require.NotNil(t, job)
	assert.EqualValues(t, 1223, job.ID)
	assert.EqualValues(t, 1, railsCalls.Load())

	// While open, further polls go straight to Rails without contacting the router.
	callsWhileOpen := routerSrv.calls.Load()
	job, healthy = rc.RequestJob(t.Context(), config, sessionInfo)
	require.True(t, healthy)
	require.NotNil(t, job)
	assert.EqualValues(t, callsWhileOpen, routerSrv.calls.Load(), "the router is not contacted while the breaker is open")
	assert.EqualValues(t, 2, railsCalls.Load())
}

func TestRequestJob_CircuitBreakerRecoversAfterCooldown(t *testing.T) {
	rc, config, routerSrv, railsCalls := setupRouterWithRailsFallback(t)
	now := time.Now()
	installTestBreaker(rc, &now)
	routerSrv.failCode.Store(int32(codes.Unavailable))
	sessionInfo := &common.SessionInfo{}

	// Trip the breaker: drive failures past the grace window.
	for i := 0; i < routerBreakerFailureThreshold; i++ {
		rc.RequestJob(t.Context(), config, sessionInfo)
	}
	now = now.Add(routerBreakerFailureGrace + time.Second)
	rc.RequestJob(t.Context(), config, sessionInfo)
	routerCallsAtTrip := routerSrv.calls.Load()

	// Open: the next poll serves from Rails and leaves the router alone.
	rc.RequestJob(t.Context(), config, sessionInfo)
	assert.EqualValues(t, routerCallsAtTrip, routerSrv.calls.Load())

	// the router recovers and the open timeout elapses, so the next poll probes the router.
	routerSrv.failCode.Store(int32(codes.OK))
	now = now.Add(routerBreakerOpenTimeout + time.Second)
	railsBefore := railsCalls.Load()

	job, healthy := rc.RequestJob(t.Context(), config, sessionInfo)
	require.True(t, healthy)
	require.NotNil(t, job)
	assert.EqualValues(t, 1223, job.ID)
	assert.EqualValues(t, routerCallsAtTrip+1, routerSrv.calls.Load(), "the half-open probe reaches the router")
	assert.EqualValues(t, railsBefore, railsCalls.Load(), "a successful probe is served by the router, not Rails")

	// The breaker is closed again, so subsequent polls route through the router.
	rc.RequestJob(t.Context(), config, sessionInfo)
	assert.EqualValues(t, routerCallsAtTrip+2, routerSrv.calls.Load())
}

func TestRequestJob_CircuitBreakerRecoversDespiteEmptyDiscovery(t *testing.T) {
	routerSrv := &mockRouterServer{t: t}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })
	r := grpc.NewServer()
	rpc.RegisterJobRouterServer(r, routerSrv)
	go func() { assert.NoError(t, r.Serve(l)) }()
	t.Cleanup(r.GracefulStop)

	discoveryJSON, err := json.Marshal(&common.RouterDiscovery{ServerURL: fmt.Sprintf("grpc://%s", l.Addr())})
	require.NoError(t, err)

	var routerAvailable atomic.Bool
	routerAvailable.Store(true)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v4/runners/router/discovery", func(w http.ResponseWriter, req *http.Request) {
		if !routerAvailable.Load() {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write(discoveryJSON)
		assert.NoError(t, writeErr)
	})
	mux.HandleFunc("POST /api/v4/jobs/request", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, writeErr := w.Write([]byte(fakeJobResponse))
		assert.NoError(t, writeErr)
	})
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Failf(t, "unexpected call", "%s %s", req.Method, req.URL)
		w.WriteHeader(http.StatusNotImplemented)
	}))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	certDir := t.TempDir()
	rc := NewClient(network.NewGitLabClient(network.WithCertificateDirectory(certDir)), certDir, "runner-test")
	t.Cleanup(rc.Shutdown)
	now := time.Now()
	installTestBreaker(rc, &now)
	config := newConfig(server.URL)
	sessionInfo := &common.SessionInfo{}

	// Trip the breaker: drive failures past the grace window.
	routerSrv.failCode.Store(int32(codes.Unavailable))
	for i := 0; i < routerBreakerFailureThreshold; i++ {
		rc.RequestJob(t.Context(), config, sessionInfo)
	}
	now = now.Add(routerBreakerFailureGrace + time.Second)
	rc.RequestJob(t.Context(), config, sessionInfo)
	callsAfterTrip := routerSrv.calls.Load()

	// Cooldown elapses, but discovery now returns no router for the recovery poll.
	now = now.Add(routerBreakerOpenTimeout + time.Second)
	rc.invalidateRouterDiscovery()
	routerAvailable.Store(false)
	job, healthy := rc.RequestJob(t.Context(), config, sessionInfo)
	require.True(t, healthy)
	require.NotNil(t, job) // served from Rails
	assert.EqualValues(t, callsAfterTrip, routerSrv.calls.Load(), "the router is not dialed when discovery is empty")

	// Router and discovery recover: the breaker must still be able to probe and
	// resume routing (it would be stuck half-open if the empty-discovery poll had
	// consumed the trial request).
	routerAvailable.Store(true)
	routerSrv.failCode.Store(int32(codes.OK))
	rc.invalidateRouterDiscovery()
	job, healthy = rc.RequestJob(t.Context(), config, sessionInfo)
	require.True(t, healthy)
	require.NotNil(t, job)
	assert.EqualValues(t, callsAfterTrip+1, routerSrv.calls.Load(), "breaker recovered and routed through the router")
}

func TestIsRouterFailure(t *testing.T) {
	failures := []codes.Code{
		codes.Unavailable, codes.DeadlineExceeded, codes.Canceled,
	}
	for _, c := range failures {
		assert.True(t, isRouterFailure(c), c.String())
	}
	// Server-side and request-level errors are not breaker failures.
	ignored := []codes.Code{
		codes.OK, codes.Unimplemented, codes.Internal, codes.Unknown,
		codes.ResourceExhausted, codes.InvalidArgument, codes.NotFound,
		codes.PermissionDenied,
	}
	for _, c := range ignored {
		assert.False(t, isRouterFailure(c), c.String())
	}
}

// setupRouterWithRailsFallback starts a gRPC router server and a Rails server
// that serves both discovery and direct job requests, so tests can exercise the
// fallback path. The returned counter tracks direct Rails job requests.
func setupRouterWithRailsFallback(t *testing.T) (*Client, common.RunnerConfig, *mockRouterServer, *atomic.Int32) {
	routerSrv := &mockRouterServer{t: t}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })
	r := grpc.NewServer()
	rpc.RegisterJobRouterServer(r, routerSrv)
	go func() { assert.NoError(t, r.Serve(l)) }()
	t.Cleanup(r.GracefulStop)

	discoveryJSON, err := json.Marshal(&common.RouterDiscovery{
		ServerURL: fmt.Sprintf("grpc://%s", l.Addr()),
	})
	require.NoError(t, err)

	var railsCalls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v4/runners/router/discovery", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write(discoveryJSON)
		assert.NoError(t, writeErr)
	})
	mux.HandleFunc("POST /api/v4/jobs/request", func(w http.ResponseWriter, req *http.Request) {
		railsCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, writeErr := w.Write([]byte(fakeJobResponse))
		assert.NoError(t, writeErr)
	})
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Failf(t, "unexpected call", "%s %s", req.Method, req.URL)
		w.WriteHeader(http.StatusNotImplemented)
	}))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	certDir := t.TempDir()
	rc := NewClient(
		network.NewGitLabClient(network.WithCertificateDirectory(certDir)),
		certDir,
		"runner-test",
	)
	t.Cleanup(rc.Shutdown)
	return rc, newConfig(server.URL), routerSrv, &railsCalls
}

func setupWithRouter(t *testing.T, withTLS bool) (*Client, string) {
	certDir := ""
	var l net.Listener
	var err error
	routerProtocol := "grpc"
	var serverOpts []grpc.ServerOption
	if withTLS {
		certDir = t.TempDir()
		routerProtocol = "grpcs"
		gen := certificate.X509Generator{}

		caCertPEM, _, certTyped, caPrivateKey, err := gen.GenerateCA()
		require.NoError(t, err)

		cert, _, err := gen.GenerateWithCA("127.0.0.1", certTyped, caPrivateKey)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(certDir, "127.0.0.1.crt"), caCertPEM, 0600)
		require.NoError(t, err)

		serverOpts = append(serverOpts, grpc.Creds(credentials.NewServerTLSFromCert(&cert)))
	}
	l, err = net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })

	r := grpc.NewServer(serverOpts...)
	rpc.RegisterJobRouterServer(r, &mockRouterServer{t: t})
	go func() {
		assert.NoError(t, r.Serve(l))
	}()
	t.Cleanup(r.GracefulStop)

	discoveryJSON, err := json.Marshal(&common.RouterDiscovery{
		ServerURL: fmt.Sprintf("%s://%s", routerProtocol, l.Addr()),
	})
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v4/runners/router/discovery", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write(discoveryJSON)
		assert.NoError(t, writeErr)
	})
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { // matches everything else
		assert.Failf(t, "unexpected call", "%s %s", req.Method, req.URL)
		w.WriteHeader(http.StatusNotImplemented)
	}))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	rc := NewClient(
		network.NewGitLabClient(
			network.WithCertificateDirectory(certDir),
		),
		certDir,
		"runner-test",
	)
	t.Cleanup(rc.Shutdown)
	return rc, server.URL
}

func newConfig(serverURL string) common.RunnerConfig {
	return common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			URL:             serverURL,
			ID:              42,
			Token:           "glrt-123123123",
			TokenObtainedAt: time.Now(),
			TokenExpiresAt:  time.Now().Add(time.Hour),
			TLSCAFile:       "",
			TLSCertFile:     "",
			TLSKeyFile:      "",
			Logger:          logrus.New(),
		},
		RunnerSettings: common.RunnerSettings{
			FeatureFlags: map[string]bool{
				featureflags.UseJobRouter: true,
			},
		},
	}
}

type mockRouterServer struct {
	rpc.UnsafeJobRouterServer
	t        *testing.T
	disabled bool         // when true, reports the Job Router as disabled, mirroring the router when the job_router flag is off
	failCode atomic.Int32 // when non-zero, every GetJob fails with this gRPC code, simulating a router outage
	calls    atomic.Int32
}

func (s *mockRouterServer) GetJob(ctx context.Context, req *rpc.GetJobRequest) (*rpc.GetJobResponse, error) {
	s.calls.Add(1)

	if s.disabled {
		return nil, status.Error(codes.Unimplemented, "Job Router is not available. Please contact your administrator.")
	}

	if code := codes.Code(s.failCode.Load()); code != codes.OK {
		return nil, status.Error(code, "simulated router failure")
	}

	assert.NotEmpty(s.t, metadata.ValueFromIncomingContext(ctx, requestIDMetadataKey))
	assert.NoError(s.t, grpc.SetHeader(ctx, metadata.Pairs(
		requestIDMetadataKey, responseRequestID,
	)))
	return &rpc.GetJobResponse{
		JobResponse: []byte(fakeJobResponse),
	}, nil
}
