//go:build integration && kubernetes

package kubernetes_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/step-runner/schema/v1"
)

// Happy-path E2E: a job with `run:` defined and FF_CONCRETE=true must
// complete end-to-end via the K8s executor's Concrete-mode pod.
func TestKubernetesSteps_HappyPath_RunJobCompletes(t *testing.T) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
	// step-runner fetches sources in Concrete mode, which requires git
	// in the build image. TestAlpineImage (alpine:3.14.2) lacks git;
	// TestDockerGitImage (docker:23-git) bundles it.
	build.Image.Name = common.TestDockerGitImage
	buildtest.SetBuildFeatureFlag(build, featureflags.UseConcrete, true)

	// Replace the default script-based job with a single `run:` step
	// that always succeeds. The exact contents are unimportant; what
	// matters is that the job dispatches through Concrete and exits 0.
	stepName := "happy_path"
	build.Job.Run = spec.Run{
		schema.Step{
			Name:   &stepName,
			Script: stringPtr("exit 0"),
		},
	}

	err := build.Run(t.Context(), &common.Config{}, &common.Trace{Writer: os.Stdout})
	require.NoError(t, err,
		"Concrete-mode K8s job must complete successfully end-to-end")
}

// A Concrete-mode job that declares a service exercises the rewired
// waitForServices: the health probe must exec into the build container
// with the bootstrapped helper binary path, not into a (non-existent
// in Concrete mode) helper container.
func TestKubernetesSteps_ConcreteWithService_PortHealthCheckSucceeds(t *testing.T) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuildWithServices(
		t,
		common.GetRemoteSuccessfulBuild,
		"postgres:16",
	)
	// step-runner fetches sources in Concrete mode, which requires git
	// in the build image. TestAlpineImage (alpine:3.14.2) lacks git;
	// TestDockerGitImage (docker:23-git) bundles it.
	build.Image.Name = common.TestDockerGitImage
	buildtest.SetBuildFeatureFlag(build, featureflags.UseConcrete, true)

	// Give the service a port so waitForServices actually probes it.
	// postgres also requires either a password or
	// POSTGRES_HOST_AUTH_METHOD=trust to start; without that the
	// container exits 1 and we never get to validate the actual
	// rewiring we care about.
	require.NotEmpty(t, build.Services,
		"test fixture should have a service container")
	build.Services[0].Variables = append(build.Services[0].Variables,
		spec.Variable{Key: "HEALTHCHECK_TCP_PORT", Value: "5432"},
		spec.Variable{Key: "POSTGRES_HOST_AUTH_METHOD", Value: "trust"})

	// Minimal `run:` to keep dispatch on the Concrete code path.
	stepName := "concrete_with_service"
	build.Job.Run = spec.Run{
		schema.Step{Name: &stepName, Script: stringPtr("exit 0")},
	}

	err := build.Run(t.Context(), &common.Config{}, &common.Trace{Writer: os.Stdout})
	require.NoError(t, err,
		"Concrete-mode job with a service must complete successfully")
}

// Helper-too-old E2E: when the helper image lacks the `steps`
// subcommand, the bootstrap init container fails and Connect must
// surface the friendly upgrade message captured from
// ContainerStateTerminated.Message.
func TestKubernetesSteps_HelperImageTooOld_SurfacesUpgradeMessage(t *testing.T) {
	kubernetes.SkipKubectlIntegrationTests(t, "kubectl", "cluster-info")

	// Use a deliberately-old helper image that predates the `steps`
	// subcommand. CI_RUNNER_TEST_OLD_HELPER_IMAGE points at a frozen
	// helper tag the test infrastructure provides; when unset, skip
	// rather than fail, since this scenario depends on an artefact
	// that not every environment guarantees.
	oldHelper := os.Getenv("CI_RUNNER_TEST_OLD_HELPER_IMAGE")
	if oldHelper == "" {
		t.Skip("CI_RUNNER_TEST_OLD_HELPER_IMAGE is not set; " +
			"helper-too-old scenario requires a frozen pre-steps helper image")
	}

	build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
	build.Image.Name = common.TestAlpineImage
	build.Runner.Kubernetes.HelperImage = oldHelper
	buildtest.SetBuildFeatureFlag(build, featureflags.UseConcrete, true)

	stepName := "helper_too_old"
	build.Job.Run = spec.Run{
		schema.Step{Name: &stepName, Script: stringPtr("exit 0")},
	}

	var buf strings.Builder
	trace := &common.Trace{Writer: &buf}

	err := build.Run(t.Context(), &common.Config{}, trace)
	require.Error(t, err,
		"job with too-old helper image must fail")

	combined := buf.String() + err.Error()
	assert.Contains(t, combined,
		"helper does not contain CI Steps support",
		"failure must surface the friendly upgrade message — got: %s",
		combined)
}

func stringPtr(s string) *string { return &s }
