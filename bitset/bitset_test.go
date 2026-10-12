package bitset_test

import (
	"fmt"
	"math/big"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/bitset"
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

	require.PanicsWithValue(t, "bitset: negative index", func() { b.NextSetBit(-1) }, "negative index must panic")
	require.PanicsWithValue(t, "bitset: negative index", func() { b.NextSetBit(-100) })
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

	allocs = testing.AllocsPerRun(100, func() { b.Cardinality() })
	require.Zero(t, allocs, "Cardinality must not allocate")
}

// TestBitSet_TableDriven covers every exported method through normal,
// boundary and empty cases in one table.
func TestBitSet_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"get on fresh set is false", func(t *testing.T) {
			b := bitset.New()
			require.False(t, b.Get(0))
			require.False(t, b.Get(63))
			require.False(t, b.Get(64))
		}},
		{"get beyond end is false", func(t *testing.T) {
			b := bitset.New()
			b.Set(0)
			require.False(t, b.Get(1<<20))
		}},
		{"set bit zero", func(t *testing.T) {
			b := bitset.New()
			b.Set(0)
			require.True(t, b.Get(0))
			require.Equal(t, 1, b.Len())
			require.Equal(t, 1, b.Cardinality())
		}},
		{"set bit 63 stays in first word", func(t *testing.T) {
			b := bitset.New()
			b.Set(63)
			require.True(t, b.Get(63))
			require.False(t, b.Get(62))
			require.Equal(t, 64, b.Len())
		}},
		{"set bit 64 crosses the word boundary", func(t *testing.T) {
			b := bitset.New()
			b.Set(64)
			require.True(t, b.Get(64))
			require.False(t, b.Get(63))
			require.Equal(t, 65, b.Len())
		}},
		{"set same bit twice keeps cardinality one", func(t *testing.T) {
			b := bitset.New()
			b.Set(10)
			b.Set(10)
			require.Equal(t, 1, b.Cardinality())
		}},
		{"clear unset bit is a no-op", func(t *testing.T) {
			b := bitset.New()
			b.Clear(5)
			require.False(t, b.Get(5))
			require.Zero(t, b.Cardinality())
		}},
		{"clear beyond end is a no-op", func(t *testing.T) {
			b := bitset.New()
			b.Set(3)
			b.Clear(1 << 16)
			require.Equal(t, 1, b.Cardinality())
		}},
		{"clear trims trailing words", func(t *testing.T) {
			b := bitset.New()
			b.Set(1000)
			b.Clear(1000)
			require.Zero(t, b.Len(), "cleared tail must shrink the logical size")
			require.True(t, b.IsEmpty())
		}},
		{"flip sets a clear bit", func(t *testing.T) {
			b := bitset.New()
			b.Flip(7)
			require.True(t, b.Get(7))
		}},
		{"flip clears a set bit", func(t *testing.T) {
			b := bitset.New()
			b.Set(7)
			b.Flip(7)
			require.False(t, b.Get(7))
		}},
		{"flip beyond end sets the bit", func(t *testing.T) {
			b := bitset.New()
			b.Flip(200)
			require.True(t, b.Get(200))
			require.Equal(t, 201, b.Len())
		}},
		{"flip twice is identity", func(t *testing.T) {
			b := bitset.New()
			b.Set(42)
			b.Flip(42)
			b.Flip(42)
			require.True(t, b.Get(42))
		}},
		{"flip clears a high bit and trims", func(t *testing.T) {
			b := bitset.New()
			b.Set(300)
			b.Flip(300)
			require.Zero(t, b.Len())
		}},
		{"cardinality counts across words", func(t *testing.T) {
			b := bitset.New()
			for _, i := range []int{0, 63, 64, 127, 128} {
				b.Set(i)
			}
			require.Equal(t, 5, b.Cardinality())
		}},
		{"cardinality of empty set", func(t *testing.T) {
			require.Zero(t, bitset.New().Cardinality())
		}},
		{"is empty flips with content", func(t *testing.T) {
			b := bitset.New()
			require.True(t, b.IsEmpty())
			b.Set(1)
			require.False(t, b.IsEmpty())
			b.Clear(1)
			require.True(t, b.IsEmpty())
		}},
		{"len of empty set is zero", func(t *testing.T) {
			require.Zero(t, bitset.New().Len())
		}},
		{"len equals highest set bit plus one", func(t *testing.T) {
			b := bitset.New()
			b.Set(10)
			require.Equal(t, 11, b.Len())
			b.Set(200)
			require.Equal(t, 201, b.Len())
		}},
		{"len after clearing highest bit drops", func(t *testing.T) {
			b := bitset.New()
			b.Set(10)
			b.Set(200)
			b.Clear(200)
			require.Equal(t, 11, b.Len())
		}},
		{"next set bit from zero on empty set", func(t *testing.T) {
			require.Equal(t, -1, bitset.New().NextSetBit(0))
		}},
		{"next set bit inclusive at the hit", func(t *testing.T) {
			b := bitset.New()
			b.Set(5)
			require.Equal(t, 5, b.NextSetBit(5))
		}},
		{"next set bit skips clear words", func(t *testing.T) {
			b := bitset.New()
			b.Set(5)
			b.Set(500)
			require.Equal(t, 500, b.NextSetBit(6))
		}},
		{"next set bit beyond end returns -1", func(t *testing.T) {
			b := bitset.New()
			b.Set(5)
			require.Equal(t, -1, b.NextSetBit(6))
		}},
		{"prev set bit inclusive at the hit", func(t *testing.T) {
			b := bitset.New()
			b.Set(5)
			require.Equal(t, 5, b.PrevSetBit(5))
		}},
		{"prev set bit from far above the end", func(t *testing.T) {
			b := bitset.New()
			b.Set(5)
			require.Equal(t, 5, b.PrevSetBit(1<<20))
		}},
		{"prev set bit negative returns -1", func(t *testing.T) {
			b := bitset.New()
			b.Set(5)
			require.Equal(t, -1, b.PrevSetBit(-1))
		}},
		{"prev set bit with no earlier bit", func(t *testing.T) {
			b := bitset.New()
			b.Set(70)
			require.Equal(t, -1, b.PrevSetBit(69))
		}},
		{"and with self is identity", func(t *testing.T) {
			b := bitset.New()
			b.Set(1)
			b.Set(70)
			c := b.Clone()
			b.And(c)
			require.Equal(t, []int{1, 70}, b.ToSlice())
		}},
		{"or with self is identity", func(t *testing.T) {
			b := bitset.New()
			b.Set(1)
			c := b.Clone()
			b.Or(c)
			require.Equal(t, []int{1}, b.ToSlice())
		}},
		{"xor with self clears everything", func(t *testing.T) {
			b := bitset.New()
			b.Set(1)
			b.Set(70)
			c := b.Clone()
			b.Xor(c)
			require.True(t, b.IsEmpty(), "a xor a = 0")
		}},
		{"andNot with self clears everything", func(t *testing.T) {
			b := bitset.New()
			b.Set(1)
			c := b.Clone()
			b.AndNot(c)
			require.True(t, b.IsEmpty())
		}},
		{"or with longer operand grows", func(t *testing.T) {
			b := bitset.New()
			b.Set(1)
			other := bitset.New()
			other.Set(300)
			b.Or(other)
			require.Equal(t, []int{1, 300}, b.ToSlice())
		}},
		{"xor with longer operand grows", func(t *testing.T) {
			b := bitset.New()
			other := bitset.New()
			other.Set(300)
			b.Xor(other)
			require.Equal(t, []int{300}, b.ToSlice())
		}},
		{"andNot trims the tail", func(t *testing.T) {
			b := bitset.New()
			b.Set(300)
			other := bitset.New()
			other.Set(300)
			b.AndNot(other)
			require.Zero(t, b.Len())
		}},
		{"and with longer operand drops the tail", func(t *testing.T) {
			b := bitset.New()
			b.Set(5)
			b.Set(300)
			other := bitset.New()
			other.Set(5)
			b.And(other)
			require.Equal(t, []int{5}, b.ToSlice())
		}},
		{"clone of empty set", func(t *testing.T) {
			b := bitset.New()
			c := b.Clone()
			require.True(t, b.Equal(c))
			require.True(t, c.IsEmpty())
		}},
		{"clone independence on set", func(t *testing.T) {
			b := bitset.New()
			b.Set(1)
			c := b.Clone()
			c.Set(2)
			require.False(t, b.Get(2), "clone writes must not leak into the original")
		}},
		{"equal is symmetric", func(t *testing.T) {
			a := bitset.New()
			a.Set(3)
			b := bitset.New()
			b.Set(3)
			require.True(t, a.Equal(b))
			require.True(t, b.Equal(a))
		}},
		{"equal ignores internal representation", func(t *testing.T) {
			a := bitset.New()
			a.Set(300)
			a.Clear(300) // trimmed to empty
			b := bitset.New()
			require.True(t, a.Equal(b), "trimmed and never-grown sets are equal")
		}},
		{"to slice of empty set", func(t *testing.T) {
			require.Empty(t, bitset.New().ToSlice())
		}},
		{"to slice is ascending", func(t *testing.T) {
			b := bitset.New()
			for _, i := range []int{300, 0, 150, 63} {
				b.Set(i)
			}
			require.Equal(t, []int{0, 63, 150, 300}, b.ToSlice())
		}},
		{"all matches to slice with early break", func(t *testing.T) {
			b := bitset.New()
			for i := range 5 {
				b.Set(i * 65)
			}
			count := 0
			for range b.All() {
				count++
				if count == 2 {
					break
				}
			}
			require.Equal(t, 2, count)
		}},
		{"set high bit then low bit keeps both", func(t *testing.T) {
			b := bitset.New()
			b.Set(1000)
			b.Set(1)
			require.Equal(t, []int{1, 1000}, b.ToSlice())
		}},
		{"word boundary pair 63 64", func(t *testing.T) {
			b := bitset.New()
			b.Set(63)
			b.Set(64)
			require.Equal(t, []int{63, 64}, b.ToSlice())
			b.Clear(63)
			require.Equal(t, []int{64}, b.ToSlice())
		}},
		{"clear bit zero in first word", func(t *testing.T) {
			b := bitset.New()
			b.Set(0)
			b.Clear(0)
			require.True(t, b.IsEmpty())
		}},
		{"next set bit chain walks all bits", func(t *testing.T) {
			b := bitset.New()
			for _, i := range []int{0, 64, 65, 129} {
				b.Set(i)
			}
			var walked []int
			for from := b.NextSetBit(0); from >= 0; from = b.NextSetBit(from + 1) {
				walked = append(walked, from)
			}
			require.Equal(t, []int{0, 64, 65, 129}, walked)
		}},
		{"prev set bit chain walks all bits backwards", func(t *testing.T) {
			b := bitset.New()
			for _, i := range []int{0, 64, 65, 129} {
				b.Set(i)
			}
			var walked []int
			for from := b.PrevSetBit(1 << 10); from >= 0; from = b.PrevSetBit(from - 1) {
				walked = append(walked, from)
			}
			require.Equal(t, []int{129, 65, 64, 0}, walked)
		}},
		{"get with negative index returns false", func(t *testing.T) {
			b := bitset.New()
			b.Set(5)
			require.False(t, b.Get(-1))
		}},
		{"cardinality after mixed ops", func(t *testing.T) {
			b := bitset.New()
			b.Set(1)
			b.Set(2)
			b.Set(3)
			b.Flip(2)
			b.Clear(3)
			b.Set(200)
			require.Equal(t, 2, b.Cardinality()) // 1 and 200
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// bigRef converts a BitSet into a big.Int holding the same bits.
func bigRef(b *bitset.BitSet) *big.Int {
	x := new(big.Int)
	for _, i := range b.ToSlice() {
		x.SetBit(x, i, 1)
	}
	return x
}

// TestBitSetAgainstBigInt is a differential test: random Set/Clear/Flip and
// boolean-op traffic replayed against math/big.Int bit arithmetic.
func TestBitSetAgainstBigInt(t *testing.T) {
	const span = 320 // five words
	for _, seed := range [][2]uint64{{2026, 1012}, {7, 11}, {99, 4242}} {
		t.Run(fmt.Sprintf("seed-%d-%d", seed[0], seed[1]), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(seed[0], seed[1]))
			b := bitset.New()
			ref := new(big.Int)
			other := bitset.New()
			otherRef := new(big.Int)
			for step := range 400 {
				idx := rng.IntN(span)
				switch rng.IntN(8) {
				case 0, 1, 2:
					b.Set(idx)
					ref.SetBit(ref, idx, 1)
				case 3:
					b.Clear(idx)
					ref.SetBit(ref, idx, 0)
				case 4:
					b.Flip(idx)
					ref.Xor(ref, new(big.Int).Lsh(big.NewInt(1), uint(idx)))
				case 5: // build a random operand and And
					other = bitset.New()
					otherRef = new(big.Int)
					for range rng.IntN(4) {
						j := rng.IntN(span)
						other.Set(j)
						otherRef.SetBit(otherRef, j, 1)
					}
					b.And(other)
					ref.And(ref, otherRef)
				case 6: // Or / Xor
					other = bitset.New()
					otherRef = new(big.Int)
					for range rng.IntN(4) {
						j := rng.IntN(span)
						other.Set(j)
						otherRef.SetBit(otherRef, j, 1)
					}
					if rng.IntN(2) == 0 {
						b.Or(other)
						ref.Or(ref, otherRef)
					} else {
						b.Xor(other)
						ref.Xor(ref, otherRef)
					}
				case 7: // AndNot
					other = bitset.New()
					otherRef = new(big.Int)
					for range rng.IntN(4) {
						j := rng.IntN(span)
						other.Set(j)
						otherRef.SetBit(otherRef, j, 1)
					}
					b.AndNot(other)
					ref.AndNot(ref, otherRef)
				}
				require.Zero(t, ref.Cmp(bigRef(b)), "step %d contents: got %v; want %v", step, bigRef(b), ref)
				require.Equal(t, ref.BitLen(), b.Len(), "step %d Len", step)
				require.Equal(t, ref.Bit(0) == 1, b.Get(0), "step %d Get(0)", step)
				require.Equal(t, ref.BitLen() == 0, b.IsEmpty(), "step %d IsEmpty", step)
				// NextSetBit / PrevSetBit agree with big.Int scans
				next := -1
				for i := range span {
					if ref.Bit(i) == 1 {
						next = i
						break
					}
				}
				require.Equal(t, next, b.NextSetBit(0), "step %d NextSetBit(0)", step)
				prev := -1
				for i := span - 1; i >= 0; i-- {
					if ref.Bit(i) == 1 {
						prev = i
						break
					}
				}
				require.Equal(t, prev, b.PrevSetBit(span-1), "step %d PrevSetBit", step)
			}
		})
	}
}
