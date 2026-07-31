//go:build !integration

package watchers

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

var (
	defaultName      = "some-pod"
	defaultNamespace = "some-namespace"
	defaultLabels    = map[string]string{"some": "label"}
	executorRetries  = 3
)

func TestPodWatcher(t *testing.T) {
	tests := map[string]struct {
		pod                   *v1.Pod
		shouldDelete          bool
		expectedErrMsg        string
		expectedFailureReason spec.JobFailureReason
	}{
		"deleted": {
			pod:            defaultPod(),
			shouldDelete:   true,
			expectedErrMsg: "is deleted",
		},
		"deletion timestamp": {
			pod:            withDeletionTimestamp(defaultPod()),
			expectedErrMsg: "is being deleted",
		},
		"disruption": {
			pod:            withDisruption(defaultPod(), "disruption-msg", "disruption-reason"),
			expectedErrMsg: `disrupted: reason "disruption-reason", message "disruption-msg"`,
		},
		"invalid image": {
			pod:                   withContainerWaiting(defaultPod(), "some-msg", "InvalidImageName"),
			expectedErrMsg:        "image pull failed: some-msg",
			expectedFailureReason: common.ConfigurationError,
		},
		"pull error": {
			pod:                   withContainerWaiting(defaultPod(), "some-msg", "ErrImagePull"),
			expectedErrMsg:        "image pull failed: some-msg",
			expectedFailureReason: common.ImagePullFailure,
		},
		"pull backoff": {
			pod:                   withContainerWaiting(defaultPod(), "some-msg", "ImagePullBackOff"),
			expectedErrMsg:        "image pull failed: some-msg",
			expectedFailureReason: common.ImagePullFailure,
		},
		"pull error - network failure": {
			pod: withContainerWaiting(defaultPod(),
				`failed to pull and unpack image: failed to do request: Head "https://registry.example.com": dial tcp: lookup registry.example.com: no such host`,
				"ErrImagePull"),
			expectedErrMsg:        "image pull failed",
			expectedFailureReason: common.RunnerExternalDependencyFailure,
		},
		"pull error - access denied": {
			pod: withContainerWaiting(defaultPod(),
				`rpc error: code = Unknown desc = failed to pull and unpack image "registry.example.com/private/image:latest": failed to resolve reference "registry.example.com/private/image:latest": pull access denied, repository does not exist or may require authorization: server message: insufficient_scope: authorization failed`,
				"ErrImagePull"),
			expectedErrMsg:        "image pull failed",
			expectedFailureReason: common.ConfigurationError,
		},
		"pull error - image not found": {
			pod: withContainerWaiting(defaultPod(),
				`failed to pull and unpack image: failed to resolve reference: unexpected status code 404 not found`,
				"ErrImagePull"),
			expectedErrMsg:        "image pull failed",
			expectedFailureReason: common.ConfigurationError,
		},
		"crash loop backoff": {
			pod:                   withContainerWaiting(defaultPod(), "back-off 10s", "CrashLoopBackOff"),
			expectedErrMsg:        `container "some-container" is in CrashLoopBackOff: back-off 10s`,
			expectedFailureReason: common.ConfigurationError,
		},
		"crash loop backoff with last termination": {
			pod:                   withContainerWaitingAndLastTermination(defaultPod(), "some-container", "back-off 10s", "CrashLoopBackOff", "exec format error"),
			expectedErrMsg:        `container "some-container" is in CrashLoopBackOff: back-off 10s: exec format error`,
			expectedFailureReason: common.ConfigurationError,
		},
		"healthy pod": {
			pod: defaultPod(),
		},
		"error, but different pod name": {
			pod: withName(withDeletionTimestamp(defaultPod()), "some-other-pod"),
		},
		"error, but different namespace": {
			pod: withNameSpace(withDeletionTimestamp(defaultPod()), "some-other-namespace"),
		},
		// Note: the fake client infrastructure does not handle labels correctly, thus this unit test would fail.
		// The real client, informer, ... however handle that correctly
		// "error, but different labels": {
		// 	pod: withLabels(withDeletionTimestamp(defaultPod()), map[string]string{"some": "other", "random": "labels"}),
		// },
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			fakeKubeClient, waitForWatchStart := newFakeKubeClientWithSyncedWatch("pods")
			fakeLogger := newMockLogger(t)

			podWatcher := NewPodWatcher(ctx, fakeLogger, fakeKubeClient, defaultNamespace, defaultLabels, 0)
			podErrors := podWatcher.Errors()

			err := podWatcher.Start()
			assert.NoError(t, err, "starting pod watcher")
			waitForWatchStart(t)

			factory := podWatcher.factory

			// This is to mimic the case when we get some error where the executor retries and podWatcher.UpdatePodName should
			// switch to the new pod name, and ignore any old one.
			// Currently that is only happening for pull issues.
			for try := range executorRetries {
				expectedPodNameForThisTry := fmt.Sprintf("%s-%d", defaultName, try)
				actualPodNameForThisTry := fmt.Sprintf("%s-%d", test.pod.GetName(), try)

				testPod := withName(test.pod, actualPodNameForThisTry)
				podWatcher.UpdatePodName(expectedPodNameForThisTry)

				_, err = fakeKubeClient.CoreV1().Pods(testPod.GetNamespace()).Create(ctx, testPod, metav1.CreateOptions{})
				assert.NoError(t, err, "(try %d) creating pod")

				if test.shouldDelete {
					err := fakeKubeClient.CoreV1().Pods(testPod.GetNamespace()).Delete(ctx, testPod.GetName(), metav1.DeleteOptions{})
					assert.NoError(t, err, "(try %d) deleting pod")
				}

				podErr := waitForError(podErrors)
				if test.expectedErrMsg == "" {
					assert.NoError(t, podErr, "(try %d) not to receive an error from the pod watcher", try)
				} else {
					assert.ErrorContains(t, podErr, expectedPodNameForThisTry, "(try %d) expected the error to be for pod %q", try, expectedPodNameForThisTry)
					assert.ErrorContains(t, podErr, test.expectedErrMsg, "(try %d) expected an error like %q from the pod watcher", try, test.expectedErrMsg)
					if test.expectedFailureReason != "" {
						var buildErr *common.BuildError
						require.ErrorAs(t, podErr, &buildErr)
						assert.Equal(t, test.expectedFailureReason, buildErr.FailureReason)
					}
				}
			}

			podWatcher.Stop()
			// We check the factory's context to see if it had actually been canceled and by that inferring that it has been shut
			// down.
			assert.Equal(t, context.Canceled, factory.ctx.Err(), "expected factory's context to be canceled")
		})
	}
}

