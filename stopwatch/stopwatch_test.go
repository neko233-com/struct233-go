package stopwatch_test

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/stopwatch"
)

// fakeClock is a manually advanced time source so the table-driven tests never
// sleep and stay fully deterministic.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)}
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

// step is one scripted action plus the state expected right after it.
type step struct {
	op          string        // "check", "start", "stop", "reset", "restart", "advance"
	advance     time.Duration // only for "advance"
	panicWant   string        // expected panic value; empty means the op must not panic
	wantRunning bool
	wantElapsed time.Duration
}

// script is one table-driven subtest.
type script struct {
	name    string
	started bool // create with NewStarted instead of New
	steps   []step
}

// formatTime mirrors the documented String() shape, so the table cross-checks
// it independently of the exact formatting test below.
func formatTime(d time.Duration, running bool) string {
	out := d.String()
	if running {
		out += " (running)"
	}
	return out
}

func TestTableDrivenLifecycle(t *testing.T) {
	scripts := []script{
		{
			name: "newStartsStoppedAndZeroed",
			steps: []step{
				{op: "check", wantRunning: false, wantElapsed: 0},
			},
		},
		{
			name:    "newStartedStartsRunning",
			started: true,
			steps: []step{
				{op: "check", wantRunning: true, wantElapsed: 0},
			},
		},
		{
			name: "startOpensSegment",
			steps: []step{
				{op: "check", wantRunning: false, wantElapsed: 0},
				{op: "start", wantRunning: true, wantElapsed: 0},
			},
		},
		{
			name: "startStopCountsSegment",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 500 * time.Millisecond, wantRunning: true, wantElapsed: 500 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 500 * time.Millisecond},
			},
		},
		{
			name: "twoSegmentsAccumulate",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 300 * time.Millisecond, wantRunning: true, wantElapsed: 300 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 300 * time.Millisecond},
				{op: "advance", advance: 2 * time.Second, wantRunning: false, wantElapsed: 300 * time.Millisecond},
				{op: "start", wantRunning: true, wantElapsed: 300 * time.Millisecond},
				{op: "advance", advance: 200 * time.Millisecond, wantRunning: true, wantElapsed: 500 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 500 * time.Millisecond},
			},
		},
		{
			name: "threeSegmentsAccumulate",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 100 * time.Millisecond, wantRunning: true, wantElapsed: 100 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 100 * time.Millisecond},
				{op: "start", wantRunning: true, wantElapsed: 100 * time.Millisecond},
				{op: "advance", advance: 200 * time.Millisecond, wantRunning: true, wantElapsed: 300 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 300 * time.Millisecond},
				{op: "start", wantRunning: true, wantElapsed: 300 * time.Millisecond},
				{op: "advance", advance: 300 * time.Millisecond, wantRunning: true, wantElapsed: 600 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 600 * time.Millisecond},
			},
		},
		{
			name: "doubleStartPanics",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "start", panicWant: "stopwatch: already started", wantRunning: true, wantElapsed: 0},
			},
		},
		{
			name: "stopWhileStoppedPanics",
			steps: []step{
				{op: "stop", panicWant: "stopwatch: not running", wantRunning: false, wantElapsed: 0},
			},
		},
		{
			name: "stopTwicePanics",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: time.Second},
				{op: "stop", wantRunning: false, wantElapsed: time.Second},
				{op: "stop", panicWant: "stopwatch: not running", wantRunning: false, wantElapsed: time.Second},
			},
		},
		{
			name: "startAfterStopReopens",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: time.Second},
				{op: "stop", wantRunning: false, wantElapsed: time.Second},
				{op: "start", wantRunning: true, wantElapsed: time.Second},
				{op: "advance", advance: 500 * time.Millisecond, wantRunning: true, wantElapsed: 1500 * time.Millisecond},
			},
		},
		{
			name: "startPanicLeavesStateUnchanged",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 750 * time.Millisecond, wantRunning: true, wantElapsed: 750 * time.Millisecond},
				{op: "start", panicWant: "stopwatch: already started", wantRunning: true, wantElapsed: 750 * time.Millisecond},
				{op: "advance", advance: 250 * time.Millisecond, wantRunning: true, wantElapsed: time.Second},
			},
		},
		{
			name: "stopPanicLeavesStateUnchanged",
			steps: []step{
				{op: "advance", advance: 5 * time.Second, wantRunning: false, wantElapsed: 0},
				{op: "stop", panicWant: "stopwatch: not running", wantRunning: false, wantElapsed: 0},
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 100 * time.Millisecond, wantRunning: true, wantElapsed: 100 * time.Millisecond},
			},
		},
		{
			name: "resetWhileStaysZero",
			steps: []step{
				{op: "reset", wantRunning: false, wantElapsed: 0},
			},
		},
		{
			name: "resetWhileRunningDropsSegment",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 5 * time.Second, wantRunning: true, wantElapsed: 5 * time.Second},
				{op: "reset", wantRunning: false, wantElapsed: 0},
				{op: "check", wantRunning: false, wantElapsed: 0},
			},
		},
		{
			name: "resetClearsAccumulated",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: time.Second},
				{op: "stop", wantRunning: false, wantElapsed: time.Second},
				{op: "start", wantRunning: true, wantElapsed: time.Second},
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: 2 * time.Second},
				{op: "stop", wantRunning: false, wantElapsed: 2 * time.Second},
				{op: "reset", wantRunning: false, wantElapsed: 0},
			},
		},
		{
			name: "resetTwiceIsIdempotent",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 3 * time.Second, wantRunning: true, wantElapsed: 3 * time.Second},
				{op: "stop", wantRunning: false, wantElapsed: 3 * time.Second},
				{op: "reset", wantRunning: false, wantElapsed: 0},
				{op: "reset", wantRunning: false, wantElapsed: 0},
			},
		},
		{
			name: "restartWhileStoppedStartsFresh",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 2 * time.Second, wantRunning: true, wantElapsed: 2 * time.Second},
				{op: "stop", wantRunning: false, wantElapsed: 2 * time.Second},
				{op: "restart", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: time.Second},
			},
		},
		{
			name: "restartWhileRunningDropsSegment",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 2 * time.Second, wantRunning: true, wantElapsed: 2 * time.Second},
				{op: "restart", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: time.Second},
				{op: "stop", wantRunning: false, wantElapsed: time.Second},
			},
		},
		{
			name: "restartCountsOnlyNewSegment",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 750 * time.Millisecond, wantRunning: true, wantElapsed: 750 * time.Millisecond},
				{op: "restart", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 250 * time.Millisecond, wantRunning: true, wantElapsed: 250 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 250 * time.Millisecond},
			},
		},
		{
			name: "restartTwiceKeepsFreshSegment",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: time.Second},
				{op: "restart", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 500 * time.Millisecond, wantRunning: true, wantElapsed: 500 * time.Millisecond},
				{op: "restart", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 250 * time.Millisecond, wantRunning: true, wantElapsed: 250 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 250 * time.Millisecond},
			},
		},
		{
			name:    "elapsedIncludesRunningSegment",
			started: true,
			steps: []step{
				{op: "advance", advance: 1500 * time.Millisecond, wantRunning: true, wantElapsed: 1500 * time.Millisecond},
				{op: "check", wantRunning: true, wantElapsed: 1500 * time.Millisecond},
			},
		},
		{
			name: "elapsedFrozenWhileStopped",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: time.Second},
				{op: "stop", wantRunning: false, wantElapsed: time.Second},
				{op: "advance", advance: time.Hour, wantRunning: false, wantElapsed: time.Second},
			},
		},
		{
			name: "gapBetweenSegmentsNotCounted",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 250 * time.Millisecond, wantRunning: true, wantElapsed: 250 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 250 * time.Millisecond},
				{op: "advance", advance: time.Hour, wantRunning: false, wantElapsed: 250 * time.Millisecond},
				{op: "start", wantRunning: true, wantElapsed: 250 * time.Millisecond},
				{op: "advance", advance: 250 * time.Millisecond, wantRunning: true, wantElapsed: 500 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 500 * time.Millisecond},
			},
		},
		{
			name: "zeroLengthSegment",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "stop", wantRunning: false, wantElapsed: 0},
			},
		},
		{
			name: "advanceZeroChangesNothing",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 0, wantRunning: true, wantElapsed: 0},
				{op: "stop", wantRunning: false, wantElapsed: 0},
				{op: "advance", advance: 0, wantRunning: false, wantElapsed: 0},
			},
		},
		{
			name: "mixedGapsAndSegments",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 100 * time.Millisecond, wantRunning: true, wantElapsed: 100 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 100 * time.Millisecond},
				{op: "advance", advance: 3 * time.Second, wantRunning: false, wantElapsed: 100 * time.Millisecond},
				{op: "restart", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 400 * time.Millisecond, wantRunning: true, wantElapsed: 400 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 400 * time.Millisecond},
				{op: "advance", advance: 9 * time.Second, wantRunning: false, wantElapsed: 400 * time.Millisecond},
				{op: "start", wantRunning: true, wantElapsed: 400 * time.Millisecond},
				{op: "advance", advance: 100 * time.Millisecond, wantRunning: true, wantElapsed: 500 * time.Millisecond},
			},
		},
		{
			name: "longSegmentOneHour",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: time.Hour, wantRunning: true, wantElapsed: time.Hour},
				{op: "stop", wantRunning: false, wantElapsed: time.Hour},
			},
		},
		{
			name: "restartAfterReset",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 2 * time.Second, wantRunning: true, wantElapsed: 2 * time.Second},
				{op: "reset", wantRunning: false, wantElapsed: 0},
				{op: "restart", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: time.Second},
				{op: "stop", wantRunning: false, wantElapsed: time.Second},
			},
		},
		{
			name: "resetAfterRestart",
			steps: []step{
				{op: "restart", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 2 * time.Second, wantRunning: true, wantElapsed: 2 * time.Second},
				{op: "reset", wantRunning: false, wantElapsed: 0},
			},
		},
		{
			name: "manyTinySegments",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 250 * time.Millisecond, wantRunning: true, wantElapsed: 250 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 250 * time.Millisecond},
				{op: "start", wantRunning: true, wantElapsed: 250 * time.Millisecond},
				{op: "advance", advance: 250 * time.Millisecond, wantRunning: true, wantElapsed: 500 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 500 * time.Millisecond},
				{op: "start", wantRunning: true, wantElapsed: 500 * time.Millisecond},
				{op: "advance", advance: 250 * time.Millisecond, wantRunning: true, wantElapsed: 750 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 750 * time.Millisecond},
				{op: "start", wantRunning: true, wantElapsed: 750 * time.Millisecond},
				{op: "advance", advance: 250 * time.Millisecond, wantRunning: true, wantElapsed: time.Second},
				{op: "stop", wantRunning: false, wantElapsed: time.Second},
			},
		},
		{
			name: "startStopChainRepeated",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: time.Second},
				{op: "stop", wantRunning: false, wantElapsed: time.Second},
				{op: "start", wantRunning: true, wantElapsed: time.Second},
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: 2 * time.Second},
				{op: "stop", wantRunning: false, wantElapsed: 2 * time.Second},
			},
		},
		{
			name: "doubleStartPanicThenStopWorks",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 500 * time.Millisecond, wantRunning: true, wantElapsed: 500 * time.Millisecond},
				{op: "start", panicWant: "stopwatch: already started", wantRunning: true, wantElapsed: 500 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 500 * time.Millisecond},
			},
		},
		{
			name: "stopPanicThenStartWorks",
			steps: []step{
				{op: "stop", panicWant: "stopwatch: not running", wantRunning: false, wantElapsed: 0},
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 100 * time.Millisecond, wantRunning: true, wantElapsed: 100 * time.Millisecond},
			},
		},
		{
			name: "startPanicsTwiceBothReported",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "start", panicWant: "stopwatch: already started", wantRunning: true, wantElapsed: 0},
				{op: "start", panicWant: "stopwatch: already started", wantRunning: true, wantElapsed: 0},
			},
		},
		{
			name: "stopPanicsTwiceBothReported",
			steps: []step{
				{op: "stop", panicWant: "stopwatch: not running", wantRunning: false, wantElapsed: 0},
				{op: "stop", panicWant: "stopwatch: not running", wantRunning: false, wantElapsed: 0},
				{op: "restart", wantRunning: true, wantElapsed: 0},
			},
		},
		{
			name: "subSecondPrecision",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 375 * time.Millisecond, wantRunning: true, wantElapsed: 375 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 375 * time.Millisecond},
			},
		},
		{
			name: "nanosecondSegment",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: time.Nanosecond, wantRunning: true, wantElapsed: time.Nanosecond},
				{op: "stop", wantRunning: false, wantElapsed: time.Nanosecond},
			},
		},
		{
			name: "alternatingSegments",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 10 * time.Millisecond, wantRunning: true, wantElapsed: 10 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 10 * time.Millisecond},
				{op: "start", wantRunning: true, wantElapsed: 10 * time.Millisecond},
				{op: "advance", advance: 20 * time.Millisecond, wantRunning: true, wantElapsed: 30 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 30 * time.Millisecond},
				{op: "start", wantRunning: true, wantElapsed: 30 * time.Millisecond},
				{op: "advance", advance: 30 * time.Millisecond, wantRunning: true, wantElapsed: 60 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 60 * time.Millisecond},
			},
		},
		{
			name: "stopThenDayLongGapThenStart",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: time.Second},
				{op: "stop", wantRunning: false, wantElapsed: time.Second},
				{op: "advance", advance: 24 * time.Hour, wantRunning: false, wantElapsed: time.Second},
				{op: "start", wantRunning: true, wantElapsed: time.Second},
			},
		},
		{
			name: "resetWhileRunningAfterAccumulated",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: time.Second},
				{op: "stop", wantRunning: false, wantElapsed: time.Second},
				{op: "start", wantRunning: true, wantElapsed: time.Second},
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: 2 * time.Second},
				{op: "reset", wantRunning: false, wantElapsed: 0},
				{op: "advance", advance: 5 * time.Second, wantRunning: false, wantElapsed: 0},
			},
		},
		{
			name: "restartMidSegmentThenStop",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: time.Second},
				{op: "restart", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 2 * time.Second, wantRunning: true, wantElapsed: 2 * time.Second},
				{op: "stop", wantRunning: false, wantElapsed: 2 * time.Second},
			},
		},
		{
			name: "largeAccumulation",
			steps: []step{
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: time.Hour, wantRunning: true, wantElapsed: time.Hour},
				{op: "stop", wantRunning: false, wantElapsed: time.Hour},
				{op: "start", wantRunning: true, wantElapsed: time.Hour},
				{op: "advance", advance: 2 * time.Hour, wantRunning: true, wantElapsed: 3 * time.Hour},
				{op: "stop", wantRunning: false, wantElapsed: 3 * time.Hour},
			},
		},
		{
			name:    "startedThenResetKeepsStopped",
			started: true,
			steps: []step{
				{op: "advance", advance: 4 * time.Second, wantRunning: true, wantElapsed: 4 * time.Second},
				{op: "reset", wantRunning: false, wantElapsed: 0},
				{op: "advance", advance: 4 * time.Second, wantRunning: false, wantElapsed: 0},
			},
		},
		{
			name:    "startedThenRestartMidSegment",
			started: true,
			steps: []step{
				{op: "advance", advance: 900 * time.Millisecond, wantRunning: true, wantElapsed: 900 * time.Millisecond},
				{op: "restart", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 100 * time.Millisecond, wantRunning: true, wantElapsed: 100 * time.Millisecond},
				{op: "stop", wantRunning: false, wantElapsed: 100 * time.Millisecond},
			},
		},
		{
			name:    "startedStopPanicsSecondTime",
			started: true,
			steps: []step{
				{op: "advance", advance: time.Second, wantRunning: true, wantElapsed: time.Second},
				{op: "stop", wantRunning: false, wantElapsed: time.Second},
				{op: "stop", panicWant: "stopwatch: not running", wantRunning: false, wantElapsed: time.Second},
				{op: "reset", wantRunning: false, wantElapsed: 0},
			},
		},
		{
			name: "interleavedPanicsAndLifecycle",
			steps: []step{
				{op: "stop", panicWant: "stopwatch: not running", wantRunning: false, wantElapsed: 0},
				{op: "start", wantRunning: true, wantElapsed: 0},
				{op: "advance", advance: 2 * time.Second, wantRunning: true, wantElapsed: 2 * time.Second},
				{op: "start", panicWant: "stopwatch: already started", wantRunning: true, wantElapsed: 2 * time.Second},
				{op: "stop", wantRunning: false, wantElapsed: 2 * time.Second},
				{op: "advance", advance: 2 * time.Second, wantRunning: false, wantElapsed: 2 * time.Second},
				{op: "restart", wantRunning: true, wantElapsed: 0},
			},
		},
	}

	for _, tc := range scripts {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock()
			var sw *stopwatch.Stopwatch
			if tc.started {
				sw = stopwatch.NewStarted(stopwatch.WithClock(clock.Now))
			} else {
				sw = stopwatch.New(stopwatch.WithClock(clock.Now))
			}
			for i, st := range tc.steps {
				switch st.op {
				case "check", "start", "stop", "reset", "restart", "advance":
				default:
					t.Fatalf("step %d: unknown op %q", i, st.op)
				}
				do := func() {
					switch st.op {
					case "check":
					case "start":
						sw.Start()
					case "stop":
						sw.Stop()
					case "reset":
						sw.Reset()
					case "restart":
						sw.Restart()
					case "advance":
						clock.Advance(st.advance)
					}
				}
				if st.panicWant != "" {
					assert.PanicsWithValue(t, st.panicWant, do, "step %d (%s)", i, st.op)
				} else {
					require.NotPanics(t, do, "step %d (%s)", i, st.op)
				}
				assert.Equal(t, st.wantRunning, sw.IsRunning(), "step %d (%s): IsRunning", i, st.op)
				assert.Equal(t, st.wantElapsed, sw.Elapsed(), "step %d (%s): Elapsed", i, st.op)
				assert.Equal(t, formatTime(st.wantElapsed, st.wantRunning), sw.String(), "step %d (%s): String", i, st.op)
			}
		})
	}
}

