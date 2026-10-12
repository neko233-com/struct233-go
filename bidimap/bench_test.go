package bidimap_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/bidimap"
)

// benchBiMap builds a bimap holding n bindings.
func benchBiMap(n int) *bidimap.BiMap[int, int] {
	b := bidimap.NewWithCapacity[int, int](n)
	for i := range n {
		b.Put(i, i)
	}
	return b
}

func BenchmarkBiMapPut(b *testing.B) {
	m := bidimap.New[int, int]()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		m.Put(i%1024, i)
	}
}

func BenchmarkBiMapForcePut(b *testing.B) {
	m := bidimap.New[int, int]()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		m.ForcePut(i%1024, i)
	}
}

func BenchmarkBiMapGet(b *testing.B) {
	m := benchBiMap(1024)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_, _ = m.Get(i % 1024)
	}
}

func BenchmarkBiMapGetKeyOf(b *testing.B) {
	m := benchBiMap(1024)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_, _ = m.GetKeyOf(i % 1024)
	}
}

func BenchmarkBiMapRemovePut(b *testing.B) {
	m := benchBiMap(1024)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		k := i % 1024
		_, _ = m.Remove(k)
		m.Put(k, k)
	}
}

func BenchmarkBiMapInverse(b *testing.B) {
	m := benchBiMap(1024)
	b.ReportAllocs()
	for b.Loop() {
		_ = m.Inverse()
	}
}

func BenchmarkBiMapInverseGet(b *testing.B) {
	inv := benchBiMap(1024).Inverse()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_, _ = inv.Get(i % 1024)
	}
}
