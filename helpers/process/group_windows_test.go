//go:build !integration

package process

import (
	"os/exec"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetProcessGroup(t *testing.T) {
	tests := map[string]bool{
		"legacy process feature flag enabled":  true,
		"legacy process feature flag disabled": false,
	}

	for tn, featureEnabled := range tests {
		t.Run(tn, func(t *testing.T) {
			cmd := exec.Command("sleep", "1")

			require.Nil(t, cmd.SysProcAttr)
			setProcessGroup(cmd, featureEnabled)

			if featureEnabled {
				require.Nil(t, cmd.SysProcAttr)
			} else {
				assert.Equal(t, uint32(syscall.CREATE_NEW_PROCESS_GROUP), cmd.SysProcAttr.CreationFlags)
			}
		})
	}
}
