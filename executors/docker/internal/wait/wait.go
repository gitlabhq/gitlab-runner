package wait

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

type Waiter interface {
	Wait(ctx context.Context, containerID string) error
}

type Killer interface {
	KillWait(ctx context.Context, containerID string) error
}

type KillWaiter interface {
	Waiter
	Killer
}

type dockerWaiter struct {
	client docker.Client
}

func NewDockerWaiter(c docker.Client) KillWaiter {
	return &dockerWaiter{
		client: c,
	}
}

// Wait blocks until the container specified has stopped.
func (d *dockerWaiter) Wait(ctx context.Context, containerID string) error {
	return d.retryWait(ctx, containerID, nil)
}

// KillWait blocks (periodically attempting to kill the container) until the
// specified container has stopped.
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

		var e *common.BuildError
		if errors.As(err, &e) || docker.IsErrNotFound(err) || retries > 3 {
			return err
		}
		retries++

		time.Sleep(time.Second)
	}

	return ctx.Err()
}

// wait waits until the container has stopped.
//
// The function passed to stopFn() is periodically called (to ensure that the
// daemon absolutely receives the request) and is used to stop the container.
func (d *dockerWaiter) wait(ctx context.Context, containerID string, stopFn func()) error {
	statusCh, errCh := d.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	if stopFn != nil {
		stopFn()
	}

	for {
		select {
		case <-ticker.C:
			if stopFn != nil {
				stopFn()
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
