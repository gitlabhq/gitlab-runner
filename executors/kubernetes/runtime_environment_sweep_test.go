//go:build !integration

package kubernetes

import (
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var sweepNow = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// expiredAt is an expiry whose grace has passed at sweepNow.
func expiredAt() time.Time { return sweepNow.Add(-common.RuntimeEnvironmentSweepGrace - time.Minute) }

func sweepPVC(ns, name string, expiresAt time.Time, runnerID string) *api.PersistentVolumeClaim {
	pvc := &api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				common.RuntimeEnvironmentExpiresAtLabel: common.FormatRuntimeEnvironmentExpiry(expiresAt),
			},
		},
	}
	if runnerID != "" {
		pvc.Labels[common.RuntimeEnvironmentRunnerIDLabel] = runnerID
	}
	return pvc
}

func sweepPod(ns, name, claim string, phase api.PodPhase) *api.Pod {
	return &api.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: api.PodSpec{Volumes: []api.Volume{{
			Name: suspendPVCVolName,
			VolumeSource: api.VolumeSource{
				PersistentVolumeClaim: &api.PersistentVolumeClaimVolumeSource{ClaimName: claim},
			},
		}}},
		Status: api.PodStatus{Phase: phase},
	}
}

func sweepNamespace(name, runnerID string) *api.Namespace {
	ns := &api.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if runnerID != "" {
		ns.Labels = map[string]string{common.RuntimeEnvironmentRunnerIDLabel: runnerID}
	}
	return ns
}

func sweepRunnerConfig(k8s *common.KubernetesConfig) *common.RunnerConfig {
	return &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{ID: 7},
		RunnerSettings:    common.RunnerSettings{Kubernetes: k8s},
	}
}

// remainingPVCs reads the tracker directly so it bypasses error reactors.
func remainingPVCs(t *testing.T, client *fake.Clientset) []string {
	t.Helper()
	obj, err := client.Tracker().List(
		api.SchemeGroupVersion.WithResource("persistentvolumeclaims"),
		api.SchemeGroupVersion.WithKind("PersistentVolumeClaim"), "")
	require.NoError(t, err)
	list := obj.(*api.PersistentVolumeClaimList)
	names := make([]string, 0, len(list.Items))
	for _, p := range list.Items {
		names = append(names, p.Namespace+"/"+p.Name)
	}
	return names
}

func remainingNamespaces(t *testing.T, client *fake.Clientset) []string {
	t.Helper()
	list, err := client.CoreV1().Namespaces().List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	names := make([]string, 0, len(list.Items))
	for _, n := range list.Items {
		names = append(names, n.Name)
	}
	return names
}

func TestSweepRuntimeEnvironments_PVCs(t *testing.T) {
	terminating := sweepPVC("ci", "terminating", expiredAt(), "7")
	terminating.DeletionTimestamp = &metav1.Time{Time: sweepNow}
	terminating.Finalizers = []string{"kubernetes.io/pvc-protection"}

	tests := []struct {
		name    string
		objects []runtime.Object
		want    []string
	}{
		{
			name:    "past its expiry but within grace is kept",
			objects: []runtime.Object{sweepPVC("ci", "grace", sweepNow.Add(-time.Minute), "7")},
			want:    []string{"ci/grace"},
		},
		{
			name: "without an expiry stamp is kept",
			objects: []runtime.Object{func() *api.PersistentVolumeClaim {
				p := sweepPVC("ci", "unstamped", expiredAt(), "7")
				delete(p.Labels, common.RuntimeEnvironmentExpiresAtLabel)
				return p
			}()},
			want: []string{"ci/unstamped"},
		},
		{
			name: "expired but mounted by a pending pod is kept",
			objects: []runtime.Object{
				sweepPVC("ci", "scheduling", expiredAt(), "7"),
				sweepPod("ci", "resume", "scheduling", api.PodPending),
			},
			want: []string{"ci/scheduling"},
		},
		{
			name: "expired and referenced only by finished pods is released",
			objects: []runtime.Object{
				sweepPVC("ci", "done", expiredAt(), "7"),
				sweepPod("ci", "succeeded", "done", api.PodSucceeded),
				sweepPod("ci", "failed", "done", api.PodFailed),
			},
			want: []string{},
		},
		{
			name:    "another runner's environment is never touched",
			objects: []runtime.Object{sweepPVC("ci", "theirs", expiredAt(), "8")},
			want:    []string{"ci/theirs"},
		},
		{
			name:    "PVC already being deleted is left alone",
			objects: []runtime.Object{terminating},
			want:    []string{"ci/terminating"},
		},
		{
			name:    "PVC outside the configured namespace is not listed",
			objects: []runtime.Object{sweepPVC("other", "expired", expiredAt(), "7")},
			want:    []string{"other/expired"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset(tt.objects...)

			err := sweepRuntimeEnvironments(t.Context(), client, sweepRunnerConfig(&common.KubernetesConfig{Namespace: "ci"}),
				sweepNow, logrus.New())
			require.NoError(t, err)

			assert.ElementsMatch(t, tt.want, remainingPVCs(t, client))
		})
	}
}

