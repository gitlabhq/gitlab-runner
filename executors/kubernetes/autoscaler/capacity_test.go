//go:build !integration

package autoscaler

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func pausePod(name, node string) *api.Pod {
	return &api.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       api.PodSpec{NodeName: node},
	}
}

func TestCapacityTracker_ReserveFollowsScheduledPods(t *testing.T) {
	var scheduled, reserved int
	tracker := newCapacityTracker(func(s, r int) { scheduled, reserved = s, r })

	assert.False(t, tracker.reserve(), "nothing scheduled")

	tracker.observe(pausePod("pending", ""))
	assert.False(t, tracker.reserve(), "a pending pause pod is not a slot")

	tracker.observe(pausePod("a", "node-1"))
	tracker.observe(pausePod("b", "node-2"))
	tracker.observe(pausePod("b", "node-2"))
	assert.Equal(t, 2, scheduled, "repeated updates count a pod once")

	assert.True(t, tracker.reserve())
	assert.True(t, tracker.reserve())
	assert.Equal(t, 2, reserved)
	assert.False(t, tracker.reserve())

	tracker.release()
	assert.Equal(t, 1, reserved)
	assert.True(t, tracker.reserve())

	tracker.release()
	tracker.release()
	tracker.release()
	assert.Equal(t, 0, reserved, "release does not go negative")
}

func TestCapacityTracker_PreemptedPodReducesScheduled(t *testing.T) {
	tracker := newCapacityTracker(nil)
	tracker.observe(pausePod("a", "node-1"))
	tracker.observe(pausePod("b", "node-2"))
	require.True(t, tracker.reserve())

	terminating := pausePod("a", "node-1")
	now := metav1.Now()
	terminating.DeletionTimestamp = &now
	tracker.observe(terminating)
	s, r := tracker.counts()
	assert.Equal(t, 1, s)
	assert.Equal(t, 1, r, "the request is still counted until its pod has a node")
	assert.False(t, tracker.reserve())

	tracker.forget(terminating)
	tracker.release()
	assert.True(t, tracker.reserve())

	tracker.forget(cache.DeletedFinalStateUnknown{Obj: pausePod("b", "node-2")})
	s, _ = tracker.counts()
	assert.Equal(t, 0, s)
}

func TestCapacityTracker_StartDeliversExistingAndNewPods(t *testing.T) {
	labels := map[string]string{pausePodLabel: pausePodLabelValue, runnerIDLabel: "r", systemIDLabel: "s"}
	mine := pausePod("mine-scheduled", "node-1")
	mine.Labels = labels
	pending := pausePod("mine-pending", "")
	pending.Labels = labels
	other := pausePod("other", "node-1")
	other.Labels = map[string]string{"app": "x"}
	client := fake.NewClientset(mine, pending, other)

	m := &PausePodManager{config: PausePodManagerConfig{RunnerShortToken: "r", SystemID: "s"}}
	tracker := newCapacityTracker(nil)
	require.NoError(t, tracker.start(t.Context(), client, "default", m.podSelector(), logrus.NewEntry(logrus.New())))

	require.Eventually(t, func() bool { s, _ := tracker.counts(); return s == 1 }, 5*time.Second, 10*time.Millisecond)

	pending.Spec.NodeName = "node-2"
	_, err := client.CoreV1().Pods("default").Update(t.Context(), pending, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.Eventually(t, func() bool { s, _ := tracker.counts(); return s == 2 }, 5*time.Second, 10*time.Millisecond)
}

func TestPausePodManager_ReserveWithoutTracker(t *testing.T) {
	manager, err := NewPausePodManager(fake.NewClientset(), PausePodManagerConfig{
		Namespace: "default", RunnerShortToken: "r", SystemID: "s",
	}, logrus.NewEntry(logrus.New()), nil)
	require.NoError(t, err)

	assert.False(t, manager.Reserve())
	manager.Release()
}

func TestPausePodManager_Preemptive(t *testing.T) {
	policies := PolicyList{
		{Periods: []string{"* * * * *"}, IdleCount: 0},
		{Periods: []string{"* 9-17 * * mon-fri"}, Timezone: "UTC", IdleCount: 3, PreemptiveMode: true},
	}
	manager, err := NewPausePodManager(fake.NewClientset(), PausePodManagerConfig{
		Namespace: "default", RunnerShortToken: "r", SystemID: "s", Policies: policies,
	}, logrus.NewEntry(logrus.New()), nil)
	require.NoError(t, err)
	require.NotNil(t, manager.tracker, "one preemptive policy is enough to watch the pause pods")

	officeHours := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	night := time.Date(2026, 10, 21, 2, 0, 0, 0, time.UTC)
	assert.True(t, manager.preemptive(officeHours))
	assert.False(t, manager.preemptive(night), "the night policy has no preemptive mode")

	manager.config.Policies[1].PreemptiveMode = false
	assert.False(t, manager.preemptive(officeHours))
}

func signalProviderConfig(idleCount int) *common.RunnerConfig {
	preemptive := idleCount > 0
	return &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{Token: "test-token"},
		RunnerSettings: common.RunnerSettings{
			Kubernetes: &common.KubernetesConfig{
				Namespace: "default",
				Autoscaler: &common.KubernetesAutoscalerConfig{
					Policy: []common.AutoscalerPolicyConfig{{IdleCount: idleCount, Periods: []string{"* * * * *"}, PreemptiveMode: &preemptive}},
				},
			},
		},
		ConfigLoadedAt: time.Now(),
	}
}

