//go:build !integration

package kubernetes

import (
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

func TestIsSuspendableJob(t *testing.T) {
	tests := []struct {
		name           string
		suspendOptions spec.SuspendOptions
		ffEnabled      bool
		want           bool
	}{
		{name: "SuspendOnSuccess", suspendOptions: spec.SuspendOptions{SuspendOnSuccess: true}, ffEnabled: true, want: true},
		{name: "SuspendOnFailure", suspendOptions: spec.SuspendOptions{SuspendOnFailure: true}, ffEnabled: true, want: true},
		{name: "resume job (RuntimeEnvironmentKey set)", suspendOptions: spec.SuspendOptions{RuntimeEnvironmentKey: "some-key"}, ffEnabled: true, want: true},
		{name: "normal job", suspendOptions: spec.SuspendOptions{}, ffEnabled: true, want: false},
		{name: "FF disabled", suspendOptions: spec.SuspendOptions{SuspendOnSuccess: true}, ffEnabled: false, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			job := spec.Job{SuspendOptions: tc.suspendOptions}
			if tc.ffEnabled {
				job.Variables = append(job.Variables, spec.Variable{Key: featureflags.SuspendableEnvironments, Value: "true"})
			}
			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					Build: &common.Build{Job: job, Runner: &common.RunnerConfig{}},
				},
			}
			assert.Equal(t, tc.want, e.usesSuspendResume())
		})
	}
}

func TestGenerateSuspendPVCName_Format(t *testing.T) {
	name := generateSuspendPVCName()

	require.True(t, strings.HasPrefix(name, "gl-runner-env-"), "expected prefix 'gl-runner-env-', got %q", name)
	assert.Len(t, name, 26) // "gl-runner-env-" (14) + 12 hex chars
}

func TestOverlayConfigMapName(t *testing.T) {
	e := &executor{
		AbstractExecutor: executors.AbstractExecutor{
			Build: &common.Build{Job: spec.Job{ID: 42}, Runner: &common.RunnerConfig{}},
		},
	}
	assert.Equal(t, "gl-runner-overlay-42", e.overlayConfigMapName())
}

func TestEnsureSuspendRootfsPVC(t *testing.T) {
	tests := []struct {
		name             string
		storageClass     string
		pvcSize          string
		wantErr          bool
		wantStorageClass *string
	}{
		{name: "creates PVC with ReadWriteOnce"},
		{name: "storage class is set on PVC", storageClass: "fast-ssd", wantStorageClass: new("fast-ssd")},
		{name: "invalid size returns error", pvcSize: "not-a-quantity", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{
				SuspendPVCStorageClass: tt.storageClass,
				SuspendPVCSize:         tt.pvcSize,
			})
			ctx := t.Context()
			err := e.ensureSuspendRootfsPVC(ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			pvc, getErr := e.kubeClient.CoreV1().PersistentVolumeClaims("test-ns").Get(ctx, e.suspendRootfsPVCName, metav1.GetOptions{})
			require.NoError(t, getErr)
			require.Len(t, pvc.Spec.AccessModes, 1)
			assert.Equal(t, "ReadWriteOnce", string(pvc.Spec.AccessModes[0]))
			assert.Equal(t, tt.wantStorageClass, pvc.Spec.StorageClassName)
		})
	}
}

func TestEnsureSuspendRootfsPVC_Retry_ReusesExistingPVC(t *testing.T) {
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
	ctx := t.Context()

	require.NoError(t, e.ensureSuspendRootfsPVC(ctx))
	firstName := e.suspendRootfsPVCName
	require.NotEmpty(t, firstName)

	require.NoError(t, e.ensureSuspendRootfsPVC(ctx))
	assert.Equal(t, firstName, e.suspendRootfsPVCName,
		"retry must reuse the PVC from the first attempt, not mint a new one")

	list, err := e.kubeClient.CoreV1().PersistentVolumeClaims("test-ns").List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, list.Items, 1, "retry must not create a second, orphaned PVC")
}

