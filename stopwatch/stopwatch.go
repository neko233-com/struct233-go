// Package stopwatch provides a Guava-style stopwatch: an elapsed-time
// measurer that accumulates every start..stop segment and ignores the gaps in
// between (com.google.common.base.Stopwatch).
//
// Use it whenever a measured span may pause and resume: a request retried
// across back-offs, a level played in several sessions, or any "busy time"
// accounting. The clock is injectable, which keeps tests deterministic
// without sleeping.
//
//	sw := stopwatch.NewStarted()
//	work()
//	sw.Stop()
//	fmt.Println(sw.Elapsed())
//
// Invalid state transitions panic in the repository style where Guava would
// throw IllegalStateException: starting a running stopwatch panics with
// "stopwatch: already started", stopping a stopped one with
// "stopwatch: not running".
package stopwatch

import (
	"sync"
	"time"
)

// Stopwatch measures elapsed time across start/stop segments. Create one with
// New or NewStarted; the zero value is not usable.
//
// Stopwatch is safe for concurrent use: the control methods (Start, Stop,
// Reset, Restart) and the read methods (Elapsed, IsRunning, String) may be
// called from different goroutines.
type Stopwatch struct {
	mu          sync.Mutex
	now         func() time.Time // time source, injected by WithClock
	startedAt   time.Time        // when the current segment began; valid while running
	accumulated time.Duration    // total of all completed segments
	running     bool             // whether a segment is in progress
}

// Option configures a Stopwatch created by New or NewStarted.
type Option func(*Stopwatch)

// WithClock sets the time source used to measure the segments. Default
// time.Now. Injecting a clock keeps tests deterministic; the caller guarantees
// that a custom clock is monotonic. It panics with "stopwatch: nil clock" when
// applied to a Stopwatch and now is nil.
func WithClock(now func() time.Time) Option {
	return func(s *Stopwatch) { s.now = now }
}

// New creates a stopped Stopwatch with zero elapsed time. It panics with
// "stopwatch: nil option" when an Option is nil, and with
// "stopwatch: nil clock" when an Option injects a nil clock.
func New(opts ...Option) *Stopwatch {
	s := &Stopwatch{now: time.Now}
	for _, opt := range opts {
		if opt == nil {
			panic("stopwatch: nil option")
		}
		opt(s)
	}
	if s.now == nil {
		panic("stopwatch: nil clock")
	}
	return s
}

// NewStarted creates a Stopwatch like New and starts its first segment right
// away. It panics under the same conditions as New.
func NewStarted(opts ...Option) *Stopwatch {
	s := New(opts...)
	s.Start()
	return s
}

// Start opens a new measurement segment and returns the receiver so calls can
// be chained. It panics with "stopwatch: already started" when the stopwatch
// is already running.
func (s *Stopwatch) Start() *Stopwatch {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		panic("stopwatch: already started")
	}
	s.startedAt = s.now()
	s.running = true
	return s
}

// Stop closes the current segment, adding its duration to the accumulated
// total, and returns the receiver so calls can be chained. It panics with
// "stopwatch: not running" when no segment is open.
func (s *Stopwatch) Stop() *Stopwatch {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		panic("stopwatch: not running")
	}
	s.accumulated += s.now().Sub(s.startedAt)
	s.running = false
	return s
}

// Reset zeroes the elapsed time and leaves the stopwatch stopped. It never
// panics: resetting a running stopwatch simply drops the open segment.
func (s *Stopwatch) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accumulated = 0
	s.startedAt = time.Time{}
	s.running = false
}

// Restart zeroes the elapsed time and opens a fresh segment in one step, the
// equivalent of Reset followed by Start. It never panics: restarting a running
// stopwatch drops the open segment and starts a new one.
func (s *Stopwatch) Restart() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accumulated = 0
	s.startedAt = s.now()
	s.running = true
}

// Elapsed returns the measured time so far: the sum of all completed segments
// plus, while running, the time since the last Start. Time that passes while
// the stopwatch is stopped is never counted.
func (s *Stopwatch) Elapsed() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.elapsedLocked()
}

// IsRunning reports whether a segment is currently open, that is whether Start
// was called more recently than Stop.
func (s *Stopwatch) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// String returns a human-readable form of the elapsed time, such as "1.5s"
// when stopped and "1.5s (running)" while a segment is open.
func (s *Stopwatch) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.elapsedLocked().String()
	if s.running {
		out += " (running)"
	}
	return out
}

// elapsedLocked computes the current total. s.mu must be held.
func (s *Stopwatch) elapsedLocked() time.Duration {
	total := s.accumulated
	if s.running {
		total += s.now().Sub(s.startedAt)
	}
	return total
}
