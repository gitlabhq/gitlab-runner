package watchers

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/pull"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// emitErrorTimeout is the time we wait for a consumer of the error channel to receive a message. We must not block the
// informer, and if there is nobody listening for an error, there is no need to block on that anyway. We still give
// consumers some time, it was observed that _sometimes_ it takes a bit for the channel receiver to actually be able to
// receive (looking at you, windows).
const emitErrorTimeout = time.Millisecond

//go:generate mockery --name=logger --inpackage
type logger interface {
	Debugln(args ...any)
}

type PodWatcher struct {
	factory *selfManagedInformerFactory
	logger  logger

	podName atomic.Value

	// Don't send on this channel in a blocking manner, so that we don't block the informer. You can use the emitError()
	// method.
	errors chan error
}

// NewPodWatcher creates a pod watcher based on the kubeclient, namespace, and labels, and with a maximum duration for
// allowed for the initial cache sync.
// Internally, it creates a informer factory which can manage itself, so that it can be used and shut down properly.
func NewPodWatcher(ctx context.Context, logger logger, kubeClient kubernetes.Interface, namespace string, labels map[string]string, maxSyncDuration time.Duration) *PodWatcher {
	return &PodWatcher{
		factory: newScopedInformerFactory(ctx, kubeClient, namespace, labels, maxSyncDuration),
		logger:  logger,
		errors:  make(chan error),
	}
}

// Start starts the watcher, by creating an informer via the informer factory, starting that, waiting for events to
// come in, and forwarding (terminal) pod errors to the subscriber.
func (p *PodWatcher) Start() error {
	gvr := v1.SchemeGroupVersion.WithResource("pods")

	// kubeAPI: pods, list, watch
	informer, err := p.factory.ForResource(gvr)
	if err != nil {
		return fmt.Errorf("creating informer for pods: %w", err)
	}

	_, err = informer.Informer().AddEventHandler(p.resourceHandler())
	if err != nil {
		return fmt.Errorf("registering event handler: %w", err)
	}

	// kubeAPI: ignore
	p.factory.Start()

	// kubeAPI: ignore
	for informer, isSynced := range p.factory.WaitForCacheSync() {
		if isSynced {
			continue
		}
		err = errors.Join(err, fmt.Errorf("not synced: %s", informer))
	}

	return err
}

// UpdatePodName sets the pod name we are interested in
func (p *PodWatcher) UpdatePodName(podName string) {
	p.podName.Store(podName)
}

// Stop shuts down the pod watcher by shutting down its dependants: the informer factory and thus the
// informers created based on it.
func (p *PodWatcher) Stop() {
	// kubeAPI: ignore
	p.factory.Shutdown()
}

// Errors reports observed errors on the pod in question. This method MUST only ever called by one consumer at a time.
func (p *PodWatcher) Errors() <-chan error {
	return p.errors
}

func (p *PodWatcher) onPodChange(pod *v1.Pod) {
	podErr := checkTerminalPodErrors(pod)
	if podErr == nil {
		return
	}

	p.emitError(podErr)
}

// emitError sends out an error in a non-blocking way, so that the informer is not blocked.
func (p *PodWatcher) emitError(err error) {
	select {
	case p.errors <- err:
		// nothing to do, we've sent out the pod error
	case <-time.After(emitErrorTimeout):
		p.logger.Debugln(fmt.Sprintf("pod error not consumed in time (%s): %s", emitErrorTimeout, err))
	}
}

func (p *PodWatcher) resourceHandler() cache.ResourceEventHandler {
	return cache.FilteringResourceEventHandler{
		FilterFunc: func(obj any) bool {
			// We need to filter on the pod name; when the executor retries on pull issues, it starts the machinery from
			// fresh. While this is happening, the old pod might still be terminating. We don't care about these old pods
			// anymore in this context, and thus don't want to receive updates thereof.
			pod := asPod(obj)
			if pod == nil {
				p.logger.Debugln("update for unsupported object observed", obj)
				return false
			}
			return pod.GetName() == p.currentPodName()
		},
		Handler: cache.ResourceEventHandlerFuncs{
			// In FilterFunc we already checked, that the obj is indeed a non-nil pod, thus we don't have to check in the
			// handlers anymore and only have to do the type assertion.
			AddFunc: func(obj any) {
				p.onPodChange(asPod(obj))
			},
			UpdateFunc: func(_, newObj any) {
				p.onPodChange(asPod(newObj))
			},
			DeleteFunc: func(obj any) {
				pod := asPod(obj)
				p.emitError(fmt.Errorf("pod %q is deleted", pod.GetNamespace()+"/"+pod.GetName()))
			},
		},
	}
}

func (p *PodWatcher) currentPodName() string {
	if podName, ok := p.podName.Load().(string); ok {
		return podName
	}
	return ""
}

// asPod is a convenience helper to type-assert an untyped object to a pod.
func asPod(obj any) *v1.Pod {
	pod, _ := obj.(*v1.Pod)
	return pod
}

// checkTerminalPodErrors checks a pod for errors that are terminal, ie. the system can't recover from.
func checkTerminalPodErrors(pod *v1.Pod) error {
	fullPodName := fmt.Sprintf("%s/%s", pod.GetNamespace(), pod.GetName())

	dt := getPodCondition(pod, v1.DisruptionTarget)
	if dt != nil && dt.Status == v1.ConditionTrue {
		return fmt.Errorf("pod %q is disrupted: reason %q, message %q", fullPodName, dt.Reason, dt.Message)
	}

	if pod.DeletionTimestamp != nil {
		return fmt.Errorf("pod %q is being deleted", fullPodName)
	}

	// collect all containers' statuses, except those for ephemeral containers
	allContainerStatuses := slices.Concat(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses)
	if err := CheckTerminalContainerErrors(allContainerStatuses); err != nil {
		return err
	}

	return nil
}

// CheckTerminalContainerErrors checks individual container statuses for errors we can't recover from.
func CheckTerminalContainerErrors(containerStatuses []v1.ContainerStatus) error {
	for _, containerStatus := range containerStatuses {
		if containerStatus.Ready {
			continue
		}

		waiting := containerStatus.State.Waiting
		if waiting == nil {
			continue
		}

		switch waiting.Reason {
		case "InvalidImageName":
			return &common.BuildError{Inner: fmt.Errorf("image pull failed: %s", waiting.Message)}
		case "ErrImagePull", "ImagePullBackOff":
			msg := fmt.Sprintf("image pull failed: %s", waiting.Message)
			imagePullErr := &pull.ImagePullError{Message: msg, Container: containerStatus.Name, Image: containerStatus.Image}
			return &common.BuildError{Inner: imagePullErr, FailureReason: common.ImagePullFailure}
		}
	}

	return nil
}

func getPodCondition(pod *v1.Pod, condition v1.PodConditionType) *v1.PodCondition {
	conditions := pod.Status.Conditions
	for i := range conditions {
		if conditions[i].Type == condition {
			return &conditions[i]
		}
	}
	return nil
}
