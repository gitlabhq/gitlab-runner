package exec

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/wait"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/step-runner/pkg/api/client"
	"gitlab.com/gitlab-org/step-runner/pkg/api/client/extended"
)

type stepsDocker struct {
	ctx     context.Context
	client  docker.Client
	waiter  wait.KillWaiter
	logger  logrus.FieldLogger
	request *client.RunRequest
}

func NewStepsDocker(ctx context.Context, client docker.Client, waiter wait.KillWaiter, logger logrus.FieldLogger, request *client.RunRequest) Docker {
	return &stepsDocker{
		ctx:     ctx,
		client:  client,
		waiter:  waiter,
		logger:  logger,
		request: request,
	}
}

func (sd *stepsDocker) Exec(ctx context.Context, containerID string, streams IOStreams, _ wait.GracefulExitFunc) error {
	sd.logger.Debugln("Executing steps on container", containerID, "...")

	sd.logger.Debugln("Starting container", containerID, "...")
	err := sd.client.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		return fmt.Errorf("starting container %q: %w", containerID, err)
	}

	execErr := sd.executeStepsRequest(ctx, containerID, streams)
	if execErr != nil {
		execErr = fmt.Errorf("container exec on %q finished with: %w", containerID, execErr)
	}

	killErr := sd.waiter.StopKillWait(sd.ctx, containerID, nil, nil)
	if killErr != nil {
		killErr = fmt.Errorf("container kill on %q finished with: %w", containerID, killErr)
	}

	return errors.Join(execErr, killErr)
}

func (sd *stepsDocker) executeStepsRequest(ctx context.Context, container string, streams IOStreams) error {
	dialer := tunnelingDialer{containerID: container, client: sd.client, logger: sd.logger}

	srClient, err := extended.New(&dialer)
	if err != nil {
		return fmt.Errorf("creating steps client: %w", err)
	}
	//nolint:errcheck
	defer srClient.CloseConn()

	out := extended.FollowOutput{Logs: streams.Stdout}

	status, err := srClient.RunAndFollow(ctx, sd.request, &out)
	if err != nil {
		return fmt.Errorf("executing step request: %w", err)
	}

	return errFromStatus(status)
}

func errFromStatus(status client.Status) error {
	berr := &common.BuildError{Inner: errors.New(status.Message)}

	switch status.State {
	case client.StateSuccess:
		return nil
	case client.StateUnspecified:
		berr.FailureReason = common.UnknownFailure
	case client.StateFailure:
		berr.FailureReason = common.ScriptFailure
	case client.StateRunning:
		// this should not happen!!!
	case client.StateCancelled:
		// nothing to do here since there is no common.CancelledFailure
	}

	// TODO: also set berr.ExitCode if we add an exit-code to client.Status

	return berr
}
