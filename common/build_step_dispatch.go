package common

import (
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/step-runner/schema/v1"
)

const stepRunBuildStage = BuildStage("step_" + spec.StepNameRun)

// stepDispatch converts a build stage to a list of steps to run.
//
// Depending on the configuration, this can also include stages that we're
// in the process of migrating (from scripts) to a step.
//
//nolint:gocognit
func stepDispatch(build *Build, executor Executor, stage BuildStage) (bool, []schema.Step) {
	switch stage {
	case BuildStagePrepare, BuildStageGetSources, BuildStageClearWorktree, BuildStageRestoreCache, BuildStageDownloadArtifacts, BuildStageArchiveOnSuccessCache, BuildStageArchiveOnFailureCache, BuildStageUploadOnFailureArtifacts, BuildStageUploadOnSuccessArtifacts, BuildStageCleanup:
		// don't handle non-user script stages
		return false, nil

	case stepRunBuildStage:
		return true, build.Job.Run

	case BuildStageAfterScript:
		// don't handle after_script (yet)
		return false, nil

	default: // user script
		if !build.IsFeatureFlagOn(featureflags.UseScriptToStepMigration) {
			return false, nil
		}

		shell := executor.Shell()
		if shell == nil {
			return false, nil
		}

		var script []string

		if shell.PreBuildScript != "" {
			script = append(script, shell.PreBuildScript)
		}

		for _, step := range build.Steps {
			if StepToBuildStage(step) == stage {
				script = append(script, step.Script...)
				if step.Name == "release" {
					for i, s := range step.Script {
						script[i] = build.GetAllVariables().ExpandValue(s)
					}
				}
				break
			}
		}

		if shell.PostBuildScript != "" {
			script = append(script, shell.PostBuildScript)
		}

		// if no script, no-op
		if len(script) == 0 {
			return true, nil // handled, but nothing to do
		}

		return true, []schema.Step{
			{
				Name: func(s string) *string { return &s }("user_script"),
				Step: "builtin://script_legacy",
				Inputs: schema.StepInputs{
					"script":           script,
					"debug_trace":      build.IsDebugTraceEnabled(),
					"posix_escape":     true,
					"check_for_errors": build.IsFeatureFlagOn(featureflags.EnableBashExitCodeCheck),
					"trace_sections":   build.IsFeatureFlagOn(featureflags.ScriptSections),
				},
			},
		}
	}
}
