package volumes

import (
	"errors"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestNewDefaultContainerManager(t *testing.T) {
	logger := common.NewBuildLogger(nil, nil)

	m := NewDefaultContainerManager(logger, nil, nil, true)
	assert.IsType(t, &defaultContainerManager{}, m)
}

func getDefaultContainerManager() (*defaultContainerManager, *mockContainerClient) {
	cClient := new(mockContainerClient)

	m := &defaultContainerManager{
		logger:              common.NewBuildLogger(nil, nil),
		containerClient:     cClient,
		failedContainerIDs:  make([]string, 0),
		helperImage:         &types.ImageInspect{ID: "helper-image"},
		outdatedHelperImage: false,
	}

	return m, cClient
}

func TestDefaultContainerManager_FindExistingCacheContainer(t *testing.T) {
	containerName := "container-name"
	containerPath := "container-path"

	testCases := map[string]struct {
		inspectResult       types.ContainerJSON
		inspectError        error
		expectedContainerID string
		expectedRemoveID    string
	}{
		"error on container inspection": {
			inspectError:        errors.New("test error"),
			expectedContainerID: "",
		},
		"container with valid cache exists": {
			inspectResult: types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					ID: "existingWithValidCacheID",
				},
				Config: &container.Config{
					Volumes: map[string]struct{}{
						containerPath: {},
					},
				},
			},
			inspectError:        nil,
			expectedContainerID: "existingWithValidCacheID",
		},
		"container without valid cache exists": {
			inspectResult: types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					ID: "existingWithInvalidCacheID",
				},
				Config: &container.Config{
					Volumes: map[string]struct{}{
						"different-path": {},
					},
				},
			},
			inspectError:        nil,
			expectedContainerID: "",
			expectedRemoveID:    "existingWithInvalidCacheID",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			m, cClient := getDefaultContainerManager()

			defer cClient.AssertExpectations(t)

			cClient.On("InspectContainer", containerName).
				Return(testCase.inspectResult, testCase.inspectError).
				Once()

			if testCase.expectedRemoveID != "" {
				cClient.On("RemoveContainer", testCase.expectedRemoveID).
					Return(nil).
					Once()
			}

			containerID := m.FindExistingCacheContainer(containerName, containerPath)
			assert.Equal(t, testCase.expectedContainerID, containerID)
		})
	}
}

func TestDefaultContainerManager_CreateCacheContainer(t *testing.T) {
	containerName := "container-name"
	containerPath := "container-path"

	testCases := map[string]struct {
		expectedContainerID       string
		createResult              container.ContainerCreateCreatedBody
		createError               error
		containerID               string
		startError                error
		waitForContainerError     error
		expectedFailedContainerID string
		expectedError             error
	}{
		"error on container create": {
			createError:   errors.New("test error"),
			expectedError: errors.New("test error"),
		},
		"error on container create with returnedID": {
			createResult: container.ContainerCreateCreatedBody{
				ID: "containerID",
			},
			createError:               errors.New("test error"),
			expectedFailedContainerID: "containerID",
			expectedError:             errors.New("test error"),
		},
		"error on container start": {
			createResult: container.ContainerCreateCreatedBody{
				ID: "containerID",
			},
			containerID:               "containerID",
			startError:                errors.New("test error"),
			expectedFailedContainerID: "containerID",
			expectedError:             errors.New("test error"),
		},
		"error on wait for container": {
			createResult: container.ContainerCreateCreatedBody{
				ID: "containerID",
			},
			containerID:               "containerID",
			waitForContainerError:     errors.New("test error"),
			expectedFailedContainerID: "containerID",
			expectedError:             errors.New("test error"),
		},
		"success": {
			createResult: container.ContainerCreateCreatedBody{
				ID: "containerID",
			},
			containerID:         "containerID",
			expectedContainerID: "containerID",
			expectedError:       nil,
		},
	}

	// TODO: Remove in 12.0
	outdatedHelperImageValues := map[bool][]string{
		true:  {"gitlab-runner-cache", "container-path"},
		false: {"gitlab-runner-helper", "cache-init", "container-path"},
	}

	for testName, testCase := range testCases {
		for outdatedHelperImage, expectedCommand := range outdatedHelperImageValues {
			t.Run(fmt.Sprintf("%s-outdated-helper-image-is-%v", testName, outdatedHelperImage), func(t *testing.T) {
				m, cClient := getDefaultContainerManager()
				m.outdatedHelperImage = outdatedHelperImage

				defer cClient.AssertExpectations(t)

				configMatcher := mock.MatchedBy(func(config *container.Config) bool {
					if config.Image != "helper-image" {
						return false
					}

					if len(config.Cmd) != len(expectedCommand) {
						return false
					}

					return config.Cmd[0] == expectedCommand[0]
				})

				cClient.On("LabelContainer", configMatcher, "cache", "cache.dir=container-path").
					Once()

				cClient.On("CreateContainer", configMatcher, mock.Anything, mock.Anything, containerName).
					Return(testCase.createResult, testCase.createError).
					Once()

				if testCase.createError == nil {
					cClient.On("StartContainer", testCase.containerID, mock.Anything).
						Return(testCase.startError).
						Once()

					if testCase.startError == nil {
						cClient.On("WaitForContainer", testCase.containerID).
							Return(testCase.waitForContainerError).
							Once()
					}
				}

				require.Empty(t, m.failedContainerIDs, "Initial list of failed containers should be empty")

				containerID, err := m.CreateCacheContainer(containerName, containerPath)
				assert.Equal(t, err, testCase.expectedError)
				assert.Equal(t, testCase.expectedContainerID, containerID)

				if testCase.expectedFailedContainerID != "" {
					assert.Len(t, m.failedContainerIDs, 1)
					assert.Contains(
						t, m.failedContainerIDs, testCase.expectedFailedContainerID,
						"List of failed container should be updated with %s", testCase.expectedContainerID,
					)
				} else {
					assert.Empty(t, m.failedContainerIDs, "List of failed containers should not be updated")
				}
			})
		}
	}
}

func TestDefaultContainerManager_FailedContainerIDs(t *testing.T) {
	expectedElements := []string{"element1", "element2"}
	m := &defaultContainerManager{
		failedContainerIDs: expectedElements,
	}

	assert.Equal(t, expectedElements, m.FailedContainerIDs())
}
