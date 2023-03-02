//go:build !integration

package docker

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/wait"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/session"
)

func TestCommandExecutor_Connect(t *testing.T) {
	tests := []struct {
		name                  string
		buildContainerRunning bool
		hasBuildContainer     bool
		containerInspectErr   error
		expectedErr           error
	}{
		{
			name:                  "Connect Timeout",
			buildContainerRunning: false,
			hasBuildContainer:     true,
			expectedErr:           buildContainerTerminalTimeout{},
		},
		{
			name:                  "Successful connect",
			buildContainerRunning: true,
			hasBuildContainer:     true,
			containerInspectErr:   nil,
		},
		{
			name:                  "Container inspect failed",
			buildContainerRunning: false,
			hasBuildContainer:     true,
			containerInspectErr:   errors.New("container not found"),
			expectedErr:           errors.New("container not found"),
		},
		{
			name:                  "No build container",
			buildContainerRunning: false,
			hasBuildContainer:     false,
			expectedErr:           buildContainerTerminalTimeout{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := new(docker.MockClient)
			defer c.AssertExpectations(t)

			s := commandExecutor{
				executor: executor{
					AbstractExecutor: executors.AbstractExecutor{
						Context: context.Background(),
						BuildShell: &common.ShellConfiguration{
							DockerCommand: []string{"/bin/sh"},
						},
					},
					client: c,
				},
				terminalWaitForContainerTimeout: 1 * time.Second,
			}

			if test.hasBuildContainer {
				s.buildContainer = &types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						ID: "1234",
					},
				}

				c.On("ContainerInspect", s.Context, "1234").Return(types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						State: &types.ContainerState{
							Running: test.buildContainerRunning,
						},
					},
				}, test.containerInspectErr)
			}

			conn, err := s.Connect()

			if test.buildContainerRunning {
				assert.NoError(t, err)
				assert.NotNil(t, conn)
				assert.IsType(t, terminalConn{}, conn)
				return
			}

			assert.EqualError(t, err, test.expectedErr.Error())
			assert.Nil(t, conn)
		})
	}
}

func TestTerminalConn_FailToStart(t *testing.T) {
	tests := []struct {
		name                   string
		containerExecCreateErr error
		containerExecAttachErr error
	}{
		{
			name:                   "Failed to create exec container",
			containerExecCreateErr: errors.New("failed to create exec container"),
			containerExecAttachErr: nil,
		},
		{
			name:                   "Failed to attach exec container",
			containerExecCreateErr: nil,
			containerExecAttachErr: errors.New("failed to attach exec container"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := new(docker.MockClient)
			defer c.AssertExpectations(t)

			s := commandExecutor{
				executor: executor{
					AbstractExecutor: executors.AbstractExecutor{
						Context: context.Background(),
						BuildShell: &common.ShellConfiguration{
							DockerCommand: []string{"/bin/sh"},
						},
					},
					client: c,
				},
				buildContainer: &types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						ID: "1234",
					},
				},
			}

			c.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					State: &types.ContainerState{
						Running: true,
					},
				},
			}, nil)

			c.On("ContainerExecCreate", mock.Anything, mock.Anything, mock.Anything).Return(
				types.IDResponse{},
				test.containerExecCreateErr,
			).Once()

			if test.containerExecCreateErr == nil {
				c.On("ContainerExecAttach", mock.Anything, mock.Anything, mock.Anything).Return(
					types.HijackedResponse{},
					test.containerExecAttachErr,
				).Once()
			}

			conn, err := s.Connect()
			require.NoError(t, err)

			timeoutCh := make(chan error)
			disconnectCh := make(chan error)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "wss://example.com/foo", nil)
			conn.Start(w, req, timeoutCh, disconnectCh)

			resp := w.Result()
			assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
			defer resp.Body.Close()
		})
	}
}

type nopReader struct {
}

func (w *nopReader) Read(b []byte) (int, error) {
	return len(b), nil
}

type nopConn struct {
}

func (nopConn) Read(b []byte) (n int, err error) {
	return len(b), nil
}

func (nopConn) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (nopConn) Close() error {
	return nil
}

func (nopConn) LocalAddr() net.Addr {
	return &net.TCPAddr{}
}

func (nopConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{}
}

func (nopConn) SetDeadline(t time.Time) error {
	return nil
}

func (nopConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (nopConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestTerminalConn_Start(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	s := commandExecutor{
		executor: executor{
			AbstractExecutor: executors.AbstractExecutor{
				Context: context.Background(),
				BuildShell: &common.ShellConfiguration{
					DockerCommand: []string{"/bin/sh"},
				},
			},
			client: c,
			waiter: wait.NewDockerKillWaiter(c),
		},
		buildContainer: &types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				ID: "1234",
			},
		},
	}

	c.On("ContainerInspect", mock.Anything, "1234").Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Running: true,
			},
		},
	}, nil).Once()

	c.On("ContainerExecCreate", mock.Anything, mock.Anything, mock.Anything).Return(types.IDResponse{
		ID: "4321",
	}, nil).Once()

	c.On("ContainerExecAttach", mock.Anything, mock.Anything, mock.Anything).Return(types.HijackedResponse{
		Conn:   nopConn{},
		Reader: bufio.NewReader(&nopReader{}),
	}, nil).Once()

	bodyCh := make(chan container.WaitResponse, 1)
	bodyCh <- container.WaitResponse{StatusCode: 0}
	c.On("ContainerWait", mock.Anything, "1234", container.WaitConditionNotRunning).
		Return((<-chan container.WaitResponse)(bodyCh), nil)

	session, err := session.NewSession(nil)
	require.NoError(t, err)
	session.Token = "validToken"

	session.SetInteractiveTerminal(&s)

	srv := httptest.NewServer(session.Handler())

	u := url.URL{
		Scheme: "ws",
		Host:   srv.Listener.Addr().String(),
		Path:   session.Endpoint + "/exec",
	}
	headers := http.Header{
		"Authorization": []string{"validToken"},
	}

	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), headers)
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.Equal(t, resp.StatusCode, http.StatusSwitchingProtocols)
	defer resp.Body.Close()

	defer conn.Close()

	go func() {
		for {
			err := conn.WriteMessage(websocket.BinaryMessage, []byte("data"))
			if err != nil {
				return
			}

			time.Sleep(time.Second)
		}
	}()

	started := time.Now()

	for time.Since(started) < 5*time.Second {
		if !session.Connected() {
			break
		}

		time.Sleep(50 * time.Microsecond)
	}

	assert.False(t, session.Connected())
}
