package memoize_test

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/memoize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// refModel is a hand-written map+lock cache used as the reference
// implementation for the differential tests. It recomputes a fresh integer
// value on every cache miss and expires entries after ttl.
type refModel struct {
	mu      sync.Mutex
	clock   *fakeClock
	ttl     time.Duration // 0 means values never expire
	next    int           // value of the next computation
	calls   int64
	hits    uint64
	misses  uint64
	hasVal  bool
	value   int
	expires time.Time
}

// newRefModel returns a reference model with the given ttl (0 = no expiration).
func newRefModel(clock *fakeClock, ttl time.Duration) *refModel {
	return &refModel{clock: clock, ttl: ttl}
}

// Get returns the cached value or computes a new one, mirroring Supplier.Get.
func (m *refModel) Get() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.hasVal && (m.ttl == 0 || m.clock.Now().Before(m.expires)) {
		m.hits++
		return m.value
	}
	m.misses++
	m.calls++
	m.next++
	m.value = m.next
	m.hasVal = true
	if m.ttl > 0 {
		m.expires = m.clock.Now().Add(m.ttl)
	}
	return m.value
}

// Invalidate drops the cached value, mirroring Supplier.Invalidate.
func (m *refModel) Invalidate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hasVal = false
	m.expires = time.Time{}
}

// Stats returns the hit/miss counters as a memoize.Stats snapshot.
func (m *refModel) Stats() memoize.Stats {
	m.mu.Lock()
	defer m.mu.Unlock()
	return memoize.Stats{Hits: m.hits, Misses: m.misses}
}

// Calls returns how often the reference computation ran.
func (m *refModel) Calls() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// opKind enumerates the differential-sequence operations.
type opKind int

const (
	opGet opKind = iota
	opInvalidate
	opAdvance
)

// newPair returns a real Supplier and the matching reference model. Both hand
// out 1, 2, 3... on successive computations so results are directly
// comparable.
func newPair(clock *fakeClock, ttl time.Duration) (*memoize.Supplier[int], *refModel, *atomic.Int64) {
	next := &atomic.Int64{}
	s := memoize.MemoizeExpiring(func() int {
		return int(next.Add(1))
	}, ttl, memoize.WithClock(clock.Now))
	return s, newRefModel(clock, ttl), next
}

// diffRun executes one random operation sequence against the real Supplier and
// the reference model and fails on the first difference.
func diffRun(t *testing.T, rng *rand.Rand, nOps int, ttl time.Duration) {
	t.Helper()
	clock := newFakeClock()
	s, ref, next := newPair(clock, ttl)

	for i := range nOps {
		switch opKind(rng.Intn(3)) {
		case opGet:
			got, want := s.Get(), ref.Get()
			require.Equal(t, want, got, "op %d (Get): supplier and reference disagree", i)
		case opInvalidate:
			s.Invalidate()
			ref.Invalidate()
		case opAdvance:
			clock.Advance(time.Duration(rng.Int63n(int64(ttl) + 1)))
		}
	}
	require.Equal(t, ref.Stats(), s.Stats(), "hit/miss counters must match the reference")
	require.Equal(t, ref.Calls(), next.Load(), "computation counts must match the reference")
}

// TestDifferentialAgainstReferenceModel runs random Get/Invalidate/Advance
// sequences against Supplier and a hand-written map+lock cache.
func TestDifferentialAgainstReferenceModel(t *testing.T) {
	ttls := []time.Duration{time.Nanosecond, time.Second, time.Minute, time.Hour}
	seeds := []int64{1, 7, 42, 1234, 987654321}
	for _, seed := range seeds {
		for _, ttl := range ttls {
			t.Run(fmt.Sprintf("seed=%d/ttl=%s", seed, ttl), func(t *testing.T) {
				rng := rand.New(rand.NewSource(seed))
				diffRun(t, rng, 500, ttl)
			})
		}
	}
}

// TestDifferentialNoTTLExpiry runs the differential with an expiring supplier
// whose TTL is never reached, matching plain Memoize semantics.
func TestDifferentialNoTTLExpiry(t *testing.T) {
	clock := newFakeClock()
	s, ref, next := newPair(clock, time.Hour)
	rng := rand.New(rand.NewSource(2026))

	for i := range 1000 {
		switch opKind(rng.Intn(2)) {
		case opGet:
			require.Equal(t, ref.Get(), s.Get(), "op %d (Get)", i)
		case opInvalidate:
			s.Invalidate()
			ref.Invalidate()
		}
	}
	require.Equal(t, ref.Stats(), s.Stats())
	require.Equal(t, ref.Calls(), next.Load())
}

// TestDifferentialParallel runs the mixed Get/Invalidate/Advance workload
// concurrently against a Supplier and checks the invariants that must hold no
// matter how the operations interleave: every Get returns one of the values
// handed out by the single computation counter, and the hit/miss/call counters
// add up exactly.
func TestDifferentialParallel(t *testing.T) {
	clock := newFakeClock()
	s, _, next := newPair(clock, time.Minute)

	const workers = 16
	const opsPerWorker = 300
	var gets atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(workers)
	for w := range workers {
		go func(w int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(w)))
			<-start
			for range opsPerWorker {
				switch opKind(rng.Intn(3)) {
				case opGet:
					v := s.Get()
					gets.Add(1)
					assert.GreaterOrEqual(t, v, 1)
					assert.LessOrEqual(t, v, int(next.Load()))
				case opInvalidate:
					s.Invalidate()
				case opAdvance:
					clock.Advance(time.Duration(rng.Int63n(int64(time.Minute))))
				}
			}
		}(w)
	}
	close(start)
	wg.Wait()

	st := s.Stats()
	require.Equal(t, uint64(gets.Load()), st.Hits+st.Misses, "every Get is a hit or a miss")
	require.Equal(t, uint64(next.Load()), st.Misses, "every computation is counted as a miss")
	s.Invalidate()
	require.GreaterOrEqual(t, s.Get(), 1, "the supplier stays usable after the workload")
}

// FuzzSupplierOps fuzzes random operation sequences against the reference
// model with an injected fake clock, so runs are fully deterministic.
func FuzzSupplierOps(f *testing.F) {
	f.Add(uint8(20), int64(42), uint64(time.Second))
	f.Add(uint8(64), int64(7), uint64(time.Nanosecond))
	f.Add(uint8(255), int64(1), uint64(time.Hour))
	f.Fuzz(func(t *testing.T, nOps uint8, seed int64, ttlRaw uint64) {
		ttl := time.Duration(ttlRaw%uint64(time.Hour)) + 1 // 1ns .. 1h
		rng := rand.New(rand.NewSource(seed))
		clock := newFakeClock()
		s, ref, next := newPair(clock, ttl)

		for i := range int(nOps) {
			switch opKind(rng.Intn(3)) {
			case opGet:
				if got, want := s.Get(), ref.Get(); got != want {
					t.Fatalf("op %d: supplier=%d reference=%d", i, got, want)
				}
			case opInvalidate:
				s.Invalidate()
				ref.Invalidate()
			case opAdvance:
				clock.Advance(time.Duration(rng.Int63n(int64(ttl) + 1)))
			}
		}
		if ref.Stats() != s.Stats() {
			t.Fatalf("stats differ: supplier=%+v reference=%+v", s.Stats(), ref.Stats())
		}
		if ref.Calls() != next.Load() {
			t.Fatalf("calls differ: supplier=%d reference=%d", next.Load(), ref.Calls())
		}
	})
}
