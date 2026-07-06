//go:build !integration

package kubernetes

import (
	"context"
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
	testclient "k8s.io/client-go/kubernetes/fake"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/steps"
	"gitlab.com/gitlab-org/step-runner/schema/v1"
)

// featuresFn must advertise NativeStepsIntegration so the runner's
// dispatch in common/build.go can route Concrete-mode jobs through
// Connect().
func TestFeaturesFn_AdvertisesNativeStepsIntegration(t *testing.T) {
	var features common.FeaturesInfo

	featuresFn(&features)

	assert.True(t, features.NativeStepsIntegration,
		"K8s executor must advertise NativeStepsIntegration so that "+
			"Build.UseNativeSteps() can return true for K8s jobs")
	// Spot-check that the existing fields are unaffected.
	assert.True(t, features.Session,
		"existing feature flags must remain set")
}

// With the executor's feature advertisement, the UseNativeSteps
// predicate in common/steps.go must evaluate to true for a Linux
// runner host running a `run:`-using job.
func TestBuildUseNativeSteps_TrueWhenK8sAdvertisesAndJobRunSet(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("UseNativeSteps returns false on Windows by design")
	}

	build := &common.Build{
		Runner: &common.RunnerConfig{},
		ExecutorFeatures: common.FeaturesInfo{
			NativeStepsIntegration: true,
		},
	}
	// Job.Run is non-empty → one of the three predicate triggers fires.
	stepName := "step1"
	build.Job.Run = spec.Run{schema.Step{Name: &stepName}}

	assert.True(t, build.UseNativeSteps(),
		"UseNativeSteps must return true when the K8s executor advertises "+
			"NativeStepsIntegration and the job has run: defined")
}

// Connect must reject before any pod is created or log stream is
// opened when FF_CONCRETE is not enabled. The s.pod == nil assertion
// below confirms the early-return.
func TestConnect_RejectsWhenFFConcreteDisabled(t *testing.T) {
	ex := newStepsTestExecutor(t)
	// FF_CONCRETE intentionally NOT enabled.
	buildtest.SetBuildFeatureFlag(ex.Build, featureflags.UseConcrete, false)

	dialer, err := ex.Connect(t.Context())

	require.Error(t, err, "Connect must fail without FF_CONCRETE")
	assert.Nil(t, dialer, "no dialer returned on the FF_CONCRETE rejection")
	assert.True(t, errors.Is(err, errFFConcreteRequired) ||
		strings.Contains(err.Error(),
			"native steps on kubernetes requires FF_CONCRETE"),
		"error must surface the FF_CONCRETE requirement, got: %v", err)

	// The rejection must happen BEFORE any pod is created — s.pod stays nil.
	assert.Nil(t, ex.pod,
		"FF_CONCRETE gate must reject before ensureStepsPod runs")
}

// stepsConn.Close must close stdin first, then cancel the stream
// context, then close stdout. The order is load-bearing so step-runner
// can shut down its protocol cleanly before the SPDY stream is torn
// down.
func TestStepsConn_Close_OrdersStdinCancelStdout(t *testing.T) {
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	cancelCalled := false
	cancel := func() { cancelCalled = true }

	conn := &stepsConn{
		stdinWriter:  stdinWriter,
		stdoutReader: stdoutReader,
		cancel:       cancel,
	}

	// Close should not error.
	require.NoError(t, conn.Close())

	// stdin pipe closed — reads on stdinReader return EOF.
	_, err := io.ReadAll(stdinReader)
	require.NoError(t, err, "stdinReader should EOF cleanly after Close")

	assert.True(t, cancelCalled, "stream context must be cancelled by Close")

	// stdoutReader closed — writes on stdoutWriter fail with ErrClosedPipe.
	_, writeErr := stdoutWriter.Write([]byte("x"))
	assert.ErrorIs(t, writeErr, io.ErrClosedPipe,
		"stdout pipe must be closed by Close")
}

// Given a pod whose bootstrap init container terminated with the
// "steps not found" stderr substring, helperImageUpgradeMessage
// returns the friendly upgrade message.
func TestHelperImageUpgradeMessage_RecognisesStepsNotFound(t *testing.T) {
	ex := newStepsTestExecutor(t)
	ex.pod = &api.Pod{
		Status: api.PodStatus{
			InitContainerStatuses: []api.ContainerStatus{
				{
					Name: "init-steps-bootstrap",
					State: api.ContainerState{
						Terminated: &api.ContainerStateTerminated{
							ExitCode: 1,
							Message:  "Command steps not found\n",
						},
					},
				},
			},
		},
	}

	got := ex.helperImageUpgradeMessage(t.Context())

	assert.Contains(t, got,
		"helper does not contain CI Steps support",
		"detect must return the friendly upgrade message")
}

