package striped_test

import (
	"strconv"
	"sync"
	"testing"

	"github.com/neko233-com/struct233-go/striped"
)

func benchHashString(s string) uint64 {
	h := uint64(0xcbf29ce484222325)
	for i := 0; i < len(s); i++ {
		h = (h ^ uint64(s[i])) * 0x100000001b3
	}
	return h
}

// BenchmarkGet measures the hot path: hash + mask + pointer return.
func BenchmarkGet(b *testing.B) {
	s := striped.New(256, benchHashString)
	key := "benchmark-key"

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = s.Get(key)
	}
}

// BenchmarkGetVaryingKeys measures Get with a per-iteration key so the hash
// function is exercised end to end. Keys are precomputed so the benchmark
// reports only Get's own cost.
func BenchmarkGetVaryingKeys(b *testing.B) {
	s := striped.New(256, benchHashString)
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = "key-" + strconv.Itoa(i)
	}

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = s.Get(keys[i&(len(keys)-1)])
	}
}

// BenchmarkGetAt measures direct stripe access without hashing.
func BenchmarkGetAt(b *testing.B) {
	s := striped.New(256, benchHashString)

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = s.GetAt(i)
	}
}

// BenchmarkGetLockUnlock measures the full acquire/release cycle.
func BenchmarkGetLockUnlock(b *testing.B) {
	s := striped.New(256, benchHashString)
	key := "benchmark-key"

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		mu := s.Get(key)
		mu.Lock()
		mu.Unlock()
	}
}

// BenchmarkGetRW measures the read-write variant's lookup path.
func BenchmarkGetRW(b *testing.B) {
	s := striped.NewRW(256, benchHashString)
	key := "benchmark-key"

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = s.GetRW(key)
	}
}

// BenchmarkGetRWRLockRUnlock measures a shared-read acquire/release cycle.
func BenchmarkGetRWRLockRUnlock(b *testing.B) {
	s := striped.NewRW(256, benchHashString)
	key := "benchmark-key"

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		mu := s.GetRW(key)
		mu.RLock()
		mu.RUnlock()
	}
}

// BenchmarkGetParallel measures Get + Lock/Unlock under parallel contention.
func BenchmarkGetParallel(b *testing.B) {
	s := striped.New(256, benchHashString)
	keys := make([]string, 256)
	for i := range keys {
		keys[i] = "key-" + strconv.Itoa(i)
	}

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			mu := s.Get(keys[i&(len(keys)-1)])
			mu.Lock()
			mu.Unlock()
			i++
		}
	})
}

// BenchmarkGetParallelGlobalLock is the baseline: one global mutex instead of
// stripes. Comparing the two shows the striping win under contention.
func BenchmarkGetParallelGlobalLock(b *testing.B) {
	var mu sync.Mutex

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			mu.Unlock()
		}
	})
}
