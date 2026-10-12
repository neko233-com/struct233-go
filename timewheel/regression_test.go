package timewheel_test

import (
	"math"
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/timewheel"
	"github.com/stretchr/testify/require"
)

func TestPreEpochTimersAcrossHierarchy(t *testing.T) {
	start := time.Unix(-100, 0)
	w := timewheel.New(time.Second, 8, timewheel.WithClock(func() time.Time { return start }))
	var got []int
	for _, seconds := range []int{1, 9, 65, 101} {
		w.Schedule(time.Duration(seconds)*time.Second, func() { got = append(got, seconds) })
	}
	require.Equal(t, 4, w.Advance(start.Add(110*time.Second)))
	require.Equal(t, []int{1, 9, 65, 101}, got)
}

func TestMaximumDelayDoesNotOverflowRounding(t *testing.T) {
	start := time.Unix(0, 0)
	w := timewheel.New(time.Nanosecond, 8, timewheel.WithClock(func() time.Time { return start }))
	timer := w.Schedule(math.MaxInt64, func() { t.Fatal("long delay fired early") })
	require.Zero(t, w.Advance(start.Add(time.Nanosecond)))
	require.False(t, timer.Done())
	require.True(t, timer.Cancel())
}

func TestOutOfRangeTimeRejectedWithoutPoisoningWheel(t *testing.T) {
	start := time.Unix(0, math.MaxInt64-int64(time.Second))
	w := timewheel.New(time.Second, 8, timewheel.WithClock(func() time.Time { return start }))
	require.Panics(t, func() { w.Schedule(2*time.Second, func() {}) })
	require.Zero(t, w.Len(), "rejected scheduling must not register a timer")
	require.Panics(t, func() { w.Advance(start.Add(3 * time.Second)) })
	require.Panics(t, func() { w.AdvanceBy(3 * time.Second) })
	require.Zero(t, w.Len(), "rejected advances must release the lock")
	require.Panics(t, func() {
		w.ScheduleAt(time.Unix(0, math.MaxInt64), func() {})
	}, "an unrepresentable rounded deadline must not enter a bucket")
	timer := w.Schedule(0, func() {})
	require.Equal(t, 1, w.Advance(start))
	require.True(t, timer.Done())
}

func TestAdvanceAtMaximumTickTerminates(t *testing.T) {
	start := time.Unix(0, math.MaxInt64-1)
	w := timewheel.New(time.Nanosecond, 8, timewheel.WithClock(func() time.Time { return start }))
	timer := w.Schedule(time.Nanosecond, func() {})
	require.Equal(t, 1, w.Advance(start.Add(time.Nanosecond)))
	require.True(t, timer.Done())
	require.Zero(t, w.Len())
}

func TestHierarchyOverflowRejected(t *testing.T) {
	require.Panics(t, func() { timewheel.New(time.Second, 8, timewheel.WithLevels(23)) })
}

func TestCancellingBucketEntriesPreservesRemainingFIFO(t *testing.T) {
	start := time.Unix(0, 0)
	w := timewheel.New(time.Second, 8, timewheel.WithClock(func() time.Time { return start }))
	var got []int
	var timers []*timewheel.Timer
	for i := range 6 {
		timers = append(timers, w.Schedule(time.Second, func() { got = append(got, i) }))
	}
	require.True(t, timers[1].Cancel())
	require.True(t, timers[5].Cancel())
	require.True(t, timers[0].Cancel())
	require.Equal(t, 3, w.AdvanceBy(time.Second))
	require.Equal(t, []int{2, 3, 4}, got)
}