func TestDeleteSuspendRootfsPVC(t *testing.T) {
	tests := []struct {
		name       string
		pvcName    string
		suspended  bool
		isResume   bool
		pod        *api.Pod
		wantDelete bool
	}{
		{
			name:       "When no PVC exists, no-op",
			pvcName:    "",
			pod:        &api.Pod{},
			wantDelete: false,
		},
		{
			name:       "If a regular job is completed, delete the attached PVC",
			pvcName:    "gl-runner-env-abc",
			pod:        &api.Pod{},
			wantDelete: true,
		},
		{
			name:       "When a regular job pod is already deleted, delete the attached PVC",
			pvcName:    "gl-runner-env-abc",
			pod:        nil,
			wantDelete: true,
		},
		{
			name:       "When a resumed job is completed which expects another follow up resume job then do not delete the inherited PVC",
			pvcName:    "gl-runner-env-abc",
			suspended:  true,
			isResume:   true,
			pod:        &api.Pod{},
			wantDelete: false,
		},
		{
			name:       "When a resume job is completed then delete the inherited PVC",
			pvcName:    "gl-runner-env-abc",
			isResume:   true,
			pod:        &api.Pod{},
			wantDelete: true,
		},
		{
			name:       "When a resume job is not found, likely because prepare failed then do not delete the attached PVC",
			pvcName:    "gl-runner-env-abc",
			isResume:   true,
			pod:        nil,
			wantDelete: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
			e.suspendRootfsPVCName = tc.pvcName
			e.suspended = tc.suspended
			e.isResume = tc.isResume
			e.pod = tc.pod

			e.deleteSuspendRootfsPVC(t.Context())

			assert.Equal(t, tc.wantDelete, hasDelete(e.kubeClient.(*fake.Clientset), "persistentvolumeclaims"))
		})
	}
}

func TestCreateOverlayConfigMap(t *testing.T) {
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
	ctx := t.Context()

	err := e.createOverlayConfigMap(ctx)
	require.NoError(t, err)

	cm, getErr := e.kubeClient.CoreV1().ConfigMaps("test-ns").Get(ctx, e.overlayConfigMapName(), metav1.GetOptions{})
	require.NoError(t, getErr)

	assert.Empty(t, cm.OwnerReferences)
	val, ok := cm.Data[overlayScriptKey]
	require.True(t, ok)
	assert.Equal(t, overlayEntrypointScript, val)
}

func TestDeleteOverlayConfigMap_Deletes(t *testing.T) {
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
	ctx := t.Context()

	err := e.createOverlayConfigMap(ctx)
	require.NoError(t, err, "createOverlayConfigMap must succeed before testing delete")

	e.deleteOverlayConfigMap(ctx)

	_, getErr := e.kubeClient.CoreV1().ConfigMaps("test-ns").Get(ctx, e.overlayConfigMapName(), metav1.GetOptions{})
	require.Error(t, getErr, "ConfigMap should not exist after deleteOverlayConfigMap")
}

func TestSuspendVolumes(t *testing.T) {
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
	e.suspendRootfsPVCName = "my-rootfs-pvc"

	vols := e.suspendVolumes()
	require.Len(t, vols, 2)

	pvcVol, cmVol := vols[0], vols[1]

	require.NotNil(t, pvcVol.VolumeSource.PersistentVolumeClaim)
	assert.Equal(t, "my-rootfs-pvc", pvcVol.VolumeSource.PersistentVolumeClaim.ClaimName)

	require.NotNil(t, cmVol.VolumeSource.ConfigMap)
	require.NotNil(t, cmVol.VolumeSource.ConfigMap.DefaultMode)
	assert.Equal(t, int32(0o755), *cmVol.VolumeSource.ConfigMap.DefaultMode)
}

func TestSuspend_ReturnsPVCField(t *testing.T) {
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
	e.pod = &api.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"}}
	e.suspendRootfsPVCName = "test-rootfs-pvc"

	fields, err := e.Suspend(t.Context())
	require.NoError(t, err)

	assert.True(t, e.suspended)
	assert.Equal(t, "test-rootfs-pvc", fields.Get(envKeyPVC))
	assert.Equal(t, "test-ns", fields.Get(envKeyNamespace))
}

func TestSuspend_Errors(t *testing.T) {
	tests := []struct {
		name            string
		pod             *api.Pod
		pvcName         string
		wantErrContains string
	}{
		{
			name:            "nil pod",
			pvcName:         "test-rootfs-pvc",
			wantErrContains: "no pod",
		},
		{
			name:            "no rootfs PVC name",
			pod:             &api.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"}},
			wantErrContains: "no rootfs PVC was created",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
			e.pod = tt.pod
			e.suspendRootfsPVCName = tt.pvcName

			_, err := e.Suspend(t.Context())

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrContains)
		})
	}
}

