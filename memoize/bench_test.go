package memoize_test

import (
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/memoize"
)

// BenchmarkGetHit measures the cached Get path after the value has been
// computed. It must stay allocation-free.
func BenchmarkGetHit(b *testing.B) {
	s := memoize.Memoize(func() int { return 42 })
	_ = s.Get() // warm the cache

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = s.Get()
	}
}

// BenchmarkGetHitExpiring measures the cached Get path of an expiring
// supplier, including the freshness check against the clock.
func BenchmarkGetHitExpiring(b *testing.B) {
	s := memoize.MemoizeExpiring(func() int { return 42 }, time.Hour)
	_ = s.Get() // warm the cache

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = s.Get()
	}
}

// BenchmarkGetMiss measures the recompute path after every Invalidate, i.e.
// the uncontended cost of a full computation cycle.
func BenchmarkGetMiss(b *testing.B) {
	s := memoize.Memoize(func() int { return 42 })

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		s.Invalidate()
		_ = s.Get()
	}
}

// BenchmarkGetHitParallel measures the cached Get path under contention.
func BenchmarkGetHitParallel(b *testing.B) {
	s := memoize.Memoize(func() int { return 42 })
	_ = s.Get() // warm the cache

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = s.Get()
		}
	})
}
