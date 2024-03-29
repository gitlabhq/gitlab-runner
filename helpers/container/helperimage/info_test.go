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
			expectedError: windows.ErrUnsupportedWindowsVersion,
		},
		{osType: "unsupported", expectedError: errors.NewErrOSNotSupported("unsupported")},
	}

	for _, test := range tests {
		t.Run(test.osType, func(t *testing.T) {
			_, err := Get("HEAD", Config{OSType: test.osType, KernelVersion: test.version})

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

func TestImageVersion(t *testing.T) {
	tests := []struct {
		version     string
		expectedTag string
	}{
		{version: "1.2.3", expectedTag: "v1.2.3"},
		{version: "16.6.0~beta.105.gd2263193", expectedTag: "v16.6.0"},
		{version: "v16.6.0~beta.105.gd2263193", expectedTag: "latest"},
		{version: "development", expectedTag: "latest"},
		{version: "", expectedTag: "latest"},
		{version: "head", expectedTag: "latest"},
	}

	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			assert.Equal(t, test.expectedTag, Version(test.version))
		})
	}
}
