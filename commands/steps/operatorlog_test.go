//go:build !integration

package steps_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/commands/steps"
	runnersteps "gitlab.com/gitlab-org/gitlab-runner/steps"
	schema "gitlab.com/gitlab-org/step-runner/schema/v1"
)

// sockDialer is a minimal steps.Connector that dials the server's unix socket.
type sockDialer string

func (d sockDialer) Connect(context.Context) (func() (io.ReadWriteCloser, error), error) {
	return func() (io.ReadWriteCloser, error) { return net.Dial("unix", string(d)) }, nil
}

// TestServe_OperatorLogsDoNotLeakToDefault checks that a failing job's operator
// logs don't reach slog's default sink, which the docker executor tees into the
// job trace (executors/docker/steps.go).
func TestServe_OperatorLogsDoNotLeakToDefault(t *testing.T) {
	// The default sink is the docker container stderr that's teed into the trace.
	var dflt bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&dflt, nil)))
	defer slog.SetDefault(prev)

	sock := filepath.Join(shortTempDir(t), "s.sock")
	ctx := t.Context()

	go func() { _ = steps.Serve(ctx, sock, steps.IOStreams{}) }()
	require.Eventually(t, func() bool {
		if c, err := net.Dial("unix", sock); err == nil {
			_ = c.Close()
			return true
		}
		return false
	}, waitDeadline, waitTick, "server never started listening")

	name, script := "boom", "exit 7"
	var trace bytes.Buffer
	err := runnersteps.Execute(ctx, runnersteps.Options{
		Connector: sockDialer(sock),
		JobInfo:   runnersteps.JobInfo{ID: 1, Timeout: time.Minute, ProjectDir: t.TempDir()},
		Steps:     []schema.Step{{Name: &name, Script: &script}},
		Trace:     &trace,
		Log:       logrus.WithField("test", "operatorlog"),
	})
	require.Error(t, err, "a failing step must surface as an error")

	// With the operator logger discarded, nothing should reach slog's default
	// sink. Assert it's empty rather than matching a specific message, so the
	// test still catches leaks if step-runner changes its operator log text.
	assert.Empty(t, dflt.String(),
		"no logs should reach slog's default sink (docker tees it into the job trace)")
}