func TestResume_ValidFields_SetsPVCNameAndIsResume(t *testing.T) {
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
	_, err := e.kubeClient.CoreV1().Namespaces().Create(t.Context(),
		&api.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "resume-ns"}}, metav1.CreateOptions{})
	require.NoError(t, err)

	err = e.Resume(t.Context(), url.Values{
		envKeyPVC:       {"rootfs-pvc"},
		envKeyNamespace: {"resume-ns"},
	})
	require.NoError(t, err)

	assert.Equal(t, "rootfs-pvc", e.suspendRootfsPVCName)
	assert.Equal(t, "resume-ns", e.configurationOverwrites.namespace)
	assert.True(t, e.isResume)
}

func TestResume_NamespaceDoesNotExist_ReturnsError(t *testing.T) {
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})

	err := e.Resume(t.Context(), url.Values{
		envKeyPVC:       {"rootfs-pvc"},
		envKeyNamespace: {"deleted-ns"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleted-ns")
	assert.Contains(t, err.Error(), "no longer exists")

	// Failed existence check must not leave partial state behind.
	assert.False(t, e.isResume)
	assert.Empty(t, e.suspendRootfsPVCName)
}

func newTestExecutorForResume(t *testing.T) *executor {
	t.Helper()
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
	e.Build.Runner.RunnerCredentials.ID = 7
	e.Build.Runner.SystemID = "my-system"
	e.Build.Job.Variables = append(e.Build.Job.Variables,
		spec.Variable{Key: featureflags.SuspendableEnvironments, Value: "true"},
	)
	return e
}

func TestPrepareResume_Error(t *testing.T) {
	tests := []struct {
		name            string
		envKey          string
		wantErrContains string
	}{
		{
			name:            "malformed env key",
			envKey:          "not/a/valid/env/key/with/too/many/slashes/and/garbage",
			wantErrContains: "parse environment key",
		},
		{
			name: "runner ID mismatch",
			envKey: common.RuntimeEnvironmentKey{
				RunnerID: 99,
				SystemID: "my-system",
				Fields:   url.Values{envKeyPVC: {"rootfs-pvc"}},
			}.String(),
			wantErrContains: "runner ID",
		},
		{
			name: "system ID mismatch",
			envKey: common.RuntimeEnvironmentKey{
				RunnerID: 7,
				SystemID: "other-system",
				Fields:   url.Values{envKeyPVC: {"rootfs-pvc"}},
			}.String(),
			wantErrContains: "system ID",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestExecutorForResume(t)
			e.Build.Job.SuspendOptions.RuntimeEnvironmentKey = tc.envKey

			options := common.ExecutorPrepareOptions{
				Context: t.Context(),
				Build:   e.Build,
			}
			err := e.prepareResume(options)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErrContains)
		})
	}
}

func TestResume_FieldValidation_Error(t *testing.T) {
	tests := []struct {
		name            string
		fields          url.Values
		wantErrContains string
	}{
		{
			name:            "missing namespace field",
			fields:          url.Values{envKeyPVC: {"rootfs-pvc"}},
			wantErrContains: "namespace",
		},
		{
			name:            "missing pvc field",
			fields:          url.Values{envKeyNamespace: {"test-ns"}},
			wantErrContains: "pvc",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestExecutorForResume(t)
			err := e.Resume(t.Context(), tc.fields)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErrContains)
		})
	}
}

