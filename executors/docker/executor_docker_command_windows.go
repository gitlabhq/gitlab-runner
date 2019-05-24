package docker

import (
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

func init() {
	options := executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: true,
		DefaultBuildsDir:              `C:\builds`,
		DefaultCacheDir:               `C:\cache`,
		SharedBuildsDir:               false,
		Shell: common.ShellScriptInfo{
			Shell:         "powershell",
			Type:          common.NormalShell,
			RunnerCommand: "gitlab-runner-helper",
		},
		ShowHostname: true,
		Metadata: map[string]string{
			"OSType": osTypeWindows,
		},
	}

	creator := func() common.Executor {
		e := &commandExecutor{
			executor: executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: options,
				},
			},
		}
		e.SetCurrentStage(common.ExecutorStageCreated)
		return e
	}

	featuresUpdater := func(features *common.FeaturesInfo) {
		features.Variables = true
		features.Image = true
		features.Services = true
		features.Session = false
		features.Terminal = false
	}

	common.RegisterExecutor("docker-windows", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}
