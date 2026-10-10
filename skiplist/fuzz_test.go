package skiplist_test

import (
	"slices"
	"testing"

	"github.com/neko233-com/struct233-go/skiplist"
)

// FuzzSkipListAgainstMap drives Put/Remove/Get from arbitrary byte streams and
// checks the skiplist against a plain map on every step.
func FuzzSkipListAgainstMap(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2, 3})
	f.Add([]byte{0x01, 0x05, 0x09, 0x0D, 0xFF, 0x80, 0x40})

	f.Fuzz(func(t *testing.T, ops []byte) {
		sl := skiplist.New[int, int]()
		ref := make(map[int]int)

		for i, b := range ops {
			key := int(b>>2) % 16

			switch b & 3 {
			case 0, 1: // put (uneven split of op codes, both are puts)
				old, existed := sl.Put(key, i)
				refOld, refExisted := ref[key]
				if existed != refExisted {
					t.Fatalf("Put(%d) existed=%v; want %v", key, existed, refExisted)
				}
				if existed && old != refOld {
					t.Fatalf("Put(%d) old=%d; want %d", key, old, refOld)
				}
				ref[key] = i
			case 2: // remove
				old, removed := sl.Remove(key)
				refOld, refExisted := ref[key]
				if removed != refExisted {
					t.Fatalf("Remove(%d) removed=%v; want %v", key, removed, refExisted)
				}
				if removed && old != refOld {
					t.Fatalf("Remove(%d) old=%d; want %d", key, old, refOld)
				}
				delete(ref, key)
			case 3: // get
				got, ok := sl.Get(key)
				want, wantOK := ref[key]
				if ok != wantOK || (ok && got != want) {
					t.Fatalf("Get(%d) = %d, %v; want %d, %v", key, got, ok, want, wantOK)
				}
			}

			if sl.Len() != len(ref) {
				t.Fatalf("Len() = %d; want %d (ops=%x)", sl.Len(), len(ref), ops[:i+1])
			}

			wantKeys := make([]int, 0, len(ref))
			for k := range ref {
				wantKeys = append(wantKeys, k)
			}
			slices.Sort(wantKeys)
			if got := sl.Keys(); !slices.Equal(got, wantKeys) {
				t.Fatalf("Keys() = %v; want %v", got, wantKeys)
			}
		}
	})
}
