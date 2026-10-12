package pool_test

import (
	"testing"
)

// FuzzPoolOps replays the byte stream of the fuzzer as Get/Put/Clear/Prealloc
// traffic against the reference stack model, checking object identity, the
// idle count and every counter after each step.
func FuzzPoolOps(f *testing.F) {
	f.Add([]byte("GPG"))
	f.Add([]byte("XXXGGG"))
	f.Add([]byte("BBPPPG"))
	f.Add([]byte("GXPGPXCBPG"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		// keep scripts bounded so a fuzz case cannot grow without limit
		ops := data
		if len(ops) > 32 {
			ops = ops[:32]
		}
		letters := []byte("GPXCB")
		script := make([]byte, 0, len(ops))
		for _, b := range ops {
			script = append(script, letters[int(b)%len(letters)])
		}
		// first byte selects the cap: 0 (unbounded) .. 3
		max := 0
		if len(data) > 0 {
			max = int(data[0]) % 4
		}
		newDualPool(t, max).runOps(string(script))
	})
}
