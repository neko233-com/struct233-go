// Package timewheel provides a hierarchical timing wheel: a scheduler that runs
// callbacks after a delay without one timer per pending entry.
//
// Scheduling and cancelling are O(1), so the wheel scales to millions of
// pending timers; the delay is bucketed into levels, and coarse buckets cascade
// into finer ones as time passes, so a callback still runs within one tick of
// its deadline.
//
//	w := timewheel.New(10*time.Millisecond, 512)   // 10ms resolution
//	w.Schedule(5*time.Second, func() { reconnect() })
//
//	// drive it from the game loop (deterministic, injectable clock)...
//	w.Tick()
//	// ...or from a goroutine with a real ticker
//	go w.Run(ctx)
//
// Advance only fires timers whose fire tick - the first tick boundary at or
// after their deadline - has been reached, so a callback never runs before its
// deadline. Timers scheduled with a non-positive delay fire on the very next
// Advance, even when no time has passed. Callbacks run outside the wheel lock,
// so a callback may schedule or cancel timers safely.
package timewheel

import (
	"context"
	"sort"
	"sync"
	"time"
)

// defaultLevels is the hierarchy depth used when WithLevels is not given.
const defaultLevels = 5

// Timer is a scheduled callback. It is created by Schedule or ScheduleAt.
type Timer struct {
	w         *Wheel
	deadline  time.Time
	fn        func()
	seq       uint64 // insertion order, breaks deadline ties (FIFO)
	fired     bool   // guarded by w.mu
	cancelled bool   // guarded by w.mu
}

// Cancel cancels the timer, reporting whether it was still pending. It reports
// false once the timer has fired or has already been cancelled.
func (t *Timer) Cancel() bool {
	w := t.w
	w.mu.Lock()
	defer w.mu.Unlock()
	if t.fired || t.cancelled {
		return false
	}
	t.cancelled = true
	w.pending--
	return true
}

// Deadline returns the time the timer is due. The wheel rounds that up to the
// next tick boundary when it runs the callback.
func (t *Timer) Deadline() time.Time { return t.deadline }

// Done reports whether the timer has fired or been cancelled.
func (t *Timer) Done() bool {
	w := t.w
	w.mu.Lock()
	defer w.mu.Unlock()
	return t.fired || t.cancelled
}

// Option configures a Wheel.
type Option func(*Wheel)

// WithClock sets the time source used by Schedule, Tick and Run. Default:
// time.Now. Injecting a clock keeps the wheel deterministic in tests.
func WithClock(now func() time.Time) Option {
	return func(w *Wheel) { w.now = now }
}

// WithLevels sets how many hierarchy levels the wheel uses. Default 5. Values
// below 1 panic when applied.
//
// Level L buckets are wheelSize^L ticks wide, so the largest delay the wheel
// expresses in one pass is tick * wheelSize^levels; longer delays are stored in
// the top level and cascaded again each rotation until they are due, which
// costs extra work but still fires on time.
func WithLevels(levels int) Option {
	return func(w *Wheel) { w.levels = levels }
}

// Wheel is a hierarchical timing wheel. It is safe for concurrent use, but its
// time is advanced by an explicit driver: either one goroutine calling
// Advance/AdvanceBy/Tick, or Run.
type Wheel struct {
	mu        sync.Mutex
	tick      time.Duration
	wheelSize int
	levels    int
	now       func() time.Time
	buckets   [][][]*Timer // [level][slot]
	immediate []*Timer     // scheduled with a non-positive delay
	pending   int
	seq       uint64
	lastTick  int64     // tick index the wheel has been advanced to
	lastTime  time.Time // time that tick index corresponds to
}

