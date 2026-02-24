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
	"runtime"
	"strings"
	"sync"
	"time"

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

func (m *machineCommand) Create(ctx context.Context, driver, name string, opts ...string) error {
	args := []string{
		"create",
		"--driver", driver,
	}
	for _, opt := range opts {
		args = append(args, "--"+opt)
	}
	args = append(args, name)

	cmd := newDockerMachineCommand(ctx, args...)

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

func (m *machineCommand) Provision(ctx context.Context, name string) error {
	cmd := newDockerMachineCommand(ctx, "provision", name)

	fields := logrus.Fields{
		"operation": "provision",
		"name":      name,
	}
	stdoutLogWriter(cmd, fields)
	stderrLogWriter(cmd, fields)

	return cmd.Run()
}

func (m *machineCommand) Stop(ctx context.Context, name string) error {
	cmd := newDockerMachineCommand(ctx, "stop", name)

	fields := logrus.Fields{
		"operation": "stop",
		"name":      name,
	}
	stdoutLogWriter(cmd, fields)
	stderrLogWriter(cmd, fields)

	return cmd.Run()
}

func (m *machineCommand) Remove(ctx context.Context, name string) error {
	cmd := newDockerMachineCommand(ctx, "rm", "-y", name)

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

func (m *machineCommand) ForceRemove(ctx context.Context, name string) error {
	cmd := newDockerMachineCommand(ctx, "rm", "-f", name)

	fields := logrus.Fields{
		"operation": "force-remove",
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
	dir, err := os.ReadDir(getMachineDir())
	if err != nil {
		errExist := err
		// On Windows, ReadDir() on a regular file will satisfy ErrNotExist,
		// due to this bug: https://github.com/golang/go/issues/46734
		//
		// For a workaround, we explicitly check whether the directory
		// exists or not with a Stat call.
		//nolint:goconst
		if runtime.GOOS == "windows" {
			_, errExist = os.Stat(getMachineDir())
		}
		if os.IsNotExist(errExist) {
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

func (m *machineCommand) get(ctx context.Context, args ...string) (out string, err error) {
	// Execute docker-machine to fetch IP
	cmd := newDockerMachineCommand(ctx, args...)

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

func (m *machineCommand) IP(ctx context.Context, name string) (string, error) {
	return m.get(ctx, "ip", name)
}

func (m *machineCommand) URL(ctx context.Context, name string) (string, error) {
	return m.get(ctx, "url", name)
}

func (m *machineCommand) CertPath(ctx context.Context, name string) (string, error) {
	return m.get(ctx, "inspect", name, "-f", "{{.HostOptions.AuthOptions.StorePath}}")
}

func (m *machineCommand) Status(ctx context.Context, name string) (string, error) {
	return m.get(ctx, "status", name)
}

func (m *machineCommand) Exist(ctx context.Context, name string) bool {
	configPath := filepath.Join(getMachineDir(), name, "config.json")
	_, err := os.Stat(configPath)
	if err != nil {
		return false
	}

	cmd := newDockerMachineCommand(ctx, "inspect", name)

	fields := logrus.Fields{
		"operation": "exists",
		"name":      name,
	}
	stderrLogWriter(cmd, fields)

	return cmd.Run() == nil
}

func (m *machineCommand) CanConnect(ctx context.Context, name string, skipCache bool) bool {
	m.cacheLock.RLock()
	cachedInfo, ok := m.cache[name]
	m.cacheLock.RUnlock()

	if ok && !skipCache && time.Now().Before(cachedInfo.expires) {
		return cachedInfo.canConnect
	}

	canConnect := m.canConnect(ctx, name)
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

func (m *machineCommand) canConnect(ctx context.Context, name string) bool {
	// Execute docker-machine config which actively ask the machine if it is up and online
	cmd := newDockerMachineCommand(ctx, "config", name)

	err := cmd.Run()
	return err == nil
}

func (m *machineCommand) Credentials(ctx context.Context, name string) (dc Credentials, err error) {
	if !m.CanConnect(ctx, name, true) {
		err = errors.New("can't connect")
		return
	}

	dc.TLSVerify = true
	dc.Host, err = m.URL(ctx, name)
	if err == nil {
		dc.CertPath, err = m.CertPath(ctx, name)
	}
	return
}

func newDockerMachineCommand(ctx context.Context, args ...string) *exec.Cmd {
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

func getBaseDir() string {
	homeDir := os.Getenv("HOME")
	if runtime.GOOS == "windows" {
		homeDir = os.Getenv("USERPROFILE")
	}

	baseDir := os.Getenv("MACHINE_STORAGE_PATH")
	if baseDir == "" {
		baseDir = filepath.Join(homeDir, ".docker", "machine")
	}

	return baseDir
}

func getMachineDir() string {
	return filepath.Join(getBaseDir(), "machines")
}

func NewMachineCommand() Machine {
	return &machineCommand{
		cache: map[string]machineInfo{},
	}
}
