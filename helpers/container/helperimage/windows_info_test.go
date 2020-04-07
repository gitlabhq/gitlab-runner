package helperimage

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/windows"
)

func Test_windowsInfo_create(t *testing.T) {
	revision := "4011f186"
	tests := []struct {
		operatingSystem string
		expectedInfo    Info
		expectedErr     error
	}{
		{
			operatingSystem: "Windows Server Datacenter Version 1803 (OS Build 17134.590)",
			expectedInfo: Info{
				Architecture:            windowsSupportedArchitecture,
				Name:                    name,
				Tag:                     fmt.Sprintf("%s-%s-%s", windowsSupportedArchitecture, revision, baseImage1803),
				IsSupportingLocalImport: false,
				Cmd:                     powerShellCmd,
			},
			expectedErr: nil,
		},
		{
			operatingSystem: "Windows Server 2019 Datacenter Evaluation Version 1809 (OS Build 17763.316)",
			expectedInfo: Info{
				Architecture:            windowsSupportedArchitecture,
				Name:                    name,
				Tag:                     fmt.Sprintf("%s-%s-%s", windowsSupportedArchitecture, revision, baseImage1809),
				IsSupportingLocalImport: false,
				Cmd:                     powerShellCmd,
			},
			expectedErr: nil,
		},
		{
			operatingSystem: "Windows Server Datacenter Version 1809 (OS Build 1803.590)",
			expectedInfo: Info{
				Architecture:            windowsSupportedArchitecture,
				Name:                    name,
				Tag:                     fmt.Sprintf("%s-%s-%s", windowsSupportedArchitecture, revision, baseImage1809),
				IsSupportingLocalImport: false,
				Cmd:                     powerShellCmd,
			},
			expectedErr: nil,
		},
		{
			operatingSystem: "Windows Server Datacenter Version 1903 (OS Build 18362.592)",
			expectedInfo: Info{
				Architecture:            windowsSupportedArchitecture,
				Name:                    name,
				Tag:                     fmt.Sprintf("%s-%s-%s", windowsSupportedArchitecture, revision, baseImage1903),
				IsSupportingLocalImport: false,
				Cmd:                     powerShellCmd,
			},
			expectedErr: nil,
		},
		{
			operatingSystem: "Windows Server Datacenter Version 1909 (OS Build 18363.720)",
			expectedInfo: Info{
				Architecture:            windowsSupportedArchitecture,
				Name:                    name,
				Tag:                     fmt.Sprintf("%s-%s-%s", windowsSupportedArchitecture, revision, baseImage1909),
				IsSupportingLocalImport: false,
				Cmd:                     powerShellCmd,
			},
			expectedErr: nil,
		},
		{
			operatingSystem: "some random string",
			expectedErr:     windows.NewUnsupportedWindowsVersionError("some random string"),
		},
	}

	for _, test := range tests {
		t.Run(test.operatingSystem, func(t *testing.T) {
			w := new(windowsInfo)

			image, err := w.Create(revision, Config{OperatingSystem: test.operatingSystem})

			assert.Equal(t, test.expectedInfo, image)
			assert.True(t, errors.Is(err, test.expectedErr), "expected err %T, but got %T", test.expectedErr, err)
		})
	}
}

func Test_windowsInfo_baseImage_NoSupportedVersion(t *testing.T) {
	oldHelperImages := helperImages
	defer func() {
		helperImages = oldHelperImages
	}()

	helperImages = map[string]string{
		windows.V1809: baseImage1809,
	}

	unsupportedVersion := "Windows Server Datacenter Version 1803 (OS Build 17134.590)"

	w := new(windowsInfo)
	_, err := w.baseImage(unsupportedVersion)
	var unsupportedErr *windows.UnsupportedWindowsVersionError
	require.True(t, errors.As(err, &unsupportedErr), "expected err %T, but got %T", unsupportedErr, err)
	assert.Equal(t, unsupportedVersion, unsupportedErr.Version)
}