// The sweep honors the expiry stamped when each PVC was created, not the
// sweeping manager's current TTL: a PVC stamped under a long TTL survives a
// manager with a short one, and one stamped under a short TTL is swept by a
// manager with a long one.
func TestSweepRuntimeEnvironments_HonorsStampedExpiryOverCurrentTTL(t *testing.T) {
	longTTL := 30 * 24 * time.Hour
	shortTTL := time.Hour
	createdAt := sweepNow.Add(-2 * shortTTL)

	stampedLong := sweepPVC("ci", "stamped-long", createdAt.Add(longTTL), "7")
	stampedLong.CreationTimestamp = metav1.NewTime(createdAt)
	stampedShort := sweepPVC("ci", "stamped-short", createdAt.Add(shortTTL), "7")
	stampedShort.CreationTimestamp = metav1.NewTime(createdAt)

	for _, ttl := range []time.Duration{shortTTL, longTTL} {
		t.Run(ttl.String(), func(t *testing.T) {
			client := fake.NewClientset(stampedLong.DeepCopy(), stampedShort.DeepCopy())
			config := sweepRunnerConfig(&common.KubernetesConfig{Namespace: "ci"})
			config.RuntimeEnvironmentTTL = &ttl

			require.NoError(t, sweepRuntimeEnvironments(t.Context(), client, config, sweepNow, logrus.New()))

			assert.ElementsMatch(t, []string{"ci/stamped-long"}, remainingPVCs(t, client))
		})
	}
}

func TestSweepRuntimeEnvironments_NamespacePerJob(t *testing.T) {
	terminating := sweepNamespace("ci-job-term", "7")
	terminating.DeletionTimestamp = &metav1.Time{Time: sweepNow}
	terminating.Finalizers = []string{"kubernetes"}
	releasing := sweepPVC("ci-job-6", "releasing", expiredAt(), "7")
	releasing.DeletionTimestamp = &metav1.Time{Time: sweepNow}
	releasing.Finalizers = []string{"kubernetes.io/pvc-protection"}
	objects := []runtime.Object{
		// Expired environment: PVC and then its namespace go.
		sweepNamespace("ci-job-1", "7"),
		sweepPVC("ci-job-1", "expired", expiredAt(), "7"),
		// Live environment: namespace stays because it holds a PVC.
		sweepNamespace("ci-job-2", "7"),
		sweepPVC("ci-job-2", "fresh", sweepNow.Add(time.Hour), "7"),
		// Expired but a job is running on it.
		sweepNamespace("ci-job-3", "7"),
		sweepPVC("ci-job-3", "busy", expiredAt(), "7"),
		sweepPod("ci-job-3", "job", "busy", api.PodRunning),
		// Namespace still being set up: not labelled yet, so never deleted.
		sweepNamespace("ci-job-4", ""),
		// Another runner's namespace.
		sweepNamespace("ci-job-5", "8"),
		sweepPVC("ci-job-5", "theirs", expiredAt(), "8"),
		// Already terminating: not deleted again.
		terminating,
		// PVC released but still terminating: the namespace goes anyway.
		sweepNamespace("ci-job-6", "7"),
		releasing,
	}
	client := fake.NewClientset(objects...)

	err := sweepRuntimeEnvironments(t.Context(), client,
		sweepRunnerConfig(&common.KubernetesConfig{NamespacePerJob: true}), sweepNow, logrus.New())
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"ci-job-2/fresh", "ci-job-3/busy", "ci-job-5/theirs", "ci-job-6/releasing"},
		remainingPVCs(t, client))
	assert.ElementsMatch(t, []string{"ci-job-2", "ci-job-3", "ci-job-4", "ci-job-5", "ci-job-term"},
		remainingNamespaces(t, client))
}

