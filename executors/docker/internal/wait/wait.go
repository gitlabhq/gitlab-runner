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

type KillWaiter interface {
	Waiter

	StopKillWait(ctx context.Context, containerID string, timeout *time.Duration) error
}

type dockerWaiter struct {
	client docker.Client
}

func NewDockerKillWaiter(c docker.Client) KillWaiter {
	return &dockerWaiter{
		client: c,
	}
}

// Wait blocks until the container specified has stopped.
func (d *dockerWaiter) Wait(ctx context.Context, containerID string) error {
	return d.retryWait(ctx, containerID, nil)
}

// StopKillWait blocks (periodically attempting to stop and kill the container)
// until the specified container has stopped.
//
// A nil timeout uses the container or daemon's default value. A negative timeout
// immediately kills the container.
func (d *dockerWaiter) StopKillWait(ctx context.Context, containerID string, timeout *time.Duration) error {
	return d.retryWait(ctx, containerID, func() {
		_ = d.client.ContainerStop(ctx, containerID, timeout)
	})
}

func (d *dockerWaiter) retryWait(ctx context.Context, containerID string, stopFn func()) error {
	retries := 0

	for ctx.Err() == nil {
		err := d.wait(ctx, containerID, stopFn)
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
// The passed `stopFn` function is periodically called (to ensure that the
// daemon absolutely receives the request) and is used to stop the container.
func (d *dockerWaiter) wait(ctx context.Context, containerID string, stopFn func()) error {
	statusCh, errCh := d.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	if stopFn != nil {
		stopFn()
	}

	for {
		select {
		case <-time.After(time.Second):
			if stopFn != nil {
				stopFn()
			}

		case err := <-errCh:
			return err

		case status := <-statusCh:
			if status.StatusCode != 0 {
				return &common.BuildError{
					Inner:    fmt.Errorf("exit code %d", status.StatusCode),
					ExitCode: int(status.StatusCode),
				}
			}

			return nil
		}
	}
}
