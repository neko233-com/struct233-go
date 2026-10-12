package ttlcache_test

import (
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/ttlcache"
)

// sink keeps benchmark results alive so the compiler cannot drop the work.
var sink any

// benchCache returns a clock-driven cache pre-filled with n entries.
func benchCache(n int) *ttlcache.Cache[int, int] {
	clock := newFakeClock()
	c := ttlcache.NewWithClock[int, int](n, time.Hour, clock.Now)
	for i := range n {
		c.Put(i, i)
	}
	return c
}

func BenchmarkTTLCacheGet(b *testing.B) {
	c := benchCache(4096)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_, _ = c.Get(i & 4095)
	}
}

func BenchmarkTTLCacheGetMiss(b *testing.B) {
	c := benchCache(4096)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_, _ = c.Get(1<<20 + (i & 4095))
	}
}

func BenchmarkTTLCachePeekHit(b *testing.B) {
	c := benchCache(4096)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_, _ = c.Peek(i & 4095)
	}
}

func BenchmarkTTLCachePutUpdate(b *testing.B) {
	c := benchCache(4096)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		c.Put(i&4095, i)
	}
}

func BenchmarkTTLCachePutInsert(b *testing.B) {
	clock := newFakeClock()
	c := ttlcache.NewWithClock[int, int](1<<20, time.Hour, clock.Now)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		c.Put(i, i)
	}
}

func BenchmarkTTLCachePutWithTTL(b *testing.B) {
	c := benchCache(4096)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		c.PutWithTTL(i&4095, i, time.Minute)
	}
}

func BenchmarkTTLCacheRemove(b *testing.B) {
	c := benchCache(4096)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		k := i & 4095
		c.Remove(k)
		c.Put(k, k) // keep the cache populated for the next removal
	}
}

func BenchmarkTTLCacheCleanupNone(b *testing.B) {
	c := benchCache(4096) // fresh clock: nothing is expired
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		sink = c.Cleanup()
	}
}

func BenchmarkTTLCacheKeys(b *testing.B) {
	c := benchCache(1024)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		sink = len(c.Keys())
	}
}
