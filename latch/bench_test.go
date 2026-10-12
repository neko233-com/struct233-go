package latch_test

import (
	"context"
	"testing"

	"github.com/neko233-com/struct233-go/latch"
)

// sink keeps benchmark results alive so the compiler cannot drop the work.
var sink any

// BenchmarkAwaitOpen measures the already-open fast path, which must neither
// block nor take the mutex.
func BenchmarkAwaitOpen(b *testing.B) {
	l := latch.New(1)
	l.CountDown()
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if err := l.Await(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCountDown measures the decrement path, including the final call
// that closes the latch and wakes its waiters.
func BenchmarkCountDown(b *testing.B) {
	l := latch.New(b.N)

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		l.CountDown()
	}
}

// BenchmarkCount measures the remaining-count read-out.
func BenchmarkCount(b *testing.B) {
	l := latch.New(1 << 30)

	b.ReportAllocs()
	for b.Loop() {
		sink = l.Count()
	}
}

// BenchmarkOpen measures the non-blocking open probe.
func BenchmarkOpen(b *testing.B) {
	l := latch.New(1 << 30)

	b.ReportAllocs()
	for b.Loop() {
		sink = l.Open()
	}
}
