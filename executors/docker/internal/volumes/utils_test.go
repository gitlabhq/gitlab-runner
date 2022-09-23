//go:build !integration

package volumes

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
)

type isHostMountedVolumeTestCases map[string]isHostMountedVolumeTestCase

type isHostMountedVolumeTestCase struct {
	dir            string
	volumes        []string
	expectedResult bool
	expectedError  error
}

func testIsHostMountedVolume(t *testing.T, volumesParser parser.Parser, testCases isHostMountedVolumeTestCases) {
	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			result, err := IsHostMountedVolume(volumesParser, testCase.dir, testCase.volumes...)
			assert.Equal(t, testCase.expectedResult, result)
			if testCase.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, testCase.expectedError.Error())
			}
		})
	}
}

func TestIsHostMountedVolume_Linux(t *testing.T) {
	testCases := isHostMountedVolumeTestCases{
		"empty volumes": {
			dir:            "/test/to/checked/dir",
			volumes:        []string{},
			expectedResult: false,
		},
		"no host volumes": {
			dir:            "/test/to/checked/dir",
			volumes:        []string{"/tests/to"},
			expectedResult: false,
		},
		"dir not within volumes": {
			dir:            "/test/to/checked/dir",
			volumes:        []string{"/host:/root"},
			expectedResult: false,
		},
		"dir within volumes": {
			dir:            "/test/to/checked/dir",
			volumes:        []string{"/host:/test/to"},
			expectedResult: true,
		},
		"error on parsing": {
			dir:           "/test/to/checked/dir",
			volumes:       []string{""},
			expectedError: parser.NewInvalidVolumeSpecErr(""),
		},
	}

	testIsHostMountedVolume(t, parser.NewLinuxParser(), testCases)
}

func TestManagedList_Add(t *testing.T) {
	tests := map[string]struct {
		path          string
		expectedError error
	}{
		"add non-duplicated path": {
			path: "/new/path",
		},
		"add duplicated path": {
			path:          "/duplicate",
			expectedError: NewErrVolumeAlreadyDefined(filepath.FromSlash("/duplicate")),
		},
		"add non-normalized duplicated path": {
			path:          "/duplicate/",
			expectedError: NewErrVolumeAlreadyDefined(filepath.FromSlash("/duplicate")),
		},
		"add child path": {
			path: "/duplicate/child",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			m := pathList{}
			err := m.Add("/duplicate")
			require.NoError(t, err)

			err = m.Add(test.path)
			assert.Equal(t, test.expectedError, err)
		})
	}
}
