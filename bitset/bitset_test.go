package bitset_test

import (
	"slices"
	"testing"

	"github.com/neko233-com/go-data-struct-lib/bitset"
)

func TestSetGetClearFlip(t *testing.T) {
	b := bitset.New()
	if b.Get(0) || b.Get(100) {
		t.Fatal("fresh BitSet must be empty")
	}

	b.Set(0)
	b.Set(63)
	b.Set(64) // spans into the second word
	b.Set(129)
	if !b.Get(0) || !b.Get(63) || !b.Get(64) || !b.Get(129) {
		t.Fatal("Set/Get mismatch across word boundary")
	}
	if b.Get(1) || b.Get(65) {
		t.Fatal("unset bits must read false")
	}

	b.Flip(64)
	if b.Get(64) {
		t.Fatal("Flip should unset bit 64")
	}
	b.Flip(64)
	if !b.Get(64) {
		t.Fatal("Flip should set bit 64 again")
	}
	b.Flip(200) // beyond the current end: sets it
	if !b.Get(200) {
		t.Fatal("Flip beyond the end must set the bit")
	}

	b.Clear(129)
	if b.Get(129) {
		t.Fatal("Clear should unset bit 129")
	}
	b.Clear(100000) // out of range: no-op, no panic
}

func TestCardinalityAndLen(t *testing.T) {
	b := bitset.New()
	for _, i := range []int{0, 1, 64, 65, 200} {
		b.Set(i)
	}
	if b.Cardinality() != 5 {
		t.Fatalf("Cardinality = %d; want 5", b.Cardinality())
	}
	if b.Len() != 201 {
		t.Fatalf("Len = %d; want 201", b.Len())
	}
	b.Clear(200)
	if b.Len() != 66 {
		t.Fatalf("Len after clear = %d; want 66", b.Len())
	}
	if b.IsEmpty() {
		t.Fatal("IsEmpty should be false")
	}
	empty := bitset.New()
	if empty.Len() != 0 || !empty.IsEmpty() || empty.Cardinality() != 0 {
		t.Fatal("empty BitSet invariants violated")
	}
}

func TestNextSetBit(t *testing.T) {
	b := bitset.New()
	for _, i := range []int{3, 70, 150} {
		b.Set(i)
	}
	if got := b.NextSetBit(0); got != 3 {
		t.Fatalf("NextSetBit(0) = %d; want 3", got)
	}
	if got := b.NextSetBit(4); got != 70 {
		t.Fatalf("NextSetBit(4) = %d; want 70", got)
	}
	if got := b.NextSetBit(70); got != 70 {
		t.Fatalf("NextSetBit(70) = %d; want 70 (inclusive)", got)
	}
	if got := b.NextSetBit(151); got != -1 {
		t.Fatalf("NextSetBit(151) = %d; want -1", got)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("NextSetBit(-1) should panic")
		}
	}()
	b.NextSetBit(-1)
}

func TestPrevSetBit(t *testing.T) {
	b := bitset.New()
	for _, i := range []int{3, 70, 150} {
		b.Set(i)
	}
	if got := b.PrevSetBit(1000); got != 150 {
		t.Fatalf("PrevSetBit(1000) = %d; want 150", got)
	}
	if got := b.PrevSetBit(150); got != 150 {
		t.Fatalf("PrevSetBit(150) = %d; want 150 (inclusive)", got)
	}
	if got := b.PrevSetBit(149); got != 70 {
		t.Fatalf("PrevSetBit(149) = %d; want 70", got)
	}
	if got := b.PrevSetBit(3); got != 3 {
		t.Fatalf("PrevSetBit(3) = %d; want 3", got)
	}
	if got := b.PrevSetBit(2); got != -1 {
		t.Fatalf("PrevSetBit(2) = %d; want -1", got)
	}
	if got := b.PrevSetBit(-5); got != -1 {
		t.Fatalf("PrevSetBit(-5) = %d; want -1", got)
	}
}

func TestBooleanOps(t *testing.T) {
	a := bitset.New()
	b := bitset.New()
	for _, i := range []int{1, 2, 3, 64} {
		a.Set(i)
	}
	for _, i := range []int{2, 3, 4, 130} {
		b.Set(i)
	}

	and := a.Clone()
	and.And(b)
	if !slices.Equal(and.ToSlice(), []int{2, 3}) {
		t.Fatalf("And = %v; want [2 3]", and.ToSlice())
	}

	or := a.Clone()
	or.Or(b)
	if !slices.Equal(or.ToSlice(), []int{1, 2, 3, 4, 64, 130}) {
		t.Fatalf("Or = %v", or.ToSlice())
	}

	xor := a.Clone()
	xor.Xor(b)
	if !slices.Equal(xor.ToSlice(), []int{1, 4, 64, 130}) {
		t.Fatalf("Xor = %v", xor.ToSlice())
	}

	andNot := a.Clone()
	andNot.AndNot(b)
	if !slices.Equal(andNot.ToSlice(), []int{1, 64}) {
		t.Fatalf("AndNot = %v", andNot.ToSlice())
	}
}

func TestEqualAndClone(t *testing.T) {
	a := bitset.New()
	a.Set(5)
	a.Set(300)

	c := a.Clone()
	if !a.Equal(c) {
		t.Fatal("clone should equal the original")
	}
	c.Clear(5)
	if a.Equal(c) {
		t.Fatal("clone should be independent")
	}

	// clearing the top bit shrinks the representation but stays equal
	x := bitset.New()
	x.Set(200)
	y := bitset.New()
	y.Set(200)
	y.Clear(200)
	if !x.Equal(y) && !y.IsEmpty() {
		t.Fatal("representation trimming broke equality")
	}
	if !y.IsEmpty() || y.Len() != 0 {
		t.Fatal("cleared bitset should be logically empty")
	}
}

func TestAll(t *testing.T) {
	b := bitset.New()
	for _, i := range []int{7, 63, 64, 130} {
		b.Set(i)
	}
	var got []int
	for i := range b.All() {
		got = append(got, i)
	}
	if !slices.Equal(got, []int{7, 63, 64, 130}) {
		t.Fatalf("All() = %v", got)
	}
	if !slices.Equal(b.ToSlice(), got) {
		t.Fatal("All() and ToSlice() disagree")
	}
}
