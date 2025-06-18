//go:build integration

package shells

import (
	"bytes"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

func TestPowershellUTF8EncodingStdin(t *testing.T) {
	for _, shell := range []string{SNPowershell, SNPwsh} {
		t.Run(shell, func(t *testing.T) {
			helpers.SkipIntegrationTests(t, shell)

			cmd := exec.Command(shell, stdinCmdArgs(shell)...)

			buf := new(bytes.Buffer)
			// script to detect regression based on https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29160
			cmd.Stdin = strings.NewReader(`& { $Q_Test_ = '∅'; Write-Host "Actual: $($Q_Test_) $(($Q_Test_ | Format-Hex -Encoding UTF8).Bytes -join ', ')" }`)
			cmd.Stdout = buf
			cmd.Stderr = buf
			// When running this test from pwsh (caller) and the shell for the test is PowerShell (test), the PSModulePath environment variable from the caller
			// shell taints the PSModulePath value in the test shell, causing a CommandNotFoundException to be thrown. Removing PSModulePath from the environment
			// fixes this error.
			cmd.Env = testEnv()

			require.NoError(t, cmd.Run())

			require.Contains(t, buf.String(), "Actual: ∅ 226, 136, 133")
		})
	}
}

// testEnv returns the test's entire environment, except PSModulePath
func testEnv() []string {
	return slices.DeleteFunc(os.Environ(), func(e string) bool {
		return strings.HasPrefix(e, "PSModulePath=")
	})
}
