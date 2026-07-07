//go:build !integration

package instance

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

// captureTrace is a buildlogger.Trace that records everything written to it,
// letting tests assert on warnings emitted via BuildLogger. It is safe for
// concurrent use as the buildlogger may write from multiple goroutines.
type captureTrace struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (t *captureTrace) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.buf.Write(p)
}

func (t *captureTrace) IsStdout() bool { return true }

func (t *captureTrace) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.buf.String()
}

func newTestStepsExecutor(jobID int64, client executors.Client) *executor {
	e := &executor{client: client}
	e.ExecutorOptions.Shell = common.ShellScriptInfo{
		Shell:         "bash",
		RunnerCommand: "gitlab-runner",
	}
	e.Build = &common.Build{Job: spec.Job{ID: jobID}}
	e.BuildLogger = buildlogger.New(nil, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})
	return e
}

// newTestStepsExecutorWithTrace is like newTestStepsExecutor but routes the
// BuildLogger output to the returned trace so warnings can be asserted on.
func newTestStepsExecutorWithTrace(jobID int64, client executors.Client) (*executor, *captureTrace) {
	e := newTestStepsExecutor(jobID, client)
	trace := &captureTrace{}
	e.BuildLogger = buildlogger.New(trace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})
	return e, trace
}

// versionOutput mimics `gitlab-runner --version` for the given version, on a
// linux/amd64 instance.
func versionOutput(version string) string {
	return fmt.Sprintf("Version:      %s\nGit revision: abcdef\nOS/Arch:      linux/amd64\n", version)
}

// runHandler routes the mocked client.Run calls: `--version` returns the given
// version output, `steps serve` defers to the serve callback.
func runHandler(version string, serve func(ctx context.Context, opts executors.RunOptions) error) func(context.Context, executors.RunOptions) error {
	return func(ctx context.Context, opts executors.RunOptions) error {
		switch {
		case strings.Contains(opts.Command, "--version"):
			if opts.Stdout != nil {
				_, _ = io.WriteString(opts.Stdout, versionOutput(version))
			}
			return nil
		case strings.Contains(opts.Command, "steps serve"):
			return serve(ctx, opts)
		default:
			return fmt.Errorf("unexpected command: %s", opts.Command)
		}
	}
}

func TestConnect(t *testing.T) {
	const jobID int64 = 42
	// step-runner binds the socket inside the mktemp directory and reports the
	// path back; the connector uses whatever it reports for the proxy.
	const reportedSocket = "/tmp/step-runner.ab12cd34/step-runner.sock"

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client := executors.NewMockClient(t)

	// serveDone closes when the serve command's Run returns, letting us assert
	// that closing the connection actually terminates step-runner.
	serveDone := make(chan struct{})
	var serveCommand string
	client.EXPECT().Run(mock.Anything, mock.Anything).RunAndReturn(
		runHandler(common.AppVersion.Version, func(ctx context.Context, opts executors.RunOptions) error {
			serveCommand = opts.Command
			fmt.Fprintf(opts.Stdout, "step-runner is listening on socket %s\n", reportedSocket)
			// Emulate the remote `cat` keepalive: block until stdin reaches EOF
			// (the write end is closed) or the context is cancelled.
			defer close(serveDone)
			_, _ = io.Copy(io.Discard, opts.Stdin)
			return nil
		}),
	)

	dial, err := newTestStepsExecutor(jobID, client).Connect(ctx)
	require.NoError(t, err)
	require.NotNil(t, dial)

	// serve creates a private socket directory with mktemp -d and runs
	// step-runner in the background tied to a stdin keepalive.
	assert.Contains(t, serveCommand, "mktemp -d /tmp/step-runner.XXXXXXXX")
	assert.Contains(t, serveCommand, `steps serve --socket "$dir/step-runner.sock"`)
	// After signalling step-runner with `kill`, the shell `wait`s for it so it is
	// reaped and has unbound its socket before the EXIT trap removes the directory.
	assert.Contains(t, serveCommand, "wait $srv")
	// The cleanup trap also fires on uncaught signals (not just EXIT), so a
	// SIGHUP/SIGTERM from SSH-channel teardown still kills step-runner and removes
	// the directory instead of orphaning the process.
	assert.Contains(t, serveCommand, "EXIT INT TERM HUP")
	assert.Contains(t, serveCommand, `trap 'kill ${srv:-} 2>/dev/null; rm -rf "$dir"'`)

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()

	var proxyCommand string
	client.EXPECT().DialRun(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, command string) (net.Conn, error) {
			proxyCommand = command
			return clientConn, nil
		},
	).Once()

	conn, err := dial()
	require.NoError(t, err)
	// The proxy dials the socket path step-runner reported, not a precomputed one.
	assert.Contains(t, proxyCommand, "steps proxy --socket '"+reportedSocket+"'")

	// Closing the returned connection must terminate step-runner by closing the
	// serve command's stdin (EOF), so the serve Run returns.
	require.NoError(t, conn.Close())
	select {
	case <-serveDone:
	case <-time.After(5 * time.Second):
		t.Fatal("closing the connection did not terminate step-runner")
	}
}

