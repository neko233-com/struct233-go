// Package delayqueue provides an unbounded blocking queue whose elements only
// become available once their deadline has passed (java.util.concurrent.DelayQueue).
//
// It is the primitive behind "run this at T" without one timer per element: Put
// stores an Element carrying a deadline, and Take blocks until the earliest
// element is due, so a couple of goroutines can drive thousands of pending
// expiries at once.
//
//	dq := delayqueue.New[Order]()
//	dq.Put(delayqueue.NewElement(order, time.Now().Add(30*time.Second)))
//	if e, ok := dq.Take(); ok {          // blocks until the order is due
//		handle(e.Value())
//	}
//
// The clock is injectable. NewWithClock decides when an element counts as due,
// which keeps the deadline logic deterministic in tests; because an injected
// clock says nothing about real time, such a queue re-checks its deadlines
// while blocking instead of sleeping until the deadline.
package delayqueue

import (
	"sync"
	"time"
)

// pollInterval is how often a queue with an injected clock re-checks its
// deadlines while blocking. The system clock needs no polling: it can compute
// the exact real wait.
const pollInterval = time.Millisecond

// Element is a queued value together with the time it becomes available.
type Element[T any] struct {
	value    T
	deadline time.Time
	seq      uint64 // insertion order, breaks deadline ties (FIFO)
}

// NewElement wraps value with the deadline at which it becomes available.
func NewElement[T any](value T, deadline time.Time) *Element[T] {
	return &Element[T]{value: value, deadline: deadline}
}

// Value returns the wrapped value.
func (e *Element[T]) Value() T { return e.value }

// Deadline returns the time the element becomes available.
func (e *Element[T]) Deadline() time.Time { return e.deadline }

// Expired reports whether the element is due at now.
func (e *Element[T]) Expired(now time.Time) bool { return !e.deadline.After(now) }

// Delay returns how long the element still has to wait at now; it is 0 or
// negative once the element is due.
func (e *Element[T]) Delay(now time.Time) time.Duration { return e.deadline.Sub(now) }

// Queue is an unbounded blocking delay queue. It is safe for concurrent use.
type Queue[T any] struct {
	mu       sync.Mutex
	notEmpty *sync.Cond
	items    []*Element[T] // min-heap on deadline
	now      func() time.Time
	polling  bool // the clock is injected, so blocking waits poll
	closed   bool
	seq      uint64
}

// New creates an empty queue driven by the system clock.
func New[T any]() *Queue[T] {
	return newQueue[T](time.Now, false)
}

// NewWithClock creates an empty queue that reads the current time from now,
// which keeps tests deterministic. Blocking calls re-check their deadlines at a
// short fixed interval because an injected clock cannot be converted into a real
// wait; the system clock has no such penalty.
func NewWithClock[T any](now func() time.Time) *Queue[T] {
	if now == nil {
		panic("delayqueue: nil clock")
	}
	return newQueue[T](now, true)
}

func newQueue[T any](now func() time.Time, polling bool) *Queue[T] {
	q := &Queue[T]{now: now, polling: polling}
	q.notEmpty = sync.NewCond(&q.mu)
	return q
}

// Put stores e, which never blocks because the queue is unbounded. It reports
// false when the queue is closed (the element is dropped).
func (q *Queue[T]) Put(e *Element[T]) bool {
	if e == nil {
		panic("delayqueue: element must not be nil")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return false
	}
	q.seq++
	e.seq = q.seq
	q.pushLocked(e)
	q.notEmpty.Broadcast() // a new earliest element must wake blocked takers
	return true
}

// Take removes the earliest due element, blocking until one is due. It reports
// false once the queue is closed and no due element is left.
func (q *Queue[T]) Take() (*Element[T], bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for {
		if e := q.popDueLocked(); e != nil {
			return e, true
		}
		if q.closed {
			return nil, false
		}
		q.waitLocked(q.remainingLocked())
	}
}

// TryTake removes the earliest element if it is already due, without blocking.
func (q *Queue[T]) TryTake() (*Element[T], bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if e := q.popDueLocked(); e != nil {
		return e, true
	}
	return nil, false
}

// Poll removes the earliest due element, waiting at most timeout for one to
// become due. A timeout <= 0 only takes an element that is due right now.
func (q *Queue[T]) Poll(timeout time.Duration) (*Element[T], bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	for {
		if e := q.popDueLocked(); e != nil {
			return e, true
		}
		if q.closed || timeout <= 0 {
			return nil, false
		}
		left := time.Until(deadline)
		if left <= 0 {
			return nil, false
		}
		if r := q.remainingLocked(); r > 0 && r < left {
			left = r // the earliest element is due before the timeout: re-check then
		}
		q.waitLocked(left)
	}
}

