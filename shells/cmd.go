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
	// can't use doubleQuoter here since that causes the backlashes to be escaped which doesn't work well
	return "\"" + batchEscapeInsideQuotedString(text) + "\""
}

func batchQuoteEscapeCommand(text string) string {
	text = batchEscapeInsideQuotedString(text)
	text = strings.ReplaceAll(text, "%", "^%")
	return "\"" + text + "\""
}

func batchEscapeInsideQuotedString(text string) string {
	// taken from: http://www.robvanderwoude.com/escapechars.php
	text = strings.ReplaceAll(text, "^", "^^")
	text = strings.ReplaceAll(text, "!", "^^!")
	text = strings.ReplaceAll(text, "&", "^&")
	text = strings.ReplaceAll(text, "<", "^<")
	text = strings.ReplaceAll(text, ">", "^>")
	text = strings.ReplaceAll(text, "|", "^|")
	text = strings.ReplaceAll(text, "\r", "")
	text = strings.ReplaceAll(text, "\n", "!nl!")
	return text
}

func batchEscapeVariable(text string) string {
	text = strings.ReplaceAll(text, "%", "%%")
	text = batchEscape(text)
	return text
}

// If not inside a quoted string (e.g., echo text), escape more things
func batchEscape(text string) string {
	text = batchEscapeInsideQuotedString(text)
	text = strings.ReplaceAll(text, "(", "^(")
	text = strings.ReplaceAll(text, ")", "^)")
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

func (b *CmdWriter) Linef(format string, arguments ...interface{}) {
	b.Line(fmt.Sprintf(format, arguments...))
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
		return strings.ReplaceAll(errCheck, "!", "%")
	}

	return errCheck
}

func (b *CmdWriter) Command(command string, arguments ...string) {
	b.Line(b.buildCommand(batchQuoteEscapeCommand, command, arguments...))
	b.checkErrorLevel()
}

func (b *CmdWriter) CommandArgExpand(command string, arguments ...string) {
	b.Line(b.buildCommand(batchQuote, command, arguments...))
	b.checkErrorLevel()
}

func (b *CmdWriter) buildCommand(quoter stringQuoter, command string, arguments ...string) string {
	list := []string{
		batchQuote(command),
	}

	for _, argument := range arguments {
		list = append(list, quoter(argument))
	}

	return strings.Join(list, " ")
}

func (b *CmdWriter) TmpFile(name string) string {
	return b.cleanPath(path.Join(b.TemporaryPath, name))
}

func (b *CmdWriter) cleanPath(name string) string {
	return helpers.ToBackslash(b.Absolute(name))
}

func (b *CmdWriter) EnvVariableKey(name string) string {
	return fmt.Sprintf("%%%s%%", name)
}

func (b *CmdWriter) isTmpFile(path string) bool {
	return strings.HasPrefix(path, b.TemporaryPath)
}

func (b *CmdWriter) Variable(variable common.JobVariable) {
	if variable.File {
		variableFile := b.TmpFile(variable.Key)
		b.Linef("md %q 2>NUL 1>NUL", batchEscape(helpers.ToBackslash(b.TemporaryPath)))
		b.Linef("echo %s > %s", batchEscapeVariable(variable.Value), batchEscape(variableFile))
		b.Linef("SET %s=%s", batchEscapeVariable(variable.Key), batchEscape(variableFile))
	} else {
		if b.isTmpFile(variable.Value) {
			variable.Value = b.cleanPath(variable.Value)
		}
		b.Linef("SET %s=%s", batchEscapeVariable(variable.Key), batchEscapeVariable(variable.Value))
	}
}

func (b *CmdWriter) IfDirectory(path string) {
	b.Linef("IF EXIST %s (", batchQuote(helpers.ToBackslash(path)))
	b.Indent()
}

func (b *CmdWriter) IfFile(path string) {
	b.Linef("IF EXIST %s (", batchQuote(helpers.ToBackslash(path)))
	b.Indent()
}

