package pqueue_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/neko233-com/go-data-struct-lib/pqueue"
)

func TestMinQueueDefaultOrder(t *testing.T) {
	q := pqueue.New[int]()
	if _, ok := q.Pop(); ok {
		t.Fatal("Pop on empty queue should miss")
	}
	if _, ok := q.Peek(); ok {
		t.Fatal("Peek on empty queue should miss")
	}

	items := make([]int, 1000)
	rng := rand.New(rand.NewPCG(7, 7))
	for i := range items {
		items[i] = rng.IntN(100000)
		q.Push(items[i])
	}
	if q.Len() != len(items) {
		t.Fatalf("Len = %d; want %d", q.Len(), len(items))
	}
	if k, ok := q.Peek(); !ok || k != slices.Min(items) {
		t.Fatalf("Peek = %d, %v; want %d", k, ok, slices.Min(items))
	}

	slices.Sort(items)
	for i, want := range items {
		if got, ok := q.Pop(); !ok || got != want {
			t.Fatalf("Pop #%d = %d, %v; want %d", i, got, ok, want)
		}
	}
	if q.Len() != 0 {
		t.Fatal("queue should be empty after draining")
	}
}

func TestMaxQueueCustomComparator(t *testing.T) {
	q := pqueue.NewFunc(func(a, b int) int { return b - a })
	for _, v := range []int{5, 1, 9, 3} {
		q.Push(v)
	}
	want := []int{9, 5, 3, 1}
	var got []int
	for {
		v, ok := q.Pop()
		if !ok {
			break
		}
		got = append(got, v)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("descending pops = %v; want %v", got, want)
	}
}

func TestNewFromSlice(t *testing.T) {
	src := []int{9, 4, 7, 1, 8}
	q := pqueue.NewFromSlice(src, func(a, b int) int { return a - b })

	var got []int
	for {
		v, ok := q.Pop()
		if !ok {
			break
		}
		got = append(got, v)
	}
	if !slices.Equal(got, []int{1, 4, 7, 8, 9}) {
		t.Fatalf("heapify pops = %v", got)
	}
	// the input slice must not be reordered by heap construction
	if !slices.Equal(src, []int{9, 4, 7, 1, 8}) {
		t.Fatalf("input slice mutated: %v", src)
	}
}

func TestClear(t *testing.T) {
	q := pqueue.New[int]()
	q.Push(1)
	q.Push(2)
	q.Clear()
	if q.Len() != 0 {
		t.Fatalf("Len after Clear = %d", q.Len())
	}
	if _, ok := q.Pop(); ok {
		t.Fatal("Pop after Clear should miss")
	}
	if _, ok := q.Peek(); ok {
		t.Fatal("Peek after Clear should miss")
	}
	q.Push(3)
	if v, ok := q.Peek(); !ok || v != 3 {
		t.Fatal("queue unusable after Clear")
	}
}

func TestToSliceIsIndependent(t *testing.T) {
	q := pqueue.New[int]()
	q.Push(1)
	s := q.ToSlice()
	s[0] = 99
	if v, _ := q.Peek(); v != 1 {
		t.Fatal("ToSlice should return an independent copy")
	}
}

func TestNewFuncPanicsOnNilComparator(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewFunc with nil comparator should panic")
		}
	}()
	pqueue.NewFunc[int](nil)
}
