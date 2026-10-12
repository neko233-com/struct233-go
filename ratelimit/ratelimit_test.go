package ratelimit_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/ratelimit"
)

// fakeClock is a deterministic, race-free clock for tests.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func TestBurstIsSpentThenRefilled(t *testing.T) {
	clock := newFakeClock()
	l := ratelimit.New(2, 3, ratelimit.WithClock(clock.Now))
	require.Equal(t, 2.0, l.Rate())
	require.Equal(t, 3, l.Burst())

	for i := range 3 {
		require.True(t, l.Allow(), "allow #%d must fit in the burst", i+1)
	}
	require.False(t, l.Allow(), "the burst must be exhausted")
	require.Zero(t, l.Tokens())

	clock.Advance(500 * time.Millisecond) // 2 tokens/s * 0.5s = exactly 1 token
	require.InDelta(t, 1.0, l.Tokens(), 1e-9)
	require.True(t, l.Allow(), "the refilled token must be spendable")
	require.False(t, l.Allow(), "only one token was refilled")
}

func TestTokensAreCappedAtBurst(t *testing.T) {
	clock := newFakeClock()
	l := ratelimit.New(100, 4, ratelimit.WithClock(clock.Now))
	require.True(t, l.AllowN(4))

	clock.Advance(time.Hour) // 360000 tokens worth of idle time
	require.InDelta(t, 4.0, l.Tokens(), 1e-9, "refill must cap at Burst")

	clock.Advance(24 * time.Hour)
	require.InDelta(t, 4.0, l.Tokens(), 1e-9, "idling longer must not grow the bucket")
}

func TestTokensNeverGoNegative(t *testing.T) {
	clock := newFakeClock()
	l := ratelimit.New(1, 2, ratelimit.WithClock(clock.Now))
	require.True(t, l.AllowN(2))
	require.Zero(t, l.Tokens())

	require.False(t, l.Allow())
	require.GreaterOrEqual(t, l.Tokens(), 0.0)
	require.False(t, l.AllowN(3), "n > Burst is never satisfiable")
	require.GreaterOrEqual(t, l.Tokens(), 0.0)
}

func TestAllowNReportsUnsatisfiable(t *testing.T) {
	clock := newFakeClock()
	l := ratelimit.New(1, 5, ratelimit.WithClock(clock.Now))

	require.False(t, l.AllowN(6), "more than Burst can never be admitted")
	require.InDelta(t, 5.0, l.Tokens(), 1e-9, "a rejected request must not consume tokens")
	require.True(t, l.AllowN(5), "exactly Burst must be satisfiable")
	require.Zero(t, l.Tokens())
}

func TestAllowNNonPositiveIsFree(t *testing.T) {
	clock := newFakeClock()
	l := ratelimit.New(1, 3, ratelimit.WithClock(clock.Now))
	require.True(t, l.AllowN(3))
	require.Zero(t, l.Tokens())

	require.True(t, l.AllowN(0), "nothing to spend is always allowed")
	require.True(t, l.AllowN(-2), "negative n is treated like zero")
	require.Zero(t, l.Tokens(), "n <= 0 must not consume tokens")
}

func TestAllowNPartialAvailabilityDoesNotConsume(t *testing.T) {
	clock := newFakeClock()
	l := ratelimit.New(1, 5, ratelimit.WithClock(clock.Now))
	require.True(t, l.AllowN(3))
	require.InDelta(t, 2.0, l.Tokens(), 1e-9)

	require.False(t, l.AllowN(3), "a partially satisfiable request must be rejected")
	require.InDelta(t, 2.0, l.Tokens(), 1e-9, "the rejected request must leave the bucket untouched")
	require.True(t, l.AllowN(2), "what is available stays spendable")
}

func TestReserveReportsWaitForNextToken(t *testing.T) {
	clock := newFakeClock()
	l := ratelimit.New(2, 1, ratelimit.WithClock(clock.Now))
	require.Zero(t, l.Reserve(), "a full bucket has a token available now")

	require.True(t, l.Allow())
	// 2 tokens/s means one token per 500ms. Reserve derives the duration from a
	// float division, so compare within a tolerance instead of exactly.
	require.InDelta(t, float64(500*time.Millisecond), float64(l.Reserve()), float64(time.Millisecond))

	clock.Advance(250 * time.Millisecond)
	require.InDelta(t, float64(250*time.Millisecond), float64(l.Reserve()), float64(time.Millisecond),
		"half a token must halve the wait")
}

