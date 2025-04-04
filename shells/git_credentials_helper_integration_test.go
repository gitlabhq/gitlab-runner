//go:build integration

package shells_test

import (
	"os"
	"runtime"
	"slices"
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
			t.Parallel()

			shellstest.OnEachShellWithWriter(t, func(t *testing.T, shellName string, w shells.ShellWriter) {
				helpers.SkipIntegrationTests(t, shellName)
				t.Parallel()

				shell := common.GetShell(shellName)
				require.NotNil(t, shell, "shell %q not available", shellName)

				tmpDir := t.TempDir()

				w.Command("git", "init")

				w.Command("git", "config", "--local", "--replace-all", "credential.helper", shell.GetExternalCommandEmptyArgument(runtime.GOOS))
				w.Command("git", "config", "--local", "--add", "credential.helper", "!"+shell.GetGitCredHelperCommand(runtime.GOOS))
				w.Command("git", "config", "--local", "--list")

				w.CommandWithStdin("protocol=https\nhost=some-host\nusername=some-user", "git", "credential", "fill")

				env := testEnv()
				if jt := tc.jobToken; jt != "" {
					env = append(env, "CI_JOB_TOKEN="+jt)
				}

				output := runShell(t, shellName, tmpDir, w, env)

				assert.Contains(t, output, "credential.helper=\n", "resets the list of cred helpers")
				assert.Contains(t, output, tc.expectedStdout, "git credential helper returns the expected password")
			})
		})
	}
}

// testEnv returns the test's entire environment, except the job token
func testEnv() []string {
	return slices.DeleteFunc(os.Environ(), func(e string) bool {
		return strings.HasPrefix(e, "CI_JOB_TOKEN=")
	})
}
