package volumes

import (
	"runtime"
	"testing"

	"github.com/docker/docker/api/types"
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

func skipOnOsType(t *testing.T, osType string) {
	if runtime.GOOS != osType {
		return
	}

	t.Skipf("skipping the test because running on %q OS type", osType)
}

func testIsHostMountedVolume(t *testing.T, osType string, testCases isHostMountedVolumeTestCases) {
	t.Run(osType, func(t *testing.T) {
		for testName, testCase := range testCases {
			t.Run(testName, func(t *testing.T) {
				volumesParser, err := parser.New(types.Info{OSType: osType})
				require.NoError(t, err)

				result, err := IsHostMountedVolume(volumesParser, testCase.dir, testCase.volumes...)
				assert.Equal(t, testCase.expectedResult, result)
				if testCase.expectedError == nil {
					assert.NoError(t, err)
				} else {
					assert.EqualError(t, err, testCase.expectedError.Error())
				}
			})
		}
	})
}

func TestIsHostMountedVolume_Linux(t *testing.T) {
	skipOnOsType(t, parser.OSTypeWindows)

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

	testIsHostMountedVolume(t, parser.OSTypeLinux, testCases)
}

func TestIsHostMountedVolume_Windows(t *testing.T) {
	skipOnOsType(t, parser.OSTypeLinux)

	testCases := isHostMountedVolumeTestCases{
		"empty volumes": {
			dir:            `c:\test\to\checked\dir`,
			volumes:        []string{},
			expectedResult: false,
		},
		"no host volumes": {
			dir:            `c:\test\to\checked\dir`,
			volumes:        []string{`c:\test\to`},
			expectedResult: false,
		},
		"dir not within volumes": {
			dir:            `c:\test\to\checked\dir`,
			volumes:        []string{`c:\host:c:\destination`},
			expectedResult: false,
		},
		"dir within volumes": {
			dir:            `c:\test\to\checked\dir`,
			volumes:        []string{`c:\host:c:\test\to`},
			expectedResult: true,
		},
		"error on parsing": {
			dir:           `c:\test\to\checked\dir`,
			volumes:       []string{""},
			expectedError: parser.NewInvalidVolumeSpecErr(""),
		},
	}

	testIsHostMountedVolume(t, parser.OSTypeWindows, testCases)
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
			expectedError: NewErrVolumeAlreadyDefined("/duplicate"),
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
