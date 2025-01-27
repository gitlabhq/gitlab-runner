package common

import (
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

// Native steps execution is enabled if:
// - the job uses the run keyword.
// - the feature flag is enabled.
// - the executor supports native steps.
func (b *Build) UseNativeSteps() bool {
	return b.JobResponse.Run != "" && b.IsFeatureFlagOn(featureflags.UseNativeSteps) && b.ExecutorFeatures.NativeStepsIntegration
}
