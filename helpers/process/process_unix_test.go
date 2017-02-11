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

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
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

	build := &common.Build{}
	build.ID = 1
	build.RepoURL = "http://gitlab.example.com/example/project.git"

	startedCh := make(chan struct{})

	PrepareProcessGroup(cmd, &common.ShellConfiguration{}, build, startedCh)

	cmd.Stdin = bytes.NewBufferString(script)
	cmd.Start()
	startedCh <- struct{}{}
	close(startedCh)

	return cmd
}

func testKillProcessGroup(t *testing.T, script string) {
	if SkipIntegrationTests(t, "bash") {
		return
	}

	logrus.SetLevel(logrus.DebugLevel)

	cmd := createTestProcess(script)
	time.Sleep(10 * time.Millisecond)

	cmdPid := cmd.Process.Pid
	childPid := findChild(cmdPid)

	assert.NoError(t, checkProcess(cmdPid))
	assert.NoError(t, checkProcess(childPid))

	KillProcessGroup(cmd)
	time.Sleep(10 * time.Millisecond)

	assert.EqualError(t, checkProcess(cmdPid), "os: process already finished", "Process check should return errorFinished error")
	assert.EqualError(t, checkProcess(childPid), "os: process already finished", "Process check should return errorFinished error")
}

var simpleScript = ": | eval $'sleep 60'"
var nonTerminatableScript = ": | eval $'trap \"sleep 70\" SIGTERM\nsleep 60'"

func TestKillProcessGroupForSimpleScript(t *testing.T) {
	ProcessKillWaitTime = 2 * time.Second
	testKillProcessGroup(t, simpleScript)
}

func TestKillProcessGroupForNonTerminatableScript(t *testing.T) {
	ProcessKillWaitTime = 2 * time.Second
	testKillProcessGroup(t, nonTerminatableScript)
}
