package wait

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

type Waiter interface {
	Wait(ctx context.Context, containerID string) error
	KillWait(ctx context.Context, containerID string) error
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
	return d.retryWait(ctx, containerID, nil)
}

func (d *dockerWaiter) KillWait(ctx context.Context, containerID string) error {
	return d.retryWait(ctx, containerID, func() {
		d.client.ContainerKill(ctx, containerID, "SIGKILL")
	})
}

func (d *dockerWaiter) retryWait(ctx context.Context, containerID string, fn func()) error {
	retries := 0
	for ctx.Err() == nil {
		err := d.wait(ctx, containerID, fn)
		if err == nil {
			return nil
		}
		if _, ok := err.(*common.BuildError); ok {
			return err
		}
		if docker.IsErrNotFound(err) || retries > 3 {
			return err
		}

		retries++
		time.Sleep(time.Second)
	}
	return ctx.Err()
}

func (d *dockerWaiter) wait(ctx context.Context, containerID string, fn func()) error {
	statusCh, errCh := d.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	if fn != nil {
		fn()
	}
	for {
		select {
		case <-timer.C:
			if fn != nil {
				fn()
			}

		case err := <-errCh:
			return err

		case status := <-statusCh:
			if status.StatusCode != 0 {
				return &common.BuildError{
					Inner: fmt.Errorf("exit code %d", status.StatusCode),
				}
			}
			return nil
		}
	}
}
