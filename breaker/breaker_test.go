package breaker_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/breaker"
)

// errBoom is the sentinel failure returned by test calls.
var errBoom = errors.New("boom")

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

// Advance moves the clock forward, which is the only way time passes here.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func TestClosedByDefaultAdmitsEveryCall(t *testing.T) {
	b := breaker.New()
	require.Equal(t, breaker.StateClosed, b.State(), "a fresh breaker must be closed")

	for i := range 10 {
		require.True(t, b.Allow(), "call #%d must be admitted while closed", i+1)
	}
	require.Equal(t, breaker.Counts{}, b.Counts(), "Allow alone must not move the counters")
}

func TestStateNames(t *testing.T) {
	require.Equal(t, "closed", breaker.StateClosed.String())
	require.Equal(t, "open", breaker.StateOpen.String())
	require.Equal(t, "half-open", breaker.StateHalfOpen.String())
	require.Equal(t, "State(7)", breaker.State(7).String(),
		"an out-of-range state must not report a known name")
}

func TestOpensAfterConsecutiveFailures(t *testing.T) {
	b := breaker.New(breaker.WithFailureThreshold(3))

	for i := range 2 {
		b.Failure()
		require.Equal(t, breaker.StateClosed, b.State(), "failure #%d is below the threshold", i+1)
		require.True(t, b.Allow(), "failure #%d is below the threshold", i+1)
	}

	b.Failure()
	require.Equal(t, breaker.StateOpen, b.State(), "the 3rd consecutive failure must open the circuit")
	require.Equal(t, 3, b.Counts().ConsecutiveFailures)

	require.False(t, b.Allow(), "an open circuit must refuse calls")
	require.False(t, b.Allow(), "an open circuit must keep refusing calls")
	require.EqualValues(t, 2, b.Counts().Rejections, "every refusal while open is counted")
}

func TestSuccessResetsConsecutiveFailures(t *testing.T) {
	b := breaker.New(breaker.WithFailureThreshold(3))

	b.Failure()
	b.Failure()
	require.Equal(t, 2, b.Counts().ConsecutiveFailures)

	b.Success()
	require.Zero(t, b.Counts().ConsecutiveFailures, "a success must clear the failure streak")

	b.Failure()
	b.Failure()
	require.Equal(t, breaker.StateClosed, b.State(),
		"the streak restarted, so 2 failures stay below the threshold")

	b.Failure()
	require.Equal(t, breaker.StateOpen, b.State(),
		"3 failures in a row after the success must open the circuit")
}

func TestOpenBecomesHalfOpenAfterTimeout(t *testing.T) {
	clock := newFakeClock()
	b := breaker.New(
		breaker.WithFailureThreshold(1),
		breaker.WithOpenTimeout(30*time.Second),
		breaker.WithClock(clock.Now),
	)

	b.Failure()
	require.Equal(t, breaker.StateOpen, b.State())

	clock.Advance(30*time.Second - time.Nanosecond)
	require.Equal(t, breaker.StateOpen, b.State(), "one nanosecond early the circuit is still open")
	require.False(t, b.Allow(), "no probe is admitted before the cooldown elapses")

	clock.Advance(time.Nanosecond)
	require.Equal(t, breaker.StateHalfOpen, b.State(),
		"now()-openedAt >= OpenTimeout must start probing")
	require.True(t, b.Allow(), "half-open must admit a probe")
}

func TestHalfOpenFailureReopensWithFreshTimeout(t *testing.T) {
	clock := newFakeClock()
	b := breaker.New(
		breaker.WithFailureThreshold(1),
		breaker.WithOpenTimeout(10*time.Second),
		breaker.WithClock(clock.Now),
	)

	b.Failure()
	clock.Advance(10 * time.Second)
	require.True(t, b.Allow(), "the cooldown elapsed, so a probe is admitted")

	b.Failure()
	require.Equal(t, breaker.StateOpen, b.State(), "a failed probe must re-open the circuit")

	clock.Advance(9 * time.Second)
	require.Equal(t, breaker.StateOpen, b.State(), "the cooldown restarted at the failed probe")
	require.False(t, b.Allow())

	clock.Advance(time.Second)
	require.Equal(t, breaker.StateHalfOpen, b.State(), "the fresh cooldown must elapse on its own")
}

