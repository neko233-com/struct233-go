package concurrent

import (
	"container/heap"
	"sync"
	"time"
)

// HeapBlockingPriorityQueue is a blocking priority queue whose head is always the smallest
// element per the comparator, the analog of
// java.util.concurrent.PriorityBlockingQueue.
//
// Elements that compare equal leave the queue in insertion order (FIFO
// tie-break), so the ordering is total and deterministic. Put/Offer block
// while a bounded queue is full, Take/Poll block while the queue is empty, and
// Close unblocks every waiter: values already queued stay drainable while
// further Put/Offer calls fail.
//
//	pq := concurrent.NewHeapBlockingPriorityQueue[int](func(a, b int) int { return a - b })
//	pq.Put(3)
//	if v, ok := pq.Take(); ok { // blocks until work is queued
//		println(v)
//	}
type HeapBlockingPriorityQueue[T any] struct {
	mu       sync.Mutex
	notEmpty *sync.Cond
	notFull  *sync.Cond
	h        priorityHeap[T]
	capacity int // 0 means unbounded
	closed   bool
	seq      uint64
}

// priorityQueueEntry is a queued value together with its insertion order.
type priorityQueueEntry[T any] struct {
	v   T
	seq uint64
}

// priorityHeap is the binary heap behind HeapBlockingPriorityQueue, ordered by the
// comparator and then by insertion order. It is only touched while the queue
// mutex is held.
type priorityHeap[T any] struct {
	items []priorityQueueEntry[T]
	cmp   func(a, b T) int
}

// Len returns the number of heap entries.
func (h *priorityHeap[T]) Len() int { return len(h.items) }

// Less orders entries by comparator, breaking comparator ties by insertion
// order so equal elements pop FIFO.
func (h *priorityHeap[T]) Less(i, j int) bool {
	if c := h.cmp(h.items[i].v, h.items[j].v); c != 0 {
		return c < 0
	}
	return h.items[i].seq < h.items[j].seq
}

// Swap exchanges two heap entries.
func (h *priorityHeap[T]) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
}

// Push appends x to the heap; x must be a priorityQueueEntry[T].
func (h *priorityHeap[T]) Push(x any) {
	h.items = append(h.items, x.(priorityQueueEntry[T]))
}

// Pop removes and returns the trailing heap entry; container/heap moves the
// head there before calling Pop.
func (h *priorityHeap[T]) Pop() any {
	last := len(h.items) - 1
	it := h.items[last]
	clear(h.items[last : last+1])
	h.items = h.items[:last]
	return it
}

// NewHeapBlockingPriorityQueue creates an unbounded priority queue ordered by cmp, which
// must follow the usual comparator contract (negative when a < b, zero when
// equal, positive otherwise). It panics if cmp is nil.
func NewHeapBlockingPriorityQueue[T any](cmp func(a, b T) int) *HeapBlockingPriorityQueue[T] {
	return newPriorityQueue[T](cmp, 0)
}

// NewHeapBlockingPriorityQueueWithCapacity creates a priority queue holding at most capacity
// elements. Put blocks while the queue is full and Offer waits at most its
// timeout for space. It panics if cmp is nil or capacity <= 0.
func NewHeapBlockingPriorityQueueWithCapacity[T any](cmp func(a, b T) int, capacity int) *HeapBlockingPriorityQueue[T] {
	if capacity <= 0 {
		panic("concurrent: priority queue capacity must be positive")
	}
	return newPriorityQueue[T](cmp, capacity)
}

// newPriorityQueue builds a queue with the given capacity (0 means unbounded).
func newPriorityQueue[T any](cmp func(a, b T) int, capacity int) *HeapBlockingPriorityQueue[T] {
	if cmp == nil {
		panic("concurrent: nil comparator")
	}
	q := &HeapBlockingPriorityQueue[T]{capacity: capacity}
	q.h.cmp = cmp
	q.notEmpty = sync.NewCond(&q.mu)
	q.notFull = sync.NewCond(&q.mu)
	return q
}

// Len returns the number of queued elements.
func (q *HeapBlockingPriorityQueue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.h.items)
}

// Put enqueues v at its priority position. On an unbounded queue it never
// blocks; on a bounded queue it blocks while the queue is full. It reports
// false when the queue was closed (the value is dropped).
func (q *HeapBlockingPriorityQueue[T]) Put(v T) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.fullLocked() && !q.closed {
		q.notFull.Wait()
	}
	if q.closed {
		return false
	}
	q.pushLocked(v)
	q.notEmpty.Signal()
	return true
}

