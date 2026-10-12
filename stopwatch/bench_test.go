package stopwatch_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/stopwatch"
)

// BenchmarkElapsed measures the hot read path on a running stopwatch; it must
// not allocate.
func BenchmarkElapsed(b *testing.B) {
	clock := newFakeClock()
	sw := stopwatch.NewStarted(stopwatch.WithClock(clock.Now))

	b.ReportAllocs()
	for b.Loop() {
		_ = sw.Elapsed()
	}
}

// BenchmarkStartStop measures a full control cycle, including both lock
// acquisitions and the two clock reads.
func BenchmarkStartStop(b *testing.B) {
	clock := newFakeClock()
	sw := stopwatch.New(stopwatch.WithClock(clock.Now))

	b.ReportAllocs()
	for b.Loop() {
		sw.Start()
		sw.Stop()
	}
}
