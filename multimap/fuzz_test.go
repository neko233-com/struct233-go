package multimap_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/multimap"
)

// decodeOps turns fuzz bytes into at most maxOps operations of three bytes each:
// one operation code, one key and one value seed.
func decodeOps(data []byte) []op[uint8, uint8] {
	const maxOps = 24
	ops := make([]op[uint8, uint8], 0, maxOps)
	for i := 0; i+2 < len(data) && len(ops) < maxOps; i += 3 {
		kind := listOpKind(data[i] % 6)
		o := op[uint8, uint8]{kind: kind, key: data[i+1] % 6}
		switch kind {
		case listPut, listRemove:
			o.values = []uint8{data[i+2] % 5}
		case listPutAll:
			o.values = spread(data[i+2], 1+int(data[i]%4))
		case listReplace:
			o.values = spread(data[i+2], int(data[i]%5))
		}
		ops = append(ops, o)
	}
	return ops
}

// spread returns n values starting at base and stepping through the small
// alphabet, so that duplicates happen often.
func spread(base uint8, n int) []uint8 {
	out := make([]uint8, 0, n)
	for range n {
		out = append(out, base%5)
		base++
	}
	return out
}

// FuzzListMultimap replays random operations against the map plus
// insertion-order model.
func FuzzListMultimap(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2})
	f.Add([]byte{0, 0, 0, 0, 0, 0})
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})
	f.Add([]byte{2, 3, 4, 2, 3, 4, 2, 3, 4})
	f.Add([]byte{255, 254, 253, 252, 251, 250, 249, 248, 247})

	f.Fuzz(func(t *testing.T, data []byte) {
		m := multimap.NewList[uint8, uint8]()
		mo := newListModel[uint8, uint8]()
		for _, o := range decodeOps(data) {
			applyOp(t, m, mo, o)
			requireListMatchesModel(t, m, mo)
			requireListInvariants(t, m)
		}
	})
}

// FuzzSetMultimap replays random operations against the map of sets model.
func FuzzSetMultimap(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2})
	f.Add([]byte{1, 1, 1, 1, 1, 1, 1, 1, 1})
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})
	f.Add([]byte{2, 3, 4, 2, 3, 4, 2, 3, 4})
	f.Add([]byte{255, 254, 253, 252, 251, 250, 249, 248, 247})

	f.Fuzz(func(t *testing.T, data []byte) {
		m := multimap.NewSet[uint8, uint8]()
		mo := newSetModel[uint8, uint8]()
		for _, o := range decodeOps(data) {
			applySetOp(t, m, mo, o)
			requireSetMatchesModel(t, m, mo)
			requireSetInvariants(t, m)
		}
	})
}
