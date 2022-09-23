//go:build !integration

package session

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

func TestExecSuccessful(t *testing.T) {
	validToken := "validToken"
	session, err := NewSession(nil)
	require.NoError(t, err)

	session.Token = validToken

	mockTerminalConn := new(terminal.MockConn)
	defer mockTerminalConn.AssertExpectations(t)

	mockTerminalConn.On("Start", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once()
	mockTerminalConn.On("Close").Return(nil).Once()

	mockTerminal := new(terminal.MockInteractiveTerminal)
	defer mockTerminal.AssertExpectations(t)

	mockTerminal.On("Connect").Return(mockTerminalConn, nil).Once()

	session.SetInteractiveTerminal(mockTerminal)

	req := httptest.NewRequest(http.MethodPost, session.Endpoint+"/exec", nil)

	req.Header.Add("Connection", "upgrade")
	req.Header.Add("Upgrade", "websocket")
	req.Header.Add("Authorization", validToken)

	w := httptest.NewRecorder()

	session.Handler().ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestExecFailedRequest(t *testing.T) {
	validToken := "validToken"

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
			authorization:      validToken,
			expectedStatusCode: http.StatusServiceUnavailable,
		},
		{
			name:               "Request is not websocket upgraded",
			attachTerminal:     true,
			isWebsocketUpgrade: false,
			authorization:      validToken,
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
			name:               "Failed to start terminal",
			attachTerminal:     true,
			isWebsocketUpgrade: true,
			authorization:      validToken,
			connectionErr:      errors.New("failed to connect to terminal"),
			expectedStatusCode: http.StatusInternalServerError,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			session, err := NewSession(nil)
			require.NoError(t, err)
			session.Token = validToken

			mockTerminalConn := new(terminal.MockConn)
			defer mockTerminalConn.AssertExpectations(t)

			mockTerminal := new(terminal.MockInteractiveTerminal)
			defer mockTerminal.AssertExpectations(t)

			if c.authorization == validToken && c.isWebsocketUpgrade && c.attachTerminal {
				mockTerminal.On("Connect").Return(mockTerminalConn, c.connectionErr).Once()
			}

			if c.attachTerminal {
				session.SetInteractiveTerminal(mockTerminal)
			}

			req := httptest.NewRequest(http.MethodPost, session.Endpoint+"/exec", nil)

			if c.isWebsocketUpgrade {
				req.Header.Add("Connection", "upgrade")
				req.Header.Add("Upgrade", "websocket")
			}
			req.Header.Add("Authorization", c.authorization)

			w := httptest.NewRecorder()

			session.Handler().ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, c.expectedStatusCode, resp.StatusCode)
		})
	}
}

func TestDoNotAllowMultipleConnections(t *testing.T) {
	validToken := "validToken"
	session, err := NewSession(nil)
	require.NoError(t, err)
	session.Token = validToken

	mockTerminalConn := new(terminal.MockConn)
	defer mockTerminalConn.AssertExpectations(t)

	mockTerminal := new(terminal.MockInteractiveTerminal)
	defer mockTerminal.AssertExpectations(t)
	mockTerminal.On("Connect").Return(mockTerminalConn, nil).Once()

	session.SetInteractiveTerminal(mockTerminal)

	// Simulating another connection has already started.
	conn, err := session.newTerminalConn()
	require.NotNil(t, conn)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, session.Endpoint+"/exec", nil)
	req.Header.Add("Connection", "upgrade")
	req.Header.Add("Upgrade", "websocket")
	req.Header.Add("Authorization", validToken)

	w := httptest.NewRecorder()
	session.Handler().ServeHTTP(w, req)
	resp := w.Result()
	defer resp.Body.Close()
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

	mockConn := new(terminal.MockConn)
	defer mockConn.AssertExpectations(t)
	mockConn.On("Close").Return(nil).Once()

	sess.terminalConn = mockConn

	err = sess.Kill()
	assert.NoError(t, err)
	assert.Nil(t, sess.terminalConn)
}

func TestKillFailedToClose(t *testing.T) {
	sess, err := NewSession(nil)
	require.NoError(t, err)

	mockConn := new(terminal.MockConn)
	defer mockConn.AssertExpectations(t)
	mockConn.On("Close").Return(errors.New("some error")).Once()

	sess.terminalConn = mockConn

	err = sess.Kill()
	assert.Error(t, err)

	// Even though an error occurred closing it still is removed.
	assert.Nil(t, sess.terminalConn)
}