// helperImageUpgradeMessage must return empty string when the init
// container terminated for an unrelated reason — we don't want to
// misattribute every init failure to a helper-too-old condition.
func TestHelperImageUpgradeMessage_EmptyOnUnrelatedFailure(t *testing.T) {
	ex := newStepsTestExecutor(t)
	ex.pod = &api.Pod{
		Status: api.PodStatus{
			InitContainerStatuses: []api.ContainerStatus{
				{
					Name: "init-steps-bootstrap",
					State: api.ContainerState{
						Terminated: &api.ContainerStateTerminated{
							ExitCode: 137,
							Message:  "Killed (OOM)",
						},
					},
				},
			},
		},
	}

	assert.Empty(t, ex.helperImageUpgradeMessage(t.Context()),
		"unrelated init failure must not be reported as helper-too-old")
}

func TestHelperImageUpgradeMessage_EmptyOnNilPod(t *testing.T) {
	ex := newStepsTestExecutor(t)
	ex.pod = nil

	assert.Empty(t, ex.helperImageUpgradeMessage(t.Context()))
}

func TestHelperImageUpgradeMessage_FallsBackToLastTerminationState(t *testing.T) {
	ex := newStepsTestExecutor(t)
	ex.pod = &api.Pod{
		Status: api.PodStatus{
			InitContainerStatuses: []api.ContainerStatus{
				{
					Name: "init-steps-bootstrap",
					// Current state is empty (e.g., container removed),
					// but the last termination state carries the message.
					LastTerminationState: api.ContainerState{
						Terminated: &api.ContainerStateTerminated{
							ExitCode: 1,
							Message:  "Command steps not found\n",
						},
					},
				},
			},
		},
	}

	got := ex.helperImageUpgradeMessage(t.Context())
	assert.Contains(t, got, "helper does not contain CI Steps support")
}

// Regression test for the stale-snapshot bug: s.pod is set once at
// creation time and never refreshed by waitForPodRunning. Without an
// explicit refresh in helperImageUpgradeMessage, the init-container
// statuses populated asynchronously by the API server are invisible
// and the friendly upgrade message never surfaces.
//
// This test seeds s.pod with an empty Status (the creation-time
// view) and a fake kube client that returns the "real" pod with the
// `steps not found` termination message populated. The function must
// re-fetch and surface the friendly message.
func TestHelperImageUpgradeMessage_RefreshesStaleSnapshot(t *testing.T) {
	ex := newStepsTestExecutor(t)

	// Creation-time snapshot: name + namespace populated, status empty.
	ex.pod = &api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-steps-pod",
			Namespace: "default",
		},
	}

	// Fresh pod as the API would return it after the bootstrap init
	// container has terminated.
	freshPod := &api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-steps-pod",
			Namespace: "default",
		},
		Status: api.PodStatus{
			InitContainerStatuses: []api.ContainerStatus{
				{
					Name: "init-steps-bootstrap",
					State: api.ContainerState{
						Terminated: &api.ContainerStateTerminated{
							ExitCode: 1,
							Message:  "Command steps not found\n",
						},
					},
				},
			},
		},
	}
	ex.kubeClient = testclient.NewClientset(freshPod)

	got := ex.helperImageUpgradeMessage(t.Context())

	assert.Contains(t, got,
		"helper does not contain CI Steps support",
		"refresh must observe the asynchronously-populated init status")
}

