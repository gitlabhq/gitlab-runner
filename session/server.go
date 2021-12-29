package session

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/certificate"
)

var (
	ErrInvalidURL = errors.New("url not valid, scheme defined")
)

type sessionFinderFn func(url string) *Session

type Server struct {
	config        ServerConfig
	log           *logrus.Entry
	tlsListener   net.Listener
	sessionFinder sessionFinderFn
	httpServer    *http.Server

	CertificatePublicKey []byte
	AdvertiseAddress     string
}

type ServerConfig struct {
	AdvertiseAddress string
	ListenAddress    string
	ShutdownTimeout  time.Duration
}

func NewServer(
	config ServerConfig,
	logger *logrus.Entry,
	certGen certificate.Generator,
	sessionFinder sessionFinderFn,
) (*Server, error) {
	if logger == nil {
		logger = logrus.NewEntry(logrus.StandardLogger())
	}

	server := Server{
		config:        config,
		log:           logger,
		sessionFinder: sessionFinder,
		httpServer:    &http.Server{},
	}

	host, err := server.getPublicHost()
	if err != nil {
		return nil, err
	}

	cert, publicKey, err := certGen.Generate(host)
	if err != nil {
		return nil, err
	}

	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	// We separate out the listener creation here so that we can return an error
	// if the provided address is invalid or there is some other listener error.
	listener, err := net.Listen("tcp", server.config.ListenAddress)
	if err != nil {
		return nil, err
	}

	server.tlsListener = tls.NewListener(listener, &tlsConfig)

	err = server.setAdvertiseAddress()
	if err != nil {
		return nil, err
	}

	server.CertificatePublicKey = publicKey
	server.httpServer.Handler = http.HandlerFunc(server.handleSessionRequest)

	return &server, nil
}

func (s *Server) getPublicHost() (string, error) {
	for _, address := range []string{s.config.AdvertiseAddress, s.config.ListenAddress} {
		if address == "" {
			continue
		}

		host, _, err := net.SplitHostPort(address)
		if err != nil {
			s.log.
				WithField("address", address).
				WithError(err).
				Warn("Failed to parse session address")
		}

		if host == "" {
			continue
		}

		return host, nil
	}

	return "", errors.New("no valid address provided")
}

func (s *Server) setAdvertiseAddress() error {
	s.AdvertiseAddress = s.config.AdvertiseAddress
	if s.config.AdvertiseAddress == "" {
		s.AdvertiseAddress = s.config.ListenAddress
	}

	if strings.HasPrefix(s.AdvertiseAddress, "https://") ||
		strings.HasPrefix(s.AdvertiseAddress, "http://") {
		return ErrInvalidURL
	}

	s.AdvertiseAddress = "https://" + s.AdvertiseAddress
	_, err := url.ParseRequestURI(s.AdvertiseAddress)

	return err
}

func (s *Server) handleSessionRequest(w http.ResponseWriter, r *http.Request) {
	logger := s.log.WithField("uri", r.RequestURI)
	logger.Debug("Processing session request")

	session := s.sessionFinder(r.RequestURI)
	if session == nil || session.Handler() == nil { //nolint:staticcheck
		logger.Error("Mux handler not found")
		http.NotFound(w, r)
		return
	}

	session.Handler().ServeHTTP(w, r)
}

func (s *Server) Start() error {
	if s.httpServer == nil {
		return errors.New("http server not set")
	}

	err := s.httpServer.Serve(s.tlsListener)

	// ErrServerClosed is a legitimate error that should not cause failure
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) Close() {
	if s.httpServer != nil {
		_ = s.httpServer.Close()
	}
}
