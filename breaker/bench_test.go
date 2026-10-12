package breaker_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/breaker"
)

// BenchmarkAllowClosed measures the admit path of a closed circuit, where no
// probe bookkeeping or refusal happens.
func BenchmarkAllowClosed(b *testing.B) {
	br := breaker.New()

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if !br.Allow() {
			b.Fatal("a closed circuit must admit every call")
		}
	}
}

// BenchmarkExecuteSuccess measures the whole admit -> run -> record cycle.
func BenchmarkExecuteSuccess(b *testing.B) {
	br := breaker.New()
	fn := func() error { return nil }

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if err := br.Execute(fn); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExecuteParallel measures contention on the breaker mutex.
func BenchmarkExecuteParallel(b *testing.B) {
	br := breaker.New()
	fn := func() error { return nil }

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := br.Execute(fn); err != nil {
				// b.Error is goroutine-safe, b.Fatal must not be used here
				b.Error(err)
			}
		}
	})
}
