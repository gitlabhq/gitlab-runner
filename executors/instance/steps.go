package instance

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/internal/readywriter"
	"gitlab.com/gitlab-org/gitlab-runner/steps"
)

var _ steps.Connector = (*executor)(nil)

// Connect starts the step-runner on the instance and returns a dialer that
// connects to it, mirroring the docker executor's integration in
// executors/docker/steps.go.
//
// Unlike docker - where each job gets its own container filesystem - the
// instance executor runs jobs directly on a shared VM filesystem and supports
// CapacityPerInstance > 1 (multiple concurrent jobs per VM). We therefore place
// the socket in a private directory created with `mktemp -d`, so concurrent jobs
// on the same VM cannot collide or cross-talk and other OS users cannot reach or
// squat on the path. step-runner binds the socket there and reports the path it
// is listening on, which we use for the proxy.
func (e *executor) Connect(ctx context.Context) (func() (io.ReadWriteCloser, error), error) {
	// The gitlab-runner binary (which carries the `steps` subcommands) is
	// assumed to be present on the instance image in the executor's runner
	// command path, as documented for the instance executor. Provisioning the
	// binary onto the instance is tracked separately.
	runnerCommand := e.Shell().RunnerCommand
	if runnerCommand == "" {
		return nil, fmt.Errorf("native steps require a runner command, none configured for this executor")
	}

	// Verify the instance has a usable gitlab-runner binary before starting
	// step-runner: error clearly when it is missing or running on an
	// unsupported OS, and warn when its version differs from the manager's.
	if err := e.verifyRunnerBinary(ctx, runnerCommand); err != nil {
		return nil, err
	}

	// The connector cannot stop a remote process directly (unlike the docker
	// executor, which stops the build container). We therefore tie step-runner's
	// lifetime to the serve command's stdin: step-runner runs in the background
	// while a no-op `cat` holds stdin open; when we close the stdin pipe - on
	// connection teardown or context cancellation - `cat` reaches EOF and
	// step-runner is signalled to stop. We `wait` for it after `kill` so it is
	// reaped and has unbound its socket before the EXIT trap removes the
	// directory, avoiding a teardown race.
	//
	// `mktemp -d` atomically creates a fresh, unpredictably-named directory with
	// mode 0700. On a shared instance (capacity_per_instance > 1) or a host with
	// other OS users, this prevents another user from reaching, pre-creating, or
	// squatting on the socket path: unlike a predictable per-job name there is
	// nothing to guess and no `rm -rf` of an attacker-influenceable path. If the
	// directory cannot be created we exit rather than block on `cat`, so the
	// failure surfaces instead of hanging. The /tmp template keeps the path
	// short, staying within the unix socket sun_path length limit (~108 bytes).
	//
	// The trap kills step-runner and removes the directory on EXIT and on
	// SIGINT/SIGTERM/SIGHUP. Catching the signals matters because bash runs the
	// EXIT trap on a normal exit and on trapped signals, but an *uncaught* signal
	// (e.g. SIGHUP when the SSH channel is torn down on job cancellation) would
	// otherwise terminate the shell without cleanup, orphaning the backgrounded
	// step-runner and leaking its directory. `${srv:-}` guards the window before
	// step-runner has started. The residual case the trap cannot cover is the
	// manager being SIGKILLed with the SSH connection severed so no signal or
	// stdin EOF ever reaches the shell; that is inherent to launching a remote
	// daemon over SSH.
	//
	// step-runner binds the socket inside that directory and reports the path it
	// is listening on via its ready marker (read from readyCh below). It prints
	// the marker to stderr; redirect it to stdout (2>&1) so the marker lands on a
	// single stream regardless of whether the connector separates stdout/stderr
	// (some, e.g. the SSH stub, merge them).
	serveCommand := fmt.Sprintf(
		`dir=$(mktemp -d /tmp/step-runner.XXXXXXXX) || exit 1; `+
			`trap 'kill ${srv:-} 2>/dev/null; rm -rf "$dir"' EXIT INT TERM HUP; `+
			`%s steps serve --socket "$dir/step-runner.sock" 2>&1 & srv=$!; cat >/dev/null; kill $srv 2>/dev/null; wait $srv 2>/dev/null`,
		shellQuote(runnerCommand),
	)

	stdout := e.BuildLogger.Stream(buildlogger.StreamWorkLevel, buildlogger.Stdout)
	stderr := e.BuildLogger.Stream(buildlogger.StreamWorkLevel, buildlogger.Stderr)

	// readyWriter proxies stdout through to the build log while scanning for
	// the step-runner ready marker; readyCh receives the advertised socket path.
	readyWriter, readyCh := readywriter.New(ctx, stdout)

	// serveStdin is the serve command's stdin (the keepalive read end); closing
	// its write end (serveStop) signals EOF and terminates step-runner (see
	// serveCommand above).
	serveStdin, serveStop := io.Pipe()

	// Guarantee step-runner is reaped even if the returned dialer is never
	// invoked - e.g. the caller fails to initialise the steps client between a
	// successful Connect and the first dial. Without this, the serve goroutine
	// would block until the job context is cancelled, leaving step-runner and an
	// SSH session lingering on a reused instance.
	go func() {
		<-ctx.Done()
		_ = serveStop.Close()
	}()

	serveErrCh := make(chan error, 1)
	go func() {
		defer stdout.Close()
		defer stderr.Close()

		serveErrCh <- e.client.Run(ctx, executors.RunOptions{
			Command: serveCommand,
			Stdin:   serveStdin,
			Stdout:  readyWriter,
			Stderr:  stderr,
		})
	}()

	// Wait until step-runner reports it is listening. There is no timeout here
	// other than the job timeout carried by ctx: if step-runner never arrives,
	// we wait until the job is cancelled.
	var socketPath string
	select {
	case socket, ok := <-readyCh:
		if !ok {
			_ = serveStop.Close()
			return nil, fmt.Errorf("step-runner ready channel closed")
		}
		// step-runner picks the socket name inside the mktemp directory and
		// reports it here; an empty path means it never advertised one (e.g. an
		// incompatible version), so the proxy would have nothing to dial.
		if socket == "" {
			_ = serveStop.Close()
			return nil, fmt.Errorf("step-runner ready message missing socket path")
		}
		socketPath = socket

	case err := <-serveErrCh:
		_ = serveStop.Close()
		if err == nil {
			// A clean exit before the marker means serve stopped without ever
			// advertising a socket; %w on a nil error would render as "%!w(<nil>)".
			return nil, fmt.Errorf("step-runner serve exited before becoming ready")
		}
		return nil, fmt.Errorf("step-runner serve exited before becoming ready: %w", err)

	case <-ctx.Done():
		_ = serveStop.Close()
		return nil, ctx.Err()
	}

	// step-runner reported the socket it is listening on; the proxy dials it.
	proxyCommand := fmt.Sprintf("%s steps proxy --socket %s", shellQuote(runnerCommand), shellQuote(socketPath))

	return func() (io.ReadWriteCloser, error) {
		conn, err := e.client.DialRun(ctx, proxyCommand)
		if err != nil {
			return nil, fmt.Errorf("dialing step-runner proxy: %w", err)
		}
		// Closing the connection (when step execution completes) also stops
		// step-runner by closing the serve command's stdin.
		//
		// This assumes the dialer is invoked once per Connect: the steps client
		// (steps.Execute) drives a single long-lived streaming RPC over one
		// connection and closes it once via CloseConn, so a healthy job dials
		// exactly once. All serveConns share serveStop, so if gRPC were to
		// reconnect and close the old transport, that close would also stop
		// step-runner; the ctx.Done goroutine above is the backstop that ties
		// step-runner's lifetime to the job regardless.
		return &serveConn{ReadWriteCloser: conn, stop: serveStop}, nil
	}, nil
}

