package retry_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/retry"
	"github.com/stretchr/testify/require"
)

func TestCancellationBetweenWaitAndNextAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	p := retry.New(retry.WithSleep(func(context.Context, time.Duration) error {
		cancel() // the timer can complete just as the context becomes done
		return nil
	}))
	err := p.Do(ctx, func(context.Context) error { calls++; return errTransient })
	require.Equal(t, 1, calls)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, err, errTransient)
}

func TestBackoffDurationSaturatesWithoutWrapping(t *testing.T) {
	var delay time.Duration
	p := retry.New(retry.WithMaxAttempts(2), retry.WithBaseDelay(math.MaxInt64),
		retry.WithMaxDelay(math.MaxInt64), retry.WithJitter(0),
		retry.WithSleep(func(_ context.Context, d time.Duration) error { delay = d; return nil }))
	require.ErrorIs(t, p.Do(context.Background(), alwaysFail), errTransient)
	require.Equal(t, time.Duration(math.MaxInt64), delay)
}

func TestZeroBaseDelayStaysZeroAfterExponentialOverflow(t *testing.T) {
	p := retry.New(retry.WithMaxAttempts(1100), retry.WithBaseDelay(0),
		retry.WithSleep(func(_ context.Context, d time.Duration) error {
			require.Zero(t, d)
			return nil
		}))
	require.ErrorIs(t, p.Do(context.Background(), alwaysFail), errTransient)
}

func TestRejectsNonFiniteOptions(t *testing.T) {
	for _, n := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		require.Panics(t, func() { retry.New(retry.WithMultiplier(n)) })
		require.Panics(t, func() { retry.New(retry.WithJitter(n)) })
	}
}
