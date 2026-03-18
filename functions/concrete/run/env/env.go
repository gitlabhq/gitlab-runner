package env

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type JobStatus string

const (
	Running  = JobStatus("running")
	Success  = JobStatus("success")
	Failed   = JobStatus("failed")
	Timedout = JobStatus("timedout")
	Canceled = JobStatus("canceled")
)

type Env struct {
	ID      int64
	Token   string
	BaseURL string

	WorkingDir string
	CacheDir   string
	StagingDir string
	Shell      string
	Timeout    time.Duration
	LoginShell bool

	Env map[string]string

	GitLabEnvFile string
	GitLabEnv     map[string]string

	Stdout io.Writer
	Stderr io.Writer

	status JobStatus
}

func (e *Env) IsSuccessful() bool {
	switch e.status {
	case "", Running, Success:
		return true
	default:
		return false
	}
}

func (e *Env) SetStatus(status JobStatus) {
	e.status = status
	e.Env["CI_JOB_STATUS"] = string(status)
}

func (e *Env) Printf(format string, a ...any) {
	fmt.Fprintf(e.Stdout, "\x1b[0;m%s\x1b[0;m\n", fmt.Sprintf(format, a...))
}

func (e *Env) Noticef(format string, a ...any) {
	fmt.Fprintf(e.Stderr, "\x1b[32;1m%s\x1b[0;m\n", fmt.Sprintf(format, a...))
}

func (e *Env) Warningf(format string, a ...any) {
	fmt.Fprintf(e.Stderr, "\x1b[0;33m%s\x1b[0;m\n", fmt.Sprintf(format, a...))
}

func (e *Env) Debugf(format string, a ...any) {
	fmt.Fprintf(e.Stderr, "\x1b[32;1m%s\x1b[0;m\n", fmt.Sprintf(format, a...))
}

func (e *Env) getRunnerBinaryPath() string {
	if cmd, err := exec.LookPath("gitlab-runner"); err == nil {
		return cmd
	}
	if cmd, err := exec.LookPath("gitlab-runner-helper"); err == nil {
		return cmd
	}

	// use current executable, but skip if it looks like the executable is a
	// Go test binary
	if cmd, err := os.Executable(); err == nil && !strings.HasSuffix(cmd, ".test") {
		return cmd
	}

	return "gitlab-runner"
}

func (e *Env) RunnerCommand(ctx context.Context, extra map[string]string, args ...string) error {
	return e.Command(ctx, e.getRunnerBinaryPath(), extra, args...)
}

func (e *Env) Command(ctx context.Context, name string, env map[string]string, args ...string) error {
	environ := os.Environ()
	for k, v := range e.Env {
		environ = append(environ, k+"="+v)
	}
	for k, v := range e.GitLabEnv {
		environ = append(environ, k+"="+v)
	}
	for k, v := range env {
		environ = append(environ, k+"="+v)
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = e.WorkingDir
	cmd.Env = environ
	cmd.Stdout = e.Stdout
	cmd.Stderr = e.Stderr

	return cmd.Run()
}
