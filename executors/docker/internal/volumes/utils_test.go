package volumes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsHostMountedVolume(t *testing.T) {
	testCases := map[string]struct {
		dir            string
		volumes        []string
		expectedResult bool
	}{
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
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			result := IsHostMountedVolume(testCase.dir, testCase.volumes...)
			assert.Equal(t, testCase.expectedResult, result)
		})
	}
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
