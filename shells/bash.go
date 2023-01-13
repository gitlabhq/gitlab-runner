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
	usePosixEscape     bool
	useJSONTermination bool
}

func NewBashWriter(build *common.Build, shell string) *BashWriter {
	return &BashWriter{
		TemporaryPath:  build.TmpProjectDir(),
		Shell:          shell,
		checkForErrors: build.IsFeatureFlagOn(featureflags.EnableBashExitCodeCheck),
		useNewEval:     build.IsFeatureFlagOn(featureflags.UseNewEvalStrategy),
		useNewEscape:   build.IsFeatureFlagOn(featureflags.UseNewShellEscape),
		usePosixEscape: build.IsFeatureFlagOn(featureflags.PosixlyCorrectEscapes),
		// useJSONTermination is only used for kubernetes executor when
		// the feature flag FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY is set to false
		useJSONTermination: build.Runner.Executor == common.ExecutorKubernetes &&
			!build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy),
	}
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

	b.Line("_runner_exit_code=$?; if [ $_runner_exit_code -ne 0 ]; then exit $_runner_exit_code; fi")
}

func (b *BashWriter) Indent() {
	b.indent++
}

func (b *BashWriter) Unindent() {
	b.indent--
}

func (b *BashWriter) Command(command string, arguments ...string) {
	b.Line(b.buildCommand(b.escapeNoLegacy, command, arguments...))
	b.CheckForErrors()
}

func (b *BashWriter) CommandArgExpand(command string, arguments ...string) {
	b.Line(b.buildCommand(doubleQuote, command, arguments...))
	b.CheckForErrors()
}

func (b *BashWriter) buildCommand(quoter stringQuoter, command string, arguments ...string) string {
	list := []string{
		b.escape(command),
	}

	for _, argument := range arguments {
		list = append(list, quoter(argument))
	}

	return strings.Join(list, " ")
}

func (b *BashWriter) TmpFile(name string) string {
	return b.cleanPath(path.Join(b.TemporaryPath, name))
}

func (b *BashWriter) cleanPath(name string) string {
	return b.Absolute(name)
}

func (b *BashWriter) EnvVariableKey(name string) string {
	return fmt.Sprintf("$%s", name)
}

// Intended to be used on unmodified paths only (i.e. paths that have not been
// cleaned with cleanPath()).
func (b *BashWriter) isTmpFile(path string) bool {
	return strings.HasPrefix(path, b.TemporaryPath)
}

func (b *BashWriter) Variable(variable common.JobVariable) {
	if variable.File {
		variableFile := b.TmpFile(variable.Key)
		b.Linef("mkdir -p %q", helpers.ToSlash(b.TemporaryPath))
		b.Linef("printf '%%s' %s > %q", b.escape(variable.Value), variableFile)
		b.Linef("export %s=%q", b.escape(variable.Key), variableFile)
	} else {
		if b.isTmpFile(variable.Value) {
			variable.Value = b.cleanPath(variable.Value)
		}
		b.Linef("export %s=%s", b.escape(variable.Key), b.escape(variable.Value))
	}
}

func (b *BashWriter) IfDirectory(path string) {
	b.Linef("if [ -d %q ]; then", path)
	b.Indent()
}

func (b *BashWriter) IfFile(path string) {
	b.Linef("if [ -e %q ]; then", path)
	b.Indent()
}

func (b *BashWriter) IfCmd(cmd string, arguments ...string) {
	cmdline := b.buildCommand(b.escapeNoLegacy, cmd, arguments...)
	b.Linef("if %s >/dev/null 2>&1; then", cmdline)
	b.Indent()
}

func (b *BashWriter) IfCmdWithOutput(cmd string, arguments ...string) {
	cmdline := b.buildCommand(b.escapeNoLegacy, cmd, arguments...)
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
	if path.IsAbs(dir) || strings.HasPrefix(dir, "$PWD") {
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
	b.Line("printf '%b\\n' " +
		helpers.ANSI_CLEAR +
		"section_start:$(date +%s):section_" + id +
		"\r" + helpers.ANSI_CLEAR + b.escape(helpers.ANSI_BOLD_GREEN+command+helpers.ANSI_RESET))
}

func (b *BashWriter) SectionEnd(id string) {
	b.Line("printf '%b\\n' " +
		helpers.ANSI_CLEAR +
		"section_end:$(date +%s):section_" + id +
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

	buf.WriteString("if set -o | grep pipefail > /dev/null; then set -o pipefail; fi; set -o errexit\n")
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
	if b.usePosixEscape {
		return helpers.PosixShellEscape(input)
	}

	if b.useNewEscape {
		return helpers.ShellEscape(input)
	}

	return helpers.ShellEscapeLegacy(input)
}

func (b *BashWriter) escapeNoLegacy(input string) string {
	// We don't want to use the `escape` function since the legacy
	// escape will not always work for us, opening us for vulnerabilities
	if b.usePosixEscape {
		return helpers.PosixShellEscape(input)
	}

	return helpers.ShellEscape(input)
}

func (b *BashShell) GetName() string {
	return b.Shell
}

func (b *BashShell) GetConfiguration(info common.ShellScriptInfo) (*common.ShellConfiguration, error) {
	script := &common.ShellConfiguration{
		Command: b.Shell,
		CmdLine: b.Shell,
	}

	if info.Type == common.LoginShell {
		script.CmdLine += " -l"
		script.Arguments = []string{"-l"}
		script.DockerCommand = []string{"sh", "-c", strings.ReplaceAll(BashDetectShellScript, "$@", "-l")}
	} else {
		script.DockerCommand = []string{"sh", "-c", strings.ReplaceAll(BashDetectShellScript, "$@", "")}
	}

	if info.User == "" {
		return script, nil
	}

	script.Command = "su"
	if runtime.GOOS == OSLinux {
		script.Arguments = []string{"-s", "/bin/" + b.Shell, info.User, "-c", script.CmdLine}
	} else {
		script.Arguments = []string{info.User, "-c", script.CmdLine}
	}

	script.CmdLine = script.Command
	for _, arg := range script.Arguments {
		script.CmdLine += " " + helpers.ShellEscape(arg)
	}

	return script, nil
}

func (b *BashShell) GenerateScript(buildStage common.BuildStage, info common.ShellScriptInfo) (string, error) {
	w := NewBashWriter(info.Build, b.Shell)
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

func (b *BashShell) GenerateSaveScript(info common.ShellScriptInfo, scriptPath, script string) (string, error) {
	w := NewBashWriter(info.Build, b.Shell)
	return b.generateSaveScript(w, scriptPath, script)
}

func (b *BashShell) generateSaveScript(w *BashWriter, scriptPath, script string) (string, error) {
	w.Line(fmt.Sprintf("touch %s", scriptPath))
	w.Line(fmt.Sprintf("chmod 777 %s", scriptPath))
	w.Line(fmt.Sprintf("echo %s > %s", w.escape(script), scriptPath))

	return w.String(), nil
}

func (b *BashShell) IsDefault() bool {
	return runtime.GOOS != OSWindows && b.Shell == "bash"
}

func init() {
	common.RegisterShell(&BashShell{Shell: "sh"})
	common.RegisterShell(&BashShell{Shell: "bash"})
}
