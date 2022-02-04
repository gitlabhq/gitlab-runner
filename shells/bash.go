package shells

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"path"
	"runtime"
	"strconv"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

const BashDetectShellScript = `if [ -x /usr/local/bin/bash ]; then
	exec /usr/local/bin/bash $@
elif [ -x /usr/bin/bash ]; then
	exec /usr/bin/bash $@
elif [ -x /bin/bash ]; then
	exec /bin/bash $@
elif [ -x /usr/local/bin/sh ]; then
	exec /usr/local/bin/sh $@
elif [ -x /usr/bin/sh ]; then
	exec /usr/bin/sh $@
elif [ -x /bin/sh ]; then
	exec /bin/sh $@
elif [ -x /busybox/sh ]; then
	exec /busybox/sh $@
else
	echo shell not found
	exit 1
fi

`

type BashShell struct {
	AbstractShell
	Shell string
}

type BashWriter struct {
	bytes.Buffer
	TemporaryPath string
	Shell         string
	indent        int

	checkForErrors bool
	useNewEval     bool
}

func (b *BashWriter) GetTemporaryPath() string {
	return b.TemporaryPath
}

func (b *BashWriter) Line(text string) {
	b.WriteString(strings.Repeat("  ", b.indent) + text + "\n")
}

func (b *BashWriter) Linef(format string, arguments ...interface{}) {
	b.Line(fmt.Sprintf(format, arguments...))
}

func (b *BashWriter) CheckForErrors() {
	if !b.checkForErrors {
		return
	}

	b.Line("_runner_exit_code=$?; if [[ $_runner_exit_code -ne 0 ]]; then exit $_runner_exit_code; fi")
}

func (b *BashWriter) Exit(code int) {
	b.Line(fmt.Sprintf("exit %d", code))
	b.Line("")
}

func (b *BashWriter) Indent() {
	b.indent++
}

func (b *BashWriter) Unindent() {
	b.indent--
}

func (b *BashWriter) Command(command string, arguments ...string) {
	b.Line(b.buildCommand(command, arguments...))
	b.CheckForErrors()
}

func (b *BashWriter) buildCommand(command string, arguments ...string) string {
	list := []string{
		helpers.ShellEscape(command),
	}

	for _, argument := range arguments {
		list = append(list, strconv.Quote(argument))
	}

	return strings.Join(list, " ")
}

func (b *BashWriter) TmpFile(name string) string {
	return b.Absolute(path.Join(b.TemporaryPath, name))
}

func (b *BashWriter) EnvVariableKey(name string) string {
	return fmt.Sprintf("$%s", name)
}

func (b *BashWriter) Variable(variable common.JobVariable) {
	if variable.File {
		variableFile := b.TmpFile(variable.Key)
		b.Linef("mkdir -p %q", helpers.ToSlash(b.TemporaryPath))
		b.Linef("echo -n %s > %q", helpers.ShellEscape(variable.Value), variableFile)
		b.Linef("export %s=%q", helpers.ShellEscape(variable.Key), variableFile)
	} else {
		b.Linef("export %s=%s", helpers.ShellEscape(variable.Key), helpers.ShellEscape(variable.Value))
	}
}

func (b *BashWriter) IfDirectory(path string) {
	b.Linef("if [[ -d %q ]]; then", path)
	b.Indent()
}

func (b *BashWriter) IfFile(path string) {
	b.Linef("if [[ -e %q ]]; then", path)
	b.Indent()
}

func (b *BashWriter) IfCmd(cmd string, arguments ...string) {
	cmdline := b.buildCommand(cmd, arguments...)
	b.Linef("if %s >/dev/null 2>/dev/null; then", cmdline)
	b.Indent()
}

func (b *BashWriter) IfCmdWithOutput(cmd string, arguments ...string) {
	cmdline := b.buildCommand(cmd, arguments...)
	b.Linef("if %s; then", cmdline)
	b.Indent()
}

func (b *BashWriter) Else() {
	b.Unindent()
	b.Line("else")
	b.Indent()
}

func (b *BashWriter) EndIf() {
	b.Unindent()
	b.Line("fi")
}

func (b *BashWriter) Cd(path string) {
	b.Command("cd", path)
}

func (b *BashWriter) MkDir(path string) {
	b.Command("mkdir", "-p", path)
}

func (b *BashWriter) MkTmpDir(name string) string {
	path := path.Join(b.TemporaryPath, name)
	b.MkDir(path)

	return path
}

func (b *BashWriter) RmDir(path string) {
	b.Command("rm", "-r", "-f", path)
}

func (b *BashWriter) RmFile(path string) {
	b.Command("rm", "-f", path)
}

func (b *BashWriter) Absolute(dir string) string {
	if path.IsAbs(dir) {
		return dir
	}
	return path.Join("$PWD", dir)
}

