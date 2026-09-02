package docker

import (
	"bytes"
	"context"
	"encoding/json"
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

// logWriter logs command output line by line. Assigned to Stdout/Stderr
// rather than read from pipes: exec.Cmd waits for the copy, so nothing
// races process exit or gets lost.
type logWriter struct {
	log func(args ...any)
	buf bytes.Buffer
}

func (l *logWriter) write(line string) {
	line = strings.TrimRight(line, "\n")

	if line == "" {
		return
	}

	l.log(line)
}

func (l *logWriter) Write(p []byte) (int, error) {
	l.buf.Write(p)
	for {
		i := bytes.IndexByte(l.buf.Bytes(), '\n')
		if i < 0 {
			break
		}
		l.write(string(l.buf.Next(i + 1)))
	}
	return len(p), nil
}

// Flush logs a trailing line that ended without a newline.
func (l *logWriter) Flush() {
	if l.buf.Len() > 0 {
		l.write(l.buf.String())
		l.buf.Reset()
	}
}

func stdoutLogWriter(cmd *exec.Cmd, fields logrus.Fields) *logWriter {
	writer := &logWriter{log: logrus.WithFields(fields).Infoln}
	cmd.Stdout = writer
	return writer
}

func stderrLogWriter(cmd *exec.Cmd, fields logrus.Fields) *logWriter {
	writer := &logWriter{log: logrus.WithFields(fields).Errorln}
	cmd.Stderr = writer
	return writer
}

func runWithLogs(cmd *exec.Cmd, fields logrus.Fields) error {
	stdout := stdoutLogWriter(cmd, fields)
	stderr := stderrLogWriter(cmd, fields)
	err := cmd.Run()
	stdout.Flush()
	stderr.Flush()
	return err
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
	logrus.Debugln("Executing", cmd.Path, cmd.Args)
	return runWithLogs(cmd, fields)
}

func (m *machineCommand) Provision(ctx context.Context, name string) error {
	cmd := newDockerMachineCommand(ctx, "provision", name)

	fields := logrus.Fields{
		"operation": "provision",
		"name":      name,
	}

	return runWithLogs(cmd, fields)
}

func (m *machineCommand) UpdateLabels(ctx context.Context, name string, labels map[string]string) error {
	args := []string{"update-labels"}
	for k, v := range labels {
		args = append(args, "--label", k+"="+v)
	}
	args = append(args, name)

	cmd := newDockerMachineCommand(ctx, args...)

	fields := logrus.Fields{
		"operation": "update-labels",
		"name":      name,
	}

	stdout := stdoutLogWriter(cmd, fields)
	stderr := stderrLogWriter(cmd, fields)

	// stderr is also captured to recover the unsupported-driver error,
	// since docker-machine is a subprocess.
	var captured bytes.Buffer
	cmd.Stderr = io.MultiWriter(stderr, &captured)

	err := cmd.Run()
	stdout.Flush()
	stderr.Flush()

	if err != nil {
		if strings.Contains(captured.String(), ErrLabelsNotSupported.Error()) {
			return ErrLabelsNotSupported
		}

		return fmt.Errorf("update-labels: %w: %s", err, strings.TrimSpace(captured.String()))
	}

	return nil
}

func (m *machineCommand) Stop(ctx context.Context, name string) error {
	cmd := newDockerMachineCommand(ctx, "stop", name)

	fields := logrus.Fields{
		"operation": "stop",
		"name":      name,
	}

	return runWithLogs(cmd, fields)
}

func (m *machineCommand) Remove(ctx context.Context, name string) error {
	cmd := newDockerMachineCommand(ctx, "rm", "-y", name)

	fields := logrus.Fields{
		"operation": "remove",
		"name":      name,
	}

	if err := runWithLogs(cmd, fields); err != nil {
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

	if err := runWithLogs(cmd, fields); err != nil {
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
	stderr := stderrLogWriter(cmd, fields)

	err = cmd.Run()
	stderr.Flush()
	return err == nil
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

// Inspect reads <machineDir>/<name>/config.json — the file docker-machine
// writes during Create — and pulls out the subset of driver state we use
// for metric labelling. Driver-specific fields are only populated for
// drivers with an explicit schema (currently just "google"). Other
// drivers return DriverName only.
func (m *machineCommand) Inspect(name string) (MachineInfo, error) {
	path := filepath.Join(getMachineDir(), name, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return MachineInfo{}, fmt.Errorf("reading docker-machine state for %q: %w", name, err)
	}

	// Decode driver fields in a second pass so we can choose the schema
	// based on DriverName.
	var raw struct {
		DriverName string          `json:"DriverName"`
		Driver     json.RawMessage `json:"Driver"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return MachineInfo{}, fmt.Errorf("parsing docker-machine state for %q: %w", name, err)
	}

	info := MachineInfo{DriverName: raw.DriverName}

	if raw.DriverName == "google" {
		if err := applyGoogleDriverInfo(&info, raw.Driver); err != nil {
			return MachineInfo{}, fmt.Errorf("parsing google driver state for %q: %w", name, err)
		}
	}

	return info, nil
}

// googleDriverState is the subset of the google driver's persisted
// state the Inspect parser cares about. Any non-zero mode signal
// (InstanceGroupManager, RegionInstanceGroupManager, BulkInsert) marks
// a path where GCP picks placement post-hoc — only Resolved* fields are
// honest then; otherwise operator-intent Zone / MachineType are reality.
type googleDriverState struct {
	Zone                       string `json:"Zone"`
	MachineType                string `json:"MachineType"`
	Project                    string `json:"Project"`
	ResolvedZone               string `json:"ResolvedZone"`
	ResolvedMachineType        string `json:"ResolvedMachineType"`
	InstanceGroupManager       string `json:"InstanceGroupManager"`
	RegionInstanceGroupManager string `json:"RegionInstanceGroupManager"`
	BulkInsert                 bool   `json:"BulkInsert"`
}

func (s googleDriverState) gcpPicksPlacement() bool {
	return s.InstanceGroupManager != "" || s.RegionInstanceGroupManager != "" || s.BulkInsert
}

func applyGoogleDriverInfo(info *MachineInfo, rawDriver json.RawMessage) error {
	var s googleDriverState
	if len(rawDriver) > 0 {
		if err := json.Unmarshal(rawDriver, &s); err != nil {
			return err
		}
	}
	info.Project = s.Project
	info.Zone = s.Zone
	info.MachineType = s.MachineType
	if s.gcpPicksPlacement() {
		info.Zone = s.ResolvedZone
		info.MachineType = s.ResolvedMachineType
	}
	return nil
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
