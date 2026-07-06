//go:build !integration

package kubernetes

import (
	"runtime"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/pull"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/watchers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
	"gitlab.com/gitlab-org/step-runner/schema/v1"
)

// newStepsTestExecutor returns an executor pre-populated with the minimum
// state required by the pod-assembly helpers in steps_pod.go: a runner
// config, an Image option, a BuildShell, a Build with a Job ID, and a
// mocked pull manager that returns api.PullAlways for the build and
// helper containers.
func newStepsTestExecutor(t *testing.T) *executor {
	t.Helper()

	mockPullManager := pull.NewMockManager(t)
	mockPullManager.On("GetPullPolicyFor", mock.Anything).
		Return(api.PullAlways, nil).Maybe()

	ex := newExecutor()
	ex.pullManager = mockPullManager
	ex.options = &kubernetesOptions{
		Image: spec.Image{
			Name: "user/build-image:latest",
		},
	}
	ex.AbstractExecutor.Config = common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Kubernetes: &common.KubernetesConfig{
				Namespace:       "default",
				Image:           "user/build-image:latest",
				HelperImage:     "gitlab/gitlab-runner-helper:test",
				PullPolicy:      common.StringOrArray{"if-not-present"},
				ServiceCPULimit: "",
			},
		},
	}
	ex.AbstractExecutor.BuildShell = &common.ShellConfiguration{
		DockerCommand: []string{"sh", "-c"},
	}
	ex.AbstractExecutor.Build = &common.Build{
		Job:    spec.Job{ID: 42, JobInfo: spec.JobInfo{ProjectID: 7}},
		Runner: &ex.AbstractExecutor.Config,
	}
	ex.AbstractExecutor.BuildLogger = buildlogger.New(
		nil, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})
	ex.configurationOverwrites = &overwrites{
		namespace: "default",
	}
	return ex
}

func TestStepsRunnerBinaryPath_UnderScriptsDir(t *testing.T) {
	ex := newStepsTestExecutor(t)

	got := ex.stepsRunnerBinaryPath()
	scriptsDir := ex.scriptsDir()

	assert.True(t, strings.HasPrefix(got, scriptsDir),
		"binary path must live inside scriptsDir, got %q under %q", got, scriptsDir)
	assert.True(t, strings.HasSuffix(got, "/"+helperBinaryName),
		"binary path must end with %q, got %q", helperBinaryName, got)
}

// buildStepsBootstrapInitContainer must produce a container that
// invokes `gitlab-runner-helper steps bootstrap`, mounts the scripts
// emptyDir, and sets terminationMessagePolicy so the Connect path can
// detect a helper-too-old failure from ContainerStateTerminated.Message.
func TestBuildStepsBootstrapInitContainer_Shape(t *testing.T) {
	ex := newStepsTestExecutor(t)

	c, err := ex.buildStepsBootstrapInitContainer()
	require.NoError(t, err)

	assert.Equal(t, "init-steps-bootstrap", c.Name,
		"init container name is load-bearing for failure-mode detection")
	assert.Equal(t, []string{
		helperBinaryName, "steps", "bootstrap", ex.stepsRunnerBinaryPath(),
	}, c.Command, "init container command must invoke `steps bootstrap`")

	// The terminationMessagePolicy is what lets the Connect path detect a
	// helper image that predates the `steps` subcommand by reading
	// ContainerStateTerminated.Message.
	assert.Equal(t,
		api.TerminationMessageFallbackToLogsOnError,
		c.TerminationMessagePolicy,
		"FallbackToLogsOnError is required so stderr surfaces in "+
			"ContainerStateTerminated.Message on init failure")

	// The scripts emptyDir mount must be present so the binary written by
	// this container is visible to the build container.
	var sawScripts bool
	for _, m := range c.VolumeMounts {
		if m.Name == "scripts" {
			sawScripts = true
			assert.Equal(t, ex.scriptsDir(), m.MountPath)
		}
	}
	assert.True(t, sawScripts, "scripts emptyDir must be mounted in the init container")
}

