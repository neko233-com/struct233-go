package retry_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/retry"
)

// errTransient is the sentinel that retryable failures wrap.
var errTransient = errors.New("transient")

// errFatal is the sentinel that non-retryable failures wrap.
var errFatal = errors.New("fatal")

// alwaysFail is a retryable operation that never succeeds.
func alwaysFail(context.Context) error { return fmt.Errorf("attempt failed: %w", errTransient) }

// sleepRecorder records the delays handed to the injected sleep function
// instead of waiting, which keeps tests deterministic.
type sleepRecorder struct {
	mu     sync.Mutex
	delays []time.Duration
}

func (s *sleepRecorder) Sleep(ctx context.Context, d time.Duration) error {
	s.mu.Lock()
	s.delays = append(s.delays, d)
	s.mu.Unlock()
	return ctx.Err()
}

func (s *sleepRecorder) Delays() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Duration(nil), s.delays...)
}

func TestSucceedsOnFirstAttempt(t *testing.T) {
	rec := &sleepRecorder{}
	calls := 0

	err := retry.New(retry.WithSleep(rec.Sleep)).Do(context.Background(), func(context.Context) error {
		calls++
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 1, calls, "a successful first attempt must not be repeated")
	require.Empty(t, rec.Delays(), "a success must not be followed by a backoff")
}

func TestSucceedsAfterTransientFailures(t *testing.T) {
	const failures = 2
	rec := &sleepRecorder{}
	calls := 0

	err := retry.New(
		retry.WithMaxAttempts(5),
		retry.WithSleep(rec.Sleep),
	).Do(context.Background(), func(context.Context) error {
		calls++
		if calls <= failures {
			return fmt.Errorf("attempt %d: %w", calls, errTransient)
		}
		return nil
	})

	require.NoError(t, err, "the successful attempt's error must be nil")
	require.Equal(t, failures+1, calls, "fn must be called until it succeeds")
	require.Len(t, rec.Delays(), failures, "one backoff per failed attempt")
}

func TestExhaustsAttempts(t *testing.T) {
	const maxAttempts = 3
	rec := &sleepRecorder{}
	calls := 0

	err := retry.New(
		retry.WithMaxAttempts(maxAttempts),
		retry.WithSleep(rec.Sleep),
	).Do(context.Background(), func(context.Context) error {
		calls++
		return fmt.Errorf("attempt %d: %w", calls, errTransient)
	})

	require.Equal(t, maxAttempts, calls, "fn must be called MaxAttempts times")
	require.ErrorIs(t, err, errTransient, "the sentinel must stay in the error chain")
	// comparing against the last attempt's own error proves Do did not re-wrap it
	want := fmt.Errorf("attempt %d: %w", maxAttempts, errTransient)
	require.EqualError(t, err, want.Error(), "the final attempt's error must be returned as is")
	require.Len(t, rec.Delays(), maxAttempts-1, "no backoff is due after the last attempt")
}

func TestRetryIfRejectsErrorImmediately(t *testing.T) {
	rec := &sleepRecorder{}
	calls := 0

	err := retry.New(
		retry.WithMaxAttempts(5),
		retry.RetryIf(func(err error) bool { return !errors.Is(err, errFatal) }),
		retry.WithSleep(rec.Sleep),
	).Do(context.Background(), func(context.Context) error {
		calls++
		return fmt.Errorf("attempt %d: %w", calls, errFatal)
	})

	require.Equal(t, 1, calls, "a rejected error must stop the retries")
	require.ErrorIs(t, err, errFatal)
	want := fmt.Errorf("attempt 1: %w", errFatal)
	require.EqualError(t, err, want.Error(), "the rejected error must be returned as is")
	require.Empty(t, rec.Delays(), "a rejected error must not be waited on")
}

func TestRetryIfRetriesMatchingErrors(t *testing.T) {
	const failures = 2
	rec := &sleepRecorder{}
	calls := 0

	err := retry.New(
		retry.WithMaxAttempts(5),
		retry.RetryIf(func(err error) bool { return errors.Is(err, errTransient) }),
		retry.WithSleep(rec.Sleep),
	).Do(context.Background(), func(context.Context) error {
		calls++
		if calls <= failures {
			return fmt.Errorf("attempt %d: %w", calls, errTransient)
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, failures+1, calls, "an error the predicate accepts must be retried")
}

func TestContextAlreadyCancelledSkipsAttempts(t *testing.T) {
	rec := &sleepRecorder{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	err := retry.New(retry.WithSleep(rec.Sleep)).Do(ctx, func(context.Context) error {
		calls++
		return nil
	})

	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, calls, "a done context must not trigger a single attempt")
	require.Empty(t, rec.Delays())
}

func TestContextCancelDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// the injected wait gives up the way the real one does: it returns ctx.Err()
	sleep := func(ctx context.Context, _ time.Duration) error {
		cancel()
		return ctx.Err()
	}

	calls := 0
	err := retry.New(
		retry.WithMaxAttempts(5),
		retry.WithSleep(sleep),
	).Do(ctx, func(context.Context) error {
		calls++
		return fmt.Errorf("attempt %d: %w", calls, errTransient)
	})

	require.Equal(t, 1, calls, "a cancelled wait must stop the attempts")
	require.ErrorIs(t, err, context.Canceled, "the cancelled wait must surface")
	require.ErrorIs(t, err, errTransient, "the joined error must keep the last error")
}

func TestDefaultSleepIsCancellable(t *testing.T) {
	// no WithSleep here: the real timer must notice the cancellation instead of
	// waiting out the one hour base delay
	ctx, cancel := context.WithCancel(context.Background())

	start := time.Now()
	err := retry.New(
		retry.WithMaxAttempts(2),
		retry.WithBaseDelay(time.Hour),
	).Do(ctx, func(context.Context) error {
		cancel()
		return fmt.Errorf("attempt 1: %w", errTransient)
	})
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.Canceled, "the real sleep must return ctx.Err()")
	require.ErrorIs(t, err, errTransient, "the joined error must keep the last error")
	require.Less(t, elapsed, time.Second, "the timer must stop as soon as ctx is done")
}

func TestBackoffScheduleGrowsExponentially(t *testing.T) {
	const (
		base        = 10 * time.Millisecond
		multiplier  = 2
		maxAttempts = 5
	)
	rec := &sleepRecorder{}

	_ = retry.New(
		retry.WithBaseDelay(base),
		retry.WithMultiplier(multiplier),
		retry.WithMaxDelay(time.Hour), // high enough to never cap
		retry.WithJitter(0),
		retry.WithMaxAttempts(maxAttempts),
		retry.WithSleep(rec.Sleep),
	).Do(context.Background(), alwaysFail)

	want := make([]time.Duration, 0, maxAttempts-1)
	d := float64(base)
	for range maxAttempts - 1 {
		want = append(want, time.Duration(d))
		d *= multiplier
	}
	require.Equal(t, want, rec.Delays(), "each delay must be base*multiplier^(attempt-1)")
}

func TestMaxDelayCapsBackoff(t *testing.T) {
	rec := &sleepRecorder{}

	_ = retry.New(
		retry.WithBaseDelay(100*time.Millisecond),
		retry.WithMultiplier(2),
		retry.WithMaxDelay(250*time.Millisecond),
		retry.WithJitter(0),
		retry.WithMaxAttempts(5),
		retry.WithSleep(rec.Sleep),
	).Do(context.Background(), alwaysFail)

	require.Equal(t, []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		250 * time.Millisecond,
		250 * time.Millisecond,
	}, rec.Delays(), "delays must stop growing at MaxDelay")
}

