package stages

import (
	"context"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
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
	// todo: impl
	return nil
}
