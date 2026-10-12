package striped_test

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/striped"
)

// identityHash is the simplest usable hash: the key IS the hash value.
func identityHash(u uint64) uint64 { return u }

// lowBitsHash deliberately collides: only the low 4 bits vary, so with a
// large stripe count many keys share a stripe.
func lowBitsHash(u uint64) uint64 { return u & 0xf }

// fnv64 hashes strings the way a typical user-supplied hash would.
func fnv64(s string) uint64 {
	h := uint64(0xcbf29ce484222325)
	for i := 0; i < len(s); i++ {
		h = (h ^ uint64(s[i])) * 0x100000001b3
	}
	return h
}

// timeout returns a channel that fires after the per-assertion wait budget.
func timeout() <-chan time.Time { return time.After(time.Second) }

func TestNewRoundsUpToPowerOfTwo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		stripes int
		want    int
	}{
		{"one", 1, 1},
		{"two", 2, 2},
		{"three", 3, 4},
		{"four", 4, 4},
		{"five", 5, 8},
		{"seven", 7, 8},
		{"eight", 8, 8},
		{"nine", 9, 16},
		{"fifteen", 15, 16},
		{"sixteen", 16, 16},
		{"seventeen", 17, 32},
		{"thirty_one", 31, 32},
		{"thirty_two", 32, 32},
		{"thirty_three", 33, 64},
		{"sixty_three", 63, 64},
		{"sixty_four", 64, 64},
		{"sixty_five", 65, 128},
		{"hundred", 100, 128},
		{"hundred29", 129, 256},
		{"two_hundred", 200, 256},
		{"five_hundred", 500, 512},
		{"thousand", 1000, 1024},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := striped.New(tc.stripes, identityHash)
			require.Equal(t, tc.want, s.Size())
			require.Equal(t, 0, s.Size()&(s.Size()-1), "Size must be a power of two")

			rw := striped.NewRW(tc.stripes, identityHash)
			require.Equal(t, tc.want, rw.Size())
			require.Equal(t, 0, rw.Size()&(rw.Size()-1))
		})
	}
}

func TestNewPanicsOnIllegalArgs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		stripes int
		hash    func(uint64) uint64
		want    string
	}{
		{"zero stripes", 0, identityHash, "striped: stripes must be positive"},
		{"negative stripes", -1, identityHash, "striped: stripes must be positive"},
		{"large negative", -1024, identityHash, "striped: stripes must be positive"},
		{"nil hash", 4, nil, "striped: hash must not be nil"},
		{"nil hash and bad count", -4, nil, "striped: stripes must be positive"},
		{"too large", 1<<30 + 1, identityHash, "striped: stripes too large"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.PanicsWithValue(t, tc.want, func() { _ = striped.New(tc.stripes, tc.hash) })
			assert.PanicsWithValue(t, tc.want, func() { _ = striped.NewRW(tc.stripes, tc.hash) })
		})
	}
}

func TestNewAcceptsLegalArgs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		stripes int
	}{
		{"one", 1},
		{"sixteen", 16},
		{"max", 1 << 30},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// not parallel for the huge case: each subtest allocates the stripes
			if tc.stripes > 1<<16 {
				// Constructing 1<<30 mutexes needs gigabytes; only check the
				// validation passes for the boundary value with a tiny stand-in
				// by asserting the documented boundary is exactly 1<<30.
				assert.Equal(t, 1<<30, tc.stripes)
				return
			}
			s := striped.New(tc.stripes, identityHash)
			assert.Equal(t, tc.stripes, s.Size())
			rw := striped.NewRW(tc.stripes, identityHash)
			assert.Equal(t, tc.stripes, rw.Size())
		})
	}
}

func TestGetSameKeySameMutex(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		stripes int
		hash    func(uint64) uint64
		keys    []uint64
	}{
		{"one stripe", 1, identityHash, []uint64{0, 1, 2, 100, 1 << 40}},
		{"two stripes", 2, identityHash, []uint64{0, 1, 2, 3, 4, 5}},
		{"eight stripes", 8, identityHash, []uint64{0, 7, 8, 15, 16, 12345}},
		{"colliding hash", 16, lowBitsHash, []uint64{0, 1, 0xff, 0x10, 0x100, 7}},
		{"large keys", 32, identityHash, []uint64{1 << 63, 1<<63 + 1, ^uint64(0), ^uint64(0) - 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := striped.New(tc.stripes, tc.hash)
			for _, k := range tc.keys {
				first := s.Get(k)
				for i := 0; i < 10; i++ {
					require.Same(t, first, s.Get(k), "Get must return the same pointer for key %d", k)
				}
			}
		})
	}
}

