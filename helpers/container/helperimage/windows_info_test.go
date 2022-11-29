//go:build !integration

package helperimage

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/windows"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

func Test_windowsInfo_create(t *testing.T) {
	revision := "4011f186"

	for _, shell := range []string{"", shells.SNPowershell, shells.SNPwsh} {
		expectedPowershellCmdLine := getPowerShellCmd(shell)
		if shell == "" {
			assert.Equal(t, shells.SNPowershell, expectedPowershellCmdLine[0])
		}

		tests := []struct {
			operatingSystem string
			shell           string
			expectedInfo    Info
			expectedErr     error
		}{
			{
				operatingSystem: "Windows Server 2019 Datacenter Evaluation Version 1809 (OS Build 17763.316)",
				expectedInfo: Info{
					Architecture: windowsSupportedArchitecture,
					Name:         GitLabRegistryName,
					Tag: fmt.Sprintf(
						"%s-%s-%s",
						windowsSupportedArchitecture,
						revision,
						baseImage1809,
					),
					IsSupportingLocalImport: false,
					Cmd:                     expectedPowershellCmdLine,
				},
				expectedErr: nil,
			},
			{
				operatingSystem: "Windows Server Datacenter Version 1809 (OS Build 1803.590)",
				expectedInfo: Info{
					Architecture: windowsSupportedArchitecture,
					Name:         GitLabRegistryName,
					Tag: fmt.Sprintf(
						"%s-%s-%s",
						windowsSupportedArchitecture,
						revision,
						baseImage1809,
					),
					IsSupportingLocalImport: false,
					Cmd:                     expectedPowershellCmdLine,
				},
				expectedErr: nil,
			},
			{
				operatingSystem: "Windows 10 Pro Version 2004 (OS Build 19041.329)",
				expectedInfo: Info{
					Architecture: windowsSupportedArchitecture,
					Name:         GitLabRegistryName,
					Tag: fmt.Sprintf(
						"%s-%s-%s",
						windowsSupportedArchitecture,
						revision,
						baseImage2004,
					),
					IsSupportingLocalImport: false,
					Cmd:                     expectedPowershellCmdLine,
				},
				expectedErr: nil,
			},
			{
				operatingSystem: "Microsoft Windows Server Version 21H2 (OS Build 20348.169)",
				expectedInfo: Info{
					Architecture: windowsSupportedArchitecture,
					Name:         GitLabRegistryName,
					Tag: fmt.Sprintf(
						"%s-%s-%s",
						windowsSupportedArchitecture,
						revision,
						baseImage21H1,
					),
					IsSupportingLocalImport: false,
					Cmd:                     expectedPowershellCmdLine,
				},
				expectedErr: nil,
			},
			{
				operatingSystem: "some random string",
				expectedErr:     windows.NewUnsupportedWindowsVersionError("some random string"),
			},
		}

		t.Run(shell, func(t *testing.T) {
			for _, test := range tests {
				t.Run(test.operatingSystem, func(t *testing.T) {
					w := new(windowsInfo)

					image, err := w.Create(
						revision,
						Config{
							OperatingSystem: test.operatingSystem,
							Shell:           shell,
						},
					)

					assert.Equal(t, test.expectedInfo, image)
					assert.ErrorIs(t, err, test.expectedErr)
				})
			}
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
	require.ErrorAs(t, err, &unsupportedErr)
	assert.Equal(t, unsupportedVersion, unsupportedErr.Version)
}
