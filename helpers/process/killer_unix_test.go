//nolint:lll
//go:build !integration && (aix || android || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris)

package process

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
