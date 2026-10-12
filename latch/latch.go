// Package latch provides a count-down latch: goroutines block on Await until
// another goroutine has called CountDown the configured number of times
// (java.util.concurrent.CountDownLatch).
//
// It is the one-shot "start together" gate behind fan-out work: workers park on
// a shared latch while the coordinating goroutine releases all of them with a
// single CountDown.
//
//	l := latch.New(2)
//	defer l.CountDown()
//	if err := l.Await(ctx); err != nil {
//		return err
//	}
package latch

import (
	"context"
	"sync"
	"time"
)

// Latch is a single-use count-down latch. Once the count reaches zero it stays
// open forever.
type Latch struct {
	mu     sync.Mutex
	count  int
	opened chan struct{} // closed exactly once, when count reaches zero
}

// New creates a latch that opens after count CountDown calls (count >= 0).
// It panics when count < 0.
func New(count int) *Latch {
	if count < 0 {
		panic("latch: count must not be negative")
	}
	l := &Latch{count: count, opened: make(chan struct{})}
	if count == 0 {
		close(l.opened)
	}
	return l
}

// Count returns how many CountDown calls are still needed (never negative).
func (l *Latch) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.count
}

// CountDown decrements the count, opening the latch and waking every waiter
// when it reaches zero. Extra calls after that are no-ops.
func (l *Latch) CountDown() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.count == 0 {
		return
	}
	l.count--
	if l.count == 0 {
		// the count only ever reaches zero once, so the channel is closed
		// exactly once and a second close cannot happen
		close(l.opened)
	}
}

// Await blocks until the latch opens, or returns ctx.Err() when the context is
// done first. It returns nil immediately when the latch is already open.
func (l *Latch) Await(ctx context.Context) error {
	if l.Open() {
		return nil
	}
	select {
	case <-l.opened:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AwaitTimeout waits up to d for the latch to open and reports whether it did.
func (l *Latch) AwaitTimeout(d time.Duration) bool {
	if l.Open() {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-l.opened:
		return true
	case <-timer.C:
		return false
	}
}

// Open reports whether the latch has reached zero.
func (l *Latch) Open() bool {
	// opened is never replaced, so a non-blocking receive is enough and keeps
	// this path free of the mutex
	select {
	case <-l.opened:
		return true
	default:
		return false
	}
}
