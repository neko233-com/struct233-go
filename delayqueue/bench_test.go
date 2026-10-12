package delayqueue_test

import (
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/delayqueue"
)

func BenchmarkPutTryTake(b *testing.B) {
	q := delayqueue.New[int]()
	e := delayqueue.NewElement(1, time.Now())

	b.ReportAllocs()
	for b.Loop() {
		q.Put(e)
		q.TryTake()
	}
}

func BenchmarkPeek(b *testing.B) {
	q := delayqueue.New[int]()
	for i := range 128 {
		q.Put(delayqueue.NewElement(i, time.Now().Add(time.Duration(i)*time.Second)))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		q.Peek()
	}
}

func BenchmarkPutWithSystemClock(b *testing.B) {
	q := delayqueue.New[int]()
	deadline := time.Now()

	b.ReportAllocs()
	for b.Loop() {
		q.Put(delayqueue.NewElement(1, deadline))
		q.Clear()
	}
}
