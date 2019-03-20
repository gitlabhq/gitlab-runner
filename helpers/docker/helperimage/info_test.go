package helperimage

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
)

func TestGetInfo(t *testing.T) {
	testCases := []struct {
		osType                  string
		expectedHelperImageType interface{}
		expectedError           interface{}
	}{
		{osType: OSTypeLinux, expectedHelperImageType: &linuxInfo{}, expectedError: nil},
		{osType: OSTypeWindows, expectedHelperImageType: &windowsInfo{}, expectedError: nil},
		{osType: "unsupported", expectedHelperImageType: nil, expectedError: newUnsupportedOSTypeError("unsupported")},
	}

	for _, testCase := range testCases {
		t.Run(testCase.osType, func(t *testing.T) {
			i, err := GetInfo(types.Info{OSType: testCase.osType})

			assert.IsType(t, testCase.expectedHelperImageType, i)
			assert.Equal(t, testCase.expectedError, err)
		})
	}
}
