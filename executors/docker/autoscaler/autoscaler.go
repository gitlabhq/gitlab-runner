package autoscaler

import (
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/internal/autoscaler"
)

func NewProvider(dockerProvider common.ExecutorProvider) common.ExecutorProvider {
	return autoscaler.New(
		dockerProvider,
		autoscaler.Config{MapJobImageToVMImage: false},
	)
}
