package semaphore_test

import (
	"context"
	"testing"

	"github.com/neko233-com/struct233-go/semaphore"
)

// BenchmarkAcquireRelease measures the uncontended acquire/release pair: the
// single permit is always free again by the time the next iteration starts, so
// only the fast path (no queueing, no channel) is exercised.
func BenchmarkAcquireRelease(b *testing.B) {
	sem := semaphore.New(1)
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if err := sem.Acquire(ctx); err != nil {
			b.Fatal(err)
		}
		sem.Release(1)
	}
}

// BenchmarkTryAcquireParallel measures contention on the internal mutex, the
// path taken by non-blocking callers competing for permits.
func BenchmarkTryAcquireParallel(b *testing.B) {
	sem := semaphore.New(1 << 30)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if sem.TryAcquire() {
				sem.Release(1)
			}
		}
	})
}
