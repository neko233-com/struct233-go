package barrier_test

import (
	"testing"
)

// FuzzBarrierOps replays the byte stream of the fuzzer as arrivals,
// cancellations and resets against the cyclic-generation state machine model.
// Every scripted outcome must match what the model predicts.
func FuzzBarrierOps(f *testing.F) {
	f.Add([]byte("AAACR"))
	f.Add([]byte("ACRCA"))
	f.Add([]byte("AAAA"))
	f.Add([]byte("RRR"))
	f.Add([]byte("C"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		// keep scripts bounded so a fuzz case cannot outlive the test timeout
		ops := data
		if len(ops) > 8 {
			ops = ops[:8]
		}
		script := make([]byte, 0, len(ops))
		for _, b := range ops {
			switch b % 3 {
			case 0:
				script = append(script, 'A')
			case 1:
				script = append(script, 'C')
			case 2:
				script = append(script, 'R')
			}
		}
		for _, parties := range []int{1, 2, 3} {
			runBarrierScript(t, parties, string(script))
		}
	})
}
