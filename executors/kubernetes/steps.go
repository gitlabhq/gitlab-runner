package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/executors/internal/readywriter"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/steps"
)

// helperTooOldStderrSubstring is the substring the bootstrap init
// container emits to stderr when the helper image predates the `steps`
// subcommand. Kubernetes copies the trailing stderr into
// ContainerStateTerminated.Message when TerminationMessagePolicy is set
// to FallbackToLogsOnError (see buildStepsBootstrapInitContainer).
const helperTooOldStderrSubstring = "steps not found"

// helperTooOldUserMessage is the user-facing error returned when the
// bootstrap init container fails for the helper-too-old reason. The
// wording matches executors/docker/steps.go for cross-executor parity.
const helperTooOldUserMessage = "helper does not contain CI Steps " +
	"support: please upgrade your version of the GitLab Runner helper " +
	"binary"

// stepsConn wraps a bidirectional SPDY exec session into a single
// io.ReadWriteCloser, suitable for use as the dialer's return value.
//
// Read sources bytes from the SPDY session's stdout pipe; Write forwards
// bytes to the SPDY session's stdin pipe; Close shuts the session down
// in a specific order — stdin first (signalling EOF to step-runner so it
// can shut down its protocol cleanly), then the stream context (which
// terminates the streaming goroutine), then stdout (which releases any
// blocked readers).
type stepsConn struct {
	stdinWriter  *io.PipeWriter
	stdoutReader *io.PipeReader
	cancel       context.CancelFunc
}

func (c *stepsConn) Read(p []byte) (int, error) {
	return c.stdoutReader.Read(p)
}

func (c *stepsConn) Write(p []byte) (int, error) {
	return c.stdinWriter.Write(p)
}

func (c *stepsConn) Close() error {
	_ = c.stdinWriter.Close()
	c.cancel()
	return c.stdoutReader.Close()
}

// Connect implements steps.Connector for the Kubernetes executor. It
// returns a dialer closure that establishes a bidirectional bytestream
// between the runner and the step-runner serve process running in the
// build container of a Concrete-mode pod.
//
// The flow is:
//  1. Require FF_CONCRETE (defense-in-depth; the build-level gate in
//     Build.executeScript rejects such jobs first).
//  2. Ensure the Concrete pod exists (idempotent on s.pod).
//  3. Open a follow stream against the build container's logs and wrap
//     it with readywriter so the ready marker is extracted from stderr.
//  4. Watch pod container status concurrently so we surface container
//     and pod failures that occur before the ready marker arrives.
//  5. On ready: cancel the log stream and return the dialer.
//  6. On any failure: cancel the log stream and return the error.
func (s *executor) Connect(ctx context.Context) (func() (io.ReadWriteCloser, error), error) {
	if !s.Build.IsFeatureFlagOn(featureflags.UseConcrete) {
		return nil, common.ErrNativeStepsRequireConcrete
	}

	// Match classic dispatch: retry pod creation on pull-policy failures.
	err := s.withPullRetry(ctx, func() error {
		return s.ensureStepsPod(ctx)
	})
	if err != nil {
		// Only refetch the pod and inspect init-container statuses when
		// the failure mentions the bootstrap init container by name. Any
		// other pre-step failure (image pull, scheduling, API rejection
		// on a different container) cannot be a helper-too-old condition,
		// so the API roundtrip is unnecessary.
		if strings.Contains(err.Error(), stepsBootstrapInitContainerName) {
			if msg := s.helperImageUpgradeMessage(ctx); msg != "" {
				// Wrap the underlying error so errors.Is/As keep working
				// for downstream callers and so operators reading
				// structured logs still see the originating cause (e.g.,
				// "pod failed to enter running state").
				return nil, fmt.Errorf("%s: %w", msg, err)
			}
		}
		return nil, fmt.Errorf("ensuring steps pod: %w", err)
	}

	// logCtx lets us cancel the log stream goroutine independently from
	// ctx once the ready marker is observed; all output thereafter flows
	// over the bidirectional dialer connection.
	logCtx, logCancel := context.WithCancel(ctx)

	stdout := s.BuildLogger.Stream(
		buildlogger.StreamWorkLevel, buildlogger.Stdout)
	rw, readyCh := readywriter.New(logCtx, stdout)

	logs, err := s.kubeClient.CoreV1().Pods(s.pod.Namespace).
		GetLogs(s.pod.Name, &api.PodLogOptions{
			Container: buildContainerName,
			Follow:    true,
		}).Stream(logCtx)
	if err != nil {
		logCancel()
		stdout.Close()
		return nil, fmt.Errorf("streaming build container logs: %w", err)
	}

	logsDone := make(chan error, 1)
	go func() {
		defer logs.Close()
		defer stdout.Close()
		_, copyErr := io.Copy(rw, logs)
		logsDone <- copyErr
	}()

	// watchPodStatus is scoped to ctx, not logCtx: the pod-status watcher
	// must outlive the log stream so container failures occurring after
	// the ready marker (during the protocol session over the dialer) can
	// still surface. The channel is buffered, so at most one status drains
	// before Connect returns; any subsequent send blocks the watcher's
	// goroutine until ctx is cancelled — matching the runWithAttach pattern.
	podStatusCh := s.watchPodStatus(ctx, &podContainerStatusChecker{
		shouldCheckContainerFilter: func(cs api.ContainerStatus) bool {
			return cs.Name == buildContainerName
		},
	})

	socketPath, err := awaitStepRunnerReady(
		ctx, logCancel, readyCh, logsDone, podStatusCh, s.podWatcher.Errors())
	if err != nil {
		return nil, err
	}

	return func() (io.ReadWriteCloser, error) {
		return s.execStepsProxy(ctx, socketPath)
	}, nil
}

