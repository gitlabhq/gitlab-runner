package stages

import (
	"context"
	"fmt"
	"os"
	"os/exec"

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
	UseLegacyBashEval bool `json:"use_legacy_bash_eval,omitempty"`
}

func (s Step) Run(ctx context.Context, e *env.Env) error {
	if len(s.Script) == 0 {
		return nil
	}

	if !s.shouldRun(e) {
		e.Debugf("Skipping step %s: not applicable for current job status", s.Step)
		return nil
	}

	// any user scripts that would previously be executed in the helper
	// container benefit from being able to use the bundled git and CA certs
	var envVars map[string]string
	switch s.Step {
	case "pre_clone_script", "post_clone_script":
		envVars = e.HelperEnvs(envVars)
	}

	sw := scriptwriter.New(s.Step, e.Shell)
	sw.LookPath = shellLookPath(e, envVars)
	sw.DebugTrace = s.Debug
	sw.ExitCodeCheck = s.BashExitCodeCheck
	sw.ScriptSections = s.ScriptSections
	sw.UseLegacyBashEval = s.UseLegacyBashEval

	script := sw.Build(s.Script)
	if err := shell(ctx, e, script, envVars); err != nil {
		// AllowFailure is intentionally not honored: abstract shell
		// ignores it on script steps too, and the runner core has no
		// way to record a non-zero exit while clearing failure_reason.
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

// shellLookPath finds shells where the legacy executor finds them. Login-shell
// executors start the shell from this process, so they use its PATH. Other
// executors find the shell inside the job, so they use the job's PATH.
func shellLookPath(e *env.Env, envVars map[string]string) func(string) (string, error) {
	if e.LoginShell {
		return exec.LookPath
	}
	return func(name string) (string, error) { return e.LookPath(name, envVars) }
}

func shell(ctx context.Context, e *env.Env, script string, envVars map[string]string) error {
	isPwsh := e.Shell == "pwsh" || e.Shell == "powershell"

	ext := ".sh"
	if isPwsh {
		ext = ".ps1"
	}

	f, err := os.CreateTemp(e.TmpDir, "runner-script-*"+ext)
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
		cmd, err = shellLookPath(e, envVars)(e.Shell)
		args = []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", f.Name()}
	case e.LoginShell:
		cmd, err = shellLookPath(e, envVars)(e.Shell)
		args = []string{"-l", f.Name()}
	default:
		cmd = f.Name()
	}
	if err != nil {
		return fmt.Errorf("finding shell %q: %w", e.Shell, err)
	}

	return e.Command(ctx, cmd, envVars, args...)
}
