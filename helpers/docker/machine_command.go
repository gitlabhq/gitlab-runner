package docker

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/machine/commands/mcndirs"
	"github.com/sirupsen/logrus"
)

const (
	defaultDockerMachineExecutable = "docker-machine"
	crashreportTokenOption         = "--bugsnag-api-token"
	crashreportToken               = "no-report"
)

var dockerMachineExecutable = defaultDockerMachineExecutable

type logWriter struct {
	log    func(args ...interface{})
	reader *bufio.Reader
}

func (l *logWriter) write(line string) {
	line = strings.TrimRight(line, "\n")

	if line == "" {
		return
	}

	l.log(line)
}

func (l *logWriter) watch() {
	var err error
	for err != io.EOF {
		var line string
		line, err = l.reader.ReadString('\n')
		if err != nil && err != io.EOF {
			if !strings.Contains(err.Error(), "bad file descriptor") {
				logrus.WithError(err).Warn("Problem while reading command output")
			}
			return
		}

		l.write(line)
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
	cache     map[string]machineInfo
	cacheLock sync.RWMutex
}

type machineInfo struct {
	expires time.Time

	canConnect bool
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

	cmd := newDockerMachineCommand(args...)

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
	cmd := newDockerMachineCommand("provision", name)

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

	cmd := newDockerMachineCommandCtx(ctx, "stop", name)

	fields := logrus.Fields{
		"operation": "stop",
		"name":      name,
	}
	stdoutLogWriter(cmd, fields)
	stderrLogWriter(cmd, fields)

	return cmd.Run()
}

func (m *machineCommand) Remove(name string) error {
	cmd := newDockerMachineCommand("rm", "-y", name)

	fields := logrus.Fields{
		"operation": "remove",
		"name":      name,
	}
	stdoutLogWriter(cmd, fields)
	stderrLogWriter(cmd, fields)

	if err := cmd.Run(); err != nil {
		return err
	}

	m.cacheLock.Lock()
	delete(m.cache, name)
	m.cacheLock.Unlock()
	return nil
}

func (m *machineCommand) List() (hostNames []string, err error) {
	dir, err := os.ReadDir(mcndirs.GetMachineDir())
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
	cmd := newDockerMachineCommand(args...)

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

	cmd := newDockerMachineCommand("inspect", name)

	fields := logrus.Fields{
		"operation": "exists",
		"name":      name,
	}
	stderrLogWriter(cmd, fields)

	return cmd.Run() == nil
}

func (m *machineCommand) CanConnect(name string, skipCache bool) bool {
	m.cacheLock.RLock()
	cachedInfo, ok := m.cache[name]
	m.cacheLock.RUnlock()

	if ok && !skipCache && time.Now().Before(cachedInfo.expires) {
		return cachedInfo.canConnect
	}

	canConnect := m.canConnect(name)
	if !canConnect {
		return false // we only cache positive hits. Machines usually do not disconnect.
	}

	m.cacheLock.Lock()
	m.cache[name] = machineInfo{
		expires:    time.Now().Add(5 * time.Minute),
		canConnect: true,
	}
	m.cacheLock.Unlock()
	return true
}

func (m *machineCommand) canConnect(name string) bool {
	// Execute docker-machine config which actively ask the machine if it is up and online
	cmd := newDockerMachineCommand("config", name)

	err := cmd.Run()
	return err == nil
}

func (m *machineCommand) Credentials(name string) (dc Credentials, err error) {
	if !m.CanConnect(name, true) {
		err = errors.New("can't connect")
		return
	}

	dc.TLSVerify = true
	dc.Host, err = m.URL(name)
	if err == nil {
		dc.CertPath, err = m.CertPath(name)
	}
	return
}

func newDockerMachineCommandCtx(ctx context.Context, args ...string) *exec.Cmd {
	token := os.Getenv("MACHINE_BUGSNAG_API_TOKEN")
	if token == "" {
		token = crashreportToken
	}

	commandArgs := []string{
		fmt.Sprintf("%s=%s", crashreportTokenOption, token),
	}
	commandArgs = append(commandArgs, args...)

	cmd := exec.CommandContext(ctx, dockerMachineExecutable, commandArgs...)
	cmd.Env = os.Environ()

	return cmd
}

func newDockerMachineCommand(args ...string) *exec.Cmd {
	return newDockerMachineCommandCtx(context.Background(), args...)
}

func NewMachineCommand() Machine {
	return &machineCommand{
		cache: map[string]machineInfo{},
	}
}
