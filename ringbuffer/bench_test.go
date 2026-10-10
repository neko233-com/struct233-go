package ringbuffer_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/ringbuffer"
)

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