func TestPodWatcherNoConsumer(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	podWithErr := withDeletionTimestamp(defaultPod())
	fakeKubeClient, waitForWatchStart := newFakeKubeClientWithSyncedWatch("pods")
	fakeLogger := newMockLogger(t)

	podWatcher := NewPodWatcher(ctx, fakeLogger, fakeKubeClient, defaultNamespace, defaultLabels, 0)

	err := podWatcher.Start()
	assert.NoError(t, err, "starting pod watcher")
	waitForWatchStart(t)

	podWatcher.UpdatePodName(podWithErr.GetName())

	expectedLog := fmt.Sprintf(`pod error not consumed in time (%s): pod "%s/%s" is deleted`, emitErrorTimeout, podWithErr.GetNamespace(), podWithErr.GetName())
	logObserved := make(chan struct{})
	fakeLogger.On("Debugln", expectedLog).Run(func(_ mock.Arguments) {
		close(logObserved)
	}).Once()

	_, err = fakeKubeClient.CoreV1().Pods(podWithErr.GetNamespace()).Create(ctx, podWithErr, metav1.CreateOptions{})
	assert.NoError(t, err, "creating pod")

	err = fakeKubeClient.CoreV1().Pods(podWithErr.GetNamespace()).Delete(ctx, podWithErr.GetName(), metav1.DeleteOptions{})
	assert.NoError(t, err, "deleting pod")

	maxWaitTime := time.Second
	select {
	case <-logObserved:
	case <-time.After(maxWaitTime):
		assert.Fail(t, "expected issue to be logged", "expected log line to appear within %s", maxWaitTime)
	}
}

func TestPodWatcherWrongObject(t *testing.T) {
	tests := map[string]struct {
		object               any
		expectUnsupportedLog bool
	}{
		"nil": {
			expectUnsupportedLog: true,
		},
		"pod": {
			object: defaultPod(),
		},
		"random object": {
			object:               map[string]any{"blupp": "blapp"},
			expectUnsupportedLog: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			fakeLogger := newMockLogger(t)
			podWatcher := &PodWatcher{
				logger: fakeLogger,
				errors: make(chan error, 10),
			}
			podWatcher.UpdatePodName(defaultName)

			if test.expectUnsupportedLog {
				fakeLogger.On(
					"Debugln", "update for unsupported object observed", test.object,
				).Once()
			}

			handler := podWatcher.resourceHandler()

			assert.NotPanics(t, func() {
				handler.OnAdd(test.object, false)
			})
		})
	}
}

