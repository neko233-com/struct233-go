package deque_test

import (
	"testing"

	"github.com/neko233-com/go-data-struct-lib/deque"
)

func BenchmarkDequePushBackPopFront(b *testing.B) {
	d := deque.New[int](1024)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		d.PushBack(i)
		d.PopFront()
	}
}

func BenchmarkDequePushFrontPopBack(b *testing.B) {
	d := deque.New[int](1024)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		d.PushFront(i)
		d.PopBack()
	}
}

func BenchmarkDequePushBack(b *testing.B) {
	b.ReportAllocs()
	d := deque.New[int](b.N + 1)
	for i := 0; b.Loop(); i++ {
		d.PushBack(i)
	}
}

func BenchmarkDequeToSlice(b *testing.B) {
	d := deque.New[int](1024)
	for i := range 1024 {
		d.PushBack(i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		d.ToSlice()
	}
}
