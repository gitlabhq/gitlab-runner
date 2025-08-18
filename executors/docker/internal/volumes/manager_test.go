//go:build !integration

package volumes

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types/volume"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/labels"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/test"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/path"
)

func TestErrVolumeAlreadyDefined(t *testing.T) {
	err := NewErrVolumeAlreadyDefined("test-path")
	assert.EqualError(t, err, `volume for container path "test-path" is already defined`)
}

func TestNewDefaultManager(t *testing.T) {
	logger := newMockDebugLogger(t)

	m := NewManager(logger, nil, nil, ManagerConfig{}, nil)
	assert.IsType(t, &manager{}, m)
}

func newDefaultManager(t *testing.T, config ManagerConfig) *manager {
	b := &common.Build{
		Runner: &common.RunnerConfig{},
	}

	loggerMock := newMockDebugLogger(t)
	loggerMock.On("Debugln", mock.Anything).Maybe()

	m := &manager{
		logger:         loggerMock,
		config:         config,
		managedVolumes: make(map[string]bool),
		labeler:        labels.NewLabeler(b),
	}

	return m
}

func addUnixParser(t *testing.T, manager *manager) *parser.MockParser {
	return addParser(t, manager, path.NewUnixPath())
}

func addParser(t *testing.T, manager *manager, p parser.Path) *parser.MockParser {
	parserMock := parser.NewMockParser(t)
	parserMock.On("Path").Return(p)

	manager.parser = parserMock
	return parserMock
}