func (b *CmdWriter) IfCmd(cmd string, arguments ...string) {
	cmdline := b.buildCommand(batchQuote, cmd, arguments...)
	b.Linef("%s 2>NUL 1>NUL", cmdline)
	errCheck := "IF !errorlevel! EQU 0 ("
	b.Line(b.updateErrLevelCheck(errCheck))
	b.Indent()
}

func (b *CmdWriter) IfCmdWithOutput(cmd string, arguments ...string) {
	cmdline := b.buildCommand(batchQuote, cmd, arguments...)
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
	b.Linef("dir %s || md %s", args, args)
}

func (b *CmdWriter) MkTmpDir(name string) string {
	path := helpers.ToBackslash(path.Join(b.TemporaryPath, name))
	b.MkDir(path)

	return path
}

func (b *CmdWriter) RmDir(path string) {
	b.Linef("rd /s /q %s 2>NUL 1>NUL", batchQuote(helpers.ToBackslash(path)))
}

func (b *CmdWriter) RmFile(path string) {
	b.Linef("del /f /q %s 2>NUL 1>NUL", batchQuote(helpers.ToBackslash(path)))
}

func (b *CmdWriter) RmFilesRecursive(path string, name string) {}

func (b *CmdWriter) Printf(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_RESET + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + batchEscapeVariable(coloredText))
}

func (b *CmdWriter) Noticef(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_BOLD_GREEN + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + batchEscapeVariable(coloredText))
}

func (b *CmdWriter) Warningf(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_YELLOW + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + batchEscapeVariable(coloredText))
}

func (b *CmdWriter) Errorf(format string, arguments ...interface{}) {
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

func (b *CmdWriter) Join(elem ...string) string {
	newPath := path.Join(elem...)
	return helpers.ToBackslash(newPath)
}

func (b *CmdWriter) SectionStart(id, command string) {}

func (b *CmdWriter) SectionEnd(id string) {}

func (b *CmdWriter) Finish(trace bool) string {
	var buffer bytes.Buffer
	w := bufio.NewWriter(&buffer)

	if trace {
		_, _ = io.WriteString(w, "@echo on\r\n")
	} else {
		_, _ = io.WriteString(w, "@echo off\r\n")
	}

	_, _ = io.WriteString(w, "setlocal enableextensions\r\n")
	_, _ = io.WriteString(w, "setlocal enableDelayedExpansion\r\n")
	_, _ = io.WriteString(w, "set nl=^\r\n\r\n\r\n")

	_, _ = io.WriteString(w, b.String())
	_ = w.Flush()
	return buffer.String()
}

func (b *CmdShell) GetConfiguration(info common.ShellScriptInfo) (script *common.ShellConfiguration, err error) {
	script = &common.ShellConfiguration{
		Command:   "cmd",
		Arguments: []string{"/C"},
		PassFile:  true,
		Extension: "cmd",
		CmdLine:   "cmd /C",
	}
	return
}

func (b *CmdShell) GenerateScript(
	buildStage common.BuildStage,
	info common.ShellScriptInfo,
) (script string, err error) {
	//nolint:lll
	w := &CmdWriter{
		TemporaryPath:                     info.Build.TmpProjectDir(),
		disableDelayedErrorLevelExpansion: info.Build.IsFeatureFlagOn(featureflags.CmdDisableDelayedErrorLevelExpansion),
	}

	if buildStage == common.BuildStagePrepare {
		if info.Build.Hostname != "" {
			w.Line("echo Running on %COMPUTERNAME% via " + batchEscape(info.Build.Hostname) + "...")
		} else {
			w.Line("echo Running on %COMPUTERNAME%...")
		}

		w.Warningf("DEPRECATION: CMD shell is deprecated and will no longer be supported")
	}

	err = b.writeScript(w, buildStage, info)
	script = w.Finish(info.Build.IsDebugTraceEnabled())
	return
}

func (b *CmdShell) GenerateSaveScript(info common.ShellScriptInfo, scriptPath, script string) (string, error) {
	return "", nil
}

func (b *CmdShell) IsDefault() bool {
	return runtime.GOOS == OSWindows
}

func init() {
	common.RegisterShell(&CmdShell{})
}
