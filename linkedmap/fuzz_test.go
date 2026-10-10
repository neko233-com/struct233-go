package linkedmap_test

import (
	"slices"
	"testing"

	"github.com/neko233-com/struct233-go/linkedmap"
)

// FuzzLinkedMapOrderAndValues drives Put/Remove from arbitrary bytes and checks
// both the insertion order and the values against reference bookkeeping.
func FuzzLinkedMapOrderAndValues(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 4, 8, 12, 1, 5})
	f.Add([]byte{0x00, 0x05, 0x01, 0x09, 0x02})

	f.Fuzz(func(t *testing.T, ops []byte) {
		m := linkedmap.New[int, int]()
		order := make([]int, 0, 8)
		values := make(map[int]int)

		for i, raw := range ops {
			key := int(raw>>2) % 8

			switch raw & 3 {
			case 0, 1, 2: // put
				old, existed := m.Put(key, i)
				refOld, refExisted := values[key]
				if existed != refExisted {
					t.Fatalf("Put(%d) existed=%v; want %v", key, existed, refExisted)
				}
				if existed && old != refOld {
					t.Fatalf("Put(%d) old=%d; want %d", key, old, refOld)
				}
				if !existed {
					order = append(order, key) // new keys go to the end
				}
				values[key] = i
			case 3: // remove
				old, removed := m.Remove(key)
				refOld, refExisted := values[key]
				if removed != refExisted {
					t.Fatalf("Remove(%d) removed=%v; want %v", key, removed, refExisted)
				}
				if removed && old != refOld {
					t.Fatalf("Remove(%d) old=%d; want %d", key, old, refOld)
				}
				if removed {
					delete(values, key)
					if idx := slices.Index(order, key); idx >= 0 {
						order = slices.Delete(order, idx, idx+1)
					}
				}
			}

			if got := m.Keys(); !slices.Equal(got, order) {
				t.Fatalf("Keys() = %v; want %v", got, order)
			}
			wantValues := make([]int, 0, len(order))
			for _, k := range order {
				wantValues = append(wantValues, values[k])
			}
			if got := m.Values(); !slices.Equal(got, wantValues) {
				t.Fatalf("Values() = %v; want %v", got, wantValues)
			}

			var iterated []int
			for k, v := range m.All() {
				iterated = append(iterated, k)
				if want := values[k]; v != want {
					t.Fatalf("All() value for %d = %d; want %d", k, v, want)
				}
			}
			if !slices.Equal(iterated, order) {
				t.Fatalf("All() order = %v; want %v", iterated, order)
			}
		}
	})
}
