package shells

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPowershell_LineBreaks(t *testing.T) {
	testCases := map[string]struct {
		shell                   string
		eol                     string
		expectedEdition         string
		expectedErrorPreference string
	}{
		"Windows newline on Desktop": {
			shell:                   "powershell",
			eol:                     "\r\n",
			expectedEdition:         "Desktop",
			expectedErrorPreference: "",
		},
		"Windows newline on Core": {
			shell:                   "pwsh",
			eol:                     "\r\n",
			expectedEdition:         "Core",
			expectedErrorPreference: `$ErrorActionPreference = "Stop"` + "\r\n\r\n",
		},
		"Linux newline on Core": {
			shell:                   "pwsh",
			eol:                     "\n",
			expectedEdition:         "Core",
			expectedErrorPreference: `$ErrorActionPreference = "Stop"` + "\n\n",
		},
	}
	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			eol := tc.eol
			writer := &PsWriter{Shell: tc.shell, EOL: eol}
			writer.Command("foo", "")

			expectedOutput := "\xef\xbb\xbf" +
				tc.expectedErrorPreference +
				`& "foo" ""` + eol + "if(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }" + eol +
				eol +
				eol
			assert.Equal(t, expectedOutput, writer.Finish(false))
		})
	}
}

func TestPowershell_CommandShellEscapes(t *testing.T) {
	writer := &PsWriter{Shell: "powershell", EOL: "\r\n"}
	writer.Command("foo", "x&(y)")

	assert.Equal(
		t,
		"& \"foo\" \"x&(y)\"\r\nif(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }\r\n\r\n",
		writer.String(),
	)
}

func TestPowershell_IfCmdShellEscapes(t *testing.T) {
	writer := &PsWriter{Shell: "powershell", EOL: "\r\n"}
	writer.IfCmd("foo", "x&(y)")

	//nolint:lll
	assert.Equal(t, "Set-Variable -Name cmdErr -Value $false\r\nTry {\r\n  & \"foo\" \"x&(y)\" 2>$null\r\n  if(!$?) { throw &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }\r\n} Catch {\r\n  Set-Variable -Name cmdErr -Value $true\r\n}\r\nif(!$cmdErr) {\r\n", writer.String())
}

func TestPowershell_MkTmpDirOnUNCShare(t *testing.T) {
	writer := &PsWriter{TemporaryPath: `\\unc-server\share`, EOL: "\n"}
	writer.MkTmpDir("tmp")

	assert.Equal(
		t,
		`New-Item -ItemType directory -Force -Path "\\unc-server\share\tmp" | out-null`+writer.EOL,
		writer.String(),
	)
}
