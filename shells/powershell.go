package shells

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

const (
	dockerWindowsExecutor = "docker-windows"

	SNPwsh       = "pwsh"
	SNPowershell = "powershell"

	// Before executing a script, powershell parses it.
	// A `ParserError` can then be thrown if a parsing error is found.
	// Those errors are not catched by the powershell_trap_script thus causing the job to hang
	// To avoid this problem, the PwshValidationScript is used to validate the given script and eventually to cause
	// the job to fail if a `ParserError` is thrown
	pwshJSONTerminationScript = `
param (
	[Parameter(Mandatory=$true,Position=1)]
	[string]$Path
)

%s -File $Path
$out_json= '{"command_exit_code": ' + $LASTEXITCODE + ', "script": "' + $MyInvocation.MyCommand.Name + '"}'
echo ""
echo "$out_json"
Exit 0
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
	resolvePaths  bool
}

func stdinCmdArgs() []string {
	return []string{
		"-NoProfile",
		"-NoLogo",
		"-InputFormat",
		"text",
		"-OutputFormat",
		"text",
		"-NonInteractive",
		"-ExecutionPolicy",
		"Bypass",
		"-Command",
		"-",
	}
}

func fileCmdArgs() []string {
	return []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command"}
}

func PwshJSONTerminationScript(shell string) string {
	return fmt.Sprintf(pwshJSONTerminationScript, shell)
}

func PowershellDockerCmd(shell string) []string {
	return append([]string{shell}, stdinCmdArgs()...)
}

func psQuote(text string) string {
	// taken from: http://www.robvanderwoude.com/escapechars.php
	text = strings.ReplaceAll(text, "`", "``")
	// text = strings.ReplaceAll(text, "\0", "`0")
	text = strings.ReplaceAll(text, "\a", "`a")
	text = strings.ReplaceAll(text, "\b", "`b")
	text = strings.ReplaceAll(text, "\f", "^f")
	text = strings.ReplaceAll(text, "\r", "`r")
	text = strings.ReplaceAll(text, "\n", "`n")
	text = strings.ReplaceAll(text, "\t", "^t")
	text = strings.ReplaceAll(text, "\v", "^v")
	text = strings.ReplaceAll(text, "#", "`#")
	text = strings.ReplaceAll(text, "'", "`'")
	text = strings.ReplaceAll(text, "\"", "`\"")
	return `"` + text + `"`
}

func psQuoteVariable(text string) string {
	text = psQuote(text)
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
	p.Line(p.buildCommand(command, arguments...))
	p.checkErrorLevel()
}

func (p *PsWriter) SectionStart(id, command string) {}

func (p *PsWriter) SectionEnd(id string) {}

func (p *PsWriter) buildCommand(command string, arguments ...string) string {
	list := []string{
		psQuote(command),
	}

	for _, argument := range arguments {
		list = append(list, psQuote(argument))
	}

	return "& " + strings.Join(list, " ")
}

func (p *PsWriter) resolvePath(path string) string {
	if p.resolvePaths {
		return fmt.Sprintf("$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath(%s)", psQuote(path))
	}

	return psQuote(p.fromSlash(path))
}

func (p *PsWriter) TmpFile(name string) string {
	if p.resolvePaths {
		return p.Join(p.TemporaryPath, name)
	}

	filePath := p.Absolute(p.Join(p.TemporaryPath, name))
	return p.fromSlash(filePath)
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
		p.Linef("$%s=%s", variable.Key, psQuoteVariable(variable.Value))
	}

	p.Linef("$env:%s=$%s", variable.Key, variable.Key)
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
	p.ifInTryCatch(p.buildCommand(cmd, arguments...) + " 2>$null")
}

func (p *PsWriter) IfCmdWithOutput(cmd string, arguments ...string) {
	p.ifInTryCatch(p.buildCommand(cmd, arguments...))
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
		resolvedPath, psQuote(name),
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

	if trace {
		buf.WriteString("Set-PSDebug -Trace 2" + p.EOL)
	}

	buf.WriteString(`$ErrorActionPreference = "Stop"` + p.EOL)
	buf.WriteString(p.String() + p.EOL)
	buf.WriteString("}" + p.EOL + p.EOL)
}

func (p *PsWriter) finishPowerShell(buf *strings.Builder, trace bool) {
	// write UTF-8 BOM (Powershell Core doesn't use a BOM as mentioned in
	// https://gitlab.com/gitlab-org/gitlab-runner/-/issues/3896#note_157830131)
	buf.WriteString("\xef\xbb\xbf")

	if trace {
		buf.WriteString("Set-PSDebug -Trace 2" + p.EOL)
	}

	buf.WriteString(p.String() + p.EOL)
}

func (b *PowerShell) GetName() string {
	return b.Shell
}

func (b *PowerShell) GetConfiguration(info common.ShellScriptInfo) (*common.ShellConfiguration, error) {
	script := &common.ShellConfiguration{
		Command:       b.Shell,
		PassFile:      b.Shell != SNPwsh && info.Build.Runner.Executor != dockerWindowsExecutor,
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
			b.Shell+" "+strings.Join(stdinCmdArgs(), " "),
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

	return stdinCmdArgs()
}

func (b *PowerShell) GenerateScript(buildStage common.BuildStage, info common.ShellScriptInfo) (string, error) {
	w := &PsWriter{
		Shell:         b.Shell,
		EOL:           b.EOL,
		TemporaryPath: info.Build.TmpProjectDir(),
		resolvePaths:  info.Build.IsFeatureFlagOn(featureflags.UsePowershellPathResolver),
	}

	return b.generateScript(w, buildStage, info)
}

func (b *PowerShell) generateScript(
	w ShellWriter,
	buildStage common.BuildStage,
	info common.ShellScriptInfo,
) (string, error) {
	b.ensurePrepareStageHostnameMessage(w, buildStage, info)
	err := b.writeScript(w, buildStage, info)
	if err != nil {
		return "", err
	}

	script := w.Finish(info.Build.IsDebugTraceEnabled())
	return script, nil
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
	return false
}

func init() {
	eol := "\r\n"
	if runtime.GOOS != OSWindows {
		eol = "\n"
	}

	common.RegisterShell(&PowerShell{Shell: SNPwsh, EOL: eol})
	common.RegisterShell(&PowerShell{Shell: SNPowershell, EOL: "\r\n"})
}
