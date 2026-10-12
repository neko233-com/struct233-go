package unionfind_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/unionfind"
)

// sink keeps benchmark results alive so the compiler cannot drop the work.
var sink any

func BenchmarkUnionFindUnion(b *testing.B) {
	const n = 1 << 16
	s := unionfind.New(n)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		s.Union(i&(n-1), (i+1)&(n-1))
	}
	sink = s.Count()
}

func BenchmarkUnionFindUnionMerge(b *testing.B) {
	const n = 1 << 12
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		s := unionfind.New(n)
		for j := 1; j < n; j++ {
			s.Union(j, j-1)
		}
		sink = s.Count()
	}
}

func BenchmarkUnionFindFind(b *testing.B) {
	const n = 1 << 14
	s := unionfind.New(n)
	for i := 1; i < n; i++ {
		s.Union(i, i-1)
	}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		sink = s.Find(i & (n - 1))
	}
}

func BenchmarkUnionFindConnectedHit(b *testing.B) {
	const n = 1 << 13
	s := unionfind.New(n)
	for i := 1; i < n; i++ {
		s.Union(i, i-1)
	}
	hit := false
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		hit = s.Connected(i&(n-1), (i*7)&(n-1))
	}
	sink = hit
}

func BenchmarkUnionFindConnectedMiss(b *testing.B) {
	const n = 1 << 13
	s := unionfind.New(n)
	for i := 0; i < n/2; i++ {
		s.Union(2*i, 2*i+1) // only pairs are connected
	}
	hit := true
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		hit = s.Connected(i&(n-1), (i+(n/2))&(n-1))
	}
	sink = hit
}

func BenchmarkUnionFindSize(b *testing.B) {
	const n = 1 << 13
	s := unionfind.New(n)
	for i := 1; i < n; i++ {
		s.Union(i, i-1)
	}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		sink = s.Size(i & (n - 1))
	}
}

func BenchmarkUnionFindComponents(b *testing.B) {
	const n = 1 << 10
	s := unionfind.New(n)
	for i := 2; i < n; i += 2 {
		s.Union(i, i-1)
	}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		sink = len(s.Components())
	}
}
