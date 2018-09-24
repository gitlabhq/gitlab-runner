package docker

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
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
		err = build.Run(&common.Config{}, &common.Trace{Writer: buildLogWriter})
		require.NoError(t, err)
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

func TestInteractiveWebTerminalWaitForContainerTimeout(t *testing.T) {
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

}

func TestInteractiveWebTerminalAttachStrategy(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
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

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.NoError(t, err)

	require.False(t, build.Session.Connected())
}
