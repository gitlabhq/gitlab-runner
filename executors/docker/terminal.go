package docker

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/docker/docker/api/types"
	"gitlab.com/gitlab-org/gitlab-terminal"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	terminalsession "gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

func (s *commandExecutor) Connect() (terminalsession.Conn, error) {
	// Waiting for the container to start,  is not ideal as it might be hiding a
	// real issue and the user is not aware of it. Ideally, the runner should
	// inform the user in an interactive way that the container has no started
	// yet and should wait/try again. This isn't an easy task to do since we
	// can't access the WebSocket here since that is the responsibility of
	// `gitlab-terminal` package. There are plans to improve this please take a
	// look at https://gitlab.com/gitlab-org/gitlab-ce/issues/50384#proposal and
	// https://gitlab.com/gitlab-org/gitlab-terminal/issues/4
	containerStarted := make(chan struct{})
	containerStartedErr := make(chan error)
	go func() {
		for {
			if s.buildContainer == nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			if s.buildContainer != nil {
				container, err := s.client.ContainerInspect(s.Context, s.buildContainer.ID)
				if err != nil {
					containerStartedErr <- err
					break
				}

				if container.State.Running {
					containerStarted <- struct{}{}
					break
				}

				continue
			}
		}
	}()

	ctx, cancel := context.WithTimeout(s.Context, waitForContainerTimeout)
	defer cancel()

	select {
	case <-containerStarted:
		return terminalConn{
			logger:      &s.BuildLogger,
			ctx:         s.Context,
			client:      s.client,
			containerID: s.buildContainer.ID,
			shell:       s.BuildShell.DockerCommand,
		}, nil
	case err := <-containerStartedErr:
		return nil, err
	case <-ctx.Done():
		s.Errorln("Timed out waiting for the container to start the terminal. Please retry")
		return nil, errors.New("timeout for waiting for container")
	}
}

type terminalConn struct {
	logger *common.BuildLogger
	ctx    context.Context

	client      docker_helpers.Client
	containerID string
	shell       []string
}

func (t terminalConn) Start(w http.ResponseWriter, r *http.Request, timeoutCh, disconnectCh chan error) {
	execConfig := types.ExecConfig{
		Tty:          true,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          t.shell,
	}

	exec, err := t.client.ContainerExecCreate(t.ctx, t.containerID, execConfig)
	if err != nil {
		t.logger.Errorln("failed to create exec container for terminal:", err)
	}

	resp, err := t.client.ContainerExecAttach(t.ctx, exec.ID, execConfig)
	if err != nil {
		t.logger.Errorln("failed to exec attach to container for terminal:", err)
	}

	dockerTTY := newDockerTTY(&resp)

	proxy := terminal.NewStreamProxy(1) // one stopper: terminal exit handler
	terminalsession.ProxyTerminal(
		timeoutCh,
		disconnectCh,
		proxy.StopCh,
		func() {
			terminal.ProxyStream(w, r, dockerTTY, proxy)
		},
	)
}

func (t terminalConn) Close() error {
	return nil
}
