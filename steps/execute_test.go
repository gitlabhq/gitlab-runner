//go:build !integration

package steps_test

import (
	"context"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/step-runner/pkg/api/client"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/steps"
	"gitlab.com/gitlab-org/gitlab-runner/steps/stepstest"
)

// TestExecute_RegisterCancel_TriggersCancelRPC exercises the contract this
// branch introduces: when a caller supplies Options.RegisterCancel, Execute
// must hand back a callback whose invocation issues a Cancel RPC for the
// running job, and the resulting cancelled status must surface as a
// ClientStatusError with ErrorCancelled. Without this test the only
// guarantee comes from integration runs.
func TestExecute_RegisterCancel_TriggersCancelRPC(t *testing.T) {
	server := stepstest.New(t)

	jobInfo := steps.JobInfo{
		ID:         42,
		Timeout:    30 * time.Second,
		ProjectDir: t.TempDir(),
		Variables:  spec.Variables{},
	}

	// Buffered so RegisterCancel never blocks; Execute calls it once.
	registered := make(chan context.CancelFunc, 1)

	opts := steps.Options{
		Connector: server.Connector(),
		JobInfo:   jobInfo,
		Steps:     nil,
		Trace:     io.Discard,
		RegisterCancel: func(cb context.CancelFunc) {
			registered <- cb
		},
		Log: logrus.NewEntry(&logrus.Logger{}),
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- steps.Execute(ctx, opts) }()

	select {
	case cb := <-registered:
		require.NotNil(t, cb, "Execute must register a non-nil cancel callback")
		cb()
	case <-ctx.Done():
		t.Fatalf("Execute never invoked RegisterCancel: %v", ctx.Err())
	}

	var execErr error
	select {
	case execErr = <-done:
	case <-ctx.Done():
		t.Fatalf("Execute did not return after cancel: %v", ctx.Err())
	}

	var cserr *steps.ClientStatusError
	require.ErrorAs(t, execErr, &cserr, "Execute must surface a ClientStatusError on cancel")
	assert.Equal(t, client.StateCancelled, cserr.Status.State)
	assert.Equal(t, client.ErrorCancelled, cserr.Status.ErrorKind)

	assert.Equal(t,
		[]string{strconv.FormatInt(jobInfo.ID, 10)},
		server.Cancels(),
		"the cancel callback must call Cancel with the job's request ID",
	)
}

// TestExecute_NilRegisterCancel_DoesNotPanic guards the documented contract
// that RegisterCancel is optional: the dispatched-step path in
// build.executeStage passes nil and must not crash.
func TestExecute_NilRegisterCancel_DoesNotPanic(t *testing.T) {
	server := stepstest.New(t)

	opts := steps.Options{
		Connector: server.Connector(),
		JobInfo: steps.JobInfo{
			ID:         7,
			Timeout:    30 * time.Second,
			ProjectDir: t.TempDir(),
			Variables:  spec.Variables{},
		},
		Steps:          nil,
		Trace:          io.Discard,
		RegisterCancel: nil,
		Log:            logrus.NewEntry(&logrus.Logger{}),
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	// Cancel via context once Execute is past Run; Execute must return
	// without ever having tried to invoke a nil RegisterCancel.
	done := make(chan error, 1)
	go func() { done <- steps.Execute(ctx, opts) }()

	// Give Execute a moment to register the call, then cancel via ctx.
	// We don't need precise timing — context cancellation is the path,
	// not RegisterCancel.
	time.AfterFunc(200*time.Millisecond, cancel)

	select {
	case <-done:
		// Returning at all (without panic) is the assertion.
	case <-time.After(5 * time.Second):
		t.Fatal("Execute did not return after context cancel")
	}

	assert.Empty(t, server.Cancels(), "no Cancel RPC should fire when RegisterCancel is nil")
}
