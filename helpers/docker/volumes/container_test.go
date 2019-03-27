package volumes

import (
	"errors"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestNewDefaultContainerManager(t *testing.T) {
	logger := common.NewBuildLogger(nil, nil)

	m := newDefaultContainerManager(logger, nil, nil, nil)
	assert.IsType(t, &defaultContainerManager{}, m)
}

func getDefaultContainerManager() (*defaultContainerManager, *mockContainerClient, *mockHelperImageResolver, *mockRegistry) {
	cClient := new(mockContainerClient)
	hiResolver := new(mockHelperImageResolver)
	tmpIDsRegistry := new(mockRegistry)

	m := &defaultContainerManager{
		logger:              common.NewBuildLogger(nil, nil),
		containerClient:     cClient,
		helperImageResolver: hiResolver,
		tmpContainerIDs:     tmpIDsRegistry,
	}

	return m, cClient, hiResolver, tmpIDsRegistry
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
			m, cClient, hiResolver, tmpIDsRegistry := getDefaultContainerManager()

			defer func() {
				cClient.AssertExpectations(t)
				hiResolver.AssertExpectations(t)
				tmpIDsRegistry.AssertExpectations(t)
			}()

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
		hiResolverResult       *types.ImageInspect
		hiResolverError        error
		hiResolverCacheCommand []string
		expectedContainerID    string
		createResult           container.ContainerCreateCreatedBody
		createError            error
		containerID            string
		startError             error
		waitForContainerError  error
		expectedTmpID          string
		expectedError          error
	}{
		"error on helper image resolving": {
			hiResolverResult: nil,
			hiResolverError:  errors.New("test error"),
			expectedError:    errors.New("test error"),
		},
		"error on container create": {
			hiResolverResult:       &types.ImageInspect{ID: "imageID"},
			hiResolverCacheCommand: []string{"cache-command"},
			createError:            errors.New("test error"),
			expectedError:          errors.New("test error"),
		},
		"error on container create with returnedID": {
			hiResolverResult:       &types.ImageInspect{ID: "imageID"},
			hiResolverCacheCommand: []string{"cache-command"},
			createResult: container.ContainerCreateCreatedBody{
				ID: "containerID",
			},
			createError:   errors.New("test error"),
			expectedTmpID: "containerID",
			expectedError: errors.New("test error"),
		},
		"error on container start": {
			hiResolverResult:       &types.ImageInspect{ID: "imageID"},
			hiResolverCacheCommand: []string{"cache-command"},
			createResult: container.ContainerCreateCreatedBody{
				ID: "containerID",
			},
			containerID:   "containerID",
			startError:    errors.New("test error"),
			expectedTmpID: "containerID",
			expectedError: errors.New("test error"),
		},
		"error on wait for container": {
			hiResolverResult:       &types.ImageInspect{ID: "imageID"},
			hiResolverCacheCommand: []string{"cache-command"},
			createResult: container.ContainerCreateCreatedBody{
				ID: "containerID",
			},
			containerID:           "containerID",
			waitForContainerError: errors.New("test error"),
			expectedTmpID:         "containerID",
			expectedError:         errors.New("test error"),
		},
		"success": {
			hiResolverResult:       &types.ImageInspect{ID: "imageID"},
			hiResolverCacheCommand: []string{"cache-command"},
			createResult: container.ContainerCreateCreatedBody{
				ID: "containerID",
			},
			containerID:         "containerID",
			expectedContainerID: "containerID",
			expectedError:       nil,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			m, cClient, hiResolver, tmpIDsRegistry := getDefaultContainerManager()

			defer func() {
				cClient.AssertExpectations(t)
				hiResolver.AssertExpectations(t)
				tmpIDsRegistry.AssertExpectations(t)
			}()

			hiResolver.On("ResolveHelperImage").
				Return(testCase.hiResolverResult, testCase.hiResolverError).
				Once()

			if testCase.hiResolverError == nil {
				hiResolver.On("GetCacheCommand", mock.Anything).
					Return(testCase.hiResolverCacheCommand).
					Once()
			}

			if testCase.hiResolverError == nil {
				configMatcher := mock.MatchedBy(func(config *container.Config) bool {
					return config.Image == testCase.hiResolverResult.ID &&
						len(config.Cmd) == len(testCase.hiResolverCacheCommand) &&
						config.Cmd[0] == testCase.hiResolverCacheCommand[0]
				})

				cClient.On("LabelContainer", configMatcher, "cache", "cache.dir=container-path").
					Once()

				cClient.On("CreateContainer", configMatcher, mock.Anything, mock.Anything, containerName).
					Return(testCase.createResult, testCase.createError).
					Once()

			}

			if testCase.containerID != "" {
				cClient.On("StartContainer", testCase.containerID, mock.Anything).
					Return(testCase.startError).
					Once()

				if testCase.startError == nil {
					cClient.On("WaitForContainer", testCase.containerID).
						Return(testCase.waitForContainerError).
						Once()
				}
			}

			if testCase.expectedTmpID != "" {
				tmpIDsRegistry.On("Append", testCase.expectedTmpID).
					Once()
			}

			containerID, err := m.CreateCacheContainer(containerName, containerPath)
			assert.Equal(t, err, testCase.expectedError)
			assert.Equal(t, testCase.expectedContainerID, containerID)
		})
	}
}
