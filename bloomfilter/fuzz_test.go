package bloomfilter_test

import "testing"

// FuzzBloomFilter replays the byte stream of the fuzzer as Add/Test/Clear
// traffic and requires the filter to keep agreeing with the map model: no
// false negatives, exact Count, and a zero false-positive estimate exactly on
// an empty filter. False positives are legal and never asserted against.
func FuzzBloomFilter(f *testing.F) {
	f.Add([]byte{0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 0, 0})
	f.Add([]byte{4, 0, 0, 0, 1, 2, 4, 0, 0, 3, 2, 2})
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{2, 15, 0, 3, 15, 0, 1, 15, 0, 4, 0, 0})
	f.Add([]byte{})
	f.Add([]byte{5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	f.Fuzz(func(t *testing.T, data []byte) {
		runBFModel(t, decodeBFOps(data))
	})
}
