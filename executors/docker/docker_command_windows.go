package docker

import (
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/permission"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

func init() {
	options := executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: true,
		DefaultBuildsDir:              `c:\builds`,
		DefaultCacheDir:               `c:\cache`,
		SharedBuildsDir:               false,
		Shell: common.ShellScriptInfo{
			Shell:         shells.SNPowershell,
			Type:          common.NormalShell,
			RunnerCommand: "gitlab-runner-helper",
		},
		ShowHostname: true,
		Metadata: map[string]string{
			metadataOSType: osTypeWindows,
		},
	}

	creator := func() common.Executor {
		e := &commandExecutor{
			executor: executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: options,
				},
				volumeParser: parser.NewWindowsParser(),
			},
		}

		e.newVolumePermissionSetter = func() (permission.Setter, error) {
			return permission.NewDockerWindowsSetter(), nil
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

	common.RegisterExecutorProvider("docker-windows", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		ConfigUpdater:    configUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}
