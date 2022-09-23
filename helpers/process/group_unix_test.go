//nolint:lll
//go:build !integration && (aix || android || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris)

package process

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetProcessGroup(t *testing.T) {
	for _, pg := range []bool{true, false} {
		t.Run(fmt.Sprintf("process_%t", pg), func(t *testing.T) {
			cmd := exec.Command("sleep", "1")
			require.Nil(t, cmd.SysProcAttr)
			setProcessGroup(cmd, pg)
			assert.True(t, cmd.SysProcAttr.Setpgid)
		})
	}
}
