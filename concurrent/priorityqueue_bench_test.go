package concurrent_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/concurrent"
)

func BenchmarkPriorityQueuePushPop(b *testing.B) {
	q := concurrent.NewHeapBlockingPriorityQueue[int](func(a, b int) int { return a - b })

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		q.Put(i)
		q.TryTake()
	}
}

func BenchmarkPriorityQueuePeek(b *testing.B) {
	q := concurrent.NewHeapBlockingPriorityQueue[int](func(a, b int) int { return a - b })
	q.Put(1)
	q.Put(2)
	q.Put(3)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = q.Peek()
	}
}