// serveConn wraps the proxy connection so that closing it also terminates the
// step-runner serve process by closing its stdin pipe.
type serveConn struct {
	io.ReadWriteCloser
	stop *io.PipeWriter
}

func (c *serveConn) Close() error {
	err := c.ReadWriteCloser.Close()
	_ = c.stop.Close()
	return err
}

// shellQuote single-quotes a string for safe interpolation into a POSIX shell
// command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// verifyRunnerBinary probes the gitlab-runner binary on the instance via
// `--version`. It errors when the binary is missing/unusable (native steps
// cannot proceed without it) or when the instance runs an OS we don't support
// for native steps, and warns when the instance binary's version differs from
// the manager's (which can cause steps to behave unexpectedly).
func (e *executor) verifyRunnerBinary(ctx context.Context, runnerCommand string) error {
	// Bound the probe so a wedged session cannot hold the entire job: `--version`
	// is a fast local exec, so a generous timeout still surfaces a hung instance
	// long before the job timeout would.
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// stdout and stderr must be distinct buffers: the SSH connector drains them
	// in separate goroutines, and bytes.Buffer is not safe for concurrent use.
	// `--version` writes to stdout; stderr is captured only to surface it on
	// failure.
	var stdout, stderr bytes.Buffer
	err := e.client.Run(ctx, executors.RunOptions{
		Command: shellQuote(runnerCommand) + " --version",
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err != nil {
		return fmt.Errorf(
			"native steps require the gitlab-runner binary on the instance, but %q could not be run (%w); "+
				"ensure gitlab-runner is installed on the instance image and available in PATH",
			runnerCommand, err,
		)
	}

	version, osArch := parseRunnerVersion(stdout.String())

	if os, _, ok := strings.Cut(osArch, "/"); ok && strings.EqualFold(os, "windows") {
		return fmt.Errorf("native steps are not supported on windows instances")
	}

	// Only warn on a genuine mismatch between two known release versions. Skip
	// when the manager is a dev build (AppVersion.Version is forced to
	// common.DevelopmentVersion at init), which never matches a real instance
	// version and would warn spuriously.
	if version != "" && common.AppVersion.Version != common.DevelopmentVersion && version != common.AppVersion.Version {
		e.BuildLogger.Warningln(fmt.Sprintf(
			"The gitlab-runner version on the instance (%s) differs from the manager version (%s); "+
				"native steps may behave unexpectedly. Align the instance image with the manager version.",
			version, common.AppVersion.Version,
		))
	}

	return nil
}

// parseRunnerVersion extracts the version and os/arch from `gitlab-runner
// --version` output (see common.AppVersionInfo.Extended). Either value may be
// empty if not found.
func parseRunnerVersion(output string) (version, osArch string) {
	for _, line := range strings.Split(output, "\n") {
		if v, ok := strings.CutPrefix(line, "Version:"); ok {
			version = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "OS/Arch:"); ok {
			osArch = strings.TrimSpace(v)
		}
	}
	return version, osArch
}