func TestConnect_serveExitsBeforeReady(t *testing.T) {
	client := executors.NewMockClient(t)
	client.EXPECT().Run(mock.Anything, mock.Anything).RunAndReturn(
		runHandler(common.AppVersion.Version, func(ctx context.Context, opts executors.RunOptions) error {
			return fmt.Errorf("boom")
		}),
	)

	dial, err := newTestStepsExecutor(1, client).Connect(t.Context())
	require.Error(t, err)
	assert.Nil(t, dial)
	assert.ErrorContains(t, err, "step-runner serve exited before becoming ready")
}

func TestConnect_serveExitsCleanlyBeforeReady(t *testing.T) {
	client := executors.NewMockClient(t)
	client.EXPECT().Run(mock.Anything, mock.Anything).RunAndReturn(
		// serve returns a nil error (clean exit) without ever emitting the ready
		// marker. The connector must report the plain error rather than wrapping a
		// nil with %w, which would render as "%!w(<nil>)".
		runHandler(common.AppVersion.Version, func(ctx context.Context, opts executors.RunOptions) error {
			return nil
		}),
	)

	dial, err := newTestStepsExecutor(1, client).Connect(t.Context())
	require.Error(t, err)
	assert.Nil(t, dial)
	assert.ErrorContains(t, err, "step-runner serve exited before becoming ready")
	assert.NotContains(t, err.Error(), "%!w", "nil error must not be wrapped with %%w")
	assert.NotContains(t, err.Error(), "<nil>", "nil error must not leak into the message")
}

func TestConnect_noRunnerCommand(t *testing.T) {
	e := newTestStepsExecutor(1, executors.NewMockClient(t))
	e.ExecutorOptions.Shell.RunnerCommand = ""

	dial, err := e.Connect(t.Context())
	require.Error(t, err)
	assert.Nil(t, dial)
}

func TestConnect_missingRunnerBinary(t *testing.T) {
	client := executors.NewMockClient(t)
	client.EXPECT().Run(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, opts executors.RunOptions) error {
			return fmt.Errorf("sh: gitlab-runner: command not found")
		},
	).Once()

	dial, err := newTestStepsExecutor(1, client).Connect(t.Context())
	require.Error(t, err)
	assert.Nil(t, dial)
	assert.ErrorContains(t, err, "gitlab-runner is installed on the instance image")
}

func TestConnect_windowsInstance(t *testing.T) {
	client := executors.NewMockClient(t)
	client.EXPECT().Run(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, opts executors.RunOptions) error {
			fmt.Fprintf(opts.Stdout, "Version:      %s\nOS/Arch:      windows/amd64\n", common.AppVersion.Version)
			return nil
		},
	).Once()

	dial, err := newTestStepsExecutor(1, client).Connect(t.Context())
	require.Error(t, err)
	assert.Nil(t, dial)
	assert.ErrorContains(t, err, "not supported on windows")
}

