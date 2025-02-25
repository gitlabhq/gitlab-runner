package shells

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"golang.org/x/text/encoding/unicode"
)

const (
	dockerWindowsExecutor = "docker-windows"

	SNPwsh       = "pwsh"
	SNPowershell = "powershell"

	// When the shell is set to 'powershell', the UTF8 BOM character is prepended to the initialization script, which causes unmarshalling to fail.
	// To prevent this, we add the 'echo ""' command.
	// We also introduce the variable '$script_path' to extract the script name without extension from '$PSCommandPath'.
	pwshJSONInitializationScript = `$script_path= %s -command "(Get-Item $PSCommandPath).BaseName"
$start_json= '{"script": "' + $script_path + '"}'
echo ""
echo "$start_json"
`

	// Before executing a script, powershell parses it.
	// A `ParserError` can then be thrown if a parsing error is found.
	// Those errors are not catched by the powershell_trap_script thus causing the job to hang
	// To avoid this problem, the PwshValidationScript is used to validate the given script and eventually to cause
	// the job to fail if a `ParserError` is thrown
	// As $Path already refers to the script being executed, the script name will be extracted from there in this context
	pwshJSONTerminationScript = `
param (
	[Parameter(Mandatory=$true,Position=1)]
	[string]$Path
)

%[1]s -File $Path; $command_exit_code = $LASTEXITCODE
$script_path= %[1]s -command "(Get-Item $Path).BaseName"
$out_json= '{"command_exit_code": ' + $command_exit_code + ', "script": "' + $script_path + '"}'
echo ""
echo "$out_json"
Exit 0
`

	// This script expected the PID of the process which must be terminated with its children
	// It has been designed this way to handle both Kubernetes and Shell executor
	// For Kubernetes executor, the PID is retrieved through a command
	// For Shell executor, the process ID as it is already known
	powershellStageProcessesKillerScript = `
function List-Children ($ProcessId) {
    $children = Get-CIMInstance Win32_Process | Where-Object { $_.ParentProcessId -eq $ProcessId }
	foreach ($child in $children) {
		List-Children $child.ProcessId
		If($child.ProcessId) { Stop-Process -Id $child.ProcessId; }
	}
};

$processId=%s; List-Children $processId
`
)

type powershellChangeUserError struct {
	shell    string
	executor string
}

func (p *powershellChangeUserError) Error() string {
	return fmt.Sprintf("%s doesn't support changing user with the %s executor", p.shell, p.executor)
}

type PowerShell struct {
	AbstractShell
	Shell string
	EOL   string
}

type PsWriter struct {
	bytes.Buffer
	TemporaryPath string
	indent        int
	Shell         string
	EOL           string
	PassFile      bool
	resolvePaths  bool

	useJSONInitializationTermination bool
}

func NewPsWriter(b *PowerShell, info common.ShellScriptInfo) *PsWriter {
	return &PsWriter{
		Shell:         b.Shell,
		EOL:           b.EOL,
		PassFile:      b.passAsFile(info),
		TemporaryPath: info.Build.TmpProjectDir(),
		resolvePaths:  info.Build.IsFeatureFlagOn(featureflags.UsePowershellPathResolver),
		// useJSONInitializationTermination is only used for kubernetes executor when
		// the feature flag FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY is set to false
		useJSONInitializationTermination: info.Build.Runner.Executor == common.ExecutorKubernetes &&
			!info.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy),
	}
}

var encoder = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()

func stdinCmdArgs(shell string, preCmds ...string) []string {
	if shell == SNPwsh {
		return pwshStdinCmdArgs(shell, preCmds...)
	}

	return powershellStdinCmdArgs(shell, preCmds...)
}

