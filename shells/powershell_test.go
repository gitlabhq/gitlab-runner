package shells

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestPowershell_LineBreaks(t *testing.T) {
	testCases := map[string]struct {
		shell                   string
		eol                     string
		expectedEdition         string
		expectedErrorPreference string
	}{
		"Windows newline on Desktop": {
			shell:                   SNPowershell,
			eol:                     "\r\n",
			expectedEdition:         "Desktop",
			expectedErrorPreference: "",
		},
		"Windows newline on Core": {
			shell:                   SNPwsh,
			eol:                     "\r\n",
			expectedEdition:         "Core",
			expectedErrorPreference: `$ErrorActionPreference = "Stop"` + "\r\n\r\n",
		},
		"Linux newline on Core": {
			shell:                   SNPwsh,
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

			expectedOutput :=
				tc.expectedErrorPreference +
					`& "foo" ""` + eol + "if(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }" + eol +
					eol +
					eol
			if tc.shell != SNPwsh {
				expectedOutput = "\xef\xbb\xbf" + expectedOutput
			}
			assert.Equal(t, expectedOutput, writer.Finish(false))
		})
	}
}

func TestPowershell_CommandShellEscapes(t *testing.T) {
	writer := &PsWriter{Shell: SNPowershell, EOL: "\r\n"}
	writer.Command("foo", "x&(y)")

	assert.Equal(
		t,
		"& \"foo\" \"x&(y)\"\r\nif(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }\r\n\r\n",
		writer.String(),
	)
}

func TestPowershell_IfCmdShellEscapes(t *testing.T) {
	writer := &PsWriter{Shell: SNPowershell, EOL: "\r\n"}
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

func TestPowershell_GetName(t *testing.T) {
	for _, shellName := range []string{SNPwsh, SNPowershell} {
		shell := common.GetShell(shellName)
		assert.Equal(t, shellName, shell.GetName())
	}
}

func TestPowershell_IsDefault(t *testing.T) {
	for _, shellName := range []string{SNPwsh, SNPowershell} {
		shell := common.GetShell(shellName)
		assert.False(t, shell.IsDefault())
	}
}

func TestPowershell_GetConfiguration(t *testing.T) {
	testCases := map[string]struct {
		shell    string
		executor string

		expectedPassFile bool
	}{
		"powershell on docker-windows": {
			shell:            SNPowershell,
			executor:         dockerWindowsExecutor,
			expectedPassFile: false,
		},
		"pwsh on docker-windows": {
			shell:            SNPwsh,
			executor:         dockerWindowsExecutor,
			expectedPassFile: false,
		},
		"pwsh on docker": {
			shell:            SNPwsh,
			executor:         "docker",
			expectedPassFile: false,
		},
		"pwsh on kubernetes": {
			shell:            SNPwsh,
			executor:         "kubernetes",
			expectedPassFile: false,
		},
		"pwsh on shell": {
			shell:            SNPwsh,
			executor:         "shell",
			expectedPassFile: false,
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			shell := common.GetShell(tc.shell)
			info := common.ShellScriptInfo{
				Shell: tc.shell,
				Build: &common.Build{
					Runner: &common.RunnerConfig{},
				},
			}
			info.Build.Runner.Executor = tc.executor

			shellConfig, err := shell.GetConfiguration(info)
			require.NoError(t, err)
			assert.Equal(t, tc.shell, shellConfig.Command)
			if tc.expectedPassFile {
				assert.Equal(t, fileCmdArgs(), shellConfig.Arguments)
			} else {
				assert.Equal(t, stdinCmdArgs(), shellConfig.Arguments)
			}
			assert.Equal(t, PowershellDockerCmd(tc.shell), shellConfig.DockerCommand)
			assert.Equal(t, tc.expectedPassFile, shellConfig.PassFile)
			assert.Equal(t, "ps1", shellConfig.Extension)
		})
	}
}

func TestPowershellCmdArgs(t *testing.T) {
	for _, tc := range []string{SNPwsh, SNPowershell} {
		t.Run(tc, func(t *testing.T) {
			args := PowershellDockerCmd(tc)
			assert.Equal(t, append([]string{tc}, stdinCmdArgs()...), args)
		})
	}
}