func TestConnect_versionMismatchWarnsButProceeds(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client := executors.NewMockClient(t)
	client.EXPECT().Run(mock.Anything, mock.Anything).RunAndReturn(
		runHandler("0.0.0-different", func(ctx context.Context, opts executors.RunOptions) error {
			fmt.Fprintf(opts.Stdout, "step-runner is listening on socket /tmp/step-runner-1/step-runner.sock\n")
			<-ctx.Done()
			return ctx.Err()
		}),
	)

	dial, err := newTestStepsExecutor(1, client).Connect(ctx)
	require.NoError(t, err)
	assert.NotNil(t, dial)
}

func TestConnect_contextCancelledBeforeReady(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	client := executors.NewMockClient(t)
	started := make(chan struct{})
	client.EXPECT().Run(mock.Anything, mock.Anything).RunAndReturn(
		runHandler(common.AppVersion.Version, func(ctx context.Context, opts executors.RunOptions) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}),
	)

	errCh := make(chan error, 1)
	go func() {
		_, err := newTestStepsExecutor(1, client).Connect(ctx)
		errCh <- err
	}()

	<-started
	cancel()

	select {
	case err := <-errCh:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Connect did not return after context cancellation")
	}
}

func TestConnect_missingSocketPath(t *testing.T) {
	client := executors.NewMockClient(t)
	client.EXPECT().Run(mock.Anything, mock.Anything).RunAndReturn(
		runHandler(common.AppVersion.Version, func(ctx context.Context, opts executors.RunOptions) error {
			// step-runner emits the ready marker but advertises an empty socket
			// path (e.g. an incompatible version), so the proxy has nothing to dial.
			fmt.Fprintf(opts.Stdout, "step-runner is listening on socket \n")
			<-ctx.Done()
			return ctx.Err()
		}),
	)

	dial, err := newTestStepsExecutor(1, client).Connect(t.Context())
	require.Error(t, err)
	assert.Nil(t, dial)
	assert.ErrorContains(t, err, "missing socket path")
}

func TestConnect_dialError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client := executors.NewMockClient(t)
	client.EXPECT().Run(mock.Anything, mock.Anything).RunAndReturn(
		runHandler(common.AppVersion.Version, func(ctx context.Context, opts executors.RunOptions) error {
			fmt.Fprintf(opts.Stdout, "step-runner is listening on socket /tmp/step-runner-1/step-runner.sock\n")
			<-ctx.Done()
			return ctx.Err()
		}),
	)
	client.EXPECT().DialRun(mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("dial failed")).Once()

	dial, err := newTestStepsExecutor(1, client).Connect(ctx)
	require.NoError(t, err)

	conn, err := dial()
	assert.Nil(t, conn)
	require.Error(t, err)
	assert.ErrorContains(t, err, "dialing step-runner proxy")
	assert.ErrorContains(t, err, "dial failed")
}

type errCloser struct {
	io.ReadWriteCloser
	err error
}

func (c errCloser) Close() error { return c.err }

func TestServeConn_ClosePropagatesErrorAndStopsServe(t *testing.T) {
	stopR, stopW := io.Pipe()
	sentinel := fmt.Errorf("close failed")
	c := &serveConn{ReadWriteCloser: errCloser{err: sentinel}, stop: stopW}

	require.ErrorIs(t, c.Close(), sentinel)

	// stop pipe must have been closed (serve stdin EOF), so a read returns EOF.
	_, err := stopR.Read(make([]byte, 1))
	assert.ErrorIs(t, err, io.EOF)
}

func TestShellQuote(t *testing.T) {
	tests := map[string]string{
		"":                  "''",
		"gitlab-runner":     "'gitlab-runner'",
		"/tmp/my runner/gr": "'/tmp/my runner/gr'",
		"it's":              `'it'\''s'`,
		"a'b'c":             `'a'\''b'\''c'`,
	}
	for in, want := range tests {
		assert.Equalf(t, want, shellQuote(in), "shellQuote(%q)", in)
	}
}

