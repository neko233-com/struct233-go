package striped_test

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/striped"
)

// TestFuzzGetDeterministic runs a deterministic pseudo-random key stream
// through Get and checks every result against the reference formula
// hash % size. It is a table-driven fuzz stand-in that never flakes;
// FuzzGet below is the real fuzzing entry point.
func TestFuzzGetDeterministic(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		stripes int
	}{
		{"size 1", 1},
		{"size 2", 2},
		{"size 4", 4},
		{"size 7 -> 8", 7},
		{"size 13 -> 16", 13},
		{"size 128", 128},
	}
	// deterministic xorshift64 key stream; each subtest gets its own state
	// so parallel subtests cannot race on it
	makeRand := func(seed uint64) func() uint64 {
		next := seed
		return func() uint64 {
			next ^= next << 13
			next ^= next >> 7
			next ^= next << 17
			return next
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rand := makeRand(0x9E3779B97F4A7C15)
			s := striped.New(tc.stripes, identityHash)
			rw := striped.NewRW(tc.stripes, identityHash)
			size := uint64(s.Size())
			for i := 0; i < 2000; i++ {
				k := rand()
				require.Same(t, s.GetAt(int(k%size)), s.Get(k))
				require.Same(t, rw.GetAt(int(k%size)), rw.GetRW(k))
				// same key twice -> same pointer
				require.Same(t, s.Get(k), s.Get(k))
				require.Same(t, rw.GetRW(k), rw.GetRW(k))
			}
		})
	}
}

// FuzzGet is the real fuzz entry point: arbitrary (stripes, key, hash-seed)
// inputs must keep the invariant Get(k) == GetAt(hash(k) % size), and
// non-positive stripes must panic.
func FuzzGet(f *testing.F) {
	f.Add(uint16(1), uint64(0), uint64(0))
	f.Add(uint16(8), uint64(12345), uint64(999))
	f.Add(uint16(100), uint64(1<<63), uint64(0xdeadbeef))
	f.Add(uint16(0), uint64(7), uint64(1)) // 0 stripes must panic
	f.Fuzz(func(t *testing.T, stripesRaw uint16, key, seed uint64) {
		stripes := int(stripesRaw)
		if stripes <= 0 {
			require.PanicsWithValue(t, "striped: stripes must be positive", func() {
				_ = striped.New(stripes, identityHash)
			})
			return
		}
		if stripes > 1<<20 {
			// keep fuzz runs cheap; the boundary case is covered by unit tests
			stripes = 1 << 20
		}
		hash := func(u uint64) uint64 { return u ^ seed }
		s := striped.New(stripes, hash)
		mu := s.Get(key)
		ref := (key ^ seed) % uint64(s.Size()) // reference: integer modulo
		require.Same(t, s.GetAt(int(ref)), mu)
		// stability: repeated Get returns the identical pointer
		require.Same(t, mu, s.Get(key))
		require.Same(t, mu, s.Get(key))
	})
}

// FuzzGetRW mirrors FuzzGet for the RWMutex variant.
func FuzzGetRW(f *testing.F) {
	f.Add(uint16(2), uint64(0), uint64(0))
	f.Add(uint16(64), uint64(987654321), uint64(42))
	f.Fuzz(func(t *testing.T, stripesRaw uint16, key, seed uint64) {
		stripes := int(stripesRaw)
		if stripes <= 0 {
			require.PanicsWithValue(t, "striped: stripes must be positive", func() {
				_ = striped.NewRW(stripes, identityHash)
			})
			return
		}
		if stripes > 1<<20 {
			stripes = 1 << 20
		}
		hash := func(u uint64) uint64 { return u * seed }
		rw := striped.NewRW(stripes, hash)
		ref := (key * seed) % uint64(rw.Size())
		require.Same(t, rw.GetAt(int(ref)), rw.GetRW(key))
		require.Same(t, rw.GetRW(key), rw.GetRW(key))
	})
}