func TestJitterStaysWithinBand(t *testing.T) {
	const jitter = 0.5
	nominal := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond}

	for run := range 64 {
		rec := &sleepRecorder{}
		_ = retry.New(
			retry.WithBaseDelay(nominal[0]),
			retry.WithMultiplier(2),
			retry.WithMaxDelay(time.Second),
			retry.WithJitter(jitter),
			retry.WithMaxAttempts(len(nominal)+1),
			retry.WithSleep(rec.Sleep),
		).Do(context.Background(), alwaysFail)

		got := rec.Delays()
		require.Len(t, got, len(nominal), "run %d", run)
		for i, d := range nominal {
			lo := time.Duration(float64(d) * (1 - jitter))
			hi := time.Duration(float64(d) * (1 + jitter))
			// the jittered value is truncated to a Duration, so allow one nanosecond
			require.GreaterOrEqual(t, got[i], lo-time.Nanosecond,
				"run %d: delay #%d must not fall below %v", run, i, lo)
			require.LessOrEqual(t, got[i], hi,
				"run %d: delay #%d must not exceed %v", run, i, hi)
		}
	}
}

func TestDefaultJitterStaysWithinBand(t *testing.T) {
	// the documented defaults: 3 attempts, 50ms base, x2 growth, 0.2 jitter
	const defaultJitter = 0.2
	nominal := []time.Duration{50 * time.Millisecond, 100 * time.Millisecond}

	for run := range 64 {
		rec := &sleepRecorder{}
		_ = retry.New(retry.WithSleep(rec.Sleep)).Do(context.Background(), alwaysFail)

		got := rec.Delays()
		require.Len(t, got, len(nominal), "the default attempt budget must be 3")
		for i, d := range nominal {
			lo := time.Duration(float64(d) * (1 - defaultJitter))
			hi := time.Duration(float64(d) * (1 + defaultJitter))
			require.GreaterOrEqual(t, got[i], lo-time.Nanosecond,
				"run %d: default delay #%d must not fall below %v", run, i, lo)
			require.LessOrEqual(t, got[i], hi,
				"run %d: default delay #%d must not exceed %v", run, i, hi)
		}
	}
}

