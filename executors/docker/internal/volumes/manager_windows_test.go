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
		protected  bool

		expectedVolumeCreateOpts *volume.CreateOptions
		expectedBindings         []string
		expectedError            error
	}{
		"pipe name volume specified": {
			volume:     `\\.\pipe\docker_engine`,
			uniqueName: "uniq",
			expectedVolumeCreateOpts: testVolumeCreatOpts("uniq-cache-8abd376d059fcf32b6258f48c760885d", map[string]string{
				"destination": `\\.\pipe\docker_engine`,
			}),
			expectedBindings: []string{`\\.\pipe\host:\\.\pipe\duplicated`, `uniq-cache-8abd376d059fcf32b6258f48c760885d:\\.\pipe\docker_engine`},
		},
		"duplicate pipe name volume specified": {
			volume:        `\\.\pipe\duplicated`,
			uniqueName:    "uniq",
			expectedError: NewErrVolumeAlreadyDefined(`\\.\pipe\duplicated`),
		},
		"protected": {
			volume:     `\\.\pipe\docker_engine`,
			uniqueName: "uniq",
			protected:  true,
			expectedVolumeCreateOpts: testVolumeCreatOpts("uniq-cache-8abd376d059fcf32b6258f48c760885d-protected", map[string]string{
				"destination": `\\.\pipe\docker_engine`,
				"protected":   "true",
			}),
			expectedBindings: []string{`\\.\pipe\host:\\.\pipe\duplicated`, `uniq-cache-8abd376d059fcf32b6258f48c760885d-protected:\\.\pipe\docker_engine`},
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
			volumeParser := addParser(t, m, path.NewWindowsPath())
			mClient := docker.NewMockClient(t)
			m.client = mClient

			existingBindingParts := strings.Split(existingBinding, ":")
			volumeParser.On("ParseVolume", existingBinding).
				Return(&parser.Volume{Source: existingBindingParts[0], Destination: existingBindingParts[1]}, nil).
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
