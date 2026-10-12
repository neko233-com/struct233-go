package deque_test

import "testing"

// FuzzDequeOps replays the byte stream of the fuzzer as two-ended operation
// traffic (PushFront/PushBack/PopFront/PopBack/Front/Back/Clear) and requires
// the deque to keep agreeing with the plain slice reference model after every
// step.
func FuzzDequeOps(f *testing.F) {
	f.Add([]byte{0, 1, 0, 2, 2, 0, 1, 3, 3, 0, 4, 0, 5, 0})
	f.Add([]byte{1, 5, 0, 6, 2, 0, 3, 0, 1, 7, 0, 8, 6, 0})
	f.Add([]byte{2, 0, 3, 0, 4, 0, 5, 0, 0, 1, 1, 2})
	f.Add([]byte{6, 0, 0, 1, 1, 2, 6, 0, 0, 3})
	f.Add([]byte{})
	f.Add([]byte{0, 255, 1, 128, 2, 0, 3, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		runDequeOps(t, data)
	})
}
