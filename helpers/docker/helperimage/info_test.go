package helperimage

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/errors"
)

func TestGetInfo(t *testing.T) {
	testCases := []struct {
		osType                  string
		expectedHelperImageType interface{}
		expectedError           interface{}
	}{
		{osType: OSTypeLinux, expectedError: nil},
		{osType: OSTypeWindows, expectedError: ErrUnsupportedOSVersion},
		{osType: "unsupported", expectedError: errors.NewErrOSNotSupported("unsupported")},
	}

	for _, testCase := range testCases {
		t.Run(testCase.osType, func(t *testing.T) {
			_, err := Get(headRevision, Config{OSType: testCase.osType})

			assert.Equal(t, testCase.expectedError, err)
		})
	}
}

func TestContainerImage_String(t *testing.T) {
	image := Info{
		Name: "abc",
		Tag:  "1234",
	}

	assert.Equal(t, "abc:1234", image.String())
}

func Test_imageRevision(t *testing.T) {
	testCases := []struct {
		revision    string
		expectedTag string
	}{
		{
			revision:    headRevision,
			expectedTag: latestImageRevision,
		},
		{
			revision:    "1234",
			expectedTag: "1234",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.revision, func(t *testing.T) {
			assert.Equal(t, testCase.expectedTag, imageRevision(testCase.revision))
		})
	}

}
