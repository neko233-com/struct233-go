package skiplist_test

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/skiplist"
)

const benchSize = 1 << 14

func benchSkipList(n int) *skiplist.SkipListMap[int, int] {
	sl := skiplist.New[int, int]()
	rng := rand.New(rand.NewPCG(1, 2))
	for range n {
		k := rng.IntN(n)
		sl.Put(k, k)
	}
	return sl
}

// TestReadPathsDoNotAllocate guards the zero-allocation read path.
func TestReadPathsDoNotAllocate(t *testing.T) {
	sl := benchSkipList(1024)

	allocs := testing.AllocsPerRun(100, func() { sl.Get(512) })
	require.Zero(t, allocs, "Get must not allocate")

	allocs = testing.AllocsPerRun(100, func() { sl.ContainsKey(900) })
	require.Zero(t, allocs, "ContainsKey must not allocate")

	allocs = testing.AllocsPerRun(100, func() {
		k, _, _ := sl.Floor(700)
		_ = k
	})
	require.Zero(t, allocs, "Floor must not allocate")
}

func BenchmarkSkipListPut(b *testing.B) {
	b.ReportAllocs()
	sl := skiplist.New[int, int]()
	for i := 0; b.Loop(); i++ {
		sl.Put(i, i)
	}
}

func BenchmarkSkipListGet(b *testing.B) {
	sl := benchSkipList(benchSize)
	rng := rand.New(rand.NewPCG(3, 4))
	keys := make([]int, 1024)
	for i := range keys {
		keys[i] = rng.IntN(benchSize)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		sl.Get(keys[i&1023])
	}
}

func BenchmarkSkipListRange(b *testing.B) {
	sl := benchSkipList(1 << 12)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		n := 0
		for range sl.Range(1000, 2000, true, false) {
			n++
		}
		if n == 0 {
			b.Fatal("empty range")
		}
	}
}

func BenchmarkSkipListIterate(b *testing.B) {
	sl := benchSkipList(1 << 12)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		n := 0
		for range sl.All() {
			n++
		}
		if n == 0 {
			b.Fatal("empty skiplist")
		}
	}
}