func (b *BashWriter) Join(elem ...string) string {
	return path.Join(elem...)
}

func (b *BashWriter) Printf(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_RESET + fmt.Sprintf(format, arguments...)
	b.Line("echo " + helpers.ShellEscape(coloredText))
}

func (b *BashWriter) Noticef(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_BOLD_GREEN + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + helpers.ShellEscape(coloredText))
}

func (b *BashWriter) Warningf(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_YELLOW + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + helpers.ShellEscape(coloredText))
}

func (b *BashWriter) Errorf(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_BOLD_RED + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + helpers.ShellEscape(coloredText))
}

func (b *BashWriter) EmptyLine() {
	b.Line("echo")
}

func (b *BashWriter) Finish(trace bool) string {
	var buffer bytes.Buffer
	w := bufio.NewWriter(&buffer)

	b.writeShebang(w)
	b.writeTrace(w, trace)
	b.writeScript(w)

	_ = w.Flush()
	return buffer.String()
}

func (b *BashWriter) writeShebang(w io.Writer) {
	if b.Shell != "" {
		_, _ = io.WriteString(w, "#!/usr/bin/env "+b.Shell+"\n\n")
	}
}

func (b *BashWriter) writeTrace(w io.Writer, trace bool) {
	if trace {
		_, _ = io.WriteString(w, "set -o xtrace\n")
	}
}

func (b *BashWriter) writeEval(w io.Writer) {
	command := ": | eval " + helpers.ShellEscape(b.String()) + "\n"
	if b.useNewEval {
		command = ": | (eval " + helpers.ShellEscape(b.String()) + ")\n"
	}

	_, _ = io.WriteString(w, command)
}

func (b *BashWriter) writeScript(w io.Writer) {
	_, _ = io.WriteString(w, "set -eo pipefail\n")
	_, _ = io.WriteString(w, "set +o noclobber\n")
	b.writeEval(w)
	_, _ = io.WriteString(w, "exit 0\n")
}

func (b *BashShell) GetName() string {
	return b.Shell
}

func (b *BashShell) GetConfiguration(info common.ShellScriptInfo) (*common.ShellConfiguration, error) {
	var detectScript string
	var shellCommand string
	if info.Type == common.LoginShell {
		detectScript = strings.ReplaceAll(BashDetectShellScript, "$@", "--login")
		shellCommand = b.Shell + " --login"
	} else {
		detectScript = strings.ReplaceAll(BashDetectShellScript, "$@", "")
		shellCommand = b.Shell
	}

	script := &common.ShellConfiguration{}
	script.DockerCommand = []string{"sh", "-c", detectScript}

	// su
	if info.User != "" {
		script.Command = "su"
		if runtime.GOOS == "linux" {
			script.Arguments = append(script.Arguments, "-s", "/bin/"+b.Shell)
		}
		script.Arguments = append(
			script.Arguments,
			info.User,
			"-c", shellCommand,
		)
	} else {
		script.Command = b.Shell
		if info.Type == common.LoginShell {
			script.Arguments = append(script.Arguments, "--login")
		}
	}

	return script, nil
}

func (b *BashShell) GenerateScript(buildStage common.BuildStage, info common.ShellScriptInfo) (string, error) {
	w := &BashWriter{
		TemporaryPath:  info.Build.TmpProjectDir(),
		Shell:          b.Shell,
		checkForErrors: info.Build.IsFeatureFlagOn(featureflags.EnableBashExitCodeCheck),
		useNewEval:     info.Build.IsFeatureFlagOn(featureflags.UseNewEvalStrategy),
	}

	return b.generateScript(w, buildStage, info)
}

func (b *BashShell) generateScript(
	w ShellWriter,
	buildStage common.BuildStage,
	info common.ShellScriptInfo,
) (string, error) {
	b.ensurePrepareStageHostnameMessage(w, buildStage, info)
	err := b.writeScript(w, buildStage, info)
	script := w.Finish(info.Build.IsDebugTraceEnabled())
	return script, err
}

func (b *BashShell) ensurePrepareStageHostnameMessage(
	w ShellWriter,
	buildStage common.BuildStage,
	info common.ShellScriptInfo,
) {
	if buildStage == common.BuildStagePrepare {
		if info.Build.Hostname != "" {
			w.Line("echo " + strconv.Quote("Running on $(hostname) via "+info.Build.Hostname+"..."))
		} else {
			w.Line("echo " + strconv.Quote("Running on $(hostname)..."))
		}
	}
}

func (b *BashShell) IsDefault() bool {
	return runtime.GOOS != OSWindows && b.Shell == "bash"
}

func init() {
	common.RegisterShell(&BashShell{Shell: "sh"})
	common.RegisterShell(&BashShell{Shell: "bash"})
}