func TestReserveDoesNotConsume(t *testing.T) {
	clock := newFakeClock()
	l := ratelimit.New(2, 2, ratelimit.WithClock(clock.Now))
	require.True(t, l.AllowN(2))

	require.NotZero(t, l.Reserve())
	require.NotZero(t, l.Reserve(), "repeated Reserve must not spend the token")
	require.Zero(t, l.Tokens(), "Reserve must not change the token count")
}

func TestConcurrentAllowDoesNotExceedBudget(t *testing.T) {
	const (
		rate     = 1000 // tokens per second
		burst    = 100
		workers  = 8
		attempts = 2000
	)

	start := time.Now()
	l := ratelimit.New(rate, burst)

	var admitted atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range attempts {
				if l.Allow() {
					admitted.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	// every admitted event spends a token; tokens come only from the initial
	// burst plus the lazy refill of rate*elapsed, so the bound below must hold.
	// the small slack absorbs float rounding and the instant between New and
	// the first clock reading.
	got := admitted.Load()
	bound := int64(burst) + int64(rate*elapsed.Seconds()) + 10
	require.Positive(t, got, "concurrent callers must still admit some events")
	require.LessOrEqual(t, got, bound,
		"admitted %d events must stay within burst+rate*elapsed=%d", got, bound)
}

func TestWaitHonoursContext(t *testing.T) {
	clock := newFakeClock()
	l := ratelimit.New(1, 1, ratelimit.WithClock(clock.Now))
	require.True(t, l.Allow()) // empty the bucket; the frozen clock never refills

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := l.Wait(ctx)
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, elapsed, time.Second, "a done context must return without waiting for a token")
}

func TestWaitHonoursDeadline(t *testing.T) {
	clock := newFakeClock()
	l := ratelimit.New(1, 1, ratelimit.WithClock(clock.Now))
	require.True(t, l.Allow())

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := l.Wait(ctx)
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, elapsed, time.Second, "Wait must stop as soon as the context is done")
}

func TestWaitConsumesAvailableTokenWithoutSleeping(t *testing.T) {
	clock := newFakeClock()
	l := ratelimit.New(1, 1, ratelimit.WithClock(clock.Now))

	start := time.Now()
	err := l.Wait(context.Background())
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Zero(t, l.Tokens(), "Wait must consume the token it was admitted for")
	require.Less(t, elapsed, 500*time.Millisecond, "a full bucket must be admitted without sleeping")
}

func TestPanicsOnInvalidArguments(t *testing.T) {
	clock := newFakeClock()
	require.Panics(t, func() { ratelimit.New(0, 1, ratelimit.WithClock(clock.Now)) })
	require.Panics(t, func() { ratelimit.New(-1, 1, ratelimit.WithClock(clock.Now)) })
	require.Panics(t, func() { ratelimit.New(1, 0, ratelimit.WithClock(clock.Now)) })
	require.Panics(t, func() { ratelimit.New(1, -3, ratelimit.WithClock(clock.Now)) })
	require.Panics(t, func() { ratelimit.New(1, 1, ratelimit.WithClock(nil)) })
	require.Panics(t, func() { ratelimit.New(1, 1, nil) })
}

func TestSlidingWindowAdmitsLimitPerWindow(t *testing.T) {
	clock := newFakeClock()
	w := ratelimit.NewSlidingWindow(3, time.Second, ratelimit.WithClock(clock.Now))
	require.Equal(t, 3, w.Limit())
	require.Equal(t, time.Second, w.Window())

	require.True(t, w.Allow()) // t0
	clock.Advance(200 * time.Millisecond)
	require.True(t, w.Allow()) // t0+200ms
	clock.Advance(200 * time.Millisecond)
	require.True(t, w.Allow()) // t0+400ms
	require.False(t, w.Allow(), "the window is full")

	clock.Advance(599 * time.Millisecond) // t0+999ms: nothing has left the window
	require.False(t, w.Allow())
	require.Zero(t, w.Remaining())

	clock.Advance(time.Millisecond) // t0+1000ms: the first event leaves the window
	require.Equal(t, 1, w.Remaining(), "exactly one slot must have been freed")
	require.True(t, w.Allow(), "the freed slot is spendable")
	require.False(t, w.Allow(), "and the window is full again")
}