func TestSweepRuntimeEnvironments_Errors(t *testing.T) {
	tests := []struct {
		name      string
		verb      string
		resource  string
		wantCalls int
	}{
		{name: "list PVCs forbidden", verb: "list", resource: "persistentvolumeclaims", wantCalls: 1},
		{name: "list pods fails once per namespace", verb: "list", resource: "pods", wantCalls: 1},
		{name: "delete PVC fails, the next PVC is still tried", verb: "delete", resource: "persistentvolumeclaims", wantCalls: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset(
				sweepPVC("ci", "expired-a", expiredAt(), "7"),
				sweepPVC("ci", "expired-b", expiredAt(), "7"),
			)
			calls := 0
			client.PrependReactor(tt.verb, tt.resource, func(k8stesting.Action) (bool, runtime.Object, error) {
				calls++
				return true, nil, errors.New("boom")
			})

			err := sweepRuntimeEnvironments(t.Context(), client,
				sweepRunnerConfig(&common.KubernetesConfig{Namespace: "ci"}), sweepNow, logrus.New())
			assert.ErrorContains(t, err, "boom")
			// Any of these failures leaves both PVCs in place.
			assert.Len(t, remainingPVCs(t, client), 2)
			assert.Equal(t, tt.wantCalls, calls)
		})
	}
}

func TestSweepRuntimeEnvironments_DeleteNamespaceFails(t *testing.T) {
	client := fake.NewClientset(
		sweepNamespace("ci-job-1", "7"),
		sweepPVC("ci-job-1", "expired", expiredAt(), "7"),
	)
	client.PrependReactor("delete", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("boom")
	})

	err := sweepRuntimeEnvironments(t.Context(), client,
		sweepRunnerConfig(&common.KubernetesConfig{NamespacePerJob: true}), sweepNow, logrus.New())
	assert.ErrorContains(t, err, "boom")
	assert.Empty(t, remainingPVCs(t, client), "the PVC is still released")
}

func TestReleaseRuntimeEnvironment(t *testing.T) {
	cm := func(name, pvc, runnerID string) *api.ConfigMap {
		cm := &api.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ci", Labels: map[string]string{}}}
		if pvc != "" {
			cm.Labels[runtimeEnvironmentPVCLabel] = pvc
		}
		if runnerID != "" {
			cm.Labels[common.RuntimeEnvironmentRunnerIDLabel] = runnerID
		}
		return cm
	}
	allCMs := []string{"overlay-env", "overlay-env-other-runner", "overlay-other", "unrelated"}

	tests := []struct {
		name     string
		fields   map[string][]string
		failCMs  bool
		wantErr  string
		wantPVCs []string
		wantCMs  []string
	}{
		{
			name:     "deletes the PVC and its ConfigMaps",
			fields:   map[string][]string{envKeyNamespace: {"ci"}, envKeyPVC: {"env"}},
			wantPVCs: []string{},
			wantCMs:  []string{"overlay-env-other-runner", "overlay-other", "unrelated"},
		},
		{
			name:     "already gone counts as released",
			fields:   map[string][]string{envKeyNamespace: {"ci"}, envKeyPVC: {"missing"}},
			wantPVCs: []string{"ci/env"},
			wantCMs:  allCMs,
		},
		{
			name:     "ConfigMap error keeps the PVC so the next tick retries",
			fields:   map[string][]string{envKeyNamespace: {"ci"}, envKeyPVC: {"env"}},
			failCMs:  true,
			wantErr:  "forbidden",
			wantPVCs: []string{"ci/env"},
			wantCMs:  allCMs,
		},
		{
			name:     "missing namespace field",
			fields:   map[string][]string{envKeyPVC: {"env"}},
			wantErr:  "namespace",
			wantPVCs: []string{"ci/env"},
			wantCMs:  allCMs,
		},
		{
			name:     "missing pvc field",
			fields:   map[string][]string{envKeyNamespace: {"ci"}},
			wantErr:  "pvc",
			wantPVCs: []string{"ci/env"},
			wantCMs:  allCMs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset(sweepPVC("ci", "env", expiredAt(), "7"),
				cm("overlay-env", "env", "7"), cm("overlay-env-other-runner", "env", "8"),
				cm("overlay-other", "other", "7"), cm("unrelated", "", ""))
			if tt.failCMs {
				client.PrependReactor("list", "configmaps", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("forbidden")
				})
			}

			err := releaseRuntimeEnvironment(t.Context(), client, runtimeEnvironmentSelector(7), tt.fields)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.ElementsMatch(t, tt.wantPVCs, remainingPVCs(t, client))

			obj, err := client.Tracker().List(api.SchemeGroupVersion.WithResource("configmaps"),
				api.SchemeGroupVersion.WithKind("ConfigMap"), "ci")
			require.NoError(t, err)
			cms := []string{}
			for _, c := range obj.(*api.ConfigMapList).Items {
				cms = append(cms, c.Name)
			}
			assert.ElementsMatch(t, tt.wantCMs, cms)
		})
	}
}

