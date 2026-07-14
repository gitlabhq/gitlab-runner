package common

import (
	"runtime"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

// Native steps execution is enabled when the executor supports native steps and
// the job uses the run keyword, the script-to-steps migration, or FF_CONCRETE.
//
// Windows is excluded for the hybrid per-stage path; executors whose native
// steps run only through the concrete whole-job path are exempt, since
// concrete works on Windows.
func (b *Build) UseNativeSteps() bool {
	if !b.ExecutorFeatures.NativeStepsIntegration {
		return false
	}

	if runtime.GOOS == "windows" && !b.ExecutorFeatures.NativeStepsViaConcreteOnly {
		return false
	}

	return len(b.Job.Run) > 0 || b.IsFeatureFlagOn(featureflags.UseScriptToStepMigration) || b.IsFeatureFlagOn(featureflags.UseConcrete)
}

// nativeStepsBlockedWithoutConcrete reports whether a native-steps job must be
// rejected because the executor runs native steps only through the concrete
// whole-job path, but FF_CONCRETE is not enabled.
func (b *Build) nativeStepsBlockedWithoutConcrete() bool {
	return b.ExecutorFeatures.NativeStepsViaConcreteOnly && b.UseNativeSteps() && !b.IsFeatureFlagOn(featureflags.UseConcrete)
}
