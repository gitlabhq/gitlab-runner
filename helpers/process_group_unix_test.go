package helpers

import (
	"bytes"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func findChild(ppid int) int {
	lines, _ := exec.Command("ps", "ax", "-o", "%P %p").CombinedOutput()

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
	command := "su"
	arguments := []string{"-s", "/bin/bash", "test-user", "-c", "bash --login"}

	cmd := exec.Command(command, arguments...)
	SetProcessGroup(cmd)

	cmd.Stdin = bytes.NewBufferString(script)
	cmd.Start()

	time.Sleep(time.Second * 2)

	return cmd
}

func testKillProcessGroup(t *testing.T, script string) {
	if SkipIntegrationTests(t, "su") {
		return
	}

	_, userLookupError := user.Lookup("test-user")
	if userLookupError != nil {
		t.Skip("User 'test-user' must exist for this test to be executed")
		return
	}

	cmd := createTestProcess(script)

	cmdPid := cmd.Process.Pid
	childPid := findChild(cmdPid)

	assert.Nil(t, checkProcess(cmdPid))
	assert.Nil(t, checkProcess(childPid))

	KillProcessGroup(cmd)

	cmdProcessCheck := checkProcess(cmdPid)
	childProcessCheck := checkProcess(childPid)

	assert.NotNil(t, cmdProcessCheck, "Process check should return errorFinished error")
	if cmdProcessCheck != nil {
		assert.Equal(t, "os: process already finished", cmdProcessCheck.Error())
	}

	assert.NotNil(t, childProcessCheck, "Process check should return errorFinished error")
	if childProcessCheck != nil {
		assert.Equal(t, "os: process already finished", childProcessCheck.Error())
	}
}

var simpleScript = "sleep 300"
var nonTermableScript = `
trap "sleep 350" SIGTERM
sleep 300
`

func TestKillProcessGroupForSimpleScript(t *testing.T) {
	testKillProcessGroup(t, simpleScript)
}

func TestKillProcessGroupForNonTermableScript(t *testing.T) {
	testKillProcessGroup(t, nonTermableScript)
}
