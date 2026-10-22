//go:build !integration

package kubernetes

import (
	"encoding/json"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestEnsureSuspendRootfsPVC_LabelsRunnerIDAndExpiry(t *testing.T) {
	ttl := 48 * time.Hour
	e := newSuspendableExecutor(t)
	e.Build.Runner.RunnerCredentials.ID = 42
	e.Config.RuntimeEnvironmentTTL = &ttl

	before := time.Now()
	require.NoError(t, e.ensureSuspendRootfsPVC(t.Context()))
	after := time.Now()

	pvc, err := e.kubeClient.CoreV1().PersistentVolumeClaims("test-ns").
		Get(t.Context(), e.suspendRootfsPVCName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "42", pvc.Labels[common.RuntimeEnvironmentRunnerIDLabel])

	expiresAt, err := common.ParseRuntimeEnvironmentExpiry(pvc.Labels[common.RuntimeEnvironmentExpiresAtLabel])
	require.NoError(t, err)
	assert.False(t, expiresAt.Before(before.Add(ttl).Truncate(time.Second)))
	assert.False(t, expiresAt.After(after.Add(ttl)))
}

// The expiry stamped at creation wins over the TTL of the manager that
// resumes: here the creation time plus the resuming manager's TTL is long
// past, but the stamp is not.
func TestResume_HonorsStampedExpiryOverCurrentTTL(t *testing.T) {
	stampTTL := 30 * 24 * time.Hour
	resumeTTL := time.Hour

	e := newSuspendableExecutor(t)
	e.Config.RuntimeEnvironmentTTL = &stampTTL
	_, err := e.kubeClient.CoreV1().Namespaces().Create(t.Context(),
		&api.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-ns"}}, metav1.CreateOptions{})
	require.NoError(t, err)
	require.NoError(t, e.ensureSuspendRootfsPVC(t.Context()))

	pvcs := e.kubeClient.CoreV1().PersistentVolumeClaims("test-ns")
	pvc, err := pvcs.Get(t.Context(), e.suspendRootfsPVCName, metav1.GetOptions{})
	require.NoError(t, err)
	pvc.CreationTimestamp = metav1.NewTime(time.Now().Add(-2 * resumeTTL))
	_, err = pvcs.Update(t.Context(), pvc, metav1.UpdateOptions{})
	require.NoError(t, err)

	r := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
	r.kubeClient = e.kubeClient
	r.Config.RuntimeEnvironmentTTL = &resumeTTL

	err = r.Resume(t.Context(), url.Values{envKeyPVC: {pvc.Name}, envKeyNamespace: {"test-ns"}})
	require.NoError(t, err)
	assert.True(t, r.isResume)
}

func TestEnsureSuspendResources_NamespacePerJobLabelsNamespace(t *testing.T) {
	tests := []struct {
		name            string
		namespacePerJob bool
		isResume        bool
		wantLabel       bool
	}{
		{name: "namespace per job, first run", namespacePerJob: true, wantLabel: true},
		{name: "namespace per job, resume keeps existing labels", namespacePerJob: true, isResume: true},
		{name: "shared namespace is never labelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newSuspendableExecutor(t)
			e.Build.Runner.RunnerCredentials.ID = 42
			e.Config.Kubernetes.NamespacePerJob = tt.namespacePerJob
			e.isResume = tt.isResume
			if tt.isResume {
				e.suspendRootfsPVCName = "existing"
			}
			_, err := e.kubeClient.CoreV1().Namespaces().Create(t.Context(),
				&api.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-ns"}}, metav1.CreateOptions{})
			require.NoError(t, err)

			require.NoError(t, e.ensureSuspendResources(t.Context()))

			ns, err := e.kubeClient.CoreV1().Namespaces().Get(t.Context(), "test-ns", metav1.GetOptions{})
			require.NoError(t, err)
			if tt.wantLabel {
				assert.Equal(t, "42", ns.Labels[common.RuntimeEnvironmentRunnerIDLabel])
				return
			}
			assert.NotContains(t, ns.Labels, common.RuntimeEnvironmentRunnerIDLabel)
		})
	}
}

func TestLabelsMergePatch_EscapesJSON(t *testing.T) {
	// %q would emit Go escapes such as \x7f and \a, which are not valid JSON.
	labels := map[string]string{`key"\`: "v\x7f\a\u2028<>&"}

	patch, err := labelsMergePatch(labels)
	require.NoError(t, err)

	var got struct {
		Metadata struct {
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
	}
	require.NoError(t, json.Unmarshal(patch, &got))
	assert.Equal(t, labels, got.Metadata.Labels)
}

func TestEnsureSuspendResources_NamespaceLabelFailureDoesNotFailJob(t *testing.T) {
	e := newSuspendableExecutor(t)
	e.Config.Kubernetes.NamespacePerJob = true
	injectReactorError(e, "patch", "namespaces", errors.New("forbidden"))

	// Without cleanup permissions suspend and resume must still work.
	require.NoError(t, e.ensureSuspendResources(t.Context()))
	assert.NotEmpty(t, e.suspendRootfsPVCName)

	pvc, err := e.kubeClient.CoreV1().PersistentVolumeClaims("test-ns").
		Get(t.Context(), e.suspendRootfsPVCName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, e.runtimeEnvironmentRunnerID(), pvc.Labels[common.RuntimeEnvironmentRunnerIDLabel])
}

func TestResume_RuntimeEnvironmentAvailability(t *testing.T) {
	tests := []struct {
		name        string
		namespace   bool
		nsDeleting  bool
		pvc         *api.PersistentVolumeClaim
		reactorVerb string
		wantGone    bool
		wantErr     string
	}{
		{
			name:      "fresh PVC resumes",
			namespace: true,
			pvc:       resumePVC(time.Now().Add(47 * time.Hour)),
		},
		{
			name:      "PVC just before its expiry resumes",
			namespace: true,
			pvc:       resumePVC(time.Now().Add(time.Minute)),
		},
		{
			name:      "PVC past its expiry is refused",
			namespace: true,
			pvc:       resumePVC(time.Now().Add(-time.Minute)),
			wantGone:  true,
			wantErr:   "expired",
		},
		{
			name:      "PVC without an expiry stamp is refused",
			namespace: true,
			pvc: func() *api.PersistentVolumeClaim {
				p := resumePVC(time.Now().Add(time.Hour))
				delete(p.Labels, common.RuntimeEnvironmentExpiresAtLabel)
				return p
			}(),
			wantGone: true,
			wantErr:  "has no expiry stamp",
		},
		{
			name:      "PVC with an unparseable expiry stamp is refused",
			namespace: true,
			pvc: func() *api.PersistentVolumeClaim {
				p := resumePVC(time.Now().Add(time.Hour))
				p.Labels[common.RuntimeEnvironmentExpiresAtLabel] = "soon"
				return p
			}(),
			wantGone: true,
			wantErr:  "expiry stamp",
		},
		{
			name:      "missing PVC",
			namespace: true,
			wantGone:  true,
			wantErr:   "rootfs-pvc",
		},
		{
			name:      "PVC being deleted",
			namespace: true,
			pvc: func() *api.PersistentVolumeClaim {
				p := resumePVC(time.Now().Add(time.Hour))
				p.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				p.Finalizers = []string{"kubernetes.io/pvc-protection"}
				return p
			}(),
			wantGone: true,
			wantErr:  "being deleted",
		},
		{
			name:     "missing namespace",
			wantGone: true,
			wantErr:  "no longer exists",
		},
		{
			name:       "namespace being deleted",
			namespace:  true,
			nsDeleting: true,
			pvc:        resumePVC(time.Now().Add(time.Hour)),
			wantGone:   true,
			wantErr:    "namespace \"resume-ns\" is being deleted",
		},
		{
			name:      "PVC of another runner is refused",
			namespace: true,
			pvc: func() *api.PersistentVolumeClaim {
				p := resumePVC(time.Now().Add(time.Hour))
				p.Labels[common.RuntimeEnvironmentRunnerIDLabel] = "999"
				return p
			}(),
			wantGone: true,
			wantErr:  "belongs to runner \"999\"",
		},
		{
			name:      "PVC without a runner ID is refused",
			namespace: true,
			pvc: func() *api.PersistentVolumeClaim {
				p := resumePVC(time.Now().Add(time.Hour))
				delete(p.Labels, common.RuntimeEnvironmentRunnerIDLabel)
				return p
			}(),
			wantGone: true,
			wantErr:  "has no runner ID label",
		},
		{
			name:        "transient PVC lookup error is retryable",
			namespace:   true,
			pvc:         resumePVC(time.Now().Add(time.Hour)),
			reactorVerb: "get",
			wantErr:     "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
			if tt.namespace {
				ns := &api.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "resume-ns"}}
				if tt.nsDeleting {
					ns.DeletionTimestamp = &metav1.Time{Time: time.Now()}
					ns.Finalizers = []string{"kubernetes"}
				}
				_, err := e.kubeClient.CoreV1().Namespaces().Create(t.Context(), ns, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			if tt.pvc != nil {
				_, err := e.kubeClient.CoreV1().PersistentVolumeClaims("resume-ns").
					Create(t.Context(), tt.pvc, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			if tt.reactorVerb != "" {
				injectReactorError(e, tt.reactorVerb, "persistentvolumeclaims", errors.New("boom"))
			}

			err := e.Resume(t.Context(), url.Values{
				envKeyPVC:       {"rootfs-pvc"},
				envKeyNamespace: {"resume-ns"},
			})

			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.True(t, e.isResume)
				assert.Equal(t, "rootfs-pvc", e.suspendRootfsPVCName)
				return
			}

			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
			assert.False(t, e.isResume)
			assert.Empty(t, e.suspendRootfsPVCName)

			var buildErr *common.BuildError
			if !tt.wantGone {
				assert.False(t, errors.As(err, &buildErr), "transient errors must stay retryable")
				return
			}
			require.True(t, errors.As(err, &buildErr), "want *common.BuildError, got %T", err)
			assert.Equal(t, common.RuntimeEnvironmentUnavailable, buildErr.FailureReason)
			assert.ErrorIs(t, err, common.ErrRuntimeEnvironmentUnavailable)
		})
	}
}

func TestResume_ForbiddenLookupIsNotRetried(t *testing.T) {
	tests := []struct {
		name     string
		resource string
	}{
		{name: "namespace get forbidden", resource: "namespaces"},
		{name: "PVC get forbidden", resource: "persistentvolumeclaims"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestExecutorWithKubeClient(t, &common.KubernetesConfig{})
			_, err := e.kubeClient.CoreV1().Namespaces().Create(t.Context(),
				&api.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "resume-ns"}}, metav1.CreateOptions{})
			require.NoError(t, err)
			injectReactorError(e, "get", tt.resource,
				kubeerrors.NewForbidden(schema.GroupResource{Resource: tt.resource}, "x", errors.New("denied")))

			err = e.Resume(t.Context(), url.Values{
				envKeyPVC:       {"rootfs-pvc"},
				envKeyNamespace: {"resume-ns"},
			})

			var buildErr *common.BuildError
			require.ErrorAs(t, err, &buildErr)
			assert.Equal(t, common.ConfigurationError, buildErr.FailureReason)
			assert.True(t, kubeerrors.IsForbidden(err))
			assert.ErrorContains(t, err, "RBAC permissions")
			assert.False(t, e.isResume)
		})
	}
}

func resumePVC(expiresAt time.Time) *api.PersistentVolumeClaim {
	return &api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rootfs-pvc",
			Namespace: "resume-ns",
			Labels: map[string]string{
				common.RuntimeEnvironmentExpiresAtLabel: common.FormatRuntimeEnvironmentExpiry(expiresAt),
				// the test executors run as runner ID 0
				common.RuntimeEnvironmentRunnerIDLabel: "0",
			},
		},
	}
}
