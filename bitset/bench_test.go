package bitset_test

import (
	"testing"

	"github.com/neko233-com/go-data-struct-lib/bitset"
)

func benchBitSet(n int) *bitset.BitSet {
	b := bitset.New()
	for i := 0; i < n; i += 3 {
		b.Set(i)
	}
	return b
}

func BenchmarkBitSetSet(b *testing.B) {
	b.ReportAllocs()
	bs := bitset.New()
	for i := 0; b.Loop(); i++ {
		bs.Set(i & 4095)
	}
}

func BenchmarkBitSetGet(b *testing.B) {
	bs := benchBitSet(4096)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		bs.Get(i & 4095)
	}
}

func BenchmarkBitSetCardinality(b *testing.B) {
	bs := benchBitSet(4096)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		bs.Cardinality()
	}
}

func BenchmarkBitSetOr(b *testing.B) {
	a := benchBitSet(4096)
	other := benchBitSet(8192)

	b.ReportAllocs()
	for b.Loop() {
		a.Or(other)
	}
}

func BenchmarkBitSetToSlice(b *testing.B) {
	bs := benchBitSet(4096)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		bs.ToSlice()
	}
}
