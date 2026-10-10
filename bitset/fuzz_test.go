package bitset_test

import (
	"slices"
	"testing"

	"github.com/neko233-com/struct233-go/bitset"
)

const fuzzBitSpan = 200

// FuzzBitSetAgainstMap drives Set/Clear/Flip from arbitrary bytes and compares
// the bit vector against a reference set of indexes.
func FuzzBitSetAgainstMap(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2, 3, 4, 5})
	f.Add([]byte{0x04, 0xFC, 0x80, 0xFF})

	f.Fuzz(func(t *testing.T, ops []byte) {
		b := bitset.New()
		ref := make(map[int]struct{})

		for _, raw := range ops {
			index := int(raw>>2) % fuzzBitSpan
			switch raw & 3 {
			case 0:
				b.Set(index)
				ref[index] = struct{}{}
			case 1:
				b.Clear(index)
				delete(ref, index)
			case 2, 3:
				b.Flip(index)
				if _, ok := ref[index]; ok {
					delete(ref, index)
				} else {
					ref[index] = struct{}{}
				}
			}

			if got, want := b.Cardinality(), len(ref); got != want {
				t.Fatalf("Cardinality() = %d; want %d", got, want)
			}

			wantIndexes := make([]int, 0, len(ref))
			for i := range ref {
				wantIndexes = append(wantIndexes, i)
			}
			slices.Sort(wantIndexes)
			if got := b.ToSlice(); !slices.Equal(got, wantIndexes) {
				t.Fatalf("ToSlice() = %v; want %v", got, wantIndexes)
			}

			// every index in the span agrees with the reference set
			for i := range fuzzBitSpan {
				_, want := ref[i]
				if got := b.Get(i); got != want {
					t.Fatalf("Get(%d) = %v; want %v", i, got, want)
				}
			}

			// NextSetBit walks exactly the reference indexes
			from := 0
			for _, want := range wantIndexes {
				got := b.NextSetBit(from)
				if got != want {
					t.Fatalf("NextSetBit(%d) = %d; want %d", from, got, want)
				}
				from = want + 1
			}
			if got := b.NextSetBit(from); got != -1 {
				t.Fatalf("NextSetBit(%d) = %d; want -1", from, got)
			}
		}
	})
}
