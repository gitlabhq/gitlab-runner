package wait

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/test"
)

func TestDockerWaiter_Wait(t *testing.T) {
	testErr := errors.New("testErr")

	tests := map[string]struct {
		containerInspect types.ContainerJSON
		inspectErr       error
		attempts         int
		expectedErr      error
	}{
		"container exited successfully": {
			containerInspect: types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					State: &types.ContainerState{
						Status:   "exited",
						ExitCode: 0,
					},
				},
			},
			inspectErr:  nil,
			attempts:    1,
			expectedErr: nil,
		},
		"container not running": {
			containerInspect: types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					State: &types.ContainerState{
						Status: "created",
					},
				},
			},
			inspectErr:  nil,
			attempts:    1,
			expectedErr: nil,
		},
		"container inspect failed": {
			containerInspect: types.ContainerJSON{},
			inspectErr:       testErr,
			attempts:         5,
			expectedErr:      testErr,
		},
		"container not found": {
			containerInspect: types.ContainerJSON{},
			inspectErr:       &test.NotFoundError{},
			attempts:         1,
			expectedErr:      &test.NotFoundError{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mClient := new(docker.MockClient)
			defer mClient.AssertExpectations(t)

			mClient.On("ContainerInspect", mock.Anything, mock.Anything).
				Return(tt.containerInspect, tt.inspectErr).
				Times(tt.attempts)

			waiter := NewDockerWaiter(mClient)

			err := waiter.Wait(context.Background(), "id")
			assert.True(t, errors.Is(err, tt.expectedErr), "expected err %T, but got %T", tt.expectedErr, err)
		})
	}
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
	failedContainer := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Status:   "exited",
				ExitCode: 1,
			},
		},
	}

	mClient := new(docker.MockClient)
	defer mClient.AssertExpectations(t)

	mClient.On("ContainerInspect", mock.Anything, mock.Anything).
		Return(failedContainer, nil).
		Once()

	waiter := NewDockerWaiter(mClient)

	err := waiter.Wait(context.Background(), "id")
	var buildError *common.BuildError
	assert.True(t, errors.As(err, &buildError), "expected err %T, but got %T", buildError, err)
}

func TestDockerWaiter_WaitRunningContainer(t *testing.T) {
	runningContainer := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Running: true,
			},
		},
	}

	exitedSuccessContainer := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Status:   "exited",
				ExitCode: 0,
			},
		},
	}

	mClient := new(docker.MockClient)
	defer mClient.AssertExpectations(t)

	mClient.On("ContainerInspect", mock.Anything, mock.Anything).
		Return(runningContainer, nil).
		Times(2)

	mClient.On("ContainerInspect", mock.Anything, mock.Anything).
		Return(exitedSuccessContainer, nil).
		Once()

	waiter := NewDockerWaiter(mClient)

	err := waiter.Wait(context.Background(), "id")
	assert.NoError(t, err)
}