func TestSlidingWindowAllowNIsAllOrNothing(t *testing.T) {
	clock := newFakeClock()
	w := ratelimit.NewSlidingWindow(5, time.Second, ratelimit.WithClock(clock.Now))

	require.True(t, w.AllowN(3))
	require.Equal(t, 2, w.Remaining())
	require.False(t, w.AllowN(3), "a partially satisfiable batch must be rejected")
	require.Equal(t, 2, w.Remaining(), "the rejected batch must not admit anything")
	require.True(t, w.AllowN(2))
	require.Zero(t, w.Remaining())

	require.False(t, w.AllowN(6), "n > Limit is never satisfiable")
	require.True(t, w.AllowN(0), "nothing to spend is always allowed")
	require.True(t, w.AllowN(-1))
	require.Zero(t, w.Remaining(), "n <= 0 must not admit anything")
}

func TestSlidingWindowRetryAfter(t *testing.T) {
	clock := newFakeClock()
	w := ratelimit.NewSlidingWindow(1, time.Second, ratelimit.WithClock(clock.Now))
	require.Zero(t, w.RetryAfter(), "an empty window admits right now")

	require.True(t, w.Allow())
	require.Equal(t, time.Second, w.RetryAfter())

	clock.Advance(400 * time.Millisecond)
	require.Equal(t, 600*time.Millisecond, w.RetryAfter(), "RetryAfter counts down with the clock")
	require.False(t, w.Allow(), "RetryAfter must not admit anything")
	require.Equal(t, 600*time.Millisecond, w.RetryAfter())
}

func TestSlidingWindowResetStartsAFreshWindow(t *testing.T) {
	clock := newFakeClock()
	w := ratelimit.NewSlidingWindow(2, time.Minute, ratelimit.WithClock(clock.Now))
	require.True(t, w.AllowN(2))
	require.Zero(t, w.Remaining())

	w.Reset()
	require.Equal(t, 2, w.Remaining())
	require.True(t, w.AllowN(2), "the reset window must be spendable again")
}

func TestSlidingWindowWaitAdmitsWhenASlotIsFree(t *testing.T) {
	clock := newFakeClock()
	w := ratelimit.NewSlidingWindow(1, time.Minute, ratelimit.WithClock(clock.Now))

	start := time.Now()
	require.NoError(t, w.Wait(context.Background()))
	require.Less(t, time.Since(start), 100*time.Millisecond, "a free slot must not sleep")
	require.Zero(t, w.Remaining(), "Wait must admit the event it waited for")
}

func TestSlidingWindowWaitHonoursContext(t *testing.T) {
	clock := newFakeClock()
	w := ratelimit.NewSlidingWindow(1, time.Minute, ratelimit.WithClock(clock.Now))
	require.True(t, w.Allow())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := w.Wait(ctx)

	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(start), time.Second, "a done context must return without waiting")
}

func TestSlidingWindowAllowDoesNotAllocate(t *testing.T) {
	clock := newFakeClock()
	w := ratelimit.NewSlidingWindow(16, time.Second, ratelimit.WithClock(clock.Now))

	allocs := testing.AllocsPerRun(100, func() {
		clock.Advance(time.Second) // free a slot so the admit path runs
		w.Allow()
	})
	require.Zero(t, allocs, "Allow must reuse its ring buffer")
}

func TestSlidingWindowConcurrentAllowStaysWithinLimit(t *testing.T) {
	const (
		limit     = 200
		window    = 100 * time.Millisecond
		workers   = 8
		perWorker = 1000
	)

	w := ratelimit.NewSlidingWindow(limit, window)
	start := time.Now()

	var admitted atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perWorker {
				if w.Allow() {
					admitted.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	// every admitted event holds one of the limit slots for a whole window, so
	// the total cannot exceed limit per elapsed window
	windows := int64(elapsed/window) + 1
	got := admitted.Load()
	require.Positive(t, got, "concurrent callers must still admit some events")
	require.LessOrEqual(t, got, int64(limit)*windows,
		"admitted %d events must stay within limit*windows=%d", got, int64(limit)*windows)
}

func TestSlidingWindowPanicsOnInvalidArguments(t *testing.T) {
	clock := newFakeClock()
	require.Panics(t, func() { ratelimit.NewSlidingWindow(0, time.Second) })
	require.Panics(t, func() { ratelimit.NewSlidingWindow(-1, time.Second) })
	require.Panics(t, func() { ratelimit.NewSlidingWindow(1, 0) })
	require.Panics(t, func() { ratelimit.NewSlidingWindow(1, -time.Second) })
	require.Panics(t, func() { ratelimit.NewSlidingWindow(1, time.Second, nil) })
	require.Panics(t, func() { ratelimit.NewSlidingWindow(1, time.Second, ratelimit.WithClock(nil)) })
	require.NotPanics(t, func() { ratelimit.NewSlidingWindow(1, time.Second, ratelimit.WithClock(clock.Now)) })
}
