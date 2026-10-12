// Package memoize provides lazy memoizing suppliers: a Supplier computes a
// value on the first Get and serves the cached result afterwards
// (com.google.common.base.Suppliers.memoize / memoizeWithExpiration).
//
// It fits expensive one-shot setup such as loading a config or building a
// connection pool: the value is computed at most once no matter how many
// goroutines ask for it, and MemoizeExpiring refreshes it after a TTL.
//
//	s := memoize.Memoize(func() *Config { return loadConfig() })
//	cfg := s.Get() // computed here, every later call reuses it
//
// The clock is injectable, which keeps the expiration testable without
// sleeping.
package memoize

import (
	"sync"
	"sync/atomic"
	"time"
)

// Option configures a Supplier at construction time.
type Option func(*supplierOptions)

// supplierOptions collects the optional settings a Supplier is built from.
type supplierOptions struct {
	now func() time.Time
}

// WithClock sets the time source used for expiration. Default time.Now.
// Injecting a clock keeps expiration tests deterministic. A nil clock panics
// when the Supplier is constructed ("memoize: nil clock").
func WithClock(now func() time.Time) Option {
	return func(o *supplierOptions) { o.now = now }
}

// Stats is a snapshot of the Supplier hit/miss counters.
type Stats struct {
	// Hits is the number of Get calls answered from the cache.
	Hits uint64
	// Misses is the number of Get calls that had to run the supplier function,
	// including attempts whose function panicked.
	Misses uint64
}

// Supplier is a goroutine-safe memoizing value supplier (Guava's
// Suppliers.memoize / memoizeWithExpiration).
//
// Get computes the value exactly once and then reuses it, until Invalidate or
// (for an expiring supplier) the TTL forces a recomputation. Concurrent first
// calls run the function exactly once: the other callers wait for that single
// computation instead of starting their own (singleflight). When the function
// panics, the panic propagates to the caller and nothing is cached, so a later
// Get retries.
//
// A Supplier must not be copied after first use.
type Supplier[T any] struct {
	fn  func() T
	now func() time.Time
	ttl time.Duration // 0 means the value never expires

	mu        sync.Mutex
	value     T
	computed  bool
	expiresAt time.Time

	hits   atomic.Uint64
	misses atomic.Uint64
}

// Memoize returns a Supplier that runs fn exactly once: the first Get computes
// the value and every later Get returns that same cached value. A nil fn
// panics ("memoize: nil fn").
func Memoize[T any](fn func() T, opts ...Option) *Supplier[T] {
	return newSupplier(fn, 0, opts)
}

// MemoizeExpiring returns a Supplier that caches the value of fn for ttl and
// recomputes it on the first Get after the TTL has elapsed (Guava's
// Suppliers.memoizeWithExpiration: one recomputation per expiration). A
// ttl <= 0 panics ("memoize: ttl must be positive") and a nil fn panics
// ("memoize: nil fn").
func MemoizeExpiring[T any](fn func() T, ttl time.Duration, opts ...Option) *Supplier[T] {
	if ttl <= 0 {
		panic("memoize: ttl must be positive")
	}
	return newSupplier(fn, ttl, opts)
}

// newSupplier builds a Supplier from the shared constructor arguments.
func newSupplier[T any](fn func() T, ttl time.Duration, opts []Option) *Supplier[T] {
	if fn == nil {
		panic("memoize: nil fn")
	}
	o := supplierOptions{now: time.Now}
	for _, opt := range opts {
		if opt == nil {
			panic("memoize: nil option")
		}
		opt(&o)
	}
	if o.now == nil {
		panic("memoize: nil clock")
	}
	return &Supplier[T]{fn: fn, now: o.now, ttl: ttl}
}

// Get returns the memoized value, running the supplier function when no fresh
// value is cached. Concurrent first calls run fn exactly once while the other
// callers wait for that computation. A panicking fn is not cached: the panic
// reaches the caller and the next Get retries.
func (s *Supplier[T]) Get() T {
	s.mu.Lock()
	if s.freshLocked() {
		s.hits.Add(1)
		v := s.value
		s.mu.Unlock()
		return v
	}
	return s.computeLocked()
}

// freshLocked reports whether a usable value is cached: a non-expiring
// supplier keeps it forever, an expiring one until the TTL elapses (the value
// is considered expired exactly at the deadline, Guava style). s.mu must be
// held.
func (s *Supplier[T]) freshLocked() bool {
	if !s.computed {
		return false
	}
	if s.ttl == 0 {
		return true
	}
	return s.now().Before(s.expiresAt)
}

// computeLocked runs fn and caches its result. s.mu must be held on entry; the
// mutex is released even when fn panics, in which case nothing is cached.
func (s *Supplier[T]) computeLocked() T {
	defer s.mu.Unlock()
	s.misses.Add(1)
	v := s.fn()
	s.value = v
	s.computed = true
	if s.ttl > 0 {
		s.expiresAt = s.now().Add(s.ttl)
	}
	return v
}

// Invalidate drops the cached value so the next Get runs the function again.
// It blocks while a computation is in flight and then discards that result,
// keeping "the next Get recomputes" true even under concurrency.
func (s *Supplier[T]) Invalidate() {
	s.mu.Lock()
	s.computed = false
	s.expiresAt = time.Time{}
	s.mu.Unlock()
}

// Stats returns a snapshot of the hit/miss counters.
func (s *Supplier[T]) Stats() Stats {
	return Stats{Hits: s.hits.Load(), Misses: s.misses.Load()}
}