func pwshStdinCmdArgs(shell string, preCmds ...string) []string {
	// The stdin script we pass is always UTF-8 encoded, however, depending on
	// how powershell is configured, it may not be expecting UTF-8.
	//
	// To get around this issue, we pass an initialization script which sets
	// the correct input and output encoding.
	//
	// The initialization script then calls '<shell> -Command -', so that our
	// main script is executed by it being passed to stdin like usual.
	//
	// The initilization script itself is encoded so that it can be passed with
	// -EncodeCommand, to avoid potential issues of passing script as an
	// argument. Confusingly, -EncodeCommand expects our initialization script
	// to be base64-encoded utf16.
	//
	// Note: the encoded script, depending on powershell configurations, can be
	// limited to a certain length. The minimum maximum length is 8190. This
	// encoded initialization script should be kept small.
	var sb strings.Builder

	for _, preCmd := range preCmds {
		sb.WriteString(preCmd + "\r\n")
	}
	sb.WriteString("$OutputEncoding = [console]::InputEncoding = [console]::OutputEncoding = New-Object System.Text.UTF8Encoding\r\n")
	sb.WriteString(shell + " -NoProfile -NonInteractive -Command -\r\n")
	sb.WriteString("if(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }")
	encoded, _ := encoder.String(sb.String())

	return append(
		defaultPowershellFlags,
		"-EncodedCommand",
		base64.StdEncoding.EncodeToString([]byte(encoded)),
	)
}

var defaultPowershellFlags = []string{
	"-NoProfile",
	"-NoLogo",
	"-InputFormat",
	"text",
	"-OutputFormat",
	"text",
	"-NonInteractive",
	"-ExecutionPolicy",
	"Bypass",
}

// Avoid using -EncodedCommand due to the powershell progress stream leaking to
// to the output: https://github.com/PowerShell/PowerShell/issues/5912.
func powershellStdinCmdArgs(shell string, preCmds ...string) []string {
	script := "-"

	if len(preCmds) > 0 {
		script = ""
		for _, preCmd := range preCmds {
			script += preCmd + "; "
		}
		script += shell + " -NoProfile -Command -"
	}

	return append(
		defaultPowershellFlags,
		"-Command",
		script,
	)
}

func fileCmdArgs() []string {
	return []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File"}
}

func PwshJSONTerminationScript(shell string) string {
	return fmt.Sprintf(pwshJSONTerminationScript, shell)
}

func PowershellStageProcessesKillerScript(processId string) string {
	return fmt.Sprintf(powershellStageProcessesKillerScript, processId)
}

func PowershellDockerCmd(shell string, preCmds ...string) []string {
	return append([]string{shell}, stdinCmdArgs(shell, preCmds...)...)
}

func psReplaceSpecialChars(text string) string {
	// taken from https://ss64.com/ps/syntax-esc.html
	text = strings.ReplaceAll(text, "`", "``")
	text = strings.ReplaceAll(text, "\a", "`a")
	text = strings.ReplaceAll(text, "\b", "`b")
	text = strings.ReplaceAll(text, "\f", "`f")
	text = strings.ReplaceAll(text, "\r", "`r")
	text = strings.ReplaceAll(text, "\n", "`n")
	text = strings.ReplaceAll(text, "\t", "`t")
	text = strings.ReplaceAll(text, "\v", "`v")
	text = strings.ReplaceAll(text, "#", "`#")
	text = strings.ReplaceAll(text, "'", "`'")
	text = strings.ReplaceAll(text, "\"", "`\"")

	return text
}

func psSingleQuote(text string) string {
	return singleQuote(text)
}

// github.com/PowerShell/PowerShell/blob/v7.3.1/src/System.Management.Automation/engine/parser/CharTraits.cs#L276-L282
func psDoubleQuote(text string) string {
	text = psReplaceSpecialChars(text)
	text = strings.ReplaceAll(text, "“", "`“")
	text = strings.ReplaceAll(text, "”", "`”")
	text = strings.ReplaceAll(text, "„", "`„")
	return doubleQuote(text)
}

func psQuoteVariable(text string) string {
	text = psDoubleQuote(text)
	text = strings.ReplaceAll(text, "$", "`$")
	text = strings.ReplaceAll(text, "``e", "`e")
	return text
}

func (p *PsWriter) GetTemporaryPath() string {
	return p.TemporaryPath
}