// The bootstrap init container's pull policy must be resolved under its
// own name (stepsBootstrapInitContainerName), not helperContainerName.
// The retry path advances the failure cursor under the container's actual
// name via UpdatePolicyForContainer, so resolving the build-time policy
// under any other key leaves the cursor unread and the policy never cycles
// on ErrImagePull. The strict mock fails on an unexpected call, so a
// regression to the wrong key fails this test.
func TestBuildStepsBootstrapInitContainer_PullPolicyKeyedByInitContainerName(t *testing.T) {
	ex := newStepsTestExecutor(t)

	m := pull.NewMockManager(t)
	m.On("GetPullPolicyFor", stepsBootstrapInitContainerName).
		Return(api.PullNever, nil).Once()
	ex.pullManager = m

	c, err := ex.buildStepsBootstrapInitContainer()
	require.NoError(t, err)

	assert.Equal(t, api.PullNever, c.ImagePullPolicy,
		"bootstrap pull policy must be resolved under %q", stepsBootstrapInitContainerName)
}

// End-to-end guard for the bootstrap pull-policy cursor. The read-side
// half landed in !6817 (28273af692: buildStepsBootstrapInitContainer reads
// under stepsBootstrapInitContainerName); this ties the three production
// pieces together to prove they share one cursor:
//
//   - preparePullManager registers the bootstrap init container's policies
//     under stepsBootstrapInitContainerName (only when UseNativeSteps),
//   - withPullRetry advances that container's cursor on an ImagePullError,
//   - buildStepsBootstrapInitContainer reads the policy under the same key.
//
// If any of the three keyed off a different name (the original bug used
// helperContainerName read-side), the cursor would never advance and the
// init container would be rebuilt with the first policy on every attempt.
func TestStepsBootstrapPullPolicy_CyclesAcrossRetries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("UseNativeSteps returns false on Windows by design")
	}

	ex := newStepsTestExecutor(t)
	// UseNativeSteps must be true so preparePullManager registers the
	// bootstrap init container under stepsBootstrapInitContainerName.
	ex.Build.ExecutorFeatures.NativeStepsIntegration = true
	ex.Build.Job.Run = spec.Run{schema.Step{Name: stringPtr("step1")}}
	require.True(t, ex.Build.UseNativeSteps())

	// Three distinct policies so the cursor has somewhere to advance. The
	// allowlist must admit all three or ComputeEffectivePullPolicies filters
	// the set down before it reaches the manager.
	allThree := []common.DockerPullPolicy{
		common.PullPolicyAlways,
		common.PullPolicyIfNotPresent,
		common.PullPolicyNever,
	}
	ex.options.Image.PullPolicies = allThree
	ex.Config.Kubernetes.AllowedPullPolicies = allThree

	pm, err := ex.preparePullManager()
	require.NoError(t, err)
	ex.pullManager = pm

	var seen []api.PullPolicy
	retryErr := ex.withPullRetry(t.Context(), func() error {
		c, buildErr := ex.buildStepsBootstrapInitContainer()
		require.NoError(t, buildErr)
		seen = append(seen, c.ImagePullPolicy)
		// Always fail with a pull error on the bootstrap init container so
		// withPullRetry advances its cursor until the policies are exhausted.
		return &pull.ImagePullError{
			Container: stepsBootstrapInitContainerName,
			Image:     ex.getHelperImage(),
			Message:   "simulated pull failure",
		}
	})

	require.Error(t, retryErr,
		"retry must give up once the registered policies are exhausted")
	assert.Equal(t,
		[]api.PullPolicy{api.PullAlways, api.PullIfNotPresent, api.PullNever},
		seen,
		"each retry must read the next registered policy under %q",
		stepsBootstrapInitContainerName)
}

// In Concrete mode the build container's Command must be the
// bootstrapped `steps serve` invocation, Env must be nil (step-runner
// injects env vars over the protocol), and Stdin must be true so the
// proxy can attach. The image's own Entrypoint/Cmd are not surfaced.
func TestStepsBuildContainer_CommandEnvStdin(t *testing.T) {
	ex := newStepsTestExecutor(t)

	c, err := ex.stepsBuildContainer()
	require.NoError(t, err)

	wantCmd := []string{ex.stepsRunnerBinaryPath(), "steps", "serve", "sh", "-c"}
	assert.Equal(t, wantCmd, c.Command,
		"Command must be <helper> steps serve <BuildShell.DockerCommand...>")

	assert.Nil(t, c.Env,
		"Env must be nil — step-runner injects env via the protocol stream")
	assert.True(t, c.Stdin, "Stdin must be true so the proxy can attach")
	assert.Nil(t, c.Args, "Args must be unset (entire invocation is in Command)")
}

