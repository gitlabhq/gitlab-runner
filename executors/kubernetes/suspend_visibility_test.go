//go:build !integration

package kubernetes

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

func TestGuardSweepVisibility(t *testing.T) {
	tests := []struct {
		name            string
		runnerFF        *bool
		jobVariableFF   bool
		suspendOptions  spec.SuspendOptions
		isResume        bool
		namespacePerJob bool
		configNamespace string
		jobNamespace    string
		wantErr         string
	}{
		{
			name:            "flag only from a job variable is refused",
			jobVariableFF:   true,
			suspendOptions:  spec.SuspendOptions{SuspendOnFailure: true},
			configNamespace: "ci",
			jobNamespace:    "ci",
			wantErr:         "runner configuration",
		},
		{
			name:            "release-only resume with the flag from a job variable is allowed",
			jobVariableFF:   true,
			suspendOptions:  spec.SuspendOptions{RuntimeEnvironmentKey: "k"},
			isResume:        true,
			configNamespace: "ci",
			jobNamespace:    "ci",
		},
		{
			name:            "overwritten namespace is refused on first run",
			runnerFF:        new(true),
			suspendOptions:  spec.SuspendOptions{SuspendOnSuccess: true},
			configNamespace: "ci",
			jobNamespace:    "team-a",
			wantErr:         "namespace",
		},
		{
			name:           "runner-level flag, empty configured namespace resolves to default",
			runnerFF:       new(true),
			suspendOptions: spec.SuspendOptions{SuspendOnSuccess: true},
			jobNamespace:   DefaultResourceIdentifier,
		},
		{
			name:            "resume in the environment's original namespace is allowed",
			runnerFF:        new(true),
			suspendOptions:  spec.SuspendOptions{SuspendOnSuccess: true, RuntimeEnvironmentKey: "k"},
			isResume:        true,
			configNamespace: "ci",
			jobNamespace:    "old-ci",
		},
		{
			name:            "namespace per job is swept across namespaces",
			runnerFF:        new(true),
			suspendOptions:  spec.SuspendOptions{SuspendOnSuccess: true},
			namespacePerJob: true,
			configNamespace: "ci",
			jobNamespace:    "ci-job-42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{
				Namespace:       tt.configNamespace,
				NamespacePerJob: tt.namespacePerJob,
			})
			e.Build.Job.SuspendOptions = tt.suspendOptions
			if tt.runnerFF != nil {
				e.Build.Runner.FeatureFlags = map[string]bool{featureflags.SuspendableEnvironments: *tt.runnerFF}
			}
			if tt.jobVariableFF {
				e.Build.Job.Variables = append(e.Build.Job.Variables,
					spec.Variable{Key: featureflags.SuspendableEnvironments, Value: "true"})
			}
			e.isResume = tt.isResume
			e.configurationOverwrites.namespace = tt.jobNamespace

			err := e.guardSweepVisibility()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
			var buildErr *common.BuildError
			require.True(t, errors.As(err, &buildErr), "want *common.BuildError, got %T", err)
			assert.Equal(t, common.ConfigurationError, buildErr.FailureReason)
		})
	}
}

// A refused resume never created the job's namespace, so tearing it down
// tolerates NotFound.
func TestTeardownBuildNamespace_MissingNamespaceIsIgnored(t *testing.T) {
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{NamespacePerJob: true})
	// "test-ns" is never created — Delete returns NotFound.
	require.NoError(t, e.teardownBuildNamespace(t.Context()))
	// The delete must have been attempted, not silently skipped.
	assert.True(t, hasDelete(e.kubeClient.(*fake.Clientset), "namespaces"))
}

func TestCleanup_RetainsSuspendedEnvironment(t *testing.T) {
	runningPod := &api.Pod{ObjectMeta: metav1.ObjectMeta{Name: "job", Namespace: "ci-job-1"}}

	tests := []struct {
		name          string
		suspended     bool
		isResume      bool
		resumeStarted bool
		pvcName       string
		pod           *api.Pod
		wantReleased  bool
		wantNamespace bool
	}{
		{name: "first run fails before suspending", pvcName: "env", wantReleased: true, wantNamespace: true},
		{name: "resume refused before Resume set state", wantNamespace: true},
		{name: "resume prepare fails after Resume", isResume: true, pvcName: "env"},
		{name: "resume prepare fails after the pod is created", isResume: true, pvcName: "env", pod: runningPod},
		{name: "resume run fails before the pod is created", isResume: true, resumeStarted: true, pvcName: "env"},
		{name: "resume ends without a suspend trigger", isResume: true, resumeStarted: true, pvcName: "env", pod: runningPod,
			wantReleased: true, wantNamespace: true},
		{name: "job suspends", suspended: true, pvcName: "env", pod: runningPod},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{NamespacePerJob: true})
			e.configurationOverwrites.namespace = "ci-job-1"
			e.suspended = tt.suspended
			e.isResume = tt.isResume
			e.resumeStarted = tt.resumeStarted
			e.suspendRootfsPVCName = tt.pvcName
			e.pod = tt.pod

			e.Cleanup()

			client := e.kubeClient.(*fake.Clientset)
			assert.Equal(t, tt.wantReleased, hasDelete(client, "persistentvolumeclaims"), "PVC deleted")
			assert.Equal(t, tt.wantNamespace, hasDelete(client, "namespaces"), "namespace torn down")
		})
	}
}
