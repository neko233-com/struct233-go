package multimap_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/multimap"
)

// benchList builds a ListMultimap holding valuesPerKey values for each of the
// keySpan keys.
func benchList(keySpan, valuesPerKey int) *multimap.ListMultimap[int, int] {
	m := multimap.NewListWithCapacity[int, int](keySpan)
	for k := range keySpan {
		for v := range valuesPerKey {
			m.Put(k, v)
		}
	}
	return m
}

// benchSet builds a SetMultimap holding valuesPerKey values for each of the
// keySpan keys.
func benchSet(keySpan, valuesPerKey int) *multimap.SetMultimap[int, int] {
	m := multimap.NewSetWithCapacity[int, int](keySpan)
	for k := range keySpan {
		for v := range valuesPerKey {
			m.Put(k, v)
		}
	}
	return m
}

func BenchmarkListPut(b *testing.B) {
	m := multimap.NewList[int, int]()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		m.Put(i%1024, i)
	}
}

func BenchmarkListPutAll(b *testing.B) {
	m := multimap.NewList[int, int]()
	values := []int{1, 2, 3, 4}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		m.PutAll(i%1024, values...)
	}
}

func BenchmarkListGet(b *testing.B) {
	m := benchList(1024, 8)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = m.Get(i % 1024)
	}
}

func BenchmarkListContainsEntry(b *testing.B) {
	m := benchList(1024, 8)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if !m.ContainsEntry(i%1024, i%8) {
			b.Fatal("the benchmark lost an entry")
		}
	}
}

func BenchmarkListEntries(b *testing.B) {
	m := benchList(1024, 8)
	b.ReportAllocs()
	for b.Loop() {
		if len(m.Entries()) != 8192 {
			b.Fatal("the benchmark lost an entry")
		}
	}
}

func BenchmarkListInvert(b *testing.B) {
	m := benchList(1024, 8)
	b.ReportAllocs()
	for b.Loop() {
		if m.Invert().Size() != 8192 {
			b.Fatal("the inverted multimap lost an entry")
		}
	}
}

func BenchmarkListRemoveAll(b *testing.B) {
	m := benchList(1024, 8)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		k := i % 1024
		if m.RemoveAll(k) == nil {
			b.Fatal("the benchmark lost a key")
		}
		m.PutAll(k, 0, 1, 2, 3, 4, 5, 6, 7)
	}
}

func BenchmarkSetPut(b *testing.B) {
	m := multimap.NewSet[int, int]()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		m.Put(i%1024, i%16)
	}
}

func BenchmarkSetPutAll(b *testing.B) {
	m := multimap.NewSet[int, int]()
	values := []int{1, 2, 3, 4}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		m.PutAll(i%1024, values...)
	}
}

func BenchmarkSetGet(b *testing.B) {
	m := benchSet(1024, 8)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = m.Get(i % 1024)
	}
}

func BenchmarkSetContainsEntry(b *testing.B) {
	m := benchSet(1024, 8)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if !m.ContainsEntry(i%1024, i%8) {
			b.Fatal("the benchmark lost an entry")
		}
	}
}

func BenchmarkSetInvert(b *testing.B) {
	m := benchSet(1024, 8)
	b.ReportAllocs()
	for b.Loop() {
		if m.Invert().Size() != 8192 {
			b.Fatal("the inverted multimap lost an entry")
		}
	}
}
