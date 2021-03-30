package docker

import (
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func configUpdater(input *common.RunnerConfig, output *common.ConfigInfo) {
	if input.RunnerSettings.Docker != nil {
		output.GpuEnabled = strings.Trim(input.RunnerSettings.Docker.Gpus, " ") != ""
	}
}
