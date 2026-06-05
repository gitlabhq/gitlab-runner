//go:build !integration

package wait

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/test"
)

func TestDockerWaiter_Wait(t *testing.T) {
	testErr := errors.New("testErr")

	tests := map[string]struct {
		containerOKBody container.WaitResponse
		waitErr         error
		attempts        int
		expectedErr     error
	}{
		"container exited successfully": {
			containerOKBody: container.WaitResponse{
				StatusCode: 0,
			},
			waitErr:     nil,
			attempts:    1,
			expectedErr: nil,
		},
		"container wait failed": {
			containerOKBody: container.WaitResponse{},
			waitErr:         testErr,
			attempts:        5,
			expectedErr:     testErr,
		},
		"container not found": {
			containerOKBody: container.WaitResponse{},
			waitErr:         new(test.NotFoundError),
			attempts:        5,
			expectedErr:     new(test.NotFoundError),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mClient := docker.NewMockClient(t)
			bodyCh := make(chan container.WaitResponse, 1)
			errCh := make(chan error, tt.attempts)

			if tt.expectedErr != nil {
				for i := 0; i < tt.attempts; i++ {
					errCh <- tt.waitErr
				}
			} else {
				bodyCh <- tt.containerOKBody
			}

			mClient.On("ContainerWait", mock.Anything, mock.Anything, container.WaitConditionNotRunning).
				Return((<-chan container.WaitResponse)(bodyCh), (<-chan error)(errCh)).
				Times(tt.attempts)

			waiter := NewDockerKillWaiter(mClient)

			err := waiter.Wait(t.Context(), "id")
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestDockerWaiter_StopKillWait(t *testing.T) {
	mClient := docker.NewMockClient(t)

	bodyCh := make(chan container.WaitResponse)
	mClient.On("ContainerWait", mock.Anything, mock.Anything, container.WaitConditionNotRunning).
		Return((<-chan container.WaitResponse)(bodyCh), nil).
		Once()

	var wg sync.WaitGroup

	wg.Add(2)
	mClient.On("ContainerStop", mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) {
			wg.Done()
		}).
		Return(nil).
		Twice()

	waiter := NewDockerKillWaiter(mClient)

	go func() {
		wg.Wait()
		bodyCh <- container.WaitResponse{
			StatusCode: 0,
		}
	}()

	err := waiter.StopKillWait(t.Context(), "id", nil, nil)
	assert.NoError(t, err)
}

// TestDockerWaiter_StopKillWait_CancelledContext verifies that ContainerStop is
// still called even when the build context is already cancelled before
// StopKillWait is invoked. This is the race that occurs when Abort() fires and
// cancels the build context before the cleanup path reaches StopKillWait.
func TestDockerWaiter_StopKillWait_CancelledContext(t *testing.T) {
	mClient := docker.NewMockClient(t)

	bodyCh := make(chan container.WaitResponse)
	mClient.On("ContainerWait", mock.Anything, mock.Anything, container.WaitConditionNotRunning).
		Return((<-chan container.WaitResponse)(bodyCh), nil).
		Once()

	var wg sync.WaitGroup

	wg.Add(2)
	mClient.On("ContainerStop", mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) {
			wg.Done()
		}).
		Return(nil).
		Twice()

	waiter := NewDockerKillWaiter(mClient)

	go func() {
		wg.Wait()
		bodyCh <- container.WaitResponse{StatusCode: 0}
	}()

	// Simulate the build context already being cancelled (Abort() fired).
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := waiter.StopKillWait(ctx, "id", nil, nil)
	assert.NoError(t, err)
	// ContainerStop must have been called despite the cancelled context.
	mClient.AssertExpectations(t)
}

// TestDockerWaiter_StopKillWait_CancelledContext_NilGracefulExit verifies that
// ContainerStop is still called when the build context is already cancelled and
// gracefulExitFunc is nil (as when FF_USE_NATIVE_CONTAINER_STOP is enabled).
// This is the path where PID 1 receives SIGTERM solely via ContainerStop.
func TestDockerWaiter_StopKillWait_CancelledContext_NilGracefulExit(t *testing.T) {
	mClient := docker.NewMockClient(t)

	bodyCh := make(chan container.WaitResponse)
	mClient.On("ContainerWait", mock.Anything, mock.Anything, container.WaitConditionNotRunning).
		Return((<-chan container.WaitResponse)(bodyCh), nil).
		Once()

	var wg sync.WaitGroup

	wg.Add(2)
	mClient.On("ContainerStop", mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) {
			wg.Done()
		}).
		Return(nil).
		Twice()

	waiter := NewDockerKillWaiter(mClient)

	go func() {
		wg.Wait()
		bodyCh <- container.WaitResponse{StatusCode: 0}
	}()

	// Simulate the build context already being cancelled (Abort() fired).
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// gracefulExitFunc is nil — no kill script, only ContainerStop.
	err := waiter.StopKillWait(ctx, "id", nil, nil)
	assert.NoError(t, err)
	// ContainerStop must have been called despite the cancelled context.
	mClient.AssertExpectations(t)
}

