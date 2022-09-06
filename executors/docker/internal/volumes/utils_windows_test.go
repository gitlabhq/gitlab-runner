//go:build !integration

package volumes

import (
	"testing"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
)

func TestIsHostMountedVolume_Windows(t *testing.T) {
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

	testIsHostMountedVolume(t, parser.NewWindowsParser(), testCases)
}
