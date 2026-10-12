package bitset_test

import (
	"math/big"
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

			// cross-check the same state against math/big.Int
			x := new(big.Int)
			for _, i := range wantIndexes {
				x.SetBit(x, i, 1)
			}
			if got := bigRef(b); got.Cmp(x) != 0 {
				t.Fatalf("big.Int cross-check: got %v; want %v", got, x)
			}
			if b.Len() != x.BitLen() {
				t.Fatalf("Len() = %d; big.Int BitLen() = %d", b.Len(), x.BitLen())
			}
			if b.Cardinality() != len(wantIndexes) {
				t.Fatalf("Cardinality() = %d; want %d", b.Cardinality(), len(wantIndexes))
			}
		}
	})
}

// FuzzBitSetBooleanOpsAgainstBigInt replays random Set/Clear/Flip and
// And/Or/Xor/AndNot traffic against math/big.Int bit arithmetic.
func FuzzBitSetBooleanOpsAgainstBigInt(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2, 3, 4, 5, 16, 17, 32, 33})
	f.Add([]byte{0x04, 0xFC, 0x80, 0xFF, 0x10, 0x20, 0x30, 0x40})

	f.Fuzz(func(t *testing.T, ops []byte) {
		const span = 128
		b, other := bitset.New(), bitset.New()
		ref, otherRef := new(big.Int), new(big.Int)

		for i, raw := range ops {
			index := int(raw>>3) % span
			switch raw & 7 {
			case 0, 1:
				b.Set(index)
				ref.SetBit(ref, index, 1)
			case 2:
				b.Clear(index)
				ref.SetBit(ref, index, 0)
			case 3:
				b.Flip(index)
				ref.Xor(ref, new(big.Int).Lsh(big.NewInt(1), uint(index)))
			case 4, 5: // rebuild the operand
				other, otherRef = bitset.New(), new(big.Int)
				for j := i; j < len(ops) && j < i+3; j++ {
					k := int(ops[j]>>3) % span
					other.Set(k)
					otherRef.SetBit(otherRef, k, 1)
				}
			case 6:
				switch (raw >> 2) % 3 {
				case 0:
					b.And(other)
					ref.And(ref, otherRef)
				case 1:
					b.Or(other)
					ref.Or(ref, otherRef)
				case 2:
					b.Xor(other)
					ref.Xor(ref, otherRef)
				}
			case 7:
				b.AndNot(other)
				ref.AndNot(ref, otherRef)
			}

			if got := bigRef(b); got.Cmp(ref) != 0 {
				t.Fatalf("after %d ops: got %v; want %v", i+1, got, ref)
			}
			if b.Len() != ref.BitLen() {
				t.Fatalf("after %d ops: Len() = %d; big.Int BitLen() = %d", i+1, b.Len(), ref.BitLen())
			}
			popcount := 0
			for j := range span {
				if ref.Bit(j) != 0 {
					popcount++
				}
			}
			if b.Cardinality() != popcount {
				t.Fatalf("after %d ops: Cardinality() = %d; want %d", i+1, b.Cardinality(), popcount)
			}
			for _, probe := range []int{0, 1, 63, 64, span - 1} {
				if got, want := b.Get(probe), ref.Bit(probe) == 1; got != want {
					t.Fatalf("after %d ops: Get(%d) = %v; want %v", i+1, probe, got, want)
				}
			}
		}
	})
}
