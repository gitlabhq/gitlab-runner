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
			attempts:        1,
			expectedErr:     new(test.NotFoundError),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mClient := new(docker.MockClient)
			defer mClient.AssertExpectations(t)

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

			err := waiter.Wait(context.Background(), "id")
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestDockerWaiter_StopKillWait(t *testing.T) {
	mClient := new(docker.MockClient)
	defer mClient.AssertExpectations(t)

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

	err := waiter.StopKillWait(context.Background(), "id", nil)
	assert.NoError(t, err)
}

func TestDockerWaiter_WaitContextCanceled(t *testing.T) {
	mClient := new(docker.MockClient)
	defer mClient.AssertExpectations(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	waiter := NewDockerKillWaiter(mClient)

	err := waiter.Wait(ctx, "id")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestDockerWaiter_WaitNonZeroExitCode(t *testing.T) {
	exitCode := 1
	failedContainer := container.WaitResponse{
		StatusCode: int64(exitCode),
	}

	mClient := new(docker.MockClient)
	defer mClient.AssertExpectations(t)

	bodyCh := make(chan container.WaitResponse, 1)
	bodyCh <- failedContainer
	mClient.On("ContainerWait", mock.Anything, mock.Anything, container.WaitConditionNotRunning).
		Return((<-chan container.WaitResponse)(bodyCh), nil)

	waiter := NewDockerKillWaiter(mClient)

	err := waiter.Wait(context.Background(), "id")

	var buildError *common.BuildError
	assert.ErrorAs(t, err, &buildError)
	assert.True(t, buildError.ExitCode == exitCode, "expected exit code %v, but got %v", exitCode, buildError.ExitCode)
}
