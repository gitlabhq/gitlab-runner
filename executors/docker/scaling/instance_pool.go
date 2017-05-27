package scaling

import (
	"sync"
	"container/heap"
)

type instancePool struct {
	lock sync.Mutex
	instances instancePriorityQueue
	deleteInstances []*instanceDetails
}

func (i *instancePool) Put(details *instanceDetails) {
	i.lock.Lock()
	defer i.lock.Unlock()

	heap.Push(&i.instances, details)
}

func (i *instancePool) Get() *instanceDetails {
	i.lock.Lock()
	defer i.lock.Unlock()

	if i.instances.Len() == 0 {
		return nil
	}

	return heap.Pop(&i.instances).(*instanceDetails)
}

func (i *instancePool) Delete() {

}
