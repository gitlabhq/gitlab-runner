package common

import (
	"runtime"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

// Native steps execution is enabled if:
// - we are not running on windows
// - the executor supports native steps.
// - the job uses the run keyword or script_to_steps migraton is active.
func (b *Build) UseNativeSteps() bool {
	if runtime.GOOS == "windows" {
		return false
	}

	if !b.ExecutorFeatures.NativeStepsIntegration {
		return false
	}

	return len(b.Job.Run) > 0 || b.IsFeatureFlagOn(featureflags.UseScriptToStepMigration)
}