func TestOnRetryObservesFailedAttempts(t *testing.T) {
	type observation struct {
		attempt int
		err     error
		delay   time.Duration
	}
	rec := &sleepRecorder{}
	seen := make([]observation, 0, 2)
	calls := 0

	_ = retry.New(
		retry.WithMaxAttempts(3),
		retry.WithBaseDelay(time.Second),
		retry.WithJitter(0),
		retry.WithSleep(rec.Sleep),
		retry.WithOnRetry(func(attempt int, err error, delay time.Duration) {
			seen = append(seen, observation{attempt: attempt, err: err, delay: delay})
		}),
	).Do(context.Background(), func(context.Context) error {
		calls++
		return fmt.Errorf("attempt %d: %w", calls, errTransient)
	})

	require.Len(t, seen, 2, "the observer fires before each wait, not after the last attempt")
	delays := rec.Delays()
	require.Len(t, delays, len(seen))
	for i, obs := range seen {
		require.Equal(t, i+1, obs.attempt, "attempts are reported 1-based")
		require.ErrorIs(t, obs.err, errTransient, "the failed attempt's error is reported")
		require.EqualError(t, obs.err, fmt.Sprintf("attempt %d: %s", i+1, errTransient))
		require.Equal(t, delays[i], obs.delay, "the announced delay must be the one that follows")
	}
}

func TestDoBuildsPolicyOnTheFly(t *testing.T) {
	rec := &sleepRecorder{}
	calls := 0

	err := retry.Do(context.Background(), func(context.Context) error {
		calls++
		return fmt.Errorf("attempt %d: %w", calls, errTransient)
	}, retry.WithMaxAttempts(2), retry.WithSleep(rec.Sleep))

	require.Equal(t, 2, calls, "the options must reach the policy built by Do")
	require.ErrorIs(t, err, errTransient)
	require.Len(t, rec.Delays(), 1, "exactly one backoff fits into two attempts")
}

