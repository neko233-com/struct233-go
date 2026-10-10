package linkedmap_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/linkedmap"
)

// TestGetDoesNotAllocate guards the read path.
func TestGetDoesNotAllocate(t *testing.T) {
	m := linkedmap.New[int, int]()
	for i := range 1024 {
		m.Put(i, i)
	}

	allocs := testing.AllocsPerRun(100, func() { m.Get(512) })
	require.Zero(t, allocs, "Get must not allocate")
}

func BenchmarkLinkedMapPut(b *testing.B) {
	b.ReportAllocs()
	m := linkedmap.New[int, int]()
	for i := 0; b.Loop(); i++ {
		m.Put(i, i)
	}
}

func BenchmarkLinkedMapPutUpdate(b *testing.B) {
	m := linkedmap.New[int, int]()
	for i := range 1024 {
		m.Put(i, i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		m.Put(i&1023, i)
	}
}

func BenchmarkLinkedMapGet(b *testing.B) {
	m := linkedmap.New[int, int]()
	for i := range 1024 {
		m.Put(i, i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		m.Get(i & 1023)
	}
}

func BenchmarkLinkedMapKeys(b *testing.B) {
	m := linkedmap.New[int, int]()
	for i := range 1024 {
		m.Put(i, i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.Keys()
	}
}