func (p *PsWriter) Line(text string) {
	p.WriteString(strings.Repeat("  ", p.indent) + text + p.EOL)
}

func (p *PsWriter) Linef(format string, arguments ...interface{}) {
	p.Line(fmt.Sprintf(format, arguments...))
}

func (p *PsWriter) CheckForErrors() {
	p.checkErrorLevel()
}

func (p *PsWriter) Indent() {
	p.indent++
}

func (p *PsWriter) Unindent() {
	p.indent--
}

func (p *PsWriter) checkErrorLevel() {
	p.Line("if(!$?) { Exit &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }")
	p.Line("")
}

func (p *PsWriter) Command(command string, arguments ...string) {
	p.Line(p.buildCommand(psSingleQuote, command, arguments...))
	p.checkErrorLevel()
}

// CommandWithStdin runs command with arguments and provides stdin to standard input stream of the command
func (p *PsWriter) CommandWithStdin(stdin, command string, arguments ...string) {
	// This mimics something like `echo "foobar" | blipp.exe` for pwsh/powershell, passing in stdin _as is_ to the
	// command. It does not mess with the encoding, BOM, ...
	// The unfortunate side-effect is, that we use a temporary file for this.

	mainCommand := `Start-Process -NoNewWindow -RedirectStandardInput $tmpFile -Wait -FilePath ` + psSingleQuote(command)
	if len(arguments) > 0 {
		for i, arg := range arguments {
			arguments[i] = psSingleQuote(arg)
		}
		mainCommand += " -ArgumentList " + strings.Join(arguments, ",")
	}

	block := strings.Join([]string{
		`try {`,
		`$tmpFile = Get-Item ([System.IO.Path]::GetTempFilename())`,
		fmt.Sprintf(`[System.IO.File]::WriteAllText($tmpFile, %s)`, psSingleQuote(stdin)),
		mainCommand,
		`} finally {`,
		`if ($tmpFile) { Remove-Item -Force -Path $tmpFile }`,
		`}`,
	}, p.EOL)

	p.Line(block)
	p.CheckForErrors()
}

func (p *PsWriter) CommandArgExpand(command string, arguments ...string) {
	p.Line(p.buildCommand(psDoubleQuote, command, arguments...))
	p.checkErrorLevel()
}

func (p *PsWriter) SectionStart(id, command string, options []string) {
	p.Noticef("$ %s", command)
}

func (p *PsWriter) SectionEnd(id string) {}

func (p *PsWriter) buildCommand(quoter stringQuoter, command string, arguments ...string) string {
	list := []string{
		psDoubleQuote(command),
	}

	for _, argument := range arguments {
		list = append(list, quoter(argument))
	}

	return "& " + strings.Join(list, " ")
}

func (p *PsWriter) resolvePath(path string) string {
	if p.resolvePaths {
		return fmt.Sprintf(
			"$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%s)", psDoubleQuote(path),
		)
	}

	return psDoubleQuote(p.fromSlash(path))
}

func (p *PsWriter) TmpFile(name string) string {
	if p.resolvePaths {
		return p.Join(p.TemporaryPath, name)
	}

	return p.cleanPath(p.Join(p.TemporaryPath, name))
}

func (p *PsWriter) cleanPath(name string) string {
	if p.resolvePaths {
		return name
	}

	return p.fromSlash(p.Absolute(name))
}

func (p *PsWriter) fromSlash(path string) string {
	if p.resolvePaths {
		return path
	}

	if p.Shell == SNPwsh {
		// pwsh wants OS slash style, not necessarily backslashes
		return filepath.FromSlash(path)
	}
	return helpers.ToBackslash(path)
}

func (p *PsWriter) EnvVariableKey(name string) string {
	return fmt.Sprintf("$%s", name)
}

func (p *PsWriter) isTmpFile(path string) bool {
	return strings.HasPrefix(path, p.TemporaryPath)
}

