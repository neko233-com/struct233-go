// Package ratelimit provides goroutine-safe rate limiters.
//
// Limiter is a token bucket: the bucket starts full and refills lazily, so
// callers never need to sleep just to make it refill.
//
//	l := ratelimit.New(100, 10) // 100 events/s, bursts of 10
//	if l.Allow() {
//		// do work
//	}
//
// SlidingWindow is the exact alternative for "at most N events in any window"
// quotas, where a fixed window would let twice the limit through around the
// boundary.
//
//	w := ratelimit.NewSlidingWindow(1000, time.Second)
//	if w.Allow() {
//		// do work
//	}
//
// The clock is injectable, which keeps tests deterministic: Allow, AllowN,
// Reserve, Remaining, RetryAfter and Tokens read time from the injected clock,
// while Wait always sleeps with real time because it needs a real timer.
package ratelimit

import (
	"context"
	"math"
	"sync"
	"time"
)

// Option configures a limiter. The type is deliberately not generic, so a
// constructor accepts options without the caller repeating anything.
type Option func(*config)

// config accumulates option state until a constructor resolves it.
type config struct {
	now func() time.Time
}

// WithClock sets the time source used to refill tokens and to age sliding
// windows. Default: time.Now. Injecting a clock keeps tests deterministic (no
// sleeping).
func WithClock(now func() time.Time) Option {
	return func(c *config) { c.now = now }
}

// resolveClock applies the options and returns the configured clock, defaulting
// to time.Now. It panics on a nil option or an injected nil clock.
func resolveClock(opts []Option) func() time.Time {
	cfg := config{now: time.Now}
	for _, opt := range opts {
		if opt == nil {
			panic("ratelimit: nil option")
		}
		opt(&cfg)
	}
	if cfg.now == nil {
		panic("ratelimit: nil clock")
	}
	return cfg.now
}

// Limiter is a goroutine-safe token-bucket rate limiter: at most `rate` tokens
// are added per second, the bucket holds at most `burst` tokens.
type Limiter struct {
	mu     sync.Mutex
	rate   float64 // tokens added per second
	burst  int     // bucket capacity
	now    func() time.Time
	last   time.Time // clock reading of the previous refill
	tokens float64   // available tokens, never negative and never above burst
}

// New creates a limiter that admits rate events per second with the given
// burst capacity. It panics when rate is non-finite or <= 0, burst < 1, or an
// option is invalid.
func New(rate float64, burst int, opts ...Option) *Limiter {
	l := &Limiter{rate: rate, burst: burst, now: resolveClock(opts)}
	if l.rate <= 0 || math.IsNaN(l.rate) || math.IsInf(l.rate, 0) {
		panic("ratelimit: rate must be finite and positive")
	}
	if l.burst < 1 {
		panic("ratelimit: burst must be at least 1")
	}
	// start full: pin last to the injected clock so no time is credited for
	// the gap between process start and construction
	l.tokens = float64(l.burst)
	l.last = l.now()
	return l
}

// Rate returns the refill rate in tokens per second.
func (l *Limiter) Rate() float64 { return l.rate }

// Burst returns the bucket capacity.
func (l *Limiter) Burst() int { return l.burst }

// Tokens returns the number of tokens currently available (fractional,
// capped at Burst). Its result is deterministic under an injected clock.
func (l *Limiter) Tokens() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	return l.tokens
}

// Allow reports whether one token was available and consumes it.
func (l *Limiter) Allow() bool { return l.AllowN(1) }

// AllowN is Allow for n tokens: it reports false without consuming anything
// when fewer than n tokens are available, true for n <= 0 (nothing to spend)
// and false for n > Burst (never satisfiable).
func (l *Limiter) AllowN(n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n <= 0 {
		return true
	}
	if n > l.burst {
		return false
	}
	l.refill()
	if l.tokens < float64(n) {
		return false
	}
	l.tokens -= float64(n)
	return true
}

// Reserve returns how long the caller must wait before one token becomes
// available, WITHOUT consuming it (zero when a token is available now). Its
// result is deterministic under an injected clock. It rounds up to the next
// nanosecond and saturates at the maximum time.Duration.
func (l *Limiter) Reserve() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	if l.tokens >= 1 {
		return 0
	}
	d := math.Ceil((1 - l.tokens) / l.rate * float64(time.Second))
	if d >= float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(d)
}

