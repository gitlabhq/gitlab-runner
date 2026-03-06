package stages

import (
	"context"
	"fmt"
	"os"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/stages/internal/scriptwriter"
)

type Step struct {
	Step         string   `json:"step,omitempty"`
	Script       []string `json:"script,omitempty"`
	AllowFailure bool     `json:"allow_failure,omitempty"`
	OnSuccess    bool     `json:"on_success,omitempty"`
	OnFailure    bool     `json:"on_failure,omitempty"`

	Debug             bool `json:"debug,omitempty"`
	BashExitCodeCheck bool `json:"bash_exit_code_check,omitempty"`
	ScriptSections    bool `json:"script_sections,omitempty"`
}

func (s Step) Run(ctx context.Context, e *env.Env) error {
	if len(s.Script) == 0 {
		return nil
	}

	if !s.shouldRun(e) {
		e.Debugf("Skipping step %s: not applicable for current job status", s.Step)
		return nil
	}

	sw := scriptwriter.New(s.Step, e.Shell)
	sw.DebugTrace = s.Debug
	sw.ExitCodeCheck = s.BashExitCodeCheck
	sw.ScriptSections = s.ScriptSections

	script := sw.Build(s.Script)
	if err := shell(ctx, e, script, s.Step); err != nil {
		if s.AllowFailure {
			e.Warningf("Step %s failed (allow_failure): %v", s.Step, err)
			return nil
		}
		return fmt.Errorf("step %s: %w", s.Step, err)
	}

	return nil
}

func (s Step) shouldRun(e *env.Env) bool {
	if e.IsSuccessful() {
		return s.OnSuccess
	}
	return s.OnFailure
}

func shell(ctx context.Context, e *env.Env, script, stepName string) error {
	isPwsh := e.Shell == "pwsh" || e.Shell == "powershell"

	ext := ".sh"
	if isPwsh {
		ext = ".ps1"
	}

	f, err := os.CreateTemp("", "runner-script-*"+ext)
	if err != nil {
		return fmt.Errorf("creating script file: %w", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if _, err := f.WriteString(script); err != nil {
		return fmt.Errorf("writing script file: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("closing script file: %w", err)
	}

	if err := os.Chmod(f.Name(), 0o700); err != nil {
		return fmt.Errorf("setting script permissions: %w", err)
	}

	var cmd string
	var args []string

	switch {
	case isPwsh:
		cmd = e.Shell
		args = []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", f.Name()}
	case e.LoginShell:
		cmd = e.Shell
		args = []string{"-l", f.Name()}
	default:
		cmd = f.Name()
	}

	// any user scripts that would previously be executed in the helper
	// container benefit from being able to use the bundled git and CA certs
	var envVars map[string]string
	switch stepName {
	case "pre_clone_script", "post_clone_script":
		envVars = e.HelperEnvs(envVars)
	}

	return e.Command(ctx, cmd, envVars, args...)
}
