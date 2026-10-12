package order_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/order"
)

func BenchmarkNatural(b *testing.B) {
	cmp := order.Natural[int]()

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		cmp(i, i+1)
	}
}

func BenchmarkReverse(b *testing.B) {
	cmp := order.Reverse(order.Natural[int]())

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		cmp(i, i+1)
	}
}

func BenchmarkThenOfTwo(b *testing.B) {
	byMod3 := order.ByKey(func(v int) int { return v % 3 }, order.Natural[int]())
	byMod7 := order.ByKey(func(v int) int { return v % 7 }, order.Natural[int]())
	cmp := order.Then(byMod3, byMod7)

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		cmp(i, i+1)
	}
}

func BenchmarkSortedInts(b *testing.B) {
	items := make([]int, 1024)
	for i := range items {
		items[i] = (i * 7919) % 1024
	}
	cmp := order.Natural[int]()

	b.ReportAllocs()
	for b.Loop() {
		order.Sorted(cmp, items)
	}
}

func BenchmarkIsSorted(b *testing.B) {
	items := make([]int, 1024)
	for i := range items {
		items[i] = i
	}
	cmp := order.Natural[int]()

	b.ReportAllocs()
	for b.Loop() {
		order.IsSorted(cmp, items)
	}
}

func BenchmarkFluentChain(b *testing.B) {
	type player struct {
		name  string
		score int
	}
	byNameThenScore := order.Then(
		order.ByKey(func(p player) string { return p.name }, order.Natural[string]()),
		order.Reverse(order.ByKey(func(p player) int { return p.score }, order.Natural[int]())),
	)

	items := make([]player, 64)
	for i := range items {
		items[i] = player{name: string(rune('a' + i%26)), score: i}
	}

	b.ReportAllocs()
	for b.Loop() {
		order.Sorted(byNameThenScore, items)
	}
}
