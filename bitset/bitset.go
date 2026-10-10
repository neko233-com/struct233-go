// Package bitset provides a growable bit vector, similar to java.util.BitSet.
//
// Bit indexes start at 0 and must be non-negative. Not safe for concurrent use.
package bitset

import (
	"iter"
	"math/bits"
	"slices"
)

const wordSize = 64

// BitSet is a growable bit vector.
type BitSet struct {
	words []uint64
}

// New creates an empty BitSet.
func New() *BitSet { return &BitSet{} }

// Set sets the bit at index i.
func (b *BitSet) Set(i int) {
	w := i >> 6
	if w >= len(b.words) {
		b.words = append(b.words, make([]uint64, w+1-len(b.words))...)
	}
	b.words[w] |= 1 << uint(i&(wordSize-1))
}

// Clear unsets the bit at index i.
func (b *BitSet) Clear(i int) {
	w := i >> 6
	if w >= len(b.words) {
		return
	}
	b.words[w] &^= 1 << uint(i&(wordSize-1))
	b.trim()
}

// Flip toggles the bit at index i.
func (b *BitSet) Flip(i int) {
	w := i >> 6
	if w >= len(b.words) {
		// toggling a bit beyond the end is equivalent to setting it
		b.Set(i)
		return
	}
	b.words[w] ^= 1 << uint(i&(wordSize-1))
	b.trim()
}

// Get reports whether the bit at index i is set. Bits beyond the current
// size are always false.
func (b *BitSet) Get(i int) bool {
	w := i >> 6
	if w < 0 || w >= len(b.words) {
		return false
	}
	return b.words[w]&(1<<uint(i&(wordSize-1))) != 0
}

// trim drops trailing zero words so that word-level comparison
// (Equal, Len) matches the logical contents.
func (b *BitSet) trim() {
	for len(b.words) > 0 && b.words[len(b.words)-1] == 0 {
		b.words = b.words[:len(b.words)-1]
	}
}

// Cardinality returns the number of set bits.
func (b *BitSet) Cardinality() int {
	n := 0
	for _, w := range b.words {
		n += bits.OnesCount64(w)
	}
	return n
}

// IsEmpty reports whether no bit is set.
func (b *BitSet) IsEmpty() bool { return b.Cardinality() == 0 }

// Len returns the highest set bit index plus one (0 when empty),
// matching java.util.BitSet.length().
func (b *BitSet) Len() int {
	for w := len(b.words) - 1; w >= 0; w-- {
		if b.words[w] != 0 {
			return w*wordSize + (wordSize - bits.LeadingZeros64(b.words[w]))
		}
	}
	return 0
}

// NextSetBit returns the smallest set-bit index >= from, or -1 if none.
// It panics if from is negative.
func (b *BitSet) NextSetBit(from int) int {
	if from < 0 {
		panic("bitset: negative index")
	}
	w := from >> 6
	if w >= len(b.words) {
		return -1
	}
	if word := b.words[w] >> uint(from&(wordSize-1)); word != 0 {
		return from + bits.TrailingZeros64(word)
	}
	for w++; w < len(b.words); w++ {
		if b.words[w] != 0 {
			return w*wordSize + bits.TrailingZeros64(b.words[w])
		}
	}
	return -1
}

// PrevSetBit returns the greatest set-bit index <= from, or -1 if none.
func (b *BitSet) PrevSetBit(from int) int {
	if from < 0 {
		return -1
	}
	w := from >> 6
	if w >= len(b.words) {
		if len(b.words) == 0 {
			return -1
		}
		w = len(b.words) - 1
	} else {
		mask := uint64(1)<<uint(from&(wordSize-1)+1) - 1
		if word := b.words[w] & mask; word != 0 {
			return w*wordSize + (wordSize - bits.LeadingZeros64(word)) - 1
		}
		w--
	}
	for ; w >= 0; w-- {
		if b.words[w] != 0 {
			return w*wordSize + (wordSize - bits.LeadingZeros64(b.words[w])) - 1
		}
	}
	return -1
}

// And intersects b with other (in place).
func (b *BitSet) And(other *BitSet) {
	n := min(len(b.words), len(other.words))
	for i := range n {
		b.words[i] &= other.words[i]
	}
	clear(b.words[n:])
	b.trim()
}

// Or unions b with other (in place).
func (b *BitSet) Or(other *BitSet) {
	if len(other.words) > len(b.words) {
		b.words = append(b.words, make([]uint64, len(other.words)-len(b.words))...)
	}
	for i, w := range other.words {
		b.words[i] |= w
	}
}

// Xor toggles in b every bit that is set in other (in place).
func (b *BitSet) Xor(other *BitSet) {
	if len(other.words) > len(b.words) {
		b.words = append(b.words, make([]uint64, len(other.words)-len(b.words))...)
	}
	for i, w := range other.words {
		b.words[i] ^= w
	}
	b.trim()
}

// AndNot clears in b every bit that is set in other (in place).
func (b *BitSet) AndNot(other *BitSet) {
	n := min(len(b.words), len(other.words))
	for i := range n {
		b.words[i] &^= other.words[i]
	}
	b.trim()
}

// Clone returns an independent copy.
func (b *BitSet) Clone() *BitSet {
	return &BitSet{words: slices.Clone(b.words)}
}

// Equal reports whether both sets contain the same set bits.
func (b *BitSet) Equal(other *BitSet) bool {
	return slices.Equal(b.words, other.words)
}

// ToSlice returns the indexes of all set bits in ascending order.
func (b *BitSet) ToSlice() []int {
	out := make([]int, 0, b.Cardinality())
	for w, word := range b.words {
		for word != 0 {
			out = append(out, w*wordSize+bits.TrailingZeros64(word))
			word &= word - 1
		}
	}
	return out
}

// All iterates over the set-bit indexes in ascending order.
func (b *BitSet) All() iter.Seq[int] {
	return func(yield func(int) bool) {
		for w, word := range b.words {
			for word != 0 {
				if !yield(w*wordSize + bits.TrailingZeros64(word)) {
					return
				}
				word &= word - 1
			}
		}
	}
}
