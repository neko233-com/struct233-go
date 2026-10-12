package pqueue_test

import "testing"

// FuzzPriorityQueueOps replays the byte stream of the fuzzer as Push/Pop
// traffic and requires the queue to keep agreeing with the sort.Slice
// reference model: every Pop must match the model's stable dequeue order and
// the final drain must equal the sorted multiset of everything pushed.
func FuzzPriorityQueueOps(f *testing.F) {
	f.Add([]byte{1, 5, 1, 3, 1, 9, 0, 0, 0, 0})
	f.Add([]byte{1, 2, 1, 2, 1, 2, 0, 0})
	f.Add([]byte{0, 0, 1, 1, 0, 0, 1, 7, 0, 0, 0, 0})
	f.Add([]byte{1, 0, 1, 255, 1, 128, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{})
	f.Add([]byte{0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		runPQModel(t, decodePQOps(data), func(a, b int) int { return a - b })
		runPQModel(t, decodePQOps(data), func(a, b int) int { return b - a })
	})
}
