package shells

import (
	"bytes"
	"fmt"
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

// bashJSONTerminationScript prints a json log-line to provide exit code context to
// executors that cannot directly retrieve the exit status of the script.
const bashJSONTerminationScript = `runner_script_trap() {
	exit_code=$?
	out_json="{\"command_exit_code\": $exit_code, \"script\": \"$0\"}"

	echo ""
	echo "$out_json"
	exit 0
}

trap runner_script_trap EXIT
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

	checkForErrors     bool
	useNewEval         bool
	useNewEscape       bool
	useJSONTermination bool
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
		b.escape(command),
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
		b.Linef("echo -n %s > %q", b.escape(variable.Value), variableFile)
		b.Linef("export %s=%q", b.escape(variable.Key), variableFile)
	} else {
		b.Linef("export %s=%s", b.escape(variable.Key), b.escape(variable.Value))
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

func (b *BashWriter) RmFilesRecursive(path string, name string) {
	b.IfDirectory(path)
	// `find -delete` is not portable; https://unix.stackexchange.com/a/194348
	b.Linef("find %q -name %q -exec rm {} +", path, name)
	b.EndIf()
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
	b.Line("echo " + b.escape(coloredText))
}

func (b *BashWriter) Noticef(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_BOLD_GREEN + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + b.escape(coloredText))
}

func (b *BashWriter) Warningf(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_YELLOW + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + b.escape(coloredText))
}

func (b *BashWriter) Errorf(format string, arguments ...interface{}) {
	coloredText := helpers.ANSI_BOLD_RED + fmt.Sprintf(format, arguments...) + helpers.ANSI_RESET
	b.Line("echo " + b.escape(coloredText))
}

func (b *BashWriter) EmptyLine() {
	b.Line("echo")
}

func (b *BashWriter) SectionStart(id, command string) {
	b.Line("echo -e " +
		helpers.ANSI_CLEAR +
		"section_start:`date +%s`:section_" + id +
		"\r" + helpers.ANSI_CLEAR + helpers.ShellEscape(helpers.ANSI_BOLD_GREEN+command+helpers.ANSI_RESET))
}

func (b *BashWriter) SectionEnd(id string) {
	b.Line("echo -e " +
		helpers.ANSI_CLEAR +
		"section_end:`date +%s`:section_" + id +
		"\r" + helpers.ANSI_CLEAR)
}

func (b *BashWriter) Finish(trace bool) string {
	var buf strings.Builder

	if b.Shell != "" {
		buf.WriteString("#!/usr/bin/env " + b.Shell + "\n\n")
	}

	if b.useJSONTermination {
		buf.WriteString(bashJSONTerminationScript)
	}

	if trace {
		buf.WriteString("set -o xtrace\n")
	}

	buf.WriteString("set -eo pipefail\n")
	buf.WriteString("set +o noclobber\n")

	if b.useNewEval {
		buf.WriteString(": | (eval " + b.escape(b.String()) + ")\n")
	} else {
		buf.WriteString(": | eval " + b.escape(b.String()) + "\n")
	}

	buf.WriteString("exit 0\n")

	return buf.String()
}

func (b *BashWriter) escape(input string) string {
	if b.useNewEscape {
		return helpers.ShellEscape(input)
	}

	return helpers.ShellEscapeLegacy(input)
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
		useNewEscape:   info.Build.IsFeatureFlagOn(featureflags.UseNewShellEscape),
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
