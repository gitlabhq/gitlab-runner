package shells

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

const (
	dockerWindowsExecutor = "docker-windows"

	SNPwsh       = "pwsh"
	SNPowershell = "powershell"
)

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

func (p *PsWriter) buildCommand(command string, arguments ...string) string {
	list := []string{
		psQuote(command),
	}

	for _, argument := range arguments {
		list = append(list, psQuote(argument))
	}

	return "& " + strings.Join(list, " ")
}

func (p *PsWriter) TmpFile(name string) string {
	filePath := p.Absolute(filepath.Join(p.TemporaryPath, name))
	return p.fromSlash(filePath)
}

func (p *PsWriter) fromSlash(path string) string {
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
		p.Linef(
			"New-Item -ItemType directory -Force -Path %s | out-null",
			psQuote(p.fromSlash(p.TemporaryPath)),
		)
		p.Linef(
			"Set-Content %s -Value %s -Encoding UTF8 -Force",
			psQuote(variableFile),
			psQuoteVariable(variable.Value),
		)
		p.Linef("$%s=%s", variable.Key, psQuote(variableFile))
	} else {
		p.Linef("$%s=%s", variable.Key, psQuoteVariable(variable.Value))
	}

	p.Linef("$env:%s=$%s", variable.Key, variable.Key)
}

func (p *PsWriter) IfDirectory(path string) {
	p.Linef("if(Test-Path %s -PathType Container) {", psQuote(p.fromSlash(path)))
	p.Indent()
}

func (p *PsWriter) IfFile(path string) {
	p.Linef("if(Test-Path %s -PathType Leaf) {", psQuote(p.fromSlash(path)))
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
	p.Line("cd " + psQuote(p.fromSlash(path)))
	p.checkErrorLevel()
}

func (p *PsWriter) MkDir(path string) {
	p.Linef("New-Item -ItemType directory -Force -Path %s | out-null", psQuote(p.fromSlash(path)))
}

func (p *PsWriter) MkTmpDir(name string) string {
	dirPath := filepath.Join(p.TemporaryPath, name)
	p.MkDir(dirPath)

	return dirPath
}

func (p *PsWriter) RmDir(path string) {
	path = psQuote(p.fromSlash(path))
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
	path = psQuote(p.fromSlash(path))
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
	if filepath.IsAbs(dir) {
		return dir
	}

	p.Linef("$CurrentDirectory = (Resolve-Path .%s).Path", string(os.PathSeparator))
	return filepath.Join("$CurrentDirectory", dir)
}

func (p *PsWriter) Join(elem ...string) string {
	newPath := filepath.Join(elem...)
	return newPath
}

func (p *PsWriter) Finish(trace bool) string {
	var buffer bytes.Buffer
	w := bufio.NewWriter(&buffer)

	// write BOM
	_, _ = io.WriteString(w, "\xef\xbb\xbf")

	p.writeShebang(w)
	p.writeTrace(w, trace)
	if p.Shell == SNPwsh {
		_, _ = io.WriteString(w, `$ErrorActionPreference = "Stop"`+p.EOL+p.EOL)
	}

	// add empty line to close code-block when it is piped to STDIN
	p.Line("")
	_, _ = io.WriteString(w, p.String())
	_ = w.Flush()
	return buffer.String()
}

func (p *PsWriter) writeShebang(w io.Writer) {
	switch p.Shell {
	case SNPwsh:
		_, _ = io.WriteString(w, "#requires -PSEdition Core"+p.EOL+p.EOL)
	case SNPowershell:
		_, _ = io.WriteString(w, "#requires -PSEdition Desktop"+p.EOL+p.EOL)
	}
}

func (p *PsWriter) writeTrace(w io.Writer, trace bool) {
	if trace {
		_, _ = io.WriteString(w, "Set-PSDebug -Trace 2"+p.EOL)
	}
}

func (b *PowerShell) GetName() string {
	return b.Shell
}

func (b *PowerShell) GetConfiguration(info common.ShellScriptInfo) (script *common.ShellConfiguration, err error) {
	script = &common.ShellConfiguration{
		Command:   b.Shell,
		Arguments: []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command"},
		PassFile:  info.Build.Runner.Executor != dockerWindowsExecutor,
		Extension: "ps1",
		DockerCommand: []string{
			b.Shell,
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
		},
	}
	return
}

func (b *PowerShell) GenerateScript(
	buildStage common.BuildStage,
	info common.ShellScriptInfo,
) (script string, err error) {
	w := &PsWriter{
		Shell:         b.Shell,
		EOL:           b.EOL,
		TemporaryPath: info.Build.TmpProjectDir(),
	}

	if buildStage == common.BuildStagePrepare {
		if info.Build.Hostname != "" {
			w.Linef(
				`echo "Running on $([Environment]::MachineName) via %s..."`,
				psQuoteVariable(info.Build.Hostname),
			)
		} else {
			w.Line(`echo "Running on $([Environment]::MachineName)..."`)
		}
	}

	err = b.writeScript(w, buildStage, info)

	// No need to set up BOM or tracing since no script was generated.
	if w.Buffer.Len() > 0 {
		script = w.Finish(info.Build.IsDebugTraceEnabled())
	}

	return
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
