//go:build !integration

package commands

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

func TestRegisterDefaultWindowsDockerCacheVolume(t *testing.T) {
	testCases := map[string]struct {
		userDefinedVolumes []string
		expectedVolumes    []string
	}{
		"user did not define anything": {
			userDefinedVolumes: []string{},
			expectedVolumes:    []string{defaultDockerWindowCacheDir},
		},
		"user defined an extra volume": {
			userDefinedVolumes: []string{"c:\\Users\\SomeUser\\config.json:c:\\config.json"},
			expectedVolumes:    []string{defaultDockerWindowCacheDir, "c:\\Users\\SomeUser\\config.json:c:\\config.json"},
		},
		"user defined volume binding to default cache dir": {
			userDefinedVolumes: []string{fmt.Sprintf("c:\\Users\\SomeUser\\cache:%s", defaultDockerWindowCacheDir)},
			expectedVolumes:    []string{fmt.Sprintf("c:\\Users\\SomeUser\\cache:%s", defaultDockerWindowCacheDir)},
		},
		"user defined cache as source leads to incorrect parsing of volume and never adds cache volume": {
			userDefinedVolumes: []string{"c:\\cache:c:\\User\\ContainerAdministrator\\cache"},
			expectedVolumes:    []string{"c:\\cache:c:\\User\\ContainerAdministrator\\cache"},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			s := setupDockerRegisterCommand(&common.DockerConfig{
				Volumes: testCase.userDefinedVolumes,
			})

			s.askDockerWindows()
			assert.ElementsMatch(t, testCase.expectedVolumes, s.Docker.Volumes)
		})
	}
}

func TestDefaultWindowsShell(t *testing.T) {
	tests := []struct {
		shell         string
		expectedShell string
	}{
		{
			shell:         "cmd",
			expectedShell: "cmd",
		},
		{
			shell:         "powershell",
			expectedShell: shells.SNPowershell,
		},
		{
			shell:         "pwsh",
			expectedShell: shells.SNPwsh,
		},
		{
			shell:         "",
			expectedShell: shells.SNPwsh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			cmd := newRegisterCommand()
			cmd.Shell = tt.shell
			cmd.Executor = "shell"

			cmd.askExecutorOptions()

			assert.Equal(t, tt.expectedShell, cmd.Shell)
		})
	}
}
