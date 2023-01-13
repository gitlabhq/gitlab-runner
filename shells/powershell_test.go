//go:build !integration

package shells

import (
	"fmt"
	"path"
	"runtime"
	"strings"
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
		shebang                 string
		passFile                bool
	}{
		"Windows newline on Desktop via stdin": {
			shell:                   SNPowershell,
			eol:                     "\r\n",
			expectedErrorPreference: "",
		},
		"Windows newline on Desktop via file": {
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
			shebang:                 `#!/usr/bin/env pwsh` + "\n",
			expectedErrorPreference: `$ErrorActionPreference = "Stop"` + "\n",
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			eol := tc.eol
			writer := &PsWriter{Shell: tc.shell, EOL: eol}
			writer.Command("foo", "")

			expectedOutput :=
				tc.expectedErrorPreference +
					`& "foo" ''` + eol + "if(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }" + eol +
					eol +
					eol
			if tc.shell == SNPwsh {
				expectedOutput = tc.shebang + "& {" + eol + eol + expectedOutput + "}" + eol + eol
			} else {
				expectedOutput = "& {" + eol + eol + expectedOutput + "}" + eol + eol

				if tc.passFile {
					expectedOutput = "\xef\xbb\xbf" + expectedOutput
				}
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
		"& \"foo\" 'x&(y)'\r\nif(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }\r\n\r\n",
		writer.String(),
	)
}

func TestPowershell_IfCmdShellEscapes(t *testing.T) {
	writer := &PsWriter{Shell: SNPowershell, EOL: "\r\n"}
	writer.IfCmd("foo", "x&(y)")

	//nolint:lll
	assert.Equal(t, "Set-Variable -Name cmdErr -Value $false\r\nTry {\r\n  & \"foo\" 'x&(y)' 2>$null\r\n  if(!$?) { throw &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }\r\n} Catch {\r\n  Set-Variable -Name cmdErr -Value $true\r\n}\r\nif(!$cmdErr) {\r\n", writer.String())
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

//nolint:lll
func TestPowershell_GetConfiguration(t *testing.T) {
	const (
		powershellStdinExpectedLine = "powershell -NoProfile -NoLogo -InputFormat text -OutputFormat text -NonInteractive -ExecutionPolicy Bypass -Command -"
		pwshStdinExpectedLine       = "pwsh -NoProfile -NoLogo -InputFormat text -OutputFormat text -NonInteractive -ExecutionPolicy Bypass -EncodedCommand JABPAHUAdABwAHUAdABFAG4AYwBvAGQAaQBuAGcAIAA9ACAAWwBjAG8AbgBzAG8AbABlAF0AOgA6AEkAbgBwAHUAdABFAG4AYwBvAGQAaQBuAGcAIAA9ACAAWwBjAG8AbgBzAG8AbABlAF0AOgA6AE8AdQB0AHAAdQB0AEUAbgBjAG8AZABpAG4AZwAgAD0AIABOAGUAdwAtAE8AYgBqAGUAYwB0ACAAUwB5AHMAdABlAG0ALgBUAGUAeAB0AC4AVQBUAEYAOABFAG4AYwBvAGQAaQBuAGcADQAKAHAAdwBzAGgAIAAtAEMAbwBtAG0AYQBuAGQAIAAtAA0ACgA="
	)

	testCases := map[string]struct {
		shell    string
		executor string
		user     string
		os       string
		passFile bool

		expectedError        error
		expectedPassFile     bool
		expectedCommand      string
		expectedCmdLine      string
		getExpectedArguments func(shell string) []string
	}{
		"powershell on docker-windows": {
			shell:    SNPowershell,
			executor: dockerWindowsExecutor,

			expectedPassFile:     false,
			expectedCommand:      SNPowershell,
			getExpectedArguments: stdinCmdArgs,
			expectedCmdLine:      powershellStdinExpectedLine,
		},
		"pwsh on docker-windows": {
			shell:    SNPwsh,
			executor: dockerWindowsExecutor,

			expectedPassFile:     false,
			expectedCommand:      SNPwsh,
			getExpectedArguments: stdinCmdArgs,
			expectedCmdLine:      pwshStdinExpectedLine,
		},
		"pwsh on docker": {
			shell:    SNPwsh,
			executor: "docker",

			expectedPassFile:     false,
			expectedCommand:      SNPwsh,
			getExpectedArguments: stdinCmdArgs,
			expectedCmdLine:      pwshStdinExpectedLine,
		},
		"pwsh on kubernetes": {
			shell:    SNPwsh,
			executor: "kubernetes",

			expectedPassFile:     false,
			expectedCommand:      SNPwsh,
			getExpectedArguments: stdinCmdArgs,
			expectedCmdLine:      pwshStdinExpectedLine,
		},
		"pwsh on shell": {
			shell:    SNPwsh,
			executor: "shell",

			expectedPassFile:     false,
			expectedCommand:      SNPwsh,
			getExpectedArguments: stdinCmdArgs,
			expectedCmdLine:      pwshStdinExpectedLine,
		},
		"pwsh on shell with custom user (linux)": {
			shell:    SNPwsh,
			executor: "shell",
			user:     "custom",
			os:       OSLinux,

			expectedPassFile: false,
			expectedCommand:  "su",
			expectedCmdLine:  "su -s /usr/bin/pwsh custom -c " + pwshStdinExpectedLine,
			getExpectedArguments: func(shell string) []string {
				return []string{"-s", "/usr/bin/pwsh", "custom", "-c", SNPwsh + " " + strings.Join(stdinCmdArgs(shell), " ")}
			},
		},
		"pwsh on shell with custom user (darwin)": {
			shell:    SNPwsh,
			executor: "shell",
			user:     "custom",
			os:       "darwin",

			expectedPassFile: false,
			expectedCommand:  "su",
			expectedCmdLine:  "su custom -c " + pwshStdinExpectedLine,
			getExpectedArguments: func(shell string) []string {
				return []string{"custom", "-c", SNPwsh + " " + strings.Join(stdinCmdArgs(shell), " ")}
			},
		},
		"pwsh on shell with custom user (windows)": {
			shell:    SNPwsh,
			executor: "shell",
			user:     "custom",
			os:       OSWindows,

			expectedPassFile: false,
			expectedCommand:  "su",
			expectedCmdLine:  "su custom -c " + pwshStdinExpectedLine,
			getExpectedArguments: func(shell string) []string {
				return []string{"-s", "custom", "-c", SNPwsh + " " + strings.Join(stdinCmdArgs(shell), " ")}
			},
		},
		"powershell on shell - FF_DISABLE_POWERSHELL_STDIN true": {
			shell:    SNPowershell,
			executor: "shell",
			passFile: true,

			expectedPassFile: true,
			expectedCommand:  SNPowershell,
			getExpectedArguments: func(_ string) []string {
				return fileCmdArgs()
			},
			expectedCmdLine: "powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command",
		},
		"powershell on shell - FF_DISABLE_POWERSHELL_STDIN false": {
			shell:    SNPowershell,
			executor: "shell",

			expectedPassFile: false,
			expectedCommand:  SNPowershell,
			getExpectedArguments: func(_ string) []string {
				return stdinCmdArgs(SNPowershell)
			},
			expectedCmdLine: powershellStdinExpectedLine,
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			if tc.os != "" && tc.os != runtime.GOOS {
				t.Skipf("test only runs on %s", tc.os)
			}

			shell := common.GetShell(tc.shell)
			info := common.ShellScriptInfo{
				Shell: tc.shell,
				User:  tc.user,
				Build: &common.Build{
					Runner: &common.RunnerConfig{},
				},
			}

			if tc.passFile {
				info.Build.JobResponse.Variables = append(
					info.Build.JobResponse.Variables,
					common.JobVariable{
						Key:   "FF_DISABLE_POWERSHELL_STDIN",
						Value: "true",
					},
				)
			}

			info.Build.Runner.Executor = tc.executor

			shellConfig, err := shell.GetConfiguration(info)
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.getExpectedArguments(tc.shell), shellConfig.Arguments)
			assert.Equal(t, tc.expectedCommand, shellConfig.Command)
			assert.Equal(t, PowershellDockerCmd(tc.shell), shellConfig.DockerCommand)
			assert.Equal(t, tc.expectedCmdLine, shellConfig.CmdLine)
			assert.Equal(t, tc.expectedPassFile, shellConfig.PassFile)
			assert.Equal(t, "ps1", shellConfig.Extension)
		})
	}
}

