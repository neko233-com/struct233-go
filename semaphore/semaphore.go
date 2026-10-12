// Package semaphore provides a counting semaphore, the analog of
// java.util.concurrent.Semaphore.
//
// A semaphore owns a fixed number of permits: a caller takes one or more
// before touching a shared resource and gives them back afterwards, so at most
// Limit callers hold a permit at the same time. Callers block while none are
// left.
//
//	sem := semaphore.New(4) // at most 4 goroutines in the critical section
//	if err := sem.Acquire(ctx); err != nil {
//		return err
//	}
//	defer sem.Release(1)
//	doWork()
//
// The semaphore is not strictly fair. Blocked callers are kept in a FIFO queue
// and woken in arrival order, but TryAcquire, TryAcquireN and AcquireTimeout
// may take permits in front of that queue, exactly like the default (non-fair)
// JDK implementation. A queued request is never skipped either, so a request
// for many permits at the head of the queue delays the smaller ones behind it
// until it fits.
package semaphore

import (
	"context"
	"math"
	"sync"
	"time"
)

// Semaphore is a counting semaphore (java.util.concurrent.Semaphore): it hands
// out at most a fixed number of permits, and callers block while none are left.
// It is safe for concurrent use.
type Semaphore struct {
	mu        sync.Mutex
	permits   int64     // configured permit count, immutable after New
	available int64     // free permits; Release may push it above permits
	waiters   []*waiter // FIFO queue of blocked callers
}

// waiter is one blocked caller: it needs n permits and is released by closing
// ready once those permits have been deducted for it.
type waiter struct {
	n       int64
	ready   chan struct{}
	granted bool // set under the mutex right before ready is closed
}

// New creates a semaphore holding the given number of permits (> 0).
// It panics when permits <= 0.
func New(permits int64) *Semaphore {
	if permits <= 0 {
		panic("semaphore: permits must be positive")
	}
	return &Semaphore{permits: permits, available: permits}
}

// Acquire takes one permit, blocking until one is free or ctx is done.
// It returns ctx.Err() when the context ends first.
func (s *Semaphore) Acquire(ctx context.Context) error { return s.AcquireN(ctx, 1) }

// AcquireN takes n permits at once, blocking until all of them are free or ctx
// is done (no partial acquisition). It returns nil immediately for n <= 0 and
// ctx.Err() when the context ends first.
func (s *Semaphore) AcquireN(ctx context.Context, n int64) error {
	if n <= 0 {
		return nil
	}
	// check the context before spending so a cancelled caller never takes a
	// permit in the fast path below
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	// barge only when nobody is queued: taking permits ahead of a queued
	// caller would starve it
	if len(s.waiters) == 0 && s.available >= n {
		s.available -= n
		s.mu.Unlock()
		return nil
	}
	w := &waiter{n: n, ready: make(chan struct{})}
	s.waiters = append(s.waiters, w)
	s.mu.Unlock()

	select {
	case <-w.ready:
		return nil
	case <-ctx.Done():
		s.mu.Lock()
		defer s.mu.Unlock()
		if w.granted {
			// Release handed the permits over in the same instant: keep the
			// grant instead of dropping it on the floor
			return nil
		}
		s.removeLocked(w)
		s.notifyLocked()
		return ctx.Err()
	}
}

// TryAcquire takes one permit without blocking, reporting whether it succeeded.
func (s *Semaphore) TryAcquire() bool { return s.TryAcquireN(1) }

// TryAcquireN tries to take n permits without blocking. It reports true for
// n <= 0 and false for n > Limit (never satisfiable). It does not check the
// waiter queue, so it may take permits ahead of blocked callers.
func (s *Semaphore) TryAcquireN(n int64) bool {
	if n <= 0 {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if n > s.permits || s.available < n {
		return false
	}
	s.available -= n
	return true
}

// AcquireTimeout is AcquireN with a real-time deadline: it reports false when
// the permits did not become free within d. A non-positive d only attempts an
// immediate TryAcquireN. The timed-out waiter is always removed from the queue.
func (s *Semaphore) AcquireTimeout(n int64, d time.Duration) bool {
	if n <= 0 {
		return true
	}
	if d <= 0 {
		return s.TryAcquireN(n)
	}
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	return s.AcquireN(ctx, n) == nil
}

// Release returns n permits to the semaphore. It panics when n < 0 or the
// resulting permit count would overflow int64, leaving the count unchanged.
//
// Queued callers are served in FIFO order while their whole request fits.
// Releasing more permits than are missing pushes Permits above Limit, which
// callers can use to grow the limit at runtime; balancing those extra permits
// stays the caller's responsibility, as in java.util.concurrent.Semaphore.
// Release(0) is a no-op.
func (s *Semaphore) Release(n int64) {
	if n < 0 {
		panic("semaphore: release count must not be negative")
	}
	if n == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if n > math.MaxInt64-s.available {
		panic("semaphore: permit count overflow")
	}
	s.available += n
	s.notifyLocked()
}

// notifyLocked grants every satisfiable request at the head of the queue.
// Removing a cancelled head can make existing free permits usable again.
func (s *Semaphore) notifyLocked() {
	for len(s.waiters) > 0 && s.available >= s.waiters[0].n {
		w := s.waiters[0]
		s.waiters[0] = nil // do not keep the granted waiter alive through the queue
		s.waiters = s.waiters[1:]
		s.available -= w.n
		w.granted = true
		close(w.ready)
	}
}

// Permits returns the number of permits currently free.
func (s *Semaphore) Permits() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.available
}

// Limit returns the configured number of permits.
func (s *Semaphore) Limit() int64 { return s.permits }

// Waiters returns how many callers are currently blocked.
func (s *Semaphore) Waiters() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.waiters)
}

// removeLocked drops w from the waiter queue. The caller must hold the mutex;
// a waiter that is no longer queued was already granted.
func (s *Semaphore) removeLocked(w *waiter) {
	for i, q := range s.waiters {
		if q != w {
			continue
		}
		copy(s.waiters[i:], s.waiters[i+1:])
		s.waiters[len(s.waiters)-1] = nil // do not keep the abandoned waiter alive
		s.waiters = s.waiters[:len(s.waiters)-1]
		return
	}
}