func TestHalfOpenSuccessesCloseTheCircuit(t *testing.T) {
	clock := newFakeClock()
	b := breaker.New(
		breaker.WithFailureThreshold(2),
		breaker.WithSuccessThreshold(2),
		breaker.WithOpenTimeout(time.Minute),
		breaker.WithClock(clock.Now),
	)

	b.Failure()
	b.Failure()
	require.Equal(t, breaker.StateOpen, b.State())

	clock.Advance(time.Minute)
	require.True(t, b.Allow())
	require.Equal(t, breaker.StateHalfOpen, b.State())

	b.Success()
	require.Equal(t, breaker.StateHalfOpen, b.State(), "one probe is below SuccessThreshold")
	require.Zero(t, b.Counts().ConsecutiveFailures, "a probe success clears the failure streak")

	require.True(t, b.Allow())
	b.Success()
	require.Equal(t, breaker.StateClosed, b.State(), "SuccessThreshold probes must close the circuit")
	require.Zero(t, b.Counts().ConsecutiveFailures, "closing must clear the failure streak")
	require.Equal(t, 2, b.Counts().ConsecutiveSuccesses,
		"the half-open streak survives the close and is reset on the next half-open entry")

	for i := range 6 {
		require.True(t, b.Allow(), "call #%d must be admitted again", i+1)
	}

	// the failure streak was cleared, so the circuit needs a full threshold again
	b.Failure()
	require.Equal(t, breaker.StateClosed, b.State(),
		"a single failure right after recovery must not re-open the circuit")
}

func TestHalfOpenAdmitsLimitedProbes(t *testing.T) {
	clock := newFakeClock()
	b := breaker.New(
		breaker.WithFailureThreshold(1),
		breaker.WithSuccessThreshold(10), // stay half-open while the probes come in
		breaker.WithHalfOpenMaxCalls(2),
		breaker.WithOpenTimeout(time.Minute),
		breaker.WithClock(clock.Now),
	)

	b.Failure()
	clock.Advance(time.Minute)

	require.True(t, b.Allow(), "probe 1 must be admitted")
	require.True(t, b.Allow(), "probe 2 must be admitted")
	require.False(t, b.Allow(), "a 3rd concurrent probe exceeds HalfOpenMaxCalls")
	require.EqualValues(t, 0, b.Counts().Rejections,
		"a saturated half-open circuit is probing, not refusing")
	require.EqualValues(t, 0, b.Counts().Successes)

	b.Success() // probe 1 reported
	require.Equal(t, breaker.StateHalfOpen, b.State())
	require.True(t, b.Allow(), "a reported probe must release its slot")
	require.False(t, b.Allow(), "probe 2 is still in flight, so the circuit stays saturated")

	b.Success() // probe 2 reported
	require.Equal(t, breaker.StateHalfOpen, b.State())
	require.True(t, b.Allow(), "one slot is free again, so another probe is admitted")
	require.False(t, b.Allow(), "the circuit is saturated again by that probe")
	require.EqualValues(t, 2, b.Counts().Successes)
	require.EqualValues(t, 0, b.Counts().ConsecutiveFailures)
}

func TestExecuteReturnsErrOpenWithoutCallingFn(t *testing.T) {
	clock := newFakeClock()
	b := breaker.New(
		breaker.WithFailureThreshold(1),
		breaker.WithOpenTimeout(time.Hour),
		breaker.WithClock(clock.Now),
	)
	b.Failure()

	called := false
	err := b.Execute(func() error {
		called = true
		return nil
	})

	require.ErrorIs(t, err, breaker.ErrOpen, "an open circuit must report ErrOpen")
	require.False(t, called, "fn must not run while the circuit is open")
	require.EqualValues(t, 1, b.Counts().Rejections, "the refused call is counted")
}

func TestExecuteRecordsOutcomeAndReturnsFnError(t *testing.T) {
	b := breaker.New()

	require.NoError(t, b.Execute(func() error { return nil }))
	require.EqualValues(t, 1, b.Counts().Successes)

	err := b.Execute(func() error { return errBoom })
	require.Equal(t, errBoom, err, "the error from fn must be returned unchanged")
	require.EqualValues(t, 1, b.Counts().Failures, "a failing fn must be recorded as a failure")
	require.EqualValues(t, 1, b.Counts().ConsecutiveFailures)
}

func TestExecuteFailuresOpenTheCircuit(t *testing.T) {
	b := breaker.New(breaker.WithFailureThreshold(2))

	require.Equal(t, errBoom, b.Execute(func() error { return errBoom }))
	require.Equal(t, breaker.StateClosed, b.State())
	require.Equal(t, errBoom, b.Execute(func() error { return errBoom }))
	require.Equal(t, breaker.StateOpen, b.State(), "Execute failures must trip the threshold")

	calls := 0
	require.ErrorIs(t, b.Execute(func() error { calls++; return nil }), breaker.ErrOpen)
	require.Zero(t, calls, "fn must not run while the circuit is open")
}

