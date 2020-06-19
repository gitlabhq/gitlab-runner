package ssh

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/tevino/abool"
	cryptoSSH "golang.org/x/crypto/ssh"
)

type StubSSHServer struct {
	User     string
	Password string
	Config   *cryptoSSH.ServerConfig

	stop       chan bool
	shouldExit *abool.AtomicBool
}

func NewStubServer(user, pass string, privateKey []byte) (*StubSSHServer, error) {
	server := &StubSSHServer{
		User:     user,
		Password: pass,
		Config: &cryptoSSH.ServerConfig{
			PasswordCallback: func(conn cryptoSSH.ConnMetadata, password []byte) (*cryptoSSH.Permissions, error) {
				if conn.User() == user && string(password) == pass {
					return nil, nil
				}
				return nil, fmt.Errorf("wrong password for %q", conn.User())
			},
		},
		stop:       make(chan bool),
		shouldExit: abool.New(),
	}

	key, err := cryptoSSH.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	server.Config.AddHostKey(key)

	return server, nil
}

func (s *StubSSHServer) Start() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		return 0, err
	}

	go func() {
		<-s.stop
		s.shouldExit.Set()
		_ = listener.Close()
	}()

	address := strings.SplitN(listener.Addr().String(), ":", 2)
	go s.mainLoop(listener)

	return strconv.Atoi(address[1])
}

func (s *StubSSHServer) Stop() {
	s.stop <- true
}

func (s *StubSSHServer) mainLoop(listener net.Listener) {
	for {
		if s.shouldExit.IsSet() {
			return
		}

		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		if s.shouldExit.IsSet() {
			return
		}

		// upgrade to ssh connection
		_, _, _, _ = cryptoSSH.NewServerConn(conn, s.Config)
		// This is enough just for handling incoming connections
	}
}
