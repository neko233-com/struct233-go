package lru_test

import "testing"

// FuzzLRUOps replays the byte stream of the fuzzer as Put/Get/Peek/Contains/
// Remove/Clear traffic and requires the cache to keep agreeing with the map +
// recency-slice reference model after every step: return values, eviction
// reports, length and the LRU->MRU key order.
func FuzzLRUOps(f *testing.F) {
	f.Add(2, []byte{0, 1, 1, 0, 2, 2, 0, 3, 3, 1, 1, 0, 0, 4, 4})
	f.Add(1, []byte{0, 1, 1, 0, 2, 2, 2, 1, 0, 3, 3})
	f.Add(3, []byte{0, 5, 5, 4, 5, 0, 0, 6, 6, 5, 0, 0, 7, 7, 6, 0, 0})
	f.Add(4, []byte{1, 1, 0, 2, 2, 3, 1, 0, 3, 3, 0, 4, 4, 5, 0})
	f.Add(8, []byte{})
	f.Add(2, []byte{0, 0, 0, 0, 1, 0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, capacity int, data []byte) {
		if capacity < 1 || capacity > 16 {
			t.Skip("model only tracks capacities 1..16")
		}
		runLRUOps(t, capacity, data)
	})
}
