//go:build !integration

package shell

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestProvider_AdvertisesNativeSteps(t *testing.T) {
	var features common.FeaturesInfo
	require.NoError(t, NewProvider("gitlab-runner").GetFeatures(&features))

	// Advertised unconditionally; run: without FF_CONCRETE is rejected at
	// execution by Build.executeScript.
	assert.True(t, features.NativeStepsIntegration, "NativeStepsIntegration")
	assert.True(t, features.NativeStepsViaConcreteOnly,
		"shell runs native steps only via concrete, so it must declare "+
			"NativeStepsViaConcreteOnly for the early build-level gate")
}

// dialable reports whether the unix socket at path currently accepts a
// connection.
func dialable(t *testing.T, path string) bool {
	t.Helper()
	conn, err := net.Dial("unix", path)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func TestStepRunnerServer_LazyStartSharedAndShutdown(t *testing.T) {
	srv := newStepRunnerServer()
	t.Cleanup(func() { srv.shutdown(context.Background()) }) //nolint:usetesting // t.Context() is already canceled at Cleanup time; a fresh Background context is required

	// Lazily starts on first use and listens on a generated, non-default socket.
	sock, err := srv.ensureStarted()
	require.NoError(t, err)
	require.NotEmpty(t, sock)
	require.NotEqual(t, "step-runner.sock", filepath.Base(sock),
		"socket must be auto-generated, not the well-known default")
	require.True(t, dialable(t, sock), "server should be accepting connections after ensureStarted")

	sockDir := filepath.Dir(sock)
	require.DirExists(t, sockDir)

	// Shared: a second call returns the same socket without respawning.
	sock2, err := srv.ensureStarted()
	require.NoError(t, err)
	require.Equal(t, sock, sock2)

	// Graceful shutdown stops the server and removes its socket directory.
	srv.shutdown(t.Context())
	require.NoDirExists(t, sockDir)
	require.False(t, dialable(t, sock), "server should no longer accept connections after shutdown")
}

func TestStepRunnerServer_ShutdownBeforeStartIsNoop(t *testing.T) {
	srv := newStepRunnerServer()
	// Must not panic or block when the server was never started.
	srv.shutdown(t.Context())
}

func TestStepRunnerServer_RespawnsAfterUnexpectedExit(t *testing.T) {
	srv := newStepRunnerServer()
	t.Cleanup(func() { srv.shutdown(context.Background()) }) //nolint:usetesting // t.Context() is already canceled at Cleanup time; a fresh Background context is required

	sock1, err := srv.ensureStarted()
	require.NoError(t, err)

	// Simulate the server exiting out from under the wrapper (e.g. a crash):
	// stop it directly without going through shutdown, so the wrapper still
	// believes it is running.
	srv.mu.Lock()
	srv.srv.Stop()
	srv.mu.Unlock()

	// Once ensureStarted observes the exit it must respawn a fresh server on
	// a new socket. Until the server goroutine actually returns, ensureStarted
	// still reports the old socket, so poll.
	var sock2 string
	require.Eventually(t, func() bool {
		var err error
		sock2, err = srv.ensureStarted()
		return err == nil && sock2 != sock1
	}, 10*time.Second, 20*time.Millisecond,
		"a crashed server should be respawned on a new socket")
	require.True(t, dialable(t, sock2))
}

func TestExecutorConnect_DialsSharedServer(t *testing.T) {
	srv := newStepRunnerServer()
	t.Cleanup(func() { srv.shutdown(context.Background()) }) //nolint:usetesting // t.Context() is already canceled at Cleanup time; a fresh Background context is required

	e := &executor{stepRunner: srv}

	dial, err := e.Connect(t.Context())
	require.NoError(t, err)
	require.NotNil(t, dial)

	// The dialer yields a working, full-duplex connection to the server...
	conn, err := dial()
	require.NoError(t, err)
	require.NotNil(t, conn)
	_ = conn.Close()

	// ...and is reusable: each call returns a fresh, independent connection.
	conn2, err := dial()
	require.NoError(t, err)
	require.NotNil(t, conn2)
	assert.NotSame(t, conn, conn2)
	_ = conn2.Close()
}