func TestPowershellCmdArgs(t *testing.T) {
	for _, tc := range []string{SNPwsh, SNPowershell} {
		t.Run(tc, func(t *testing.T) {
			args := PowershellDockerCmd(tc)
			assert.Equal(t, append([]string{tc}, stdinCmdArgs(tc)...), args)
		})
	}
}

//nolint:lll
func TestPowershellPathResolveOperations(t *testing.T) {
	var templateReplacer = func(escaped string) func(string) string {
		return func(tpl string) string {
			return fmt.Sprintf(tpl, escaped)
		}
	}

	testCases := map[string]struct {
		op       func(path string, w *PsWriter)
		template string
		expected map[string]func(string) string
	}{
		"cd": {
			op: func(path string, w *PsWriter) {
				w.Cd(path)
			},
			template: "cd $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%[1]v)\nif(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }\n\n",
			expected: map[string]func(string) string{
				`path/name`: templateReplacer(`"path/name"`),
				`\\unc\`:    templateReplacer(`"\\unc\"`),
				`C:\path\`:  templateReplacer(`"C:\path\"`),
			},
		},
		"mkdir": {
			op: func(path string, w *PsWriter) {
				w.MkDir(path)
			},
			template: "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%[1]v) | out-null\n",
			expected: map[string]func(string) string{
				`path/name`: templateReplacer(`"path/name"`),
				`\\unc\`:    templateReplacer(`"\\unc\"`),
				`C:\path\`:  templateReplacer(`"C:\path\"`),
			},
		},
		"mktmpdir": {
			op: func(path string, w *PsWriter) {
				w.TemporaryPath = path
				w.MkTmpDir("dir")
			},
			template: "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%[1]v) | out-null\n",
			expected: map[string]func(string) string{
				`path/name`: templateReplacer(`"path/name/dir"`),
				`\\unc\`:    templateReplacer(`"\\unc\/dir"`),
				`C:\path\`:  templateReplacer(`"C:\path\/dir"`),
			},
		},
		"rm": {
			op: func(path string, w *PsWriter) {
				w.RmFile(path)
			},
			template: "if( (Get-Command -Name Remove-Item2 -Module NTFSSecurity -ErrorAction SilentlyContinue) -and (Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%[1]v) -PathType Leaf) ) {\n  Remove-Item2 -Force $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%[1]v)\n} elseif(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%[1]v)) {\n  Remove-Item -Force $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%[1]v)\n}\n\n",
			expected: map[string]func(string) string{
				`path/name`:    templateReplacer(`"path/name"`),
				`\\unc\file`:   templateReplacer(`"\\unc\file"`),
				`C:\path\file`: templateReplacer(`"C:\path\file"`),
			},
		},
		"rmfilesrecursive": {
			op: func(path string, w *PsWriter) {
				w.RmFilesRecursive(path, "test")
			},
			template: "if(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%[1]v) -PathType Container) {\n  Get-ChildItem -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%[1]v) -Filter \"test\" -Recurse | ForEach-Object { Remove-Item -Force $_.FullName }\n}\n",
			expected: map[string]func(string) string{
				`path/name`:    templateReplacer(`"path/name"`),
				`\\unc\file`:   templateReplacer(`"\\unc\file"`),
				`C:\path\file`: templateReplacer(`"C:\path\file"`),
			},
		},
		"rmdir": {
			op: func(path string, w *PsWriter) {
				w.RmDir(path)
			},
			template: "if( (Get-Command -Name Remove-Item2 -Module NTFSSecurity -ErrorAction SilentlyContinue) -and (Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%[1]v) -PathType Container) ) {\n  Remove-Item2 -Force -Recurse $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%[1]v)\n} elseif(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%[1]v)) {\n  Remove-Item -Force -Recurse $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%[1]v)\n}\n\n",
			expected: map[string]func(string) string{
				`path/name`:    templateReplacer(`"path/name"`),
				`\\unc\file`:   templateReplacer(`"\\unc\file"`),
				`C:\path\file`: templateReplacer(`"C:\path\file"`),
			},
		},
		"ifdirectory": {
			op: func(path string, w *PsWriter) {
				w.IfDirectory(path)
			},
			template: "if(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%[1]v) -PathType Container) {\n",
			expected: map[string]func(string) string{
				`path/name`:    templateReplacer(`"path/name"`),
				`\\unc\file`:   templateReplacer(`"\\unc\file"`),
				`C:\path\file`: templateReplacer(`"C:\path\file"`),
			},
		},
		"iffile": {
			op: func(path string, w *PsWriter) {
				w.IfFile(path)
			},
			template: "if(Test-Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%[1]v) -PathType Leaf) {\n",
			expected: map[string]func(string) string{
				`path/name`:    templateReplacer(`"path/name"`),
				`\\unc\file`:   templateReplacer(`"\\unc\file"`),
				`C:\path\file`: templateReplacer(`"C:\path\file"`),
			},
		},
		"file variable": {
			op: func(path string, w *PsWriter) {
				w.TemporaryPath = path
				w.Variable(common.JobVariable{File: true, Key: "a key", Value: "foobar"})
			},
			template: "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"%[1]v\") | out-null\n[System.IO.File]::WriteAllText($ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"%[1]v/a key\"), \"foobar\")\n$a key=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"%[1]v/a key\")\n$env:a key=$a key\n",
			expected: map[string]func(string) string{
				`path/name`:    templateReplacer(`path/name`),
				`\\unc\file`:   templateReplacer(`\\unc\file`),
				`C:\path\file`: templateReplacer(`C:\path\file`),
			},
		},
	}

	for tn, tc := range testCases {
		for path, expected := range tc.expected {
			for _, shell := range []string{SNPowershell, SNPwsh} {
				t.Run(fmt.Sprintf("%s:%s: %s", shell, tn, path), func(t *testing.T) {
					writer := &PsWriter{TemporaryPath: "\\temp", Shell: shell, EOL: "\n", resolvePaths: true}
					tc.op(path, writer)
					assert.Equal(t, expected(tc.template), writer.String())
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
			expectedScript: shebang + "& {" +
				pwshShell.EOL + pwshShell.EOL +
				`$ErrorActionPreference = "Stop"` + pwshShell.EOL +
				`echo "Running on $([Environment]::MachineName) via "Test Hostname"..."` +
				pwshShell.EOL + pwshShell.EOL + "}" + pwshShell.EOL + pwshShell.EOL,
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

//nolint:lll
func TestPowershell_GenerateSaveScript(t *testing.T) {
	path := "path"
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

	testCases := map[string]struct {
		info            common.ShellScriptInfo
		script          string
		expectedFailure bool
		expectedScript  string
	}{
		"normal script": {
			info:   shellInfo,
			script: `&{ echo "Display special characters () {} <> [] \ | ;"}`,
			expectedScript: "& {" + pwshShell.EOL + pwshShell.EOL +
				"$in =\"JnsgZWNobyAiRGlzcGxheSBzcGVjaWFsIGNoYXJhY3RlcnMgKCkge30gPD4gW10gXCB8IDsifQ==\"" + pwshShell.EOL +
				"$customEncoding = New-Object System.Text.UTF8Encoding $True" + pwshShell.EOL +
				"$sw = [System.IO.StreamWriter]::new(\"path\", $customEncoding)" + pwshShell.EOL +
				"$sw.Write([System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($in)))" + pwshShell.EOL +
				"$sw.Flush()" + pwshShell.EOL + "$sw.Close()" + pwshShell.EOL + pwshShell.EOL + pwshShell.EOL +
				"}" + pwshShell.EOL + pwshShell.EOL,
		},
		"echo unicode script": {
			script: "echo \"`“ `“ `” `” `„ ‘ ’ ‚ ‛ ‘ ’„",
			info:   shellInfo,
			expectedScript: "& {" + pwshShell.EOL + pwshShell.EOL +
				"$in =\"ZWNobyAiYOKAnCBg4oCcIGDigJ0gYOKAnSBg4oCeIOKAmCDigJkg4oCaIOKAmyDigJgg4oCZ4oCe\"" + pwshShell.EOL +
				"$customEncoding = New-Object System.Text.UTF8Encoding $True" + pwshShell.EOL +
				"$sw = [System.IO.StreamWriter]::new(\"path\", $customEncoding)" + pwshShell.EOL +
				"$sw.Write([System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($in)))" + pwshShell.EOL +
				"$sw.Flush()" + pwshShell.EOL + "$sw.Close()" + pwshShell.EOL + pwshShell.EOL + pwshShell.EOL +
				"}" + pwshShell.EOL + pwshShell.EOL,
		},
		"echo script": {
			script: "echo normal string",
			info:   shellInfo,
			expectedScript: "& {" + pwshShell.EOL + pwshShell.EOL +
				"$in =\"ZWNobyBub3JtYWwgc3RyaW5n\"" + pwshShell.EOL +
				"$customEncoding = New-Object System.Text.UTF8Encoding $True" + pwshShell.EOL +
				"$sw = [System.IO.StreamWriter]::new(\"path\", $customEncoding)" + pwshShell.EOL +
				"$sw.Write([System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($in)))" + pwshShell.EOL +
				"$sw.Flush()" + pwshShell.EOL + "$sw.Close()" + pwshShell.EOL + pwshShell.EOL + pwshShell.EOL +
				"}" + pwshShell.EOL + pwshShell.EOL,
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			script, err := pwshShell.GenerateSaveScript(tc.info, path, tc.script)
			assert.Equal(t, tc.expectedScript, script)
			if tc.expectedFailure {
				assert.Error(t, err)
			}
		})
	}
}

