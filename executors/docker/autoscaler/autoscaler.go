package autoscaler

import (
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/internal/autoscaler"

	_ "gitlab.com/gitlab-org/gitlab-runner/executors/docker" // need docker executor registered first
)

func init() {
	common.RegisterExecutorProvider(
		"docker-autoscaler",
		autoscaler.New(
			common.GetExecutorProvider("docker"),
			autoscaler.Config{MapJobImageToVMImage: false},
		),
	)
}