func TestDefaultManager_CreateUserVolumes_HostVolume(t *testing.T) {
	existingBinding := "/host:/duplicated"

	testCases := map[string]struct {
		volume          string
		parsedVolume    *parser.Volume
		basePath        string
		expectedBinding []string
		expectedError   error
	}{
		"no volumes specified": {
			volume:          "",
			expectedBinding: []string{existingBinding},
		},
		"volume with absolute path": {
			volume:          "/host:/volume",
			parsedVolume:    &parser.Volume{Source: "/host", Destination: "/volume"},
			expectedBinding: []string{existingBinding, "/host:/volume"},
		},
		"volume with absolute path and with basePath specified": {
			volume:          "/host:/volume",
			parsedVolume:    &parser.Volume{Source: "/host", Destination: "/volume"},
			basePath:        "/builds",
			expectedBinding: []string{existingBinding, "/host:/volume"},
		},
		"volume without absolute path and without basePath specified": {
			volume:          "/host:volume",
			parsedVolume:    &parser.Volume{Source: "/host", Destination: "volume"},
			expectedBinding: []string{existingBinding, "/host:volume"},
		},
		"volume without absolute path and with basePath specified": {
			volume:          "/host:volume",
			parsedVolume:    &parser.Volume{Source: "/host", Destination: "volume"},
			basePath:        "/builds/project",
			expectedBinding: []string{existingBinding, "/host:/builds/project/volume"},
		},
		"duplicated volume specification": {
			volume:          "/host/new:/duplicated",
			parsedVolume:    &parser.Volume{Source: "/host/new", Destination: "/duplicated"},
			expectedBinding: []string{existingBinding},
			expectedError:   NewErrVolumeAlreadyDefined("/duplicated"),
		},
		"volume with mode specified": {
			volume:          "/host/new:/my/path:ro",
			parsedVolume:    &parser.Volume{Source: "/host/new", Destination: "/my/path", Mode: "ro"},
			expectedBinding: []string{existingBinding, "/host/new:/my/path:ro"},
		},
		"root volume specified": {
			volume:          "/host/new:/:ro",
			parsedVolume:    &parser.Volume{Source: "/host/new", Destination: "/", Mode: "ro"},
			expectedBinding: []string{existingBinding},
			expectedError:   errDirectoryIsRootPath,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BasePath: testCase.basePath,
			}

			m := newDefaultManager(t, config)

			volumeParser := addUnixParser(t, m)

			volumeParser.On("ParseVolume", existingBinding).
				Return(&parser.Volume{Source: "/host", Destination: "/duplicated"}, nil).
				Once()

			err := m.Create(context.Background(), existingBinding)
			require.NoError(t, err)

			if testCase.volume != "" {
				volumeParser.On("ParseVolume", testCase.volume).
					Return(testCase.parsedVolume, nil).
					Once()
			}

			err = m.Create(context.Background(), testCase.volume)
			assert.ErrorIs(t, err, testCase.expectedError)
			assert.Equal(t, testCase.expectedBinding, m.volumeBindings)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_Disabled(t *testing.T) {
	expectedBinding := []string{"/host:/duplicated"}

	testCases := map[string]struct {
		volume       string
		parsedVolume *parser.Volume
		basePath     string

		expectedError error
	}{
		"no volumes specified": {
			volume: "",
		},
		"volume with absolute path, without basePath and with disableCache": {
			volume:        "/volume",
			parsedVolume:  &parser.Volume{Destination: "/volume"},
			basePath:      "",
			expectedError: ErrCacheVolumesDisabled,
		},
		"volume with absolute path, with basePath and with disableCache": {
			volume:        "/volume",
			parsedVolume:  &parser.Volume{Destination: "/volume"},
			basePath:      "/builds/project",
			expectedError: ErrCacheVolumesDisabled,
		},
		"volume without absolute path, without basePath and with disableCache": {
			volume:        "volume",
			parsedVolume:  &parser.Volume{Destination: "volume"},
			expectedError: ErrCacheVolumesDisabled,
		},
		"volume without absolute path, with basePath and with disableCache": {
			volume:        "volume",
			parsedVolume:  &parser.Volume{Destination: "volume"},
			basePath:      "/builds/project",
			expectedError: ErrCacheVolumesDisabled,
		},
		"duplicated volume definition": {
			volume:        "/duplicated",
			parsedVolume:  &parser.Volume{Destination: "/duplicated"},
			basePath:      "",
			expectedError: ErrCacheVolumesDisabled,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BasePath:     testCase.basePath,
				DisableCache: true,
			}

			m := newDefaultManager(t, config)

			volumeParser := addUnixParser(t, m)

			volumeParser.On("ParseVolume", "/host:/duplicated").
				Return(&parser.Volume{Source: "/host", Destination: "/duplicated"}, nil).
				Once()

			err := m.Create(context.Background(), "/host:/duplicated")
			require.NoError(t, err)

			if testCase.volume != "" {
				volumeParser.On("ParseVolume", testCase.volume).
					Return(testCase.parsedVolume, nil).
					Once()
			}

			err = m.Create(context.Background(), testCase.volume)
			assert.ErrorIs(t, err, testCase.expectedError)
			assert.Equal(t, expectedBinding, m.volumeBindings)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_HostBased(t *testing.T) {
	existingBinding := "/host:/duplicated"

	testCases := map[string]struct {
		volume     string
		basePath   string
		uniqueName string
		protected  bool

		expectedBinding []string
		expectedError   error
	}{
		"volume with absolute path, without basePath": {
			volume:     "/volume",
			uniqueName: "uniq",
			expectedBinding: []string{
				existingBinding,
				"/cache/uniq/14331bf18c8e434c4b3f48a8c5cc79aa:/volume",
			},
		},
		"volume with absolute path, with basePath": {
			volume:     "/volume",
			basePath:   "/builds/project",
			uniqueName: "uniq",
			expectedBinding: []string{
				existingBinding,
				"/cache/uniq/14331bf18c8e434c4b3f48a8c5cc79aa:/volume",
			},
		},
		"volume without absolute path, without basePath": {
			volume:     "volume",
			uniqueName: "uniq",
			expectedBinding: []string{
				existingBinding,
				"/cache/uniq/210ab9e731c9c36c2c38db15c28a8d1c:volume",
			},
		},
		"volume without absolute path, with basePath": {
			volume:     "volume",
			basePath:   "/builds/project",
			uniqueName: "uniq",
			expectedBinding: []string{
				existingBinding,
				"/cache/uniq/f69aef9fb01e88e6213362a04877452d:/builds/project/volume",
			},
		},
		"duplicated volume definition": {
			volume:          "/duplicated",
			uniqueName:      "uniq",
			expectedBinding: []string{existingBinding},
			expectedError:   NewErrVolumeAlreadyDefined("/duplicated"),
		},
		"volume is root": {
			volume:          "/",
			expectedBinding: []string{existingBinding},
			expectedError:   errDirectoryIsRootPath,
		},
		"protected": {
			volume:     "some-volume",
			basePath:   "/some/base/path",
			uniqueName: "some-unique-name",
			protected:  true,
			expectedBinding: []string{
				existingBinding,
				"/cache/some-unique-name/804b0f6b0d757899a37145f9d7f3848e-protected:/some/base/path/some-volume",
			},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BasePath:     testCase.basePath,
				DisableCache: false,
				CacheDir:     "/cache",
				UniqueName:   testCase.uniqueName,
				Protected:    testCase.protected,
			}

			m := newDefaultManager(t, config)

			volumeParser := addUnixParser(t, m)

			volumeParser.On("ParseVolume", existingBinding).
				Return(&parser.Volume{Source: "/host", Destination: "/duplicated"}, nil).
				Once()

			err := m.Create(context.Background(), existingBinding)
			require.NoError(t, err)

			volumeParser.On("ParseVolume", testCase.volume).
				Return(&parser.Volume{Destination: testCase.volume}, nil).
				Once()

			err = m.Create(context.Background(), testCase.volume)
			assert.ErrorIs(t, err, testCase.expectedError)
			assert.Equal(t, testCase.expectedBinding, m.volumeBindings)
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_VolumeBased(t *testing.T) {
	existingBinding := "/host:/duplicated"

	testCases := map[string]struct {
		volume     string
		basePath   string
		uniqueName string
		protected  bool

		expectedVolumeCreateOpts *volume.CreateOptions
		expectedBindings         []string
		expectedError            error
	}{
		"volume with absolute path, without basePath and with existing volume": {
			volume:     "/volume",
			basePath:   "",
			uniqueName: "uniq",
			expectedVolumeCreateOpts: testVolumeCreatOpts("uniq-cache-14331bf18c8e434c4b3f48a8c5cc79aa", map[string]string{
				"destination": "/volume",
			}),
			expectedBindings: []string{
				existingBinding,
				"uniq-cache-14331bf18c8e434c4b3f48a8c5cc79aa:/volume",
			},
		},
		"volume without absolute path, with basePath": {
			volume:     "volume",
			basePath:   "/builds/project",
			uniqueName: "uniq",
			expectedVolumeCreateOpts: testVolumeCreatOpts("uniq-cache-f69aef9fb01e88e6213362a04877452d", map[string]string{
				"destination": "/builds/project/volume",
			}),
			expectedBindings: []string{
				existingBinding,
				"uniq-cache-f69aef9fb01e88e6213362a04877452d:/builds/project/volume",
			},
		},
		"volume is root": {
			volume:        "/",
			basePath:      "",
			uniqueName:    "uniq",
			expectedError: errDirectoryIsRootPath,
		},
		"duplicated volume definition": {
			volume:        "/duplicated",
			uniqueName:    "uniq",
			expectedError: NewErrVolumeAlreadyDefined("/duplicated"),
		},
		"protected": {
			volume:     "some/volume",
			basePath:   "/some/base/path",
			uniqueName: "some-unique-name",
			protected:  true,
			expectedVolumeCreateOpts: testVolumeCreatOpts("some-unique-name-cache-7ee4ee58453a23f50e3e88641d9e4690-protected", map[string]string{
				"destination": "/some/base/path/some/volume",
				"protected":   "true",
			}),
			expectedBindings: []string{
				existingBinding,
				"some-unique-name-cache-7ee4ee58453a23f50e3e88641d9e4690-protected:/some/base/path/some/volume",
			},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BasePath:     testCase.basePath,
				UniqueName:   testCase.uniqueName,
				DisableCache: false,
				Protected:    testCase.protected,
			}

			m := newDefaultManager(t, config)
			volumeParser := addUnixParser(t, m)
			mClient := docker.NewMockClient(t)
			m.client = mClient

			volumeParser.On("ParseVolume", existingBinding).
				Return(&parser.Volume{Source: "/host", Destination: "/duplicated"}, nil).
				Once()
			volumeParser.On("ParseVolume", testCase.volume).
				Return(&parser.Volume{Destination: testCase.volume}, nil).
				Once()

			if createOpts := testCase.expectedVolumeCreateOpts; createOpts != nil {
				mClient.
					On("VolumeCreate", mock.Anything, *createOpts).
					Return(volume.Volume{Name: createOpts.Name}, nil).
					Once()
			}

			err := m.Create(context.Background(), existingBinding)
			require.NoError(t, err)

			err = m.Create(context.Background(), testCase.volume)
			if testCase.expectedError != nil {
				assert.ErrorIs(t, err, testCase.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedBindings, m.Binds())
		})
	}
}

func TestDefaultManager_CreateUserVolumes_CacheVolume_VolumeBased_WithError(t *testing.T) {
	testErr := errors.New("test-error")
	config := ManagerConfig{
		BasePath:   "/builds/project",
		UniqueName: "unique",
	}

	m := newDefaultManager(t, config)
	volumeParser := addUnixParser(t, m)
	mClient := docker.NewMockClient(t)
	m.client = mClient

	expectedCreateOpts := testVolumeCreatOpts("unique-cache-f69aef9fb01e88e6213362a04877452d", map[string]string{
		"destination": "/builds/project/volume",
	})
	mClient.
		On("VolumeCreate", mock.Anything, *expectedCreateOpts).
		Return(volume.Volume{}, testErr).
		Once()

	volumeParser.On("ParseVolume", "volume").
		Return(&parser.Volume{Destination: "volume"}, nil).
		Once()

	err := m.Create(context.Background(), "volume")
	assert.ErrorIs(t, err, testErr)
}

func TestDefaultManager_CreateUserVolumes_ParserError(t *testing.T) {
	testErr := errors.New("parser-test-error")
	m := newDefaultManager(t, ManagerConfig{})

	volumeParser := parser.NewMockParser(t)
	m.parser = volumeParser

	volumeParser.On("ParseVolume", "volume").
		Return(nil, testErr).
		Once()

	err := m.Create(context.Background(), "volume")
	assert.ErrorIs(t, err, testErr)
}

func TestDefaultManager_CreateTemporary(t *testing.T) {
	volumeCreateErr := errors.New("volume-create")
	existingBinding := "/host:/duplicated"

	testCases := map[string]struct {
		volume          string
		volumeCreateErr error
		protected       bool

		expectedVolumeCreateOpts *volume.CreateOptions
		expectedBindings         []string
		expectedTemporary        []string
		expectedError            error
	}{
		"volume created": {
			volume: "volume",
			expectedVolumeCreateOpts: testVolumeCreatOpts("temporary-cache-f69aef9fb01e88e6213362a04877452d", map[string]string{
				"destination": "/builds/project/volume",
			}),
			expectedBindings: []string{
				existingBinding,
				"temporary-cache-f69aef9fb01e88e6213362a04877452d:/builds/project/volume",
			},
		},
		"volume root": {
			volume:        "/",
			expectedError: errDirectoryIsRootPath,
		},
		"volume creation error": {
			volume: "volume",
			expectedVolumeCreateOpts: testVolumeCreatOpts("temporary-cache-f69aef9fb01e88e6213362a04877452d", map[string]string{
				"destination": "/builds/project/volume",
			}),
			volumeCreateErr: volumeCreateErr,
			expectedError:   volumeCreateErr,
		},
		"duplicated volume definition": {
			volume:        "/duplicated",
			expectedError: &ErrVolumeAlreadyDefined{},
		},
		"protected": {
			volume:    "some/volume",
			protected: true,
			expectedVolumeCreateOpts: testVolumeCreatOpts("temporary-cache-12b6275e06323d2d4872c0c352d0c7dd-protected", map[string]string{
				"destination": "/builds/project/some/volume",
				"protected":   "true",
			}),
			expectedBindings: []string{
				existingBinding,
				"temporary-cache-12b6275e06323d2d4872c0c352d0c7dd-protected:/builds/project/some/volume",
			},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BasePath:      "/builds/project",
				TemporaryName: "temporary",
				Protected:     testCase.protected,
			}

			m := newDefaultManager(t, config)
			volumeParser := addUnixParser(t, m)
			mClient := docker.NewMockClient(t)
			m.client = mClient

			volumeParser.On("ParseVolume", existingBinding).
				Return(&parser.Volume{Source: "/host", Destination: "/duplicated"}, nil).
				Once()

			var expectedVolumeName string
			if createOpts := testCase.expectedVolumeCreateOpts; createOpts != nil {
				expectedVolumeName = createOpts.Name
				mClient.
					On("VolumeCreate", mock.Anything, *createOpts).
					Return(volume.Volume{Name: createOpts.Name}, testCase.volumeCreateErr).
					Once()
			}

			err := m.Create(context.Background(), existingBinding)
			require.NoError(t, err)

			err = m.CreateTemporary(context.Background(), testCase.volume)
			if testCase.expectedError != nil {
				assert.ErrorIs(t, err, testCase.expectedError)
				return
			}

			require.Len(t, m.temporaryVolumes, 1)
			assert.Equal(t, m.temporaryVolumes[0], expectedVolumeName)
			assert.ErrorIs(t, err, testCase.expectedError)
			assert.Equal(t, testCase.expectedBindings, m.Binds())
		})
	}
}

func TestDefaultManager_RemoveTemporary(t *testing.T) {
	testErr := errors.New("test-err")
	testCases := map[string]struct {
		temporaryVolumes []string
		clientAssertions func(*docker.MockClient)
		expectedError    error
	}{
		"no volumes to remove": {
			temporaryVolumes: []string{},
			clientAssertions: func(c *docker.MockClient) {},
			expectedError:    nil,
		},
		"all volumes removed": {
			temporaryVolumes: []string{"volume1", "volume2", "volume3"},
			clientAssertions: func(c *docker.MockClient) {
				c.On("VolumeRemove", mock.Anything, "volume1", true).Return(nil).Once()
				c.On("VolumeRemove", mock.Anything, "volume2", true).Return(nil).Once()
				c.On("VolumeRemove", mock.Anything, "volume3", true).Return(nil).Once()
			},
			expectedError: nil,
		},
		"volume not found": {
			temporaryVolumes: []string{"nonexistent-volume"},
			clientAssertions: func(c *docker.MockClient) {
				c.On("VolumeRemove", mock.Anything, "nonexistent-volume", true).Return(&test.NotFoundError{}).Once()
			},
			expectedError: &test.NotFoundError{},
		},
		"failed to remove volume": {
			temporaryVolumes: []string{"volume-name-1"},
			clientAssertions: func(c *docker.MockClient) {
				c.On("VolumeRemove", mock.Anything, "volume-name-1", true).Return(testErr).Once()
			},
			expectedError: testErr,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			mClient := docker.NewMockClient(t)

			testCase.clientAssertions(mClient)

			m := newDefaultManager(t, ManagerConfig{})
			m.client = mClient
			m.temporaryVolumes = testCase.temporaryVolumes

			err := m.RemoveTemporary(context.Background())
			assert.ErrorIs(t, err, testCase.expectedError)
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

func testVolumeCreatOpts(name string, additionalLabels map[string]string) *volume.CreateOptions {
	const pre = "com.gitlab.gitlab-runner"
	labels := map[string]string{
		pre + ".type":            "cache",
		pre + ".job.before_sha":  "",
		pre + ".job.id":          "0",
		pre + ".job.ref":         "",
		pre + ".job.sha":         "",
		pre + ".job.url":         "/-/jobs/0",
		pre + ".job.timeout":     "2h0m0s",
		pre + ".managed":         "true",
		pre + ".pipeline.id":     "",
		pre + ".project.id":      "0",
		pre + ".protected":       "false",
		pre + ".runner.id":       "",
		pre + ".runner.local_id": "0",
		pre + ".destination":     "",
	}

	for k, v := range additionalLabels {
		labels[pre+"."+k] = v
	}

	return &volume.CreateOptions{
		Name:   name,
		Labels: labels,
	}
}
