package docker

import (
	"fmt"

	"github.com/docker/docker/api/types"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// This is that path to which the step-runner binary will be copied in the build container. This path MUST be added
// to the container's PATH.
const stepRunnerBinaryPath = "/opt/step-runner"
