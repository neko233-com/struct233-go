package ratelimit_test

import (
	"math"
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/ratelimit"
	"github.com/stretchr/testify/require"
)

func TestClockRollbackDoesNotCreditSameTimeTwice(t *testing.T) {
	clock := newFakeClock()
	l := ratelimit.New(1, 1, ratelimit.WithClock(clock.Now))
	require.True(t, l.Allow())
	clock.Advance(-time.Second)
	require.Zero(t, l.Tokens())
	clock.Advance(time.Second)
	require.False(t, l.Allow(), "returning to the last observed time earns no token")
	clock.Advance(time.Second)
	require.True(t, l.Allow())
}

func TestReserveRoundsUpAndSaturates(t *testing.T) {
	for _, tc := range []struct {
		rate float64
		want time.Duration
	}{{2e9, time.Nanosecond}, {math.SmallestNonzeroFloat64, math.MaxInt64}} {
		clock := newFakeClock()
		l := ratelimit.New(tc.rate, 1, ratelimit.WithClock(clock.Now))
		require.True(t, l.Allow())
		require.Equal(t, tc.want, l.Reserve())
	}
}

func TestRejectsNonFiniteRate(t *testing.T) {
	for _, rate := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		require.Panics(t, func() { ratelimit.New(rate, 1) })
	}
}