// awaitStepRunnerReady blocks until one of the connection's fan-in
// channels resolves and translates that outcome into a (socketPath, err)
// pair: the ready path returns the step-runner socket path; every other
// path returns an error. logCancel stops the log-stream goroutine and is
// invoked exactly once, on whichever branch wins.
//
//nolint:gocognit // The select fans in five independent channels; each branch is small.
func awaitStepRunnerReady(
	ctx context.Context,
	logCancel context.CancelFunc,
	readyCh <-chan string,
	logsDone <-chan error,
	podStatusCh <-chan error,
	podWatcherErrs <-chan error,
) (string, error) {
	select {
	case socketPath, ok := <-readyCh:
		// All further output flows over the bidirectional connection;
		// the log stream is no longer needed.
		logCancel()
		if !ok {
			return "", errors.New("step-runner ready channel closed")
		}
		if socketPath == "" {
			return "", errors.New(
				"step-runner ready message missing socket path")
		}
		return socketPath, nil

	case copyErr := <-logsDone:
		logCancel()
		// logCtx derives from ctx, and logCancel() has not run before this
		// branch is entered, so a context.Canceled in copyErr here can only
		// originate from ctx itself. Check ctx first: a cancelled job must
		// surface as ctx.Err(), not be masked as ErrNoStepRunnerButOkay.
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if copyErr != nil && !errors.Is(copyErr, context.Canceled) {
			return "", fmt.Errorf(
				"build container log stream ended unexpectedly: %w", copyErr)
		}
		return "", steps.ErrNoStepRunnerButOkay

	case err := <-podStatusCh:
		logCancel()
		if IsKubernetesPodNotFoundError(err) || IsKubernetesPodFailedError(err) {
			return "", err
		}
		var containerErr *podContainerError
		if errors.As(err, &containerErr) {
			if containerErr.exitCode != 0 {
				return "", &common.BuildError{
					Inner:    err,
					ExitCode: containerErr.exitCode,
				}
			}
			return "", steps.ErrNoStepRunnerButOkay
		}
		return "", &common.BuildError{Inner: err}

	case err := <-podWatcherErrs:
		logCancel()
		return "", err

	case <-ctx.Done():
		logCancel()
		return "", ctx.Err()
	}
}