func (p *PsWriter) Variable(variable common.JobVariable) {
	if variable.File {
		variableFile := p.TmpFile(variable.Key)
		p.MkDir(p.TemporaryPath)
		p.Linef(
			"[System.IO.File]::WriteAllText(%s, %s)",
			p.resolvePath(variableFile),
			psQuoteVariable(variable.Value),
		)
		p.Linef("$%s=%s", variable.Key, p.resolvePath(variableFile))
	} else {
		if p.isTmpFile(variable.Value) {
			variable.Value = p.cleanPath(variable.Value)
		}

		p.Linef("$%s=%s", variable.Key, psQuoteVariable(variable.Value))
	}

	p.Linef("$env:%s=$%s", variable.Key, variable.Key)
}

func (p *PsWriter) SourceEnv(pathname string) {
	p.MkDir(p.TemporaryPath)
	pathname = p.resolvePath(pathname)
	p.Linef("if(!(Test-Path %s)) { New-Item -ItemType file -Force %s | out-null }", pathname, pathname)
	p.Linef("Try { Get-Content %s | ForEach { $k, $v = $_.split('='); Set-Content env:\\$k $v} } Catch {", pathname)
	p.Indent()
	p.Warningf("Unable to read env file: %s", pathname)
	p.Unindent()
	p.Line("}")
}

func (p *PsWriter) IfDirectory(path string) {
	p.Linef("if(Test-Path %s -PathType Container) {", p.resolvePath(path))
	p.Indent()
}

func (p *PsWriter) IfFile(path string) {
	p.Linef("if(Test-Path %s -PathType Leaf) {", p.resolvePath(path))
	p.Indent()
}

func (p *PsWriter) IfCmd(cmd string, arguments ...string) {
	p.ifInTryCatch(p.buildCommand(psSingleQuote, cmd, arguments...) + " 2>$null")
}

func (p *PsWriter) IfCmdWithOutput(cmd string, arguments ...string) {
	p.ifInTryCatch(p.buildCommand(psSingleQuote, cmd, arguments...))
}

func (p *PsWriter) ifInTryCatch(cmd string) {
	p.Line("Set-Variable -Name cmdErr -Value $false")
	p.Line("Try {")
	p.Indent()
	p.Line(cmd)
	p.Line("if(!$?) { throw &{if($LASTEXITCODE) {$LASTEXITCODE} else {1}} }")
	p.Unindent()
	p.Line("} Catch {")
	p.Indent()
	p.Line("Set-Variable -Name cmdErr -Value $true")
	p.Unindent()
	p.Line("}")
	p.Line("if(!$cmdErr) {")
	p.Indent()
}

func (p *PsWriter) Else() {
	p.Unindent()
	p.Line("} else {")
	p.Indent()
}

func (p *PsWriter) EndIf() {
	p.Unindent()
	p.Line("}")
}

func (p *PsWriter) Cd(path string) {
	p.Line("cd " + p.resolvePath(path))
	p.checkErrorLevel()
}

func (p *PsWriter) MkDir(path string) {
	p.Linef("New-Item -ItemType directory -Force -Path %s | out-null", p.resolvePath(path))
}

func (p *PsWriter) MkTmpDir(name string) string {
	dirPath := p.Join(p.TemporaryPath, name)
	p.MkDir(dirPath)

	return dirPath
}

func (p *PsWriter) RmDir(path string) {
	path = p.resolvePath(path)
	p.Linef(
		"if( (Get-Command -Name Remove-Item2 -Module NTFSSecurity -ErrorAction SilentlyContinue) "+
			"-and (Test-Path %s -PathType Container) ) {",
		path,
	)
	p.Indent()
	p.Line("Remove-Item2 -Force -Recurse " + path)
	p.Unindent()
	p.Linef("} elseif(Test-Path %s) {", path)
	p.Indent()
	p.Line("Remove-Item -Force -Recurse " + path)
	p.Unindent()
	p.Line("}")
	p.Line("")
}

