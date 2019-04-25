package volumes

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestErrVolumeAlreadyDefined(t *testing.T) {
	err := NewErrVolumeAlreadyDefined("test-path")
	assert.EqualError(t, err, `volume for container path "test-path" is already defined`)
}

func TestNewDefaultManager(t *testing.T) {
	logger := common.NewBuildLogger(nil, nil)

	m := NewManager(logger, nil, ManagerConfig{})
	assert.IsType(t, &manager{}, m)
}

func newDefaultManager(config ManagerConfig) *manager {
	m := &manager{
		logger:         common.NewBuildLogger(nil, nil),
		config:         config,
		managedVolumes: make(map[string]bool, 0),
	}

	return m
}

func addContainerManager(manager *manager) *MockContainerManager {
	containerManager := new(MockContainerManager)

	manager.containerManager = containerManager

	return containerManager
}

func TestDefaultManager_CreateUserVolumes_HostVolume(t *testing.T) {
	testCases := map[string]struct {
		volume            string
		baseContainerPath string
		expectedBinding   []string
		expectedError     error
	}{
		"no volumes specified": {
			volume:          "",
			expectedBinding: []string{"/host:/duplicated"},
		},
		"volume with absolute path": {
			volume:          "/host:/volume",
			expectedBinding: []string{"/host:/duplicated", "/host:/volume"},
		},
		"volume with absolute path and with baseContainerPath specified": {
			volume:            "/host:/volume",
			baseContainerPath: "/builds",
			expectedBinding:   []string{"/host:/duplicated", "/host:/volume"},
		},
		"volume without absolute path and without baseContainerPath specified": {
			volume:          "/host:volume",
			expectedBinding: []string{"/host:/duplicated", "/host:volume"},
		},
		"volume without absolute path and with baseContainerPath specified": {
			volume:            "/host:volume",
			baseContainerPath: "/builds/project",
			expectedBinding:   []string{"/host:/duplicated", "/host:/builds/project/volume"},
		},
		"duplicated volume specification": {
			volume:          "/host/new:/duplicated",
			expectedBinding: []string{"/host:/duplicated"},
			expectedError:   NewErrVolumeAlreadyDefined("/duplicated"),
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BaseContainerPath: testCase.baseContainerPath,
			}

			m := newDefaultManager(config)

			err := m.Create("/host:/duplicated")
			require.NoError(t, err)

			err = m.Create(testCase.volume)
			assert.Equal(t, testCase.expectedError, err)
			assert.Equal(t, testCase.expectedBinding, m.volumeBindings)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_Disabled(t *testing.T) {
	testCases := map[string]struct {
		volume            string
		baseContainerPath string
		disableCache      bool

		expectedBinding           []string
		expectedCacheContainerIDs []string
		expectedConfigVolume      string
		expectedError             error
	}{
		"no volumes specified": {
			volume:          "",
			expectedBinding: []string{"/host:/duplicated"},
		},
		"volume with absolute path, without baseContainerPath and with disableCache": {
			volume:            "/volume",
			baseContainerPath: "",
			disableCache:      true,
			expectedBinding:   []string{"/host:/duplicated"},
			expectedError:     ErrCacheVolumesDisabled,
		},
		"volume with absolute path, with baseContainerPath and with disableCache": {
			volume:            "/volume",
			baseContainerPath: "/builds/project",
			disableCache:      true,
			expectedBinding:   []string{"/host:/duplicated"},
			expectedError:     ErrCacheVolumesDisabled,
		},
		"volume without absolute path, without baseContainerPath and with disableCache": {
			volume:          "volume",
			disableCache:    true,
			expectedBinding: []string{"/host:/duplicated"},
			expectedError:   ErrCacheVolumesDisabled,
		},
		"volume without absolute path, with baseContainerPath and with disableCache": {
			volume:            "volume",
			baseContainerPath: "/builds/project",
			disableCache:      true,
			expectedBinding:   []string{"/host:/duplicated"},
			expectedError:     ErrCacheVolumesDisabled,
		},
		"duplicated volume definition": {
			volume:            "/duplicated",
			baseContainerPath: "",
			disableCache:      true,
			expectedBinding:   []string{"/host:/duplicated"},
			expectedError:     ErrCacheVolumesDisabled,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BaseContainerPath: testCase.baseContainerPath,
				DisableCache:      testCase.disableCache,
			}

			m := newDefaultManager(config)

			err := m.Create("/host:/duplicated")
			require.NoError(t, err)

			err = m.Create(testCase.volume)
			assert.Equal(t, testCase.expectedError, err)
			assert.Equal(t, testCase.expectedBinding, m.volumeBindings)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_HostBased(t *testing.T) {
	testCases := map[string]struct {
		volume            string
		baseContainerPath string
		cacheDir          string
		uniqName          string

		expectedBinding           []string
		expectedCacheContainerIDs []string
		expectedConfigVolume      string
		expectedError             error
	}{
		"volume with absolute path, without baseContainerPath and with cacheDir": {
			volume:          "/volume",
			cacheDir:        "/cache",
			uniqName:        "uniq",
			expectedBinding: []string{"/host:/duplicated", "/cache/uniq/14331bf18c8e434c4b3f48a8c5cc79aa:/volume"},
		},
		"volume with absolute path, with baseContainerPath and with cacheDir": {
			volume:            "/volume",
			baseContainerPath: "/builds/project",
			cacheDir:          "/cache",
			uniqName:          "uniq",
			expectedBinding:   []string{"/host:/duplicated", "/cache/uniq/14331bf18c8e434c4b3f48a8c5cc79aa:/volume"},
		},
		"volume without absolute path, without baseContainerPath and with cacheDir": {
			volume:          "volume",
			cacheDir:        "/cache",
			uniqName:        "uniq",
			expectedBinding: []string{"/host:/duplicated", "/cache/uniq/210ab9e731c9c36c2c38db15c28a8d1c:volume"},
		},
		"volume without absolute path, with baseContainerPath and with cacheDir": {
			volume:            "volume",
			baseContainerPath: "/builds/project",
			cacheDir:          "/cache",
			uniqName:          "uniq",
			expectedBinding:   []string{"/host:/duplicated", "/cache/uniq/f69aef9fb01e88e6213362a04877452d:/builds/project/volume"},
		},
		"duplicated volume definition": {
			volume:          "/duplicated",
			cacheDir:        "/cache",
			uniqName:        "uniq",
			expectedBinding: []string{"/host:/duplicated"},
			expectedError:   NewErrVolumeAlreadyDefined("/duplicated"),
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BaseContainerPath: testCase.baseContainerPath,
				DisableCache:      false,
				CacheDir:          testCase.cacheDir,
				UniqName:          testCase.uniqName,
			}

			m := newDefaultManager(config)

			err := m.Create("/host:/duplicated")
			require.NoError(t, err)

			err = m.Create(testCase.volume)
			assert.Equal(t, testCase.expectedError, err)
			assert.Equal(t, testCase.expectedBinding, m.volumeBindings)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_ContainerBased(t *testing.T) {
	testCases := map[string]struct {
		volume                   string
		baseContainerPath        string
		uniqName                 string
		expectedContainerName    string
		expectedContainerPath    string
		existingContainerID      string
		newContainerID           string
		expectedCacheContainerID string
		expectedError            error
	}{
		"volume with absolute path, without baseContainerPath and with existing container": {
			volume:                   "/volume",
			baseContainerPath:        "",
			uniqName:                 "uniq",
			expectedContainerName:    "uniq-cache-14331bf18c8e434c4b3f48a8c5cc79aa",
			expectedContainerPath:    "/volume",
			existingContainerID:      "existingContainerID",
			expectedCacheContainerID: "existingContainerID",
		},
		"volume with absolute path, without baseContainerPath and with new container": {
			volume:                   "/volume",
			baseContainerPath:        "",
			uniqName:                 "uniq",
			expectedContainerName:    "uniq-cache-14331bf18c8e434c4b3f48a8c5cc79aa",
			expectedContainerPath:    "/volume",
			existingContainerID:      "",
			newContainerID:           "newContainerID",
			expectedCacheContainerID: "newContainerID",
		},
		"volume without absolute path, without baseContainerPath and with existing container": {
			volume:                   "volume",
			baseContainerPath:        "",
			uniqName:                 "uniq",
			expectedContainerName:    "uniq-cache-210ab9e731c9c36c2c38db15c28a8d1c",
			expectedContainerPath:    "volume",
			existingContainerID:      "existingContainerID",
			expectedCacheContainerID: "existingContainerID",
		},
		"volume without absolute path, without baseContainerPath and with new container": {
			volume:                   "volume",
			baseContainerPath:        "",
			uniqName:                 "uniq",
			expectedContainerName:    "uniq-cache-210ab9e731c9c36c2c38db15c28a8d1c",
			expectedContainerPath:    "volume",
			existingContainerID:      "",
			newContainerID:           "newContainerID",
			expectedCacheContainerID: "newContainerID",
		},
		"volume without absolute path, with baseContainerPath and with existing container": {
			volume:                   "volume",
			baseContainerPath:        "/builds/project",
			uniqName:                 "uniq",
			expectedContainerName:    "uniq-cache-f69aef9fb01e88e6213362a04877452d",
			expectedContainerPath:    "/builds/project/volume",
			existingContainerID:      "existingContainerID",
			expectedCacheContainerID: "existingContainerID",
		},
		"volume without absolute path, with baseContainerPath and with new container": {
			volume:                   "volume",
			baseContainerPath:        "/builds/project",
			uniqName:                 "uniq",
			expectedContainerName:    "uniq-cache-f69aef9fb01e88e6213362a04877452d",
			expectedContainerPath:    "/builds/project/volume",
			existingContainerID:      "",
			newContainerID:           "newContainerID",
			expectedCacheContainerID: "newContainerID",
		},
		"duplicated volume definition": {
			volume:        "/duplicated",
			uniqName:      "uniq",
			expectedError: NewErrVolumeAlreadyDefined("/duplicated"),
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BaseContainerPath: testCase.baseContainerPath,
				UniqName:          testCase.uniqName,
				DisableCache:      false,
			}

			m := newDefaultManager(config)
			containerManager := addContainerManager(m)

			defer containerManager.AssertExpectations(t)

			err := m.Create("/host:/duplicated")
			require.NoError(t, err)

			if testCase.volume != "/duplicated" {
				containerManager.On("FindExistingCacheContainer", testCase.expectedContainerName, testCase.expectedContainerPath).
					Return(testCase.existingContainerID).
					Once()

				if testCase.newContainerID != "" {
					containerManager.On("CreateCacheContainer", testCase.expectedContainerName, testCase.expectedContainerPath).
						Return(testCase.newContainerID, nil).
						Once()
				}
			}

			err = m.Create(testCase.volume)
			assert.Equal(t, testCase.expectedError, err)

			if testCase.expectedCacheContainerID != "" {
				assert.Contains(t, m.cacheContainerIDs, testCase.expectedCacheContainerID)
			}
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_ContainerBased_WithError(t *testing.T) {
	config := ManagerConfig{
		BaseContainerPath: "/builds/project",
		UniqName:          "uniq",
	}

	m := newDefaultManager(config)
	containerManager := addContainerManager(m)

	defer containerManager.AssertExpectations(t)

	containerManager.On("FindExistingCacheContainer", "uniq-cache-f69aef9fb01e88e6213362a04877452d", "/builds/project/volume").
		Return("").
		Once()

	containerManager.On("CreateCacheContainer", "uniq-cache-f69aef9fb01e88e6213362a04877452d", "/builds/project/volume").
		Return("", errors.New("test error")).
		Once()

	err := m.Create("volume")
	assert.Error(t, err)
}

func TestDefaultManager_CreateTemporary(t *testing.T) {
	testCases := map[string]struct {
		volume                   string
		newContainerID           string
		containerCreateError     error
		expectedContainerName    string
		expectedContainerPath    string
		expectedCacheContainerID string
		expectedTmpContainerID   string
		expectedError            error
	}{
		"volume created": {
			volume:                   "volume",
			newContainerID:           "newContainerID",
			expectedContainerName:    "uniq-cache-f69aef9fb01e88e6213362a04877452d",
			expectedContainerPath:    "/builds/project/volume",
			expectedCacheContainerID: "newContainerID",
			expectedTmpContainerID:   "newContainerID",
		},
		"cache container creation error": {
			volume:               "volume",
			newContainerID:       "",
			containerCreateError: errors.New("test-error"),
			expectedError:        errors.New("test-error"),
		},
		"duplicated volume definition": {
			volume:        "/duplicated",
			expectedError: NewErrVolumeAlreadyDefined("/duplicated"),
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BaseContainerPath: "/builds/project",
				UniqName:          "uniq",
			}

			m := newDefaultManager(config)
			containerManager := addContainerManager(m)

			defer containerManager.AssertExpectations(t)

			err := m.Create("/host:/duplicated")
			require.NoError(t, err)

			if testCase.volume != "/duplicated" {
				containerManager.On("FindExistingCacheContainer", "uniq-cache-f69aef9fb01e88e6213362a04877452d", "/builds/project/volume").
					Return("").
					Once()

				containerManager.On("CreateCacheContainer", "uniq-cache-f69aef9fb01e88e6213362a04877452d", "/builds/project/volume").
					Return(testCase.newContainerID, testCase.containerCreateError).
					Once()
			}

			err = m.CreateTemporary(testCase.volume)
			assert.Equal(t, testCase.expectedError, err)

			if testCase.expectedCacheContainerID != "" {
				assert.Contains(t, m.cacheContainerIDs, testCase.expectedCacheContainerID)
			}

			if testCase.expectedTmpContainerID != "" {
				assert.Contains(t, m.tmpContainerIDs, testCase.expectedTmpContainerID)
			}
		})
	}
}

func TestDefaultManager_Binds(t *testing.T) {
	expectedElements := []string{"element1", "element2"}
	m := &manager{
		volumeBindings: expectedElements,
	}

	assert.Equal(t, expectedElements, m.Binds())
}

func TestDefaultManager_ContainerIDs(t *testing.T) {
	expectedElements := []string{"element1", "element2"}
	m := &manager{
		cacheContainerIDs: expectedElements,
	}

	assert.Equal(t, expectedElements, m.ContainerIDs())
}

func TestDefaultManager_Cleanup(t *testing.T) {
	cManager := new(MockContainerManager)
	defer cManager.AssertExpectations(t)

	cManager.On("RemoveCacheContainer", mock.Anything, "container-1").
		Return(nil).
		Once()

	m := &manager{
		containerManager: cManager,
		tmpContainerIDs:  []string{"container-1"},
	}

	done := m.Cleanup(context.Background())
	<-done
}