func newSignalProvider(t *testing.T, base common.ExecutorProvider, client kubernetes.Interface) *Provider {
	t.Helper()
	provider := NewProvider(base)
	provider.newKubeClient = func(*restclient.Config) (kubernetes.Interface, error) { return client, nil }
	provider.getKubeConfig = func(*common.KubernetesConfig) (*restclient.Config, error) { return &restclient.Config{}, nil }
	return provider
}

func TestProvider_Acquire_DeclinesWithoutScheduledPausePod(t *testing.T) {
	base := common.NewMockExecutorProvider(t)
	provider := newSignalProvider(t, base, fake.NewClientset())
	config := signalProviderConfig(1)

	_, err := provider.Acquire(config)
	var noFree *common.NoFreeExecutorError
	require.ErrorAs(t, err, &noFree)

	manager := provider.GetManager(config)
	require.NotNil(t, manager)
	assert.Equal(t, 0, manager.getActiveJobs(), "a declined request is not an active job")
}

func TestProvider_Acquire_ManagerCannotStartIsAnError(t *testing.T) {
	base := common.NewMockExecutorProvider(t)
	provider := newSignalProvider(t, base, fake.NewClientset())
	provider.getKubeConfig = func(*common.KubernetesConfig) (*restclient.Config, error) { return nil, assert.AnError }

	_, err := provider.Acquire(signalProviderConfig(1))
	require.ErrorIs(t, err, assert.AnError)
	var noFree *common.NoFreeExecutorError
	assert.False(t, errors.As(err, &noFree), "a setup failure must not look like a full cluster")
	assert.Nil(t, provider.GetManager(signalProviderConfig(1)))
}

func TestProvider_Acquire_ReservesAndReleases(t *testing.T) {
	base := common.NewMockExecutorProvider(t)
	base.EXPECT().Acquire(mock.Anything).Return("inner", nil)
	base.EXPECT().Release(mock.Anything, "inner").Return()

	provider := newSignalProvider(t, base, fake.NewClientset())
	config := signalProviderConfig(1)

	manager, err := provider.ensureManager(config)
	require.NoError(t, err)
	require.NotNil(t, manager)
	manager.tracker.observe(pausePod("pause-a", "node-1"))

	data, err := provider.Acquire(config)
	require.NoError(t, err)
	acq, ok := data.(*acquisition)
	require.True(t, ok)
	assert.Equal(t, "inner", acq.Unwrap())
	_, reserved := manager.tracker.counts()
	assert.Equal(t, 1, reserved)

	_, err = provider.Acquire(config)
	var noFree *common.NoFreeExecutorError
	require.ErrorAs(t, err, &noFree)

	provider.Release(config, data)
	_, reserved = manager.tracker.counts()
	assert.Equal(t, 0, reserved)
	assert.Equal(t, 0, manager.getActiveJobs())
}

func TestProvider_Acquire_PodScheduledReleasesBeforeRelease(t *testing.T) {
	base := common.NewMockExecutorProvider(t)
	base.EXPECT().Acquire(mock.Anything).Return(nil, nil).Twice()
	base.EXPECT().Release(mock.Anything, mock.Anything).Return().Twice()

	provider := newSignalProvider(t, base, fake.NewClientset())
	config := signalProviderConfig(1)

	_, err := provider.ensureManager(config)
	require.NoError(t, err)
	manager := provider.GetManager(config)
	manager.tracker.observe(pausePod("pause-a", "node-1"))

	data, err := provider.Acquire(config)
	require.NoError(t, err)

	data.(interface{ PodScheduled() }).PodScheduled()
	_, reserved := manager.tracker.counts()
	assert.Equal(t, 0, reserved)
	assert.Equal(t, 1, manager.getActiveJobs())

	second, err := provider.Acquire(config)
	require.NoError(t, err)

	provider.Release(config, data)
	_, reserved = manager.tracker.counts()
	assert.Equal(t, 1, reserved, "the first request was already released once")
	provider.Release(config, second)
	assert.Equal(t, 0, manager.getActiveJobs())
}

func TestProvider_Acquire_IdleCountZeroPassesThrough(t *testing.T) {
	base := common.NewMockExecutorProvider(t)
	base.EXPECT().Acquire(mock.Anything).Return(nil, nil)

	provider := newSignalProvider(t, base, fake.NewClientset())

	data, err := provider.Acquire(signalProviderConfig(0))
	require.NoError(t, err)
	acq, ok := data.(*acquisition)
	require.True(t, ok)
	assert.Nil(t, acq.release)
}

