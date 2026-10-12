package text_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/text"
)

func BenchmarkJoinSlice(b *testing.B) {
	parts := []string{"alpha", "beta", "gamma", "delta", "epsilon"}

	b.ReportAllocs()
	for b.Loop() {
		text.JoinSlice(",", parts)
	}
}

func BenchmarkJoinerSkipEmpty(b *testing.B) {
	j := text.NewJoiner(",").SkipEmpty()
	parts := []string{"alpha", "", "beta", "", "gamma"}

	b.ReportAllocs()
	for b.Loop() {
		j.JoinSlice(parts)
	}
}

func BenchmarkSplitterTrimOmitEmpty(b *testing.B) {
	s := text.NewSplitter(",").Trim().OmitEmpty()
	const in = " alpha , , beta , gamma "

	b.ReportAllocs()
	for b.Loop() {
		s.Split(in)
	}
}

func BenchmarkSplitterLimit(b *testing.B) {
	s := text.NewSplitter(",").Limit(3)
	const in = "alpha,beta,gamma,delta,epsilon"

	b.ReportAllocs()
	for b.Loop() {
		s.Split(in)
	}
}

func BenchmarkToSnakeCase(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		text.ToSnakeCase("HTTPServerUserID")
	}
}

func BenchmarkPadStart(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		text.PadStart("42", 8, '0')
	}
}