func (p *PsWriter) RmFile(path string) {
	path = p.resolvePath(path)
	p.Line(
		"if( (Get-Command -Name Remove-Item2 -Module NTFSSecurity -ErrorAction SilentlyContinue) " +
			"-and (Test-Path " + path + " -PathType Leaf) ) {")
	p.Indent()
	p.Line("Remove-Item2 -Force " + path)
	p.Unindent()
	p.Linef("} elseif(Test-Path %s) {", path)
	p.Indent()
	p.Line("Remove-Item -Force " + path)
	p.Unindent()
	p.Line("}")
	p.Line("")
}

func (p *PsWriter) RmFilesRecursive(path string, name string) {
	resolvedPath := p.resolvePath(path)
	p.IfDirectory(path)
	p.Linef(
		// `Remove-Item -Recurse` has a known issue (see Example 4 in
		// https://docs.microsoft.com/en-us/powershell/module/microsoft.powershell.management/remove-item)
		"Get-ChildItem -Path %s -Filter %s -Recurse | ForEach-Object { Remove-Item -Force $_.FullName }",
		resolvedPath, psQuoteVariable(name),
	)
	p.EndIf()
}

func (p *PsWriter) Printf(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_RESET + fmt.Sprintf(format, arguments...)
	p.Line("echo " + psQuoteVariable(coloredText))
}

func (p *PsWriter) Noticef(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_BOLD_GREEN + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	p.Line("echo " + psQuoteVariable(coloredText))
}

func (p *PsWriter) Warningf(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_YELLOW + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	p.Line("echo " + psQuoteVariable(coloredText))
}

func (p *PsWriter) Errorf(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_BOLD_RED + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	p.Line("echo " + psQuoteVariable(coloredText))
}

func (p *PsWriter) EmptyLine() {
	p.Line(`echo ""`)
}

func (p *PsWriter) Absolute(dir string) string {
	if p.resolvePaths {
		return dir
	}

	if filepath.IsAbs(dir) {
		return dir
	}

	p.Linef("$CurrentDirectory = (Resolve-Path .%s).Path", string(os.PathSeparator))
	return p.Join("$CurrentDirectory", dir)
}

func (p *PsWriter) Join(elem ...string) string {
	if p.resolvePaths {
		// We rely on the resolve function and always use forward slashes
		// when joining paths.
		return path.Join(elem...)
	}

	return filepath.Join(elem...)
}

func (p *PsWriter) Finish(trace bool) string {
	var buf strings.Builder

	if p.Shell == SNPwsh {
		p.finishPwsh(&buf, trace)
	} else {
		p.finishPowerShell(&buf, trace)
	}

	return buf.String()
}

func (p *PsWriter) finishPwsh(buf *strings.Builder, trace bool) {
	if p.EOL == "\n" {
		buf.WriteString("#!/usr/bin/env " + SNPwsh + p.EOL)
	}

	// All pwsh scripts can and should be wrapped in a script block. Regardless whether they are passed
	// as files or through stdin, this way the whole script will be executed as a block,
	// this was suggested at https://github.com/PowerShell/PowerShell/issues/15331#issuecomment-1016942586.
	// This also fixes things like https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2715 and
	// allows us to bypass file permissions when changing the current user.
	buf.WriteString("& {" + p.EOL + p.EOL)

	if p.useJSONInitializationTermination {
		buf.WriteString(fmt.Sprintf(pwshJSONInitializationScript, p.Shell) + p.EOL + p.EOL)
	}

	if trace {
		buf.WriteString("Set-PSDebug -Trace 2" + p.EOL)
	}

	buf.WriteString(`$ErrorActionPreference = "Stop"` + p.EOL)
	buf.WriteString(p.String() + p.EOL)
	buf.WriteString("}" + p.EOL + p.EOL)
}

func (p *PsWriter) finishPowerShell(buf *strings.Builder, trace bool) {
	if p.PassFile {
		// write UTF-8 BOM (Powershell Core doesn't use a BOM as mentioned in
		// https://gitlab.com/gitlab-org/gitlab-runner/-/issues/3896#note_157830131)
		buf.WriteString("\xef\xbb\xbf")
	} else {
		buf.WriteString("& {" + p.EOL + p.EOL)
	}

	if p.useJSONInitializationTermination {
		buf.WriteString(fmt.Sprintf(pwshJSONInitializationScript, p.Shell) + p.EOL + p.EOL)
	}

	if trace {
		buf.WriteString("Set-PSDebug -Trace 2" + p.EOL)
	}

	buf.WriteString(p.String() + p.EOL)

	if !p.PassFile {
		buf.WriteString("}" + p.EOL + p.EOL)
	}
}