// execStepsProxy opens a SPDY exec session against the build container
// that runs `gitlab-runner-helper steps proxy --socket <path>`, and
// returns an io.ReadWriteCloser over its stdin/stdout streams. Stderr
// is suppressed: step-runner emits its protocol on stdout, and any
// diagnostic noise on stderr should not bleed into the user-visible
// build log.
//
// Each invocation creates a fresh exec session with its own stream
// context. Invocations share no state.
func (s *executor) execStepsProxy(
	ctx context.Context, socketPath string,
) (io.ReadWriteCloser, error) {
	streamCtx, streamCancel := context.WithCancel(ctx)

	req := s.kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(s.pod.Name).
		Namespace(s.pod.Namespace).
		SubResource("exec").
		VersionedParams(&api.PodExecOptions{
			Container: buildContainerName,
			Command: []string{
				s.stepsRunnerBinaryPath(),
				"steps", "proxy",
				"--socket", socketPath,
			},
			Stdin:  true,
			Stdout: true,
			Stderr: false,
		}, scheme.ParameterCodec)

	spdyExec, err := remotecommand.NewSPDYExecutor(
		s.kubeConfig, http.MethodPost, req.URL())
	if err != nil {
		streamCancel()
		return nil, fmt.Errorf(
			"creating SPDY executor for steps proxy: %w", err)
	}

	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	go func() {
		// Cancel streamCtx as soon as the stream is done so the context
		// is released without waiting for stepsConn.Close(). Cancel is
		// idempotent — the later Close() call is harmless.
		defer streamCancel()
		defer stdinReader.Close()
		defer stdoutWriter.Close()

		streamErr := spdyExec.StreamWithContext(
			streamCtx, remotecommand.StreamOptions{
				Stdin:  stdinReader,
				Stdout: stdoutWriter,
			})
		if streamErr != nil {
			_ = stdinReader.CloseWithError(streamErr)
			_ = stdoutWriter.CloseWithError(streamErr)
		}
	}()

	return &stepsConn{
		stdinWriter:  stdinWriter,
		stdoutReader: stdoutReader,
		cancel:       streamCancel,
	}, nil
}

// helperImageUpgradeMessage returns the friendly upgrade message when
// the bootstrap init container failed because the helper binary
// predates the `steps` subcommand. Returns empty string if no such
// evidence is present.
//
// This is invoked after ensureStepsPod returns an error, so that a
// helper-too-old failure surfaces with the friendly message rather
// than the generic "ensuring steps pod" wrapped error.
//
// s.pod is set once at pod-creation time (setupStepsPod) and never
// updated by waitForPodRunning — the API populates init-container
// statuses asynchronously. Re-fetch the pod into a local so we observe
// the terminated state without mutating s.pod, which other goroutines
// (e.g., captureServiceContainersLogs after a successful pod start) may
// be reading concurrently. Failures to refresh are silently ignored:
// the caller is already returning an error; falling back to the stale
// snapshot is no worse than skipping the check entirely.
func (s *executor) helperImageUpgradeMessage(ctx context.Context) string {
	if s.pod == nil {
		return ""
	}
	pod := s.pod
	if s.kubeClient != nil {
		fresh, err := s.kubeClient.CoreV1().Pods(s.pod.Namespace).
			Get(ctx, s.pod.Name, metav1.GetOptions{})
		if err == nil {
			pod = fresh
		}
	}
	for _, cs := range pod.Status.InitContainerStatuses {
		if cs.Name != stepsBootstrapInitContainerName {
			continue
		}
		t := cs.State.Terminated
		if t == nil {
			t = cs.LastTerminationState.Terminated
		}
		if t == nil {
			continue
		}
		if strings.Contains(t.Message, helperTooOldStderrSubstring) {
			return helperTooOldUserMessage
		}
	}
	return ""
}
