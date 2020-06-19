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
			waitErr:         new(test.NotFoundError),
			attempts:        1,
			expectedErr:     new(test.NotFoundError),
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

			waiter := NewDockerKillWaiter(mClient)

			err := waiter.Wait(context.Background(), "id")
			assert.True(t, errors.Is(err, tt.expectedErr), "expected err %T, but got %T", tt.expectedErr, err)
		})
	}
}

func TestDockerWaiter_KillWait(t *testing.T) {
	mClient := new(docker.MockClient)
	defer mClient.AssertExpectations(t)

	bodyCh := make(chan container.ContainerWaitOKBody)
	mClient.On("ContainerWait", mock.Anything, mock.Anything, container.WaitConditionNotRunning).
		Return((<-chan container.ContainerWaitOKBody)(bodyCh), nil).
		Once()

	var wg sync.WaitGroup

	wg.Add(2)
	mClient.On("ContainerKill", mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) {
			wg.Done()
		}).
		Return(nil).
		Twice()

	waiter := NewDockerKillWaiter(mClient)

	go func() {
		wg.Wait()
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

	waiter := NewDockerKillWaiter(mClient)

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
	mClient.On("ContainerWait", mock.Anything, mock.Anything, container.WaitConditionNotRunning).
		Return((<-chan container.ContainerWaitOKBody)(bodyCh), nil)

	waiter := NewDockerKillWaiter(mClient)

	err := waiter.Wait(context.Background(), "id")

	var buildError *common.BuildError
	assert.True(t, errors.As(err, &buildError), "expected err %T, but got %T", buildError, err)
}