func (b *PowerShell) GetName() string {
	return b.Shell
}

const powershellGitCredHelperScript = `function f([string]$cmd){ if ($cmd.equals("get")) { Write-Host -NoNewline "password=${env:CI_JOB_TOKEN}` + "`n" + `" } }; f`

// GetGitCredHelperCommand returns a command that can be used e.g. in a git config as a credential helper.
//
// This returns something like:
//
//	pwsh -NoProfile ... -Command ''function f{...}; ...; f''
//
// as a single string.
//
// Note the double single-quotes: This is deliberate!
// This command is used with the shellwriter's Command(...), which will quote the whole string in single-quotes. We want
// this to be a single argument (to `git config`) and not split into multiple arguments. Now that we know that the
// result will be single-quoted again, we need to escape any "inner" single-quotes, and we do so doubling them here.
//
// In the end, for a successful configuration, we need the content of the git config to literally look something like:
//
//	[credential "https://gitlab.com"]
//		username = gitlab-ci-token
//		helper = "!pwsh -NoProfile -NoLogo -InputFormat text -OutputFormat text -NonInteractive -ExecutionPolicy Bypass -Command 'function f([string]$cmd){ if ($cmd.equals(\"get\")) { Write-Host -NoNewline \"password=${env:CI_JOB_TOKEN}`n\" } }; f'"
//
// or
//
//	[credential "https://gitlab.com"]
//		username = gitlab-ci-token
//		helper = "!powershell -NoProfile -NoLogo -InputFormat text -OutputFormat text -NonInteractive -ExecutionPolicy Bypass -Command 'function f([string]$cmd){ if ($cmd.equals(\"get\")) { Write-Host -NoNewline \"password=${env:CI_JOB_TOKEN}`n\" } }; f'"
//
// More docs about custom git cred helpers can be found at https://git-scm.com/docs/gitcredentials#_custom_helpers .
func (b *PowerShell) GetGitCredHelperCommand(os string) string {
	shell := b.GetName()
	script := powershellGitCredHelperScript

	// If the OS is not set explicitly, we fallback to the current processes' OS
	os = cmp.Or(os, runtime.GOOS)

	// Some special case for powershell on windows and weird quoting rules thereof.
	// To be honest, I have no clue what's going on there, if this is a powershell thing, or if the shell writer
	// interferes, or both; but it seems to be necessary and to work.
	if shell == SNPowershell && os == OSWindows {
		script = strings.ReplaceAll(script, `"`, `\"`)
	}

	return fmt.Sprintf(
		"%s %s ''%s''",
		shell,
		strings.Join(append(defaultPowershellFlags, "-Command"), " "),
		script,
	)
}

func (b *PowerShell) GetEntrypointCommand(_ common.ShellScriptInfo, probeFile string) []string {
	preCmds := []string{}
	if probeFile != "" {
		preCmds = append(preCmds, fmt.Sprintf("Out-File -Force -FilePath '%s'", probeFile))
	}
	return PowershellDockerCmd(b.Shell, preCmds...)
}

func (b *PowerShell) GetConfiguration(info common.ShellScriptInfo) (*common.ShellConfiguration, error) {
	script := &common.ShellConfiguration{
		Command:       b.Shell,
		PassFile:      b.passAsFile(info),
		Extension:     "ps1",
		DockerCommand: PowershellDockerCmd(b.Shell),
	}

	if info.User != "" {
		if script.PassFile {
			return nil, &powershellChangeUserError{
				shell:    b.Shell,
				executor: info.Build.Runner.Executor,
			}
		}

		script.Command = "su"
		if runtime.GOOS == OSLinux {
			script.Arguments = append(script.Arguments, "-s", "/usr/bin/"+b.Shell)
		}
		script.Arguments = append(
			script.Arguments,
			info.User,
			"-c",
			b.Shell+" "+strings.Join(stdinCmdArgs(b.Shell), " "),
		)
	} else {
		script.Arguments = b.scriptArgs(script)
	}

	script.CmdLine = strings.Join(append([]string{script.Command}, script.Arguments...), " ")

	return script, nil
}

