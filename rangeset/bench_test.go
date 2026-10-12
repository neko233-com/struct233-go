package rangeset_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/rangeset"
)

// sink keeps benchmark results alive so the compiler cannot drop the work.
var sink any

// benchSet returns a set of n spans of two values each, one gap apart.
func benchSet(n int) *rangeset.RangeSet[int] {
	s := rangeset.NewSet[int]()
	for i := range n {
		s.Add(rangeset.Closed(3*i, 3*i+1))
	}
	return s
}

func BenchmarkRangeSetAdd(b *testing.B) {
	s := rangeset.NewSet[int]()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		s.Add(rangeset.Closed(i%1000, i%1000+10))
	}
	sink = s.Len()
}

func BenchmarkRangeSetRemove(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		s := benchSet(64)
		s.Remove(rangeset.Closed(i%100, i%100+20))
		sink = s.Len()
	}
}

func BenchmarkRangeSetContains(b *testing.B) {
	s := benchSet(1000)
	hit := false
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		hit = s.Contains(i % 3000)
	}
	sink = hit
}

func BenchmarkRangeSetUnion(b *testing.B) {
	left, right := benchSet(100), benchSet(100)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		sink = left.Union(right).Contains(i % 300)
	}
}

func BenchmarkRangeSetIntersection(b *testing.B) {
	left, right := benchSet(100), benchSet(100)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		sink = left.Intersection(right).Contains(i % 300)
	}
}

func BenchmarkRangeSetDifference(b *testing.B) {
	left, right := benchSet(100), benchSet(100)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		sink = left.Difference(right).Contains(i % 300)
	}
}

func BenchmarkRangeSetComplement(b *testing.B) {
	s := benchSet(1000)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		sink = s.Complement().Contains(i % 3000)
	}
}

func BenchmarkRangeContains(b *testing.B) {
	r := rangeset.ClosedOpen(0, 1000)
	hit := false
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		hit = r.Contains(i % 2000)
	}
	sink = hit
}

func BenchmarkRangeSpan(b *testing.B) {
	a, c := rangeset.Closed(1, 500), rangeset.ClosedOpen(200, 900)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		sink = a.Span(c).Contains(i % 1000)
	}
}
