//go:build integration

package docker_test

import (
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
	"gitlab.com/gitlab-org/gitlab-runner/helpers/test"
	"gitlab.com/gitlab-org/gitlab-runner/session"
)

func TestInteractiveTerminal(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

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
					PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
				},
			},
		},
		Session: sess,
	}

	// Start build
	go func() {
		_ = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	}()

	srv := httptest.NewServer(build.Session.Handler())
	defer srv.Close()

	u := url.URL{
		Scheme: "ws",
		Host:   srv.Listener.Addr().String(),
		Path:   build.Session.Endpoint + "/exec",
	}
	headers := http.Header{
		"Authorization": []string{build.Session.Token},
	}

	var webSocket *websocket.Conn
	var resp *http.Response

	started := time.Now()

	for time.Since(started) < 25*time.Second {
		webSocket, resp, err = websocket.DefaultDialer.Dial(u.String(), headers)
		if err == nil {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	require.NotNil(t, webSocket)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	defer webSocket.Close()

	err = webSocket.WriteMessage(websocket.BinaryMessage, []byte("uname\n"))
	require.NoError(t, err)

	readStarted := time.Now()
	var tty []byte
	for time.Since(readStarted) < 5*time.Second {
		typ, b, err := webSocket.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.BinaryMessage, typ)
		tty = append(tty, b...)

		if strings.Contains(string(b), "Linux") {
			break
		}

		time.Sleep(50 * time.Microsecond)
	}

	t.Log(string(tty))
	assert.Contains(t, string(tty), "Linux")
}