// The build container's scripts emptyDir must be mounted at
// scriptsDir() so the bootstrapped helper binary written by the init
// container is reachable by `steps serve`.
func TestStepsBuildContainer_MountsScriptsAtScriptsDir(t *testing.T) {
	ex := newStepsTestExecutor(t)

	c, err := ex.stepsBuildContainer()
	require.NoError(t, err)

	var got string
	for _, m := range c.VolumeMounts {
		if m.Name == "scripts" {
			got = m.MountPath
		}
	}
	assert.Equal(t, ex.scriptsDir(), got,
		"build container's scripts mount must be at scriptsDir()")
}

// The build container's declared ports must be registered in the
// ProxyPool, mirroring the standard buildContainer path so attach/legacy
// session-proxy behaviour is preserved in Concrete mode. Without an
// explicit alias the proxy is keyed by the build container name; an
// alias overrides the key. No declared ports means no registration.
func TestStepsBuildContainer_RegistersBuildPortProxies(t *testing.T) {
	t.Run("declared ports register under the build container name", func(t *testing.T) {
		ex := newStepsTestExecutor(t)
		ex.AbstractExecutor.ProxyPool = proxy.NewPool()
		ex.options.Image.Ports = []spec.Port{{Number: 8080, Protocol: "http", Name: "web"}}

		_, err := ex.stepsBuildContainer()
		require.NoError(t, err)

		p, ok := ex.ProxyPool[buildContainerName]
		require.True(t, ok, "build container ports must register under %q", buildContainerName)
		port, err := p.Settings.PortByNameOrNumber("8080")
		require.NoError(t, err)
		assert.Equal(t, "web", port.Name)
		assert.Equal(t, "http", port.Protocol)
	})

	t.Run("image alias overrides the proxy key", func(t *testing.T) {
		ex := newStepsTestExecutor(t)
		ex.AbstractExecutor.ProxyPool = proxy.NewPool()
		ex.options.Image.Alias = "myapp"
		ex.options.Image.Ports = []spec.Port{{Number: 8080, Protocol: "http", Name: "web"}}

		_, err := ex.stepsBuildContainer()
		require.NoError(t, err)

		_, ok := ex.ProxyPool["myapp"]
		assert.True(t, ok, "declared alias must key the proxy")
		_, ok = ex.ProxyPool[buildContainerName]
		assert.False(t, ok, "build container name must not be used when an alias is set")
	})

	t.Run("no declared ports means no registration", func(t *testing.T) {
		ex := newStepsTestExecutor(t)
		ex.AbstractExecutor.ProxyPool = proxy.NewPool()

		_, err := ex.stepsBuildContainer()
		require.NoError(t, err)

		assert.Empty(t, ex.ProxyPool, "no ports must leave the ProxyPool empty")
	})
}

// Concrete builds its own service containers without the legacy
// buildContainer, so a service container must mount the Concrete volume
// set (scripts emptyDir) and never the logs emptyDir, while still carrying
// its image, declared ports, and a registered session proxy.
func TestStepsServiceContainers_UseStepsMountsAndRegisterProxies(t *testing.T) {
	ex := newStepsTestExecutor(t)
	ex.AbstractExecutor.ProxyPool = proxy.NewPool()
	ex.options.Services = map[string]*spec.Image{
		"db": {Name: "postgres:15", Ports: []spec.Port{{Number: 5432, Name: "sql"}}},
	}

	containers, err := ex.stepsServiceContainers()
	require.NoError(t, err)
	require.Len(t, containers, 1)

	c := containers[0]
	assert.Equal(t, "db", c.Name)
	assert.Equal(t, "postgres:15", c.Image)

	var sawScripts, sawLogs bool
	for _, m := range c.VolumeMounts {
		switch m.Name {
		case "scripts":
			sawScripts = true
		case "logs":
			sawLogs = true
		}
	}
	assert.True(t, sawScripts, "service container must mount the scripts emptyDir")
	assert.False(t, sawLogs, "service container must NOT mount the logs emptyDir (Concrete has none)")

	require.Len(t, c.Ports, 1)
	assert.Equal(t, int32(5432), c.Ports[0].ContainerPort)
	_, ok := ex.ProxyPool["proxy-db"]
	assert.True(t, ok, "declared service ports must register a session proxy")
}

