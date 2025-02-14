package wait

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

type GracefulExitFunc func(ctx context.Context, containerID string) error

//go:generate mockery --name=Waiter --inpackage
type Waiter interface {
	Wait(ctx context.Context, containerID string) error
}

//go:generate mockery --name=KillWaiter --inpackage
type KillWaiter interface {
	Waiter

	StopKillWait(ctx context.Context, containerID string, timeout *int, gracefulExitFunc GracefulExitFunc) error
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
// Timeout is the timeout (in seconds) to wait for the container to stop
// gracefully before forcibly terminating it with SIGKILL.
//
// A nil timeout uses the daemon's or containers default timeout, -1 will wait
// indefinitely. Use 0 to not wait at all.
func (d *dockerWaiter) StopKillWait(ctx context.Context, containerID string, timeout *int,
	graceGracefulExitFunc GracefulExitFunc,
) error {
	// if the job timed out or was cancelled, the ctx will already have expired, so just use context.Background()
	if graceGracefulExitFunc != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_ = graceGracefulExitFunc(ctx, containerID)
	}
	return d.retryWait(ctx, containerID, func() {
		_ = d.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: timeout})
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
			if err == nil || errdefs.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("waiting for container: %w", err)

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