func TestParseRunnerVersion(t *testing.T) {
	tests := map[string]struct {
		output      string
		wantVersion string
		wantOSArch  string
	}{
		"full output": {
			output:      "Version:      17.5.0\nGit revision: abcdef\nOS/Arch:      linux/arm64\n",
			wantVersion: "17.5.0",
			wantOSArch:  "linux/arm64",
		},
		"version only": {
			output:      "Version:      1.2.3\n",
			wantVersion: "1.2.3",
		},
		"os/arch only": {
			output:     "OS/Arch:      windows/amd64\n",
			wantOSArch: "windows/amd64",
		},
		"no space after prefix": {
			output:      "Version:1.2.3\n",
			wantVersion: "1.2.3",
		},
		"duplicate version lines, last wins": {
			output:      "Version:      1.0.0\nVersion:      2.0.0\n",
			wantVersion: "2.0.0",
		},
		"surrounding whitespace trimmed": {
			output:      "Version:\t  1.2.3  \t\nOS/Arch:\tlinux/amd64\t\n",
			wantVersion: "1.2.3",
			wantOSArch:  "linux/amd64",
		},
		"duplicate os/arch lines, last wins": {
			output:     "OS/Arch:      linux/amd64\nOS/Arch:      darwin/arm64\n",
			wantOSArch: "darwin/arm64",
		},
		"no trailing newline": {
			output:      "Version:      1.2.3",
			wantVersion: "1.2.3",
		},
		"crlf line endings keep carriage return": {
			// parseRunnerVersion splits on "\n" only; a trailing \r is trimmed by
			// TrimSpace, so CRLF output still parses cleanly.
			output:      "Version:      1.2.3\r\nOS/Arch:      linux/amd64\r\n",
			wantVersion: "1.2.3",
			wantOSArch:  "linux/amd64",
		},
		"unrelated lines ignored": {
			output: "Name:         gitlab-runner\nGit revision: abcdef\n",
		},
		"empty": {},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			version, osArch := parseRunnerVersion(tt.output)
			assert.Equal(t, tt.wantVersion, version)
			assert.Equal(t, tt.wantOSArch, osArch)
		})
	}
}

// versionWarning is the distinctive fragment of the version-mismatch warning.
const versionWarning = "differs from the manager version"

func TestVerifyRunnerBinary_devManagerDoesNotWarn(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client := executors.NewMockClient(t)
	e, trace := newTestStepsExecutorWithTrace(1, client)

	// Manager reports a dev build; the guard must skip the warning regardless of
	// the (necessarily different) instance version.
	restore := common.AppVersion.Version
	common.AppVersion.Version = "development version"
	defer func() { common.AppVersion.Version = restore }()

	client.EXPECT().Run(mock.Anything, mock.Anything).RunAndReturn(
		runHandler("17.5.0", func(ctx context.Context, opts executors.RunOptions) error {
			fmt.Fprintf(opts.Stdout, "step-runner is listening on socket /tmp/step-runner-1/step-runner.sock\n")
			<-ctx.Done()
			return ctx.Err()
		}),
	)

	dial, err := e.Connect(ctx)
	require.NoError(t, err)
	require.NotNil(t, dial)
	assert.NotContains(t, trace.String(), versionWarning, "dev-build manager must not emit a version-mismatch warning")
}

func TestVerifyRunnerBinary_matchingVersionDoesNotWarn(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client := executors.NewMockClient(t)
	e, trace := newTestStepsExecutorWithTrace(1, client)

	// Pin both manager and instance to the same real version.
	restore := common.AppVersion.Version
	common.AppVersion.Version = "17.5.0"
	defer func() { common.AppVersion.Version = restore }()

	client.EXPECT().Run(mock.Anything, mock.Anything).RunAndReturn(
		runHandler("17.5.0", func(ctx context.Context, opts executors.RunOptions) error {
			fmt.Fprintf(opts.Stdout, "step-runner is listening on socket /tmp/step-runner-1/step-runner.sock\n")
			<-ctx.Done()
			return ctx.Err()
		}),
	)

	dial, err := e.Connect(ctx)
	require.NoError(t, err)
	require.NotNil(t, dial)
	assert.NotContains(t, trace.String(), versionWarning, "matching versions must not warn")
}

