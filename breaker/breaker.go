// Package breaker provides a goroutine-safe circuit breaker.
//
// A breaker shields a caller from a dependency that keeps failing: consecutive
// failures open the circuit, every call is refused for a cooldown, then a
// limited number of probes test the recovery before the circuit closes again.
// A caller either goes through Execute or pairs every successful Allow with
// exactly one Success or Failure.
//
//	b := breaker.New(
//		breaker.WithFailureThreshold(5),
//		breaker.WithOpenTimeout(5*time.Second),
//	)
//	if err := b.Execute(func() error { return call() }); errors.Is(err, breaker.ErrOpen) {
//		// call was not run: the circuit is open
//	}
//
// The clock is injectable, which keeps the open timeout testable without
// sleeping.
package breaker

import (
	"errors"
	"strconv"
	"sync"
	"time"
)

// defaults a Breaker starts from before any Option is applied.
const (
	defaultFailureThreshold = 5
	defaultSuccessThreshold = 2
	defaultOpenTimeout      = 5 * time.Second
	defaultHalfOpenMaxCalls = 1
)

// State is the circuit state.
type State int

const (
	// StateClosed lets every call through and counts failures.
	StateClosed State = iota
	// StateOpen refuses every call until OpenTimeout elapses.
	StateOpen
	// StateHalfOpen admits a limited number of probes to test the recovery.
	StateHalfOpen
)

// String returns the state name ("closed", "open", "half-open").
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		// an out-of-range value must not masquerade as a known state
		return "State(" + strconv.Itoa(int(s)) + ")"
	}
}

// ErrOpen is returned by Execute while the circuit is open.
var ErrOpen = errors.New("breaker: circuit is open")

// Option configures a Breaker.
type Option func(*Breaker)

// WithFailureThreshold sets how many consecutive failures open the circuit.
// Default 5. Values < 1 panic when applied.
func WithFailureThreshold(n int) Option {
	return func(b *Breaker) { b.failureThreshold = n }
}

// WithSuccessThreshold sets how many consecutive half-open successes close the
// circuit again. Default 2. Values < 1 panic when applied.
func WithSuccessThreshold(n int) Option {
	return func(b *Breaker) { b.successThreshold = n }
}

// WithOpenTimeout sets how long the circuit stays open before half-open probes
// are allowed. Default 5s. Values <= 0 panic when applied.
func WithOpenTimeout(d time.Duration) Option {
	return func(b *Breaker) { b.openTimeout = d }
}

// WithHalfOpenMaxCalls sets how many half-open probes may run at the same time.
// Default 1. Values < 1 panic when applied.
func WithHalfOpenMaxCalls(n int) Option {
	return func(b *Breaker) { b.halfOpenMaxCalls = n }
}

// WithClock sets the time source used for the open timeout. Default time.Now.
// Injecting a clock keeps tests deterministic.
func WithClock(now func() time.Time) Option {
	return func(b *Breaker) { b.now = now }
}

// Counts is a snapshot of the breaker counters.
type Counts struct {
	// Successes is the number of successful calls recorded.
	Successes int64
	// Failures is the number of failed calls recorded.
	Failures int64
	// Rejections is the number of calls refused while the breaker was open.
	Rejections int64
	// ConsecutiveFailures is the number of failures since the last success.
	ConsecutiveFailures int
	// ConsecutiveSuccesses is the number of half-open successes since the
	// breaker entered half-open.
	ConsecutiveSuccesses int
}

// Breaker is a goroutine-safe circuit breaker.
//
// A caller that was admitted by Allow must report exactly one outcome with
// Success or Failure; a half-open probe that reports neither keeps its slot
// spent. Manual outcomes apply to the current state because they carry no call
// identity. Use Execute when calls may overlap state transitions or Reset: it
// discards outcomes belonging to an earlier circuit generation.
type Breaker struct {
	mu sync.Mutex

	failureThreshold int
	successThreshold int
	openTimeout      time.Duration
	halfOpenMaxCalls int
	now              func() time.Time

	state            State
	openedAt         time.Time // clock reading of the transition to open
	halfOpenInFlight int       // admitted probes that have not reported yet
	counts           Counts
	generation       uint64 // changes whenever the circuit changes state or resets
}

