package concurrent

import (
	"cmp"
	"sync"
	"testing"
	"time"
)

// notifyWait observes Cond.Wait releasing its lock, without timing sleeps.
type notifyWait struct {
	sync.Locker
	waiting chan struct{}
	once    sync.Once
}

func (l *notifyWait) Unlock() {
	l.Locker.Unlock()
	l.once.Do(func() { close(l.waiting) })
}

func TestOfferRejectsCloseAndDrainWhileWaiting(t *testing.T) {
	tests := []struct {
		name  string
		setup func(chan struct{}) (func() bool, func(), func() int)
	}{
		{"FIFO", func(waiting chan struct{}) (func() bool, func(), func() int) {
			q := NewArrayBlockingQueue[int](1)
			q.Put(1)
			q.notFull = sync.NewCond(&notifyWait{Locker: &q.mu, waiting: waiting})
			return func() bool { return q.Offer(2, time.Minute) }, func() {
				q.mu.Lock()
				q.closed = true
				q.dequeueLocked()
				q.notFull.Broadcast()
				q.mu.Unlock()
			}, q.Len
		}},
		{"priority", func(waiting chan struct{}) (func() bool, func(), func() int) {
			q := NewHeapBlockingPriorityQueueWithCapacity[int](cmp.Compare[int], 1)
			q.Put(1)
			q.notFull = sync.NewCond(&notifyWait{Locker: &q.mu, waiting: waiting})
			return func() bool { return q.Offer(2, time.Minute) }, func() {
				q.mu.Lock()
				q.closed = true
				q.popLocked()
				q.notFull.Broadcast()
				q.mu.Unlock()
			}, q.Len
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			waiting := make(chan struct{})
			offer, closeAndDrain, length := tt.setup(waiting)
			done := make(chan bool, 1)
			go func() { done <- offer() }()
			select {
			case <-waiting:
			case <-time.After(5 * time.Second):
				t.Fatal("Offer did not enter its wait")
			}
			closeAndDrain()
			select {
			case accepted := <-done:
				if accepted || length() != 0 {
					t.Fatal("Offer inserted into a closed queue after a waiting producer woke")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("Offer did not return after close")
			}
		})
	}
}
