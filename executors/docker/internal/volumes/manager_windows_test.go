//go:build !integration

package volumes

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/volume"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/path"
)

func TestDefaultManager_CreateUserVolumes_CacheVolume_VolumeBased_Windows(t *testing.T) {
	const existingBinding = `\\.\pipe\host:\\.\pipe\duplicated`

	testCases := map[string]struct {
		volume     string
		basePath   string
		uniqueName string

		expectedVolumeName string
		expectedBindings   []string
		expectedError      error
	}{
		"pipe name volume specified": {
			volume:             `\\.\pipe\docker_engine`,
			uniqueName:         "uniq",
			expectedVolumeName: "uniq-cache-8abd376d059fcf32b6258f48c760885d",
			expectedBindings:   []string{`\\.\pipe\host:\\.\pipe\duplicated`, `uniq-cache-8abd376d059fcf32b6258f48c760885d:\\.\pipe\docker_engine`},
			expectedError:      nil,
		},
		"duplicate pipe name volume specified": {
			volume:             `\\.\pipe\duplicated`,
			uniqueName:         "uniq",
			expectedVolumeName: "uniq-cache-8abd376d059fcf32b6258f48c760885d",
			expectedBindings:   []string{`\\.\pipe\host:\\.\pipe\duplicated`, `uniq-cache-8abd376d059fcf32b6258f48c760885d:\\.\pipe\docker_engine`},
			expectedError:      NewErrVolumeAlreadyDefined(`\\.\pipe\duplicated`),
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			config := ManagerConfig{
				BasePath:     testCase.basePath,
				UniqueName:   testCase.uniqueName,
				DisableCache: false,
			}

			m := newDefaultManager(config)
			volumeParser := addParser(m, path.NewWindowsPath())
			mClient := new(docker.MockClient)
			m.client = mClient

			defer func() {
				mClient.AssertExpectations(t)
				volumeParser.AssertExpectations(t)
			}()

			existingBindingParts := strings.Split(existingBinding, ":")
			volumeParser.On("ParseVolume", existingBinding).
				Return(&parser.Volume{Source: existingBindingParts[0], Destination: existingBindingParts[1]}, nil).
				Once()
			volumeParser.On("ParseVolume", testCase.volume).
				Return(&parser.Volume{Destination: testCase.volume}, nil).
				Once()

			if testCase.expectedError == nil {
				mClient.On(
					"VolumeCreate",
					mock.Anything,
					mock.MatchedBy(func(v volume.CreateOptions) bool {
						return testCreateOptionsContent(v, testCase.expectedVolumeName)
					}),
				).
					Return(volume.Volume{Name: testCase.expectedVolumeName}, nil).
					Once()
			}

			err := m.Create(context.Background(), existingBinding)
			require.NoError(t, err)

			err = m.Create(context.Background(), testCase.volume)
			if testCase.expectedError != nil {
				assert.True(
					t,
					errors.Is(err, testCase.expectedError),
					"expected err %T, but got %T",
					testCase.expectedError,
					err,
				)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedBindings, m.Binds())
		})
	}
}
