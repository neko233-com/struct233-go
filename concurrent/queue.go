package concurrent

import (
	"sync"
	"time"
)

// Queue is a bounded blocking FIFO queue, the analog of
// java.util.concurrent.ArrayBlockingQueue.
//
// Put/Offer block while the queue is full and Take/Poll block while it is
// empty. Close unblocks every waiter; values already queued can still be
// drained, and further Put/Offer calls fail.
type Queue[T any] struct {
	mu       sync.Mutex
	notEmpty *sync.Cond
	notFull  *sync.Cond
	buf      []T
	head     int
	size     int
	closed   bool
}

// NewQueue creates a queue holding at most capacity elements. It panics if
// capacity <= 0.
func NewQueue[T any](capacity int) *Queue[T] {
	if capacity <= 0 {
		panic("concurrent: queue capacity must be positive")
	}
	q := &Queue[T]{buf: make([]T, capacity)}
	q.notEmpty = sync.NewCond(&q.mu)
	q.notFull = sync.NewCond(&q.mu)
	return q
}

// Cap returns the fixed capacity.
func (q *Queue[T]) Cap() int { return len(q.buf) }

// Len returns the number of queued elements.
func (q *Queue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.size
}

// Closed reports whether Close has been called.
func (q *Queue[T]) Closed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}

// Close unblocks all waiters. Queued elements stay drainable; Put and Offer
// report failure afterwards.
func (q *Queue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.notEmpty.Broadcast()
	q.notFull.Broadcast()
}

// Put enqueues v, blocking while the queue is full. It reports false when the
// queue was closed (the value is dropped).
func (q *Queue[T]) Put(v T) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.size == len(q.buf) && !q.closed {
		q.notFull.Wait()
	}
	if q.closed {
		return false
	}
	q.enqueueLocked(v)
	q.notEmpty.Signal()
	return true
}

// TryPut enqueues v without blocking, reporting whether it fit.
func (q *Queue[T]) TryPut(v T) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || q.size == len(q.buf) {
		return false
	}
	q.enqueueLocked(v)
	q.notEmpty.Signal()
	return true
}

// Offer enqueues v, waiting at most timeout for space. It reports whether the
// value was enqueued.
func (q *Queue[T]) Offer(v T, timeout time.Duration) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return false
	}
	if timeout <= 0 {
		if q.size == len(q.buf) {
			return false
		}
		q.enqueueLocked(v)
		q.notEmpty.Signal()
		return true
	}
	deadline := time.Now().Add(timeout)
	for q.size == len(q.buf) {
		if q.closed || !time.Now().Before(deadline) {
			return false
		}
		q.waitLocked(q.notFull, deadline)
	}
	q.enqueueLocked(v)
	q.notEmpty.Signal()
	return true
}

// Take dequeues the oldest element, blocking while the queue is empty. It
// reports false once the queue is closed and drained.
func (q *Queue[T]) Take() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.size == 0 && !q.closed {
		q.notEmpty.Wait()
	}
	if q.size == 0 {
		var zero T
		return zero, false
	}
	return q.dequeueLocked(), true
}

// TryTake dequeues the oldest element without blocking.
func (q *Queue[T]) TryTake() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.size == 0 {
		var zero T
		return zero, false
	}
	return q.dequeueLocked(), true
}

// Poll dequeues the oldest element, waiting at most timeout for one to appear.
func (q *Queue[T]) Poll(timeout time.Duration) (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if timeout <= 0 {
		if q.size == 0 {
			var zero T
			return zero, false
		}
		return q.dequeueLocked(), true
	}
	deadline := time.Now().Add(timeout)
	for q.size == 0 {
		if q.closed || !time.Now().Before(deadline) {
			var zero T
			return zero, false
		}
		q.waitLocked(q.notEmpty, deadline)
	}
	return q.dequeueLocked(), true
}

// All drains a snapshot of the queued elements without removing them,
// from oldest to newest.
func (q *Queue[T]) All() []T {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]T, q.size)
	idx := q.head
	for i := range q.size {
		out[i] = q.buf[idx]
		if idx++; idx == len(q.buf) {
			idx = 0
		}
	}
	return out
}

// waitLocked blocks on cond until deadline fires or the cond is signalled; the
// caller holds q.mu and must re-check the queue state afterwards.
func (q *Queue[T]) waitLocked(cond *sync.Cond, deadline time.Time) {
	timer := time.AfterFunc(time.Until(deadline), func() {
		q.mu.Lock()
		cond.Broadcast()
		q.mu.Unlock()
	})
	cond.Wait()
	timer.Stop()
}

func (q *Queue[T]) enqueueLocked(v T) {
	idx := q.head + q.size
	if idx >= len(q.buf) {
		idx -= len(q.buf)
	}
	q.buf[idx] = v
	q.size++
}

func (q *Queue[T]) dequeueLocked() T {
	v := q.buf[q.head]
	clear(q.buf[q.head : q.head+1])
	if q.head++; q.head == len(q.buf) {
		q.head = 0
	}
	q.size--
	q.notFull.Signal()
	return v
}
