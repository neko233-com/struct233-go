package unionfind_test

import "testing"

// FuzzUnionFind replays the byte stream of the fuzzer as Union/Find/Connected/
// Size traffic over a fixed element universe and requires the set to keep
// agreeing with the equivalence-class model after every operation.
func FuzzUnionFind(f *testing.F) {
	const n = 16
	f.Add([]byte{0, 0, 1, 2, 3, 4, 0, 5, 6, 1, 7, 0})
	f.Add([]byte{0, 0, 15, 0, 0, 15, 4, 0, 0, 2, 7, 7})
	f.Add([]byte{3, 8, 8, 0, 8, 9, 2, 8, 9, 1, 8, 0})
	f.Add([]byte{5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		runUFModel(t, n, decodeUFOps(data, n))
	})
}
