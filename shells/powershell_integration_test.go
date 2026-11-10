//go:build integration

package shells

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
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

// writeCodeUnitsFunction defines a PowerShell function printing the UTF-16
// code units of its first argument, so that what the parser read can be
// compared without depending on the code page the shell writes its output in.
const writeCodeUnitsFunction = `function Write-CodeUnits { ($args[0].ToCharArray() | ForEach-Object { [int]$_ }) -join ' ' }`

// codeUnits is what Write-CodeUnits prints for text.
func codeUnits(text string) string {
	var units []string
	for _, unit := range utf16.Encode([]rune(text)) {
		units = append(units, strconv.Itoa(int(unit)))
	}
	return strings.Join(units, " ")
}

// onEachPowershell runs f with a writer for each PowerShell the host has.
// Windows PowerShell reads a script file without a BOM in the system code
// page, so its script is written with one, as the runner does when it passes
// a script as a file.
func onEachPowershell(t *testing.T, f func(t *testing.T, shell string, writer *PsWriter)) {
	writers := map[string]*PsWriter{
		SNPowershell: {Shell: SNPowershell, EOL: "\r\n", PassFile: true, TemporaryPath: "tmp"},
		SNPwsh:       {Shell: SNPwsh, EOL: "\n", TemporaryPath: "tmp"},
	}

	for shell, writer := range writers {
		t.Run(shell, func(t *testing.T) {
			helpers.SkipIntegrationTests(t, shell)
			f(t, shell, writer)
		})
	}
}

// runScriptLines runs the writer's script as a file and returns the non-empty
// lines it printed.
func runScriptLines(t *testing.T, shell string, writer *PsWriter) []string {
	t.Helper()

	scriptFile := filepath.Join(t.TempDir(), "script.ps1")
	require.NoError(t, os.WriteFile(scriptFile, []byte(writer.Finish(false)), 0o600))

	cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptFile)
	cmd.Env = testEnv()
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "output: %s", out)

	var lines []string
	for line := range strings.Lines(string(out)) {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// TestPowershell_CommandKeepsSingleQuoteCharacters runs a generated script that
// passes arguments holding U+2018 to U+201B through Command, and checks that
// the parser reads each argument unchanged.
func TestPowershell_CommandKeepsSingleQuoteCharacters(t *testing.T) {
	arguments := []string{
		"it's",
		"it‘s", // left single quotation mark
		"it’s", // right single quotation mark
		"it‚s", // single low-9 quotation mark
		"it‛s", // single high-reversed-9 quotation mark
		"a’b‘c",
		"it’’s",
		"'‘’‚‛",
	}

	onEachPowershell(t, func(t *testing.T, shell string, writer *PsWriter) {
		writer.Line(writeCodeUnitsFunction)
		var want []string
		for _, argument := range arguments {
			writer.Command("Write-CodeUnits", argument)
			want = append(want, codeUnits(argument))
		}

		assert.Equal(t, want, runScriptLines(t, shell, writer))
	})
}

// TestPowershell_VariableKeepsQuoteCharacters runs a generated script that
// sets variables holding every character PowerShell reads as a quote, and
// checks that the variable and its environment copy hold each value unchanged.
func TestPowershell_VariableKeepsQuoteCharacters(t *testing.T) {
	values := []string{
		`it's`,
		`say "hi"`,
		"a“b”c„d",   // double quotation marks
		"a‘b’c‚d‛e", // single quotation marks
		"it’’s",
		`'"‘’‚‛“”„`,
	}

	onEachPowershell(t, func(t *testing.T, shell string, writer *PsWriter) {
		writer.Line(writeCodeUnitsFunction)
		var want []string
		for i, value := range values {
			key := fmt.Sprintf("QUOTES_%d", i)
			writer.Variable(spec.Variable{Key: key, Value: value})
			writer.Linef("Write-CodeUnits ${%s}", key)
			writer.Linef("Write-CodeUnits ${env:%s}", key)
			want = append(want, codeUnits(value), codeUnits(value))
		}

		assert.Equal(t, want, runScriptLines(t, shell, writer))
	})
}
