package ssh

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"golang.org/x/crypto/ssh"
)

type StubSSHServer struct {
	User     string
	Password string
	Config   *ssh.ServerConfig

	ExecuteLocal bool

	host               string
	port               string
	privateKeyLocation string
	stopped            chan struct{}
	closed             bool
	tempDir            string
	listener           net.Listener
	once               sync.Once
	err                error
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
	PublicKey: `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDYWe6ER/dsK1J7p7KDn8iveTOMbHeAWKPUfdCZ6sYjPPslb6jFVZ+v7HsjHrV1lwT/xVejDgLYEU5dE2hrwoU3WBCH6NttlOBzWxYJPKvqIpkgOgpHn1bilx5M9OcBDhEc00nCEJMMOxiWSUJEGknv13qe32vk7aRtWO7f/BMJSw==`,
}

func NewStubServer(user, pass string) (server *StubSSHServer, err error) {
	tempDir, err := os.MkdirTemp("", "ssh-stub-server")
	if err != nil {
		return nil, err
	}

	server = &StubSSHServer{
		User:     user,
		Password: pass,
		Config: &ssh.ServerConfig{
			PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
				if conn.User() == user && string(password) == pass {
					return nil, nil
				}
				return nil, fmt.Errorf("wrong password for %q", conn.User())
			},
		},
		stopped: make(chan struct{}),
		tempDir: tempDir,
	}

	privateKeyLocation := filepath.Join(tempDir, "id_rsa_test")
	publicKeyLocation := filepath.Join(tempDir, "id_rsa_test.pub")

	defer func() {
		if err != nil {
			os.RemoveAll(tempDir)
		}
	}()

	if err := os.WriteFile(privateKeyLocation, []byte(TestSSHKeyPair.PrivateKey), 0o600); err != nil {
		return nil, err
	}

	if err := os.WriteFile(publicKeyLocation, []byte(TestSSHKeyPair.PublicKey), 0o600); err != nil {
		return nil, err
	}

	key, err := ssh.ParsePrivateKey([]byte(TestSSHKeyPair.PrivateKey))
	if err != nil {
		return nil, err
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

	s.listener = listener
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return err
	}
	s.host = host
	s.port = port

	go s.mainLoop(listener)

	return err
}

func (s *StubSSHServer) setError(err error) {
	if errors.Is(err, io.EOF) {
		return
	}
	if err != nil {
		s.once.Do(func() {
			s.err = err
		})
	}
}

func (s *StubSSHServer) Host() string {
	return s.host
}

func (s *StubSSHServer) Port() string {
	return s.port
}

func (s *StubSSHServer) Stop() error {
	if s.closed {
		return s.err
	}

	s.closed = true
	s.listener.Close()
	os.RemoveAll(s.tempDir)

	err := s.err
	// if the error is expected because we cancelled, don't return an error
	if errors.Is(err, context.Canceled) {
		err = nil
	}

	select {
	case <-s.stopped:
		return err

	case <-time.After(45 * time.Second):
		return fmt.Errorf("timed out waiting for active ssh session to close")
	}
}

//nolint:gocognit,funlen
func (s *StubSSHServer) mainLoop(listener net.Listener) {
	defer close(s.stopped)

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		conn, err := listener.Accept()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if errors.Is(err, io.EOF) {
			continue
		}
		if err != nil {
			s.setError(err)
			return
		}

		_, channels, reqs, err := ssh.NewServerConn(conn, s.Config)
		if !s.ExecuteLocal {
			// existing tests rely on us just continuing without serving the SSH request if we're not executing locally
			continue
		}
		if err != nil {
			s.setError(err)
			return
		}

		go ssh.DiscardRequests(reqs)

		go func() {
			for channel := range channels {
				wg.Add(1)

				go func(channel ssh.NewChannel) {
					defer wg.Done()

					var err error

					switch channel.ChannelType() {
					case "session":
						err = s.handleSession(ctx, channel)

					case "direct-tcpip":
						var directTCPIP struct {
							DestAddr  string
							DestPort  uint32
							LocalAddr string
							LocalPort uint32
						}

						err = ssh.Unmarshal(channel.ExtraData(), &directTCPIP)
						if err == nil {
							err = s.handleProxy(ctx, "tcp", channel, net.JoinHostPort(directTCPIP.DestAddr, strconv.FormatInt(int64(directTCPIP.DestPort), 10)))
						}

					case "direct-streamlocal@openssh.com":
						var directStreamLocal struct {
							DestAddr  string
							LocalAddr string
							LocalPort uint32
						}

						err = ssh.Unmarshal(channel.ExtraData(), &directStreamLocal)
						if err == nil {
							err = s.handleProxy(ctx, "unix", channel, directStreamLocal.DestAddr)
						}

					default:
						err = channel.Reject(ssh.UnknownChannelType, fmt.Sprintf("%v: %v", ssh.UnknownChannelType, channel.ChannelType()))
					}

					s.setError(err)
				}(channel)
			}
		}()
	}
}

