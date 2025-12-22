package internal

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// ExecutorConfig holds configuration options for script execution.
type ExecutorConfig struct {
	Stdout    io.Writer
	Stderr    io.Writer
	Env       []string
	WorkDir   string
	ShellPath string
}

// Executor executes generated bash scripts.
type Executor struct {
	config ExecutorConfig
}

// NewExecutor creates a new script executor with the given configuration.
func NewExecutor(config ExecutorConfig) *Executor {
	return &Executor{
		config: config,
	}
}

// Execute runs a script as a single process using the configured shell.
// The script is written to a temporary file before execution, matching GitLab Runner's approach.
func (e *Executor) Execute(ctx context.Context, script string) error {
	tmpFile, err := os.CreateTemp("", "step-runner-script-*.sh")
	if err != nil {
		return fmt.Errorf("creating temporary script file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.WriteString(script); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing script to temporary file: %w", err)
	}

	if err := tmpFile.Chmod(0700); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("making script executable: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temporary script file: %w", err)
	}

	cmd := exec.CommandContext(ctx, e.config.ShellPath, tmpFile.Name())

	cmd.Stdout = e.config.Stdout
	cmd.Stderr = e.config.Stderr
	cmd.Env = e.config.Env
	cmd.Dir = e.config.WorkDir

	return cmd.Run()
}
