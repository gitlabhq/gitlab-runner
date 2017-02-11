package process

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	. "gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers"
)

func findChild(ppid int) int {
	lines, _ := exec.Command("ps", "axo", "ppid,pid").CombinedOutput()

	for _, line := range strings.Split(string(lines), "\n") {
		row := strings.Split(strings.TrimRight(line, "\n"), " ")

		var pids []int
		for _, cell := range row {
			if cell == "" {
				continue
			}

			pid, err := strconv.Atoi(cell)
			if err != nil {
				continue
			}

			pids = append(pids, pid)
		}

		if len(pids) > 0 {
			if pids[0] == ppid {
				return pids[1]
			}
		}

		if line == "" {
			break
		}
	}

	return 0
}

func checkProcess(pid int) (err error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return
	}

	return process.Signal(syscall.Signal(0))
}

func createTestProcess(script string) *exec.Cmd {
	command := "bash"
	arguments := []string{"--login"}

	cmd := exec.Command(command, arguments...)
	SetProcessGroup(cmd)

	cmd.Stdin = bytes.NewBufferString(script)
	cmd.Start()

	time.Sleep(time.Second * 2)

	return cmd
}

func testKillProcessGroup(t *testing.T, script string) {
	if SkipIntegrationTests(t, "bash") {
		return
	}

	cmd := createTestProcess(script)

	cmdPid := cmd.Process.Pid
	childPid := findChild(cmdPid)

	assert.NoError(t, checkProcess(cmdPid))
	assert.NoError(t, checkProcess(childPid))

	KillProcessGroup(cmd)

	cmdProcessCheck := checkProcess(cmdPid)
	childProcessCheck := checkProcess(childPid)

	assert.Error(t, cmdProcessCheck, "Process check should return errorFinished error")
	if cmdProcessCheck != nil {
		assert.Equal(t, "os: process already finished", cmdProcessCheck.Error())
	}

	assert.Error(t, childProcessCheck, "Process check should return errorFinished error")
	if childProcessCheck != nil {
		assert.Equal(t, "os: process already finished", childProcessCheck.Error())
	}
}

var simpleScript = "sleep 60"
var nonTerminatableScript = `
trap "sleep 70" SIGTERM
sleep 60
`

func TestKillProcessGroupForSimpleScript(t *testing.T) {
	testKillProcessGroup(t, simpleScript)
}

func TestKillProcessGroupForNonTerminatableScript(t *testing.T) {
	testKillProcessGroup(t, nonTerminatableScript)
}
