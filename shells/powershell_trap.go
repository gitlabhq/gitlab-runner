package shells

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

// pwshTrapShellScript is used to wrap a shell script in a trap that makes sure the script always exits
// with exit code of 0. This can be useful in container environments where exiting with an exit code different from 0
// would kill the container.
// At the same time it writes to a file the actual exit code of the script as well as the filename
// At the same time it writes the actual exit code of the script as well as
// the filename of the script (as json) to a file.
// With powershell $? returns True if the last command was successful so the exit_code is set to 0 in that case
const pwshTrapShellScript = `
function runner_script_trap() {
	$lastExit = $?
	$code = 1
	If($lastExit -eq "True"){ $code = 0 }

	$out_json= '{"command_exit_code": ' + $code + ', "script": "' + $MyInvocation.MyCommand.Name + '"}'

	echo ""
	echo "$out_json"
}

trap {runner_script_trap}

`

type PwshTrapShellWriter struct {
	*PsWriter

	logFile string
}

func (b *PwshTrapShellWriter) Finish(trace bool) string {
	var buffer bytes.Buffer
	w := bufio.NewWriter(&buffer)

	b.writeShebang(w)
	b.writeTrap(w)
	b.writeTrace(w, trace)
	b.writeScript(w)

	_ = w.Flush()
	return buffer.String()
}

func (b *PwshTrapShellWriter) writeTrap(w io.Writer) {
	// For code readability purpose, the pwshTrapShellScript is written with \n as EOL within the script
	// However when written into the generated script for a job, the \n used within the trap script is
	// replaced by the shell EOL to avoid having multiple EOL within it and to keep it consistent
	_, _ = fmt.Fprint(w, strings.ReplaceAll(pwshTrapShellScript, "\n", b.EOL))
}

type PwshTrapShell struct {
	*PowerShell

	LogFile string
}

func (b *PwshTrapShell) GenerateScript(buildStage common.BuildStage, info common.ShellScriptInfo) (string, error) {
	w := &PwshTrapShellWriter{
		PsWriter: &PsWriter{
			TemporaryPath: info.Build.TmpProjectDir(),
			Shell:         b.Shell,
			EOL:           b.EOL,
			resolvePaths:  info.Build.IsFeatureFlagOn(featureflags.UsePowershellPathResolver),
		},
		logFile: b.LogFile,
	}

	return b.generateScript(w, buildStage, info)
}
