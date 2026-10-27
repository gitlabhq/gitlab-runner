package docker

import (
	"strconv"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/labels"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

func (e *executor) createLabeler() error {
	var envLabels map[string]string

	// Only environments that are suspended or resumed are labeled, the same
	// jobs checkSuspendResumeGuards checks. The runner-level flag takes
	// precedence over job variables, so with it on these options match
	// Build.WillSuspend and Build.RuntimeEnvironmentKey. Reading them directly
	// avoids resolving the build settings this early.
	key := acquisitionKey(e.Build)
	opts := e.Build.Job.SuspendOptions
	suspendable := opts.SuspendOnSuccess || opts.SuspendOnFailure || opts.RuntimeEnvironmentKey != ""
	if key != "" && suspendable && e.Config.IsFeatureFlagOn(featureflags.SuspendableEnvironments) {
		envLabels = map[string]string{
			labels.AcquisitionKeyLabel:              key,
			common.RuntimeEnvironmentRunnerIDLabel:  strconv.FormatInt(e.Config.ID, 10),
			common.RuntimeEnvironmentExpiresAtLabel: common.FormatRuntimeEnvironmentExpiry(e.expiresAt),
		}
	}

	e.labeler = labels.NewLabeler(e.Build, envLabels)
	return nil
}
