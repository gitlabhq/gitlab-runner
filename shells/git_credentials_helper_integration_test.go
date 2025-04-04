//go:build integration

package shells_test

import (
	"os"
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
	const defaultUser = "fallback-user"

	// `git credential fill` without protocol or host does not work but errors out,
	// so we don't even test for that.

	tests := map[string]struct {
		jobToken      string
		credRequest   string
		expectedCreds string
		expectedErr   string
	}{
		"token set, default user": {
			jobToken:    "blipp blupp",
			credRequest: "host=some-host\nprotocol=https",
			expectedCreds: "" +
				"protocol=https\n" +
				"host=some-host\n" +
				"username=" + defaultUser + "\n" +
				"password=blipp blupp\n",
		},
		"token set, explicit user": {
			jobToken:    "blipp blupp",
			credRequest: "username=some-user\nhost=some-host\nprotocol=https",
			expectedCreds: "" +
				"protocol=https\n" +
				"host=some-host\n" +
				"username=some-user\n" +
				"password=blipp blupp\n",
		},
		"token not set, default user": {
			credRequest: "host=some-host\nprotocol=https",
			expectedCreds: "" +
				"protocol=https\n" +
				"host=some-host\n" +
				"username=" + defaultUser + "\n" +
				"password=\n",
		},
		"token not set, explicit user": {
			credRequest: "username=some-user\nhost=some-host\nprotocol=https",
			expectedCreds: "" +
				"protocol=https\n" +
				"host=some-host\n" +
				"username=some-user\n" +
				"password=\n",
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

				w.Command("git", "init", "--quiet")

				w.Command("git", "config", "--local", "--add", "credential.username", defaultUser)
				w.Command("git", "config", "--local", "--replace-all", "credential.helper", shell.GetExternalCommandEmptyArgument())
				w.Command("git", "config", "--local", "--add", "credential.helper", shell.GetGitCredHelperCommand())
				w.Command("git", "config", "--local", "--list")

				w.CommandWithStdin(tc.credRequest, "git", "credential", "fill")

				env := testEnv()
				if jt := tc.jobToken; jt != "" {
					env = append(env, "CI_JOB_TOKEN="+jt)
				}

				output := runShell(t, shellName, tmpDir, w, env)

				assert.Contains(t, output, "credential.helper=\n", "resets the list of cred helpers")
				assert.Contains(t, output, tc.expectedCreds, "git credential helper returns the expected creds")
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