// Wait blocks until one token could be consumed and consumes it, or returns
// ctx.Err() when the context is done first. It sleeps with the real clock, so
// an injected clock does not make it return early.
func (l *Limiter) Wait(ctx context.Context) error {
	for {
		// check the context before spending so a cancelled caller never
		// consumes a token
		if err := ctx.Err(); err != nil {
			return err
		}
		if l.Allow() {
			return nil
		}
		d := l.Reserve()
		if d <= 0 {
			continue
		}
		timer := time.NewTimer(d)
		select {
		case <-ctx.Done():
			// stop the timer on the early-return path so it is not left armed
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// SlidingWindow is a goroutine-safe sliding-window counter limiter: it admits at
// most Limit events in any Window.
//
// A fixed window (a counter reset every Window) admits up to twice the limit
// around a boundary, and a token bucket can admit a whole burst at once. The
// sliding window does neither: it stays exact by remembering when the last
// admitted events happened. That costs one timestamp per slot (24 bytes), so a
// limit of one million events reserves about 24 MB — pick the limit accordingly.
type SlidingWindow struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	times  []time.Time // ring of admitted timestamps, oldest at head
	head   int
	size   int
}

// NewSlidingWindow creates a limiter that admits at most limit events per
// window. It panics when limit < 1, window <= 0 or an option is invalid.
func NewSlidingWindow(limit int, window time.Duration, opts ...Option) *SlidingWindow {
	now := resolveClock(opts)
	if limit < 1 {
		panic("ratelimit: limit must be at least 1")
	}
	if window <= 0 {
		panic("ratelimit: window must be positive")
	}
	return &SlidingWindow{limit: limit, window: window, now: now, times: make([]time.Time, limit)}
}

// Limit returns how many events may be admitted per window.
func (s *SlidingWindow) Limit() int { return s.limit }

// Window returns the window size.
func (s *SlidingWindow) Window() time.Duration { return s.window }

// Allow admits one event when fewer than Limit events were admitted in the last
// Window.
func (s *SlidingWindow) Allow() bool { return s.AllowN(1) }

// AllowN admits n events at once. It reports false without admitting anything
// when only part of the batch fits, true for n <= 0 (nothing to spend) and
// false for n > Limit (never satisfiable). Its result is deterministic under an
// injected clock.
func (s *SlidingWindow) AllowN(n int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n <= 0 {
		return true
	}
	if n > s.limit {
		return false
	}
	now := s.now()
	s.evictLocked(now)
	if s.limit-s.size < n {
		return false
	}
	for range n {
		s.pushLocked(now)
	}
	return true
}

// Remaining returns how many events may still be admitted in the current
// window. Its result is deterministic under an injected clock.
func (s *SlidingWindow) Remaining() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked(s.now())
	return s.limit - s.size
}

// RetryAfter returns how long until the next event can be admitted, or 0 when
// one fits right now. It does not admit anything, which makes it the
// sliding-window counterpart of Reserve.
func (s *SlidingWindow) RetryAfter() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.evictLocked(now)
	if s.size < s.limit {
		return 0
	}
	return s.times[s.head].Add(s.window).Sub(now)
}

// Wait blocks until one event could be admitted and admits it, or returns
// ctx.Err() when the context is done first. Like Limiter.Wait it sleeps with the
// real clock, so an injected clock does not make it return early.
func (s *SlidingWindow) Wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.Allow() {
			return nil
		}
		d := s.RetryAfter()
		if d <= 0 {
			continue
		}
		timer := time.NewTimer(d)
		select {
		case <-ctx.Done():
			// stop the timer on the early-return path so it is not left armed
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// Reset forgets every admitted event, so the next Allow starts a fresh window.
func (s *SlidingWindow) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.times)
	s.head, s.size = 0, 0
}

// evictLocked drops the admitted events that have left the window at now.
// The caller must hold the mutex.
func (s *SlidingWindow) evictLocked(now time.Time) {
	for s.size > 0 {
		if s.times[s.head].Add(s.window).After(now) {
			return
		}
		s.times[s.head] = time.Time{}
		s.head++
		if s.head == len(s.times) {
			s.head = 0
		}
		s.size--
	}
}

// pushLocked records one admitted event at t. The caller must hold the mutex and
// must have evicted first, so the ring is never full here.
func (s *SlidingWindow) pushLocked(t time.Time) {
	idx := s.head + s.size
	if idx >= len(s.times) {
		idx -= len(s.times)
	}
	s.times[idx] = t
	s.size++
}

// refill credits the tokens earned since the previous operation. The caller
// must hold the mutex.
func (l *Limiter) refill() {
	now := l.now()
	elapsed := now.Sub(l.last).Seconds()
	if elapsed < 0 {
		return // retain the high-water mark; a clock rollback earns no tokens
	}
	l.tokens += elapsed * l.rate
	if limit := float64(l.burst); l.tokens > limit {
		l.tokens = limit
	}
	l.last = now
}
