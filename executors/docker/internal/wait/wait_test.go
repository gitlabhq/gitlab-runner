package wait

import (
	"context"
	"errors"
	"testing"
	"time"

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
		containerOKBody container.ContainerWaitOKBody
		waitErr         error
		attempts        int
		expectedErr     error
	}{
		"container exited successfully": {
			containerOKBody: container.ContainerWaitOKBody{
				StatusCode: 0,
			},
			waitErr:     nil,
			attempts:    1,
			expectedErr: nil,
		},
		"container wait failed": {
			containerOKBody: container.ContainerWaitOKBody{},
			waitErr:         testErr,
			attempts:        5,
			expectedErr:     testErr,
		},
		"container not found": {
			containerOKBody: container.ContainerWaitOKBody{},
			waitErr:         &test.NotFoundError{},
			attempts:        1,
			expectedErr:     &test.NotFoundError{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mClient := new(docker.MockClient)
			defer mClient.AssertExpectations(t)

			bodyCh := make(chan container.ContainerWaitOKBody, 1)
			errCh := make(chan error, tt.attempts)

			if tt.expectedErr != nil {
				for i := 0; i < tt.attempts; i++ {
					errCh <- tt.waitErr
				}
			} else {
				bodyCh <- tt.containerOKBody
			}

			mClient.On("ContainerWait", mock.Anything, mock.Anything, container.WaitConditionNotRunning).
				Return((<-chan container.ContainerWaitOKBody)(bodyCh), (<-chan error)(errCh)).
				Times(tt.attempts)

			waiter := NewDockerWaiter(mClient)

			err := waiter.Wait(context.Background(), "id")
			assert.True(t, errors.Is(err, tt.expectedErr), "expected err %T, but got %T", tt.expectedErr, err)
		})
	}
}

func TestDockerWaiter_KillWait(t *testing.T) {
	mClient := new(docker.MockClient)
	defer mClient.AssertExpectations(t)

	bodyCh := make(chan container.ContainerWaitOKBody, 1)
	mClient.On("ContainerWait", mock.Anything, mock.Anything, container.WaitConditionNotRunning).Return(
		(<-chan container.ContainerWaitOKBody)(bodyCh), nil)

	mClient.On("ContainerKill", mock.Anything, mock.Anything, mock.Anything).Return(nil).Twice()

	waiter := NewDockerWaiter(mClient)

	go func() {
		time.Sleep(1500 * time.Millisecond)
		bodyCh <- container.ContainerWaitOKBody{
			StatusCode: 0,
		}
	}()

	err := waiter.KillWait(context.Background(), "id")
	assert.NoError(t, err)
}

func TestDockerWaiter_WaitContextCanceled(t *testing.T) {
	mClient := new(docker.MockClient)
	defer mClient.AssertExpectations(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	waiter := NewDockerWaiter(mClient)

	err := waiter.Wait(ctx, "id")
	assert.True(t, errors.Is(err, context.Canceled), "expected err %T, but got %T", context.Canceled, err)
}

func TestDockerWaiter_WaitNonZeroExitCode(t *testing.T) {
	failedContainer := container.ContainerWaitOKBody{
		StatusCode: 1,
	}

	mClient := new(docker.MockClient)
	defer mClient.AssertExpectations(t)

	bodyCh := make(chan container.ContainerWaitOKBody, 1)
	bodyCh <- failedContainer
	mClient.On("ContainerWait", mock.Anything, mock.Anything, container.WaitConditionNotRunning).Return(
		(<-chan container.ContainerWaitOKBody)(bodyCh), nil)

	waiter := NewDockerWaiter(mClient)

	err := waiter.Wait(context.Background(), "id")
	var buildError *common.BuildError
	assert.True(t, errors.As(err, &buildError), "expected err %T, but got %T", buildError, err)
}
