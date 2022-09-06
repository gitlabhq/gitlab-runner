//go:build !integration

package helperimage

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/windows"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/errors"
)

func TestGetInfo(t *testing.T) {
	const unsupportedVersion = "9.9"

	tests := []struct {
		osType        string
		version       string
		expectedError error
	}{
		{osType: OSTypeLinux, expectedError: nil},
		{
			osType:        OSTypeWindows,
			version:       unsupportedVersion,
			expectedError: windows.NewUnsupportedWindowsVersionError(unsupportedVersion),
		},
		{osType: "unsupported", expectedError: errors.NewErrOSNotSupported("unsupported")},
	}

	for _, test := range tests {
		t.Run(test.osType, func(t *testing.T) {
			_, err := Get(headRevision, Config{OSType: test.osType, OperatingSystem: test.version})

			assert.ErrorIs(t, err, test.expectedError)
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
	tests := []struct {
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

	for _, test := range tests {
		t.Run(test.revision, func(t *testing.T) {
			assert.Equal(t, test.expectedTag, imageRevision(test.revision))
		})
	}
}
