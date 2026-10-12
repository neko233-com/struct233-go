// Package retry runs an operation again until it succeeds, the attempt budget
// runs out, the caller rejects the error or the context ends.
//
// A Policy is built once and reused. The wait between attempts grows
// exponentially, is capped and can be jittered, so a fleet of callers does not
// retry in lockstep. The sleep function is injectable, which keeps tests
// deterministic.
//
//	p := retry.New(
//		retry.WithMaxAttempts(5),
//		retry.WithBaseDelay(100*time.Millisecond),
//	)
//	err := p.Do(ctx, func(ctx context.Context) error {
//		return callFlakyService(ctx)
//	})
//
// The package-level helpers build a throwaway policy per call:
//
//	err := retry.Do(ctx, callFlakyService, retry.WithMaxAttempts(5))
//	n, err := retry.DoValue(ctx, fetchCount)
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"time"
)

// Defaults of a Policy built without the matching option.
const (
	defaultMaxAttempts = 3
	defaultBaseDelay   = 50 * time.Millisecond
	defaultMaxDelay    = 5 * time.Second
	defaultMultiplier  = 2
	defaultJitter      = 0.2
)

// Option configures a Policy.
type Option func(*Policy)

// WithMaxAttempts sets the total number of attempts, including the first one.
// Default 3. Values < 1 panic when applied.
func WithMaxAttempts(n int) Option {
	return func(p *Policy) {
		if n < 1 {
			panic("retry: max attempts must be at least 1")
		}
		p.maxAttempts = n
	}
}

// WithBaseDelay sets the delay before the second attempt. Default 50ms.
// Negative values panic when applied.
func WithBaseDelay(d time.Duration) Option {
	return func(p *Policy) {
		if d < 0 {
			panic("retry: base delay must not be negative")
		}
		p.baseDelay = d
	}
}

// WithMaxDelay caps the backoff delay. Default 5s. Values <= 0 panic.
func WithMaxDelay(d time.Duration) Option {
	return func(p *Policy) {
		if d <= 0 {
			panic("retry: max delay must be positive")
		}
		p.maxDelay = d
	}
}

// WithMultiplier sets the exponential growth factor. Default 2. Values < 1 panic.
func WithMultiplier(f float64) Option {
	return func(p *Policy) {
		if f < 1 {
			panic("retry: multiplier must be at least 1")
		}
		p.multiplier = f
	}
}

// WithJitter sets how much the delay is randomised: the delay is multiplied by
// a random factor in [1-j, 1+j]. Default 0.2; 0 disables jitter, j outside
// [0, 1) panics. The jitter is applied after the WithMaxDelay cap, so a jittered
// delay can reach MaxDelay*(1+j).
func WithJitter(j float64) Option {
	return func(p *Policy) {
		if j < 0 || j >= 1 {
			panic("retry: jitter must be in [0, 1)")
		}
		p.jitter = j
	}
}

// WithSleep replaces the real backoff wait, which keeps tests deterministic.
// fn is called as fn(ctx, delay) and must return ctx.Err() when it gives up
// early. A nil fn panics when applied.
func WithSleep(fn func(ctx context.Context, d time.Duration) error) Option {
	return func(p *Policy) {
		if fn == nil {
			panic("retry: nil sleep function")
		}
		p.sleep = fn
	}
}

// RetryIf sets the predicate that decides whether an error is worth retrying.
// The default retries every non-nil error. A nil predicate panics.
func RetryIf(pred func(err error) bool) Option {
	return func(p *Policy) {
		if pred == nil {
			panic("retry: nil retry predicate")
		}
		p.retryIf = pred
	}
}

// WithOnRetry registers an observer called before each backoff wait with the
// attempt that just failed (1-based), its error and the upcoming delay. A nil
// hook panics when applied.
func WithOnRetry(fn func(attempt int, err error, delay time.Duration)) Option {
	return func(p *Policy) {
		if fn == nil {
			panic("retry: nil on-retry hook")
		}
		p.onRetry = fn
	}
}

// Policy is a reusable retry configuration. It is immutable after New and safe
// for concurrent use.
type Policy struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
	multiplier  float64
	jitter      float64
	sleep       func(ctx context.Context, d time.Duration) error
	retryIf     func(err error) bool
	onRetry     func(attempt int, err error, delay time.Duration)
}

// New builds a policy from the given options. It panics on invalid options.
func New(opts ...Option) *Policy {
	p := &Policy{
		maxAttempts: defaultMaxAttempts,
		baseDelay:   defaultBaseDelay,
		maxDelay:    defaultMaxDelay,
		multiplier:  defaultMultiplier,
		jitter:      defaultJitter,
		sleep:       sleepContext,
	}
	for _, opt := range opts {
		if opt == nil {
			panic("retry: nil option")
		}
		opt(p)
	}
	return p
}

// Do runs fn until it succeeds, the attempt budget is exhausted, RetryIf
// rejects the error, or ctx is done. It returns nil on success and the error
// from the final attempt otherwise; errors are returned unwrapped so errors.Is
// keeps working, except when the context ends during a backoff wait, where it
// returns errors.Join(lastErr, ctx.Err()).
func (p *Policy) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	// an attempt must not start for a caller that is already done
	if err := ctx.Err(); err != nil {
		return err
	}
	var lastErr error
	for attempt := 1; attempt <= p.maxAttempts; attempt++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}
		lastErr = err
		// a nil predicate retries every error, which is the default
		if p.retryIf != nil && !p.retryIf(err) {
			return err
		}
		if attempt == p.maxAttempts {
			break // the budget is spent, so no backoff is due
		}
		delay := p.backoff(attempt)
		if p.onRetry != nil {
			p.onRetry(attempt, err, delay)
		}
		if err := p.sleep(ctx, delay); err != nil {
			// the wait was cut short, so both facts are reported
			return errors.Join(lastErr, err)
		}
	}
	return lastErr
}

// DoValue is Do for a function that returns a value: the value of the
// successful attempt is returned.
func DoValue[T any](ctx context.Context, fn func(ctx context.Context) (T, error), opts ...Option) (T, error) {
	var value T
	err := New(opts...).Do(ctx, func(ctx context.Context) error {
		v, err := fn(ctx)
		if err != nil {
			return err
		}
		value = v
		return nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return value, nil
}

// Do runs fn with a policy built on the fly from opts.
func Do(ctx context.Context, fn func(ctx context.Context) error, opts ...Option) error {
	return New(opts...).Do(ctx, fn)
}

// backoff returns the delay that follows the given failed attempt (1-based):
// base*multiplier^(attempt-1), capped at MaxDelay and then jittered.
func (p *Policy) backoff(attempt int) time.Duration {
	d := float64(p.baseDelay) * math.Pow(p.multiplier, float64(attempt-1))
	if d > float64(p.maxDelay) {
		d = float64(p.maxDelay)
	}
	if p.jitter > 0 {
		// the global source is safe for concurrent use, which keeps a shared
		// Policy race-free
		d *= 1 + p.jitter*(2*rand.Float64()-1)
	}
	return time.Duration(d)
}

// sleepContext waits for d and reports ctx.Err() when the context ends first.
func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err() // nothing to wait for, but a done context still wins
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
		return nil
	}
}
