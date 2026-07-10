package wait

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

type GracefulExitFunc func(ctx context.Context, containerID string) error

type Waiter interface {
	Wait(ctx context.Context, containerID string) error
}

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
// until the specified container has stopped. If gracefulExitFunc is non-nil it
// is run first as a best-effort, time-bounded step. The stop itself runs to
// completion regardless of ctx cancellation; ctx is used only for its values.
//
// Timeout is the duration (in seconds) to wait for the container to stop
// gracefully before forcibly terminating it with SIGKILL. A nil timeout uses
// the daemon's or container's default timeout, -1 will wait indefinitely.
// Use 0 to not wait at all.
func (d *dockerWaiter) StopKillWait(ctx context.Context, containerID string, timeout *int,
	gracefulExitFunc GracefulExitFunc,
) error {
	// Detach cancellation so the stop still runs when the job is canceled,
	// otherwise ContainerStop is skipped and PID 1 is SIGKILLed only after the
	// network is torn down. Left unbounded so a slow stop can't fail a healthy
	// job; a stalled stop is reaped by executor.Cleanup.
	stopCtx := context.WithoutCancel(ctx)

	if gracefulExitFunc != nil {
		// Best-effort SIGTERM to the container's procs before ContainerStop,
		// bounded so a stuck exec can't block teardown. The container's own
		// SIGTERM -> SIGKILL grace is ContainerStop's timeout, not this bound.
		gracefulCtx, cancel := context.WithTimeout(stopCtx, 10*time.Second)
		defer cancel()
		_ = gracefulExitFunc(gracefulCtx, containerID)
	}

	return d.retryWait(stopCtx, containerID, func() {
		_ = d.client.ContainerStop(stopCtx, containerID, client.ContainerStopOptions{Timeout: timeout})
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
				exitCode := common.NormalizeExitCode(int(status.StatusCode))
				return &common.BuildError{
					Inner:         fmt.Errorf("exit code %d", exitCode),
					ExitCode:      exitCode,
					FailureReason: common.ScriptFailure,
				}
			}

			return nil
		}
	}
}
