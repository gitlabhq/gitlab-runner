package env

import (
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

	"gitlab.com/gitlab-org/step-runner/pkg/runner/gracefulexitcmd"
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

	// GracefulExitDelay bounds the time between cancellation and forced
	// pipe-close when a script is terminated. See gracefulexitcmd.New.
	GracefulExitDelay time.Duration

	Env map[string]string

	GitLabEnvFile string
	GitLabEnv     map[string]string

	Stdout io.Writer
	Stderr io.Writer

	status JobStatus

	resolveBundleOnce sync.Once
	bundledGit        string
	bundledCACerts    string
}

// ExpandValue expands $VAR / ${VAR} against Env + GitLabEnv overlay.
// Used for fields the helper subprocess does not expand itself.
func (e *Env) ExpandValue(s string) string {
	if s == "" {
		return s
	}
	return os.Expand(s, func(key string) string {
		if v, ok := e.GitLabEnv[key]; ok {
			return v
		}
		return e.Env[key]
	})
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

	cmd := gracefulexitcmd.New(ctx, e.GracefulExitDelay, name, args...)
	cmd.Dir = e.WorkingDir
	cmd.Env = environ
	cmd.Stdout = e.Stdout
	cmd.Stderr = e.Stderr

	return normalizeExitError(cmd.Run(), cmd.ProcessState)
}

// normalizeExitError reclassifies two exec outcomes that gracefulexitcmd
// surfaces as errors but which the runner's legacy bash-pipe execution
// (shells/bash.go on the docker executor) effectively treats as success:
//
//  1. The script exited 0, but a backgrounded child outlived
//     gracefulexitcmd's WaitDelay holding the parent's stdio pipes
//     open. WaitDelay's job is to bound that drain, not to fail the
//     job; the exit code already says the user script was fine.
//
//  2. The script's outer shell was terminated by a non-fatal
//     user-defined signal (SIGUSR1, SIGUSR2, SIGHUP, SIGPIPE). These
//     are routinely raised by user scripts that signal themselves
//     (e.g. `kill -USR1 $$`) and the bash pipeline wrapping in
//     functions/concrete/run/stages/internal/scriptwriter delivers
//     the signal to the outer shell rather than the subshell that
//     installs the trap, so what looks like a "script failure" here
//     is actually expected behaviour. Surfacing these as failures
//     diverges from the legacy executor without offering a recovery
//     path inside the user's script.
//
// Cancellation-driven SIGTERM (from gracefulexitcmd.Cmd.Cancel) is
// deliberately NOT included: the runner needs that to propagate so
// the build is reported as canceled rather than passed silently.
func normalizeExitError(err error, ps *os.ProcessState) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, exec.ErrWaitDelay) && ps != nil && ps.ExitCode() == 0 {
		return nil
	}

	if ps != nil && isNonFatalUserSignal(ps) {
		return nil
	}

	return err
}

func (e *Env) BundledGit() string {
	e.resolveBundle()

	return e.bundledGit
}

// HelperEnvs returns environment variables needed for bundled TLS support.
// It sets SSL_CERT_FILE to the bundled CA certs (if available and not
// already set), and prepends the bundled git to PATH. Returns nil if
// nothing needs to be added.
func (e *Env) HelperEnvs(existing map[string]string) map[string]string {
	e.resolveBundle()

	env := make(map[string]string)
	for k, v := range existing {
		env[k] = v
	}

	if e.bundledCACerts != "" {
		if _, ok := env["SSL_CERT_FILE"]; !ok {
			env["SSL_CERT_FILE"] = e.bundledCACerts
		}
		// libcurl (used by bundled git) ignores SSL_CERT_FILE; GIT_SSL_CAINFO
		// is what it honors. Per-host http.<host>.sslCAInfo from
		// CI_SERVER_TLS_CA_FILE still takes precedence.
		if _, ok := env["GIT_SSL_CAINFO"]; !ok {
			env["GIT_SSL_CAINFO"] = e.bundledCACerts
		}
	}

	if e.bundledGit != "git" {
		gitBinDir := filepath.Dir(e.bundledGit)
		if path, ok := env["PATH"]; ok {
			env["PATH"] = gitBinDir + ":" + path
		} else {
			env["PATH"] = gitBinDir + ":" + os.Getenv("PATH")
		}
	}

	return env
}

func (e *Env) resolveBundle() {
	e.resolveBundleOnce.Do(func() {
		e.bundledGit = "git"

		exe, err := os.Executable()
		if err != nil {
			return
		}

		exe, _ = filepath.EvalSymlinks(exe)
		baseDir := filepath.Dir(exe)

		candidate := filepath.Join(baseDir, "git", "bin", "git")
		if _, err := os.Stat(candidate); err == nil {
			e.bundledGit = candidate
		}

		candidate = filepath.Join(baseDir, "ca-certs.pem")
		if _, err := os.Stat(candidate); err == nil {
			e.bundledCACerts = candidate
		}
	})
}
