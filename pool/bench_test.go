package pool_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/pool"
)

func BenchmarkGetPut(b *testing.B) {
	p := pool.New(func() *item { return &item{} }, pool.WithMaxSize(1024))
	p.Prealloc(1)

	b.ReportAllocs()
	for b.Loop() {
		p.Put(p.Get())
	}
}

func BenchmarkGetMiss(b *testing.B) {
	// unbounded pool that always has to call the factory
	p := pool.New(func() *item { return &item{} })
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		p.Get()
	}
}

func BenchmarkParallelGetPut(b *testing.B) {
	p := pool.New(func() *item { return &item{} }, pool.WithMaxSize(64))
	p.Prealloc(64)

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p.Put(p.Get())
		}
	})
}

func BenchmarkPrealloc(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		p := pool.New(func() *item { return &item{} })
		p.Prealloc(16)
	}
}