func TestProvider_Acquire_BaseErrorReleases(t *testing.T) {
	base := common.NewMockExecutorProvider(t)
	base.EXPECT().Acquire(mock.Anything).Return(nil, assert.AnError)

	provider := newSignalProvider(t, base, fake.NewClientset())
	config := signalProviderConfig(1)

	manager, err := provider.ensureManager(config)
	require.NoError(t, err)
	manager.tracker.observe(pausePod("pause-a", "node-1"))

	_, err = provider.Acquire(config)
	require.ErrorIs(t, err, assert.AnError)
	_, reserved := manager.tracker.counts()
	assert.Equal(t, 0, reserved)
}

func TestCapacityTracker_TerminalPodsAreNotSlots(t *testing.T) {
	tracker := newCapacityTracker(nil)

	evicted := pausePod("evicted", "node-1")
	evicted.Status.Phase = api.PodFailed
	tracker.observe(evicted)
	assert.False(t, tracker.reserve())

	done := pausePod("done", "node-1")
	done.Status.Phase = api.PodSucceeded
	tracker.observe(done)
	assert.False(t, tracker.reserve())

	tracker.observe(pausePod("running", "node-1"))
	assert.True(t, tracker.reserve())
}

func TestProvider_Acquire_ManagerFailureOutsideThePreemptivePolicyPassesThrough(t *testing.T) {
	base := common.NewMockExecutorProvider(t)
	base.EXPECT().Acquire(mock.Anything).Return(nil, nil)

	provider := newSignalProvider(t, base, fake.NewClientset())
	provider.getKubeConfig = func(*common.KubernetesConfig) (*restclient.Config, error) { return nil, assert.AnError }

	_, err := provider.Acquire(signalProviderConfig(0))
	require.NoError(t, err, "no preemptive policy")

	preemptive := true
	config := signalProviderConfig(0)
	config.Kubernetes.Autoscaler.Policy = append(config.Kubernetes.Autoscaler.Policy, common.AutoscalerPolicyConfig{
		IdleCount:      1,
		Periods:        []string{"* * * * " + neverThisWeekday(time.Now())},
		PreemptiveMode: &preemptive,
	})
	_, err = provider.Acquire(config)
	require.NoError(t, err, "a preemptive policy that is not active now")
}

func neverThisWeekday(now time.Time) string {
	return strconv.Itoa((int(now.Weekday()) + 3) % 7)
}

func TestProvider_AcquireCanary_SkipsTheCapacityCheck(t *testing.T) {
	base := common.NewMockExecutorProvider(t)
	base.EXPECT().Acquire(mock.Anything).Return("inner", nil)
	base.EXPECT().Release(mock.Anything, "inner").Return()

	provider := newSignalProvider(t, base, fake.NewClientset())
	config := signalProviderConfig(1)

	_, err := provider.Acquire(config)
	var noFree *common.NoFreeExecutorError
	require.ErrorAs(t, err, &noFree, "no scheduled pause pod")

	data, err := provider.AcquireCanary(config)
	require.NoError(t, err)
	manager := provider.GetManager(config)
	require.NotNil(t, manager)
	assert.Equal(t, 1, manager.getActiveJobs())
	_, reserved := manager.tracker.counts()
	assert.Equal(t, 0, reserved, "the canary holds no reservation")

	provider.Release(config, data)
	assert.Equal(t, 0, manager.getActiveJobs())
}

func TestProvider_Acquire_IdleCountZeroWithPreemptiveModePassesThrough(t *testing.T) {
	base := common.NewMockExecutorProvider(t)
	base.EXPECT().Acquire(mock.Anything).Return(nil, nil).Times(3)
	provider := newSignalProvider(t, base, fake.NewClientset())
	t.Cleanup(func() { provider.Shutdown(t.Context(), nil) })

	on := true
	config := signalProviderConfig(0)
	config.Kubernetes.Autoscaler.Policy[0].PreemptiveMode = &on

	for range 3 {
		_, err := provider.Acquire(config)
		require.NoError(t, err, "no pause pods to check against")
	}
}

func TestCapacityTracker_ForbiddenPodWatchIsLogged(t *testing.T) {
	client := fake.NewClientset()
	client.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", nil)
	})
	logger, hook := logtest.NewNullLogger()

	tracker := newCapacityTracker(nil)
	require.NoError(t, tracker.start(t.Context(), client, "default", labels.Everything(), logrus.NewEntry(logger)))

	assert.Eventually(t, func() bool {
		for _, e := range hook.AllEntries() {
			if e.Level <= logrus.WarnLevel {
				return true
			}
		}
		return false
	}, 5*time.Second, 50*time.Millisecond, "a forbidden pod list is logged at warn")
}