func waitForError(ch <-chan error) error {
	to := time.After(emitErrorTimeout * 100)
	select {
	case <-to:
		return nil
	case err := <-ch:
		return err
	}
}

// newFakeKubeClientWithSyncedWatch returns a fake Kubernetes clientset for use with an informer, together with a
// function that blocks until the client's watch for the given resource has actually been established.
//
// This works around a race in client-go's fake clientset: cache.WaitForCacheSync only waits for an informer's
// initial List to be processed, not for its subsequent Watch call to be registered with the fake tracker. A test
// that creates/deletes objects immediately after WaitForCacheSync returns can race past that registration: the
// fake tracker has no watcher yet to notify, and does not replay missed events once one is registered, so the
// events are dropped permanently rather than merely delayed. That caused the flaky TestPodWatcher/deleted
// (https://gitlab.com/gitlab-org/gitlab-runner/-/issues/39640): no amount of widening waitForError's timeout
// could fix it, since the event genuinely never arrives.
func newFakeKubeClientWithSyncedWatch(resource string) (*fake.Clientset, func(t *testing.T)) {
	fakeKubeClient := fake.NewClientset()

	watchStarted := make(chan struct{})
	var once sync.Once
	fakeKubeClient.PrependWatchReactor(resource, func(action ktesting.Action) (bool, watch.Interface, error) {
		w, err := fakeKubeClient.Tracker().Watch(action.GetResource(), action.GetNamespace())
		once.Do(func() { close(watchStarted) })
		return true, w, err
	})

	waitForWatchStart := func(t *testing.T) {
		t.Helper()
		select {
		case <-watchStarted:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for the fake client's watch to start")
		}
	}

	return fakeKubeClient, waitForWatchStart
}

func defaultPod() *v1.Pod {
	pod := &v1.Pod{}
	pod.SetName(defaultName)
	pod.SetNamespace(defaultNamespace)
	pod.SetLabels(defaultLabels)
	return pod
}

func withName(pod *v1.Pod, name string) *v1.Pod {
	p := pod.DeepCopy()
	p.SetName(name)
	return p
}

func withNameSpace(pod *v1.Pod, namespace string) *v1.Pod {
	p := pod.DeepCopy()
	p.SetNamespace(namespace)
	return p
}

func withLabels(pod *v1.Pod, labels map[string]string) *v1.Pod {
	p := pod.DeepCopy()
	p.SetLabels(labels)
	return p
}

func withDeletionTimestamp(pod *v1.Pod) *v1.Pod {
	p := pod.DeepCopy()
	now := metav1.Now()
	p.DeletionTimestamp = &now
	return p
}

func withDisruption(pod *v1.Pod, msg, reason string) *v1.Pod {
	p := pod.DeepCopy()
	p.Status.Conditions = append(p.Status.Conditions, v1.PodCondition{
		Status:  v1.ConditionTrue,
		Message: msg,
		Reason:  reason,
		Type:    v1.DisruptionTarget,
	})
	return p
}

func withContainerWaiting(pod *v1.Pod, msg, reason string) *v1.Pod {
	p := pod.DeepCopy()
	p.Status.ContainerStatuses = append(p.Status.ContainerStatuses, v1.ContainerStatus{
		Name: "some-container",
		State: v1.ContainerState{
			Waiting: &v1.ContainerStateWaiting{
				Reason:  reason,
				Message: msg,
			},
		},
	})
	return p
}

func withContainerWaitingAndLastTermination(pod *v1.Pod, containerName, msg, reason, lastTermMsg string) *v1.Pod {
	p := pod.DeepCopy()
	p.Status.ContainerStatuses = append(p.Status.ContainerStatuses, v1.ContainerStatus{
		Name: containerName,
		State: v1.ContainerState{
			Waiting: &v1.ContainerStateWaiting{
				Reason:  reason,
				Message: msg,
			},
		},
		LastTerminationState: v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				ExitCode: 1,
				Reason:   "Error",
				Message:  lastTermMsg,
			},
		},
	})
	return p
}