func (b *PowerShell) scriptArgs(script *common.ShellConfiguration) []string {
	if script.PassFile {
		return fileCmdArgs()
	}

	return stdinCmdArgs(b.Shell)
}

func (b *PowerShell) passAsFile(info common.ShellScriptInfo) bool {
	// if DisablePowershellStdin is false, powershell is passed via stdin
	if !info.Build.IsFeatureFlagOn(featureflags.DisablePowershellStdin) {
		return false
	}

	// we only support powershell script by a file for shell & custom executors
	switch info.Build.Runner.Executor {
	case "shell", "custom":
		return true
	}

	return false
}

func (b *PowerShell) GenerateScript(
	ctx context.Context,
	buildStage common.BuildStage,
	info common.ShellScriptInfo,
) (string, error) {
	w := NewPsWriter(b, info)
	return b.generateScript(ctx, w, buildStage, info)
}

func (b *PowerShell) generateScript(
	ctx context.Context,
	w ShellWriter,
	buildStage common.BuildStage,
	info common.ShellScriptInfo,
) (string, error) {
	b.ensurePrepareStageHostnameMessage(w, buildStage, info)
	err := b.writeScript(ctx, w, buildStage, info)
	if err != nil {
		return "", err
	}

	script := w.Finish(info.Build.IsDebugTraceEnabled())
	return script, nil
}

func (b *PowerShell) GenerateSaveScript(info common.ShellScriptInfo, scriptPath, script string) (string, error) {
	w := NewPsWriter(b, info)
	return b.generateSaveScript(w, info, scriptPath, script), nil
}

func (b *PowerShell) generateSaveScript(w *PsWriter, info common.ShellScriptInfo, scriptPath, script string) string {
	var buf strings.Builder
	w.Line(fmt.Sprintf(`$in =%s`, psQuoteVariable(base64.StdEncoding.EncodeToString([]byte(script)))))
	w.Line("$customEncoding = New-Object System.Text.UTF8Encoding $True")
	w.Line(fmt.Sprintf("$sw = [System.IO.StreamWriter]::new(\"%s\", $customEncoding)", scriptPath))
	w.Line("$sw.Write([System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($in)))")
	w.Line("$sw.Flush()")
	w.Line("$sw.Close()")

	buf.WriteString("& {" + w.EOL + w.EOL)

	if info.Build.IsDebugTraceEnabled() {
		buf.WriteString("Set-PSDebug -Trace 2" + w.EOL)
	}

	buf.WriteString(w.String())
	buf.WriteString(w.EOL + w.EOL + "}" + w.EOL + w.EOL)

	return buf.String()
}

func (b *PowerShell) ensurePrepareStageHostnameMessage(
	w ShellWriter,
	buildStage common.BuildStage,
	info common.ShellScriptInfo,
) {
	if buildStage == common.BuildStagePrepare {
		if info.Build.Hostname != "" {
			w.Line(
				fmt.Sprintf(
					`echo "Running on $([Environment]::MachineName) via %s..."`,
					psQuoteVariable(info.Build.Hostname),
				),
			)
		} else {
			w.Line(`echo "Running on $([Environment]::MachineName)..."`)
		}
	}
}

func (b *PowerShell) IsDefault() bool {
	return runtime.GOOS == OSWindows
}

func init() {
	eol := "\r\n"
	if runtime.GOOS != OSWindows {
		eol = "\n"
	}

	common.RegisterShell(&PowerShell{Shell: SNPwsh, EOL: eol})
	common.RegisterShell(&PowerShell{Shell: SNPowershell, EOL: "\r\n"})
}