func TestDoValueReturnsSuccessfulValue(t *testing.T) {
	rec := &sleepRecorder{}
	calls := 0

	got, err := retry.DoValue(context.Background(), func(context.Context) (int, error) {
		calls++
		if calls == 1 {
			return -1, fmt.Errorf("attempt 1: %w", errTransient)
		}
		return 42, nil
	}, retry.WithMaxAttempts(3), retry.WithSleep(rec.Sleep))

	require.NoError(t, err)
	require.Equal(t, 42, got, "the successful attempt's value must be returned")
	require.Equal(t, 2, calls)
}

func TestDoValueReturnsLastError(t *testing.T) {
	rec := &sleepRecorder{}
	calls := 0

	got, err := retry.DoValue(context.Background(), func(context.Context) (int, error) {
		calls++
		return calls, fmt.Errorf("attempt %d: %w", calls, errTransient)
	}, retry.WithMaxAttempts(3), retry.WithSleep(rec.Sleep))

	require.Equal(t, 3, calls)
	require.ErrorIs(t, err, errTransient)
	require.EqualError(t, err, fmt.Sprintf("attempt 3: %s", errTransient),
		"the last attempt's error must be returned")
	require.Zero(t, got, "a failed DoValue must not report an attempt's value")
}

func TestPolicyIsReusableConcurrently(t *testing.T) {
	const goroutines = 8
	p := retry.New(
		retry.WithMaxAttempts(3),
		retry.WithSleep(func(ctx context.Context, _ time.Duration) error { return ctx.Err() }),
	)

	calls := make([]int, goroutines)
	errs := make([]error, goroutines)
	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = p.Do(context.Background(), func(context.Context) error {
				calls[i]++
				return fmt.Errorf("attempt %d: %w", calls[i], errTransient)
			})
		}()
	}
	wg.Wait()

	for i := range goroutines {
		require.Equal(t, 3, calls[i], "goroutine %d must run its own attempts", i)
		require.ErrorIs(t, errs[i], errTransient, "goroutine %d", i)
		require.EqualError(t, errs[i], fmt.Sprintf("attempt 3: %s", errTransient), "goroutine %d", i)
	}
}

func TestAcceptsBoundaryOptions(t *testing.T) {
	require.NotPanics(t, func() {
		retry.New(
			retry.WithMaxAttempts(1),
			retry.WithBaseDelay(0),
			retry.WithMultiplier(1),
			retry.WithJitter(0),
			retry.WithMaxDelay(time.Nanosecond),
			retry.WithSleep(func(ctx context.Context, _ time.Duration) error { return ctx.Err() }),
			retry.RetryIf(func(error) bool { return true }),
			retry.WithOnRetry(func(int, error, time.Duration) {}),
		)
	}, "the documented boundaries must be accepted")
}

func TestPanicsOnInvalidOptions(t *testing.T) {
	cases := map[string]retry.Option{
		"zero max attempts":     retry.WithMaxAttempts(0),
		"negative max attempts": retry.WithMaxAttempts(-2),
		"negative base delay":   retry.WithBaseDelay(-1),
		"zero max delay":        retry.WithMaxDelay(0),
		"negative max delay":    retry.WithMaxDelay(-time.Second),
		"multiplier below one":  retry.WithMultiplier(0.5),
		"jitter of one":         retry.WithJitter(1),
		"negative jitter":       retry.WithJitter(-0.1),
		"nil sleep":             retry.WithSleep(nil),
		"nil predicate":         retry.RetryIf(nil),
		"nil on-retry hook":     retry.WithOnRetry(nil),
		"nil option":            nil,
	}
	for name, opt := range cases {
		t.Run(name, func(t *testing.T) {
			requirePanicsWithRetryPrefix(t, opt)
		})
	}
}

// requirePanicsWithRetryPrefix asserts that applying opt to New panics with a
// message carrying the package prefix.
func requirePanicsWithRetryPrefix(t *testing.T, opt retry.Option) {
	t.Helper()
	defer func() {
		recovered := recover()
		msg, ok := recovered.(string)
		require.True(t, ok, "an invalid option must panic with a string, got %v (%T)", recovered, recovered)
		require.Contains(t, msg, "retry: ", "the panic message must carry the package prefix")
	}()
	retry.New(opt)
}
