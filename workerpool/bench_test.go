package workerpool_test

import (
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/workerpool"
)

func BenchmarkSubmit(b *testing.B) {
	p := workerpool.New(4, workerpool.WithQueueSize(1024))
	defer p.Close()

	b.ReportAllocs()
	for b.Loop() {
		if err := p.Submit(func() {}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSubmitParallel(b *testing.B) {
	p := workerpool.New(runtime.GOMAXPROCS(0), workerpool.WithQueueSize(1024))
	defer p.Close()

	var failed atomic.Int64
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := p.Submit(func() {}); err != nil {
				failed.Add(1)
				return
			}
		}
	})
	b.StopTimer()

	require.Zero(b, failed.Load(), "Submit must not fail before Close")
}