func Test_PsWriter_isTmpFile(t *testing.T) {
	tmpDir := "/foo/bar"
	bw := PsWriter{TemporaryPath: tmpDir}

	tests := map[string]struct {
		path string
		want bool
	}{
		"tmp file var":     {path: path.Join(tmpDir, "BAZ"), want: true},
		"not tmp file var": {path: "bla bla bla", want: false},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, bw.isTmpFile(tt.path))
		})
	}
}

func Test_PsWriter_cleanPath(t *testing.T) {
	tests := map[string]struct {
		path, wantLinux, wantWindows string
	}{
		"relative path": {
			path:        "foo/bar/KEY",
			wantLinux:   "$CurrentDirectory\\foo\\bar\\KEY",
			wantWindows: "$CurrentDirectory\\foo\\bar\\KEY",
		},
		"absolute path": {
			path:        "/foo/bar/KEY",
			wantLinux:   "\\foo\\bar\\KEY",
			wantWindows: "$CurrentDirectory\\foo\\bar\\KEY",
		},
		"absolute path with drive": {
			path:        "C:/foo/bar/KEY",
			wantLinux:   "$CurrentDirectory\\C:\\foo\\bar\\KEY",
			wantWindows: "C:\\foo\\bar\\KEY",
		},
	}

	bw := PsWriter{TemporaryPath: "foo/bar"}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := bw.cleanPath(tt.path)

			if runtime.GOOS == OSWindows {
				assert.Equal(t, tt.wantWindows, got)
			} else {
				assert.Equal(t, tt.wantLinux, got)
			}
		})
	}
}