func TestGuardSuspendSecurityContext(t *testing.T) {
	tests := []struct {
		name            string
		cfg             *common.KubernetesConfig
		suspendable     bool
		wantErrContains string
	}{
		{
			// Not an error: runner injects SYS_ADMIN automatically via injectSuspendCap.
			name:        "no privileged or sys_admin is ok",
			cfg:         &common.KubernetesConfig{},
			suspendable: true,
		},
		{
			name: "privileged container is ok",
			cfg: &common.KubernetesConfig{
				BuildContainerSecurityContext: common.KubernetesContainerSecurityContext{
					Privileged: new(true),
				},
			},
			suspendable: true,
		},
		{
			name:        "privileged at config level falls back to container",
			cfg:         &common.KubernetesConfig{Privileged: new(true)},
			suspendable: true,
		},
		{
			name: "sys_admin capability is ok",
			cfg: &common.KubernetesConfig{
				BuildContainerSecurityContext: common.KubernetesContainerSecurityContext{
					Capabilities: &common.KubernetesContainerCapabilities{
						Add: []api.Capability{capSysAdmin},
					},
				},
			},
			suspendable: true,
		},
		{
			name: "container run_as_non_root returns error",
			cfg: &common.KubernetesConfig{
				BuildContainerSecurityContext: common.KubernetesContainerSecurityContext{
					Privileged:   new(true),
					RunAsNonRoot: new(true),
				},
			},
			suspendable:     true,
			wantErrContains: "run_as_non_root",
		},
		{
			name: "pod-level run_as_non_root returns error",
			cfg: &common.KubernetesConfig{
				BuildContainerSecurityContext: common.KubernetesContainerSecurityContext{
					Privileged: new(true),
				},
				PodSecurityContext: common.KubernetesPodSecurityContext{
					RunAsNonRoot: new(true),
				},
			},
			suspendable:     true,
			wantErrContains: "run_as_non_root",
		},
		{
			// Kubernetes lets the container-level run_as_non_root override the pod-level
			// one. A hardened pod (run_as_non_root=true) with the build container
			// explicitly opted out (false) runs as root, so the guard must not fire.
			name: "container run_as_non_root false overrides pod-level true",
			cfg: &common.KubernetesConfig{
				BuildContainerSecurityContext: common.KubernetesContainerSecurityContext{
					Privileged:   new(true),
					RunAsNonRoot: new(false),
				},
				PodSecurityContext: common.KubernetesPodSecurityContext{
					RunAsNonRoot: new(true),
				},
			},
			suspendable: true,
		},
		{
			// Writes target /persist + overlay, not the container rootfs, so a
			// read-only root filesystem is deliberately not a hard error.
			name: "read_only_root_filesystem is not a hard error",
			cfg: &common.KubernetesConfig{
				BuildContainerSecurityContext: common.KubernetesContainerSecurityContext{
					Privileged:             new(true),
					ReadOnlyRootFilesystem: new(true),
				},
			},
			suspendable: true,
		},
		{
			name: "non-suspendable job is a no-op",
			cfg: &common.KubernetesConfig{
				BuildContainerSecurityContext: common.KubernetesContainerSecurityContext{
					RunAsNonRoot: new(true),
				},
			},
			suspendable: false,
		},
		{
			name: "container run_as_user non-zero returns error",
			cfg: &common.KubernetesConfig{
				BuildContainerSecurityContext: common.KubernetesContainerSecurityContext{
					RunAsUser: new(int64(1000)),
				},
			},
			suspendable:     true,
			wantErrContains: "run_as_user",
		},
		{
			name: "container run_as_user zero is ok",
			cfg: &common.KubernetesConfig{
				BuildContainerSecurityContext: common.KubernetesContainerSecurityContext{
					RunAsUser: new(int64(0)),
				},
			},
			suspendable: true,
		},
		{
			name: "pod-level run_as_user non-zero returns error",
			cfg: &common.KubernetesConfig{
				PodSecurityContext: common.KubernetesPodSecurityContext{
					RunAsUser: new(int64(1000)),
				},
			},
			suspendable:     true,
			wantErrContains: "run_as_user",
		},
		{
			name: "container run_as_user zero overrides pod-level non-zero",
			cfg: &common.KubernetesConfig{
				BuildContainerSecurityContext: common.KubernetesContainerSecurityContext{
					RunAsUser: new(int64(0)),
				},
				PodSecurityContext: common.KubernetesPodSecurityContext{
					RunAsUser: new(int64(1000)),
				},
			},
			suspendable: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestExecutorWithKubeClient(t, tt.cfg)
			e.DefaultBuildsDir = "/builds"
			e.options = &kubernetesOptions{Services: make(map[string]*spec.Image)}
			if tt.suspendable {
				markSuspendableFFOn(e)
				require.True(t, e.usesSuspendResume())
			} else {
				require.False(t, e.usesSuspendResume())
			}

			err := e.guardSuspendSecurityContext()

			if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGuardSuspendCapabilityDrop(t *testing.T) {
	tests := []struct {
		name            string
		cfg             *common.KubernetesConfig
		services        map[string]*spec.Image
		suspendable     bool
		wantErrContains string
	}{
		{
			name:        "no drop conflict is ok",
			cfg:         &common.KubernetesConfig{},
			suspendable: true,
		},
		{
			name: "sys_admin in build container cap_drop returns error",
			cfg: &common.KubernetesConfig{
				BuildContainerSecurityContext: common.KubernetesContainerSecurityContext{
					Capabilities: &common.KubernetesContainerCapabilities{
						Drop: []api.Capability{capSysAdmin},
					},
				},
			},
			suspendable:     true,
			wantErrContains: "cap_drop",
		},
		{
			name: "lowercase sys_admin in cap_drop still returns error",
			cfg: &common.KubernetesConfig{
				BuildContainerSecurityContext: common.KubernetesContainerSecurityContext{
					Capabilities: &common.KubernetesContainerCapabilities{
						Drop: []api.Capability{"sys_admin"},
					},
				},
			},
			suspendable:     true,
			wantErrContains: "cap_drop",
		},
		{
			name: "ALL in global cap_drop returns error",
			cfg: &common.KubernetesConfig{
				CapDrop: []string{"ALL"},
			},
			suspendable:     true,
			wantErrContains: "cap_drop",
		},
		{
			name: "privileged container bypasses the drop check",
			cfg: &common.KubernetesConfig{
				BuildContainerSecurityContext: common.KubernetesContainerSecurityContext{
					Privileged: new(true),
					Capabilities: &common.KubernetesContainerCapabilities{
						Drop: []api.Capability{"ALL"},
					},
				},
			},
			suspendable: true,
		},
		{
			name: "privileged at config level bypasses the drop check",
			cfg: &common.KubernetesConfig{
				Privileged: new(true),
				CapDrop:    []string{"ALL"},
			},
			suspendable: true,
		},
		{
			name: "overlay-wrapped service with sys_admin cap_drop returns error",
			cfg: &common.KubernetesConfig{
				ServiceContainerSecurityContext: common.KubernetesContainerSecurityContext{
					Capabilities: &common.KubernetesContainerCapabilities{
						Drop: []api.Capability{capSysAdmin},
					},
				},
			},
			services: map[string]*spec.Image{
				"svc": {Name: "svc", Command: []string{"start"}},
			},
			suspendable:     true,
			wantErrContains: "cap_drop",
		},
		{
			name: "service without entrypoint or command is not overlay-wrapped, so not checked",
			cfg: &common.KubernetesConfig{
				ServiceContainerSecurityContext: common.KubernetesContainerSecurityContext{
					Capabilities: &common.KubernetesContainerCapabilities{
						Drop: []api.Capability{"ALL"},
					},
				},
			},
			services: map[string]*spec.Image{
				"svc": {Name: "svc"},
			},
			suspendable: true,
		},
		{
			name: "non-suspendable job is a no-op",
			cfg: &common.KubernetesConfig{
				CapDrop: []string{"ALL"},
			},
			suspendable: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestExecutorWithKubeClient(t, tt.cfg)
			e.DefaultBuildsDir = "/builds"
			services := tt.services
			if services == nil {
				services = make(map[string]*spec.Image)
			}
			e.options = &kubernetesOptions{Services: services}
			if tt.suspendable {
				markSuspendableFFOn(e)
				require.True(t, e.usesSuspendResume())
			} else {
				require.False(t, e.usesSuspendResume())
			}

			err := e.guardSuspendCapabilityDrop()

			if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInjectSuspendCap_NilSecurityContext_AddsCapAndContext(t *testing.T) {
	c := &api.Container{}
	injectSuspendCap(c)
	require.NotNil(t, c.SecurityContext)
	require.NotNil(t, c.SecurityContext.Capabilities)
	assert.Contains(t, c.SecurityContext.Capabilities.Add, capSysAdmin)
}

func TestInjectSuspendCap_ExistingCaps_AppendsWithoutDuplicate(t *testing.T) {
	c := &api.Container{
		SecurityContext: &api.SecurityContext{
			Capabilities: &api.Capabilities{Add: []api.Capability{"NET_ADMIN"}},
		},
	}
	injectSuspendCap(c)
	injectSuspendCap(c) // call twice — must not duplicate
	count := 0
	for _, cap := range c.SecurityContext.Capabilities.Add {
		if cap == capSysAdmin {
			count++
		}
	}
	assert.Equal(t, 1, count, "SYS_ADMIN must appear exactly once")
	assert.Contains(t, c.SecurityContext.Capabilities.Add, api.Capability("NET_ADMIN"), "existing caps must be preserved")
}

func TestIsHostUserEnabled_SuspendableJob_ReturnsFalse(t *testing.T) {
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
	e.DefaultBuildsDir = "/builds"
	markSuspendableFFOn(e)
	hu := e.isHostUserEnabled()
	require.NotNil(t, hu)
	assert.False(t, *hu)
}

func TestIsHostUserEnabled_NonSuspendableJob_ReturnsNil(t *testing.T) {
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
	assert.Nil(t, e.isHostUserEnabled())
}

func TestGuardSuspendCompatibility_UseConcrete(t *testing.T) {
	tests := []struct {
		name            string
		suspendable     bool
		useConcrete     bool
		wantErrContains string
	}{
		{
			name:            "FF_CONCRETE with suspend options returns error",
			suspendable:     true,
			useConcrete:     true,
			wantErrContains: "FF_CONCRETE",
		},
		{
			name:        "FF_CONCRETE without suspend options is a no-op",
			suspendable: false,
			useConcrete: true,
		},
		{
			name:        "suspend options without FF_CONCRETE is fine",
			suspendable: true,
			useConcrete: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
			e.DefaultBuildsDir = "/builds"
			e.Build.Runner.RunnerCredentials.ID = 1
			e.options = &kubernetesOptions{Services: make(map[string]*spec.Image)}
			if tt.suspendable {
				markSuspendableFFOn(e)
			}
			if tt.useConcrete {
				e.Build.Job.Variables = append(e.Build.Job.Variables,
					spec.Variable{Key: featureflags.UseConcrete, Value: "true"},
				)
			}

			err := e.guardSuspendCompatibility()

			if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGuardSuspendReservedMountPaths(t *testing.T) {
	tests := []struct {
		name            string
		cfg             *common.KubernetesConfig
		suspendable     bool
		setup           func(e *executor)
		wantErrContains []string
	}{
		{
			name: "volume at /builds returns error with path and volume name",
			cfg: &common.KubernetesConfig{
				Volumes: common.KubernetesVolumes{
					EmptyDirs: []common.KubernetesEmptyDir{
						{Name: "scratch", MountPath: "/builds"},
					},
				},
			},
			suspendable:     true,
			wantErrContains: []string{"/builds", "scratch"},
		},
		{
			name: "volume at suspend PVC mount path returns error",
			cfg: &common.KubernetesConfig{
				Volumes: common.KubernetesVolumes{
					HostPaths: []common.KubernetesHostPath{
						{Name: "hp", MountPath: suspendPVCMountPath, HostPath: "/data"},
					},
				},
			},
			suspendable:     true,
			wantErrContains: []string{suspendPVCMountPath},
		},
		{
			name: "volume at overlay script dir returns error",
			cfg: &common.KubernetesConfig{
				Volumes: common.KubernetesVolumes{
					EmptyDirs: []common.KubernetesEmptyDir{
						{Name: "init", MountPath: overlayScriptDir},
					},
				},
			},
			suspendable:     true,
			wantErrContains: []string{overlayScriptDir},
		},
		{
			name: "volume at custom builds_dir returns error",
			cfg: &common.KubernetesConfig{
				Volumes: common.KubernetesVolumes{
					EmptyDirs: []common.KubernetesEmptyDir{
						{Name: "scratch", MountPath: "/mnt/builds"},
					},
				},
			},
			suspendable:     true,
			setup:           func(e *executor) { e.Config.BuildsDir = "/mnt/builds" },
			wantErrContains: []string{"/mnt/builds"},
		},
		{
			// Self-collision: builds_dir set to /persist would mount two volumes there.
			name:            "builds_dir set to reserved path returns error",
			cfg:             &common.KubernetesConfig{},
			suspendable:     true,
			setup:           func(e *executor) { e.Config.BuildsDir = suspendPVCMountPath },
			wantErrContains: []string{"builds_dir"},
		},
		{
			name: "non-colliding volume is ok",
			cfg: &common.KubernetesConfig{
				Volumes: common.KubernetesVolumes{
					EmptyDirs: []common.KubernetesEmptyDir{
						{Name: "scratch", MountPath: "/data"},
					},
				},
			},
			suspendable: true,
		},
		{
			name: "non-suspendable job is a no-op",
			cfg: &common.KubernetesConfig{
				Volumes: common.KubernetesVolumes{
					EmptyDirs: []common.KubernetesEmptyDir{
						{Name: "scratch", MountPath: "/builds"},
					},
				},
			},
			suspendable: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestExecutorWithKubeClient(t, tt.cfg)
			e.DefaultBuildsDir = "/builds"
			if tt.suspendable {
				markSuspendableFFOn(e)
				require.True(t, e.usesSuspendResume())
			} else {
				require.False(t, e.usesSuspendResume())
			}
			if tt.setup != nil {
				tt.setup(e)
			}

			err := e.guardSuspendReservedMountPaths()

			if len(tt.wantErrContains) > 0 {
				require.Error(t, err)
				for _, want := range tt.wantErrContains {
					assert.Contains(t, err.Error(), want)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSuspendRebindPaths(t *testing.T) {
	tests := []struct {
		name  string
		extra []string
		cfg   *common.KubernetesConfig
		want  string
	}{
		{
			name:  "extras and volumes combined, parent sorts before child",
			extra: []string{"/mnt/builds/project-123"},
			cfg: &common.KubernetesConfig{
				Volumes: common.KubernetesVolumes{
					HostPaths: []common.KubernetesHostPath{{Name: "hp", MountPath: "/mnt"}},
				},
			},
			want: "/mnt\n/mnt/builds/project-123",
		},
		{
			name:  "duplicate between extra and a configured volume is deduped",
			extra: []string{"/builds"},
			cfg: &common.KubernetesConfig{
				Volumes: common.KubernetesVolumes{
					ConfigMaps: []common.KubernetesConfigMap{{Name: "dup", MountPath: "/builds"}},
				},
			},
			want: "/builds",
		},
		{
			name: "both reserved paths excluded, non-colliding volume kept",
			cfg: &common.KubernetesConfig{
				Volumes: common.KubernetesVolumes{
					EmptyDirs: []common.KubernetesEmptyDir{
						{Name: "a", MountPath: suspendPVCMountPath},
						{Name: "b", MountPath: overlayScriptDir},
						{Name: "c", MountPath: "/probe"},
					},
				},
			},
			want: "/probe",
		},
		{
			name:  "path containing a newline is rejected outright",
			extra: []string{"/builds\n/var/run/secrets/kubernetes.io/serviceaccount"},
			want:  "",
		},
		{
			name:  "trailing slash and .. are normalized before dedup",
			extra: []string{"/data/", "/data/../data"},
			want:  "/data",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestExecutorWithKubeClient(t, tt.cfg)
			e.DefaultBuildsDir = "/builds"
			markSuspendableFFOn(e)

			assert.Equal(t, tt.want, e.suspendRebindPaths(tt.extra...))
		})
	}
}

func TestOverlayServiceContainer_SetsRebindPathsEnvVar(t *testing.T) {
	e := newSuspendableContainerExecutor(t, "test-image:latest")
	e.Config.Kubernetes.Volumes.Secrets = []common.KubernetesSecret{
		{Name: "probe-secret", MountPath: "/probe-secret"},
	}

	c := &api.Container{Command: []string{"redis-server"}}
	e.overlayServiceContainer(c, "redis", &spec.Image{Name: "redis:7-alpine"})

	env := make(map[string]string, len(c.Env))
	for _, ev := range c.Env {
		env[ev.Name] = ev.Value
	}

	assert.Equal(t, "/probe-secret", env["CI_SUSPEND_REBIND_PATHS"])
}

func injectReactorError(e *executor, verb, resource string, retErr error) {
	e.kubeClient.(*fake.Clientset).PrependReactor(verb, resource,
		func(_ k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, retErr
		},
	)
}

func TestEnsureSuspendResources(t *testing.T) {
	tests := []struct {
		name          string
		suspendable   bool
		isResume      bool
		setupReactor  func(e *executor)
		wantPVCCreate bool
		wantCMCreate  bool
		wantErr       string
	}{
		{
			name:          "non-suspendable — no API calls",
			suspendable:   false,
			wantPVCCreate: false,
			wantCMCreate:  false,
		},
		{
			name:          "first run — creates PVC and ConfigMap",
			suspendable:   true,
			isResume:      false,
			wantPVCCreate: true,
			wantCMCreate:  true,
		},
		{
			name:          "resume — skips PVC, creates ConfigMap",
			suspendable:   true,
			isResume:      true,
			wantPVCCreate: false,
			wantCMCreate:  true,
		},
		{
			name:        "PVC create fails",
			suspendable: true,
			setupReactor: func(e *executor) {
				injectReactorError(e, "create", "persistentvolumeclaims", errors.New("quota exceeded"))
			},
			wantErr: "creating suspend PVC",
		},
		{
			name:        "ConfigMap create fails",
			suspendable: true,
			setupReactor: func(e *executor) {
				injectReactorError(e, "create", "configmaps", errors.New("quota exceeded"))
			},
			wantErr: "creating overlay ConfigMap",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var e *executor
			if tc.suspendable {
				e = newSuspendableExecutor(t)
			} else {
				e = newTestExecutorWithKubeClient(t, &common.KubernetesConfig{SuspendPVCSize: "10Gi"})
			}
			e.isResume = tc.isResume
			if tc.setupReactor != nil {
				tc.setupReactor(e)
			}

			err := e.ensureSuspendResources(t.Context())

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			actions := e.kubeClient.(*fake.Clientset).Actions()
			hasPVC := false
			hasCM := false
			for _, a := range actions {
				if a.GetVerb() == "create" && a.GetResource().Resource == "persistentvolumeclaims" {
					hasPVC = true
				}
				if a.GetVerb() == "create" && a.GetResource().Resource == "configmaps" {
					hasCM = true
				}
			}
			assert.Equal(t, tc.wantPVCCreate, hasPVC)
			assert.Equal(t, tc.wantCMCreate, hasCM)
		})
	}
}

func TestDeleteOverlayConfigMap_NilBuild_NoOp(t *testing.T) {
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
	e.Build = nil
	assert.NotPanics(t, func() { e.deleteOverlayConfigMap(t.Context()) })
	assert.False(t, hasDelete(e.kubeClient.(*fake.Clientset), "configmaps"))
}

func TestDeleteOverlayConfigMap_OtherError_NoPanic(t *testing.T) {
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
	e.Build.Job.ID = 42
	injectReactorError(e, "delete", "configmaps", errors.New("internal error"))
	assert.NotPanics(t, func() { e.deleteOverlayConfigMap(t.Context()) })
}

func TestCreateOverlayConfigMap_Retry_UpdatesExisting(t *testing.T) {
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
	e.Build.Job.ID = 42
	ctx := t.Context()

	require.NoError(t, e.createOverlayConfigMap(ctx))

	err := e.createOverlayConfigMap(ctx)
	require.NoError(t, err, "retrying with an existing ConfigMap must update, not fail")

	cm, getErr := e.kubeClient.CoreV1().ConfigMaps("test-ns").Get(ctx, e.overlayConfigMapName(), metav1.GetOptions{})
	require.NoError(t, getErr)
	assert.Equal(t, overlayEntrypointScript, cm.Data[overlayScriptKey])
}

func TestCreateOverlayConfigMap_CreateFails_ReturnsError(t *testing.T) {
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
	e.Build.Job.ID = 42
	injectReactorError(e, "create", "configmaps", errors.New("quota exceeded"))
	err := e.createOverlayConfigMap(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create overlay ConfigMap")
}

func TestResume_NilConfigurationOverwrites_ReturnsError(t *testing.T) {
	e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
	e.configurationOverwrites = nil
	err := e.Resume(t.Context(), url.Values{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "executor not initialized")
}

func TestSuspendServiceDataMount(t *testing.T) {
	m := suspendServiceDataMount("redis", "/data")

	assert.Equal(t, suspendPVCVolName, m.Name, "must reuse existing PVC volume, not declare a new one")
	assert.Equal(t, "/data", m.MountPath)
	assert.Equal(t, "services/redis/data", m.SubPath)
	assert.Empty(t, m.SubPathExpr, "SubPath and SubPathExpr are mutually exclusive; SubPathExpr must be empty")
}
