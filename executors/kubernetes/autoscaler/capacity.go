package autoscaler

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// capacityTracker counts this runner's pause pods bound to a node and the job requests made against
// them, from Acquire until the job pod has a node or the request ends without a job.
type capacityTracker struct {
	onChange func(scheduled, reserved int)

	mu        sync.Mutex
	scheduled map[string]struct{}
	reserved  int
}

func newCapacityTracker(onChange func(scheduled, reserved int)) *capacityTracker {
	if onChange == nil {
		onChange = func(int, int) {}
	}
	return &capacityTracker{
		onChange:  onChange,
		scheduled: make(map[string]struct{}),
	}
}

func (t *capacityTracker) start(ctx context.Context, client kubernetes.Interface, namespace string, sel labels.Selector, log logrus.FieldLogger) error {
	factory := informers.NewSharedInformerFactoryWithOptions(
		client,
		0,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(lo *metav1.ListOptions) {
			lo.LabelSelector = sel.String()
		}),
	)

	informer := factory.Core().V1().Pods().Informer()
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { t.observe(obj) },
		UpdateFunc: func(_, obj any) { t.observe(obj) },
		DeleteFunc: func(obj any) { t.forget(obj) },
	})
	if err != nil {
		return fmt.Errorf("registering pause pod event handler: %w", err)
	}

	// Otherwise list and watch failures only reach klog.
	err = informer.SetWatchErrorHandler(func(_ *cache.Reflector, err error) {
		log.WithError(err).Warn("Failed to watch pause pods, job requests are declined until the watch works")
	})
	if err != nil {
		return fmt.Errorf("registering pause pod watch error handler: %w", err)
	}

	factory.Start(ctx.Done())

	return nil
}

func (t *capacityTracker) observe(obj any) {
	pod, ok := obj.(*api.Pod)
	if !ok {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if isScheduled(pod) {
		t.scheduled[pod.Name] = struct{}{}
	} else {
		delete(t.scheduled, pod.Name)
	}
	t.onChange(len(t.scheduled), t.reserved)
}

// An evicted pod is Failed with its node name still set, and the ReplicaSet controller does not delete it.
func isScheduled(pod *api.Pod) bool {
	return pod.Spec.NodeName != "" &&
		pod.DeletionTimestamp == nil &&
		pod.Status.Phase != api.PodFailed &&
		pod.Status.Phase != api.PodSucceeded
}

func (t *capacityTracker) forget(obj any) {
	name := ""
	switch o := obj.(type) {
	case *api.Pod:
		name = o.Name
	case cache.DeletedFinalStateUnknown:
		if pod, ok := o.Obj.(*api.Pod); ok {
			name = pod.Name
		}
	}
	if name == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.scheduled, name)
	t.onChange(len(t.scheduled), t.reserved)
}

func (t *capacityTracker) reserve() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.reserved >= len(t.scheduled) {
		return false
	}
	t.reserved++
	t.onChange(len(t.scheduled), t.reserved)
	return true
}

func (t *capacityTracker) release() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.reserved == 0 {
		return
	}
	t.reserved--
	t.onChange(len(t.scheduled), t.reserved)
}

func (t *capacityTracker) counts() (scheduled, reserved int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.scheduled), t.reserved
}
