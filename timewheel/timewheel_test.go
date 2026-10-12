package timewheel_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/timewheel"
)

// clock is a manually advanced time source, which keeps the wheel deterministic
// instead of racing the real clock.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock() *clock {
	return &clock{now: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// wheel builds a deterministic wheel with a 10ms tick and 8 slots per level.
func wheel(c *clock) *timewheel.Wheel {
	return timewheel.New(10*time.Millisecond, 8, timewheel.WithClock(c.Now))
}

func TestFiresOnceAtItsDeadline(t *testing.T) {
	c := newClock()
	w := wheel(c)

	var fired atomic.Int64
	timer := w.Schedule(30*time.Millisecond, func() { fired.Add(1) })
	require.Equal(t, 1, w.Len())
	require.False(t, timer.Done())

	require.Zero(t, w.AdvanceBy(29*time.Millisecond), "a callback must never run early")
	require.Zero(t, fired.Load())
	require.Equal(t, 1, w.Len())

	require.Equal(t, 1, w.AdvanceBy(time.Millisecond), "the deadline tick must fire the timer")
	require.Equal(t, int64(1), fired.Load())
	require.Zero(t, w.Len(), "a fired timer is no longer pending")
	require.True(t, timer.Done())

	require.Zero(t, w.AdvanceBy(time.Second), "a timer fires exactly once")
	require.Equal(t, int64(1), fired.Load())
}

func TestFiresInDeadlineOrderWithinOneAdvance(t *testing.T) {
	c := newClock()
	w := wheel(c)

	var mu sync.Mutex
	var order []string
	for _, item := range []struct {
		name  string
		ticks time.Duration
	}{
		{"third", 3},
		{"first", 1},
		{"second", 2},
	} {
		w.Schedule(item.ticks*10*time.Millisecond, func() {
			mu.Lock()
			defer mu.Unlock()
			order = append(order, item.name)
		})
	}

	require.Equal(t, 3, w.AdvanceBy(30*time.Millisecond), "one Advance runs every timer that came due")
	require.Equal(t, []string{"first", "second", "third"}, order)
}

func TestNonPositiveDelayFiresOnTheNextAdvance(t *testing.T) {
	c := newClock()
	w := wheel(c)

	var fired atomic.Int64
	w.Schedule(0, func() { fired.Add(1) })
	w.Schedule(-time.Hour, func() { fired.Add(1) })
	w.ScheduleAt(c.Now().Add(-time.Minute), func() { fired.Add(1) })
	require.Equal(t, 3, w.Len())

	// no time passes at all, and the three timers still fire
	require.Equal(t, 3, w.Advance(c.Now()))
	require.Equal(t, int64(3), fired.Load())
	require.Zero(t, w.Len())
}

func TestDelayAcrossLevelsFiresAtTheRightTime(t *testing.T) {
	c := newClock()
	// 10ms tick and 8 slots per level means level 3 buckets are 5.12s wide, so a
	// 30s delay has to cascade down through the hierarchy
	w := timewheel.New(10*time.Millisecond, 8, timewheel.WithClock(c.Now))

	var fired atomic.Int64
	w.Schedule(30*time.Second, func() { fired.Add(1) })

	require.Zero(t, w.AdvanceBy(30*time.Second-time.Millisecond), "the coarse bucket must not fire early")
	require.Equal(t, 1, w.AdvanceBy(time.Millisecond), "cascading must land on the deadline tick")
	require.Equal(t, int64(1), fired.Load())
	require.Zero(t, w.AdvanceBy(time.Hour))
}

func TestDelayBeyondTheTopLevelStillFiresOnTime(t *testing.T) {
	c := newClock()
	// the wheel spans tick*wheelSize^levels = 10ms*8^5 = 5.46 minutes, so an hour
	// has to be re-bucketed every rotation until it is due
	w := timewheel.New(10*time.Millisecond, 8, timewheel.WithClock(c.Now))

	var fired atomic.Int64
	w.Schedule(time.Hour, func() { fired.Add(1) })

	require.Zero(t, w.AdvanceBy(time.Hour-time.Millisecond))
	require.Zero(t, fired.Load(), "a delay past the top level must still not fire early")

	require.Equal(t, 1, w.AdvanceBy(time.Millisecond))
	require.Equal(t, int64(1), fired.Load())
}

func TestCancelBeforeAndAfterFiring(t *testing.T) {
	c := newClock()
	w := wheel(c)

	var fired atomic.Int64
	early := w.Schedule(20*time.Millisecond, func() { fired.Add(1) })
	require.True(t, early.Cancel())
	require.False(t, early.Cancel(), "a cancelled timer cannot be cancelled twice")
	require.True(t, early.Done())
	require.Zero(t, w.Len(), "Cancel must drop the timer from the pending count")

	require.Zero(t, w.AdvanceBy(50*time.Millisecond), "a cancelled callback must never run")
	require.Zero(t, fired.Load())

	late := w.Schedule(10*time.Millisecond, func() { fired.Add(1) })
	require.Equal(t, 1, w.AdvanceBy(10*time.Millisecond))
	require.False(t, late.Cancel(), "Cancel after firing reports false")
	require.Equal(t, int64(1), fired.Load())
}

func TestLenAndStop(t *testing.T) {
	c := newClock()
	w := wheel(c)

	var fired atomic.Int64
	timers := make([]*timewheel.Timer, 0, 3)
	for range 3 {
		timers = append(timers, w.Schedule(50*time.Millisecond, func() { fired.Add(1) }))
	}
	require.Equal(t, 3, w.Len())

	w.Stop()
	require.Zero(t, w.Len())
	require.Zero(t, w.AdvanceBy(time.Second), "Stop must cancel every pending timer")
	require.Zero(t, fired.Load())
	for _, timer := range timers {
		require.True(t, timer.Done())
		require.False(t, timer.Cancel())
	}
}

func TestAdvanceBackwardsIsANoOp(t *testing.T) {
	c := newClock()
	w := wheel(c)

	var fired atomic.Int64
	w.Schedule(10*time.Millisecond, func() { fired.Add(1) })

	// move the wheel forward, then ask it to go back before both the schedule and
	// the previous advance
	require.Equal(t, 1, w.AdvanceBy(10*time.Millisecond))
	require.Zero(t, w.Advance(c.Now().Add(-time.Hour)), "a backwards Advance must change nothing")

	w.Schedule(10*time.Millisecond, func() { fired.Add(1) })
	require.Zero(t, w.Advance(c.Now().Add(-time.Hour)))
	require.Equal(t, 1, w.Len())
	require.Equal(t, 1, w.AdvanceBy(10*time.Millisecond))
	require.Equal(t, int64(2), fired.Load())
}

func TestTickAdvancesOneTick(t *testing.T) {
	c := newClock()
	w := wheel(c)

	var fired atomic.Int64
	w.Schedule(2*10*time.Millisecond, func() { fired.Add(1) })

	require.Zero(t, w.Tick())
	require.Equal(t, 1, w.Tick(), "the second tick reaches the deadline")
	require.Equal(t, int64(1), fired.Load())
}

func TestManyTimersAllFireExactlyOnce(t *testing.T) {
	c := newClock()
	w := wheel(c)

	const timers = 10_000
	counts := make([]atomic.Int64, timers)
	for i := range timers {
		// spread the delays over three levels of the hierarchy
		delay := time.Duration(1+i%97) * 10 * time.Millisecond
		idx := i
		w.Schedule(delay, func() { counts[idx].Add(1) })
	}
	require.Equal(t, timers, w.Len())

	require.Equal(t, timers, w.AdvanceBy(200*10*time.Millisecond))
	require.Zero(t, w.Len())

	for i := range timers {
		require.Equal(t, int64(1), counts[i].Load(), "timer %d must run exactly once", i)
	}
}

func TestCallbackMaySchedule(t *testing.T) {
	c := newClock()
	w := wheel(c)

	var fired atomic.Int64
	w.Schedule(10*time.Millisecond, func() {
		fired.Add(1)
		// a callback runs outside the wheel lock, so this must not deadlock
		w.Schedule(20*time.Millisecond, func() { fired.Add(1) })
	})

	require.Equal(t, 1, w.AdvanceBy(10*time.Millisecond))
	require.Equal(t, 1, w.Len(), "the callback's timer must be registered")
	require.Equal(t, 1, w.AdvanceBy(20*time.Millisecond))
	require.Equal(t, int64(2), fired.Load())
}

func TestRunDrivesWithARealTicker(t *testing.T) {
	w := timewheel.New(5*time.Millisecond, 8) // the system clock

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	fired := make(chan struct{}, 1)
	w.Schedule(20*time.Millisecond, func() { fired <- struct{}{} })

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not fire a 20ms timer")
	}
}

func TestPanicsOnBadArguments(t *testing.T) {
	require.Panics(t, func() { timewheel.New(0, 8) })
	require.Panics(t, func() { timewheel.New(-time.Millisecond, 8) })
	require.Panics(t, func() { timewheel.New(time.Millisecond, 1) })
	require.Panics(t, func() { timewheel.New(time.Millisecond, 0) })
	require.Panics(t, func() { timewheel.New(time.Millisecond, 8, timewheel.WithLevels(0)) })
	require.Panics(t, func() { timewheel.New(time.Millisecond, 8, timewheel.WithClock(nil)) })
	require.Panics(t, func() { timewheel.New(time.Millisecond, 8, nil) })

	w := timewheel.New(time.Millisecond, 8)
	require.Panics(t, func() { w.Schedule(time.Second, nil) })
	require.Panics(t, func() { w.ScheduleAt(time.Now(), nil) })
	require.Panics(t, func() { w.AdvanceBy(-time.Nanosecond) })
}
