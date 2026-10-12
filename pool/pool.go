// Package pool provides a bounded, goroutine-safe object pool that recycles
// short-lived allocations.
//
// It is the typed, observable alternative to sync.Pool: entries are handed
// back in LIFO order (so the hottest object stays in cache), the idle set never
// grows past a maximum, an optional reset hook returns every object in a clean
// state, and the hit/miss counters make the reuse rate visible.
//
//	type packet struct{ buf []byte }
//
//	packets := pool.New(func() *packet { return new(packet) },
//		pool.WithReset(func(p *packet) { p.buf = p.buf[:0] }),
//		pool.WithMaxSize(1024),
//	)
//	p := packets.Get()
//	defer packets.Put(p)
//
// Note that this is an object pool: for pooling goroutines, see the workerpool
// package.
package pool

import "sync"

// Stats reports the cumulative counters of a Pool.
type Stats struct {
	// Gets is the number of Get calls.
	Gets int64
	// Hits is the number of Get calls answered from the pool.
	Hits int64
	// Misses is the number of Get calls that had to create an object.
	Misses int64
	// Puts is the number of Put calls that stored an object.
	Puts int64
	// Discards is the number of Put calls dropped because the idle set was
	// full.
	Discards int64
}

// ReuseRate returns Hits/Gets in [0, 1], the fraction of Get calls answered
// from the pool. It is 0 before the first Get.
func (s Stats) ReuseRate() float64 {
	if s.Gets == 0 {
		return 0
	}
	return float64(s.Hits) / float64(s.Gets)
}

// Option configures a Pool. Build one with WithMaxSize or WithReset; the type
// is deliberately not generic, so New accepts options without the caller
// repeating the pooled element type.
type Option func(*config)

// config accumulates option state until New resolves it against the concrete
// element type.
type config struct {
	max    int
	hasMax bool
	reset  any
}

// WithMaxSize caps how many idle objects are retained for reuse. A capacity of
// zero or less means unbounded, which risks holding on to every object the
// program ever released.
func WithMaxSize(max int) Option {
	return func(c *config) {
		c.max, c.hasMax = max, true
	}
}

// WithReset sets the hook run on every Put before the object is stored, so
// callers always get a clean object back. It is also run by Clear on every
// dropped object, letting references held by the object become collectable.
// The hook type must match the pooled type.
func WithReset[T any](reset func(T)) Option {
	return func(c *config) { c.reset = reset }
}

// Pool is a bounded, goroutine-safe LIFO pool of reusable T values. The zero
// value is not usable; call New.
type Pool[T any] struct {
	mu    sync.Mutex
	items []T
	newFn func() T
	reset func(T)
	max   int
	stats Stats
}

// New creates a pool that builds new objects with newFn. It panics when newFn
// is nil, when a negative maximum size is requested, or when the reset hook
// does not accept the pooled type.
func New[T any](newFn func() T, opts ...Option) *Pool[T] {
	if newFn == nil {
		panic("pool: newFn must not be nil")
	}
	var cfg config
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(&cfg)
	}
	if cfg.hasMax && cfg.max < 0 {
		panic("pool: max size must not be negative")
	}

	p := &Pool[T]{newFn: newFn, max: cfg.max}
	if cfg.reset != nil {
		reset, ok := cfg.reset.(func(T))
		if !ok {
			panic("pool: reset hook must accept the pooled type")
		}
		if reset == nil {
			panic("pool: reset hook must not be nil")
		}
		p.reset = reset
	}
	return p
}

// Get returns a pooled object, creating one with the factory when the pool is
// empty. The factory runs outside the internal lock, so it may be slow.
func (p *Pool[T]) Get() T {
	p.mu.Lock()
	if n := len(p.items); n > 0 {
		v := p.items[n-1]
		var zero T
		p.items[n-1] = zero // do not keep the object alive through the idle slice
		p.items = p.items[:n-1]
		p.stats.Gets++
		p.stats.Hits++
		p.mu.Unlock()
		return v
	}
	p.stats.Gets++
	p.stats.Misses++
	newFn := p.newFn
	p.mu.Unlock()
	return newFn()
}

// Put returns v to the pool after running the reset hook. When the idle set is
// already at its maximum size the object is dropped and counted as a discard,
// so a burst of Put calls cannot pin unbounded memory.
func (p *Pool[T]) Put(v T) {
	if p.reset != nil {
		p.reset(v) // outside the lock: the hook is caller code
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.max > 0 && len(p.items) >= p.max {
		p.stats.Discards++
		return
	}
	p.items = append(p.items, v)
	p.stats.Puts++
}

// Prealloc fills the pool with n idle objects built by the factory, so the
// first n Get calls are guaranteed to hit. Objects that do not fit into a
// bounded pool are dropped. It panics when n is negative.
func (p *Pool[T]) Prealloc(n int) {
	if n < 0 {
		panic("pool: prealloc count must not be negative")
	}
	if n == 0 {
		return
	}
	// build outside the lock, then publish: the factory is arbitrary code
	created := make([]T, 0, n)
	for range n {
		created = append(created, p.newFn())
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, v := range created {
		if p.max > 0 && len(p.items) >= p.max {
			return
		}
		p.items = append(p.items, v)
	}
}

// Len returns the number of idle objects available for reuse.
func (p *Pool[T]) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.items)
}

// Cap returns the maximum number of idle objects, or 0 when unbounded.
func (p *Pool[T]) Cap() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.max
}

// Clear drops every idle object, running the reset hook on each one. The pool
// stays usable for later Get and Put calls.
func (p *Pool[T]) Clear() {
	p.mu.Lock()
	items := p.items
	p.items = nil // release the backing array as well as the objects
	p.mu.Unlock()

	if p.reset == nil {
		return
	}
	for _, v := range items {
		p.reset(v)
	}
}

// Stats returns a snapshot of the counters.
func (p *Pool[T]) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stats
}
