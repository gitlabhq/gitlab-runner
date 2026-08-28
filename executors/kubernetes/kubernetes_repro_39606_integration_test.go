//go:build integration && kubernetes

package kubernetes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes"
)

// TestPredefinedStageScriptRemovedBeforeHelperExecutesIt reproduces
// https://gitlab.com/gitlab-org/gitlab-runner/-/issues/39606: a background
// watcher deletes cleanup_file_variables's script from the shared
// ScriptsBaseDir emptyDir before the helper container executes it. The
// helper reports exit 127 via the fallback marker, and the job completes.
func TestPredefinedStageScriptRemovedBeforeHelperExecutesIt(t *testing.T) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteSuccessfulBuild, withShell("bash"))
	// Autodetect arch/OS instead of relying on the executor's hardcoded amd64
	// default, so this also runs on non-amd64 (e.g. arm64) test runners.
	build.Runner.RunnerSettings.Kubernetes.HelperImageAutosetArchAndOS = true

	build.Job.Steps = spec.Steps{
		spec.Step{
			Name: spec.StepNameScript,
			Script: []string{
				// Never stops: cleanup_file_variables only runs once, right at
				// the very end, so there's no advantage in giving up after one
				// hit - keep unlinking as fast as possible for the rest of the
				// job's life to reliably beat the runner's own network
				// round-trip to the API server for the exec call that follows
				// the save.
				`(while true; do rm -f /scripts-*/cleanup_file_variables 2>/dev/null; done) &`,
				`echo step ran`,
			},
			When: spec.StepWhenAlways,
		},
	}

	out, err := buildtest.RunBuildReturningOutput(t, build)
	assert.NoError(t, err, "job should complete (not hang) even when the predefined-stage script vanishes before exec")
	assert.Contains(t, out, "step ran")
}