// New creates a breaker with the default settings. It panics on invalid options.
func New(opts ...Option) *Breaker {
	b := &Breaker{
		failureThreshold: defaultFailureThreshold,
		successThreshold: defaultSuccessThreshold,
		openTimeout:      defaultOpenTimeout,
		halfOpenMaxCalls: defaultHalfOpenMaxCalls,
		now:              time.Now,
	}
	for _, opt := range opts {
		if opt == nil {
			panic("breaker: nil option")
		}
		opt(b)
	}
	if b.failureThreshold < 1 {
		panic("breaker: failure threshold must be at least 1")
	}
	if b.successThreshold < 1 {
		panic("breaker: success threshold must be at least 1")
	}
	if b.openTimeout <= 0 {
		panic("breaker: open timeout must be positive")
	}
	if b.halfOpenMaxCalls < 1 {
		panic("breaker: half-open max calls must be at least 1")
	}
	if b.now == nil {
		panic("breaker: nil clock")
	}
	return b
}

// Allow reports whether a call may proceed, spending a half-open probe slot
// when the circuit is half-open. It returns false while the circuit is open
// (and counts a rejection).
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.allowLocked()
}

func (b *Breaker) allowLocked() bool {
	b.enterHalfOpen()
	switch b.state {
	case StateOpen:
		b.counts.Rejections++
		return false
	case StateHalfOpen:
		if b.halfOpenInFlight >= b.halfOpenMaxCalls {
			// the recovery test is saturated: refuse without a rejection, the
			// circuit is probing rather than shut
			return false
		}
		b.halfOpenInFlight++
		return true
	default:
		return true
	}
}

// Success records a successful call. When enough consecutive half-open probes
// succeed the circuit closes.
func (b *Breaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.successLocked()
}

func (b *Breaker) successLocked() {
	b.counts.Successes++
	b.counts.ConsecutiveFailures = 0
	if b.state != StateHalfOpen {
		return
	}
	if b.halfOpenInFlight > 0 {
		b.halfOpenInFlight--
	}
	b.counts.ConsecutiveSuccesses++
	if b.counts.ConsecutiveSuccesses >= b.successThreshold {
		b.state = StateClosed
		b.generation++
		b.halfOpenInFlight = 0
	}
}

// Failure records a failed call, opening the circuit once the failure
// threshold is reached (and re-opening it from half-open immediately).
func (b *Breaker) Failure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failureLocked()
}

func (b *Breaker) failureLocked() {
	b.counts.Failures++
	b.counts.ConsecutiveFailures++
	b.counts.ConsecutiveSuccesses = 0
	switch b.state {
	case StateHalfOpen:
		// a failed probe is conclusive: back to open right away
		b.open()
	case StateOpen:
		// already open: keep the running cooldown instead of extending it
	default:
		if b.counts.ConsecutiveFailures >= b.failureThreshold {
			b.open()
		}
	}
}

// Execute calls Allow, runs fn when allowed and records the outcome. It
// returns ErrOpen without calling fn while the circuit is open. A panic in fn
// is recorded as a failure and propagates to the caller. Outcomes from calls
// admitted before a state transition or Reset do not affect the new generation.
func (b *Breaker) Execute(fn func() error) (err error) {
	allowed, generation := b.admit()
	if !allowed {
		return ErrOpen
	}
	completed := false
	defer func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if generation != b.generation {
			return
		}
		if completed && err == nil {
			b.successLocked()
		} else {
			b.failureLocked()
		}
	}()
	err = fn()
	completed = true
	return err
}

// admit snapshots the generation after admission, including any transition to
// half-open. The deferred unlock also covers a panic from the injected clock.
func (b *Breaker) admit() (bool, uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	allowed := b.allowLocked()
	return allowed, b.generation
}

// State returns the current state, transitioning open -> half-open once the
// open timeout has elapsed.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.enterHalfOpen()
	return b.state
}

// Counts returns a snapshot of the counters.
func (b *Breaker) Counts() Counts {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.counts
}

// Reset returns the breaker to a closed state with cleared counters.
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = StateClosed
	b.generation++
	b.openedAt = time.Time{}
	b.halfOpenInFlight = 0
	b.counts = Counts{}
}

// enterHalfOpen admits probes once an open circuit has served its cooldown.
// The caller must hold the mutex.
func (b *Breaker) enterHalfOpen() {
	if b.state != StateOpen || b.now().Sub(b.openedAt) < b.openTimeout {
		return
	}
	b.state = StateHalfOpen
	b.generation++
	b.halfOpenInFlight = 0
	b.counts.ConsecutiveSuccesses = 0
}

// open starts a fresh cooldown window. The caller must hold the mutex.
func (b *Breaker) open() {
	b.state = StateOpen
	b.generation++
	b.openedAt = b.now()
	b.halfOpenInFlight = 0
	b.counts.ConsecutiveSuccesses = 0
}