// Peek returns the earliest element without removing it, whether or not it is
// due yet.
func (q *Queue[T]) Peek() (*Element[T], bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil, false
	}
	return q.items[0], true
}

// Len returns how many elements are queued, due or not.
func (q *Queue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// IsEmpty reports whether the queue holds no elements.
func (q *Queue[T]) IsEmpty() bool {
	return q.Len() == 0
}

// Closed reports whether Close has been called.
func (q *Queue[T]) Closed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}

// Clear drops every queued element, due or not.
func (q *Queue[T]) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	clear(q.items)
	q.items = q.items[:0]
}

// Drain removes up to n due elements in deadline order and returns them. A
// non-positive n drains every element that is due.
func (q *Queue[T]) Drain(n int) []*Element[T] {
	q.mu.Lock()
	defer q.mu.Unlock()

	var out []*Element[T]
	for n <= 0 || len(out) < n {
		e := q.popDueLocked()
		if e == nil {
			break
		}
		out = append(out, e)
	}
	return out
}

// Close unblocks every waiter: Take and Poll report false once the elements
// already queued have been drained, and Put reports false afterwards.
func (q *Queue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.notEmpty.Broadcast()
}

// remainingLocked returns how long the caller should wait before the earliest
// element becomes due, or 0 when there is nothing to wait for (an empty queue,
// or an injected clock that is polled instead). The caller must hold the mutex.
func (q *Queue[T]) remainingLocked() time.Duration {
	if q.polling || len(q.items) == 0 {
		return 0
	}
	if d := q.items[0].Delay(q.now()); d > 0 {
		return d
	}
	return 0
}

// popDueLocked removes and returns the earliest element when it is due.
// The caller must hold the mutex.
func (q *Queue[T]) popDueLocked() *Element[T] {
	if len(q.items) == 0 || !q.items[0].Expired(q.now()) {
		return nil
	}
	return q.popLocked()
}

// waitLocked blocks until the queue state may have changed: a Put or Close
// broadcasts, the earliest element becomes due, or left elapses (left <= 0 means
// "no timeout"). The caller must hold the mutex and must re-check the queue
// afterwards.
func (q *Queue[T]) waitLocked(left time.Duration) {
	switch {
	case q.polling:
		// an injected clock cannot be translated into a real wait, so re-check
		// on a short interval instead
		q.condWaitLocked(pollInterval)
	case len(q.items) == 0 && left <= 0:
		q.notEmpty.Wait() // only a Put or Close can help
	case left > 0:
		q.condWaitLocked(left)
	default:
		// the earliest element is due, so the caller is about to pop it
	}
}

// condWaitLocked waits on the cond for at most d, using a timer to broadcast
// when the deadline passes.
func (q *Queue[T]) condWaitLocked(d time.Duration) {
	timer := time.AfterFunc(d, func() {
		q.mu.Lock()
		q.notEmpty.Broadcast()
		q.mu.Unlock()
	})
	q.notEmpty.Wait()
	timer.Stop()
}

// pushLocked inserts e into the min-heap.
func (q *Queue[T]) pushLocked(e *Element[T]) {
	q.items = append(q.items, e)
	i := len(q.items) - 1
	for i > 0 {
		parent := (i - 1) / 2
		if !q.lessLocked(q.items[i], q.items[parent]) {
			break
		}
		q.items[i], q.items[parent] = q.items[parent], q.items[i]
		i = parent
	}
}

// popLocked removes and returns the earliest element.
func (q *Queue[T]) popLocked() *Element[T] {
	n := len(q.items) - 1
	q.items[0], q.items[n] = q.items[n], q.items[0]
	top := q.items[n]
	q.items[n] = nil // do not pin the element
	q.items = q.items[:n]

	for i := 0; ; {
		left, right := 2*i+1, 2*i+2
		smallest := i
		if left < n && q.lessLocked(q.items[left], q.items[smallest]) {
			smallest = left
		}
		if right < n && q.lessLocked(q.items[right], q.items[smallest]) {
			smallest = right
		}
		if smallest == i {
			break
		}
		q.items[i], q.items[smallest] = q.items[smallest], q.items[i]
		i = smallest
	}
	return top
}

// lessLocked orders elements by deadline, then by insertion order so that equal
// deadlines keep FIFO order.
func (q *Queue[T]) lessLocked(a, b *Element[T]) bool {
	if a.deadline.Equal(b.deadline) {
		return a.seq < b.seq
	}
	return a.deadline.Before(b.deadline)
}