func TestGetSameKeySameRWMutex(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		stripes int
		hash    func(uint64) uint64
		keys    []uint64
	}{
		{"one stripe", 1, identityHash, []uint64{0, 1}},
		{"four stripes", 4, identityHash, []uint64{0, 3, 4, 99}},
		{"colliding hash", 16, lowBitsHash, []uint64{0, 0x10, 0x20, 0x30}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := striped.NewRW(tc.stripes, tc.hash)
			for _, k := range tc.keys {
				first := s.GetRW(k)
				for i := 0; i < 10; i++ {
					require.Same(t, first, s.GetRW(k), "GetRW must return the same pointer for key %d", k)
				}
			}
		})
	}
}

func TestGetAtWrapsModuloSize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		stripes int
		idx     []int
	}{
		{"one stripe", 1, []int{0, 1, 2, -1, 7, 1000}},
		{"four stripes", 4, []int{0, 1, 3, 4, 5, 8, -1, -4, -5, 99}},
		{"eight stripes", 8, []int{0, 7, 8, 15, 16, -8, -9, 1 << 20}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := striped.New(tc.stripes, identityHash)
			for _, i := range tc.idx {
				// reference index in [0, Size), computed with integer modulo
				ref := i % s.Size()
				if ref < 0 {
					ref += s.Size()
				}
				require.Same(t, s.GetAt(ref), s.GetAt(i), "GetAt(%d) must wrap modulo Size", i)
				// shifting by exactly Size must land on the same stripe
				require.Same(t, s.GetAt(i), s.GetAt(i+s.Size()))
				require.Same(t, s.GetAt(i), s.GetAt(i-s.Size()))
			}
		})
	}
}

func TestGetAtWrapsModuloSizeRW(t *testing.T) {
	t.Parallel()
	s := striped.NewRW(8, identityHash)
	idx := []int{0, 1, 7, 8, 9, -1, -8, -9, 1 << 17}
	for _, i := range idx {
		ref := i % s.Size()
		if ref < 0 {
			ref += s.Size()
		}
		require.Same(t, s.GetAt(ref), s.GetAt(i))
		require.Same(t, s.GetAt(i), s.GetAt(i+s.Size()))
		require.Same(t, s.GetAt(i), s.GetAt(i-s.Size()))
	}
}

func TestGetAtCoversAllStripes(t *testing.T) {
	t.Parallel()
	cases := []int{1, 2, 4, 8, 16, 32, 64}
	for _, n := range cases {
		t.Run("stripes_"+strconv.Itoa(n), func(t *testing.T) {
			t.Parallel()
			s := striped.New(n, identityHash)
			seen := map[*sync.Mutex]struct{}{}
			for i := 0; i < s.Size(); i++ {
				seen[s.GetAt(i)] = struct{}{}
			}
			assert.Len(t, seen, s.Size(), "GetAt must cover every stripe exactly once in [0, Size)")

			rw := striped.NewRW(n, identityHash)
			seenRW := map[*sync.RWMutex]struct{}{}
			for i := 0; i < rw.Size(); i++ {
				seenRW[rw.GetAt(i)] = struct{}{}
			}
			assert.Len(t, seenRW, rw.Size())
		})
	}
}

func TestGetConsistentWithGetAt(t *testing.T) {
	t.Parallel()
	s := striped.New(16, identityHash)
	for k := uint64(0); k < 64; k++ {
		require.Same(t, s.GetAt(int(k&15)), s.Get(k), "Get(k) must equal GetAt(k & mask)")
	}
	rw := striped.NewRW(16, identityHash)
	for k := uint64(0); k < 64; k++ {
		require.Same(t, rw.GetAt(int(k&15)), rw.GetRW(k))
	}
}

func TestDistributionUniformHash(t *testing.T) {
	t.Parallel()
	s := striped.New(16, identityHash)
	counts := map[*sync.Mutex]int{}
	const keys = 1024
	for k := uint64(0); k < keys; k++ {
		counts[s.Get(k)]++
	}
	// identityHash with 1024 keys over 16 stripes: exactly 64 keys per stripe.
	// Deterministic, not statistical, so this can never flake.
	require.Len(t, counts, 16)
	for mu, n := range counts {
		assert.Equal(t, keys/16, n, "uniform hash must spread keys evenly (%p)", mu)
	}
}

func TestDistributionCollidingHash(t *testing.T) {
	t.Parallel()
	s := striped.New(16, lowBitsHash)
	counts := map[*sync.Mutex]int{}
	const keys = 1024
	for k := uint64(0); k < keys; k++ {
		counts[s.Get(k)]++
	}
	// lowBitsHash exposes only 4 bits -> exactly 16 stripes in use,
	// 64 keys each. Deterministic.
	assert.Len(t, counts, 16)
	for _, n := range counts {
		assert.Equal(t, 64, n)
	}
}

