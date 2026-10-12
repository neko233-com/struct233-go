package rangeset_test

import "testing"

// FuzzRangeSet replays the byte stream of the fuzzer as Add/Remove traffic and
// requires the range set to keep agreeing with the brute-force model after
// every mutation.
func FuzzRangeSet(f *testing.F) {
	f.Add([]byte{0, 0, 1, 5, 0, 0, 1, 4, 0, 1, 2, 5})
	f.Add([]byte{0, 4, 0, 0, 1, 5, 0, 3, 0, 9, 0, 0})
	f.Add([]byte{0, 8, 0, 0, 1, 8, 0, 0, 0, 3, 0, 2, 1, 0, 1, 3})
	f.Add([]byte{0, 9, 7, 7, 0, 0, 200, 0, 1, 6, 0, 200, 0, 5, 100, 100})
	f.Add([]byte{1, 0, 0, 0, 1, 0, 0, 0})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		runModel(t, decodeOps(data))
	})
}