// TestDockerWaiter_StopKillWait_GracefulExitFuncNotCalledWhenNil verifies that
// when gracefulExitFunc is nil, no exec is performed in the container — only
// ContainerStop is used. This ensures FF_USE_NATIVE_CONTAINER_STOP semantics:
// the kill script is not run, and PID 1 receives SIGTERM via Docker's native
// stop mechanism.
func TestDockerWaiter_StopKillWait_GracefulExitFuncNotCalledWhenNil(t *testing.T) {
	mClient := docker.NewMockClient(t)

	bodyCh := make(chan container.WaitResponse)
	mClient.On("ContainerWait", mock.Anything, mock.Anything, container.WaitConditionNotRunning).
		Return((<-chan container.WaitResponse)(bodyCh), nil).
		Once()

	stopCalled := make(chan struct{}, 1)
	mClient.On("ContainerStop", mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) {
			select {
			case stopCalled <- struct{}{}:
			default:
			}
		}).
		Return(nil)

	waiter := NewDockerKillWaiter(mClient)

	go func() {
		// Wait for at least one ContainerStop call before signaling exit.
		<-stopCalled
		bodyCh <- container.WaitResponse{StatusCode: 0}
	}()

	// Pass nil gracefulExitFunc — simulates FF_USE_NATIVE_CONTAINER_STOP=true.
	err := waiter.StopKillWait(t.Context(), "id", nil, nil)
	assert.NoError(t, err)

	// No ContainerExecCreate should have been called (no kill script).
	mClient.AssertNotCalled(t, "ContainerExecCreate", mock.Anything, mock.Anything, mock.Anything)
	// ContainerStop must have been called at least once.
	mClient.AssertCalled(t, "ContainerStop", mock.Anything, mock.Anything, mock.Anything)
}

func TestDockerWaiter_WaitContextCanceled(t *testing.T) {
	mClient := docker.NewMockClient(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	waiter := NewDockerKillWaiter(mClient)

	err := waiter.Wait(ctx, "id")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestDockerWaiter_WaitNonZeroExitCode(t *testing.T) {
	tests := map[string]struct {
		statusCode   int64
		wantExitCode int
		wantInnerMsg string
	}{
		"unix exit code 1 (identity through NormalizeExitCode)": {
			statusCode:   1,
			wantExitCode: 1,
			wantInnerMsg: "exit code 1",
		},
		// Windows DWORD 0xFFFFFFFF (4294967295) reinterprets as -1 after
		// NormalizeExitCode. Without NormalizeExitCode, ExitCode would be
		// 4294967295 on 64-bit platforms, making this assertion fail.
		"windows DWORD 0xFFFFFFFF normalises to -1": {
			statusCode:   4294967295,
			wantExitCode: -1,
			wantInnerMsg: "exit code -1",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mClient := docker.NewMockClient(t)

			bodyCh := make(chan container.WaitResponse, 1)
			bodyCh <- container.WaitResponse{StatusCode: tt.statusCode}
			mClient.On("ContainerWait", mock.Anything, mock.Anything, container.WaitConditionNotRunning).
				Return((<-chan container.WaitResponse)(bodyCh), nil)

			waiter := NewDockerKillWaiter(mClient)

			err := waiter.Wait(t.Context(), "id")

			var buildError *common.BuildError
			assert.ErrorAs(t, err, &buildError)
			assert.Equal(t, tt.wantExitCode, buildError.ExitCode,
				"ExitCode must equal NormalizeExitCode(int(statusCode))")
			assert.Equal(t, tt.wantInnerMsg, buildError.Inner.Error(),
				"Inner error message must use normalized exit code")
			assert.Equal(t, common.ScriptFailure, buildError.FailureReason)
		})
	}
}