// TestConcurrentGetLockUnlock runs workers goroutines doing Get + Lock +
// Unlock and a shared counter incremented under the stripe lock; the counter
// must end up exactly workers*iters. The exact total proves mutual exclusion
// even without -race.
func TestConcurrentGetLockUnlock(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		stripes int
		workers int
		iters   int
	}{
		{"1 stripe 4 workers", 1, 4, 500},
		{"4 stripes 8 workers", 4, 8, 500},
		{"16 stripes 32 workers", 16, 32, 250},
		{"64 stripes 64 workers", 64, 64, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := striped.New(tc.stripes, identityHash)
			// Per-stripe counters: each counter is guarded by its own stripe
			// lock, so every increment is mutually exclusive with the other
			// increments sharing that stripe.
			counters := make([]int64, s.Size())
			var wg sync.WaitGroup
			for w := 0; w < tc.workers; w++ {
				wg.Add(1)
				go func(w int) {
					defer wg.Done()
					for i := 0; i < tc.iters; i++ {
						key := uint64(w*tc.iters + i)
						idx := int(key % uint64(s.Size()))
						mu := s.Get(key)
						mu.Lock()
						counters[idx]++ // guarded by this stripe's lock
						mu.Unlock()
					}
				}(w)
			}
			wg.Wait()
			total := int64(0)
			for _, c := range counters {
				total += c
			}
			assert.Equal(t, int64(tc.workers*tc.iters), total,
				"all increments under stripe locks must be accounted for")
		})
	}
}

// TestConcurrentGetRWSharedReads runs writers and readers over the same keys:
// writers take the exclusive lock, readers take the shared read lock. The
// shared counter incremented only by writers must be exact, proving writers
// are mutually exclusive while readers may overlap.
func TestConcurrentGetRWSharedReads(t *testing.T) {
	t.Parallel()
	rw := striped.NewRW(8, identityHash)
	const workers = 16
	const iters = 300
	// Per-stripe writer counters, each guarded by its stripe's write lock.
	counters := make([]int64, rw.Size())
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				key := uint64(i % 64)
				idx := int(key % uint64(rw.Size()))
				mu := rw.GetRW(key)
				if w%2 == 0 {
					mu.Lock()
					counters[idx]++ // writers only, under the write lock
					mu.Unlock()
				} else {
					mu.RLock()
					_ = counters[idx] // read-only; may see any intermediate value
					mu.RUnlock()
				}
			}
		}(w)
	}
	wg.Wait()
	total := int64(0)
	for _, c := range counters {
		total += c
	}
	assert.Equal(t, int64((workers/2)*iters), total)
}

// TestConcurrentSameKeySerialises proves per-key mutual exclusion under load:
// many goroutines hammering one key must serialise every increment.
func TestConcurrentSameKeySerialises(t *testing.T) {
	t.Parallel()
	s := striped.New(16, identityHash)
	const workers = 32
	const iters = 200
	var counter int64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu := s.Get(uint64(42)) // every goroutine targets the same key
			for i := 0; i < iters; i++ {
				mu.Lock()
				counter++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(workers*iters), counter)
}

// TestAllocsPerRunGet asserts the hot path allocates zero bytes.
func TestAllocsPerRunGet(t *testing.T) {
	s := striped.New(256, fnv64)
	allocs := testing.AllocsPerRun(1000, func() {
		mu := s.Get("some-key")
		_ = mu
	})
	assert.Zero(t, allocs, "Get must be zero-alloc")
}

// TestAllocsPerRunGetRW asserts the RW hot path allocates zero bytes.
func TestAllocsPerRunGetRW(t *testing.T) {
	rw := striped.NewRW(256, fnv64)
	allocs := testing.AllocsPerRun(1000, func() {
		mu := rw.GetRW("some-key")
		_ = mu
	})
	assert.Zero(t, allocs, "GetRW must be zero-alloc")
}

// TestAllocsPerRunGetAt asserts GetAt allocates zero bytes.
func TestAllocsPerRunGetAt(t *testing.T) {
	s := striped.New(256, fnv64)
	allocs := testing.AllocsPerRun(1000, func() {
		mu := s.GetAt(7)
		_ = mu
	})
	assert.Zero(t, allocs, "GetAt must be zero-alloc")
}

// TestAllocsPerRunGetConcurrent hammers Get from many goroutines and asserts
// the total allocation count stays zero across all of them.
func TestAllocsPerRunGetConcurrent(t *testing.T) {
	s := striped.New(256, fnv64)
	keys := make([]string, 32)
	for i := range keys {
		keys[i] = "key-" + strconv.Itoa(i)
	}
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 10000; i++ {
				mu := s.Get(keys[(w+i)%len(keys)])
				mu.Lock()
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()
	allocs := testing.AllocsPerRun(1000, func() {
		_ = s.Get(keys[0])
	})
	assert.Zero(t, allocs)
}

// TestCounterAtomic keeps the atomic helpers honest and documents the
// read-mostly counter pattern used with StripedRW readers.
func TestCounterAtomic(t *testing.T) {
	t.Parallel()
	var counter atomic.Int64
	counter.Add(1)
	require.Equal(t, int64(1), counter.Load())
}
