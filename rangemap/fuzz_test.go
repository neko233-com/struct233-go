package rangemap_test

import "testing"

// FuzzRangeMap replays the byte stream of the fuzzer as Put/Remove traffic and
// requires the range map to keep agreeing with the point-sampling model after
// every mutation.
func FuzzRangeMap(f *testing.F) {
	f.Add([]byte{0, 0, 1, 10, 0, 0, 0, 3, 1, 4, 7, 0, 1, 8, 5, 15, 0, 0})
	f.Add([]byte{0, 4, 0, 0, 0, 1, 1, 5, 3, 20, 0, 0, 0, 1, 0, 20, 0, 0})
	f.Add([]byte{0, 0, 1, 10, 0, 1, 1, 2, 5, 8, 0, 0, 2, 0, 0, 0, 0, 0})
	f.Add([]byte{0, 9, 7, 7, 0, 0, 1, 6, 0, 200, 0, 0, 0, 5, 100, 100, 0, 0})
	f.Add([]byte{1, 0, 0, 0, 0, 0, 2, 9, 0, 0, 0, 0})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		runRMScript(t, false, decodeRMOps(data))
	})
}
