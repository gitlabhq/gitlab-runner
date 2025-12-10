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
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/certificate"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/network"
	"gitlab.com/gitlab-org/gitlab-runner/router/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
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
	t *testing.T
}

func (s *mockRouterServer) GetJob(ctx context.Context, req *rpc.GetJobRequest) (*rpc.GetJobResponse, error) {
	assert.NotEmpty(s.t, metadata.ValueFromIncomingContext(ctx, requestIDMetadataKey))
	assert.NoError(s.t, grpc.SetHeader(ctx, metadata.Pairs(
		requestIDMetadataKey, responseRequestID,
	)))
	return &rpc.GetJobResponse{
		JobResponse: []byte(fakeJobResponse),
	}, nil
}