func TestStringFormatting(t *testing.T) {
	cases := []struct {
		name    string
		advance time.Duration
		running bool
		want    string
	}{
		{name: "zeroStopped", advance: 0, running: false, want: "0s"},
		{name: "zeroRunning", advance: 0, running: true, want: "0s (running)"},
		{name: "oneAndHalfSecondsStopped", advance: 1500 * time.Millisecond, running: false, want: "1.5s"},
		{name: "oneAndHalfSecondsRunning", advance: 1500 * time.Millisecond, running: true, want: "1.5s (running)"},
		{name: "milliseconds", advance: 250 * time.Millisecond, running: false, want: "250ms"},
		{name: "minuteAndHalf", advance: 90 * time.Second, running: false, want: "1m30s"},
		{name: "nanosecond", advance: time.Nanosecond, running: true, want: "1ns (running)"},
		{name: "hour", advance: time.Hour, running: true, want: "1h0m0s (running)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock()
			sw := stopwatch.NewStarted(stopwatch.WithClock(clock.Now))
			clock.Advance(tc.advance)
			if !tc.running {
				sw.Stop()
			}
			require.Equal(t, tc.want, sw.String())
		})
	}
}

func TestNewValidation(t *testing.T) {
	require.PanicsWithValue(t, "stopwatch: nil clock",
		func() { stopwatch.New(stopwatch.WithClock(nil)) })
	require.PanicsWithValue(t, "stopwatch: nil clock",
		func() { stopwatch.NewStarted(stopwatch.WithClock(nil)) })
	require.PanicsWithValue(t, "stopwatch: nil option",
		func() { stopwatch.New(nil) })
	require.PanicsWithValue(t, "stopwatch: nil option",
		func() { stopwatch.NewStarted(nil) })
}

