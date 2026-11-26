//go:build !integration

package helperimage

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

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
			architecture  string
			overridden    bool
			expectedInfo  Info
			expectedErr   error
		}{
			{
				kernelVersion: "10.0 17763 (17763.1.amd64fre.rs5_release.180914-1434)",
				architecture:  "amd64",
				expectedInfo: Info{
					Architecture: "x86_64",
					Name:         GitLabRegistryName,
					Tag:          fmt.Sprintf("x86_64-%s-servercore1809", revision),
					Prebuilt:     "prebuilt-windows-servercore-ltsc2019-x86_64",
					Cmd:          expectedPowershellCmdLine,
				},
			},
			{
				kernelVersion: "10.0 20348 (20348.1.amd64fre.fe_release.210507-1500)",
				architecture:  "amd64",
				expectedInfo: Info{
					Architecture: "x86_64",
					Name:         GitLabRegistryName,
					Tag:          fmt.Sprintf("x86_64-%s-servercore21H2", revision),
					Prebuilt:     "prebuilt-windows-servercore-ltsc2022-x86_64",
					Cmd:          expectedPowershellCmdLine,
				},
			},
			{
				kernelVersion: "10.0 26100 (26100.1.amd64fre.ge_release.240331-1435)",
				architecture:  "amd64",
				expectedInfo: Info{
					Architecture: "x86_64",
					Name:         GitLabRegistryName,
					Tag:          fmt.Sprintf("x86_64-%s-servercore24H2", revision),
					Prebuilt:     "prebuilt-windows-servercore-ltsc2025-x86_64",
					Cmd:          expectedPowershellCmdLine,
				},
			},
			{
				// Windows 11 build 26200 is version 25H2; it shares the 24H2
				// servicing branch and maps to V24H2, so an arm64 25H2 host
				// resolves to the servercore24H2 / ltsc2025 arm64 image.
				kernelVersion: "10.0 26200 (26100.1.arm64fre.ge_release.240331-1435)",
				architecture:  "arm64",
				expectedInfo: Info{
					Architecture: "arm64",
					Name:         GitLabRegistryName,
					Tag:          fmt.Sprintf("arm64-%s-servercore24H2", revision),
					Prebuilt:     "prebuilt-windows-servercore-ltsc2025-arm64",
					Cmd:          expectedPowershellCmdLine,
				},
			},
			{
				// aarch64 normalises to arm64.
				kernelVersion: "10.0 26100 (26100.1.arm64fre.ge_release.240331-1435)",
				architecture:  "aarch64",
				expectedInfo: Info{
					Architecture: "arm64",
					Name:         GitLabRegistryName,
					Tag:          fmt.Sprintf("arm64-%s-servercore24H2", revision),
					Prebuilt:     "prebuilt-windows-servercore-ltsc2025-arm64",
					Cmd:          expectedPowershellCmdLine,
				},
			},
			{
				// arm64 on Windows Server 2019 has no published arm64 base
				// image, so the runner falls back to the x86_64 image.
				kernelVersion: "10.0 17763 (17763.1.amd64fre.rs5_release.180914-1434)",
				architecture:  "arm64",
				expectedInfo: Info{
					Architecture: "x86_64",
					Name:         GitLabRegistryName,
					Tag:          fmt.Sprintf("x86_64-%s-servercore1809", revision),
					Prebuilt:     "prebuilt-windows-servercore-ltsc2019-x86_64",
					Cmd:          expectedPowershellCmdLine,
				},
			},
			{
				// arm64 on Windows Server 2022 has no published arm64 base
				// image, so the runner falls back to the x86_64 image.
				kernelVersion: "10.0 20348 (20348.1.amd64fre.fe_release.210507-1500)",
				architecture:  "arm64",
				expectedInfo: Info{
					Architecture: "x86_64",
					Name:         GitLabRegistryName,
					Tag:          fmt.Sprintf("x86_64-%s-servercore21H2", revision),
					Prebuilt:     "prebuilt-windows-servercore-ltsc2022-x86_64",
					Cmd:          expectedPowershellCmdLine,
				},
			},
			{
				kernelVersion: "some random string",
				architecture:  "amd64",
				expectedErr:   windows.ErrUnsupportedWindowsVersion,
			},
			{
				// unsupported + overridden: partial Info with warning, empty Name/Tag/Prebuilt
				kernelVersion: "10.0 17134 (17134.1)",
				architecture:  "amd64",
				overridden:    true,
				expectedInfo:  Info{Architecture: "x86_64", Cmd: expectedPowershellCmdLine, Warnings: []string{"unsupported Windows version: 10.0 17134 (17134.1); using the configured helper_image"}},
			},
			{
				// overridden on a supported kernel: full Info unchanged, Warnings nil
				kernelVersion: "10.0 17763 (17763.1.amd64fre.rs5_release.180914-1434)",
				architecture:  "amd64",
				overridden:    true,
				expectedInfo:  Info{Architecture: "x86_64", Name: GitLabRegistryName, Tag: fmt.Sprintf("x86_64-%s-servercore1809", revision), Prebuilt: "prebuilt-windows-servercore-ltsc2019-x86_64", Cmd: expectedPowershellCmdLine},
			},
			{
				// An unspecified architecture preserves historical behaviour:
				// the x86_64 helper image is selected.
				kernelVersion: "10.0 20348 (20348.1.amd64fre.fe_release.210507-1500)",
				architecture:  "",
				expectedInfo: Info{
					Architecture: "x86_64",
					Name:         GitLabRegistryName,
					Tag:          fmt.Sprintf("x86_64-%s-servercore21H2", revision),
					Prebuilt:     "prebuilt-windows-servercore-ltsc2022-x86_64",
					Cmd:          expectedPowershellCmdLine,
				},
			},
			{
				// build 26400 falls back to V24H2 via 26300; VersionFallback=true, Warnings set
				kernelVersion: "10.0 26400 (26400.1.amd64fre.ge_release.240331-1435)",
				architecture:  "amd64",
				expectedInfo: Info{
					Architecture:    "x86_64",
					Name:            GitLabRegistryName,
					Tag:             fmt.Sprintf("x86_64-%s-servercore24H2", revision),
					Prebuilt:        "prebuilt-windows-servercore-ltsc2025-x86_64",
					Cmd:             expectedPowershellCmdLine,
					VersionFallback: true,
					Warnings:        []string{"Windows build 10.0.26400 is not a known build; using the servercore24H2 helper image (closest known build 10.0.26300). If the helper container fails to start, configure helper_image explicitly."},
				},
			},
			{
				// fallback + overridden: full Info (fallback tag), Warnings nil, VersionFallback false
				kernelVersion: "10.0 26400 (26400.1.amd64fre.ge_release.240331-1435)",
				architecture:  "amd64",
				overridden:    true,
				expectedInfo:  Info{Architecture: "x86_64", Name: GitLabRegistryName, Tag: fmt.Sprintf("x86_64-%s-servercore24H2", revision), Prebuilt: "prebuilt-windows-servercore-ltsc2025-x86_64", Cmd: expectedPowershellCmdLine},
			},
		}

		t.Run(shell, func(t *testing.T) {
			for _, test := range tests {
				t.Run(fmt.Sprintf("%s/%s/overridden=%v", test.kernelVersion, test.architecture, test.overridden), func(t *testing.T) {
					w := new(windowsInfo)

					image, err := w.Create(
						revision,
						Config{
							KernelVersion:         test.kernelVersion,
							Architecture:          test.architecture,
							Shell:                 shell,
							HelperImageOverridden: test.overridden,
						},
					)

					assert.Equal(t, test.expectedInfo, image)
					assert.ErrorIs(t, err, test.expectedErr)
				})
			}
		})
	}
}
