package docker

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
)

func TestGetHelperImage(t *testing.T) {
	testCases := []struct {
		osType                  string
		expectedHelperImageType interface{}
	}{
		{osType: "linux", expectedHelperImageType: &linuxHelperImage{}},
		{osType: "windows", expectedHelperImageType: &windowsHelperImage{}},
		{osType: "unsupported", expectedHelperImageType: &linuxHelperImage{}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.osType, func(t *testing.T) {
			i := getHelperImage(types.Info{OSType: testCase.osType})

			assert.IsType(t, testCase.expectedHelperImageType, i)
		})
	}
}