// The Concrete pod must contain exactly one build container plus the
// user-declared service containers — never a helper container.
func TestStepsPreparePodConfig_HasBuildAndServicesButNoHelper(t *testing.T) {
	ex := newStepsTestExecutor(t)

	svcA := api.Container{Name: "postgres"}
	svcB := api.Container{Name: "redis"}

	pod, err := ex.stepsPreparePodConfig(podConfigPrepareOpts{
		services:       []api.Container{svcA, svcB},
		initContainers: []api.Container{{Name: "init-steps-bootstrap"}},
	})
	require.NoError(t, err)

	names := containerNames(pod.Spec.Containers)
	assert.Equal(t, []string{buildContainerName, "postgres", "redis"}, names,
		"containers must be exactly [build, <services...>] in that order")

	for _, n := range names {
		assert.NotEqual(t, helperContainerName, n,
			"helper container must not be present in Concrete pod")
	}
}

// The Concrete pod's InitContainers list must be exactly the opts
// passed in — i.e., the single init-steps-bootstrap container — and
// must not be augmented by init-permissions or init-logs.
func TestStepsPreparePodConfig_InitContainersFromOptsOnly(t *testing.T) {
	ex := newStepsTestExecutor(t)

	pod, err := ex.stepsPreparePodConfig(podConfigPrepareOpts{
		initContainers: []api.Container{{Name: "init-steps-bootstrap"}},
	})
	require.NoError(t, err)

	names := containerNames(pod.Spec.InitContainers)
	assert.Equal(t, []string{"init-steps-bootstrap"}, names,
		"Concrete pod has exactly one init container: init-steps-bootstrap")
}

// The Concrete pod's Volumes must always include the scripts emptyDir
// and never include the logs emptyDir (the latter only exists to feed
// the helper container, which is absent in Concrete mode).
func TestStepsVolumes_IncludesScriptsAndExcludesLogs(t *testing.T) {
	ex := newStepsTestExecutor(t)

	vols := ex.stepsVolumes()

	names := volumeNames(vols)
	assert.Contains(t, names, "scripts",
		"scripts emptyDir must always be present")
	assert.NotContains(t, names, "logs",
		"logs emptyDir must not be present in Concrete pod")
}

// When the default builds-dir volume is required, the repo emptyDir
// must be added to the pod's Volumes.
func TestStepsVolumes_RepoEmptyDirWhenDefaultBuildsDirRequired(t *testing.T) {
	ex := newStepsTestExecutor(t)

	// Default builds dir is required when the user has not configured a
	// custom builds dir on the runner.
	require.True(t, ex.isDefaultBuildsDirVolumeRequired(),
		"test fixture should default to requiring the repo emptyDir")

	vols := ex.stepsVolumes()
	assert.Contains(t, volumeNames(vols), "repo",
		"repo emptyDir must be present when isDefaultBuildsDirVolumeRequired is true")
}

// stepsVolumeMounts always mounts scripts at scriptsDir, and
// includes the repo mount when isDefaultBuildsDirVolumeRequired is true.
func TestStepsVolumeMounts_IncludesScriptsAndRepo(t *testing.T) {
	ex := newStepsTestExecutor(t)

	mounts := ex.stepsVolumeMounts()

	var sawScripts, sawRepo bool
	for _, m := range mounts {
		switch m.Name {
		case "scripts":
			sawScripts = true
			assert.Equal(t, ex.scriptsDir(), m.MountPath)
		case "repo":
			sawRepo = true
		}
	}
	assert.True(t, sawScripts, "scripts must be mounted at scriptsDir()")
	assert.True(t, sawRepo,
		"repo must be mounted when isDefaultBuildsDirVolumeRequired is true")
}

// Setting Env: nil on the build container must have no effect on
// service containers. opts.services is passed through unchanged by
// stepsPreparePodConfig, so any Env populated on the service
// containers upstream of this function survives.
func TestStepsPreparePodConfig_ServiceEnvUntouched(t *testing.T) {
	ex := newStepsTestExecutor(t)

	svcEnv := []api.EnvVar{{Name: "POSTGRES_PASSWORD", Value: "secret"}}
	svc := api.Container{Name: "postgres", Env: svcEnv}

	pod, err := ex.stepsPreparePodConfig(podConfigPrepareOpts{
		services: []api.Container{svc},
	})
	require.NoError(t, err)

	require.Len(t, pod.Spec.Containers, 2, "expected [build, postgres]")
	build := pod.Spec.Containers[0]
	postgres := pod.Spec.Containers[1]

	assert.Nil(t, build.Env, "build container Env must be nil")
	assert.Equal(t, svcEnv, postgres.Env,
		"service container Env must be unchanged by Concrete pod assembly")
}

