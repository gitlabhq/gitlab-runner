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
			kernelVersion string
			shell         string
			expectedInfo  Info
			expectedErr   error
		}{
			{
				kernelVersion: "10.0 17763 (17763.1.amd64fre.rs5_release.180914-1434)",
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
				kernelVersion: "10.0 20348 (20348.1.amd64fre.fe_release.210507-1500)",
				expectedInfo: Info{
					Architecture: windowsSupportedArchitecture,
					Name:         GitLabRegistryName,
					Tag: fmt.Sprintf(
						"%s-%s-%s",
						windowsSupportedArchitecture,
						revision,
						baseImage21H2,
					),
					IsSupportingLocalImport: false,
					Cmd:                     expectedPowershellCmdLine,
				},
				expectedErr: nil,
			},
			{
				kernelVersion: "10.0.20348",
				expectedInfo: Info{
					Architecture: windowsSupportedArchitecture,
					Name:         GitLabRegistryName,
					Tag: fmt.Sprintf(
						"%s-%s-%s",
						windowsSupportedArchitecture,
						revision,
						baseImage21H2,
					),
					IsSupportingLocalImport: false,
					Cmd:                     expectedPowershellCmdLine,
				},
				expectedErr: nil,
			},
			{
				kernelVersion: "10.0 26100 (26100.1.amd64fre.ge_release.240331-1435)",
				expectedInfo: Info{
					Architecture: windowsSupportedArchitecture,
					Name:         GitLabRegistryName,
					Tag: fmt.Sprintf(
						"%s-%s-%s",
						windowsSupportedArchitecture,
						revision,
						baseImage21H2,
					),
					IsSupportingLocalImport: false,
					Cmd:                     expectedPowershellCmdLine,
				},
				expectedErr: nil,
			},
			{
				kernelVersion: "10.0 17134 (17134.1.amd64fre.rs4_release.180410-1804)",
				expectedErr:   windows.ErrUnsupportedWindowsVersion,
			},
			{
				kernelVersion: "some random string",
				expectedErr:   windows.ErrUnsupportedWindowsVersion,
			},
		}

		t.Run(shell, func(t *testing.T) {
			for _, test := range tests {
				t.Run(test.kernelVersion, func(t *testing.T) {
					w := new(windowsInfo)

					image, err := w.Create(
						revision,
						Config{
							KernelVersion: test.kernelVersion,
							Shell:         shell,
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

	unsupportedVersion := "10.0 17134 (17134.1.amd64fre.rs4_release.180410-1804)"

	w := new(windowsInfo)
	_, err := w.baseImage(unsupportedVersion)
	require.ErrorIs(t, err, windows.ErrUnsupportedWindowsVersion)
	require.Error(t, err, unsupportedVersion)
}
