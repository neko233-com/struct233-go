package ratelimit_test

import (
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/ratelimit"
)

// BenchmarkAllow measures the sequential admit path with a bucket that never
// empties, so the refill branch is exercised but no caller ever backs off.
func BenchmarkAllow(b *testing.B) {
	clock := newFakeClock()
	l := ratelimit.New(1e9, 1<<20, ratelimit.WithClock(clock.Now))

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if !l.Allow() {
			b.Fatal("bucket must not run dry")
		}
	}
}

// BenchmarkAllowParallel measures contention on the limiter mutex.
func BenchmarkAllowParallel(b *testing.B) {
	clock := newFakeClock()
	l := ratelimit.New(1e9, 1<<30, ratelimit.WithClock(clock.Now))

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Allow()
		}
	})
}

// BenchmarkAllowN measures the multi-token accounting path.
func BenchmarkAllowN(b *testing.B) {
	clock := newFakeClock()
	l := ratelimit.New(1e9, 1<<20, ratelimit.WithClock(clock.Now))

	b.ReportAllocs()
	for b.Loop() {
		l.AllowN(4)
	}
}

// BenchmarkTokens measures the read path, which refills and reports.
func BenchmarkTokens(b *testing.B) {
	clock := newFakeClock()
	l := ratelimit.New(1e9, 1<<20, ratelimit.WithClock(clock.Now))

	b.ReportAllocs()
	for b.Loop() {
		l.Tokens()
	}
}

// BenchmarkSlidingWindowAllow measures the admit path with slots constantly
// freed, so the ring eviction and the admit both run.
func BenchmarkSlidingWindowAllow(b *testing.B) {
	clock := newFakeClock()
	w := ratelimit.NewSlidingWindow(1024, time.Second, ratelimit.WithClock(clock.Now))

	b.ReportAllocs()
	for b.Loop() {
		clock.Advance(time.Millisecond)
		w.Allow()
	}
}

// BenchmarkSlidingWindowReject measures the path where the window is full.
func BenchmarkSlidingWindowReject(b *testing.B) {
	clock := newFakeClock()
	w := ratelimit.NewSlidingWindow(1024, time.Second, ratelimit.WithClock(clock.Now))
	for range 1024 {
		w.Allow()
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		w.Allow()
	}
}

// BenchmarkSlidingWindowAllowParallel measures contention on the window mutex.
// The limit bounds the ring size, so it stays realistic: the window costs
// O(limit) memory, unlike the token bucket which is O(1).
func BenchmarkSlidingWindowAllowParallel(b *testing.B) {
	clock := newFakeClock()
	w := ratelimit.NewSlidingWindow(1<<16, time.Second, ratelimit.WithClock(clock.Now))

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w.Allow()
		}
	})
}

// BenchmarkSlidingWindowEvictWithExpiry measures the path where every admitted
// event has left the window, which exercises the ring eviction.
func BenchmarkSlidingWindowEvictWithExpiry(b *testing.B) {
	clock := newFakeClock()
	w := ratelimit.NewSlidingWindow(8, time.Second, ratelimit.WithClock(clock.Now))

	b.ReportAllocs()
	for b.Loop() {
		clock.Advance(time.Second)
		w.Allow()
	}
}
