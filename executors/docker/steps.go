package docker

import (
	"fmt"
	"slices"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

// This is that path to which the step-runner binary will be copied in the build container. This path MUST be added
// to the container's PATH.
const stepRunnerBinaryPath = "/opt/step-runner"

type stepRunnerContainerConfigurator struct {
	e *executor
}

var _ containerConfigurator = &stepRunnerContainerConfigurator{}

func newStepRunnerContainerConfigurator(e *executor) *stepRunnerContainerConfigurator {
	return &stepRunnerContainerConfigurator{e: e}
}

func (c *stepRunnerContainerConfigurator) ContainerConfig(image *types.ImageInspect) (*container.Config, error) {
	return &container.Config{
		Image:        image.ID,
		Cmd:          stepRunnerBootstrapCommand,
		Tty:          false,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    true,
		StdinOnce:    true,
	}, nil
}

func (c *stepRunnerContainerConfigurator) HostConfig() (*container.HostConfig, error) {
	i := slices.IndexFunc(c.e.volumesManager.Binds(), func(bind string) bool {
		return strings.Contains(bind, stepRunnerBinaryPath)
	})
	if i == -1 {
		return nil, fmt.Errorf("failed to find volume bind with mount-point %q for %q",
			stepRunnerBinaryPath, c.e.Config.GetStepRunnerImage())
	}

	return &container.HostConfig{
		AutoRemove:     true,
		ReadonlyRootfs: true,
		RestartPolicy:  neverRestartPolicy,
		Binds:          []string{c.e.volumesManager.Binds()[i]},
	}, nil
}

func (c *stepRunnerContainerConfigurator) NetworkConfig(_ []string) *network.NetworkingConfig {
	return nil
}