// nolint:lll
func Test_PsWriter_Variable(t *testing.T) {
	tests := map[string]struct {
		variable               common.JobVariable
		writer                 PsWriter
		wantLinux, wantWindows string
	}{
		"file var, relative path": {
			variable:    common.JobVariable{Key: "KEY", Value: "the secret", File: true},
			writer:      PsWriter{TemporaryPath: "foo/bar"},
			wantLinux:   "$CurrentDirectory = (Resolve-Path ./).PathNew-Item -ItemType directory -Force -Path \"foo\\bar\" | out-null[System.IO.File]::WriteAllText(\"$CurrentDirectory\\foo\\bar\\KEY\", \"the secret\")$KEY=\"$CurrentDirectory\\foo\\bar\\KEY\"$env:KEY=$KEY",
			wantWindows: "$CurrentDirectory = (Resolve-Path .\\).PathNew-Item -ItemType directory -Force -Path \"foo\\bar\" | out-null[System.IO.File]::WriteAllText(\"$CurrentDirectory\\foo\\bar\\KEY\", \"the secret\")$KEY=\"$CurrentDirectory\\foo\\bar\\KEY\"$env:KEY=$KEY",
		},
		"file var, absolute path": {
			variable:    common.JobVariable{Key: "KEY", Value: "the secret", File: true},
			writer:      PsWriter{TemporaryPath: "/foo/bar"},
			wantLinux:   "New-Item -ItemType directory -Force -Path \"\\foo\\bar\" | out-null[System.IO.File]::WriteAllText(\"\\foo\\bar\\KEY\", \"the secret\")$KEY=\"\\foo\\bar\\KEY\"$env:KEY=$KEY",
			wantWindows: "$CurrentDirectory = (Resolve-Path .\\).PathNew-Item -ItemType directory -Force -Path \"\\foo\\bar\" | out-null[System.IO.File]::WriteAllText(\"$CurrentDirectory\\foo\\bar\\KEY\", \"the secret\")$KEY=\"$CurrentDirectory\\foo\\bar\\KEY\"$env:KEY=$KEY",
		},
		"file var, absolute path with drive": {
			variable:    common.JobVariable{Key: "KEY", Value: "the secret", File: true},
			writer:      PsWriter{TemporaryPath: "C:/foo/bar"},
			wantLinux:   "$CurrentDirectory = (Resolve-Path ./).PathNew-Item -ItemType directory -Force -Path \"C:\\foo\\bar\" | out-null[System.IO.File]::WriteAllText(\"$CurrentDirectory\\C:\\foo\\bar\\KEY\", \"the secret\")$KEY=\"$CurrentDirectory\\C:\\foo\\bar\\KEY\"$env:KEY=$KEY",
			wantWindows: "New-Item -ItemType directory -Force -Path \"C:\\foo\\bar\" | out-null[System.IO.File]::WriteAllText(\"C:\\foo\\bar\\KEY\", \"the secret\")$KEY=\"C:\\foo\\bar\\KEY\"$env:KEY=$KEY",
		},
		"tmp file var, relative path": {
			variable:    common.JobVariable{Key: "KEY", Value: "foo/bar/KEY2"},
			writer:      PsWriter{TemporaryPath: "foo/bar"},
			wantLinux:   "$CurrentDirectory = (Resolve-Path ./).Path$KEY=\"`$CurrentDirectory\\foo\\bar\\KEY2\"$env:KEY=$KEY",
			wantWindows: "$CurrentDirectory = (Resolve-Path .\\).Path$KEY=\"`$CurrentDirectory\\foo\\bar\\KEY2\"$env:KEY=$KEY",
		},
		"tmp file var, absolute path": {
			variable:    common.JobVariable{Key: "KEY", Value: "/foo/bar/KEY2"},
			writer:      PsWriter{TemporaryPath: "/foo/bar"},
			wantLinux:   "$KEY=\"\\foo\\bar\\KEY2\"$env:KEY=$KEY",
			wantWindows: "$CurrentDirectory = (Resolve-Path .\\).Path$KEY=\"`$CurrentDirectory\\foo\\bar\\KEY2\"$env:KEY=$KEY",
		},
		"regular var": {
			variable:    common.JobVariable{Key: "KEY", Value: "VALUE"},
			writer:      PsWriter{TemporaryPath: "C:/foo/bar"},
			wantLinux:   "$KEY=\"VALUE\"$env:KEY=$KEY",
			wantWindows: "$KEY=\"VALUE\"$env:KEY=$KEY",
		},

		"file var, relative path, resolvePaths": {
			variable:    common.JobVariable{Key: "KEY", Value: "the secret", File: true},
			writer:      PsWriter{TemporaryPath: "foo/bar", resolvePaths: true},
			wantLinux:   "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"foo/bar\") | out-null[System.IO.File]::WriteAllText($ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"foo/bar/KEY\"), \"the secret\")$KEY=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"foo/bar/KEY\")$env:KEY=$KEY",
			wantWindows: "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"foo/bar\") | out-null[System.IO.File]::WriteAllText($ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"foo/bar/KEY\"), \"the secret\")$KEY=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"foo/bar/KEY\")$env:KEY=$KEY",
		},
		"file var, absolute path, resolvePaths": {
			variable:    common.JobVariable{Key: "KEY", Value: "the secret", File: true},
			writer:      PsWriter{TemporaryPath: "/foo/bar", resolvePaths: true},
			wantLinux:   "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"/foo/bar\") | out-null[System.IO.File]::WriteAllText($ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"/foo/bar/KEY\"), \"the secret\")$KEY=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"/foo/bar/KEY\")$env:KEY=$KEY",
			wantWindows: "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"/foo/bar\") | out-null[System.IO.File]::WriteAllText($ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"/foo/bar/KEY\"), \"the secret\")$KEY=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"/foo/bar/KEY\")$env:KEY=$KEY",
		},
		"file var, absolute path with drive, resolvePaths": {
			variable:    common.JobVariable{Key: "KEY", Value: "the secret", File: true},
			writer:      PsWriter{TemporaryPath: "C:/foo/bar", resolvePaths: true},
			wantLinux:   "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:/foo/bar\") | out-null[System.IO.File]::WriteAllText($ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:/foo/bar/KEY\"), \"the secret\")$KEY=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:/foo/bar/KEY\")$env:KEY=$KEY",
			wantWindows: "New-Item -ItemType directory -Force -Path $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:/foo/bar\") | out-null[System.IO.File]::WriteAllText($ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:/foo/bar/KEY\"), \"the secret\")$KEY=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(\"C:/foo/bar/KEY\")$env:KEY=$KEY",
		},
		"tmp file var, relative path, resolvePaths": {
			variable:    common.JobVariable{Key: "KEY", Value: "foo/bar/KEY2"},
			writer:      PsWriter{TemporaryPath: "foo/bar", resolvePaths: true},
			wantLinux:   "$KEY=\"foo/bar/KEY2\"$env:KEY=$KEY",
			wantWindows: "$KEY=\"foo/bar/KEY2\"$env:KEY=$KEY",
		},
		"tmp file var, absolute path, resolvePaths": {
			variable:    common.JobVariable{Key: "KEY", Value: "/foo/bar/KEY2"},
			writer:      PsWriter{TemporaryPath: "/foo/bar", resolvePaths: true},
			wantLinux:   "$KEY=\"/foo/bar/KEY2\"$env:KEY=$KEY",
			wantWindows: "$KEY=\"/foo/bar/KEY2\"$env:KEY=$KEY",
		},
		"tmp file var, absolute path with drive, resolvePaths": {
			variable:    common.JobVariable{Key: "KEY", Value: "C:/foo/bar/KEY2"},
			writer:      PsWriter{TemporaryPath: "C:/foo/bar", resolvePaths: true},
			wantLinux:   "$KEY=\"C:/foo/bar/KEY2\"$env:KEY=$KEY",
			wantWindows: "$KEY=\"C:/foo/bar/KEY2\"$env:KEY=$KEY",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tt.writer.Variable(tt.variable)

			if runtime.GOOS == OSWindows {
				assert.Equal(t, tt.wantWindows, tt.writer.String())
			} else {
				assert.Equal(t, tt.wantLinux, tt.writer.String())
			}
		})
	}
}
