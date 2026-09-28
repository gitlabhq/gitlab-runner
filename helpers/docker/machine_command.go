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

const logFormatJSONOption = "--log-format=json"

var errNotJSON = errors.New("line is not a JSON object")

// The returned entry is always usable; on error it holds the raw line at the
// fallback level.
func parseLogLine(line string, fallback logrus.Level) (LogEntry, error) {
	entry := LogEntry{Time: time.Now(), Level: fallback, Msg: line}
	if !strings.HasPrefix(line, "{") {
		return entry, errNotJSON
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return entry, err
	}
	msg, ok := raw["msg"].(string)
	if !ok {
		return entry, errors.New("missing msg field")
	}
	entry.Msg = msg
	delete(raw, "msg")

	var errs []error
	if lvl, ok := raw["level"].(string); ok {
		if parsed, err := logrus.ParseLevel(lvl); err == nil {
			entry.Level = parsed
		} else {
			errs = append(errs, err)
		}
		delete(raw, "level")
	}
	if ts, ok := raw["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			entry.Time = t
		} else {
			errs = append(errs, err)
		}
		delete(raw, "time")
	}
	entry.Fields = raw
	return entry, errors.Join(errs...)
}

// logWriter logs command output line by line. Assigned to Stdout/Stderr
// rather than read from pipes: exec.Cmd waits for the copy, so nothing
// races process exit or gets lost.
type logWriter struct {
	logger *logrus.Entry
	level  logrus.Level
	output MachineOutput
	buf    bytes.Buffer

	// A stack trace is many lines; the first parse failure is a warning,
	// the rest are debug.
	parseFailures int
}

func (l *logWriter) write(line string) {
	line = strings.TrimRight(line, "\n")

	if line == "" {
		return
	}

	entry := LogEntry{Time: time.Now(), Level: l.level, Msg: line}
	if l.output.JSONLogFormat {
		var err error
		entry, err = parseLogLine(line, l.level)
		if err != nil {
			level := logrus.DebugLevel
			if l.parseFailures == 0 {
				level = logrus.WarnLevel
			}
			l.parseFailures++
			l.logger.WithError(err).WithField("line", line).Log(level, "Could not parse docker-machine JSON output")
		}
	}

	logger := l.logger
	if len(entry.Fields) > 0 {
		logger = logger.WithField("docker_machine", entry.Fields)
	}
	logger.Log(entry.Level, entry.Msg)

	if l.output.OnEntry != nil {
		l.output.OnEntry(entry)
	}
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

func newLogWriter(fields logrus.Fields, level logrus.Level, output MachineOutput) *logWriter {
	return &logWriter{
		logger: logrus.WithFields(fields),
		level:  level,
		output: output,
	}
}

func stdoutLogWriter(cmd *exec.Cmd, fields logrus.Fields, output MachineOutput) *logWriter {
	writer := newLogWriter(fields, logrus.InfoLevel, output)
	cmd.Stdout = writer
	return writer
}

func stderrLogWriter(cmd *exec.Cmd, fields logrus.Fields, output MachineOutput) *logWriter {
	writer := newLogWriter(fields, logrus.ErrorLevel, output)
	cmd.Stderr = writer
	return writer
}

func runWithLogs(cmd *exec.Cmd, fields logrus.Fields, output MachineOutput) error {
	stdout := stdoutLogWriter(cmd, fields, output)
	stderr := stderrLogWriter(cmd, fields, output)
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

func (m *machineCommand) Create(
	ctx context.Context,
	driver, name string,
	output MachineOutput,
	opts ...string,
) error {
	var args []string
	if output.JSONLogFormat {
		args = append(args, logFormatJSONOption)
	}
	args = append(args, "create", "--driver", driver)
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
	return runWithLogs(cmd, fields, output)
}

func (m *machineCommand) Provision(ctx context.Context, name string) error {
	cmd := newDockerMachineCommand(ctx, "provision", name)

	fields := logrus.Fields{
		"operation": "provision",
		"name":      name,
	}

	return runWithLogs(cmd, fields, MachineOutput{})
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

	stdout := stdoutLogWriter(cmd, fields, MachineOutput{})
	stderr := stderrLogWriter(cmd, fields, MachineOutput{})

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

	return runWithLogs(cmd, fields, MachineOutput{})
}

func (m *machineCommand) Remove(ctx context.Context, name string) error {
	cmd := newDockerMachineCommand(ctx, "rm", "-y", name)

	fields := logrus.Fields{
		"operation": "remove",
		"name":      name,
	}

	if err := runWithLogs(cmd, fields, MachineOutput{}); err != nil {
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

	if err := runWithLogs(cmd, fields, MachineOutput{}); err != nil {
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

// machineAuthOptions is the part of docker-machine's AuthOptions the runner
// needs. ServerName is empty for machines provisioned over SSH.
type machineAuthOptions struct {
	StorePath  string
	ServerName string
}

func (m *machineCommand) authOptions(ctx context.Context, name string) (machineAuthOptions, error) {
	out, err := m.get(ctx, "inspect", name, "-f", "{{json .HostOptions.AuthOptions}}")
	if err != nil {
		return machineAuthOptions{}, err
	}
	return parseMachineAuthOptions(out)
}

func parseMachineAuthOptions(out string) (machineAuthOptions, error) {
	var opts machineAuthOptions
	if err := json.Unmarshal([]byte(out), &opts); err != nil {
		return machineAuthOptions{}, fmt.Errorf("parsing docker-machine auth options: %w", err)
	}
	if opts.StorePath == "" {
		return machineAuthOptions{}, errors.New("docker-machine auth options have no StorePath")
	}
	return opts, nil
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
	stderr := stderrLogWriter(cmd, fields, MachineOutput{})

	err = cmd.Run()
	stderr.Flush()
	if err == nil {
		return true
	}

	// Only a completed inspect (exit 1) says the machine is gone. A
	// timeout, signal, or crash says nothing, and false here skips removal.
	var exitErr *exec.ExitError
	if ctx.Err() == nil && errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false
	}

	logrus.WithFields(fields).WithError(err).
		Warningln("docker-machine inspect did not complete; assuming the machine exists")
	return true
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
	if err != nil {
		return
	}

	auth, err := m.authOptions(ctx, name)
	if err != nil {
		return
	}
	dc.CertPath = auth.StorePath
	dc.TLSServerName = auth.ServerName
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
	setDockerMachineProcessAttributes(cmd)

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
