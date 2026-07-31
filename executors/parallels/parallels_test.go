//go:build !integration

package parallels

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/vm"
)

func TestValidateConfig(t *testing.T) {
	validParallels := &common.ParallelsConfig{BaseName: "base"}
	validSSH := &common.SshConfig{}

	tests := map[string]struct {
		parallels   *common.ParallelsConfig
		ssh         *common.SshConfig
		passFile    bool
		expectedErr string
	}{
		"missing Parallels configuration": {
			parallels:   nil,
			ssh:         validSSH,
			expectedErr: "missing Parallels configuration",
		},
		"missing BaseName": {
			parallels:   &common.ParallelsConfig{},
			ssh:         validSSH,
			expectedErr: "missing BaseName setting from Parallels configuration",
		},
		"shell requires script file": {
			parallels:   validParallels,
			ssh:         validSSH,
			passFile:    true,
			expectedErr: "parallels doesn't support shells that require script file",
		},
		"missing SSH configuration": {
			parallels:   validParallels,
			ssh:         nil,
			expectedErr: "missing SSH configuration",
		},
		"valid configuration": {
			parallels: validParallels,
			ssh:       validSSH,
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
								Parallels: tt.parallels,
								SSH:       tt.ssh,
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
