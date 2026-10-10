package bitset_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/go-data-struct-lib/bitset"
)

func TestSetGetClearFlip(t *testing.T) {
	b := bitset.New()
	require.False(t, b.Get(0), "fresh BitSet must be empty")
	require.False(t, b.Get(100))

	b.Set(0)
	b.Set(63)
	b.Set(64) // spans into the second word
	b.Set(129)
	require.True(t, b.Get(0))
	require.True(t, b.Get(63))
	require.True(t, b.Get(64))
	require.True(t, b.Get(129))
	require.False(t, b.Get(1))
	require.False(t, b.Get(65))

	b.Flip(64)
	require.False(t, b.Get(64), "Flip must unset a set bit")
	b.Flip(64)
	require.True(t, b.Get(64), "Flip must set a clear bit")
	b.Flip(200)
	require.True(t, b.Get(200), "Flip beyond the end must set the bit")

	b.Clear(129)
	require.False(t, b.Get(129))

	require.NotPanics(t, func() { b.Clear(100000) }, "clearing beyond the end must be a no-op")
}

func TestCardinalityAndLen(t *testing.T) {
	b := bitset.New()
	for _, i := range []int{0, 1, 64, 65, 200} {
		b.Set(i)
	}
	require.Equal(t, 5, b.Cardinality())
	require.Equal(t, 201, b.Len())

	b.Clear(200)
	require.Equal(t, 66, b.Len(), "Len must follow the highest set bit")
	require.False(t, b.IsEmpty())

	empty := bitset.New()
	require.Zero(t, empty.Len())
	require.True(t, empty.IsEmpty())
	require.Zero(t, empty.Cardinality())
}

func TestNextSetBit(t *testing.T) {
	b := bitset.New()
	for _, i := range []int{3, 70, 150} {
		b.Set(i)
	}

	require.Equal(t, 3, b.NextSetBit(0))
	require.Equal(t, 70, b.NextSetBit(4))
	require.Equal(t, 70, b.NextSetBit(70), "NextSetBit is inclusive")
	require.Equal(t, -1, b.NextSetBit(151))

	require.Panics(t, func() { b.NextSetBit(-1) }, "negative index must panic")
}

func TestPrevSetBit(t *testing.T) {
	b := bitset.New()
	for _, i := range []int{3, 70, 150} {
		b.Set(i)
	}

	require.Equal(t, 150, b.PrevSetBit(1000))
	require.Equal(t, 150, b.PrevSetBit(150), "PrevSetBit is inclusive")
	require.Equal(t, 70, b.PrevSetBit(149))
	require.Equal(t, 3, b.PrevSetBit(3))
	require.Equal(t, -1, b.PrevSetBit(2))
	require.Equal(t, -1, b.PrevSetBit(-5))

	require.Equal(t, -1, bitset.New().PrevSetBit(0), "empty BitSet must return -1")
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
	require.Equal(t, []int{2, 3}, and.ToSlice())

	or := a.Clone()
	or.Or(b)
	require.Equal(t, []int{1, 2, 3, 4, 64, 130}, or.ToSlice())

	xor := a.Clone()
	xor.Xor(b)
	require.Equal(t, []int{1, 4, 64, 130}, xor.ToSlice())

	andNot := a.Clone()
	andNot.AndNot(b)
	require.Equal(t, []int{1, 64}, andNot.ToSlice())

	// operands must stay untouched
	require.Equal(t, []int{1, 2, 3, 64}, a.ToSlice())
	require.Equal(t, []int{2, 3, 4, 130}, b.ToSlice())
}

func TestBooleanOpsOnEmptyOperand(t *testing.T) {
	a := bitset.New()
	a.Set(5)

	empty := bitset.New()
	and := a.Clone()
	and.And(empty)
	require.True(t, and.IsEmpty())

	or := a.Clone()
	or.Or(empty)
	require.Equal(t, []int{5}, or.ToSlice())

	xor := a.Clone()
	xor.Xor(empty)
	require.Equal(t, []int{5}, xor.ToSlice())

	andNot := a.Clone()
	andNot.AndNot(empty)
	require.Equal(t, []int{5}, andNot.ToSlice())
}

func TestEqualAndClone(t *testing.T) {
	a := bitset.New()
	a.Set(5)
	a.Set(300)

	c := a.Clone()
	require.True(t, a.Equal(c), "clone must equal the original")

	c.Clear(5)
	require.False(t, a.Equal(c), "clone must be independent")

	x := bitset.New()
	x.Set(200)
	y := bitset.New()
	y.Set(200)
	y.Clear(200)
	require.True(t, y.IsEmpty())
	require.Zero(t, y.Len(), "trailing zero words must be trimmed")
	require.False(t, x.Equal(y))

	empty := bitset.New()
	require.True(t, empty.Equal(bitset.New()))
	require.False(t, empty.Equal(x))
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
	require.Equal(t, []int{7, 63, 64, 130}, got)
	require.Equal(t, got, b.ToSlice(), "All and ToSlice must agree")

	count := 0
	for range b.All() {
		count++
		break
	}
	require.Equal(t, 1, count, "break must stop the iteration")
}

func TestToSliceCapHint(t *testing.T) {
	b := bitset.New()
	require.Empty(t, b.ToSlice())

	for i := range 10 {
		b.Set(i * 64)
	}
	require.Len(t, b.ToSlice(), 10)
	require.True(t, slices.Equal([]int{0, 64, 128, 192, 256, 320, 384, 448, 512, 576}, b.ToSlice()))
}

func TestReadsDoNotAllocate(t *testing.T) {
	b := bitset.New()
	b.Set(1 << 12) // grow once

	allocs := testing.AllocsPerRun(100, func() { b.Get(1 << 11) })
	require.Zero(t, allocs, "Get must not allocate")

	allocs = testing.AllocsPerRun(100, func() { b.Set(1 << 10) })
	require.Zero(t, allocs, "Set within the existing words must not allocate")
}
