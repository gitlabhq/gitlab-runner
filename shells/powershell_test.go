//go:build !integration
// +build !integration

package shells

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestPowershell_LineBreaks(t *testing.T) {
	testCases := map[string]struct {
		shell                   string
		eol                     string
		expectedErrorPreference string
	}{
		"Windows newline on Desktop": {
			shell:                   SNPowershell,
			eol:                     "\r\n",
			expectedErrorPreference: "",
		},
		"Windows newline on Core": {
			shell:                   SNPwsh,
			eol:                     "\r\n",
			expectedErrorPreference: `$ErrorActionPreference = "Stop"` + "\r\n",
		},
		"Linux newline on Core": {
			shell:                   SNPwsh,
			eol:                     "\n",
			expectedErrorPreference: `#!/usr/bin/env pwsh` + "\n" + `$ErrorActionPreference = "Stop"` + "\n",
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
			expectedPassFile: true,
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

//nolint:lll
func TestPowershellPathResolveOperations(t *testing.T) {
	testCases := map[string]struct {
		op       func(path string, w *PsWriter)
		expected map[string]string
	}{
		"cd": {
			op: func(path string, w *PsWriter) {
				w.Cd(path)
			},
			expected: map[string]string{
				`path/name`: "cd $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name\")\nif(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }\n\n",
				`\\unc\`:    "cd $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\\")\nif(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }\n\n",
				`C:\path\`:  "cd $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\\")\nif(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }\n\n",
			},
		},
		"mkdir": {
			op: func(path string, w *PsWriter) {
				w.MkDir(path)
			},
			expected: map[string]string{
				`path/name`: "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name\") | out-null\n",
				`\\unc\`:    "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\\") | out-null\n",
				`C:\path\`:  "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\\") | out-null\n",
			},
		},
		"mktmpdir": {
			op: func(path string, w *PsWriter) {
				w.TemporaryPath = path
				w.MkTmpDir("dir")
			},
			expected: map[string]string{
				`path/name`: "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name/dir\") | out-null\n",
				`\\unc\`:    "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\/dir\") | out-null\n",
				`C:\path\`:  "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\/dir\") | out-null\n",
			},
		},
		"rm": {
			op: func(path string, w *PsWriter) {
				w.RmFile(path)
			},
			expected: map[string]string{
				`path/name`:    "if( (Get-Command -Name Remove-Item2 -Module NTFSSecurity -ErrorAction SilentlyContinue) -and (Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name\") -PathType Leaf) ) {\n  Remove-Item2 -Force $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name\")\n} elseif(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name\")) {\n  Remove-Item -Force $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name\")\n}\n\n",
				`\\unc\file`:   "if( (Get-Command -Name Remove-Item2 -Module NTFSSecurity -ErrorAction SilentlyContinue) -and (Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\file\") -PathType Leaf) ) {\n  Remove-Item2 -Force $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\file\")\n} elseif(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\file\")) {\n  Remove-Item -Force $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\file\")\n}\n\n",
				`C:\path\file`: "if( (Get-Command -Name Remove-Item2 -Module NTFSSecurity -ErrorAction SilentlyContinue) -and (Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\file\") -PathType Leaf) ) {\n  Remove-Item2 -Force $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\file\")\n} elseif(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\file\")) {\n  Remove-Item -Force $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\file\")\n}\n\n",
			},
		},
		"rmfilesrecursive": {
			op: func(path string, w *PsWriter) {
				w.RmFilesRecursive(path, "test")
			},
			expected: map[string]string{
				`path/name`:    "if(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name\") -PathType Container) {\n  Get-ChildItem -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name\") -Filter \"test\" -Recurse | ForEach-Object { Remove-Item -Force $_.FullName }\n}\n",
				`\\unc\file`:   "if(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\file\") -PathType Container) {\n  Get-ChildItem -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\file\") -Filter \"test\" -Recurse | ForEach-Object { Remove-Item -Force $_.FullName }\n}\n",
				`C:\path\file`: "if(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\file\") -PathType Container) {\n  Get-ChildItem -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\file\") -Filter \"test\" -Recurse | ForEach-Object { Remove-Item -Force $_.FullName }\n}\n",
			},
		},
		"rmdir": {
			op: func(path string, w *PsWriter) {
				w.RmDir(path)
			},
			expected: map[string]string{
				`path/name`:    "if( (Get-Command -Name Remove-Item2 -Module NTFSSecurity -ErrorAction SilentlyContinue) -and (Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name\") -PathType Container) ) {\n  Remove-Item2 -Force -Recurse $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name\")\n} elseif(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name\")) {\n  Remove-Item -Force -Recurse $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name\")\n}\n\n",
				`\\unc\file`:   "if( (Get-Command -Name Remove-Item2 -Module NTFSSecurity -ErrorAction SilentlyContinue) -and (Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\file\") -PathType Container) ) {\n  Remove-Item2 -Force -Recurse $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\file\")\n} elseif(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\file\")) {\n  Remove-Item -Force -Recurse $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\file\")\n}\n\n",
				`C:\path\file`: "if( (Get-Command -Name Remove-Item2 -Module NTFSSecurity -ErrorAction SilentlyContinue) -and (Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\file\") -PathType Container) ) {\n  Remove-Item2 -Force -Recurse $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\file\")\n} elseif(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\file\")) {\n  Remove-Item -Force -Recurse $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\file\")\n}\n\n",
			},
		},
		"ifdirectory": {
			op: func(path string, w *PsWriter) {
				w.IfDirectory(path)
			},
			expected: map[string]string{
				`path/name`:    "if(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name\") -PathType Container) {\n",
				`\\unc\file`:   "if(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\file\") -PathType Container) {\n",
				`C:\path\file`: "if(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\file\") -PathType Container) {\n",
			},
		},
		"iffile": {
			op: func(path string, w *PsWriter) {
				w.IfFile(path)
			},
			expected: map[string]string{
				`path/name`:    "if(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name\") -PathType Leaf) {\n",
				`\\unc\file`:   "if(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\file\") -PathType Leaf) {\n",
				`C:\path\file`: "if(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\file\") -PathType Leaf) {\n",
			},
		},
		"file variable": {
			op: func(path string, w *PsWriter) {
				w.TemporaryPath = path
				w.Variable(common.JobVariable{File: true, Key: "a key", Value: "foobar"})
			},
			expected: map[string]string{
				`path/name`:    "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name\") | out-null\n[System.IO.File]::WriteAllText($ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name/a key\"), \"foobar\")\n$a key=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"path/name/a key\")\n$env:a key=$a key\n",
				`\\unc\file`:   "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\file\") | out-null\n[System.IO.File]::WriteAllText($ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\file/a key\"), \"foobar\")\n$a key=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"\\\\unc\\file/a key\")\n$env:a key=$a key\n",
				`C:\path\file`: "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\file\") | out-null\n[System.IO.File]::WriteAllText($ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\file/a key\"), \"foobar\")\n$a key=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:\\path\\file/a key\")\n$env:a key=$a key\n",
			},
		},
	}

	for tn, tc := range testCases {
		for path, expected := range tc.expected {
			for _, shell := range []string{SNPowershell, SNPwsh} {
				t.Run(fmt.Sprintf("%s:%s: %s", shell, tn, path), func(t *testing.T) {
					writer := &PsWriter{TemporaryPath: "\\temp", Shell: shell, EOL: "\n", resolvePaths: true}
					tc.op(path, writer)
					assert.Equal(t, expected, writer.String())
				})
			}
		}
	}
}

func TestPowershell_GenerateScript(t *testing.T) {
	shellInfo := common.ShellScriptInfo{
		Shell:         "pwsh",
		Type:          common.NormalShell,
		RunnerCommand: "/usr/bin/gitlab-runner-helper",
		Build: &common.Build{
			Runner: &common.RunnerConfig{},
		},
	}
	shellInfo.Build.Runner.Executor = "kubernetes"
	shellInfo.Build.Hostname = "Test Hostname"

	pwshShell := common.GetShell("pwsh").(*PowerShell)
	shebang := ""
	if pwshShell.EOL == "\n" {
		shebang = "#!/usr/bin/env pwsh\n"
	}

	testCases := map[string]struct {
		stage           common.BuildStage
		info            common.ShellScriptInfo
		expectedFailure bool
		expectedScript  string
	}{
		"prepare script": {
			stage:           common.BuildStagePrepare,
			info:            shellInfo,
			expectedFailure: false,
			expectedScript: shebang + `$ErrorActionPreference = "Stop"` + pwshShell.EOL +
				`echo "Running on $([Environment]::MachineName) via "Test Hostname"..."` +
				pwshShell.EOL + pwshShell.EOL,
		},
		"cleanup variables": {
			stage:           common.BuildStageCleanup,
			info:            shellInfo,
			expectedFailure: false,
			expectedScript:  ``,
		},
		"no script": {
			stage:           "no_script",
			info:            shellInfo,
			expectedFailure: true,
			expectedScript:  "",
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			script, err := pwshShell.GenerateScript(tc.stage, tc.info)
			assert.Equal(t, tc.expectedScript, script)
			if tc.expectedFailure {
				assert.Error(t, err)
			}
		})
	}
}
