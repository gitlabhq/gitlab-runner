package shells

import (
	"bufio"
	"bytes"
	"io"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// BashTrapShellScript is used to wrap a shell script in a trap that makes sure the script always exits with exit code of 0
// this can be useful in container environments where exiting with an exit code different from 0 would kill the container.
// At the same time it prints to stdout the actual exit code of the script as well as the filename of the script as json.
const BashTrapShellScript = `runner_script_trap() {
	command_exit_code=$?
	out_json='{"command_exit_code": %s, "script": "%s"}\n'
	printf "$out_json" "$command_exit_code" "$0"
	
	exit 0
}

trap runner_script_trap EXIT

`

type BashTrapShellWriter struct {
	*BashWriter
}

func (b *BashTrapShellWriter) Finish(trace bool) string {
	var buffer bytes.Buffer
	w := bufio.NewWriter(&buffer)

	b.writeShebang(w)
	b.writeTrap(w)
	b.writeTrace(w, trace)
	b.writeScript(w)

	_ = w.Flush()
	return buffer.String()
}

func (b *BashTrapShellWriter) writeTrap(w io.Writer) {
	_, _ = io.WriteString(w, BashTrapShellScript)
}

type BashTrapShell struct {
	*BashShell
}

func (b *BashTrapShell) GenerateScript(buildStage common.BuildStage, info common.ShellScriptInfo) (string, error) {
	w := &BashTrapShellWriter{
		BashWriter: &BashWriter{
			TemporaryPath: info.Build.TmpProjectDir(),
			Shell:         b.Shell,
		},
	}

	return b.generateScript(w, buildStage, info)
}