// TryPut enqueues v without blocking, reporting whether it fit. It fails on a
// full bounded queue or a closed queue.
func (q *HeapBlockingPriorityQueue[T]) TryPut(v T) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || q.fullLocked() {
		return false
	}
	q.pushLocked(v)
	q.notEmpty.Signal()
	return true
}

// Offer enqueues v, waiting at most timeout for space on a bounded queue. A
// timeout <= 0 only tries once. It reports whether the value was enqueued.
func (q *HeapBlockingPriorityQueue[T]) Offer(v T, timeout time.Duration) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return false
	}
	if timeout <= 0 {
		if q.fullLocked() {
			return false
		}
		q.pushLocked(v)
		q.notEmpty.Signal()
		return true
	}
	deadline := time.Now().Add(timeout)
	for q.fullLocked() {
		if q.closed || !time.Now().Before(deadline) {
			return false
		}
		q.waitLocked(q.notFull, deadline)
	}
	if q.closed {
		return false
	}
	q.pushLocked(v)
	q.notEmpty.Signal()
	return true
}

// Take dequeues the smallest element, blocking while the queue is empty. It
// reports false once the queue is closed and drained.
func (q *HeapBlockingPriorityQueue[T]) Take() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.h.items) == 0 && !q.closed {
		q.notEmpty.Wait()
	}
	if len(q.h.items) == 0 {
		var zero T
		return zero, false
	}
	return q.popLocked(), true
}

// TryTake dequeues the smallest element without blocking, reporting false on
// an empty queue.
func (q *HeapBlockingPriorityQueue[T]) TryTake() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.h.items) == 0 {
		var zero T
		return zero, false
	}
	return q.popLocked(), true
}

// Poll dequeues the smallest element, waiting at most timeout for one to
// appear. A timeout <= 0 only takes an element that is already queued. It
// reports false on timeout or once the queue is closed and drained.
func (q *HeapBlockingPriorityQueue[T]) Poll(timeout time.Duration) (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if timeout <= 0 {
		if len(q.h.items) == 0 {
			var zero T
			return zero, false
		}
		return q.popLocked(), true
	}
	deadline := time.Now().Add(timeout)
	for len(q.h.items) == 0 {
		if q.closed || !time.Now().Before(deadline) {
			var zero T
			return zero, false
		}
		q.waitLocked(q.notEmpty, deadline)
	}
	return q.popLocked(), true
}

// Peek returns the smallest element without removing it, or the zero value
// when the queue is empty.
func (q *HeapBlockingPriorityQueue[T]) Peek() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.h.items) == 0 {
		var zero T
		return zero, false
	}
	return q.h.items[0].v, true
}

// Close unblocks all waiters: Take and Poll report false once the elements
// already queued have been drained, and Put/Offer/TryPut report false
// afterwards.
func (q *HeapBlockingPriorityQueue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.notEmpty.Broadcast()
	q.notFull.Broadcast()
}

// fullLocked reports whether a bounded queue has no room left; the caller
// holds q.mu.
func (q *HeapBlockingPriorityQueue[T]) fullLocked() bool {
	return q.capacity > 0 && len(q.h.items) == q.capacity
}

// pushLocked inserts v at its priority position, breaking comparator ties by
// insertion order. The caller holds q.mu.
func (q *HeapBlockingPriorityQueue[T]) pushLocked(v T) {
	q.seq++
	heap.Push(&q.h, priorityQueueEntry[T]{v: v, seq: q.seq})
}

// popLocked removes and returns the smallest element. The caller holds q.mu
// and must have checked that the queue is not empty.
func (q *HeapBlockingPriorityQueue[T]) popLocked() T {
	entry := heap.Pop(&q.h).(priorityQueueEntry[T])
	q.notFull.Signal() // a producer may be waiting for space
	return entry.v
}

// waitLocked blocks on cond until deadline fires or the cond is signalled; the
// caller holds q.mu and must re-check the queue state afterwards.
func (q *HeapBlockingPriorityQueue[T]) waitLocked(cond *sync.Cond, deadline time.Time) {
	timer := time.AfterFunc(time.Until(deadline), func() {
		q.mu.Lock()
		cond.Broadcast()
		q.mu.Unlock()
	})
	cond.Wait()
	timer.Stop()
}
