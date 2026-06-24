package wait

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

// ErrStopKillWaitTimeout indicates the bounded cleanup window elapsed
// before the container reached a stopped state.
var ErrStopKillWaitTimeout = errors.New("container stop-kill wait timed out")

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
// until the specified container has stopped.
//
// Timeout is the timeout (in seconds) to wait for the container to stop
// gracefully before forcibly terminating it with SIGKILL.
//
// A nil timeout uses the daemon's or container's default timeout, -1 will wait
// indefinitely. Use 0 to not wait at all.
func (d *dockerWaiter) StopKillWait(ctx context.Context, containerID string, timeout *int,
	gracefulExitFunc GracefulExitFunc,
) error {
	// If the job timed out or was cancelled, ctx will already have expired.
	// Both the graceful-exit func and ContainerStop must use a fresh context
	// derived from context.Background() so they are not skipped when the build
	// context is cancelled before StopKillWait is called.
	//
	// Without this, retryWait's `ctx.Err() == nil` loop condition exits
	// immediately, ContainerStop is never issued, and PID 1 never receives
	// SIGTERM — it only gets SIGKILL after the network is already disconnected.
	//
	// A single context covers both the graceful-exit func and the
	// ContainerStop/retryWait loop. Docker's own SIGTERM→SIGKILL grace period
	// is controlled by the `timeout` parameter passed to ContainerStop.
	stopKillTimeout := 2 * time.Minute
	stopCause := fmt.Errorf("%w after %s", ErrStopKillWaitTimeout, stopKillTimeout)
	stopCtx, stopCancel := context.WithTimeoutCause(context.Background(), stopKillTimeout, stopCause)
	defer stopCancel()

	if gracefulExitFunc != nil {
		_ = gracefulExitFunc(stopCtx, containerID)
	}

	err := d.retryWait(stopCtx, containerID, func() {
		_ = d.client.ContainerStop(stopCtx, containerID, container.StopOptions{Timeout: timeout})
	})

	// wrap as timeout only if the stop-kill timeout fired and the error isn't a script exit
	if cause := context.Cause(stopCtx); err != nil && errors.Is(cause, ErrStopKillWaitTimeout) {
		if _, ok := errors.AsType[*common.BuildError](err); !ok {
			return &common.BuildError{
				Inner:         cause,
				FailureReason: common.RunnerSystemFailure,
			}
		}
	}

	return err
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
