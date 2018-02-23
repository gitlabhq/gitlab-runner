package docker_helpers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/machine/commands/mcndirs"
	"github.com/sirupsen/logrus"
)

type logWriter struct {
	log    func(args ...interface{})
	reader *bufio.Reader
}

func (l *logWriter) write(line string) {
	line = strings.TrimRight(line, "\n")

	if len(line) <= 0 {
		return
	}

	l.log(line)
}

func (l *logWriter) watch() {
	for {
		line, err := l.reader.ReadString('\n')
		if err == nil || err == io.EOF {
			l.write(line)
			if err == io.EOF {
				return
			}
		} else {
			if !strings.Contains(err.Error(), "bad file descriptor") {
				logrus.WithError(err).Errorln("Problem while reading command output")
			}
			return
		}
	}
}

func newLogWriter(logFunction func(args ...interface{}), reader io.Reader) {
	writer := &logWriter{
		log:    logFunction,
		reader: bufio.NewReader(reader),
	}

	go writer.watch()
}

func stdoutLogWriter(cmd *exec.Cmd, fields logrus.Fields) {
	log := logrus.WithFields(fields)
	reader, err := cmd.StdoutPipe()

	if err == nil {
		newLogWriter(log.Infoln, reader)
	}
}

func stderrLogWriter(cmd *exec.Cmd, fields logrus.Fields) {
	log := logrus.WithFields(fields)
	reader, err := cmd.StderrPipe()

	if err == nil {
		newLogWriter(log.Errorln, reader)
	}
}

type machineCommand struct {
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
		"driver":    driver,
		"name":      name,
	}
	stdoutLogWriter(cmd, fields)
	stderrLogWriter(cmd, fields)

	logrus.Debugln("Executing", cmd.Path, cmd.Args)
	return cmd.Run()
}

func (m *machineCommand) Provision(name string) error {
	cmd := exec.Command("docker-machine", "provision", name)
	cmd.Env = os.Environ()

	fields := logrus.Fields{
		"operation": "provision",
		"name":      name,
	}
	stdoutLogWriter(cmd, fields)
	stderrLogWriter(cmd, fields)

	return cmd.Run()
}

func (m *machineCommand) Stop(name string, timeout time.Duration) error {
	ctx, ctxCancelFn := context.WithTimeout(context.Background(), timeout)
	defer ctxCancelFn()

	cmd := exec.CommandContext(ctx, "docker-machine", "stop", name)
	cmd.Env = os.Environ()

	fields := logrus.Fields{
		"operation": "stop",
		"name":      name,
	}
	stdoutLogWriter(cmd, fields)
	stderrLogWriter(cmd, fields)

	return cmd.Run()
}

func (m *machineCommand) Remove(name string) error {
	cmd := exec.Command("docker-machine", "rm", "-y", name)
	cmd.Env = os.Environ()

	fields := logrus.Fields{
		"operation": "remove",
		"name":      name,
	}
	stdoutLogWriter(cmd, fields)
	stderrLogWriter(cmd, fields)

	return cmd.Run()
}

func (m *machineCommand) List() (hostNames []string, err error) {
	dir, err := ioutil.ReadDir(mcndirs.GetMachineDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, file := range dir {
		if file.IsDir() && !strings.HasPrefix(file.Name(), ".") {
			hostNames = append(hostNames, file.Name())
		}
	}

	return
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
	configPath := filepath.Join(mcndirs.GetMachineDir(), name, "config.json")
	_, err := os.Stat(configPath)
	if err != nil {
		return false
	}

	cmd := exec.Command("docker-machine", "inspect", name)
	cmd.Env = os.Environ()

	fields := logrus.Fields{
		"operation": "exists",
		"name":      name,
	}
	stderrLogWriter(cmd, fields)

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
