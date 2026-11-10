//go:build !integration

package kubernetes

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "k8s.io/api/core/v1"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/pull"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
)

// honorEntrypointExecutor returns an executor with FF_KUBERNETES_HONOR_ENTRYPOINT
// set to the given value.
func honorEntrypointExecutor(t *testing.T, on bool) *executor {
	t.Helper()
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{Image: "test-image"})
	value := "false"
	if on {
		value = "true"
	}
	e.Build.Job.Variables = append(e.Build.Job.Variables,
		spec.Variable{Key: featureflags.KubernetesHonorEntrypoint, Value: value, Public: true},
	)
	return e
}

// Core matrix: with the flag ON, a service container must honour whatever the
// job spec set, and leave unset fields unset so the image default still applies.
func TestGetCommandAndArgs_ServiceContainer_HonorEntrypointOn(t *testing.T) {
	tests := map[string]struct {
		image           spec.Image
		expectedCommand []string
		expectedArgs    []string
	}{
		"entrypoint only": {
			image:           spec.Image{Name: "svc", Entrypoint: []string{"/custom-entrypoint"}},
			expectedCommand: []string{"/custom-entrypoint"},
			expectedArgs:    nil,
		},
		"command only": {
			image:           spec.Image{Name: "svc", Command: []string{"sh", "-c", "echo ok"}},
			expectedCommand: nil,
			expectedArgs:    []string{"sh", "-c", "echo ok"},
		},
		"entrypoint and command": {
			image: spec.Image{
				Name:       "svc",
				Entrypoint: []string{"/analyzer"},
				Command:    []string{"dependency-resolution-service", "--project-types", "java"},
			},
			expectedCommand: []string{"/analyzer"},
			expectedArgs:    []string{"dependency-resolution-service", "--project-types", "java"},
		},
		"neither set falls back to image defaults": {
			image:           spec.Image{Name: "svc"},
			expectedCommand: []string{},
			expectedArgs:    nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			e := honorEntrypointExecutor(t, true)
			command, args := e.getCommandAndArgs(tc.image, true)
			assert.Equal(t, tc.expectedCommand, command, "Command")
			assert.Equal(t, tc.expectedArgs, args, "Args")
		})
	}
}

// Build and helper containers must be completely untouched by the change.
func TestGetCommandAndArgs_BuildAndHelper_HonorEntrypointOn_Unchanged(t *testing.T) {
	e := honorEntrypointExecutor(t, true)

	shellScript := []string{"/scripts-0-0/dumb-init", "--", "/bin/sh"}
	image := spec.Image{
		Name:       "test-image",
		Entrypoint: []string{"/should-be-ignored"},
		Command:    []string{"also-ignored"},
	}

	command, args := e.getCommandAndArgs(image, false, shellScript...)
	assert.Equal(t, []string{}, command, "build/helper Command must stay empty under the flag")
	assert.Equal(t, shellScript, args, "build/helper Args must still carry the shell script")
}

// The flag-off path must be byte-for-byte unchanged for both kinds of container.
func TestGetCommandAndArgs_HonorEntrypointOff_Unchanged(t *testing.T) {
	image := spec.Image{
		Name:       "svc",
		Entrypoint: []string{"/entry"},
		Command:    []string{"arg1", "arg2"},
	}

	t.Run("service", func(t *testing.T) {
		e := honorEntrypointExecutor(t, false)
		command, args := e.getCommandAndArgs(image, true)
		assert.Equal(t, []string{"/entry"}, command)
		assert.Equal(t, []string{"arg1", "arg2"}, args)
	})

	t.Run("build with shell script", func(t *testing.T) {
		e := honorEntrypointExecutor(t, false)
		shellScript := []string{"/bin/sh"}
		command, args := e.getCommandAndArgs(image, false, shellScript...)
		assert.Equal(t, shellScript, command, "explicit command wins over image entrypoint")
		assert.Equal(t, []string{"arg1", "arg2"}, args)
	})
}

// End to end through buildContainer: the resulting pod container spec is what
// actually reaches Kubernetes, so assert on it directly. This is the exact
// reproduction from the bug report.
func TestBuildContainer_ServiceHonoursCommand_FlagOn(t *testing.T) {
	e := honorEntrypointExecutor(t, true)
	e.options = &kubernetesOptions{}
	mockPM := pull.NewMockManager(t)
	mockPM.On("GetPullPolicyFor", "svc-0").Return(api.PullAlways, nil).Maybe()
	e.pullManager = mockPM

	svc := spec.Image{
		Name:    "busybox",
		Alias:   "probe",
		Command: []string{"sh", "-c", "echo ok > /builds/svc.txt; sleep 300"},
	}

	container, err := e.buildContainer(containerBuildOpts{
		name:               "svc-0",
		image:              svc.Name,
		imageDefinition:    svc,
		isServiceContainer: true,
	})
	require.NoError(t, err)

	assert.Empty(t, container.Command, "image ENTRYPOINT should still apply")
	assert.Equal(t, []string{"sh", "-c", "echo ok > /builds/svc.txt; sleep 300"}, container.Args,
		"service command must reach the pod spec")
}

