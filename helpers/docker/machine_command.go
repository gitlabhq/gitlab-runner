package docker_helpers

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
)

type logWriter struct {
	log     *logrus.Entry
	isError bool
}

func (l *logWriter) Write(data []byte) (int, error) {
	row := strings.TrimRight(string(data), "\n")

	if l.isError {
		l.log.Error(row)
	} else {
		l.log.Info(row)
	}

	return len(data), nil
}

func newLogWriter(isError bool, fields logrus.Fields) *logWriter {
	return &logWriter{
		log: logrus.WithFields(fields),
		isError: isError,
	}
}

type machineCommand struct {
	lsCmd   *exec.Cmd
	lsLock  sync.Mutex
	lsCond  *sync.Cond
	lsData  []byte
	lsError error
}

func (m *machineCommand) ls() (data []byte, err error) {
	m.lsLock.Lock()
	defer m.lsLock.Unlock()

	if m.lsCond == nil {
		m.lsCond = sync.NewCond(&m.lsLock)
	}

	if m.lsCmd == nil {
		m.lsCmd = exec.Command("docker-machine", "ls", "-q")
		m.lsCmd.Env = os.Environ()
		go func() {
			m.lsData, m.lsError = m.lsCmd.Output()
			m.lsCmd = nil
			m.lsCond.Broadcast()
		}()
	}

	m.lsCond.Wait()

	return m.lsData, m.lsError
}

func (m *machineCommand) Create(driver, name string, opts ...string) error {
	args := []string{
		"create",
		"--driver", driver,
	}
	for _, opt := range opts {
		args = append(args, "--"+opt)
	}
	args = append(args, name)

	cmd := exec.Command("docker-machine", args...)
	cmd.Env = os.Environ()

	fields := logrus.Fields{
		"operation": "create",
		"driver": driver,
		"name": name,
	}
	cmd.Stdout = newLogWriter(false, fields)
	cmd.Stderr = newLogWriter(true, fields)

	logrus.Debugln("Executing", cmd.Path, cmd.Args)
	return cmd.Run()
}

func (m *machineCommand) Provision(name string) error {
	cmd := exec.Command("docker-machine", "provision", name)
	cmd.Env = os.Environ()

	fields := logrus.Fields{
		"operation": "provision",
		"name": name,
	}
	cmd.Stdout = newLogWriter(false, fields)
	cmd.Stderr = newLogWriter(true, fields)

	return cmd.Run()
}

func (m *machineCommand) Remove(name string) error {
	cmd := exec.Command("docker-machine", "rm", "-y", name)
	cmd.Env = os.Environ()

	fields := logrus.Fields{
		"operation": "remove",
		"name": name,
	}
	cmd.Stdout = newLogWriter(false, fields)
	cmd.Stderr = newLogWriter(true, fields)

	return cmd.Run()
}

func (m *machineCommand) List(nodeFilter string) (machines []string, err error) {
	data, err := m.ls()
	if err != nil {
		return
	}

	reader := bufio.NewReader(bytes.NewReader(data))
	for {
		var line string

		line, err = reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var query string
		if n, _ := fmt.Sscanf(line, nodeFilter, &query); n != 1 {
			continue
		}

		machines = append(machines, line)
	}
}

func (m *machineCommand) get(args ...string) (out string, err error) {
	// Execute docker-machine to fetch IP
	cmd := exec.Command("docker-machine", args...)
	cmd.Env = os.Environ()
	data, err := cmd.Output()
	if err != nil {
		return
	}

	// Save the IP
	out = strings.TrimSpace(string(data))
	if out == "" {
		err = fmt.Errorf("failed to get %v", args)
	}
	return
}

func (m *machineCommand) IP(name string) (string, error) {
	return m.get("ip", name)
}

func (m *machineCommand) URL(name string) (string, error) {
	return m.get("url", name)
}

func (m *machineCommand) CertPath(name string) (string, error) {
	return m.get("inspect", name, "-f", "{{.HostOptions.AuthOptions.StorePath}}")
}

func (m *machineCommand) Status(name string) (string, error) {
	return m.get("status", name)
}

func (m *machineCommand) Exist(name string) bool {
	cmd := exec.Command("docker-machine", "inspect", name)
	cmd.Env = os.Environ()

	fields := logrus.Fields{
		"operation": "exists",
		"name": name,
	}
	cmd.Stderr = newLogWriter(true, fields)

	return cmd.Run() == nil
}

func (m *machineCommand) CanConnect(name string) bool {
	// Execute docker-machine config which actively ask the machine if it is up and online
	cmd := exec.Command("docker-machine", "config", name)
	cmd.Env = os.Environ()
	err := cmd.Run()
	if err == nil {
		return true
	}
	return false
}

func (m *machineCommand) Credentials(name string) (dc DockerCredentials, err error) {
	if !m.CanConnect(name) {
		err = errors.New("Can't connect")
		return
	}

	dc.TLSVerify = true
	dc.Host, err = m.URL(name)
	if err == nil {
		dc.CertPath, err = m.CertPath(name)
	}
	return
}

func NewMachineCommand() Machine {
	return &machineCommand{}
}
