//go:build !integration

package helperimage

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

func Test_linuxInfo_create(t *testing.T) {
	for _, shell := range []string{"sh", "bash", shells.SNPwsh} {
		expectedTagSuffix := ""
		expectedCmd := bashCmd
		if shell == shells.SNPwsh {
			expectedTagSuffix = "-pwsh"
			expectedCmd = getPowerShellCmd(shell)
		}

		tests := map[string]struct {
			shell        string
			dockerArch   string
			revision     string
			flavor       string
			expectedInfo Info
		}{
			"When dockerArch not specified we fallback to runtime arch": {
				shell:      shell,
				dockerArch: "",
				revision:   "2923a43",
				expectedInfo: Info{
					Architecture:            getExpectedArch(),
					Name:                    GitLabRegistryName,
					Tag:                     fmt.Sprintf("%s-2923a43%s", getExpectedArch(), expectedTagSuffix),
					IsSupportingLocalImport: true,
					Cmd:                     expectedCmd,
				},
			},
			"Docker runs on armv6l": {
				shell:      shell,
				dockerArch: "armv6l",
				revision:   "2923a43",
				expectedInfo: Info{
					Architecture:            "arm",
					Name:                    GitLabRegistryName,
					Tag:                     "arm-2923a43" + expectedTagSuffix,
					IsSupportingLocalImport: true,
					Cmd:                     expectedCmd,
				},
			},
			"Docker runs on amd64": {
				shell:      shell,
				dockerArch: "amd64",
				revision:   "2923a43",
				expectedInfo: Info{
					Architecture:            "x86_64",
					Name:                    GitLabRegistryName,
					Tag:                     "x86_64-2923a43" + expectedTagSuffix,
					IsSupportingLocalImport: true,
					Cmd:                     expectedCmd,
				},
			},
			"Docker runs on arm64": {
				shell:      shell,
				dockerArch: "aarch64",
				revision:   "2923a43",
				expectedInfo: Info{
					Architecture:            "arm64",
					Name:                    GitLabRegistryName,
					Tag:                     "arm64-2923a43" + expectedTagSuffix,
					IsSupportingLocalImport: true,
					Cmd:                     expectedCmd,
				},
			},
			"Docker runs on s390x": {
				shell:      shell,
				dockerArch: "s390x",
				revision:   "2923a43",
				expectedInfo: Info{
					Architecture:            "s390x",
					Name:                    GitLabRegistryName,
					Tag:                     "s390x-2923a43" + expectedTagSuffix,
					IsSupportingLocalImport: true,
					Cmd:                     expectedCmd,
				},
			},
			"Docker runs on ppc64le": {
				shell:      shell,
				dockerArch: "ppc64le",
				revision:   "2923a43",
				expectedInfo: Info{
					Architecture:            "ppc64le",
					Name:                    GitLabRegistryName,
					Tag:                     "ppc64le-2923a43" + expectedTagSuffix,
					IsSupportingLocalImport: true,
					Cmd:                     expectedCmd,
				},
			},
			"Configured architecture is unknown": {
				shell:      shell,
				dockerArch: "some-random-arch",
				revision:   "2923a43",
				expectedInfo: Info{
					Architecture:            "some-random-arch",
					Name:                    GitLabRegistryName,
					Tag:                     "some-random-arch-2923a43" + expectedTagSuffix,
					IsSupportingLocalImport: true,
					Cmd:                     expectedCmd,
				},
			},
			"Flavor configured default registry": {
				dockerArch: "amd64",
				revision:   "2923a43",
				flavor:     "ubuntu",
				expectedInfo: Info{
					Architecture:            "x86_64",
					Name:                    GitLabRegistryName,
					Tag:                     "ubuntu-x86_64-2923a43" + expectedTagSuffix,
					IsSupportingLocalImport: true,
					Cmd:                     expectedCmd,
				},
			},
		}

		t.Run(shell, func(t *testing.T) {
			for name, test := range tests {
				t.Run(name, func(t *testing.T) {
					l := new(linuxInfo)

					image, err := l.Create(
						test.revision,
						Config{
							Architecture: test.dockerArch,
							Shell:        shell,
							Flavor:       test.flavor,
						},
					)

					assert.NoError(t, err)
					assert.Equal(t, test.expectedInfo, image)
				})
			}
		})
	}
}

// We re write amd64 to x86_64 for the helper image, and we don't want this test
// to be runtime dependant.
func getExpectedArch() string {
	if runtime.GOARCH == "amd64" {
		return "x86_64"
	}

	return runtime.GOARCH
}
