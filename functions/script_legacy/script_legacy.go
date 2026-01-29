package script_legacy

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"

	"gitlab.com/gitlab-org/gitlab-runner/functions/script_legacy/internal"
	"gitlab.com/gitlab-org/step-runner/pkg/runner"
	"gitlab.com/gitlab-org/step-runner/proto"
)

// Spec returns the step specification defining inputs for scriptv2.
//
// Inputs:
//
//   - script (array, required): Array of shell commands to execute sequentially.
//     Each command is executed in the same shell session, preserving environment and state.
//
//   - debug_trace (boolean, default: false): Enable verbose script execution tracing.
//     When enabled, adds 'set -o xtrace' to print each command before execution with '+' prefix.
//     Matches GitLab Runner's CI_DEBUG_TRACE behavior.
//
//   - check_for_errors (boolean, default: false): Add explicit exit code checking after each command.
//     When enabled, captures exit code and fails immediately on non-zero values, not relying solely on errexit.
//     Matches GitLab Runner's FF_ENABLE_BASH_EXIT_CODE_CHECK feature flag.
//
//   - posix_escape (boolean, default: false): Use POSIX-compliant shell escaping without ANSI color codes.
//     When enabled, uses double-quote escaping compatible with strict POSIX sh (dash, busybox).
//     When disabled, uses bash-style ANSI-C quoting with color codes.
//     Matches GitLab Runner's FF_POSIXLY_CORRECT_ESCAPES feature flag.
//
//   - trace_sections (boolean, default: false): Wrap multi-line commands in GitLab trace sections.
//     When enabled, creates collapsible sections in GitLab CI logs for multi-line commands.
//     Uses GitLab trace section markers (section_start/section_end) with timestamps.
//     Matches GitLab Runner's FF_SCRIPT_SECTIONS feature flag behavior.
func Spec() *proto.Spec {
	return &proto.Spec{
		Spec: &proto.Spec_Content{
			Inputs: map[string]*proto.Spec_Content_Input{
				// script: Array of shell commands to execute sequentially
				"script": {
					Type:      proto.ValueType_array,
					Default:   nil,
					Sensitive: false,
				},
				// debug_trace: Enable verbose script execution tracing (set -o xtrace)
				"debug_trace": {
					Type:      proto.ValueType_boolean,
					Default:   structpb.NewBoolValue(false),
					Sensitive: false,
				},
				// check_for_errors: Add explicit exit code checking after each command
				"check_for_errors": {
					Type:      proto.ValueType_boolean,
					Default:   structpb.NewBoolValue(false),
					Sensitive: false,
				},
				// posix_escape: Use POSIX-compliant escaping without ANSI colors
				"posix_escape": {
					Type:      proto.ValueType_boolean,
					Default:   structpb.NewBoolValue(false),
					Sensitive: false,
				},
				// trace_sections: Wrap multi-line commands in GitLab trace sections
				"trace_sections": {
					Type:      proto.ValueType_boolean,
					Default:   structpb.NewBoolValue(false),
					Sensitive: false,
				},
			},
		},
	}
}

// Run executes the scriptv2 step, generating and running a shell script from the command array.
func Run(ctx context.Context, stepsCtx *runner.StepsContext) error {
	// Detect shell early - used by both generator (for shebang) and executor (for execution)
	shellPath, err := internal.DetectShell()
	if err != nil {
		return fmt.Errorf("detecting shell: %w", err)
	}

	scriptInput, err := stepsCtx.GetInput("script", runner.KindList)
	if err != nil {
		return fmt.Errorf("getting script input: %w", err)
	}

	var commands []string
	for _, v := range scriptInput.GetListValue().GetValues() {
		commands = append(commands, v.GetStringValue())
	}

	if len(commands) == 0 {
		return fmt.Errorf("script input is empty")
	}

	spec := Spec()
	debugTraceInput, err := stepsCtx.GetInputWithDefault("debug_trace", runner.KindBool, spec.GetSpec().GetInputs())
	if err != nil {
		return fmt.Errorf("getting debug_trace input: %w", err)
	}
	debugTrace := debugTraceInput.GetBoolValue()

	checkForErrorsInput, err := stepsCtx.GetInputWithDefault("check_for_errors", runner.KindBool, spec.GetSpec().GetInputs())
	if err != nil {
		return fmt.Errorf("getting check_for_errors input: %w", err)
	}
	checkForErrors := checkForErrorsInput.GetBoolValue()

	posixEscapeInput, err := stepsCtx.GetInputWithDefault("posix_escape", runner.KindBool, spec.GetSpec().GetInputs())
	if err != nil {
		return fmt.Errorf("getting posix_escape input: %w", err)
	}
	posixEscape := posixEscapeInput.GetBoolValue()

	traceSectionsInput, err := stepsCtx.GetInputWithDefault("trace_sections", runner.KindBool, spec.GetSpec().GetInputs())
	if err != nil {
		return fmt.Errorf("getting trace_sections input: %w", err)
	}
	traceSections := traceSectionsInput.GetBoolValue()

	generatorConfig := internal.ScriptGeneratorConfig{
		DebugTrace:     debugTrace,
		CheckForErrors: checkForErrors,
		PosixEscape:    posixEscape,
		TraceSections:  traceSections,
		ShellPath:      shellPath,
	}
	generator := internal.NewScriptGenerator(generatorConfig)
	script := generator.GenerateScript(commands)

	stdout, stderr := stepsCtx.Pipe()
	env := stepsCtx.GetEnvList()
	workDir := stepsCtx.WorkDir()

	// Add job variables to env
	for key, value := range stepsCtx.View().Vars {
		env = append(env, fmt.Sprintf("%s=%s", key, value.GetStringValue()))
	}

	executorConfig := internal.ExecutorConfig{
		Stdout:    stdout,
		Stderr:    stderr,
		Env:       env,
		WorkDir:   workDir,
		ShellPath: shellPath,
	}
	executor := internal.NewExecutor(executorConfig)
	if err := executor.Execute(ctx, script); err != nil {
		return fmt.Errorf("executing script: %w", err)
	}

	return nil
}
