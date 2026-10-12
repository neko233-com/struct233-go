package verify_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/verify"
)

// sink keeps benchmark results alive so the compiler cannot drop the work.
var sink *int

// passing benches measure the documented zero-allocation check path; the
// failing benches measure the panic path including the recovered payload.

func BenchmarkVerifyTruePass(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		verify.True(true, "condition must hold")
	}
}

func BenchmarkVerifyTruePassArgs(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		verify.True(true, "value %d is in range %d..%d", 3, 0, 5)
	}
}

func BenchmarkVerifyStatePass(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		verify.State(true, "server is running")
	}
}

func BenchmarkVerifyNotNilPass(b *testing.B) {
	p := new(int)
	b.ReportAllocs()
	for b.Loop() {
		sink = verify.NotNil(p, "pointer must not be nil")
	}
}

func BenchmarkVerifyIndexPass(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		verify.Index(i&7, 8, "index %d out of [0, 8)", i&7)
	}
}

func BenchmarkVerifyInRangePass(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		verify.InRange(i&255, 0, 255, "value %d not in [0, 255]", i&255)
	}
}

func BenchmarkVerifyTrueFail(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		func() {
			defer func() { _ = recover() }()
			verify.True(false, "value %d is bad", -1)
		}()
	}
}

func BenchmarkVerifyIndexFail(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		func() {
			defer func() { _ = recover() }()
			verify.Index(-1, 3, "index %d out of [0, 3)", -1)
		}()
	}
}
