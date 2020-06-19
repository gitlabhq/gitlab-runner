// +build darwin dragonfly freebsd linux netbsd openbsd

package process

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Cases for UNIX systems that are used in `killer_test.go#TestKiller`.
func testKillerTestCases() map[string]testKillerTestCase {
	return map[string]testKillerTestCase{
		"command terminated": {
			alreadyStopped: false,
			skipTerminate:  true,
			expectedError:  "",
		},
		"command not terminated": {
			alreadyStopped: false,
			skipTerminate:  false,
			expectedError:  "exit status 1",
		},
		"command already stopped": {
			alreadyStopped: true,
			expectedError:  "signal: killed",
		},
	}
}

func Test_unixKiller_getPID(t *testing.T) {
	mCmd := new(MockCommander)
	defer mCmd.AssertExpectations(t)
	mLogger := new(MockLogger)
	defer mLogger.AssertExpectations(t)

	killer := unixKiller{logger: mLogger, cmd: mCmd}

	mCmd.On("Process").Return(&os.Process{Pid: 1}).Once()

	pid := killer.getPID()
	assert.Equal(t, -1, pid)
}
