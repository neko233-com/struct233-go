package breaker_test

import (
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/breaker"
	"github.com/stretchr/testify/require"
)

func TestPanickingHalfOpenProbeReopensWithoutLeakingSlot(t *testing.T) {
	clock := newFakeClock()
	b := breaker.New(breaker.WithFailureThreshold(1), breaker.WithOpenTimeout(time.Second),
		breaker.WithClock(clock.Now))
	require.ErrorIs(t, b.Execute(func() error { return errBoom }), errBoom)
	clock.Advance(time.Second)
	require.PanicsWithValue(t, "probe panic", func() {
		_ = b.Execute(func() error { panic("probe panic") })
	})
	require.Equal(t, breaker.StateOpen, b.State())
	require.Equal(t, int64(2), b.Counts().Failures)
	clock.Advance(time.Second)
	require.NoError(t, b.Execute(func() error { return nil }))
}

func TestOldExecuteSuccessCannotCompleteCurrentHalfOpenProbe(t *testing.T) {
	clock := newFakeClock()
	b := breaker.New(breaker.WithFailureThreshold(1), breaker.WithSuccessThreshold(1),
		breaker.WithOpenTimeout(time.Second), breaker.WithClock(clock.Now))
	oldStarted := make(chan struct{})
	releaseOld := make(chan struct{})
	oldDone := make(chan error, 1)
	go func() {
		oldDone <- b.Execute(func() error {
			close(oldStarted)
			<-releaseOld
			return nil
		})
	}()
	<-oldStarted
	require.ErrorIs(t, b.Execute(func() error { return errBoom }), errBoom)
	clock.Advance(time.Second)
	probeStarted := make(chan struct{})
	releaseProbe := make(chan struct{})
	probeDone := make(chan error, 1)
	go func() {
		probeDone <- b.Execute(func() error {
			close(probeStarted)
			<-releaseProbe
			return errBoom
		})
	}()
	<-probeStarted
	close(releaseOld)
	require.NoError(t, <-oldDone)
	// Release the probe even when an assertion below fails.
	defer func() { close(releaseProbe); <-probeDone }()
	require.Equal(t, breaker.StateHalfOpen, b.State())
	require.False(t, b.Allow(), "old success must not free the current probe slot")
	require.Zero(t, b.Counts().ConsecutiveSuccesses)
}

func TestResetDiscardsInFlightExecuteOutcome(t *testing.T) {
	b := breaker.New(breaker.WithFailureThreshold(1))
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- b.Execute(func() error {
			close(started)
			<-release
			return errBoom
		})
	}()
	<-started
	b.Reset()
	close(release)
	require.ErrorIs(t, <-done, errBoom, "the caller still receives its own outcome")
	require.Equal(t, breaker.StateClosed, b.State())
	require.Equal(t, breaker.Counts{}, b.Counts())
}