// New creates a wheel with the given tick resolution (> 0) and number of slots
// per level (>= 2). It panics on invalid arguments or options.
func New(tick time.Duration, wheelSize int, opts ...Option) *Wheel {
	if tick <= 0 {
		panic("timewheel: tick must be positive")
	}
	if wheelSize < 2 {
		panic("timewheel: wheel size must be at least 2")
	}

	w := &Wheel{tick: tick, wheelSize: wheelSize, levels: defaultLevels, now: time.Now}
	for _, opt := range opts {
		if opt == nil {
			panic("timewheel: nil option")
		}
		opt(w)
	}
	if w.levels < 1 {
		panic("timewheel: levels must be at least 1")
	}
	if w.now == nil {
		panic("timewheel: nil clock")
	}

	w.buckets = make([][][]*Timer, w.levels)
	for level := range w.buckets {
		w.buckets[level] = make([][]*Timer, w.wheelSize)
	}
	w.lastTime = w.now()
	w.lastTick = w.tickOf(w.lastTime)
	return w
}

// Schedule runs fn once, after delay. A non-positive delay fires fn on the next
// Advance. It panics when fn is nil.
func (w *Wheel) Schedule(delay time.Duration, fn func()) *Timer {
	if fn == nil {
		panic("timewheel: callback must not be nil")
	}
	now := w.now()
	return w.add(now.Add(delay), fn, now)
}

// ScheduleAt is Schedule with an absolute deadline. A deadline in the past
// fires on the next Advance.
func (w *Wheel) ScheduleAt(deadline time.Time, fn func()) *Timer {
	if fn == nil {
		panic("timewheel: callback must not be nil")
	}
	now := w.now()
	return w.add(deadline, fn, now)
}

// Advance moves the wheel forward to now, runs every timer whose fire tick has
// been reached - in deadline order - and returns how many callbacks ran.
//
// An Advance that does not move time forward is a no-op and returns 0; timers
// scheduled with a non-positive delay are still fired. A panicking callback
// propagates to the caller and skips the remaining callbacks of that Advance.
func (w *Wheel) Advance(now time.Time) int {
	w.mu.Lock()
	due := w.advanceLocked(now)
	w.mu.Unlock()
	return w.fire(due)
}

// AdvanceBy advances by d from the time of the previous Advance (not from the
// injected clock), which keeps a test or game loop deterministic. It panics
// when d is negative.
func (w *Wheel) AdvanceBy(d time.Duration) int {
	if d < 0 {
		panic("timewheel: advance duration must not be negative")
	}
	w.mu.Lock()
	due := w.advanceLocked(w.lastTime.Add(d))
	w.mu.Unlock()
	return w.fire(due)
}

// Tick advances by one tick from the time of the previous Advance and returns
// how many callbacks ran.
func (w *Wheel) Tick() int {
	w.mu.Lock()
	due := w.advanceLocked(w.lastTime.Add(w.tick))
	w.mu.Unlock()
	return w.fire(due)
}

// Len returns how many timers are still pending: fired and cancelled timers are
// not counted.
func (w *Wheel) Len() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.pending
}

// Stop cancels every pending timer and empties the wheel. Timers that were
// still pending report true from Done and false from Cancel afterwards.
func (w *Wheel) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for level := range w.buckets {
		for slot := range w.buckets[level] {
			for _, timer := range w.buckets[level][slot] {
				timer.cancelled = true
			}
			w.buckets[level][slot] = nil
		}
	}
	for _, timer := range w.immediate {
		timer.cancelled = true
	}
	w.immediate = nil
	w.pending = 0
}

// Run drives the wheel with a real ticker: it calls Tick every tick until ctx is
// done, then returns. It is meant for production loops; tests use Advance.
func (w *Wheel) Run(ctx context.Context) {
	ticker := time.NewTicker(w.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.Advance(w.now())
		}
	}
}

// add registers fn for deadline, using now as the reference point.
func (w *Wheel) add(deadline time.Time, fn func(), now time.Time) *Timer {
	w.mu.Lock()
	defer w.mu.Unlock()

	// a driver may already have advanced past the clock reading, so schedule
	// relative to whichever is later; otherwise the timer would land in a bucket
	// that has already been processed
	ref := now
	if w.lastTime.After(ref) {
		ref = w.lastTime
	}

	w.seq++
	timer := &Timer{w: w, deadline: deadline, fn: fn, seq: w.seq}
	w.pending++
	w.insertLocked(timer, ref)
	return timer
}

