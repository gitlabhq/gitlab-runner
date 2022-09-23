//go:build integration

package commands_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli"
	"gitlab.com/gitlab-org/gitlab-runner/commands"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

func newTestGetServiceArgumentsCommand(t *testing.T, expectedArgs []string) func(*cli.Context) {
	return func(c *cli.Context) {
		arguments := commands.GetServiceArguments(c)

		for _, arg := range expectedArgs {
			assert.Contains(t, arguments, arg)
		}
	}
}

func testServiceCommandRun(command func(*cli.Context), args ...string) {
	app := cli.NewApp()
	app.Commands = []cli.Command{
		{
			Name:   "test-command",
			Action: command,
			Flags:  commands.GetInstallFlags(),
		},
	}

	args = append([]string{"binary", "test-command"}, args...)
	_ = app.Run(args)
}

type getServiceArgumentsTestCase struct {
	cliFlags     []string
	expectedArgs []string
}

func TestGetServiceArguments(t *testing.T) {
	tests := []getServiceArgumentsTestCase{
		{
			expectedArgs: []string{
				"--working-directory", helpers.GetCurrentWorkingDirectory(),
				"--config", commands.GetDefaultConfigFile(),
				"--service", "gitlab-runner",
				"--syslog",
			},
		},
		{
			cliFlags: []string{
				"--config", "/tmp/config.toml",
			},
			expectedArgs: []string{
				"--working-directory", helpers.GetCurrentWorkingDirectory(),
				"--config", "/tmp/config.toml",
				"--service", "gitlab-runner",
				"--syslog",
			},
		},
		{
			cliFlags: []string{
				"--working-directory", "/tmp",
			},
			expectedArgs: []string{
				"--working-directory", "/tmp",
				"--config", commands.GetDefaultConfigFile(),
				"--service", "gitlab-runner",
				"--syslog",
			},
		},
		{
			cliFlags: []string{
				"--service", "gitlab-runner-service-name",
			},
			expectedArgs: []string{
				"--working-directory", helpers.GetCurrentWorkingDirectory(),
				"--config", commands.GetDefaultConfigFile(),
				"--service", "gitlab-runner-service-name",
				"--syslog",
			},
		},
		{
			cliFlags: []string{
				"--syslog=true",
			},
			expectedArgs: []string{
				"--working-directory", helpers.GetCurrentWorkingDirectory(),
				"--config", commands.GetDefaultConfigFile(),
				"--service", "gitlab-runner",
				"--syslog",
			},
		},
		{
			cliFlags: []string{
				"--syslog=false",
			},
			expectedArgs: []string{
				"--working-directory", helpers.GetCurrentWorkingDirectory(),
				"--config", commands.GetDefaultConfigFile(),
				"--service", "gitlab-runner",
			},
		},
	}

	for id, testCase := range tests {
		t.Run(fmt.Sprintf("case-%d", id), func(t *testing.T) {
			testServiceCommandRun(newTestGetServiceArgumentsCommand(t, testCase.expectedArgs), testCase.cliFlags...)
		})
	}
}
