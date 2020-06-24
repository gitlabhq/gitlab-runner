package shells

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// BashTrapShellScript is used to wrap a shell script in a trap that makes sure the script always exits
// with exit code of 0 this can be useful in container environments where exiting with an exit code different from 0
// would kill the container.
// At the same time it writes to a file the actual exit code of the script as well as the filename
// of the script as json.
const bashTrapShellScript = `runner_script_trap() {
	exit_code=$?
	log_file=%s
	out_json="{\"command_exit_code\": $exit_code, \"script\": \"$0\"}"

	# Make sure the command status will always be printed on a new line 
	if [[ $(tail -c1 $log_file | wc -l) -gt 0 ]]; then
		printf "$out_json\n" >> $log_file
	else 
		printf "\n$out_json\n" >> $log_file
	fi
	
	exit 0
}

trap runner_script_trap EXIT

`

type BashTrapShellWriter struct {
	*BashWriter

	logFile string
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
	_, _ = fmt.Fprintf(w, bashTrapShellScript, b.logFile)
}

type BashTrapShell struct {
	*BashShell

	LogFile string
}

func (b *BashTrapShell) GenerateScript(buildStage common.BuildStage, info common.ShellScriptInfo) (string, error) {
	w := &BashTrapShellWriter{
		BashWriter: &BashWriter{
			TemporaryPath: info.Build.TmpProjectDir(),
			Shell:         b.Shell,
		},
		logFile: b.LogFile,
	}

	return b.generateScript(w, buildStage, info)
}
