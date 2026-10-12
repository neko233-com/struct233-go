package set_test

import (
	"slices"
	"testing"

	"github.com/neko233-com/struct233-go/set"
)

// FuzzHashSetAgainstMap drives Add/Remove from arbitrary bytes and compares
// the set against a plain map[T]struct{} model after every operation.
func FuzzHashSetAgainstMap(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2, 3, 4, 5})
	f.Add([]byte{0x04, 0xFC, 0x80, 0xFF})

	f.Fuzz(func(t *testing.T, ops []byte) {
		s := set.NewHashSet[int]()
		ref := make(map[int]struct{})

		for _, raw := range ops {
			v := int(raw>>1) % 16
			if raw&1 == 0 {
				s.Add(v)
				ref[v] = struct{}{}
			} else {
				s.Remove(v)
				delete(ref, v)
			}

			if s.Len() != len(ref) {
				t.Fatalf("Len() = %d; want %d", s.Len(), len(ref))
			}
			want := make([]int, 0, len(ref))
			for x := range ref {
				want = append(want, x)
			}
			slices.Sort(want)
			got := s.ToSlice()
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Fatalf("ToSlice() = %v; want %v", got, want)
			}
			for x := range 16 {
				_, wantOK := ref[x]
				if gotOK := s.Contains(x); gotOK != wantOK {
					t.Fatalf("Contains(%d) = %v; want %v", x, gotOK, wantOK)
				}
			}
		}
	})
}

// FuzzLinkedHashSetAgainstMapAndOrder drives Add/Remove from arbitrary bytes
// and checks both membership and the insertion order against a model.
func FuzzLinkedHashSetAgainstMapAndOrder(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2, 3, 4, 5})
	f.Add([]byte{0x08, 0x09, 0x02, 0x03})

	f.Fuzz(func(t *testing.T, ops []byte) {
		s := set.NewLinkedHashSet[int]()
		ref := make(map[int]struct{})
		var order []int

		for _, raw := range ops {
			v := int(raw>>1) % 16
			if raw&1 == 0 {
				if _, ok := ref[v]; !ok {
					order = append(order, v)
				}
				s.Add(v)
				ref[v] = struct{}{}
			} else {
				if _, ok := ref[v]; ok {
					order = slices.Delete(order, slices.Index(order, v), slices.Index(order, v)+1)
				}
				s.Remove(v)
				delete(ref, v)
			}

			if s.Len() != len(ref) {
				t.Fatalf("Len() = %d; want %d", s.Len(), len(ref))
			}
			if got := s.ToSlice(); !slices.Equal(got, order) {
				t.Fatalf("ToSlice() = %v; want %v", got, order)
			}
			var iterated []int
			for x := range s.All() {
				iterated = append(iterated, x)
			}
			if !slices.Equal(iterated, order) {
				t.Fatalf("All() = %v; want %v", iterated, order)
			}
		}
	})
}

// FuzzSkipListSetAgainstSortedSlice drives Add/Remove from arbitrary bytes and
// checks the sorted order and membership against a sorted-dedup slice model.
func FuzzSkipListSetAgainstSortedSlice(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2, 3, 4, 5})
	f.Add([]byte{0x08, 0x09, 0x02, 0x03})

	f.Fuzz(func(t *testing.T, ops []byte) {
		ts := set.NewSkipListSet[int]()
		model := make([]int, 0, 16) // sorted, no duplicates

		for _, raw := range ops {
			v := int(raw>>1) % 16
			if raw&1 == 0 {
				ts.Add(v)
				i, found := slices.BinarySearch(model, v)
				if !found {
					model = slices.Insert(model, i, v)
				}
			} else {
				ts.Remove(v)
				if i, found := slices.BinarySearch(model, v); found {
					model = slices.Delete(model, i, i+1)
				}
			}

			if ts.Len() != len(model) {
				t.Fatalf("Len() = %d; want %d", ts.Len(), len(model))
			}
			if got := ts.ToSlice(); !slices.Equal(got, model) {
				t.Fatalf("ToSlice() = %v; want %v", got, model)
			}
			for x := range 16 {
				i, found := slices.BinarySearch(model, x)
				_ = i
				if got := ts.Contains(x); got != found {
					t.Fatalf("Contains(%d) = %v; want %v", x, got, found)
				}
			}
		}
	})
}
