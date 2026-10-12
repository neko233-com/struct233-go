package rangemap_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/rangemap"
	"github.com/neko233-com/struct233-go/rangeset"
)

// sink keeps benchmark results alive so the compiler cannot drop the work.
var sink any

// benchRangeMap returns a map of n disjoint two-value intervals, one gap
// apart.
func benchRangeMap(n int) *rangemap.RangeMap[int64, int64] {
	m := rangemap.New[int64, int64]()
	for i := int64(0); i < int64(n); i++ {
		m.Put(rangeset.Closed(3*i, 3*i+1), i)
	}
	return m
}

func BenchmarkRangeMapPut(b *testing.B) {
	m := rangemap.New[int64, int64]()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		m.Put(rangeset.Closed(int64(i%2000), int64(i%2000)+5), int64(i))
	}
	sink = m.Len()
}

func BenchmarkRangeMapPutSplit(b *testing.B) {
	m := rangemap.New[int64, int64]()
	m.Put(rangeset.Closed(int64(0), int64(1000)), 1)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		m.Put(rangeset.Closed(int64(i%1000), int64(i%1000)+2), int64(i))
	}
	sink = m.Len()
}

func BenchmarkRangeMapGet(b *testing.B) {
	m := benchRangeMap(1000)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_, _ = m.Get(int64(i % 3000))
	}
}

func BenchmarkRangeMapGetEntry(b *testing.B) {
	m := benchRangeMap(1000)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_, _ = m.GetEntry(int64(i % 3000))
	}
}

func BenchmarkRangeMapRemove(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		m := benchRangeMap(200)
		m.Remove(rangeset.Closed(int64(i%300), int64(i%300)+100))
		sink = m.Len()
	}
}

func BenchmarkRangeMapAll(b *testing.B) {
	m := benchRangeMap(1000)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		n := 0
		for _, v := range m.All() {
			n += int(v)
		}
		sink = n
	}
}

func BenchmarkRangeMapEntries(b *testing.B) {
	m := benchRangeMap(1000)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		sink = len(m.Entries())
	}
}
