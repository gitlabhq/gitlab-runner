package wait

import (
	"context"
	"fmt"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

type Waiter interface {
	Wait(ctx context.Context, containerID string) error
}

type dockerWaiter struct {
	client docker.Client
}

func NewDockerWaiter(c docker.Client) Waiter {
	return &dockerWaiter{
		client: c,
	}
}

func (d *dockerWaiter) Wait(ctx context.Context, containerID string) error {
	retries := 0

	// Use active wait
	for ctx.Err() == nil {
		container, err := d.client.ContainerInspect(ctx, containerID)
		if err != nil {
			if docker.IsErrNotFound(err) {
				return err
			}

			if retries > 3 {
				return err
			}

			retries++
			time.Sleep(time.Second)
			continue
		}

		// Reset retry timer
		retries = 0

		if container.State.Running {
			time.Sleep(time.Second)
			continue
		}

		if container.State.ExitCode != 0 {
			return &common.BuildError{
				Inner: fmt.Errorf("exit code %d", container.State.ExitCode),
			}
		}

		return nil
	}

	return ctx.Err()
}
