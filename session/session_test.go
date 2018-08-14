package session

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

func TestExec(t *testing.T) {
	cases := []struct {
		name               string
		authorization      string
		attachTerminal     bool
		isWebsocketUpgrade bool
		connectionErr      error
		expectedStatusCode int
	}{
		{
			name:               "Interactive terminal not available",
			attachTerminal:     false,
			isWebsocketUpgrade: true,
			expectedStatusCode: http.StatusServiceUnavailable,
		},
		{
			name:               "Request is not websocket upgraded",
			attachTerminal:     true,
			isWebsocketUpgrade: false,
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name:               "Request no authorized",
			attachTerminal:     true,
			isWebsocketUpgrade: true,
			authorization:      "invalidToken",
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "Terminal connected successfully",
			attachTerminal:     true,
			isWebsocketUpgrade: true,
			authorization:      "validToken",
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "Failed to start terminal",
			attachTerminal:     true,
			isWebsocketUpgrade: true,
			authorization:      "validToken",
			connectionErr:      errors.New("failed to connect to terminal"),
			expectedStatusCode: http.StatusInternalServerError,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			session, err := NewSession(nil)
			require.NoError(t, err)
			session.Token = "validToken"

			mockTerminalConn := terminal.MockConn{}
			mockTerminalConn.On("Start", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once()
			mockTerminalConn.On("Close").Return(nil).Once()

			mockTerminal := terminal.MockInteractiveTerminal{}
			mockTerminal.On("Connect").Return(&mockTerminalConn, c.connectionErr).Once()

			if c.attachTerminal {
				session.SetInteractiveTerminal(&mockTerminal)
			}

			req := httptest.NewRequest(http.MethodPost, session.Endpoint+"/exec", nil)

			if c.isWebsocketUpgrade {
				req.Header.Add("Connection", "upgrade")
				req.Header.Add("Upgrade", "websocket")
			}
			req.Header.Add("Authorization", c.authorization)

			w := httptest.NewRecorder()

			session.Mux().ServeHTTP(w, req)

			resp := w.Result()

			assert.Equal(t, c.expectedStatusCode, resp.StatusCode)
		})
	}
}

func TestDoNotAllowMultipleConnections(t *testing.T) {
	session, err := NewSession(nil)
	require.NoError(t, err)
	session.Token = "validToken"

	mockTerminalConn := terminal.MockConn{}
	mockTerminalConn.On("Start", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once()
	mockTerminalConn.On("Close").Return().Once()

	mockTerminal := terminal.MockInteractiveTerminal{}
	mockTerminal.On("Connect").Return(&mockTerminalConn, nil).Once()

	session.SetInteractiveTerminal(&mockTerminal)

	// Simulating another connection has already started.
	conn, err := session.newTerminalConn()
	require.NotNil(t, conn)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, session.Endpoint+"/exec", nil)
	req.Header.Add("Connection", "upgrade")
	req.Header.Add("Upgrade", "websocket")
	req.Header.Add("Authorization", "validToken")

	w := httptest.NewRecorder()
	session.Mux().ServeHTTP(w, req)
	resp := w.Result()
	assert.Equal(t, http.StatusLocked, resp.StatusCode)
}

func TestConnected(t *testing.T) {
	sess, err := NewSession(nil)
	require.NoError(t, err)

	assert.False(t, sess.Connected())
	sess.terminalConn = &terminal.MockConn{}
	assert.True(t, sess.Connected())
}

func TestKill(t *testing.T) {
	sess, err := NewSession(nil)
	require.NoError(t, err)

	// No connection attached
	err = sess.Kill()
	assert.NoError(t, err)

	mockConn := terminal.MockConn{}
	mockConn.On("Close").Return(nil).Once()

	sess.terminalConn = &mockConn

	err = sess.Kill()
	assert.NoError(t, err)
	assert.Nil(t, sess.terminalConn)
}

func TestKillFailedToClose(t *testing.T) {
	sess, err := NewSession(nil)
	require.NoError(t, err)

	mockConn := terminal.MockConn{}
	mockConn.On("Close").Return(errors.New("some error")).Once()

	sess.terminalConn = &mockConn

	err = sess.Kill()
	assert.Error(t, err)

	// Even though an error occurred closing it still is removed.
	assert.Nil(t, sess.terminalConn)
}
