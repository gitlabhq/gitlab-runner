package docker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/docker/docker/api/types"
	"gitlab.com/gitlab-org/gitlab-terminal"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	terminalsession "gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

func (s *commandExecutor) watchForRunningBuildContainer(deadline time.Time) (string, error) {
	for time.Since(deadline) < 0 {
		if s.buildContainer == nil {
			time.Sleep(time.Second)
			continue
		}

		containerID := s.buildContainer.ID
		container, err := s.client.ContainerInspect(s.Context, containerID)
		if err != nil {
			return "", err
		}

		if container.State.Running {
			return containerID, nil
		}
	}

	s.BuildLogger.Errorln("Timed out waiting for the container to start the terminal. Please retry")
	return "", errors.New("timeout for waiting for build container")
}

func (s *commandExecutor) Connect() (terminalsession.Conn, error) {
	// Waiting for the container to start,  is not ideal as it might be hiding a
	// real issue and the user is not aware of it. Ideally, the runner should
	// inform the user in an interactive way that the container has no started
	// yet and should wait/try again. This isn't an easy task to do since we
	// can't access the WebSocket here since that is the responsibility of
	// `gitlab-terminal` package. There are plans to improve this please take a
	// look at https://gitlab.com/gitlab-org/gitlab-ce/issues/50384#proposal and
	// https://gitlab.com/gitlab-org/gitlab-terminal/issues/4
	containerID, err := s.watchForRunningBuildContainer(time.Now().Add(waitForContainerTimeout))
	if err != nil {
		return nil, err
	}

	ctx, cancelFn := context.WithCancel(s.Context)

	return terminalConn{
		logger:      &s.BuildLogger,
		ctx:         ctx,
		cancelFn:    cancelFn,
		executor:    s,
		client:      s.client,
		containerID: containerID,
		shell:       s.BuildShell.DockerCommand,
	}, nil
}

type terminalConn struct {
	logger   *common.BuildLogger
	ctx      context.Context
	cancelFn func()

	executor    *commandExecutor
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
		t.logger.Errorln("Failed to create exec container for terminal:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	resp, err := t.client.ContainerExecAttach(t.ctx, exec.ID, execConfig)
	if err != nil {
		t.logger.Errorln("Failed to exec attach to container for terminal:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	dockerTTY := newDockerTTY(&resp)
	proxy := terminal.NewStreamProxy(1) // one stopper: terminal exit handler

	// wait for container to exit
	go func() {
		t.logger.Debugln("Waiting for the terminal container:", t.containerID)
		err := t.executor.waitForContainer(t.ctx, t.containerID)
		t.logger.Debugln("The terminal container:", t.containerID, "finished with:", err)

		stopCh := proxy.GetStopCh()
		if err != nil {
			stopCh <- fmt.Errorf("build container exited with %q", err)
		} else {
			stopCh <- errors.New("build container exited")
		}
	}()

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
	if t.cancelFn != nil {
		t.cancelFn()
	}
	return nil
}
