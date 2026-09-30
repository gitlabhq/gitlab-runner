//go:build !integration

package localserver

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/step-runner/proto"
)

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

// Start must not return until the service is registered and serving: a bound
// socket alone accepts connections into the listen backlog, so an RPC issued
// the instant Start returns would otherwise hang or fail Unimplemented.
func TestStartReturnsOnlyOnceServingRPCs(t *testing.T) {
	srv, err := Start(Options{})
	require.NoError(t, err)
	t.Cleanup(func() {
		srv.Stop()
		<-srv.Done()
	})

	conn, err := srv.Dial()
	require.NoError(t, err)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	_, err = proto.NewStepRunnerClient(conn).Status(ctx, &proto.StatusRequest{})
	require.NoError(t, err, "the service must answer RPCs as soon as Start returns")
}

func TestStartStopRemovesSocketDir(t *testing.T) {
	srv, err := Start(Options{})
	require.NoError(t, err)

	require.NotEmpty(t, srv.SockPath)
	require.NotEqual(t, "step-runner.sock", filepath.Base(srv.SockPath),
		"socket must be auto-generated, not the well-known default")
	require.True(t, dialable(t, srv.SockPath),
		"server should be accepting connections after Start")

	dir := filepath.Dir(srv.SockPath)
	require.DirExists(t, dir)

	srv.Stop()
	select {
	case err := <-srv.Done():
		require.NoError(t, err, "graceful stop should not surface an error")
	case <-time.After(10 * time.Second):
		t.Fatal("server did not exit after Stop")
	}

	// serve owns the directory and removes it on the way out, so it is gone
	// by the time its result lands on Done.
	require.NoDirExists(t, dir)
	require.False(t, dialable(t, srv.SockPath),
		"server should no longer accept connections after Stop")
}

// A deep $TMPDIR (e.g. the custom executor exports its own temp dir, nested
// under macOS's already-long /var/folders path) must not push the socket path
// past the sun_path limit; Start falls back to /tmp.
func TestStartFallsBackWhenTempDirTooLongForSocket(t *testing.T) {
	deep := filepath.Join(t.TempDir(), strings.Repeat("d", 60), strings.Repeat("e", 60))
	require.NoError(t, os.MkdirAll(deep, 0o700))
	t.Setenv("TMPDIR", deep)

	srv, err := Start(Options{})
	require.NoError(t, err, "Start must survive an over-long $TMPDIR")
	t.Cleanup(func() {
		srv.Stop()
		<-srv.Done()
	})

	assert.LessOrEqual(t, len(srv.SockPath), maxSockPathLen)
	assert.NotContains(t, srv.SockPath, deep, "socket must not live under the over-long TMPDIR")
	require.True(t, dialable(t, srv.SockPath))
}
