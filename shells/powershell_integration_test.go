//go:build integration

package shells

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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

// TestPwsh_StreamWriterOverloadResolution validates the root cause of the em-dash
// encoding bug (https://gitlab.com/gitlab-org/gitlab-runner/-/work_items/39427).
//
// PowerShell's method binder resolves [System.IO.StreamWriter]::new(path, $encoding)
// to StreamWriter(String, Boolean) — not StreamWriter(String, Encoding) — because it
// coerces a non-null Encoding object to $True (append mode).  The result is a UTF-8
// file written WITHOUT a BOM.  On Windows, PowerShell reads BOM-less files using the
// system code page (Windows-1252), so the em-dash U+2014 (UTF-8: E2 80 94) decodes
// as â€" where 0x94 → U+201D (RIGHT DOUBLE QUOTATION MARK), breaking the parser.
//
// The fix uses the unambiguous 3-arg constructor StreamWriter(String, Boolean, Encoding)
// which always writes the UTF-8 BOM, ensuring the file is read back correctly.
func TestPwsh_StreamWriterOverloadResolution(t *testing.T) {
	helpers.SkipIntegrationTests(t, SNPwsh)

	// UTF-8 BOM bytes
	bom := []byte{0xEF, 0xBB, 0xBF}

	// Content containing an em dash (U+2014) — this is the character that triggered
	// the original bug when it appeared in CI_COMMIT_DESCRIPTION.
	content := "\u2014"

	// runScript writes scriptContent to a .ps1 file and executes it with pwsh.
	// The script is passed via a file to avoid any quoting issues with -Command.
	runScript := func(t *testing.T, scriptContent string) {
		t.Helper()
		scriptFile := filepath.Join(t.TempDir(), "test.ps1")
		require.NoError(t, os.WriteFile(scriptFile, []byte(scriptContent), 0600))
		cmd := exec.Command(SNPwsh, "-NoProfile", "-NonInteractive", "-File", scriptFile)
		cmd.Env = testEnv()
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "pwsh script failed: %s", string(out))
	}

	t.Run("2-arg StreamWriter omits BOM and is the broken form", func(t *testing.T) {
		outFile := filepath.Join(t.TempDir(), "out.txt")

		// The buggy 2-arg form: PowerShell coerces $customEncoding (non-null) to
		// $True and invokes StreamWriter(String, Boolean=append), writing UTF-8
		// without a BOM.
		runScript(t, `$customEncoding = New-Object System.Text.UTF8Encoding $True
$sw = [System.IO.StreamWriter]::new("`+outFile+`", $customEncoding)
$sw.Write("`+content+`")
$sw.Close()
`)

		data, err := os.ReadFile(outFile)
		require.NoError(t, err)

		// The 2-arg form resolves to StreamWriter(String, Boolean), so no BOM is written.
		assert.False(t, bytes.HasPrefix(data, bom),
			"expected no BOM with 2-arg StreamWriter (broken form), but BOM was present")
	})

	t.Run("3-arg StreamWriter writes BOM and is the fixed form", func(t *testing.T) {
		outFile := filepath.Join(t.TempDir(), "out.txt")

		// The fixed 3-arg form: Boolean and Encoding arguments are unambiguous,
		// StreamWriter(String, Boolean=false, Encoding) is selected, and the
		// UTF8Encoding($True) writes a BOM.
		runScript(t, `$customEncoding = New-Object System.Text.UTF8Encoding $True
$sw = [System.IO.StreamWriter]::new("`+outFile+`", $False, $customEncoding)
$sw.Write("`+content+`")
$sw.Close()
`)

		data, err := os.ReadFile(outFile)
		require.NoError(t, err)

		// The 3-arg form writes a BOM, so PowerShell (and Windows) will detect UTF-8.
		require.True(t, bytes.HasPrefix(data, bom),
			"expected UTF-8 BOM with 3-arg StreamWriter (fixed form), but no BOM was present")

		// The content after the BOM must be the valid UTF-8 encoding of the em dash.
		payload := data[len(bom):]
		assert.Equal(t, []byte(content), payload,
			"content after BOM should be the em-dash encoded in UTF-8")
	})
}