func TestVerifyRunnerBinary_differingRealVersionsWarn(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client := executors.NewMockClient(t)
	e, trace := newTestStepsExecutorWithTrace(1, client)

	// Two differing real (non-dev) versions must produce a warning.
	restore := common.AppVersion.Version
	common.AppVersion.Version = "17.5.0"
	defer func() { common.AppVersion.Version = restore }()

	client.EXPECT().Run(mock.Anything, mock.Anything).RunAndReturn(
		runHandler("17.4.0", func(ctx context.Context, opts executors.RunOptions) error {
			fmt.Fprintf(opts.Stdout, "step-runner is listening on socket /tmp/step-runner-1/step-runner.sock\n")
			<-ctx.Done()
			return ctx.Err()
		}),
	)

	dial, err := e.Connect(ctx)
	require.NoError(t, err)
	require.NotNil(t, dial)

	out := trace.String()
	assert.Contains(t, out, versionWarning)
	assert.Contains(t, out, "17.4.0", "warning should name the instance version")
	assert.Contains(t, out, "17.5.0", "warning should name the manager version")
}

func TestConnect_readyChannelClosed(t *testing.T) {
	client := executors.NewMockClient(t)
	client.EXPECT().Run(mock.Anything, mock.Anything).RunAndReturn(
		runHandler(common.AppVersion.Version, func(ctx context.Context, opts executors.RunOptions) error {
			// Emit the ready marker followed by an over-long socket path with no
			// terminating newline: readywriter aborts and closes the ready
			// channel without ever sending a socket, exercising the !ok path.
			fmt.Fprint(opts.Stdout, "step-runner is listening on socket ")
			fmt.Fprint(opts.Stdout, strings.Repeat("a", 5*1024))
			<-ctx.Done()
			return ctx.Err()
		}),
	)

	dial, err := newTestStepsExecutor(1, client).Connect(t.Context())
	require.Error(t, err)
	assert.Nil(t, dial)
	assert.ErrorContains(t, err, "step-runner ready channel closed")
}

func TestVerifyRunnerBinary_versionRunBoundedByTimeout(t *testing.T) {
	client := executors.NewMockClient(t)
	// `--version` Run hangs on its own context. verifyRunnerBinary wraps ctx in a
	// 60s timeout, so the hung Run must observe a cancelled context (the wrapped
	// one) rather than blocking on the never-cancelled parent ctx.
	client.EXPECT().Run(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, opts executors.RunOptions) error {
			<-ctx.Done()
			return ctx.Err()
		},
	).Once()

	e := newTestStepsExecutor(1, client)

	// verifyRunnerBinary derives its own context (context.WithTimeout) from the
	// passed-in ctx, so a hung Run is bounded rather than blocking forever. We
	// keep the test fast by cancelling the parent (the derived ctx is its child,
	// so it is cancelled too) instead of waiting the full 60s; cancelling proves
	// the Run is context-bounded.
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)
	go func() {
		done <- e.verifyRunnerBinary(ctx, "gitlab-runner")
	}()

	cancel()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.ErrorContains(t, err, "could not be run")
	case <-time.After(5 * time.Second):
		t.Fatal("verifyRunnerBinary did not return; --version Run is not context-bounded")
	}
}

func TestVerifyRunnerBinary_concurrentStdoutStderrWrites(t *testing.T) {
	client := executors.NewMockClient(t)
	// Emulate the SSH connector draining Stdout and Stderr from separate
	// goroutines. If verifyRunnerBinary shared a single bytes.Buffer for both,
	// this would trip the race detector. Distinct buffers must keep -race clean.
	client.EXPECT().Run(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, opts executors.RunOptions) error {
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				for i := 0; i < 100; i++ {
					_, _ = io.WriteString(opts.Stdout, versionOutput(common.AppVersion.Version))
				}
			}()
			go func() {
				defer wg.Done()
				for i := 0; i < 100; i++ {
					_, _ = io.WriteString(opts.Stderr, "some stderr noise\n")
				}
			}()
			wg.Wait()
			return nil
		},
	).Once()

	err := newTestStepsExecutor(1, client).verifyRunnerBinary(t.Context(), "gitlab-runner")
	require.NoError(t, err)
}
