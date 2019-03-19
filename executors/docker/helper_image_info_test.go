package docker

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
)

func TestGetHelperImageInfo(t *testing.T) {
	testCases := []struct {
		osType                  string
		expectedHelperImageType interface{}
		expectedError           interface{}
	}{
		{osType: OSTypeLinux, expectedHelperImageType: &linuxHelperImageInfo{}, expectedError: nil},
		{osType: OSTypeWindows, expectedHelperImageType: &windowsHelperImageInfo{}, expectedError: nil},
		{osType: "unsupported", expectedHelperImageType: nil, expectedError: errUnsupportedOSType},
	}

	for _, testCase := range testCases {
		t.Run(testCase.osType, func(t *testing.T) {
			i, err := getHelperImageInfo(types.Info{OSType: testCase.osType})

			assert.IsType(t, testCase.expectedHelperImageType, i)
			assert.Equal(t, testCase.expectedError, err)
		})
	}
}
