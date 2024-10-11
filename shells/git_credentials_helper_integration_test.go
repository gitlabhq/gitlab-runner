//go:build integration

package shells_test

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"
)

func TestGitCredHelper(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shellName string) {
		helpers.SkipIntegrationTests(t, shellName)

		shell := common.GetShell(shellName)
		require.NotNil(t, shell, "shell %q not available", shellName)

		os.Unsetenv("CI_JOB_TOKEN")

		tests := map[string]struct {
			jobToken       string
			gitCallArg     string
			expectedOutput string
		}{
			"no git arg": {
				gitCallArg:     "",
				expectedOutput: "",
			},
			"happy path": {
				jobToken:       "blipp blupp",
				gitCallArg:     "get",
				expectedOutput: "password=blipp blupp" + eol,
			},
			"env var not set": {
				gitCallArg:     "get",
				expectedOutput: "password=" + eol,
			},
		}

		for tn, tc := range tests {
			t.Run(tn, func(t *testing.T) {
				credHelperCmd := shell.GetGitCredHelperCommand()
				callArgs := prepCallArgs(t, shellName, credHelperCmd, tc.gitCallArg)
				stderr := &bytes.Buffer{}

				env := os.Environ()
				if jt := tc.jobToken; jt != "" {
					env = append(env, "CI_JOB_TOKEN="+jt)
				}

				cmd := exec.Command(shellName, callArgs...)
				cmd.Env = env
				cmd.Stderr = stderr

				output, err := cmd.Output()
				require.NoError(t, err, "running command failed; stderr: %s", stderr)

				assert.Equal(t, tc.expectedOutput, string(output))
			})
		}
	})
}

func prepCallArgs(t *testing.T, shellName, command, arg string) []string {
	// -c works for bash, powershell & pwsh
	args := []string{"-c"}

	switch shellName {
	case shells.Bash:
		// nothing to do here
	case shells.SNPowershell, shells.SNPwsh:
		// Why the double single-quotes? Please see the comment on powershell's GetGitCredHelperCommand
		command = strings.ReplaceAll(command, "''", "'")
	default:
		t.Fatalf("not sure how to prep command line args for shell %s", shellName)
	}

	// this mimics how git adds args to its helper commands
	command += " " + arg

	return append(args, command)
}

var eol = func() string {
	if os.PathSeparator == '/' {
		return "\n"
	}
	return "\r\n"
}()