func TestExecutorProvider_SweepRuntimeEnvironments(t *testing.T) {
	t.Run("no kubernetes section is a no-op", func(t *testing.T) {
		p := executorProvider{}
		assert.NoError(t, p.SweepRuntimeEnvironments(t.Context(), &common.RunnerConfig{}, sweepNow))
	})

	t.Run("client config error is returned", func(t *testing.T) {
		p := executorProvider{
			getKubeConfig: func(*common.KubernetesConfig, *overwrites) (*restclient.Config, error) {
				return nil, errors.New("no config")
			},
		}
		err := p.SweepRuntimeEnvironments(t.Context(), sweepRunnerConfig(&common.KubernetesConfig{}), sweepNow)
		assert.ErrorContains(t, err, "no config")
	})

	t.Run("sweeps the default namespace with the configured client", func(t *testing.T) {
		client := fake.NewClientset(sweepPVC(DefaultResourceIdentifier, "expired", expiredAt(), "7"))
		p := executorProvider{
			getKubeConfig: func(*common.KubernetesConfig, *overwrites) (*restclient.Config, error) {
				return &restclient.Config{}, nil
			},
			newKubeClient: func(*restclient.Config) (kubernetes.Interface, error) { return client, nil },
		}

		err := p.SweepRuntimeEnvironments(t.Context(), sweepRunnerConfig(&common.KubernetesConfig{}), sweepNow)
		require.NoError(t, err)
		assert.Empty(t, remainingPVCs(t, client))
	})
}

// A pod-list error in one namespace must not stop the sweep releasing PVCs in
// the others. The fake tracker sorts by namespace, so ns-alpha comes first.
func TestSweepRuntimeEnvironments_PodListErrorContinuesOtherNamespaces(t *testing.T) {
	client := fake.NewClientset(
		sweepPVC("ns-alpha", "expired", expiredAt(), "7"),
		sweepPVC("ns-beta", "expired", expiredAt(), "7"),
	)
	// fail the pod list only for ns-alpha
	client.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetNamespace() == "ns-alpha" {
			return true, nil, errors.New("boom")
		}
		return false, nil, nil
	})

	err := sweepRuntimeEnvironments(t.Context(), client,
		sweepRunnerConfig(&common.KubernetesConfig{NamespacePerJob: true}),
		sweepNow, logrus.New())
	assert.ErrorContains(t, err, `namespace "ns-alpha"`)

	remaining := remainingPVCs(t, client)
	assert.Contains(t, remaining, "ns-alpha/expired",
		"ns-alpha PVC is kept when its pod list failed")
	assert.NotContains(t, remaining, "ns-beta/expired",
		"ns-beta PVC is released despite the ns-alpha failure")
}
