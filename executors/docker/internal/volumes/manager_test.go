package volumes

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
)

func newDebugLoggerMock() *mockDebugLogger {
	loggerMock := new(mockDebugLogger)
	loggerMock.On("Debugln", mock.Anything)

	return loggerMock
}

func TestErrVolumeAlreadyDefined(t *testing.T) {
	err := NewErrVolumeAlreadyDefined("test-path")
	assert.EqualError(t, err, `volume for container path "test-path" is already defined`)
}

func TestNewDefaultManager(t *testing.T) {
	logger := newDebugLoggerMock()

	m := NewManager(logger, nil, nil, ManagerConfig{})
	assert.IsType(t, &manager{}, m)
}

func newDefaultManager(config ManagerConfig) *manager {
	m := &manager{
		logger:         newDebugLoggerMock(),
		config:         config,
		managedVolumes: make(map[string]bool, 0),
	}

	return m
}

func addCacheContainerManager(manager *manager) *MockCacheContainersManager {
	containerManager := new(MockCacheContainersManager)

	manager.cacheContainersManager = containerManager

	return containerManager
}

func addParser(manager *manager) *parser.MockParser {
	parserMock := new(parser.MockParser)
	manager.parser = parserMock

	return parserMock
}

func TestDefaultManager_CreateUserVolumes_HostVolume(t *testing.T) {
	testCases := map[string]struct {
		volume            string
		parsedVolume      *parser.Volume
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
			parsedVolume:    &parser.Volume{Source: "/host", Destination: "/volume"},
			expectedBinding: []string{"/host:/duplicated", "/host:/volume"},
		},
		"volume with absolute path and with baseContainerPath specified": {
			volume:            "/host:/volume",
			parsedVolume:      &parser.Volume{Source: "/host", Destination: "/volume"},
			baseContainerPath: "/builds",
			expectedBinding:   []string{"/host:/duplicated", "/host:/volume"},
		},
		"volume without absolute path and without baseContainerPath specified": {
			volume:          "/host:volume",
			parsedVolume:    &parser.Volume{Source: "/host", Destination: "volume"},
			expectedBinding: []string{"/host:/duplicated", "/host:volume"},
		},
		"volume without absolute path and with baseContainerPath specified": {
			volume:            "/host:volume",
			parsedVolume:      &parser.Volume{Source: "/host", Destination: "volume"},
			baseContainerPath: "/builds/project",
			expectedBinding:   []string{"/host:/duplicated", "/host:/builds/project/volume"},
		},
		"duplicated volume specification": {
			volume:          "/host/new:/duplicated",
			parsedVolume:    &parser.Volume{Source: "/host/new", Destination: "/duplicated"},
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

			volumeParser := addParser(m)
			defer volumeParser.AssertExpectations(t)

			volumeParser.On("ParseVolume", "/host:/duplicated").
				Return(&parser.Volume{Source: "/host", Destination: "/duplicated"}, nil).
				Once()

			err := m.Create("/host:/duplicated")
			require.NoError(t, err)

			if len(testCase.volume) > 0 {
				volumeParser.On("ParseVolume", testCase.volume).
					Return(testCase.parsedVolume, nil).
					Once()
			}

			err = m.Create(testCase.volume)
			assert.Equal(t, testCase.expectedError, err)
			assert.Equal(t, testCase.expectedBinding, m.volumeBindings)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_Disabled(t *testing.T) {
	expectedBinding := []string{"/host:/duplicated"}

	testCases := map[string]struct {
		volume            string
		parsedVolume      *parser.Volume
		baseContainerPath string

		expectedCacheContainerIDs []string
		expectedConfigVolume      string
		expectedError             error
	}{
		"no volumes specified": {
			volume: "",
		},
		"volume with absolute path, without baseContainerPath and with disableCache": {
			volume:            "/volume",
			parsedVolume:      &parser.Volume{Destination: "/volume"},
			baseContainerPath: "",
			expectedError:     ErrCacheVolumesDisabled,
		},
		"volume with absolute path, with baseContainerPath and with disableCache": {
			volume:            "/volume",
			parsedVolume:      &parser.Volume{Destination: "/volume"},
			baseContainerPath: "/builds/project",
			expectedError:     ErrCacheVolumesDisabled,
		},
		"volume without absolute path, without baseContainerPath and with disableCache": {
			volume:        "volume",
			parsedVolume:  &parser.Volume{Destination: "volume"},
			expectedError: ErrCacheVolumesDisabled,
		},
		"volume without absolute path, with baseContainerPath and with disableCache": {
			volume:            "volume",
			parsedVolume:      &parser.Volume{Destination: "volume"},
			baseContainerPath: "/builds/project",
			expectedError:     ErrCacheVolumesDisabled,
		},
		"duplicated volume definition": {
			volume:            "/duplicated",
			parsedVolume:      &parser.Volume{Destination: "/duplicated"},
			baseContainerPath: "",
			expectedError:     ErrCacheVolumesDisabled,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BaseContainerPath: testCase.baseContainerPath,
				DisableCache:      true,
			}

			m := newDefaultManager(config)

			volumeParser := addParser(m)
			defer volumeParser.AssertExpectations(t)

			volumeParser.On("ParseVolume", "/host:/duplicated").
				Return(&parser.Volume{Source: "/host", Destination: "/duplicated"}, nil).
				Once()

			err := m.Create("/host:/duplicated")
			require.NoError(t, err)

			if len(testCase.volume) > 0 {
				volumeParser.On("ParseVolume", testCase.volume).
					Return(testCase.parsedVolume, nil).
					Once()
			}

			err = m.Create(testCase.volume)
			assert.Equal(t, testCase.expectedError, err)
			assert.Equal(t, expectedBinding, m.volumeBindings)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_HostBased(t *testing.T) {
	testCases := map[string]struct {
		volume            string
		baseContainerPath string
		cacheDir          string
		uniqueName        string

		expectedBinding           []string
		expectedCacheContainerIDs []string
		expectedConfigVolume      string
		expectedError             error
	}{
		"volume with absolute path, without baseContainerPath and with cacheDir": {
			volume:          "/volume",
			cacheDir:        "/cache",
			uniqueName:      "uniq",
			expectedBinding: []string{"/host:/duplicated", "/cache/uniq/14331bf18c8e434c4b3f48a8c5cc79aa:/volume"},
		},
		"volume with absolute path, with baseContainerPath and with cacheDir": {
			volume:            "/volume",
			baseContainerPath: "/builds/project",
			cacheDir:          "/cache",
			uniqueName:        "uniq",
			expectedBinding:   []string{"/host:/duplicated", "/cache/uniq/14331bf18c8e434c4b3f48a8c5cc79aa:/volume"},
		},
		"volume without absolute path, without baseContainerPath and with cacheDir": {
			volume:          "volume",
			cacheDir:        "/cache",
			uniqueName:      "uniq",
			expectedBinding: []string{"/host:/duplicated", "/cache/uniq/210ab9e731c9c36c2c38db15c28a8d1c:volume"},
		},
		"volume without absolute path, with baseContainerPath and with cacheDir": {
			volume:            "volume",
			baseContainerPath: "/builds/project",
			cacheDir:          "/cache",
			uniqueName:        "uniq",
			expectedBinding:   []string{"/host:/duplicated", "/cache/uniq/f69aef9fb01e88e6213362a04877452d:/builds/project/volume"},
		},
		"duplicated volume definition": {
			volume:          "/duplicated",
			cacheDir:        "/cache",
			uniqueName:      "uniq",
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
				UniqueName:        testCase.uniqueName,
			}

			m := newDefaultManager(config)

			volumeParser := addParser(m)
			defer volumeParser.AssertExpectations(t)

			volumeParser.On("ParseVolume", "/host:/duplicated").
				Return(&parser.Volume{Source: "/host", Destination: "/duplicated"}, nil).
				Once()

			err := m.Create("/host:/duplicated")
			require.NoError(t, err)

			volumeParser.On("ParseVolume", testCase.volume).
				Return(&parser.Volume{Destination: testCase.volume}, nil).
				Once()

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
		uniqueName               string
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
			uniqueName:               "uniq",
			expectedContainerName:    "uniq-cache-14331bf18c8e434c4b3f48a8c5cc79aa",
			expectedContainerPath:    "/volume",
			existingContainerID:      "existingContainerID",
			expectedCacheContainerID: "existingContainerID",
		},
		"volume with absolute path, without baseContainerPath and with new container": {
			volume:                   "/volume",
			baseContainerPath:        "",
			uniqueName:               "uniq",
			expectedContainerName:    "uniq-cache-14331bf18c8e434c4b3f48a8c5cc79aa",
			expectedContainerPath:    "/volume",
			existingContainerID:      "",
			newContainerID:           "newContainerID",
			expectedCacheContainerID: "newContainerID",
		},
		"volume without absolute path, without baseContainerPath and with existing container": {
			volume:                   "volume",
			baseContainerPath:        "",
			uniqueName:               "uniq",
			expectedContainerName:    "uniq-cache-210ab9e731c9c36c2c38db15c28a8d1c",
			expectedContainerPath:    "volume",
			existingContainerID:      "existingContainerID",
			expectedCacheContainerID: "existingContainerID",
		},
		"volume without absolute path, without baseContainerPath and with new container": {
			volume:                   "volume",
			baseContainerPath:        "",
			uniqueName:               "uniq",
			expectedContainerName:    "uniq-cache-210ab9e731c9c36c2c38db15c28a8d1c",
			expectedContainerPath:    "volume",
			existingContainerID:      "",
			newContainerID:           "newContainerID",
			expectedCacheContainerID: "newContainerID",
		},
		"volume without absolute path, with baseContainerPath and with existing container": {
			volume:                   "volume",
			baseContainerPath:        "/builds/project",
			uniqueName:               "uniq",
			expectedContainerName:    "uniq-cache-f69aef9fb01e88e6213362a04877452d",
			expectedContainerPath:    "/builds/project/volume",
			existingContainerID:      "existingContainerID",
			expectedCacheContainerID: "existingContainerID",
		},
		"volume without absolute path, with baseContainerPath and with new container": {
			volume:                   "volume",
			baseContainerPath:        "/builds/project",
			uniqueName:               "uniq",
			expectedContainerName:    "uniq-cache-f69aef9fb01e88e6213362a04877452d",
			expectedContainerPath:    "/builds/project/volume",
			existingContainerID:      "",
			newContainerID:           "newContainerID",
			expectedCacheContainerID: "newContainerID",
		},
		"duplicated volume definition": {
			volume:        "/duplicated",
			uniqueName:    "uniq",
			expectedError: NewErrVolumeAlreadyDefined("/duplicated"),
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BaseContainerPath: testCase.baseContainerPath,
				UniqueName:        testCase.uniqueName,
				DisableCache:      false,
			}

			m := newDefaultManager(config)
			containerManager := addCacheContainerManager(m)
			volumeParser := addParser(m)

			defer func() {
				containerManager.AssertExpectations(t)
				volumeParser.AssertExpectations(t)
			}()

			volumeParser.On("ParseVolume", "/host:/duplicated").
				Return(&parser.Volume{Source: "/host", Destination: "/duplicated"}, nil).
				Once()

			err := m.Create("/host:/duplicated")
			require.NoError(t, err)

			if testCase.volume != "/duplicated" {
				containerManager.On("FindOrCleanExisting", testCase.expectedContainerName, testCase.expectedContainerPath).
					Return(testCase.existingContainerID).
					Once()

				if testCase.newContainerID != "" {
					containerManager.On("Create", testCase.expectedContainerName, testCase.expectedContainerPath).
						Return(testCase.newContainerID, nil).
						Once()
				}
			}

			volumeParser.On("ParseVolume", testCase.volume).
				Return(&parser.Volume{Destination: testCase.volume}, nil).
				Once()

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
		UniqueName:        "unique",
	}

	m := newDefaultManager(config)
	containerManager := addCacheContainerManager(m)
	volumeParser := addParser(m)

	defer func() {
		containerManager.AssertExpectations(t)
		volumeParser.AssertExpectations(t)
	}()

	containerManager.On("FindOrCleanExisting", "unique-cache-f69aef9fb01e88e6213362a04877452d", "/builds/project/volume").
		Return("").
		Once()

	containerManager.On("Create", "unique-cache-f69aef9fb01e88e6213362a04877452d", "/builds/project/volume").
		Return("", errors.New("test error")).
		Once()

	volumeParser.On("ParseVolume", "volume").
		Return(&parser.Volume{Destination: "volume"}, nil).
		Once()

	err := m.Create("volume")
	assert.Error(t, err)
}

func TestDefaultManager_CreateUserVolumes_ParserError(t *testing.T) {
	m := newDefaultManager(ManagerConfig{})

	volumeParser := addParser(m)
	defer volumeParser.AssertExpectations(t)

	volumeParser.On("ParseVolume", "volume").
		Return(nil, errors.New("parser-test-error")).
		Once()

	err := m.Create("volume")
	assert.Error(t, err)
}

func TestDefaultManager_CreateTemporary(t *testing.T) {
	testCases := map[string]struct {
		volume                   string
		newContainerID           string
		returnedParsedVolume     *parser.Volume
		containerCreateError     error
		expectedContainerName    string
		expectedContainerPath    string
		expectedCacheContainerID string
		expectedTmpContainerID   string
		expectedError            error
	}{
		"volume created": {
			volume:                   "volume",
			returnedParsedVolume:     &parser.Volume{Destination: "volume"},
			newContainerID:           "newContainerID",
			expectedContainerName:    "uniq-cache-f69aef9fb01e88e6213362a04877452d",
			expectedContainerPath:    "/builds/project/volume",
			expectedCacheContainerID: "newContainerID",
			expectedTmpContainerID:   "newContainerID",
		},
		"cache container creation error": {
			volume:               "volume",
			returnedParsedVolume: &parser.Volume{Destination: "volume"},
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
				UniqueName:        "unique",
			}

			m := newDefaultManager(config)
			containerManager := addCacheContainerManager(m)
			volumeParser := addParser(m)

			defer func() {
				containerManager.AssertExpectations(t)
				volumeParser.AssertExpectations(t)
			}()

			volumeParser.On("ParseVolume", "/host:/duplicated").
				Return(&parser.Volume{Source: "/host", Destination: "/duplicated"}, nil).
				Once()

			err := m.Create("/host:/duplicated")
			require.NoError(t, err)

			if testCase.volume != "/duplicated" {
				containerManager.On("FindOrCleanExisting", "unique-cache-f69aef9fb01e88e6213362a04877452d", "/builds/project/volume").
					Return("").
					Once()

				containerManager.On("Create", "unique-cache-f69aef9fb01e88e6213362a04877452d", "/builds/project/volume").
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
	ccManager := new(MockCacheContainersManager)
	defer ccManager.AssertExpectations(t)

	doneCh := make(chan bool, 1)

	ccManager.On("Cleanup", mock.Anything, []string{"container-1"}).
		Run(func(_ mock.Arguments) {
			close(doneCh)
		}).
		Return(doneCh).
		Once()

	m := &manager{
		cacheContainersManager: ccManager,
		tmpContainerIDs:        []string{"container-1"},
	}

	done := m.Cleanup(context.Background())
	<-done
}
