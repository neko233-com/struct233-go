package latch_test

import "testing"

// FuzzLatchOps replays the byte stream of the fuzzer as a deterministic
// operation sequence (CountDown / Await with a canceled context /
// AwaitTimeout(0) / Count / Open) and requires the latch to keep agreeing with
// the count-down state machine after every step. Await is exercised through its
// two deterministic branches only, so no fuzz run can block.
func FuzzLatchOps(f *testing.F) {
	f.Add(3, []byte{0, 0, 0, 3, 2, 4})
	f.Add(1, []byte{0, 0, 1, 2, 3, 0, 4, 0, 0, 0})
	f.Add(0, []byte{2, 0, 1, 3, 4, 2})
	f.Add(5, []byte{0, 0, 0, 0, 0, 0, 2, 1, 3, 4})
	f.Add(2, []byte{})
	f.Add(8, []byte{1, 2, 3, 4, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, initialCount int, data []byte) {
		if initialCount < 0 || initialCount > 64 {
			t.Skip("model only tracks counts 0..64")
		}
		runLatchOps(t, initialCount, data)
	})
}
