package barrier_test

import (
	"context"
	"testing"

	"github.com/neko233-com/struct233-go/barrier"
)

// BenchmarkBarrierAwait measures the completing path: the last arrival resets
// the generation and returns without blocking.
func BenchmarkBarrierAwait(b *testing.B) {
	br := barrier.New(1)
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if _, err := br.Await(ctx); err != nil {
			b.Fatal(err)
		}
	}
}
