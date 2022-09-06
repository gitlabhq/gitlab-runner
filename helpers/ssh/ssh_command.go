package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Client struct {
	Config

	Stdout         io.Writer
	Stderr         io.Writer
	ConnectRetries int

	client *ssh.Client
}

type Command struct {
	Command string
	Stdin   string
}

type ExitError struct {
	Inner error
}

func (e *ExitError) Error() string {
	if e.Inner == nil {
		return "error"
	}
	return e.Inner.Error()
}

func (e *ExitError) ExitCode() int {
	var cryptoExitError *ssh.ExitError
	if errors.As(e.Inner, &cryptoExitError) {
		return cryptoExitError.ExitStatus()
	}
	return 0
}

func (s *Client) getSSHKey(identityFile string) (key ssh.Signer, err error) {
	buf, err := os.ReadFile(identityFile)
	if err != nil {
		return nil, err
	}
	key, err = ssh.ParsePrivateKey(buf)
	return key, err
}

func (s *Client) getSSHAuthMethods() ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	methods = append(methods, ssh.Password(s.Password))

	if s.IdentityFile != "" {
		key, err := s.getSSHKey(s.IdentityFile)
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeys(key))
	}

	return methods, nil
}

func getHostKeyCallback(config Config) (ssh.HostKeyCallback, error) {
	if config.ShouldDisableStrictHostKeyChecking() {
		return ssh.InsecureIgnoreHostKey(), nil
	}

	if config.KnownHostsFile == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("user home directory: %w", err)
		}

		config.KnownHostsFile = filepath.Join(homeDir, ".ssh", "known_hosts")
	}

	return knownhosts.New(config.KnownHostsFile)
}

func (s *Client) Connect() error {
	if s.Host == "" {
		s.Host = "localhost"
	}
	if s.User == "" {
		s.User = "root"
	}
	if s.Port == "" {
		s.Port = "22"
	}

	methods, err := s.getSSHAuthMethods()
	if err != nil {
		return fmt.Errorf("getting SSH authentication methods: %w", err)
	}

	config := &ssh.ClientConfig{
		User: s.User,
		Auth: methods,
	}

	hostKeyCallback, err := getHostKeyCallback(s.Config)
	if err != nil {
		return fmt.Errorf("getting host key callback: %w", err)
	}
	config.HostKeyCallback = hostKeyCallback

	connectRetries := s.ConnectRetries
	if connectRetries == 0 {
		connectRetries = 3
	}

	var finalError error

	for i := 0; i < connectRetries; i++ {
		client, err := ssh.Dial("tcp", s.Host+":"+s.Port, config)
		if err == nil {
			s.client = client
			return nil
		}

		time.Sleep(sshRetryInterval * time.Second)
		finalError = fmt.Errorf("ssh Dial() error: %w", err)
	}

	return finalError
}

func (s *Client) Exec(cmd string) error {
	if s.client == nil {
		return errors.New("not connected")
	}

	session, err := s.client.NewSession()
	if err != nil {
		return err
	}
	session.Stdout = s.Stdout
	session.Stderr = s.Stderr
	err = session.Run(cmd)
	_ = session.Close()
	return err
}

func (s *Client) Run(ctx context.Context, cmd Command) error {
	if s.client == nil {
		return errors.New("not connected")
	}

	session, err := s.client.NewSession()
	if err != nil {
		return err
	}
	defer func() { _ = session.Close() }()

	session.Stdin = strings.NewReader(cmd.Stdin)
	session.Stdout = s.Stdout
	session.Stderr = s.Stderr
	err = session.Start(cmd.Command)
	if err != nil {
		return err
	}

	waitCh := make(chan error)
	go func() {
		err := session.Wait()
		if _, ok := err.(*ssh.ExitError); ok {
			err = &ExitError{Inner: err}
		}
		waitCh <- err
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		_ = session.Close()
		return <-waitCh

	case err := <-waitCh:
		return err
	}
}

func (s *Client) Cleanup() {
	if s.client != nil {
		_ = s.client.Close()
	}
}
