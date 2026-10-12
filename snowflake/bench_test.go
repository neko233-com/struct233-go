package snowflake_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/snowflake"
)

func BenchmarkGeneratorNext(b *testing.B) {
	g := snowflake.New(1)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		g.MustNext()
	}
}

func BenchmarkGeneratorParallelNext(b *testing.B) {
	g := snowflake.New(1)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			g.MustNext()
		}
	})
}