func TestChainingReturnsReceiver(t *testing.T) {
	sw := stopwatch.New()
	require.Same(t, sw, sw.Start(), "Start must return the receiver for chaining")
	require.Same(t, sw, sw.Stop(), "Stop must return the receiver for chaining")
}

// refModel is the differential-test reference: a stopwatch reduced to three
// plain variables with no encapsulation.
type refModel struct {
	now       time.Time
	running   bool
	startedAt time.Time
	accum     time.Duration
}

func newRefModel(now time.Time) *refModel {
	return &refModel{now: now}
}

func (m *refModel) elapsed() time.Duration {
	total := m.accum
	if m.running {
		total += m.now.Sub(m.startedAt)
	}
	return total
}

// applyOp runs one operation against both the real stopwatch and the
// reference model and asserts they agree. Illegal transitions must panic with
// the documented message and leave both states untouched. It is shared by the
// random differential test and the fuzz target.
func applyOp(t *testing.T, clock *fakeClock, sw *stopwatch.Stopwatch, m *refModel, op byte, i int) {
	t.Helper()
	switch op % 8 {
	case 0: // start
		if m.running {
			require.PanicsWithValue(t, "stopwatch: already started", func() { sw.Start() }, "op %d", i)
			break
		}
		require.NotPanics(t, func() { sw.Start() }, "op %d", i)
		m.running = true
		m.startedAt = m.now
	case 1: // stop
		if !m.running {
			require.PanicsWithValue(t, "stopwatch: not running", func() { sw.Stop() }, "op %d", i)
			break
		}
		require.NotPanics(t, func() { sw.Stop() }, "op %d", i)
		m.accum += m.now.Sub(m.startedAt)
		m.running = false
	case 2: // reset
		sw.Reset()
		m.accum = 0
		m.startedAt = time.Time{}
		m.running = false
	case 3: // restart
		sw.Restart()
		m.accum = 0
		m.startedAt = m.now
		m.running = true
	case 4: // advance, the only way the fake clock moves
		d := time.Duration((int(op)+1)*(i%7+1)) * time.Millisecond
		clock.Advance(d)
		m.now = m.now.Add(d)
	case 5: // elapsed read
		require.Equal(t, m.elapsed(), sw.Elapsed(), "op %d: Elapsed", i)
	case 6: // running read
		require.Equal(t, m.running, sw.IsRunning(), "op %d: IsRunning", i)
	case 7: // string read
		require.Equal(t, formatTime(m.elapsed(), m.running), sw.String(), "op %d: String", i)
	}
	// every operation, successful or panicked, must leave the observable state
	// consistent with the reference model
	require.Equal(t, m.elapsed(), sw.Elapsed(), "op %d: Elapsed after op", i)
	require.Equal(t, m.running, sw.IsRunning(), "op %d: IsRunning after op", i)
}

func TestRandomSequencesMatchReferenceModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20261012))
	for seq := 0; seq < 300; seq++ {
		clock := newFakeClock()
		sw := stopwatch.New(stopwatch.WithClock(clock.Now))
		m := newRefModel(clock.Now())
		ops := make([]byte, 120)
		rng.Read(ops)
		for i, op := range ops {
			applyOp(t, clock, sw, m, op, i)
			if t.Failed() {
				t.Fatalf("sequence %d diverged at op %d", seq, i)
			}
		}
	}
}

func FuzzStopwatchAgainstReferenceModel(f *testing.F) {
	f.Add([]byte{0, 4, 1, 4, 2, 3, 4, 1, 5, 6, 7})
	f.Add([]byte{})
	f.Add([]byte{0, 0, 1, 1, 2, 2, 3, 3})
	f.Add([]byte{0, 4, 1, 4, 1, 4, 0, 4, 1})
	f.Add([]byte{255, 254, 1, 2, 3, 4, 5, 6, 7})

	f.Fuzz(func(t *testing.T, ops []byte) {
		clock := newFakeClock()
		sw := stopwatch.New(stopwatch.WithClock(clock.Now))
		m := newRefModel(clock.Now())
		for i, op := range ops {
			applyOp(t, clock, sw, m, op, i)
		}
	})
}

