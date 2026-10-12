package concurrent_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/concurrent"
)

func BenchmarkConcurrentSetAdd(b *testing.B) {
	s := concurrent.NewHashSetWithCapacity[int](1 << 16)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			s.Add(i & 0xFFFF)
			i++
		}
	})
}

func BenchmarkConcurrentSetContains(b *testing.B) {
	s := concurrent.NewHashSetWithCapacity[int](1 << 12)
	for i := range 1 << 12 {
		s.Add(i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			s.Contains(i & 0xFFF)
			i++
		}
	})
}

func BenchmarkConcurrentMapGet(b *testing.B) {
	m := concurrent.NewHashMapWithCapacity[int, int](1 << 12)
	for i := range 1 << 12 {
		m.Put(i, i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.Get(i & 0xFFF)
			i++
		}
	})
}

func BenchmarkConcurrentSkipListGet(b *testing.B) {
	sl := concurrent.NewSkipListMap[int, int]()
	for i := range 1 << 12 {
		sl.Put(i, i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sl.Get(i & 0xFFF)
			i++
		}
	})
}

func BenchmarkConcurrentLRUGet(b *testing.B) {
	c := concurrent.NewLRU[int, int](1 << 12)
	for i := range 1 << 12 {
		c.Put(i, i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Get(i & 0xFFF)
			i++
		}
	})
}
