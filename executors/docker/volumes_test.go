package docker

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// TODO: Remove in 12.0
func TestCacheImageCommand(t *testing.T) {
	oldCommand := []string{"gitlab-runner-cache", "/builds"}
	newCommand := []string{"gitlab-runner-helper", "cache-init", "/builds"}

	cases := map[string]struct {
		outdatedHelperImage bool
		expectedCmd         []string
	}{
		"uses new helper image": {
			outdatedHelperImage: false,
			expectedCmd:         newCommand,
		},
		"uses outdated helper image": {
			outdatedHelperImage: true,
			expectedCmd:         oldCommand,
		},
	}

	for testName, testCase := range cases {
		t.Run(testName, func(t *testing.T) {
			hiResolver := new(MockHelperImageResolver)
			defer hiResolver.AssertExpectations(t)
			hiResolver.On("ResolveHelperImage").
				Return(&types.ImageInspect{ID: "helperImageID"}, nil).
				Once()

			containerConfigMatcher := mock.MatchedBy(func(c *container.Config) bool {
				return len(testCase.expectedCmd) == len(c.Cmd) &&
					testCase.expectedCmd[0] == c.Cmd[0]
			})

			cManager := new(MockContainerManager)
			defer cManager.AssertExpectations(t)
			cManager.On("LabelContainer", mock.Anything, mock.Anything, mock.Anything).
				Once()
			cManager.On("CreateContainer", containerConfigMatcher, mock.Anything, mock.Anything, "").
				Return(container.ContainerCreateCreatedBody{ID: "createdContainerID"}, nil).
				Once()
			cManager.On("StartContainer", "createdContainerID", mock.Anything).
				Return(nil).
				Once()
			cManager.On("WaitForContainer", "createdContainerID").
				Return(nil).
				Once()

			vm := &DefaultVolumesManager{
				DefaultVolumesManagerConfig: DefaultVolumesManagerConfig{
					jobsRootDir:             "/builds",
					fullProjectDir:          "/builds/namespace/project",
					gitStrategy:             common.GitClone,
					disableCache:            true,
					outdatedHelperImageUsed: testCase.outdatedHelperImage,
				},
				helperImageResolver: hiResolver,
				containerManager:    cManager,
			}

			err := vm.CreateBuildVolume([]string{})
			assert.NoError(t, err)
		})
	}
}