// stepsWaitForServices is the Concrete-owned service-health-check path. It
// must short-circuit (no pod setup, no probe exec) when no service declares
// a HEALTHCHECK_TCP_PORT, matching the legacy waitForServices early return.
// This keeps the function exercisable without a live pod/exec backend.
func TestStepsWaitForServices_NoProbeWhenNoHealthCheckPort(t *testing.T) {
	ex := newStepsTestExecutor(t)
	ex.options.Services = map[string]*spec.Image{
		"db": {Name: "postgres:15"},
	}

	require.NoError(t, ex.stepsWaitForServices(t.Context()),
		"with no HEALTHCHECK_TCP_PORT the Concrete probe must return nil early")
}

func stringPtr(s string) *string { return &s }

func containerNames(cs []api.Container) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return out
}

func volumeNames(vs []api.Volume) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.Name)
	}
	return out
}

// newStepsTestExecutorWithFakes wires a fresh executor up with the fake
// kube client, NoopPodWatcher, ProxyPool, and featureChecker that
// setupStepsPod transitively touches. It is the steps-pod-specific
// counterpart to the fixture used by the setupBuildPod test table in
// kubernetes_test.go, shrunk to the surface this file needs.
func newStepsTestExecutorWithFakes(t *testing.T) (*executor, *testclient.Clientset) {
	t.Helper()

	ex := newStepsTestExecutor(t)

	fakeClient := testclient.NewClientset()
	ex.kubeClient = fakeClient
	ex.podWatcher = watchers.NoopPodWatcher{}
	ex.AbstractExecutor.ProxyPool = proxy.NewPool()

	mockFc := newMockFeatureChecker(t)
	mockFc.On("IsHostAliasSupported").Return(true, nil).Maybe()
	ex.featureChecker = mockFc

	return ex, fakeClient
}

// PDB parity invariant: setupStepsPod must mirror setupBuildPod's
// behavior of creating a PodDisruptionBudget when the operator has
// enabled it via runner config. Otherwise the deliberately-configured
// feature would be silently dropped for native-steps jobs.
func TestSetupStepsPod_CreatesPodDisruptionBudgetWhenEnabled(t *testing.T) {
	ex, fakeClient := newStepsTestExecutorWithFakes(t)
	enabled := true
	ex.Config.Kubernetes.PodDisruptionBudget = &enabled

	err := ex.setupStepsPod(t.Context(),
		[]api.Container{{Name: stepsBootstrapInitContainerName}})
	require.NoError(t, err)

	pdbs, err := fakeClient.PolicyV1().PodDisruptionBudgets("default").
		List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, pdbs.Items, 1,
		"PDB must be created in the pod's namespace when enabled")
	assert.Equal(t, "default", pdbs.Items[0].Namespace)
	assert.NotNil(t, ex.podDisruptionBudget,
		"executor must retain the created PDB for cleanup")
}

func TestSetupStepsPod_DoesNotCreatePodDisruptionBudgetWhenDisabled(t *testing.T) {
	ex, fakeClient := newStepsTestExecutorWithFakes(t)
	disabled := false
	ex.Config.Kubernetes.PodDisruptionBudget = &disabled

	err := ex.setupStepsPod(t.Context(),
		[]api.Container{{Name: stepsBootstrapInitContainerName}})
	require.NoError(t, err)

	pdbs, err := fakeClient.PolicyV1().PodDisruptionBudgets("default").
		List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, pdbs.Items,
		"PDB must NOT be created when the operator has disabled it")
	assert.Nil(t, ex.podDisruptionBudget)
}

