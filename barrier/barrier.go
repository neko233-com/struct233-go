// Package barrier provides a cyclic barrier: a fixed number of goroutines wait
// for each other at a rendezvous point, then the barrier resets for the next
// round (java.util.concurrent.CyclicBarrier).
//
// It is the phase synchronisation primitive behind iterative algorithms: every
// party calls Await at the end of a round and learns its arrival index, and the
// barrier is already usable for the next round.
//
//	b := barrier.New(3) // three parties, any number of rounds
//	for round := 0; round < rounds; round++ {
//		step(round)
//		if _, err := b.Await(ctx); err != nil {
//			return err // ctx was done: this generation is broken
//		}
//	}
package barrier

import (
	"context"
	"errors"
	"sync"
)

// ErrBroken is returned to every waiter of a generation that was broken by a
// cancelled or timed-out waiter, and by Await calls while the barrier stays
// broken.
var ErrBroken = errors.New("barrier: broken")

// Barrier is a cyclic barrier over a fixed number of parties. It is safe for
// concurrent use.
type Barrier struct {
	mu      sync.Mutex
	parties int    // parties per generation, never changes
	waiting int    // parties parked in the current generation
	gen     uint64 // current generation number, cur.gen == gen
	broken  bool   // the current generation is broken and needs Reset
	cur     *genState
}

// genState is the shared state of one generation. Waiters hold the instance
// they arrived in, so a generation that has already been retired is never
// mistaken for the current one.
type genState struct {
	gen     uint64
	release chan struct{} // closed once to wake the whole generation
	broken  bool          // the generation ended by cancellation or Reset
	closed  bool          // release was closed, guards against a double close
}

// close wakes every waiter of the generation. The caller must hold the barrier
// mutex so the closed flag cannot race with a second close.
func (g *genState) close() {
	if g.closed {
		return
	}
	g.closed = true
	close(g.release)
}

// New creates a barrier for the given number of parties (parties >= 1).
// It panics when parties < 1.
func New(parties int) *Barrier {
	if parties < 1 {
		panic("barrier: parties must be at least 1")
	}
	return &Barrier{parties: parties, cur: &genState{release: make(chan struct{})}}
}

// Await blocks until every party of the current generation has arrived. It
// returns the 0-based arrival index of this caller within its generation.
// When ctx is done before the generation completes, the barrier is broken:
// this caller gets ctx.Err(), the other waiters get ErrBroken, and the barrier
// stays broken until Reset is called.
func (b *Barrier) Await(ctx context.Context) (index int, err error) {
	b.mu.Lock()
	if b.broken {
		b.mu.Unlock()
		return 0, ErrBroken
	}
	g := b.cur
	index = b.waiting
	if b.waiting+1 == b.parties {
		// last arrival: complete this generation and open the next one before
		// waking the waiters, so they observe a completed generation
		b.waiting = 0
		b.gen++
		b.cur = &genState{gen: b.gen, release: make(chan struct{})}
		g.close()
		b.mu.Unlock()
		return index, nil
	}
	b.waiting++
	b.mu.Unlock()

	select {
	case <-g.release:
	case <-ctx.Done():
		b.mu.Lock()
		if g.gen != b.gen {
			// the generation ended before the context fired: report its real
			// outcome instead of a spurious cancellation
			broken := g.broken
			b.mu.Unlock()
			if broken {
				return 0, ErrBroken
			}
			return index, nil
		}
		g.broken = true
		b.broken = true
		b.waiting = 0
		g.close()
		b.mu.Unlock()
		return 0, ctx.Err()
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if g.broken {
		return 0, ErrBroken
	}
	return index, nil
}

// Parties returns the configured number of parties.
func (b *Barrier) Parties() int { return b.parties }

// Waiting returns how many parties are waiting in the current generation.
func (b *Barrier) Waiting() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.waiting
}

// Broken reports whether the current generation was broken.
func (b *Barrier) Broken() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.broken
}

// Reset breaks the current generation (waking every waiter with ErrBroken when
// parties were waiting) and starts a fresh, unbroken one.
func (b *Barrier) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	// abandon the current generation: a straggler that wakes on its release
	// channel observes the break instead of a completion
	b.cur.broken = true
	b.cur.close()
	b.waiting = 0
	b.broken = false
	b.gen++
	b.cur = &genState{gen: b.gen, release: make(chan struct{})}
}