type fakeTerminalConn struct {
	commands []string
}

func (fakeTerminalConn) Close() error {
	return nil
}

func (fakeTerminalConn) Start(w http.ResponseWriter, r *http.Request, timeoutCh, disconnectCh chan error) {
}

func TestCloseTerminalConn(t *testing.T) {
	conn := &fakeTerminalConn{
		commands: []string{"command", "-c", "random"},
	}

	mockConn := new(terminal.MockConn)
	defer mockConn.AssertExpectations(t)
	mockConn.On("Close").Return(nil).Once()

	sess, err := NewSession(nil)
	sess.terminalConn = conn
	require.NoError(t, err)

	sess.closeTerminalConn(mockConn)
	assert.NotNil(t, sess.terminalConn)

	sess.closeTerminalConn(conn)
	assert.Nil(t, sess.terminalConn)
}

func TestProxy(t *testing.T) {
	validToken := "validToken"
	invalidServiceName := "invalidServiceName"
	validServiceName := "serviceName"

	cases := map[string]struct {
		authorization           string
		serviceName             string
		expectedStatusCode      int
		defineConnectionHandler bool
	}{
		"Request no authorized": {
			authorization:      "invalidToken",
			serviceName:        validServiceName,
			expectedStatusCode: http.StatusUnauthorized,
		},
		"Service proxy not found": {
			authorization:      validToken,
			serviceName:        invalidServiceName,
			expectedStatusCode: http.StatusNotFound,
		},
		"Service proxy connection handler is undefined": {
			authorization:      validToken,
			serviceName:        validServiceName,
			expectedStatusCode: http.StatusNotFound,
		},
		"Request proxied": {
			authorization:           validToken,
			serviceName:             validServiceName,
			expectedStatusCode:      http.StatusOK,
			defineConnectionHandler: true,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			session, err := NewSession(nil)
			require.NoError(t, err)
			session.Token = validToken
			mockConn := new(proxy.MockRequester)
			defer mockConn.AssertExpectations(t)

			var connectionHandler proxy.Requester
			if c.defineConnectionHandler {
				connectionHandler = mockConn
			}

			session.proxyPool = proxy.Pool{
				"serviceName": mockProxy("test", 80, "http", "default_port", connectionHandler),
			}

			req := httptest.NewRequest(http.MethodGet, session.Endpoint+"/proxy/"+c.serviceName+"/80/", nil)
			req.Header.Add("Authorization", c.authorization)

			w := httptest.NewRecorder()

			if c.defineConnectionHandler && c.expectedStatusCode == http.StatusOK {
				mockConn.On("ProxyRequest", mock.Anything, mock.Anything, mock.Anything, "80", mock.Anything).Once()
			}

			session.Handler().ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			assert.Equal(t, c.expectedStatusCode, resp.StatusCode)
		})
	}
}

func TestProxyVars(t *testing.T) {
	tests := map[string]struct {
		service      string
		port         string
		requestedURI string
		ok           bool
	}{
		"empty":              {"", "", "", false},
		"/":                  {"", "", "", false},
		"//":                 {"", "", "", false},
		"///":                {"", "", "/", false},
		"/80/":               {"", "80", "", false},
		"/80/foo/bar":        {"", "80", "foo/bar", false},
		"service//foo/bar":   {"service", "", "foo/bar", false},
		"service/80/foo":     {"service", "80", "foo", true},
		"service/80/foo/bar": {"service", "80", "foo/bar", true},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			service, port, requestedURI, ok := parseProxyParams(tn)
			assert.Equal(t, tc.service, service)
			assert.Equal(t, tc.port, port)
			assert.Equal(t, tc.requestedURI, requestedURI)
			assert.Equal(t, tc.ok, ok)
		})
	}
}

func mockProxy(
	serviceName string,
	port int,
	protocol string,
	portName string,
	connectionHandler proxy.Requester,
) *proxy.Proxy {
	p := &proxy.Proxy{
		Settings: &proxy.Settings{
			ServiceName: serviceName,
			Ports: []proxy.Port{
				mockProxyPort(port, protocol, portName),
			},
		},
	}

	if connectionHandler != nil {
		p.ConnectionHandler = connectionHandler
	}

	return p
}

func mockProxyPort(port int, protocol string, portName string) proxy.Port {
	return proxy.Port{
		Number:   port,
		Protocol: protocol,
		Name:     portName,
	}
}
