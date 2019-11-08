package shells

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

type CmdShell struct {
	AbstractShell
}

type CmdWriter struct {
	bytes.Buffer
	TemporaryPath                     string
	indent                            int
	disableDelayedErrorLevelExpansion bool
}

func batchQuote(text string) string {
	return "\"" + batchEscapeInsideQuotedString(text) + "\""
}

func batchEscapeInsideQuotedString(text string) string {
	// taken from: http://www.robvanderwoude.com/escapechars.php
	text = strings.Replace(text, "^", "^^", -1)
	text = strings.Replace(text, "!", "^^!", -1)
	text = strings.Replace(text, "&", "^&", -1)
	text = strings.Replace(text, "<", "^<", -1)
	text = strings.Replace(text, ">", "^>", -1)
	text = strings.Replace(text, "|", "^|", -1)
	text = strings.Replace(text, "\r", "", -1)
	text = strings.Replace(text, "\n", "!nl!", -1)
	return text
}

func batchEscapeVariable(text string) string {
	text = strings.Replace(text, "%", "%%", -1)
	text = batchEscape(text)
	return text
}

// If not inside a quoted string (e.g., echo text), escape more things
func batchEscape(text string) string {
	text = batchEscapeInsideQuotedString(text)
	text = strings.Replace(text, "(", "^(", -1)
	text = strings.Replace(text, ")", "^)", -1)
	return text
}

func (b *CmdShell) GetName() string {
	return "cmd"
}

func (b *CmdWriter) GetTemporaryPath() string {
	return b.TemporaryPath
}

func (b *CmdWriter) Line(text string) {
	b.WriteString(strings.Repeat("  ", b.indent) + text + "\r\n")
}

func (b *CmdWriter) CheckForErrors() {
	b.checkErrorLevel()
}

func (b *CmdWriter) Indent() {
	b.indent++
}

func (b *CmdWriter) Unindent() {
	b.indent--
}

func (b *CmdWriter) checkErrorLevel() {
	errCheck := "IF !errorlevel! NEQ 0 exit /b !errorlevel!"
	b.Line(b.updateErrLevelCheck(errCheck))
	b.Line("")
}

func (b *CmdWriter) updateErrLevelCheck(errCheck string) string {
	if b.disableDelayedErrorLevelExpansion {
		return strings.Replace(errCheck, "!", "%", -1)
	}

	return errCheck
}

func (b *CmdWriter) Command(command string, arguments ...string) {
	b.Line(b.buildCommand(command, arguments...))
	b.checkErrorLevel()
}

func (b *CmdWriter) buildCommand(command string, arguments ...string) string {
	list := []string{
		batchQuote(command),
	}

	for _, argument := range arguments {
		list = append(list, batchQuote(argument))
	}

	return strings.Join(list, " ")
}

func (b *CmdWriter) TmpFile(name string) string {
	filePath := b.Absolute(path.Join(b.TemporaryPath, name))
	return helpers.ToBackslash(filePath)
}

func (b *CmdWriter) EnvVariableKey(name string) string {
	return fmt.Sprintf("%%%s%%", name)
}

func (b *CmdWriter) Variable(variable common.JobVariable) {
	if variable.File {
		variableFile := b.TmpFile(variable.Key)
		b.Line(fmt.Sprintf("md %q 2>NUL 1>NUL", batchEscape(helpers.ToBackslash(b.TemporaryPath))))
		b.Line(fmt.Sprintf("echo %s > %s", batchEscapeVariable(variable.Value), batchEscape(variableFile)))
		b.Line("SET " + batchEscapeVariable(variable.Key) + "=" + batchEscape(variableFile))
	} else {
		b.Line("SET " + batchEscapeVariable(variable.Key) + "=" + batchEscapeVariable(variable.Value))
	}
}

func (b *CmdWriter) IfDirectory(path string) {
	b.Line("IF EXIST " + batchQuote(helpers.ToBackslash(path)) + " (")
	b.Indent()
}

func (b *CmdWriter) IfFile(path string) {
	b.Line("IF EXIST " + batchQuote(helpers.ToBackslash(path)) + " (")
	b.Indent()
}

func (b *CmdWriter) IfCmd(cmd string, arguments ...string) {
	cmdline := b.buildCommand(cmd, arguments...)
	b.Line(fmt.Sprintf("%s 2>NUL 1>NUL", cmdline))
	errCheck := "IF !errorlevel! EQU 0 ("
	b.Line(b.updateErrLevelCheck(errCheck))
	b.Indent()
}

