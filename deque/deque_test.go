package deque_test

import (
	"slices"
	"testing"

	"github.com/neko233-com/go-data-struct-lib/deque"
)

func TestFIFO(t *testing.T) {
	d := deque.New[int]()
	for i := range 5 {
		d.PushBack(i)
	}
	for i := range 5 {
		if v, ok := d.PopFront(); !ok || v != i {
			t.Fatalf("PopFront #%d = %d, %v; want %d", i, v, ok, i)
		}
	}
	if _, ok := d.PopFront(); ok {
		t.Fatal("PopFront on empty deque should miss")
	}
}

func TestLIFO(t *testing.T) {
	d := deque.New[string]()
	d.PushBack("a")
	d.PushBack("b")
	d.PushBack("c")
	if got, ok := d.PopBack(); !ok || got != "c" {
		t.Fatalf("PopBack = %q, %v; want \"c\"", got, ok)
	}
	if got, ok := d.PopBack(); !ok || got != "b" {
		t.Fatalf("PopBack = %q, %v; want \"b\"", got, ok)
	}
}

func TestBothEnds(t *testing.T) {
	d := deque.New[int]()
	d.PushFront(2)
	d.PushFront(1)
	d.PushBack(3)
	if got := d.ToSlice(); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("ToSlice = %v; want [1 2 3]", got)
	}
	if v, ok := d.Front(); !ok || v != 1 {
		t.Fatalf("Front = %d, %v", v, ok)
	}
	if v, ok := d.Back(); !ok || v != 3 {
		t.Fatalf("Back = %d, %v", v, ok)
	}
	if d.Len() != 3 {
		t.Fatalf("Len = %d", d.Len())
	}
}

func TestGrowth(t *testing.T) {
	d := deque.New[int]() // starts at minCapacity
	for i := range 1000 {
		d.PushFront(i)
		d.PushBack(i)
	}
	if d.Len() != 2000 {
		t.Fatalf("Len = %d; want 2000", d.Len())
	}
	want := make([]int, 0, 2000)
	for i := 999; i >= 0; i-- {
		want = append(want, i)
	}
	for i := range 1000 {
		want = append(want, i)
	}
	if got := d.ToSlice(); !slices.Equal(got, want) {
		t.Fatalf("ToSlice mismatch after growth (len %d)", len(got))
	}
	// interleaved pops must mirror the ends
	for i := range 500 {
		front, okF := d.PopFront()
		back, okB := d.PopBack()
		if !okF || !okB {
			t.Fatalf("pop %d: okF=%v okB=%v", i, okF, okB)
		}
		if front != 999-i || back != 999-i {
			t.Fatalf("pop %d: front=%d back=%d; want %d", i, front, back, 999-i)
		}
	}
	if d.Len() != 1000 {
		t.Fatalf("Len = %d; want 1000", d.Len())
	}
}

func TestEmptyAccessors(t *testing.T) {
	d := deque.New[int]()
	if _, ok := d.Front(); ok {
		t.Fatal("Front on empty should miss")
	}
	if _, ok := d.Back(); ok {
		t.Fatal("Back on empty should miss")
	}
	if _, ok := d.PopBack(); ok {
		t.Fatal("PopBack on empty should miss")
	}
}

func TestClearAndReuse(t *testing.T) {
	d := deque.New[int](64)
	d.PushBack(1)
	d.PushFront(0)
	capBefore := d.Cap()
	d.Clear()
	if d.Len() != 0 || d.Cap() != capBefore {
		t.Fatalf("Len=%d Cap=%d after Clear; want 0, %d", d.Len(), d.Cap(), capBefore)
	}
	d.PushBack(7)
	if v, ok := d.PopFront(); !ok || v != 7 {
		t.Fatalf("PopFront after Clear = %d, %v", v, ok)
	}
}

func TestAll(t *testing.T) {
	d := deque.New[int]()
	d.PushBack(1)
	d.PushBack(2)
	d.PushBack(3)
	var got []int
	for v := range d.All() {
		got = append(got, v)
	}
	if !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("All() = %v", got)
	}
}
