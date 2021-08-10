// +build !integration

package shells

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestPwshTrapScriptGeneration(t *testing.T) {
	shellInfo := common.ShellScriptInfo{
		Shell:         SNPwsh,
		Type:          common.NormalShell,
		RunnerCommand: "/usr/bin/gitlab-runner-helper",
		Build: &common.Build{
			Runner: &common.RunnerConfig{},
		},
	}
	shellInfo.Build.Runner.Executor = "kubernetes"
	shellInfo.Build.Hostname = "Test Hostname"

	pwshTrap := &PwshTrapShell{
		PowerShell: common.GetShell(SNPwsh).(*PowerShell),
		LogFile:    "/path/to/logfile",
	}

	tests := map[string]struct {
		stage                common.BuildStage
		info                 common.ShellScriptInfo
		expectedError        error
		assertExpectedScript func(*testing.T, string)
	}{
		"prepare script": {
			stage: common.BuildStagePrepare,
			info:  shellInfo,
			assertExpectedScript: func(t *testing.T, s string) {
				assert.Contains(t, s, "#!/usr/bin/env pwsh")
				assert.Contains(t, s, strings.ReplaceAll(pwshTrapShellScript, "\n", pwshTrap.EOL))
				assert.Contains(t, s, `echo "Running on $([Environment]::MachineName) via "Test Hostname"..."`)
				assert.Contains(t, s, `trap {runner_script_trap} runner_script_trap`)
				assert.Contains(t, s, `exit 0`)
			},
		},
		"cleanup variables": {
			stage: common.BuildStageCleanupFileVariables,
			info:  shellInfo,
			assertExpectedScript: func(t *testing.T, s string) {
				assert.Empty(t, s)
			},
		},
		"no script": {
			stage:         "no_script",
			info:          shellInfo,
			expectedError: common.ErrSkipBuildStage,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			script, err := pwshTrap.GenerateScript(tc.stage, tc.info)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			tc.assertExpectedScript(t, script)
		})
	}
}
