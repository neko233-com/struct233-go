package uuid_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/uuid"
)

// text keeps the formatted UUID alive across benchmark iterations.
var text string

func BenchmarkNewV4(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sink = uuid.NewV4()
	}
}

func BenchmarkNewV7(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sink = uuid.NewV7()
	}
}

func BenchmarkNewV7Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sink = uuid.NewV7()
		}
	})
}

func BenchmarkString(b *testing.B) {
	u := uuid.NewV4()
	b.ReportAllocs()
	for b.Loop() {
		text = u.String()
	}
}

func BenchmarkAppendText(b *testing.B) {
	u := uuid.NewV4()
	buf := make([]byte, 0, 64)
	b.ReportAllocs()
	for b.Loop() {
		buf, _ = u.AppendText(buf[:0])
	}
}

func BenchmarkParse(b *testing.B) {
	text := uuid.NewV4().String()
	b.ReportAllocs()
	for b.Loop() {
		sink, _ = uuid.Parse(text)
	}
}
