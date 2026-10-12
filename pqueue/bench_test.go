package pqueue_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/pqueue"
)

// TestPushPopDoesNotAllocate guards the steady-state Push/Pop path: once the
// backing array has grown, an equal number of pushes and pops must not
// allocate.
func TestPushPopDoesNotAllocate(t *testing.T) {
	q := pqueue.New[int]()
	q.Push(1) // warm up the backing array

	allocs := testing.AllocsPerRun(100, func() {
		q.Push(2)
		q.Pop()
	})

	require.Zero(t, allocs, "steady-state Push+Pop must not allocate")
	require.Equal(t, 1, q.Len(), "the queue must be drained back to its warm-up size")
}

// TestPeekPopDoNotAllocate guards the read paths: Peek and Pop on a
// non-empty queue must not allocate.
func TestPeekPopDoNotAllocate(t *testing.T) {
	q := pqueue.New[int]()
	for i := range 8 {
		q.Push(i)
	}

	allocs := testing.AllocsPerRun(100, func() {
		q.Peek()
		q.Pop()
		q.Push(0)
	})
	require.Zero(t, allocs, "Peek/Pop must not allocate")
	require.Equal(t, 8, q.Len())
}

// BenchmarkPriorityQueuePeek measures the constant-time top read.
func BenchmarkPriorityQueuePeek(b *testing.B) {
	q := pqueue.New[int]()
	for i := range 1024 {
		q.Push(i)
	}

	b.ReportAllocs()
	for b.Loop() {
		sink, _ = q.Peek()
	}
}

// sink keeps benchmark results alive so the compiler cannot drop the work.
var sink any

func BenchmarkPriorityQueuePush(b *testing.B) {
	b.ReportAllocs()
	q := pqueue.New[int]()
	for i := 0; b.Loop(); i++ {
		q.Push(i & 1023)
	}
}

func BenchmarkPriorityQueuePushPop(b *testing.B) {
	q := pqueue.New[int]()
	for i := range 1024 {
		q.Push(i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		q.Push(i & 1023)
		q.Pop()
	}
}

func BenchmarkPriorityQueuePop(b *testing.B) {
	q := pqueue.New[int]()
	for i := range 1024 {
		q.Push(i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		q.Pop()
	}
}

func BenchmarkNewFromSlice(b *testing.B) {
	src := make([]int, 1024)
	for i := range src {
		src[i] = 1024 - i
	}

	b.ReportAllocs()
	for b.Loop() {
		pqueue.NewFromSlice(src, func(a, b int) int { return a - b })
	}
}
