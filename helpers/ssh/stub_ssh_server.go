package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/tevino/abool"
	cryptoSSH "golang.org/x/crypto/ssh"
)

type StubSSHServer struct {
	User     string
	Password string
	Config   *cryptoSSH.ServerConfig

	host               string
	port               string
	privateKeyLocation string
	stop               chan bool
	shouldExit         *abool.AtomicBool
	cleanup            func()
}

var TestSSHKeyPair = struct {
	PublicKey  string
	PrivateKey string
}{
	PrivateKey: `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAlwAAAAdzc2gtcn
NhAAAAAwEAAQAAAIEA2FnuhEf3bCtSe6eyg5/Ir3kzjGx3gFij1H3QmerGIzz7JW+oxVWf
r+x7Ix61dZcE/8VXow4C2BFOXRNoa8KFN1gQh+jbbZTgc1sWCTyr6iKZIDoKR59W4pceTP
TnAQ4RHNNJwhCTDDsYlklCRBpJ79d6nt9r5O2kbVju3/wTCUsAAAIYw8mlC8PJpQsAAAAH
c3NoLXJzYQAAAIEA2FnuhEf3bCtSe6eyg5/Ir3kzjGx3gFij1H3QmerGIzz7JW+oxVWfr+
x7Ix61dZcE/8VXow4C2BFOXRNoa8KFN1gQh+jbbZTgc1sWCTyr6iKZIDoKR59W4pceTPTn
AQ4RHNNJwhCTDDsYlklCRBpJ79d6nt9r5O2kbVju3/wTCUsAAAADAQABAAAAgGBufUSSuz
KIgMRC8+t9Hbswv4w4kG8xkxxUU9U28sekF6ERCt2iE4IbWqtFtcXK4VyLfktcJGJgHFia
HPHjCvLVKGxBqoM1beWctSIpdjlu+VJedNkaFpEKZRe7Wpx61B7an+JdZJiR87CSJxkkGE
GLhuZwio6O8bBof2NEtScxAAAAQCzvxCvu+cswV+V4TYeTc/Wr7WN0J4omkwKWa0y69Z2Y
8zV2SpSoex+7mCsWQrumDCxIn+lQ7g45kdoYqAIPWZwAAABBAPRzwg8P861S4jMxnTFMUb
0izGpRrSSyrMWmhnB6do42CavG1LrS6bo0JTHVRb2uhP0OVfSWscb8C2s2oXK7FTMAAABB
AOKSVxw+gKB6O9Ez6Tr732hotJVeo04HGZ3ZCQWigFabouRbR5dUntt5ElRmCFVSJW/XnZ
tlxpSUh4YUnfTGi4kAAAAham9obmNhaUBKb2hucy1NYWNCb29rLVByby0zLmxvY2FsAQI=
-----END OPENSSH PRIVATE KEY-----`,
	//nolint:lll
	PublicKey: `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDYWe6ER/dsK1J7p7KDn8iveTOMbHeAWKPUfdCZ6sYjPPslb6jFVZ+v7HsjHrV1lwT/xVejDgLYEU5dE2hrwoU3WBCH6NttlOBzWxYJPKvqIpkgOgpHn1bilx5M9OcBDhEc00nCEJMMOxiWSUJEGknv13qe32vk7aRtWO7f/BMJSw==`,
}

func NewStubServer(user, pass string) (*StubSSHServer, error) {
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

	tempDir, err := os.MkdirTemp("", "ssh-stub-server")
	if err != nil {
		return nil, err
	}

	privateKeyLocation := filepath.Join(tempDir, "id_rsa_test")
	publicKeyLocation := filepath.Join(tempDir, "id_rsa_test.pub")

	if err := os.WriteFile(privateKeyLocation, []byte(TestSSHKeyPair.PrivateKey), 0o600); err != nil {
		return nil, err
	}

	if err := os.WriteFile(publicKeyLocation, []byte(TestSSHKeyPair.PublicKey), 0o600); err != nil {
		return nil, err
	}

	key, err := cryptoSSH.ParsePrivateKey([]byte(TestSSHKeyPair.PrivateKey))
	if err != nil {
		return nil, err
	}

	server.cleanup = func() {
		os.RemoveAll(tempDir)
	}

	server.privateKeyLocation = privateKeyLocation
	server.Config.AddHostKey(key)

	if err := server.start(); err != nil {
		return nil, err
	}

	return server, nil
}

func (s *StubSSHServer) start() error {
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		return err
	}

	go func() {
		<-s.stop
		s.shouldExit.Set()
		_ = listener.Close()
	}()

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return err
	}
	s.host = host
	s.port = port

	go s.mainLoop(listener)

	return err
}

func (s *StubSSHServer) Host() string {
	return s.host
}

func (s *StubSSHServer) Port() string {
	return s.port
}

func (s *StubSSHServer) Stop() {
	s.stop <- true
	s.cleanup()
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

func (s *StubSSHServer) Client() Client {
	return Client{
		Config: Config{
			User:         s.User,
			Password:     s.Password,
			Host:         "127.0.0.1",
			Port:         s.port,
			IdentityFile: s.privateKeyLocation,
		},
	}
}
