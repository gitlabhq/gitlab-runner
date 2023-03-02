//go:build !integration

package permission

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/wait"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

func TestNewDockerLinuxSetter(t *testing.T) {
	got := NewDockerLinuxSetter(&docker.MockClient{}, logrus.New(), &types.ImageInspect{})
	assert.IsType(t, &dockerLinuxSetter{}, got)
}

func TestDockerLinuxSetter_Set(t *testing.T) {
	const permissionContainerID = "permissionContainerID"
	const helperImageID = "helperImageID"
	const volume = "volume1"

	testErr := errors.New("testErr")

	volumeBindingsMatcher := mock.MatchedBy(func(cfg *container.HostConfig) bool {
		assert.Len(t, cfg.Binds, 1)
		assert.Equal(t, fmt.Sprintf("%s:%s", volume, dstMount), cfg.Binds[0])
		return true
	})

	containerCmdMatcher := mock.MatchedBy(func(cfg *container.Config) bool {
		assert.Equal(t, helperImageID, cfg.Image)
		assert.Len(t, cfg.Cmd, 3)
		require.Contains(t, cfg.Labels, "foo")
		assert.Equal(t, "bar", cfg.Labels["foo"])
		return true
	})

	tests := map[string]struct {
		clientAssertions func(c *docker.MockClient)
		waitAssertions   func(w *wait.MockWaiter)
		expectedErr      error
	}{
		"successful update of permission container": {
			clientAssertions: func(c *docker.MockClient) {
				c.On(
					"ContainerCreate",
					mock.Anything,
					containerCmdMatcher,
					volumeBindingsMatcher,
					mock.Anything,
					mock.Anything,
				).
					Return(container.CreateResponse{ID: permissionContainerID}, nil).
					Once()
				c.On("ContainerStart", mock.Anything, permissionContainerID, mock.Anything).
					Return(nil).
					Once()
				c.On("ContainerRemove", mock.Anything, permissionContainerID, mock.Anything).
					Return(nil).
					Once()
			},
			waitAssertions: func(w *wait.MockWaiter) {
				w.On("Wait", mock.Anything, permissionContainerID).Return(nil).Once()
			},
			expectedErr: nil,
		},
		"failed to start container container still removed": {
			clientAssertions: func(c *docker.MockClient) {
				c.On(
					"ContainerCreate",
					mock.Anything,
					containerCmdMatcher,
					volumeBindingsMatcher,
					mock.Anything,
					mock.Anything,
				).
					Return(container.CreateResponse{ID: permissionContainerID}, nil).
					Once()
				c.On("ContainerStart", mock.Anything, permissionContainerID, mock.Anything).
					Return(testErr).
					Once()
				c.On("ContainerRemove", mock.Anything, permissionContainerID, mock.Anything).
					Return(nil).
					Once()
			},
			waitAssertions: func(w *wait.MockWaiter) {},
			expectedErr:    testErr,
		},
		"failed to create container": {
			clientAssertions: func(c *docker.MockClient) {
				c.On(
					"ContainerCreate",
					mock.Anything,
					containerCmdMatcher,
					volumeBindingsMatcher,
					mock.Anything,
					mock.Anything,
				).
					Return(container.CreateResponse{}, testErr).
					Once()
			},
			waitAssertions: func(w *wait.MockWaiter) {},
			expectedErr:    testErr,
		},
		"container exit code is 1": {
			clientAssertions: func(c *docker.MockClient) {
				c.On(
					"ContainerCreate",
					mock.Anything,
					containerCmdMatcher,
					volumeBindingsMatcher,
					mock.Anything,
					mock.Anything,
				).
					Return(container.CreateResponse{ID: permissionContainerID}, nil).
					Once()
				c.On("ContainerStart", mock.Anything, permissionContainerID, mock.Anything).
					Return(nil).
					Once()
				c.On("ContainerRemove", mock.Anything, permissionContainerID, mock.Anything).
					Return(nil).
					Once()
			},
			waitAssertions: func(w *wait.MockWaiter) {
				w.On("Wait", mock.Anything, permissionContainerID).Return(testErr).Once()
			},
			expectedErr: testErr,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mClient := new(docker.MockClient)
			defer mClient.AssertExpectations(t)

			mWaiter := new(wait.MockWaiter)
			defer mWaiter.AssertExpectations(t)

			tt.clientAssertions(mClient)
			tt.waitAssertions(mWaiter)

			setter := dockerLinuxSetter{
				client: mClient,
				waiter: mWaiter,
				logger: logrus.New(),
				helperImage: &types.ImageInspect{
					ID: helperImageID,
				},
			}

			err := setter.Set(context.Background(), volume, map[string]string{"foo": "bar"})
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