// TestGetVolumeMounts_PresenceAndPaths pins the logs-mount contract of the
// shared getVolumeMounts. Concrete pods assemble their containers with
// stepsVolumeMounts and never call getVolumeMounts, so the Concrete guard
// was removed: getVolumeMounts now depends only on
// FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY.
//
// The 2x2 matrix across (UseNativeSteps, FF_USE_LEGACY) pins that
// getVolumeMounts is native-steps-neutral: both attach rows must keep the
// logs mount regardless of UseNativeSteps. Re-introducing a UseNativeSteps
// branch here would be a regression and must fail this test.
//
// The fixture leaves FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR off
// (default). Anyone wanting to pin the scripts-mount clause across
// DumbInit should add a separate test, deliberately out of scope here.
func TestGetVolumeMounts_PresenceAndPaths(t *testing.T) {
	type matrixRow struct {
		name           string
		useNativeSteps bool // fixture sets NativeStepsIntegration + FF_CONCRETE
		legacyFF       bool
		wantScripts    bool
		wantLogs       bool
	}

	rows := []matrixRow{
		{
			name:           "non-steps attach (FF_CONCRETE off, FF_USE_LEGACY off)",
			useNativeSteps: false, legacyFF: false,
			wantScripts: true, wantLogs: true, // ← regression canary
		},
		{
			name:           "non-steps legacy (FF_CONCRETE off, FF_USE_LEGACY on)",
			useNativeSteps: false, legacyFF: true,
			wantScripts: false, wantLogs: false,
		},
		{
			name:           "native-steps attach (FF_CONCRETE on, FF_USE_LEGACY off)",
			useNativeSteps: true, legacyFF: false,
			wantScripts: true, wantLogs: true, // native-steps-neutral: guard removed
		},
		{
			name:           "native-steps legacy (FF_CONCRETE on, FF_USE_LEGACY on)",
			useNativeSteps: true, legacyFF: true,
			wantScripts: false, wantLogs: false,
		},
	}

	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			if runtime.GOOS == "windows" && tc.useNativeSteps {
				t.Skip("UseNativeSteps returns false on Windows by design")
			}

			ex := newStepsTestExecutor(t)
			if tc.useNativeSteps {
				ex.Build.ExecutorFeatures.NativeStepsIntegration = true
				ex.Build.Job.Run = spec.Run{
					schema.Step{Name: stringPtr("trigger-use-native-steps")},
				}
			}
			buildtest.SetBuildFeatureFlag(ex.Build,
				featureflags.UseLegacyKubernetesExecutionStrategy, tc.legacyFF)
			if tc.useNativeSteps {
				buildtest.SetBuildFeatureFlag(ex.Build,
					featureflags.UseConcrete, true)
				require.True(t, ex.Build.UseNativeSteps(),
					"fixture must produce UseNativeSteps()=true")
			} else {
				require.False(t, ex.Build.UseNativeSteps(),
					"fixture must produce UseNativeSteps()=false")
			}

			mounts := ex.getVolumeMounts()

			// Presence per matrix.
			assert.Equal(t, tc.wantScripts,
				containsMount(mounts, "scripts"),
				"scripts mount presence")
			assert.Equal(t, tc.wantLogs,
				containsMount(mounts, "logs"),
				"logs mount presence")
			// repo is independent of the flags we vary; the fixture
			// has isDefaultBuildsDirVolumeRequired()=true, so it must
			// always be present.
			assert.True(t, containsMount(mounts, "repo"),
				"repo mount must be present (fixture has builds-dir required)")

			// Mount-path correctness for every mount that IS present —
			// catches accidental path swaps between adjacent clauses.
			if i := indexOfMount(mounts, "scripts"); i >= 0 {
				assert.Equal(t, ex.scriptsDir(), mounts[i].MountPath,
					"scripts mount path")
			}
			if i := indexOfMount(mounts, "logs"); i >= 0 {
				assert.Equal(t, ex.logsDir(), mounts[i].MountPath,
					"logs mount path")
			}
			if i := indexOfMount(mounts, "repo"); i >= 0 {
				assert.Equal(t, ex.AbstractExecutor.RootDir(),
					mounts[i].MountPath, "repo mount path")
			}

			// No duplicate mount names. Kube does not always reject
			// duplicates, so a silent logical duplicate is exactly the
			// kind of refactor breakage worth pinning.
			seen := map[string]bool{}
			for _, m := range mounts {
				assert.False(t, seen[m.Name],
					"duplicate mount name: %q", m.Name)
				seen[m.Name] = true
			}
		})
	}
}

func indexOfMount(mounts []api.VolumeMount, name string) int {
	for i, m := range mounts {
		if m.Name == name {
			return i
		}
	}
	return -1
}

func containsMount(mounts []api.VolumeMount, name string) bool {
	return indexOfMount(mounts, name) >= 0
}
