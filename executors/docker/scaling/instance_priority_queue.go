package scaling

type instancePriorityQueue []*instanceDetails

func (pq instancePriorityQueue) Len() int {
	return len(pq)
}

func (pq instancePriorityQueue) Less(i, j int) bool {
	return pq[i].lastUsed.Sub(pq[j].lastUsed) < 0
}

func (pq instancePriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *instancePriorityQueue) Push(x interface{}) {
	item := x.(*instanceDetails)
	*pq = append(*pq, item)
}

func (pq *instancePriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}