func TestCountsSnapshotMatchesScriptedSequence(t *testing.T) {
	clock := newFakeClock()
	b := breaker.New(
		breaker.WithFailureThreshold(2),
		breaker.WithSuccessThreshold(2),
		breaker.WithOpenTimeout(10*time.Second),
		breaker.WithClock(clock.Now),
	)

	b.Success()
	b.Failure()
	b.Success()
	b.Failure()
	b.Failure() // threshold reached
	require.Equal(t, breaker.StateOpen, b.State())
	require.Equal(t, breaker.Counts{
		Successes:           2,
		Failures:            3,
		ConsecutiveFailures: 2,
	}, b.Counts())

	require.False(t, b.Allow())
	require.False(t, b.Allow())

	clock.Advance(10 * time.Second)
	require.True(t, b.Allow())
	require.Equal(t, breaker.StateHalfOpen, b.State())
	b.Success()
	require.True(t, b.Allow())
	b.Success() // closes the circuit
	require.Equal(t, breaker.StateClosed, b.State())
	require.Equal(t, breaker.Counts{
		Successes:            4,
		Failures:             3,
		Rejections:           2,
		ConsecutiveSuccesses: 2,
	}, b.Counts())
}

func TestResetClosesCircuitAndClearsCounters(t *testing.T) {
	clock := newFakeClock()
	b := breaker.New(
		breaker.WithFailureThreshold(1),
		breaker.WithOpenTimeout(time.Minute),
		breaker.WithClock(clock.Now),
	)

	b.Failure()
	require.False(t, b.Allow())
	require.NotEqual(t, breaker.Counts{}, b.Counts())

	b.Reset()
	require.Equal(t, breaker.StateClosed, b.State())
	require.Equal(t, breaker.Counts{}, b.Counts(), "Reset must clear every counter")
	require.True(t, b.Allow(), "Reset must admit calls again")

	// the configured thresholds must survive a Reset
	b.Failure()
	require.Equal(t, breaker.StateOpen, b.State(), "Reset must keep the configured threshold")
}

func TestPanicsOnInvalidOptions(t *testing.T) {
	require.PanicsWithValue(t, "breaker: failure threshold must be at least 1",
		func() { breaker.New(breaker.WithFailureThreshold(0)) })
	require.PanicsWithValue(t, "breaker: failure threshold must be at least 1",
		func() { breaker.New(breaker.WithFailureThreshold(-3)) })
	require.PanicsWithValue(t, "breaker: success threshold must be at least 1",
		func() { breaker.New(breaker.WithSuccessThreshold(0)) })
	require.PanicsWithValue(t, "breaker: open timeout must be positive",
		func() { breaker.New(breaker.WithOpenTimeout(0)) })
	require.PanicsWithValue(t, "breaker: open timeout must be positive",
		func() { breaker.New(breaker.WithOpenTimeout(-time.Second)) })
	require.PanicsWithValue(t, "breaker: half-open max calls must be at least 1",
		func() { breaker.New(breaker.WithHalfOpenMaxCalls(0)) })
	require.PanicsWithValue(t, "breaker: nil clock",
		func() { breaker.New(breaker.WithClock(nil)) })
	require.PanicsWithValue(t, "breaker: nil option",
		func() { breaker.New(nil) })
}

func TestConcurrentCallsStayConsistent(t *testing.T) {
	const (
		workers    = 8
		iterations = 500
	)

	// -race cannot run on this box (no C toolchain, CGO_ENABLED=0); CI runs it
	// on ubuntu. The accounting assertions below still catch a lost update.
	clock := newFakeClock()
	b := breaker.New(
		breaker.WithFailureThreshold(4),
		breaker.WithSuccessThreshold(2),
		breaker.WithHalfOpenMaxCalls(4),
		breaker.WithOpenTimeout(time.Millisecond),
		breaker.WithClock(clock.Now),
	)

	var admitted, refused atomic.Int64
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range iterations {
				if i%100 == 0 {
					// let the cooldown elapse so the circuit cycles through
					// open -> half-open -> closed under contention
					clock.Advance(time.Millisecond)
				}
				if !b.Allow() {
					refused.Add(1)
					continue
				}
				admitted.Add(1)
				if (w+i)%3 == 0 {
					b.Failure()
					continue
				}
				b.Success()
			}
		}()
	}
	wg.Wait()

	counts := b.Counts()
	require.Equal(t, admitted.Load(), counts.Successes+counts.Failures,
		"every admitted call must record exactly one outcome")
	require.LessOrEqual(t, counts.Rejections, refused.Load(),
		"only refusals may be counted as rejections")
	require.Positive(t, counts.Successes)
	require.Positive(t, counts.Failures)
	require.Contains(t, []string{"closed", "open", "half-open"}, b.State().String())
}
