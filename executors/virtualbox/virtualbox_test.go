//go:build !integration

package virtualbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/vm"
)

func TestValidateConfig(t *testing.T) {
	validVirtualBox := &common.VirtualBoxConfig{BaseName: "base"}
	validSSH := &common.SshConfig{}

	tests := map[string]struct {
		virtualbox  *common.VirtualBoxConfig
		ssh         *common.SshConfig
		passFile    bool
		expectedErr string
	}{
		"missing VirtualBox configuration": {
			virtualbox:  nil,
			ssh:         validSSH,
			expectedErr: "missing VirtualBox configuration",
		},
		"missing BaseName": {
			virtualbox:  &common.VirtualBoxConfig{},
			ssh:         validSSH,
			expectedErr: "missing BaseName setting from VirtualBox configuration",
		},
		"shell requires script file": {
			virtualbox:  validVirtualBox,
			ssh:         validSSH,
			passFile:    true,
			expectedErr: "virtualbox doesn't support shells that require script file",
		},
		"missing SSH config": {
			virtualbox:  validVirtualBox,
			ssh:         nil,
			expectedErr: "missing SSH config",
		},
		"valid configuration": {
			virtualbox: validVirtualBox,
			ssh:        validSSH,
			// expectedErr left empty to signal success
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := &executor{
				Executor: vm.Executor{
					AbstractExecutor: executors.AbstractExecutor{
						Config: common.RunnerConfig{
							RunnerSettings: common.RunnerSettings{
								VirtualBox: tt.virtualbox,
								SSH:        tt.ssh,
							},
						},
						BuildShell: &common.ShellConfiguration{PassFile: tt.passFile},
					},
				},
			}

			err := e.validateConfig()

			if tt.expectedErr == "" {
				assert.NoError(t, err)
				return
			}

			var buildErr *common.BuildError
			require.ErrorAs(t, err, &buildErr)
			assert.Equal(t, common.ConfigurationError, buildErr.FailureReason)
			assert.EqualError(t, err, tt.expectedErr)
		})
	}
}
