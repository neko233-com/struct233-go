package lru_test

import (
	"testing"

	"github.com/neko233-com/go-data-struct-lib/lru"
)

func benchLRU(n int) *lru.LRU[int, int] {
	c := lru.New[int, int](n)
	for i := range n {
		c.Put(i, i)
	}
	return c
}

func BenchmarkLRUGet(b *testing.B) {
	c := benchLRU(4096)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		c.Get(i & 4095)
	}
}

func BenchmarkLRUPeek(b *testing.B) {
	c := benchLRU(4096)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		c.Peek(i & 4095)
	}
}

func BenchmarkLRUPutUpdate(b *testing.B) {
	c := benchLRU(4096)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		c.Put(i&4095, i)
	}
}

func BenchmarkLRUPutInsertEvict(b *testing.B) {
	c := benchLRU(4096)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		c.Put(i+4096, i)
	}
}