// insertLocked stores the timer in the bucket whose processing time is the fire
// tick for its deadline. The caller must hold the mutex.
func (w *Wheel) insertLocked(timer *Timer, ref time.Time) {
	d := timer.deadline.Sub(ref)
	if d <= 0 {
		w.immediate = append(w.immediate, timer)
		return
	}

	// the fire tick is the first tick boundary at or after the deadline
	ticks := int64((d + w.tick - 1) / w.tick)
	level := 0
	for level+1 < w.levels && ticks >= w.unitTicks(level+1) {
		level++
	}
	slot := int((w.tickOf(ref) + ticks) / w.unitTicks(level) % int64(w.wheelSize))
	w.buckets[level][slot] = append(w.buckets[level][slot], timer)
}

// advanceLocked moves the wheel to now and returns the timers that came due in
// deadline order. The caller must hold the mutex, and must run the callbacks
// after releasing it.
func (w *Wheel) advanceLocked(now time.Time) []*Timer {
	if now.Before(w.lastTime) {
		return nil // time did not move forward
	}

	due := make([]*Timer, 0, len(w.immediate))
	for _, timer := range w.immediate {
		if timer.cancelled {
			continue // Cancel already dropped it from the pending count
		}
		w.expireLocked(timer, &due)
	}
	w.immediate = nil

	target := w.tickOf(now)
	if w.pending > 0 {
		for tick := w.lastTick + 1; tick <= target; tick++ {
			w.processTickLocked(tick, &due)
		}
	}
	w.lastTick = target
	w.lastTime = now

	sort.Slice(due, func(i, j int) bool {
		if due[i].deadline.Equal(due[j].deadline) {
			return due[i].seq < due[j].seq
		}
		return due[i].deadline.Before(due[j].deadline)
	})
	return due
}

// processTickLocked moves the wheel to tick and collects the timers that came
// due. The caller must hold the mutex.
func (w *Wheel) processTickLocked(tick int64, due *[]*Timer) {
	now := w.tickTime(tick)

	// coarser levels first: their buckets cascade into finer ones, which are
	// always processed at a later tick than the cascade
	for level := w.levels - 1; level >= 1; level-- {
		unit := w.unitTicks(level)
		if tick%unit != 0 {
			continue
		}
		slot := int(tick / unit % int64(w.wheelSize))
		bucket := w.buckets[level][slot]
		w.buckets[level][slot] = nil
		for _, timer := range bucket {
			if timer.cancelled {
				continue
			}
			if timer.deadline.After(now) {
				w.insertLocked(timer, now)
				continue
			}
			w.expireLocked(timer, due)
		}
	}

	// level 0 fires: its bucket is one tick wide
	slot := int(tick % int64(w.wheelSize))
	bucket := w.buckets[0][slot]
	w.buckets[0][slot] = nil
	for _, timer := range bucket {
		if timer.cancelled {
			continue
		}
		if !timer.deadline.After(now) {
			w.expireLocked(timer, due)
			continue
		}
		// the deadline is inside the next tick: hold the timer one tick longer
		// instead of firing it early
		next := int((tick + 1) % int64(w.wheelSize))
		w.buckets[0][next] = append(w.buckets[0][next], timer)
	}
}

// expireLocked marks a timer as fired and appends it to due.
func (w *Wheel) expireLocked(timer *Timer, due *[]*Timer) {
	timer.fired = true
	w.pending--
	*due = append(*due, timer)
}

// fire runs the callbacks outside the wheel lock and reports how many ran.
func (w *Wheel) fire(due []*Timer) int {
	for _, timer := range due {
		timer.fn()
	}
	return len(due)
}

// unitTicks returns how many ticks one bucket of the given level spans.
func (w *Wheel) unitTicks(level int) int64 {
	unit := int64(1)
	for range level {
		unit *= int64(w.wheelSize)
	}
	return unit
}

// tickOf returns the tick index of t.
func (w *Wheel) tickOf(t time.Time) int64 {
	return floorDiv(t.UnixNano(), int64(w.tick))
}

// tickTime returns the time a tick index starts at.
func (w *Wheel) tickTime(tick int64) time.Time {
	return time.Unix(0, tick*int64(w.tick))
}

// floorDiv divides rounding towards negative infinity, so tick indexes stay
// congruent for clocks before the Unix epoch.
func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}
