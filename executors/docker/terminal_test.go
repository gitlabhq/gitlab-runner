package docker

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/session"
)

func TestInteractiveTerminal(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	successfulBuild, err := common.GetRemoteLongRunningBuild()
	assert.NoError(t, err)

	sess, err := session.NewSession(nil)
	require.NoError(t, err)

	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
		Session: sess,
	}

	// Start build
	buildLogWriter := bytes.NewBuffer(nil)
	go func() {
		_ = build.Run(&common.Config{}, &common.Trace{Writer: buildLogWriter})
	}()

	buildStarted := make(chan struct{})
	go func() {
		for {
			if buildLogWriter.Len() == 0 {
				time.Sleep(1 * time.Second)
				continue
			}

			ln, _ := buildLogWriter.ReadBytes('\n')

			// Print out to the user to aid debugging
			if len(ln) > 0 {
				fmt.Fprint(os.Stdout, string(ln))
			}

			// We signal that the build has start and we can start using terminal.
			if strings.Contains(string(ln), "sleep") {
				buildStarted <- struct{}{}
			}
		}
	}()

	<-buildStarted

	srv := httptest.NewServer(build.Session.Mux())
	defer srv.Close()

	u := url.URL{Scheme: "ws", Host: srv.Listener.Addr().String(), Path: build.Session.Endpoint + "/exec"}
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), http.Header{"Authorization": []string{build.Session.Token}})
	require.NoError(t, err)

	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	defer func() {
		if conn != nil {
			defer conn.Close()
		}
	}()

	err = conn.WriteMessage(websocket.BinaryMessage, []byte("uname\n"))
	require.NoError(t, err)

	var unameResult string
	for i := 0; i < 3; i++ {
		typ, b, err := conn.ReadMessage()
		require.NoError(t, err)
		assert.Equal(t, websocket.BinaryMessage, typ)
		unameResult = string(b)
	}

	assert.Contains(t, unameResult, "Linux")
}

func TestCommandExecutor_Connect_Timeout(t *testing.T) {
	c := &docker_helpers.MockClient{}

	s := commandExecutor{
		executor: executor{
			AbstractExecutor: executors.AbstractExecutor{
				Context: context.Background(),
			},
			client: c,
		},
		buildContainer: &types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				ID: "1234",
			},
		},
	}
	c.On("ContainerInspect", s.Context, "1234").Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Running: false,
			},
		},
	}, nil)

	buildLogWriter := bytes.NewBuffer(nil)
	s.BuildLogger = common.NewBuildLogger(&common.Trace{Writer: buildLogWriter}, logrus.NewEntry(logrus.New()))

	conn, err := s.Connect()
	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, buildLogWriter.String(), "Timed out waiting for the container to start the terminal. Please retry")
}

func TestCommandExecutor_Connect(t *testing.T) {
	c := &docker_helpers.MockClient{}

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
	c.On("ContainerInspect", s.Context, "1234").Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Running: true,
			},
		},
	}, nil)

	conn, err := s.Connect()
	assert.NoError(t, err)
	assert.NotNil(t, conn)
	assert.IsType(t, terminalConn{}, conn)
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
	c := &docker_helpers.MockClient{}

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

	c.On("ContainerInspect", mock.Anything, "1234").Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Running: false,
			},
		},
	}, nil)

	session, err := session.NewSession(nil)
	require.NoError(t, err)
	session.Token = "validToken"

	session.SetInteractiveTerminal(&s)

	srv := httptest.NewServer(session.Mux())

	u := url.URL{Scheme: "ws", Host: srv.Listener.Addr().String(), Path: session.Endpoint + "/exec"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{"Authorization": []string{"validToken"}})

	go func() {
		for {
			conn.WriteMessage(websocket.BinaryMessage, []byte("data"))
			time.Sleep(time.Second)
		}
	}()

	time.Sleep(5 * time.Second)

	assert.False(t, session.Connected())
}