// With suspend/resume active, overlayServiceContainer folds the resolved
// Command+Args into the overlay's args so the real service process is exec'd
// after pivot_root. Before the fix those were both empty under the flag, so the
// overlay exec'd nothing. Guard that path explicitly.
func TestPreparePodServices_SuspendOverlay_CarriesServiceProcess_FlagOn(t *testing.T) {
	e := newSuspendableExecutorWithServices(t, []common.Service{
		{
			Name:       "redis:7",
			Entrypoint: []string{"/bin/redis-server"},
			Command:    []string{"--port", "6379"},
		},
	})
	e.Build.Job.Variables = append(e.Build.Job.Variables,
		spec.Variable{Key: featureflags.KubernetesHonorEntrypoint, Value: "true", Public: true},
	)

	containers, err := e.preparePodServices()
	require.NoError(t, err)
	require.Len(t, containers, 1)

	assert.Equal(t, []string{"/bin/sh", overlayScriptPath}, containers[0].Command,
		"overlay script must be the container command")
	assert.Equal(t, []string{"/bin/redis-server", "--port", "6379"}, containers[0].Args,
		"overlay must carry the service's real process as args")
}

// The native steps pod path (steps_pod.go) builds service containers without
// the legacy buildContainer, so it calls getCommandAndArgs through a separate
// call site. It must honour the service's entrypoint/command under the flag
// exactly as the legacy path does, otherwise the bug survives whenever
// FF_USE_NATIVE_STEPS is on.
func TestStepsServiceContainer_HonoursEntrypointAndCommand(t *testing.T) {
	tests := map[string]struct {
		honorEntrypoint bool
		service         spec.Image
		expectedCommand []string
		expectedArgs    []string
	}{
		"command only, flag on": {
			honorEntrypoint: true,
			service:         spec.Image{Name: "postgres:15", Command: []string{"postgres", "-p", "5432"}},
			expectedCommand: nil,
			expectedArgs:    []string{"postgres", "-p", "5432"},
		},
		"entrypoint only, flag on": {
			honorEntrypoint: true,
			service:         spec.Image{Name: "postgres:15", Entrypoint: []string{"/custom-entrypoint"}},
			expectedCommand: []string{"/custom-entrypoint"},
			expectedArgs:    nil,
		},
		"entrypoint and command, flag on": {
			honorEntrypoint: true,
			service: spec.Image{
				Name:       "postgres:15",
				Entrypoint: []string{"/custom-entrypoint"},
				Command:    []string{"-p", "5432"},
			},
			expectedCommand: []string{"/custom-entrypoint"},
			expectedArgs:    []string{"-p", "5432"},
		},
		"neither set, flag on": {
			honorEntrypoint: true,
			service:         spec.Image{Name: "postgres:15"},
			expectedCommand: []string{},
			expectedArgs:    nil,
		},
		"entrypoint and command, flag off": {
			honorEntrypoint: false,
			service: spec.Image{
				Name:       "postgres:15",
				Entrypoint: []string{"/custom-entrypoint"},
				Command:    []string{"-p", "5432"},
			},
			expectedCommand: []string{"/custom-entrypoint"},
			expectedArgs:    []string{"-p", "5432"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ex := newStepsTestExecutor(t)
			ex.AbstractExecutor.ProxyPool = proxy.NewPool()
			ex.Build.Job.Variables = append(ex.Build.Job.Variables, spec.Variable{
				Key:    featureflags.KubernetesHonorEntrypoint,
				Value:  strconv.FormatBool(tc.honorEntrypoint),
				Public: true,
			})
			svc := tc.service
			ex.options.Services = map[string]*spec.Image{"db": &svc}

			containers, err := ex.stepsServiceContainers()
			require.NoError(t, err)
			require.Len(t, containers, 1)

			assert.Equal(t, tc.expectedCommand, containers[0].Command, "Command")
			assert.Equal(t, tc.expectedArgs, containers[0].Args, "Args")
		})
	}
}
