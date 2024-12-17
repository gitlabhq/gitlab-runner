package watchers

import (
	"context"
	reflect "reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// defaultFactoryResync is set to 0, because we don't need resync right now.
// Note: setting the resync period to 0 for the factory has the side effect that individual informers can't set a resync
// period either for themselves.
// Note: resync is not relist; resync is replaying objects in the cache, essentially generating synthetic update events
// for them.
const defaultFactoryResync = time.Duration(0)

// selfManagedInformerFactory is an informer factory which manages it's own context and therefore lifetime. Thus it can
// shut down and manage itself properly.
//
// It has the same interface as an informers.SharedInformerFactory, except that methods which would take a context use the
// context held by the selfManagedInformerFactory, ie. the methods Start(), WaitForCacheSync() & Stop() are different.
// We do this, so that even though the factory hangs off of a parent context, we still can control it's lifetime
// independently of the parent context, and without needing to keep track of the context elsewhere, but still shut down
// correctly when the parent context gets canceled.
//
// If we'd ever wanted to reuse / share this informer factory with other components than the pod watcher, we can pull it
// out. We might want to think about making it a regular informers.SharedInformerFactory then, and handle its context
// from the outside, to have better control of this now shared factory and its lifetime.
type selfManagedInformerFactory struct {
	informers.SharedInformerFactory

	ctx    context.Context
	cancel context.CancelFunc
}

// newScopedInformerFactory creates an informer factory scoped to a specific namespace and to specific labels.
func newScopedInformerFactory(ctx context.Context, kubeClient kubernetes.Interface, namespaceScope string, labelScope map[string]string) *selfManagedInformerFactory {
	ctx, cancel := context.WithCancel(ctx)

	f := &selfManagedInformerFactory{
		ctx:    ctx,
		cancel: cancel,
		SharedInformerFactory: informers.NewSharedInformerFactoryWithOptions(
			kubeClient,
			defaultFactoryResync,
			informers.WithNamespace(namespaceScope),
			informers.WithTweakListOptions(func(lo *metav1.ListOptions) {
				lo.LabelSelector = labels.SelectorFromSet(labelScope).String()
			}),
		),
	}

	return f
}

func (f *selfManagedInformerFactory) Start() {
	f.SharedInformerFactory.Start(f.ctx.Done())
}

func (f *selfManagedInformerFactory) WaitForCacheSync() map[reflect.Type]bool {
	return f.SharedInformerFactory.WaitForCacheSync(f.ctx.Done())
}

func (f *selfManagedInformerFactory) Shutdown() {
	f.cancel()
	f.SharedInformerFactory.Shutdown()
}
