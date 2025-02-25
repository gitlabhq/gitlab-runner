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
			expectedStdout string
		}{
			"no git arg": {
				gitCallArg:     "",
				expectedStdout: "",
			},
			"happy path": {
				jobToken:       "blipp blupp",
				gitCallArg:     "get",
				expectedStdout: "password=blipp blupp\n",
			},
			"env var not set": {
				gitCallArg:     "get",
				expectedStdout: "password=\n",
			},
			"everything else is a no-op": {
				gitCallArg: "foobar",
			},
		}

		for tn, tc := range tests {
			t.Run(tn, func(t *testing.T) {
				credHelperCmd := shell.GetGitCredHelperCommand()
				callArgs := prepCallArgs(t, shellName, credHelperCmd, tc.gitCallArg)
				stdout := &bytes.Buffer{}
				stderr := &bytes.Buffer{}

				env := os.Environ()
				if jt := tc.jobToken; jt != "" {
					env = append(env, "CI_JOB_TOKEN="+jt)
				}

				cmd := exec.Command(shellName, callArgs...)
				cmd.Env = env
				cmd.Stderr = stderr
				cmd.Stdout = stdout

				err := cmd.Run()
				require.NoError(t, err, "running command failed\n  stdout: %s\n  stderr: %s", stdout, stderr)

				assert.Equal(t, tc.expectedStdout, stdout.String())
				assert.Empty(t, stderr.String(), "expected no errors on stderr")
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
