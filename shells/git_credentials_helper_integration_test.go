//go:build integration

package shells_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	const credReqFile = "credReq.tmp"

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			shellstest.OnEachShellWithWriter(t, func(t *testing.T, shellName string, w shells.ShellWriter) {
				helpers.SkipIntegrationTests(t, shellName)
				t.Parallel()

				tmpDir := t.TempDir()

				env := testEnv()
				if jt := tc.jobToken; jt != "" {
					env = append(env, "CI_JOB_TOKEN="+jt)
				}

				// dump the credential request into a file for later use
				err := os.WriteFile(filepath.Join(tmpDir, credReqFile), []byte(tc.credRequest), 0644)
				require.NoError(t, err, "write cred request file")

				w.Command("git", "init", "--quiet")
				conf := filepath.Join(tmpDir, ".git", "config")

				// set up the cred helper
				w.SetupGitCredHelper(conf, "credential", defaultUser)
				// dump the whole local config
				w.Command("git", "config", "--local", "--list")
				// run cred fill in a shell agnostic way:
				//	- run it through a git alias, thus using git's POSIX shell
				//	- consume the cred request from a file in the current working directory
				// so that we don't have to care about encoding, BOM, ...
				w.Command("git", "-c", `alias.fillCreds=!f(){ git credential fill < `+credReqFile+` ; }; f`, "fillCreds")

				output := runShell(t, shellName, tmpDir, w, env)

				b, err := os.ReadFile(conf)
				require.NoError(t, err, "reading generated git config")
				t.Logf("git config:\n----\n%s\n----\n", b)

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
