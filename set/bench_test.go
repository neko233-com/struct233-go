package set_test

import (
	"testing"

	"github.com/neko233-com/go-data-struct-lib/set"
)

func BenchmarkHashSetAdd(b *testing.B) {
	b.ReportAllocs()
	s := set.NewHashSetWithCapacity[int](b.N)
	for i := 0; b.Loop(); i++ {
		s.Add(i)
	}
}

func BenchmarkHashSetContains(b *testing.B) {
	s := set.NewHashSetWithCapacity[int](1024)
	for i := range 1024 {
		s.Add(i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		s.Contains(i & 1023)
	}
}

func BenchmarkHashSetUnion(b *testing.B) {
	a := set.NewHashSetWithCapacity[int](512)
	c := set.NewHashSetWithCapacity[int](512)
	for i := range 512 {
		a.Add(i)
		c.Add(i + 256)
	}

	b.ReportAllocs()
	for b.Loop() {
		a.Union(c)
	}
}

func BenchmarkTreeSetAdd(b *testing.B) {
	b.ReportAllocs()
	ts := set.NewTreeSet[int]()
	for i := 0; b.Loop(); i++ {
		ts.Add(i)
	}
}

func BenchmarkTreeSetContains(b *testing.B) {
	ts := set.NewTreeSet[int]()
	for i := range 1024 {
		ts.Add(i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		ts.Contains(i & 1023)
	}
}

func BenchmarkLinkedHashSetAdd(b *testing.B) {
	b.ReportAllocs()
	s := set.NewLinkedHashSet[int]()
	for i := 0; b.Loop(); i++ {
		s.Add(i)
	}
}