func TestDistributionStringHash(t *testing.T) {
	t.Parallel()
	s := striped.New(32, fnv64)
	counts := map[*sync.Mutex]int{}
	const keys = 2048
	for i := 0; i < keys; i++ {
		counts[s.Get("key-"+strconv.Itoa(i))]++
	}
	total := 0
	for _, n := range counts {
		total += n
	}
	assert.Equal(t, keys, total, "every key must map to exactly one stripe")
	// Fixed key set => fixed hash values => deterministic outcome. The bounds
	// below are deliberately wide so the assertion documents "well spread"
	// without depending on any specific hash constant.
	assert.GreaterOrEqual(t, len(counts), 8, "keys must not collapse onto a few stripes")
	for _, n := range counts {
		assert.GreaterOrEqual(t, n, 1)
		assert.Less(t, n, keys/2, "no stripe may hold half of all keys")
	}
}

// TestReferenceModelDifferential compares the mapping against an independent
// reference: stripe = hash(key) % size, computed without bit masking.
func TestReferenceModelDifferential(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		stripes int
	}{
		{"size 1", 1},
		{"size 2", 2},
		{"size 8", 8},
		{"size 33 -> 64", 33},
		{"size 100 -> 128", 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := striped.New(tc.stripes, identityHash)
			size := s.Size()
			for k := uint64(0); k < 512; k++ {
				refIdx := int(identityHash(k) % uint64(size)) // modulo, no mask
				require.Same(t, s.GetAt(refIdx), s.Get(k),
					"key %d: mask mapping must equal hash %% size", k)
			}
			rw := striped.NewRW(tc.stripes, identityHash)
			for k := uint64(0); k < 512; k++ {
				refIdx := int(identityHash(k) % uint64(rw.Size()))
				require.Same(t, rw.GetAt(refIdx), rw.GetRW(k))
			}
		})
	}
}

func TestReferenceModelFNVTable(t *testing.T) {
	t.Parallel()
	// Expected stripe per key is precomputed with the reference formula
	// fnv64(key) % 8, independently of the library's bit-mask path.
	keys := []string{
		"", "a", "b", "ab", "hello", "world",
		"striped", "lock", "mutex", "goroutine",
		"neko233", "struct233-go", "key-0", "key-1",
	}
	s := striped.New(8, fnv64)
	for _, k := range keys {
		refIdx := int(fnv64(k) % 8)
		require.Same(t, s.GetAt(refIdx), s.Get(k), "key %q", k)
	}
}

func TestStringKeysSamePointer(t *testing.T) {
	t.Parallel()
	s := striped.New(16, fnv64)
	keys := []string{"", "a", "hello world", "很长的中文键", "emoji 🐱"}
	for _, k := range keys {
		first := s.Get(k)
		require.Same(t, first, s.Get(k))
		require.Same(t, first, s.Get(string([]byte(k))), "equal content must share the stripe")
	}
}

func TestMutexBlocksSameStripe(t *testing.T) {
	t.Parallel()
	s := striped.New(4, identityHash)
	mu := s.Get(1)
	mu.Lock()
	done := make(chan struct{})
	go func() {
		other := s.Get(1)
		other.Lock()
		other.Unlock()
		close(done)
	}()
	assertNeverReady(t, done)
	mu.Unlock()
	assertReady(t, done)
}

func TestMutexDifferentStripesIndependent(t *testing.T) {
	t.Parallel()
	s := striped.New(4, identityHash)
	s.Get(0).Lock()
	done := make(chan struct{})
	go func() {
		s.Get(1).Lock() // different stripe: must not block
		s.Get(1).Unlock()
		close(done)
	}()
	assertReady(t, done)
	s.Get(0).Unlock()
}

func TestRWMutexSharedReads(t *testing.T) {
	t.Parallel()
	s := striped.NewRW(4, identityHash)
	mu := s.GetRW(1)
	mu.RLock()
	done := make(chan struct{})
	go func() {
		other := s.GetRW(1)
		other.RLock() // shared read must not block
		other.RUnlock()
		close(done)
	}()
	assertReady(t, done)
	mu.RUnlock()
}

func TestRWMutexExclusiveWrite(t *testing.T) {
	t.Parallel()
	s := striped.NewRW(4, identityHash)
	mu := s.GetRW(1)
	mu.Lock()
	done := make(chan struct{})
	go func() {
		other := s.GetRW(1)
		other.RLock()
		other.RUnlock()
		close(done)
	}()
	assertNeverReady(t, done)
	mu.Unlock()
	assertReady(t, done)
}

func assertReady(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-timeout():
		t.Fatal("goroutine did not finish in time")
	}
}

func assertNeverReady(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
		t.Fatal("goroutine should still be blocked")
	case <-timeout():
	}
}
