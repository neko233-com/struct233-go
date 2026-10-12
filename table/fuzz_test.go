package table_test

import "testing"

// FuzzTable replays the byte stream of the fuzzer as Put/Remove traffic and
// requires the table to keep agreeing with the map[row]map[col]V plus
// insertion-order model after every mutation.
func FuzzTable(f *testing.F) {
	f.Add([]byte{0, 1, 1, 0, 10, 0, 1, 1, 0, 11, 1, 1, 1, 0, 0})
	f.Add([]byte{0, 1, 1, 0, 1, 0, 1, 2, 0, 2, 1, 2, 1, 0, 0, 2, 1, 1, 0, 3})
	f.Add([]byte{1, 1, 1, 0, 0, 0, 2, 1, 0, 0, 3, 2, 0, 0, 0, 1, 1, 0, 5, 4})
	f.Add([]byte{4, 0, 0, 0, 0, 0, 1, 1, 0, 9, 2, 2, 0, 0, 3, 3, 0, 0, 0})
	f.Add([]byte{0, 255, 255, 0, 127, 0, 255, 1, 0, 0})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		runTableScript(t, false, decodeTableOps(data))
	})
}
