package session

import (
	"net/http"
	"reflect"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

type connectionInUseError struct{}

func (connectionInUseError) Error() string {
	return "Connection already in use"
}

type Session struct {
	Endpoint string
	Token    string

	mux *http.ServeMux

	interactiveTerminal terminal.InteractiveTerminal
	terminalConn        terminal.Conn

	// Signal when client disconnects from terminal.
	DisconnectCh chan error
	// Signal when terminal session timeout.
	TimeoutCh chan error

	log *logrus.Entry
	sync.Mutex
}

func NewSession(logger *logrus.Entry) (*Session, error) {
	endpoint, token, err := generateEndpoint()
	if err != nil {
		return nil, err
	}

	if logger == nil {
		logger = logrus.NewEntry(logrus.StandardLogger())
	}

	logger = logger.WithField("uri", endpoint)

	sess := &Session{
		Endpoint:     endpoint,
		Token:        token,
		DisconnectCh: make(chan error),
		TimeoutCh:    make(chan error),

		log: logger,
	}

	sess.setMux()

	return sess, nil
}

func generateEndpoint() (string, string, error) {
	sessionUUID, err := helpers.GenerateRandomUUID(32)
	if err != nil {
		return "", "", err
	}

	token, err := generateToken()
	if err != nil {
		return "", "", err
	}

	return "/session/" + sessionUUID, token, nil
}

func generateToken() (string, error) {
	token, err := helpers.GenerateRandomUUID(32)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *Session) setMux() {
	s.Lock()
	defer s.Unlock()

	s.mux = http.NewServeMux()
	s.mux.HandleFunc(s.Endpoint+"/exec", s.execHandler)
}

func (s *Session) execHandler(w http.ResponseWriter, r *http.Request) {
	logger := s.log.WithField("uri", r.RequestURI)
	logger.Debug("Exec terminal session request")

	if !s.terminalAvailable() {
		logger.Error("Interactive terminal not set")
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	if !websocket.IsWebSocketUpgrade(r) {
		logger.Error("Request is not a web socket connection")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	if s.Token != r.Header.Get("Authorization") {
		logger.Error("Authorization header is not valid")
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	terminalConn, err := s.newTerminalConn()
	if _, ok := err.(connectionInUseError); ok {
		logger.Warn("Terminal already connected, revoking connection")
		http.Error(w, http.StatusText(http.StatusLocked), http.StatusLocked)
		return
	}

	if err != nil {
		logger.WithError(err).Error("Failed to connect to terminal")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	defer s.closeTerminalConn(terminalConn)
	logger.Debugln("Starting terminal session")
	terminalConn.Start(w, r, s.TimeoutCh, s.DisconnectCh)
}

func (s *Session) terminalAvailable() bool {
	s.Lock()
	defer s.Unlock()

	return s.interactiveTerminal != nil
}

func (s *Session) newTerminalConn() (terminal.Conn, error) {
	s.Lock()
	defer s.Unlock()

	if s.terminalConn != nil {
		return nil, connectionInUseError{}
	}

	conn, err := s.interactiveTerminal.Connect()
	if err != nil {
		return nil, err
	}

	s.terminalConn = conn

	return conn, nil
}

func (s *Session) closeTerminalConn(conn terminal.Conn) {
	s.Lock()
	defer s.Unlock()

	err := conn.Close()
	if err != nil {
		s.log.WithError(err).Warn("Failed to close terminal connection")
	}

	if reflect.ValueOf(s.terminalConn) == reflect.ValueOf(conn) {
		s.log.Warningln("Closed active terminal connection")
		s.terminalConn = nil
	}
}

func (s *Session) SetInteractiveTerminal(interactiveTerminal terminal.InteractiveTerminal) {
	s.Lock()
	defer s.Unlock()
	s.interactiveTerminal = interactiveTerminal
}

func (s *Session) Mux() *http.ServeMux {
	return s.mux
}

func (s *Session) Connected() bool {
	s.Lock()
	defer s.Unlock()

	return s.terminalConn != nil
}

func (s *Session) Kill() error {
	s.Lock()
	defer s.Unlock()

	if s.terminalConn == nil {
		return nil
	}

	err := s.terminalConn.Close()
	s.terminalConn = nil

	return err
}
