package volumes

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestNewDefaultManager(t *testing.T) {
	logger := common.NewBuildLogger(nil, nil)

	m := NewDefaultManager(logger, nil, nil, DefaultManagerConfig{})
	assert.IsType(t, &defaultManager{}, m)
}

func newDefaultManager(config DefaultManagerConfig) *defaultManager {
	m := &defaultManager{
		logger: common.NewBuildLogger(nil, nil),
		config: config,
	}

	return m
}

func addRegistry(manager *defaultManager) (*mockRegistry, *mockRegistry, *mockRegistry) {
	bindingsRegistry := new(mockRegistry)
	cacheIDsRegistry := new(mockRegistry)
	tmpIdsRegistry := new(mockRegistry)

	manager.volumeBindings = bindingsRegistry
	manager.cacheContainerIDs = cacheIDsRegistry
	manager.tmpContainerIDs = tmpIdsRegistry

	return bindingsRegistry, cacheIDsRegistry, tmpIdsRegistry
}

func addContainerManager(manager *defaultManager) *mockContainerManager {
	containerManager := new(mockContainerManager)

	manager.containerManager = containerManager

	return containerManager
}

func TestDefaultManager_CreateUserVolumes_HostVolume(t *testing.T) {
	testCases := map[string]struct {
		volumes         []string
		fullProjectDir  string
		expectedBinding string
	}{
		"no volumes specified": {
			volumes: []string{},
		},
		"volume with absolute path": {
			volumes:         []string{"/host:/volume"},
			expectedBinding: "/host:/volume",
		},
		"volume with absolute path and with fullProjectDir specified": {
			volumes:         []string{"/host:/volume"},
			fullProjectDir:  "/builds",
			expectedBinding: "/host:/volume",
		},
		"volume without absolute path and without fullProjectDir specified": {
			volumes:         []string{"/host:volume"},
			expectedBinding: "/host:volume",
		},
		"volume without absolute path and with fullProjectDir specified": {
			volumes:         []string{"/host:volume"},
			fullProjectDir:  "/builds/project",
			expectedBinding: "/host:/builds/project/volume",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := DefaultManagerConfig{
				FullProjectDir: testCase.fullProjectDir,
			}

			m := newDefaultManager(config)
			bindingsRegistry, cacheIDsRegistry, tmpIdsRegistry := addRegistry(m)

			defer func() {
				bindingsRegistry.AssertExpectations(t)
				cacheIDsRegistry.AssertExpectations(t)
				tmpIdsRegistry.AssertExpectations(t)
			}()

			bindingsRegistry.On("Append", testCase.expectedBinding).Maybe()

			err := m.CreateUserVolumes(testCase.volumes)
			assert.NoError(t, err)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_Disabled(t *testing.T) {
	testCases := map[string]struct {
		volumes        []string
		fullProjectDir string
		disableCache   bool

		expectedBindings          []string
		expectedCacheContainerIDs []string
		expectedConfigVolume      string
	}{
		"no volumes specified": {
			volumes:          []string{},
			expectedBindings: nil,
		},
		"volume with absolute path, without fullProjectDir and with disableCache": {
			volumes:          []string{"/volume"},
			fullProjectDir:   "",
			disableCache:     true,
			expectedBindings: nil,
		},
		"volume with absolute path, with fullProjectDir and with disableCache": {
			volumes:          []string{"/volume"},
			fullProjectDir:   "/builds/project",
			disableCache:     true,
			expectedBindings: nil,
		},
		"volume without absolute path, without fullProjectDir and with disableCache": {
			volumes:          []string{"volume"},
			disableCache:     true,
			expectedBindings: nil,
		},
		"volume without absolute path, with fullProjectDir and with disableCache": {
			volumes:          []string{"volume"},
			fullProjectDir:   "/builds/project",
			disableCache:     true,
			expectedBindings: nil,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := DefaultManagerConfig{
				FullProjectDir: testCase.fullProjectDir,
				DisableCache:   testCase.disableCache,
			}

			m := newDefaultManager(config)
			bindingsRegistry, cacheIDsRegistry, tmpIdsRegistry := addRegistry(m)

			defer func() {
				bindingsRegistry.AssertExpectations(t)
				cacheIDsRegistry.AssertExpectations(t)
				tmpIdsRegistry.AssertExpectations(t)
			}()

			err := m.CreateUserVolumes(testCase.volumes)
			assert.NoError(t, err)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_HostBased(t *testing.T) {
	testCases := map[string]struct {
		volumes         []string
		fullProjectDir  string
		disableCache    bool
		cacheDir        string
		projectUniqName string

		expectedBinding           string
		expectedCacheContainerIDs []string
		expectedConfigVolume      string
	}{
		"volume with absolute path, without fullProjectDir, without disableCache and with cacheDir": {
			volumes:         []string{"/volume"},
			disableCache:    false,
			cacheDir:        "/cache",
			projectUniqName: "project-uniq",
			expectedBinding: "/cache/project-uniq/14331bf18c8e434c4b3f48a8c5cc79aa:/volume",
		},
		"volume with absolute path, with fullProjectDir, without disableCache and with cacheDir": {
			volumes:         []string{"/volume"},
			fullProjectDir:  "/builds/project",
			disableCache:    false,
			cacheDir:        "/cache",
			projectUniqName: "project-uniq",
			expectedBinding: "/cache/project-uniq/14331bf18c8e434c4b3f48a8c5cc79aa:/volume",
		},
		"volume without absolute path, without fullProjectDir, without disableCache and with cacheDir": {
			volumes:         []string{"volume"},
			disableCache:    false,
			cacheDir:        "/cache",
			projectUniqName: "project-uniq",
			expectedBinding: "/cache/project-uniq/210ab9e731c9c36c2c38db15c28a8d1c:volume",
		},
		"volume without absolute path, with fullProjectDir, without disableCache and with cacheDir": {
			volumes:         []string{"volume"},
			fullProjectDir:  "/builds/project",
			disableCache:    false,
			cacheDir:        "/cache",
			projectUniqName: "project-uniq",
			expectedBinding: "/cache/project-uniq/f69aef9fb01e88e6213362a04877452d:/builds/project/volume",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := DefaultManagerConfig{
				FullProjectDir:  testCase.fullProjectDir,
				DisableCache:    testCase.disableCache,
				CacheDir:        testCase.cacheDir,
				ProjectUniqName: testCase.projectUniqName,
			}

			m := newDefaultManager(config)
			bindingsRegistry, cacheIDsRegistry, tmpIdsRegistry := addRegistry(m)

			defer func() {
				bindingsRegistry.AssertExpectations(t)
				cacheIDsRegistry.AssertExpectations(t)
				tmpIdsRegistry.AssertExpectations(t)
			}()
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_ContainerBased(t *testing.T) {
	testCases := map[string]struct {
		volumes                  []string
		fullProjectDir           string
		projectUniqName          string
		expectedContainerName    string
		expectedContainerPath    string
		existingContainerID      string
		newContainerID           string
		expectedCacheContainerID string
	}{
		"volume with absolute path, without fullProjectDir and with existing container": {
			volumes:                  []string{"/volume"},
			fullProjectDir:           "",
			projectUniqName:          "project-uniq",
			expectedContainerName:    "project-uniq-cache-14331bf18c8e434c4b3f48a8c5cc79aa",
			expectedContainerPath:    "/volume",
			existingContainerID:      "existingContainerID",
			expectedCacheContainerID: "existingContainerID",
		},
		"volume with absolute path, without fullProjectDir and with new container": {
			volumes:                  []string{"/volume"},
			fullProjectDir:           "",
			projectUniqName:          "project-uniq",
			expectedContainerName:    "project-uniq-cache-14331bf18c8e434c4b3f48a8c5cc79aa",
			expectedContainerPath:    "/volume",
			existingContainerID:      "",
			newContainerID:           "newContainerID",
			expectedCacheContainerID: "newContainerID",
		},
		"volume without absolute path, without fullProjectDir and with existing container": {
			volumes:                  []string{"volume"},
			fullProjectDir:           "",
			projectUniqName:          "project-uniq",
			expectedContainerName:    "project-uniq-cache-210ab9e731c9c36c2c38db15c28a8d1c",
			expectedContainerPath:    "volume",
			existingContainerID:      "existingContainerID",
			expectedCacheContainerID: "existingContainerID",
		},
		"volume without absolute path, without fullProjectDir and with new container": {
			volumes:                  []string{"volume"},
			fullProjectDir:           "",
			projectUniqName:          "project-uniq",
			expectedContainerName:    "project-uniq-cache-210ab9e731c9c36c2c38db15c28a8d1c",
			expectedContainerPath:    "volume",
			existingContainerID:      "",
			newContainerID:           "newContainerID",
			expectedCacheContainerID: "newContainerID",
		},
		"volume without absolute path, with fullProjectDir and with existing container": {
			volumes:                  []string{"volume"},
			fullProjectDir:           "/builds/project",
			projectUniqName:          "project-uniq",
			expectedContainerName:    "project-uniq-cache-f69aef9fb01e88e6213362a04877452d",
			expectedContainerPath:    "/builds/project/volume",
			existingContainerID:      "existingContainerID",
			expectedCacheContainerID: "existingContainerID",
		},
		"volume without absolute path, with fullProjectDir and with new container": {
			volumes:                  []string{"volume"},
			fullProjectDir:           "/builds/project",
			projectUniqName:          "project-uniq",
			expectedContainerName:    "project-uniq-cache-f69aef9fb01e88e6213362a04877452d",
			expectedContainerPath:    "/builds/project/volume",
			existingContainerID:      "",
			newContainerID:           "newContainerID",
			expectedCacheContainerID: "newContainerID",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := DefaultManagerConfig{
				FullProjectDir:  testCase.fullProjectDir,
				ProjectUniqName: testCase.projectUniqName,
			}

			m := newDefaultManager(config)
			bindingsRegistry, cacheIDsRegistry, tmpIdsRegistry := addRegistry(m)
			containerManager := addContainerManager(m)

			defer func() {
				bindingsRegistry.AssertExpectations(t)
				cacheIDsRegistry.AssertExpectations(t)
				tmpIdsRegistry.AssertExpectations(t)
				containerManager.AssertExpectations(t)
			}()

			containerManager.On("FindExistingCacheContainer", testCase.expectedContainerName, testCase.expectedContainerPath).
				Return(testCase.existingContainerID).
				Once()

			if testCase.newContainerID != "" {
				containerManager.On("CreateCacheContainer", testCase.expectedContainerName, testCase.expectedContainerPath).
					Return(testCase.newContainerID, nil).
					Once()
			}

			cacheIDsRegistry.On("Append", testCase.expectedCacheContainerID).
				Once()

			err := m.CreateUserVolumes(testCase.volumes)
			assert.NoError(t, err)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_ContainerBased_WithError(t *testing.T) {
	config := DefaultManagerConfig{
		FullProjectDir:  "/builds/project",
		ProjectUniqName: "project-uniq",
	}

	m := newDefaultManager(config)
	bindingsRegistry, cacheIDsRegistry, tmpIdsRegistry := addRegistry(m)
	containerManager := addContainerManager(m)

	defer func() {
		bindingsRegistry.AssertExpectations(t)
		cacheIDsRegistry.AssertExpectations(t)
		tmpIdsRegistry.AssertExpectations(t)
		containerManager.AssertExpectations(t)
	}()

	containerManager.On("FindExistingCacheContainer", "project-uniq-cache-f69aef9fb01e88e6213362a04877452d", "/builds/project/volume").
		Return("").
		Once()

	containerManager.On("CreateCacheContainer", "project-uniq-cache-f69aef9fb01e88e6213362a04877452d", "/builds/project/volume").
		Return("", errors.New("test error")).
		Once()

	err := m.CreateUserVolumes([]string{"volume"})
	assert.Error(t, err)
}

func TestDefaultManager_CreateBuildVolume_WithoutError(t *testing.T) {
	testCases := map[string]struct {
		jobsRootDir           string
		volumes               []string
		gitStrategy           common.GitStrategy
		disableCache          bool
		cacheDir              string
		projectUniqName       string
		expectedContainerName string
		expectedContainerPath string
		newContainerID        string
		expectedError         error
		expectedBinding       string
		expectedTmpAndCacheID string
	}{
		"invalid project full dir": {
			jobsRootDir:   "builds",
			expectedError: errors.New("build directory needs to be absolute and non-root path"),
		},
		"build directory within host mounted volumes": {
			jobsRootDir: "/builds/root",
			volumes:     []string{"/host/builds:/builds"},
		},
		"persistent cache container": {
			jobsRootDir:     "/builds/root",
			gitStrategy:     common.GitFetch,
			disableCache:    false,
			cacheDir:        "/cache",
			projectUniqName: "project-uniq",
			expectedBinding: "/cache/project-uniq/28934d7b9a9154212a5dd671e4fa5704:/builds/root",
		},
		"temporary cache container": {
			jobsRootDir:           "/builds/root",
			gitStrategy:           common.GitClone,
			expectedContainerName: "",
			expectedContainerPath: "/builds/root",
			newContainerID:        "newContainerID",
			expectedTmpAndCacheID: "newContainerID",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := DefaultManagerConfig{
				GitStrategy:     testCase.gitStrategy,
				DisableCache:    testCase.disableCache,
				CacheDir:        testCase.cacheDir,
				ProjectUniqName: testCase.projectUniqName,
			}

			m := newDefaultManager(config)
			bindingsRegistry, cacheIDsRegistry, tmpIdsRegistry := addRegistry(m)
			containerManager := addContainerManager(m)

			defer func() {
				bindingsRegistry.AssertExpectations(t)
				cacheIDsRegistry.AssertExpectations(t)
				tmpIdsRegistry.AssertExpectations(t)
				containerManager.AssertExpectations(t)
			}()

			if testCase.expectedBinding != "" {
				bindingsRegistry.On("Append", testCase.expectedBinding).
					Once()
			}

			if testCase.expectedContainerPath != "" {
				containerManager.On("CreateCacheContainer", testCase.expectedContainerName, testCase.expectedContainerPath).
					Return(testCase.newContainerID, nil).
					Once()
			}

			if testCase.expectedTmpAndCacheID != "" {
				cacheIDsRegistry.On("Append", testCase.expectedTmpAndCacheID).
					Once()
				tmpIdsRegistry.On("Append", testCase.expectedTmpAndCacheID).
					Once()
			}

			err := m.CreateBuildVolume(testCase.jobsRootDir, testCase.volumes)
			if testCase.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, testCase.expectedError.Error())
			}
		})
	}
}

func TestDefaultManager_CreateBuildVolume_WithError(t *testing.T) {
	config := DefaultManagerConfig{
		GitStrategy: common.GitClone,
	}

	m := newDefaultManager(config)
	bindingsRegistry, cacheIDsRegistry, tmpIdsRegistry := addRegistry(m)
	containerManager := addContainerManager(m)

	defer func() {
		bindingsRegistry.AssertExpectations(t)
		cacheIDsRegistry.AssertExpectations(t)
		tmpIdsRegistry.AssertExpectations(t)
		containerManager.AssertExpectations(t)
	}()

	containerManager.On("CreateCacheContainer", "", "/builds/root").
		Return("", errors.New("test error")).
		Once()

	err := m.CreateBuildVolume("/builds/root", []string{"/host/source:/destination"})
	assert.Error(t, err)
}

func TestDefaultManager_VolumeBindings(t *testing.T) {
	registry := new(mockRegistry)
	defer registry.AssertExpectations(t)

	expectedElements := []string{"element1", "element2"}
	registry.On("Elements").Return(expectedElements).Once()

	m := &defaultManager{
		volumeBindings: registry,
	}
	assert.Equal(t, expectedElements, m.VolumeBindings())
}

func TestDefaultManager_CacheContainerIDs(t *testing.T) {
	registry := new(mockRegistry)
	defer registry.AssertExpectations(t)

	expectedElements := []string{"element1", "element2"}
	registry.On("Elements").Return(expectedElements).Once()

	m := &defaultManager{
		cacheContainerIDs: registry,
	}
	assert.Equal(t, expectedElements, m.CacheContainerIDs())
}

func TestDefaultManager_TmpContainerIDs(t *testing.T) {
	registry := new(mockRegistry)
	defer registry.AssertExpectations(t)

	expectedElements := []string{"element1", "element2"}
	registry.On("Elements").Return(expectedElements).Once()

	m := &defaultManager{
		tmpContainerIDs: registry,
	}
	assert.Equal(t, expectedElements, m.TmpContainerIDs())
}
