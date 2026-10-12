package timewheel_test

import (
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/timewheel"
)

func BenchmarkSchedule(b *testing.B) {
	w := timewheel.New(time.Millisecond, 512)

	b.ReportAllocs()
	for b.Loop() {
		w.Schedule(time.Hour, func() {})
		w.Stop()
	}
}

func BenchmarkAdvance(b *testing.B) {
	c := newClock()
	w := timewheel.New(10*time.Millisecond, 512, timewheel.WithClock(c.Now))
	for i := range 10_000 {
		w.Schedule(time.Duration(1+i%1000)*10*time.Millisecond, func() {})
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.now = c.now.Add(time.Second)
		w.Advance(c.Now())
	}
}

func BenchmarkCancel(b *testing.B) {
	c := newClock()
	w := timewheel.New(10*time.Millisecond, 512, timewheel.WithClock(c.Now))

	b.ReportAllocs()
	for b.Loop() {
		timer := w.Schedule(time.Hour, func() {})
		timer.Cancel()
	}
}