func (s *StubSSHServer) handleProxy(ctx context.Context, network string, channel ssh.NewChannel, addr string) error {
	dialer := net.Dialer{Timeout: 30 * time.Second}

	upstream, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return err
	}
	defer upstream.Close()

	conn, _, err := channel.Accept()
	if err != nil {
		return err
	}
	defer upstream.Close()

	recvCh := make(chan error, 1)
	sendCh := make(chan error, 1)
	go func() {
		recvCh <- copier(upstream, conn, "conn to upstream")
	}()

	go func() {
		err := copier(conn, upstream, "upstream to conn")
		if errors.Is(err, syscall.ENOTCONN) || errors.Is(err, io.EOF) {
			err = nil
		}
		sendCh <- err
	}()

	select {
	case err = <-recvCh:
		if err != nil {
			return err
		}
		err = <-sendCh
	case err = <-sendCh:
	}

	return err
}

//nolint:gocognit
func copier(to io.Writer, from io.Reader, desc string) (err error) {
	defer func() {
		if t, ok := from.(interface{ CloseRead() error }); ok {
			if cerr := t.CloseRead(); cerr != nil && err == nil {
				err = fmt.Errorf("close reader (%s): %w", desc, cerr)
			}
		}

		if t, ok := to.(interface{ CloseWrite() error }); ok {
			if cerr := t.CloseWrite(); cerr != nil && err == nil {
				err = fmt.Errorf("close writer (%s): %w", desc, cerr)
			}
		}
	}()

	if _, err := io.Copy(to, from); err != nil {
		return fmt.Errorf("copy (%s): %w", desc, err)
	}

	return nil
}

//nolint:gocognit,funlen
func (s *StubSSHServer) handleSession(ctx context.Context, channel ssh.NewChannel) error {
	conn, reqs, err := channel.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	for req := range reqs {
		switch req.Type {
		case "exec":
			if req.WantReply {
				if err := req.Reply(true, nil); err != nil {
					return err
				}
			}

			var command struct {
				Value []byte
			}
			if err := ssh.Unmarshal(req.Payload, &command); err != nil {
				return fmt.Errorf("session unmarshal: %w", err)
			}

			// this is in place of a proper shlex implementation, but should probably work
			// for all of our use-cases.
			args := strings.Split(string(command.Value), " ")

			if ctx.Err() != nil {
				return ctx.Err()
			}

			cmd := exec.CommandContext(ctx, args[0], args[1:]...)
			cmd.Dir = s.tempDir
			cmd.Stdout = conn
			cmd.Stderr = conn
			cmd.Stdin = conn

			runErr := runCmd(cmd)
			if ctx.Err() != nil {
				return ctx.Err()
			}

			var exitError *exec.ExitError
			code := 0
			if errors.As(err, &exitError) {
				code = exitError.ExitCode()
			}

			var exit [4]byte
			binary.BigEndian.PutUint32(exit[:], uint32(code))

			if err := conn.CloseWrite(); err != nil {
				return err
			}
			if _, err := conn.SendRequest("exit-status", false, exit[:]); err != nil {
				return err
			}
			return runErr

		default:
			return fmt.Errorf("unknown request type: %s", req.Type)
		}
	}

	return nil
}

func (s *StubSSHServer) Client() Client {
	return Client{
		SshConfig: common.SshConfig{
			User:         s.User,
			Password:     s.Password,
			Host:         "127.0.0.1",
			Port:         s.port,
			IdentityFile: s.privateKeyLocation,
		},
	}
}
