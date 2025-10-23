package docker

import (
	"fmt"
	"slices"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// This is that path to which the step-runner binary will be copied in the build container. This path MUST be added
// to the container's PATH. When copying the step-runner binary from the step-runner container to the shared volume,
// this path can be anything as long the mount-point and destination argument to the bootstrap command are the same.
const stepRunnerBinaryPath = "/opt/step-runner"

var stepRunnerBootstrapCommand = []string{"/step-runner", "bootstrap", stepRunnerBinaryPath}

func (e *commandExecutor) requestStepRunnerContainer() (*container.InspectResponse, error) {
	return e.createContainer(
		stepRunnerContainerType,
		common.Image{Name: e.Config.GetStepRunnerImage()},
		nil,
		newStepRunnerContainerConfigurator(&e.executor),
	)
}

type stepRunnerContainerConfigurator struct {
	e *executor
}

var _ containerConfigurator = &stepRunnerContainerConfigurator{}

func newStepRunnerContainerConfigurator(e *executor) *stepRunnerContainerConfigurator {
	return &stepRunnerContainerConfigurator{e: e}
}

func (c *stepRunnerContainerConfigurator) ContainerConfig(image *image.InspectResponse) (*container.Config, error) {
	return &container.Config{
		Image:           image.ID,
		Cmd:             stepRunnerBootstrapCommand,
		Tty:             false,
		AttachStdin:     false,
		AttachStdout:    true,
		AttachStderr:    true,
		OpenStdin:       false,
		StdinOnce:       true,
		NetworkDisabled: true,
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
		NetworkMode:    network.NetworkNone,
		Runtime:        c.e.Config.Docker.Runtime,
		Isolation:      container.Isolation(c.e.Config.Docker.Isolation),
	}, nil
}

func (c *stepRunnerContainerConfigurator) NetworkConfig(_ []string) *network.NetworkingConfig {
	return nil
}