// awaitStepRunnerReady fans in five channels; this table drives each
// branch in isolation and asserts the outcome translation, plus that
// logCancel is invoked exactly once on whichever branch wins.
func TestAwaitStepRunnerReady(t *testing.T) {
	streamErr := errors.New("stream failed")
	notFound := kubeerrors.NewNotFound(
		k8sschema.GroupResource{Resource: "pods"}, "test-pod")

	tests := []struct {
		name      string
		cancelCtx bool
		arrange   func(ready chan string, logsDone, podStatus, podWatcherErrs chan error)
		assert    func(t *testing.T, socketPath string, err error)
	}{
		{
			name: "ready returns socket path",
			arrange: func(ready chan string, _, _, _ chan error) {
				ready <- "/run/step-runner.sock"
			},
			assert: func(t *testing.T, socketPath string, err error) {
				require.NoError(t, err)
				assert.Equal(t, "/run/step-runner.sock", socketPath)
			},
		},
		{
			name: "ready channel closed",
			arrange: func(ready chan string, _, _, _ chan error) {
				close(ready)
			},
			assert: func(t *testing.T, socketPath string, err error) {
				assert.Empty(t, socketPath)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "ready channel closed")
			},
		},
		{
			name: "ready missing socket path",
			arrange: func(ready chan string, _, _, _ chan error) {
				ready <- ""
			},
			assert: func(t *testing.T, socketPath string, err error) {
				assert.Empty(t, socketPath)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "missing socket path")
			},
		},
		{
			name: "logs done clean is no-step-runner",
			arrange: func(_ chan string, logsDone, _, _ chan error) {
				logsDone <- nil
			},
			assert: func(t *testing.T, _ string, err error) {
				assert.ErrorIs(t, err, steps.ErrNoStepRunnerButOkay)
			},
		},
		{
			name: "logs done unexpected error",
			arrange: func(_ chan string, logsDone, _, _ chan error) {
				logsDone <- streamErr
			},
			assert: func(t *testing.T, _ string, err error) {
				require.Error(t, err)
				assert.ErrorIs(t, err, streamErr)
				assert.Contains(t, err.Error(), "log stream ended unexpectedly")
			},
		},
		{
			name: "logs done context canceled without ctx cancel is suppressed",
			arrange: func(_ chan string, logsDone, _, _ chan error) {
				logsDone <- context.Canceled
			},
			assert: func(t *testing.T, _ string, err error) {
				assert.ErrorIs(t, err, steps.ErrNoStepRunnerButOkay)
			},
		},
		{
			name: "pod not found surfaces as-is",
			arrange: func(_ chan string, _, podStatus, _ chan error) {
				podStatus <- notFound
			},
			assert: func(t *testing.T, _ string, err error) {
				assert.True(t, IsKubernetesPodNotFoundError(err))
			},
		},
		{
			name: "pod failed surfaces as-is",
			arrange: func(_ chan string, _, podStatus, _ chan error) {
				podStatus <- &podPhaseError{name: "p", phase: api.PodFailed}
			},
			assert: func(t *testing.T, _ string, err error) {
				assert.True(t, IsKubernetesPodFailedError(err))
			},
		},
		{
			name: "pod container non-zero exit becomes BuildError",
			arrange: func(_ chan string, _, podStatus, _ chan error) {
				podStatus <- &podContainerError{
					containerName: "build", exitCode: 3, reason: "boom"}
			},
			assert: func(t *testing.T, _ string, err error) {
				var buildErr *common.BuildError
				require.ErrorAs(t, err, &buildErr)
				assert.Equal(t, 3, buildErr.ExitCode)
			},
		},
		{
			name: "pod container zero exit is no-step-runner",
			arrange: func(_ chan string, _, podStatus, _ chan error) {
				podStatus <- &podContainerError{containerName: "build", exitCode: 0}
			},
			assert: func(t *testing.T, _ string, err error) {
				assert.ErrorIs(t, err, steps.ErrNoStepRunnerButOkay)
			},
		},
		{
			name: "pod status generic error becomes BuildError",
			arrange: func(_ chan string, _, podStatus, _ chan error) {
				podStatus <- streamErr
			},
			assert: func(t *testing.T, _ string, err error) {
				var buildErr *common.BuildError
				require.ErrorAs(t, err, &buildErr)
				assert.ErrorIs(t, buildErr.Inner, streamErr)
			},
		},
		{
			name: "pod watcher error surfaces as-is",
			arrange: func(_ chan string, _, _, podWatcherErrs chan error) {
				podWatcherErrs <- streamErr
			},
			assert: func(t *testing.T, _ string, err error) {
				assert.ErrorIs(t, err, streamErr)
			},
		},
		{
			name:      "ctx done returns ctx err",
			cancelCtx: true,
			arrange:   func(_ chan string, _, _, _ chan error) {},
			assert: func(t *testing.T, _ string, err error) {
				assert.ErrorIs(t, err, context.Canceled)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.cancelCtx {
				cancel()
			}

			ready := make(chan string, 1)
			logsDone := make(chan error, 1)
			podStatus := make(chan error, 1)
			podWatcherErrs := make(chan error, 1)
			tt.arrange(ready, logsDone, podStatus, podWatcherErrs)

			logCancelCalls := 0
			logCancel := func() { logCancelCalls++ }

			socketPath, err := awaitStepRunnerReady(
				ctx, logCancel, ready, logsDone, podStatus, podWatcherErrs)

			tt.assert(t, socketPath, err)
			assert.Equal(t, 1, logCancelCalls,
				"logCancel must be invoked exactly once")
		})
	}
}

// Regression guard for the ctx-cancelled-vs-logsDone race: when the job's
// ctx is cancelled, io.Copy on the derived logCtx returns context.Canceled
// and logsDone fires. Whichever of ctx.Done() or logsDone wins the select,
// awaitStepRunnerReady must surface ctx.Err(), never masking the
// cancellation as ErrNoStepRunnerButOkay. Looped so the regression cannot
// hide behind a lucky scheduling order.
func TestAwaitStepRunnerReady_CancelRace(t *testing.T) {
	for range 100 {
		ctx, cancel := context.WithCancel(t.Context())

		logsDone := make(chan error, 1)
		logsDone <- context.Canceled
		cancel() // both logsDone and ctx.Done() are now ready

		_, err := awaitStepRunnerReady(
			ctx, func() {}, make(chan string), logsDone,
			make(chan error), make(chan error))

		require.ErrorIs(t, err, context.Canceled,
			"cancellation must surface as ctx.Err()")
		assert.NotErrorIs(t, err, steps.ErrNoStepRunnerButOkay,
			"cancellation must not be masked as ErrNoStepRunnerButOkay")
	}
}
