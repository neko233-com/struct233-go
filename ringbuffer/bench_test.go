package ringbuffer_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/ringbuffer"
)

// TestPeekDoesNotAllocate guards the hit read path.
func TestPeekDoesNotAllocate(t *testing.T) {
	r := ringbuffer.New[int](64)
	for i := range 64 {
		r.Push(i)
	}

	allocs := testing.AllocsPerRun(100, func() { r.Peek() })
	require.Zero(t, allocs, "Peek hit must not allocate")
}

// TestPushWithEvictionDoesNotAllocate guards the full-buffer overwrite path.
func TestPushWithEvictionDoesNotAllocate(t *testing.T) {
	r := ringbuffer.New[int](1024)
	for i := range 1024 {
		r.Push(i)
	}

	allocs := testing.AllocsPerRun(100, func() { r.Push(1) })
	require.Zero(t, allocs, "Push with eviction must not allocate")
}

func BenchmarkRingBufferPush(b *testing.B) {
	b.ReportAllocs()
	r := ringbuffer.New[int](1024)
	for i := 0; b.Loop(); i++ {
		r.Push(i)
	}
}

func BenchmarkRingBufferPeek(b *testing.B) {
	r := ringbuffer.New[int](1024)
	for i := range 1024 {
		r.Push(i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		r.Peek()
	}
}

func BenchmarkRingBufferPop(b *testing.B) {
	r := ringbuffer.New[int](1024)
	for i := range 1024 {
		r.Push(i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		r.Pop()
		r.Push(1) // keep the buffer steady
	}
}

func BenchmarkRingBufferAt(b *testing.B) {
	r := ringbuffer.New[int](1024)
	for i := range 1024 {
		r.Push(i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		r.At(i & 1023)
	}
}

func BenchmarkRingBufferToSlice(b *testing.B) {
	r := ringbuffer.New[int](1024)
	for i := range 1024 {
		r.Push(i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		r.ToSlice()
	}
}