// TestRealClockSmoke is the one test that touches time.Now: it only asserts
// loose lower bounds so it cannot flake on a loaded machine.
func TestRealClockSmoke(t *testing.T) {
	sw := stopwatch.NewStarted()
	time.Sleep(50 * time.Millisecond)
	sw.Stop()
	require.GreaterOrEqual(t, sw.Elapsed(), 40*time.Millisecond,
		"a 50ms sleep must measure at least 40ms")
	require.False(t, sw.IsRunning())

	sw.Start()
	time.Sleep(20 * time.Millisecond)
	require.GreaterOrEqual(t, sw.Elapsed(), 50*time.Millisecond,
		"the running segment adds to the accumulated one")
	sw.Stop()
}

func TestConcurrentReadersAndController(t *testing.T) {
	clock := newFakeClock()
	sw := stopwatch.NewStarted(stopwatch.WithClock(clock.Now))

	done := make(chan struct{})
	var wg sync.WaitGroup
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				_ = sw.Elapsed()
				_ = sw.IsRunning()
				_ = sw.String()
			}
		}()
	}

	const cycles = 500
	for i := 0; i < cycles; i++ {
		clock.Advance(time.Millisecond) // counted: the stopwatch is running
		sw.Stop()
		clock.Advance(time.Millisecond) // ignored: the stopwatch is stopped
		sw.Start()
	}

	close(done)
	wg.Wait()

	require.True(t, sw.IsRunning())
	require.Equal(t, cycles*time.Millisecond, sw.Elapsed(),
		"only the running half of every cycle may be counted")
}

// TestElapsedDoesNotAllocate pins the hot read path to zero allocations.
func TestElapsedDoesNotAllocate(t *testing.T) {
	clock := newFakeClock()
	running := stopwatch.NewStarted(stopwatch.WithClock(clock.Now))
	stopped := stopwatch.New(stopwatch.WithClock(clock.Now))

	assert.Zero(t, testing.AllocsPerRun(1000, func() { _ = running.Elapsed() }),
		"Elapsed on a running stopwatch must not allocate")
	assert.Zero(t, testing.AllocsPerRun(1000, func() { _ = stopped.Elapsed() }),
		"Elapsed on a stopped stopwatch must not allocate")
	assert.Zero(t, testing.AllocsPerRun(1000, func() { _ = running.IsRunning() }),
		"IsRunning must not allocate")
}