func (b *CmdWriter) IfCmdWithOutput(cmd string, arguments ...string) {
	cmdline := b.buildCommand(cmd, arguments...)
	b.Line(cmdline)
	errCheck := "IF !errorlevel! EQU 0 ("
	b.Line(b.updateErrLevelCheck(errCheck))
	b.Indent()
}

func (b *CmdWriter) Else() {
	b.Unindent()
	b.Line(") ELSE (")
	b.Indent()
}

func (b *CmdWriter) EndIf() {
	b.Unindent()
	b.Line(")")
}

func (b *CmdWriter) Cd(path string) {
	b.Line("cd /D " + batchQuote(helpers.ToBackslash(path)))
	b.checkErrorLevel()
}

func (b *CmdWriter) MkDir(path string) {
	args := batchQuote(helpers.ToBackslash(path)) + " 2>NUL 1>NUL"
	b.Line("dir " + args + " || md " + args)
}

func (b *CmdWriter) MkTmpDir(name string) string {
	path := helpers.ToBackslash(path.Join(b.TemporaryPath, name))
	b.MkDir(path)

	return path
}

func (b *CmdWriter) RmDir(path string) {
	b.Line("rd /s /q " + batchQuote(helpers.ToBackslash(path)) + " 2>NUL 1>NUL")
}

func (b *CmdWriter) RmFile(path string) {
	b.Line("del /f /q " + batchQuote(helpers.ToBackslash(path)) + " 2>NUL 1>NUL")
}

func (b *CmdWriter) Print(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_RESET + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + batchEscapeVariable(coloredText))
}

func (b *CmdWriter) Notice(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_BOLD_GREEN + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + batchEscapeVariable(coloredText))
}

func (b *CmdWriter) Warning(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_YELLOW + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + batchEscapeVariable(coloredText))
}

func (b *CmdWriter) Error(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_BOLD_RED + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + batchEscapeVariable(coloredText))
}

func (b *CmdWriter) EmptyLine() {
	b.Line("echo.")
}

func (b *CmdWriter) Absolute(dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join("%CD%", dir)
}

func (b *CmdWriter) Finish(trace bool) string {
	var buffer bytes.Buffer
	w := bufio.NewWriter(&buffer)

	if trace {
		io.WriteString(w, "@echo on\r\n")
	} else {
		io.WriteString(w, "@echo off\r\n")
	}

	io.WriteString(w, "setlocal enableextensions\r\n")
	io.WriteString(w, "setlocal enableDelayedExpansion\r\n")
	io.WriteString(w, "set nl=^\r\n\r\n\r\n")

	io.WriteString(w, b.String())
	w.Flush()
	return buffer.String()
}

func (b *CmdShell) GetConfiguration(info common.ShellScriptInfo) (script *common.ShellConfiguration, err error) {
	script = &common.ShellConfiguration{
		Command:   "cmd",
		Arguments: []string{"/C"},
		PassFile:  true,
		Extension: "cmd",
	}
	return
}

func (b *CmdShell) GenerateScript(buildStage common.BuildStage, info common.ShellScriptInfo) (script string, err error) {
	w := &CmdWriter{
		TemporaryPath:                     info.Build.TmpProjectDir(),
		disableDelayedErrorLevelExpansion: info.Build.IsFeatureFlagOn(featureflags.CmdDisableDelayedErrorLevelExpansion),
	}

	if buildStage == common.BuildStagePrepare {
		if len(info.Build.Hostname) != 0 {
			w.Line("echo Running on %COMPUTERNAME% via " + batchEscape(info.Build.Hostname) + "...")
		} else {
			w.Line("echo Running on %COMPUTERNAME%...")
		}

		w.Warning("DEPRECATION: CMD shell is deprecated and will be removed in 13.0: https://gitlab.com/gitlab-org/gitlab-runner/issues/4163")
	}

	err = b.writeScript(w, buildStage, info)
	script = w.Finish(info.Build.IsDebugTraceEnabled())
	return
}

func (b *CmdShell) IsDefault() bool {
	// TODO: Remove in 13.0 - Make PowerShell default shell for Windows.
	return runtime.GOOS == "windows"
}

func init() {
	common.RegisterShell(&CmdShell{})
}
