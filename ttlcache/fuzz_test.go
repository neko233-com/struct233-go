package ttlcache_test

import (
	"testing"
)

// FuzzTTLCache replays a decoded operation stream against both the cache and
// the map + expiry timestamp model and requires full model agreement after
// every operation. The fake clock keeps each replay fully deterministic, so
// any failure is exactly reproducible.
func FuzzTTLCache(f *testing.F) {
	for _, seed := range [][]byte{
		{0, 0, 0, 0},
		{1, 2, 3, 4},
		{9, 3, 7, 2},
		{2, 2, 2, 2},
		{7, 5, 3, 1},
		{0, 1, 2, 3, 4, 5, 6, 7},
		{4, 2, 0, 1, 8, 6, 4, 2},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		runTCModel(t, decodeTCOps(data))
	})
}
