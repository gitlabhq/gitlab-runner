package volumes

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
