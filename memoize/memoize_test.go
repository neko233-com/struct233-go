package memoize_test

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/memoize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClock is a manually advanced time source for deterministic expiration
// tests.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

// newFakeClock returns a fakeClock parked at a fixed instant.
func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
}

// Now returns the current fake time.
func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the fake time forward by d.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// countingFn returns a supplier function whose value is its 1-based call
// number and which records how often it ran.
type countingFn struct {
	calls atomic.Int64
}

// fn returns the supplier function to hand to Memoize / MemoizeExpiring.
func (c *countingFn) fn() int {
	return int(c.calls.Add(1))
}

// calls returns how often the function has run.
func (c *countingFn) count() int64 {
	return c.calls.Load()
}

func TestMemoizeTable(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "int value is cached",
			run: func(t *testing.T) {
				var calls atomic.Int64
				s := memoize.Memoize(func() int { calls.Add(1); return 42 })
				for range 5 {
					require.Equal(t, 42, s.Get())
				}
				require.Equal(t, int64(1), calls.Load())
			},
		},
		{
			name: "int zero value is cached and not recomputed",
			run: func(t *testing.T) {
				var calls atomic.Int64
				s := memoize.Memoize(func() int { calls.Add(1); return 0 })
				require.Zero(t, s.Get())
				require.Zero(t, s.Get())
				require.Equal(t, int64(1), calls.Load(), "the zero value must be a valid cached result")
			},
		},
		{
			name: "negative int value is cached",
			run: func(t *testing.T) {
				var calls atomic.Int64
				s := memoize.Memoize(func() int { calls.Add(1); return -7 })
				require.Equal(t, -7, s.Get())
				require.Equal(t, -7, s.Get())
				require.Equal(t, int64(1), calls.Load())
			},
		},
		{
			name: "string value is cached",
			run: func(t *testing.T) {
				var calls atomic.Int64
				s := memoize.Memoize(func() string { calls.Add(1); return "hello" })
				require.Equal(t, "hello", s.Get())
				require.Equal(t, "hello", s.Get())
				require.Equal(t, int64(1), calls.Load())
			},
		},
		{
			name: "empty string is a valid cached value",
			run: func(t *testing.T) {
				var calls atomic.Int64
				s := memoize.Memoize(func() string { calls.Add(1); return "" })
				require.Empty(t, s.Get())
				require.Empty(t, s.Get())
				require.Equal(t, int64(1), calls.Load())
			},
		},
		{
			name: "bool false is a valid cached value",
			run: func(t *testing.T) {
				var calls atomic.Int64
				s := memoize.Memoize(func() bool { calls.Add(1); return false })
				require.False(t, s.Get())
				require.False(t, s.Get())
				require.Equal(t, int64(1), calls.Load())
			},
		},
		{
			name: "float value is cached",
			run: func(t *testing.T) {
				var calls atomic.Int64
				s := memoize.Memoize(func() float64 { calls.Add(1); return 3.5 })
				require.Equal(t, 3.5, s.Get())
				require.Equal(t, 3.5, s.Get())
				require.Equal(t, int64(1), calls.Load())
			},
		},
		{
			name: "struct value is cached",
			run: func(t *testing.T) {
				type point struct {
					X, Y int
				}
				var calls atomic.Int64
				s := memoize.Memoize(func() point { calls.Add(1); return point{X: 1, Y: 2} })
				require.Equal(t, point{X: 1, Y: 2}, s.Get())
				require.Equal(t, point{X: 1, Y: 2}, s.Get())
				require.Equal(t, int64(1), calls.Load())
			},
		},
		{
			name: "pointer identity is preserved",
			run: func(t *testing.T) {
				var calls atomic.Int64
				s := memoize.Memoize(func() *int { calls.Add(1); v := 42; return &v })
				first := s.Get()
				second := s.Get()
				require.Same(t, first, second, "the same pointer must be returned on every Get")
				require.Equal(t, int64(1), calls.Load())
			},
		},
		{
			name: "nil pointer is a valid cached value",
			run: func(t *testing.T) {
				var calls atomic.Int64
				s := memoize.Memoize(func() *int { calls.Add(1); return nil })
				require.Nil(t, s.Get())
				require.Nil(t, s.Get())
				require.Equal(t, int64(1), calls.Load(), "nil must be cached like any other value")
			},
		},
		{
			name: "slice value is cached",
			run: func(t *testing.T) {
				var calls atomic.Int64
				s := memoize.Memoize(func() []int { calls.Add(1); return []int{1, 2, 3} })
				require.Equal(t, []int{1, 2, 3}, s.Get())
				require.Equal(t, []int{1, 2, 3}, s.Get())
				require.Equal(t, int64(1), calls.Load())
			},
		},
		{
			name: "map reference is preserved",
			run: func(t *testing.T) {
				var calls atomic.Int64
				s := memoize.Memoize(func() map[string]int { calls.Add(1); return map[string]int{"a": 1} })
				m := s.Get()
				m["a"] = 99
				require.Equal(t, map[string]int{"a": 99}, s.Get(), "Get must return the same map, not a copy")
				require.Equal(t, int64(1), calls.Load())
			},
		},
		{
			name: "interface value is cached",
			run: func(t *testing.T) {
				var calls atomic.Int64
				s := memoize.Memoize(func() error { calls.Add(1); return errors.New("boom") })
				require.EqualError(t, s.Get(), "boom")
				require.EqualError(t, s.Get(), "boom")
				require.Equal(t, int64(1), calls.Load())
			},
		},
		{
			name: "nil interface is a valid cached value",
			run: func(t *testing.T) {
				var calls atomic.Int64
				s := memoize.Memoize(func() error { calls.Add(1); return nil })
				require.NoError(t, s.Get())
				require.NoError(t, s.Get())
				require.Equal(t, int64(1), calls.Load())
			},
		},
		{
			name: "channel identity is preserved",
			run: func(t *testing.T) {
				var calls atomic.Int64
				s := memoize.Memoize(func() chan int { calls.Add(1); return make(chan int, 1) })
				ch := s.Get()
				require.True(t, ch == s.Get(), "the same channel must be returned on every Get")
				require.Equal(t, int64(1), calls.Load())
			},
		},
		{
			name: "function is lazy until the first Get",
			run: func(t *testing.T) {
				var calls atomic.Int64
				s := memoize.Memoize(func() int { calls.Add(1); return 1 })
				require.Equal(t, int64(0), calls.Load(), "construction must not run fn")
				require.Zero(t, s.Stats())
				require.Equal(t, 1, s.Get())
				require.Equal(t, int64(1), calls.Load())
			},
		},
		{
			name: "many sequential Gets reuse the value",
			run: func(t *testing.T) {
				c := &countingFn{}
				s := memoize.Memoize(c.fn)
				for i := range 100 {
					require.Equal(t, 1, s.Get(), "iteration %d", i)
				}
				require.Equal(t, int64(1), c.count())
				require.Equal(t, memoize.Stats{Hits: 99, Misses: 1}, s.Stats())
			},
		},
		{
			name: "closure capture is frozen at the first computation",
			run: func(t *testing.T) {
				base := 10
				var calls atomic.Int64
				s := memoize.Memoize(func() int { calls.Add(1); return base })
				require.Equal(t, 10, s.Get())
				base = 20 // after the first Get this must not matter
				require.Equal(t, 10, s.Get())
				require.Equal(t, 10, s.Get())
				require.Equal(t, int64(1), calls.Load())
			},
		},
		{
			name: "stats count misses and hits",
			run: func(t *testing.T) {
				s := memoize.Memoize(func() int { return 1 })
				require.Equal(t, memoize.Stats{}, s.Stats())
				s.Get()
				require.Equal(t, memoize.Stats{Hits: 0, Misses: 1}, s.Stats())
				s.Get()
				s.Get()
				require.Equal(t, memoize.Stats{Hits: 2, Misses: 1}, s.Stats())
			},
		},
		{
			name: "MemoizeExpiring caches within the ttl like Memoize",
			run: func(t *testing.T) {
				clock := newFakeClock()
				c := &countingFn{}
				s := memoize.MemoizeExpiring(c.fn, 10*time.Second, memoize.WithClock(clock.Now))
				require.Equal(t, 1, s.Get())
				require.Equal(t, 1, s.Get())
				require.Equal(t, int64(1), c.count())
				require.Equal(t, memoize.Stats{Hits: 1, Misses: 1}, s.Stats())
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, tc.run)
	}
}

func TestMemoizeExpiringTable(t *testing.T) {
	type step struct {
		advance    time.Duration
		invalidate bool
		gets       int
		wantValue  int
	}
	tests := []struct {
		name      string
		ttl       time.Duration
		steps     []step
		wantCalls int64
	}{
		{
			name:      "stays cached while fresh",
			ttl:       10 * time.Second,
			steps:     []step{{gets: 3, wantValue: 1}},
			wantCalls: 1,
		},
		{
			name: "advance below ttl keeps the value",
			ttl:  10 * time.Second,
			steps: []step{
				{advance: 5 * time.Second, gets: 2, wantValue: 1},
				{advance: 4 * time.Second, gets: 1, wantValue: 1},
			},
			wantCalls: 1,
		},
		{
			name: "advance exactly ttl expires at the deadline",
			ttl:  10 * time.Second,
			steps: []step{
				{gets: 1, wantValue: 1},
				{advance: 10 * time.Second, gets: 1, wantValue: 2},
			},
			wantCalls: 2,
		},
		{
			name: "advance past ttl recomputes",
			ttl:  10 * time.Second,
			steps: []step{
				{gets: 1, wantValue: 1},
				{advance: 11 * time.Second, gets: 1, wantValue: 2},
			},
			wantCalls: 2,
		},
		{
			name: "one recomputation per expiration",
			ttl:  10 * time.Second,
			steps: []step{
				{gets: 1, wantValue: 1},
				{advance: 11 * time.Second, gets: 5, wantValue: 2},
			},
			wantCalls: 2,
		},
		{
			name: "recomputed value is cached until the next expiration",
			ttl:  10 * time.Second,
			steps: []step{
				{gets: 1, wantValue: 1},
				{advance: 11 * time.Second, gets: 1, wantValue: 2},
				{advance: 5 * time.Second, gets: 2, wantValue: 2},
				{advance: 6 * time.Second, gets: 1, wantValue: 3},
			},
			wantCalls: 3,
		},
		{
			name: "nanosecond ttl",
			ttl:  1,
			steps: []step{
				{gets: 1, wantValue: 1},
				{advance: 1, gets: 1, wantValue: 2},
			},
			wantCalls: 2,
		},
		{
			name: "zero advance never expires",
			ttl:  time.Second,
			steps: []step{
				{advance: 0, gets: 10, wantValue: 1},
				{advance: 0, gets: 1, wantValue: 1},
			},
			wantCalls: 1,
		},
		{
			name: "invalidate forces a recompute inside the ttl",
			ttl:  time.Hour,
			steps: []step{
				{gets: 1, wantValue: 1},
				{invalidate: true, gets: 1, wantValue: 2},
			},
			wantCalls: 2,
		},
		{
			name:      "invalidate before the first Get is a no-op",
			ttl:       time.Hour,
			steps:     []step{{invalidate: true, gets: 1, wantValue: 1}},
			wantCalls: 1,
		},
		{
			name: "double invalidate triggers one recompute",
			ttl:  time.Hour,
			steps: []step{
				{gets: 1, wantValue: 1},
				{invalidate: true},
				{invalidate: true, gets: 2, wantValue: 2},
			},
			wantCalls: 2,
		},
		{
			name: "invalidate after expiry recomputes once",
			ttl:  10 * time.Second,
			steps: []step{
				{gets: 1, wantValue: 1},
				{advance: 20 * time.Second, invalidate: true, gets: 1, wantValue: 2},
			},
			wantCalls: 2,
		},
		{
			name: "repeated expirations recompute each time",
			ttl:  10 * time.Second,
			steps: []step{
				{gets: 1, wantValue: 1},
				{advance: 10 * time.Second, gets: 1, wantValue: 2},
				{advance: 10 * time.Second, gets: 1, wantValue: 3},
				{advance: 10 * time.Second, gets: 1, wantValue: 4},
			},
			wantCalls: 4,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock()
			c := &countingFn{}
			s := memoize.MemoizeExpiring(c.fn, tc.ttl, memoize.WithClock(clock.Now))
			for i, st := range tc.steps {
				clock.Advance(st.advance)
				if st.invalidate {
					s.Invalidate()
				}
				for g := range st.gets {
					require.Equal(t, st.wantValue, s.Get(), "step %d, get %d", i, g)
				}
			}
			require.Equal(t, tc.wantCalls, c.count())
		})
	}
}

func TestPanicTable(t *testing.T) {
	errBoom := errors.New("boom")
	tests := []struct {
		name       string
		panicValue any
	}{
		{name: "string panic", panicValue: "boom"},
		{name: "error panic", panicValue: errBoom},
		{name: "int panic", panicValue: 7},
		{name: "struct panic", panicValue: struct{ X int }{X: 1}},
		{name: "wrapped error panic", panicValue: fmt.Errorf("wrapped: %w", errBoom)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int64
			s := memoize.Memoize(func() int {
				if calls.Add(1) == 1 {
					panic(tc.panicValue)
				}
				return 42
			})

			require.PanicsWithValue(t, tc.panicValue, func() { s.Get() })
			require.Equal(t, memoize.Stats{Hits: 0, Misses: 1}, s.Stats(),
				"the failed attempt must count as a miss")

			require.Equal(t, 42, s.Get(), "the panic must not be cached, the retry succeeds")
			require.Equal(t, 42, s.Get())
			require.Equal(t, int64(2), calls.Load())
			require.Equal(t, memoize.Stats{Hits: 1, Misses: 2}, s.Stats())
		})
	}
}

func TestConstructorPanicsTable(t *testing.T) {
	okFn := func() int { return 1 }
	tests := []struct {
		name      string
		build     func()
		wantPanic string
	}{
		{
			name:      "Memoize with nil fn",
			build:     func() { memoize.Memoize[int](nil) },
			wantPanic: "memoize: nil fn",
		},
		{
			name:      "MemoizeExpiring with nil fn",
			build:     func() { memoize.MemoizeExpiring[int](nil, time.Second) },
			wantPanic: "memoize: nil fn",
		},
		{
			name:      "MemoizeExpiring with zero ttl",
			build:     func() { memoize.MemoizeExpiring(okFn, 0) },
			wantPanic: "memoize: ttl must be positive",
		},
		{
			name:      "MemoizeExpiring with negative ttl",
			build:     func() { memoize.MemoizeExpiring(okFn, -time.Second) },
			wantPanic: "memoize: ttl must be positive",
		},
		{
			name:      "MemoizeExpiring with nil fn and bad ttl panics on ttl first",
			build:     func() { memoize.MemoizeExpiring[int](nil, -1) },
			wantPanic: "memoize: ttl must be positive",
		},
		{
			name:      "nil clock",
			build:     func() { memoize.Memoize(okFn, memoize.WithClock(nil)) },
			wantPanic: "memoize: nil clock",
		},
		{
			name:      "nil clock on expiring supplier",
			build:     func() { memoize.MemoizeExpiring(okFn, time.Second, memoize.WithClock(nil)) },
			wantPanic: "memoize: nil clock",
		},
		{
			name:      "nil option",
			build:     func() { memoize.Memoize(okFn, nil) },
			wantPanic: "memoize: nil option",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.PanicsWithValue(t, tc.wantPanic, tc.build)
		})
	}
}

func TestInvalidateTable(t *testing.T) {
	tests := []struct {
		name      string
		ops       []string // "get" or "invalidate"
		wantVals  []int    // values returned by the "get" ops, in order
		wantCalls int64
		wantStats memoize.Stats
	}{
		{
			name:      "invalidate before the first get",
			ops:       []string{"invalidate", "get"},
			wantVals:  []int{1},
			wantCalls: 1,
			wantStats: memoize.Stats{Hits: 0, Misses: 1},
		},
		{
			name:      "invalidate after get recomputes",
			ops:       []string{"get", "invalidate", "get"},
			wantVals:  []int{1, 2},
			wantCalls: 2,
			wantStats: memoize.Stats{Hits: 0, Misses: 2},
		},
		{
			name:      "double invalidate recomputes once",
			ops:       []string{"get", "invalidate", "invalidate", "get"},
			wantVals:  []int{1, 2},
			wantCalls: 2,
			wantStats: memoize.Stats{Hits: 0, Misses: 2},
		},
		{
			name:      "many invalidates then one get",
			ops:       []string{"invalidate", "invalidate", "invalidate", "invalidate", "invalidate", "get"},
			wantVals:  []int{1},
			wantCalls: 1,
			wantStats: memoize.Stats{Hits: 0, Misses: 1},
		},
		{
			name:      "invalidate keeps the hit and miss counters",
			ops:       []string{"get", "get", "invalidate", "get"},
			wantVals:  []int{1, 1, 2},
			wantCalls: 2,
			wantStats: memoize.Stats{Hits: 1, Misses: 2},
		},
		{
			name:      "gets after an invalidate reuse the new value",
			ops:       []string{"get", "invalidate", "get", "get", "get"},
			wantVals:  []int{1, 2, 2, 2},
			wantCalls: 2,
			wantStats: memoize.Stats{Hits: 2, Misses: 2},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &countingFn{}
			s := memoize.Memoize(c.fn)
			var got []int
			for _, op := range tc.ops {
				switch op {
				case "get":
					got = append(got, s.Get())
				case "invalidate":
					s.Invalidate()
				default:
					t.Fatalf("unknown op %q", op)
				}
			}
			require.Equal(t, tc.wantVals, got)
			require.Equal(t, tc.wantCalls, c.count())
			require.Equal(t, tc.wantStats, s.Stats())
		})
	}
}

func TestConcurrentGetComputesOnce(t *testing.T) {
	var calls atomic.Int64
	s := memoize.Memoize(func() int {
		calls.Add(1)
		time.Sleep(20 * time.Millisecond) // widen the race window
		return 42
	})

	const n = 100
	results := make([]int, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(n)
	for i := range n {
		go func() {
			defer wg.Done()
			<-start
			results[i] = s.Get()
		}()
	}
	close(start)
	wg.Wait()

	require.Equal(t, int64(1), calls.Load(), "concurrent Gets must share a single computation")
	for i, v := range results {
		require.Equal(t, 42, v, "result %d", i)
	}
	require.Equal(t, memoize.Stats{Hits: n - 1, Misses: 1}, s.Stats())
}

func TestConcurrentExpiringRecomputesOnceAfterExpiry(t *testing.T) {
	clock := newFakeClock()
	var calls atomic.Int64
	s := memoize.MemoizeExpiring(func() int {
		calls.Add(1)
		time.Sleep(10 * time.Millisecond)
		return int(calls.Load())
	}, 10*time.Second, memoize.WithClock(clock.Now))

	require.Equal(t, 1, s.Get())
	clock.Advance(11 * time.Second) // expire the cached value

	const n = 100
	results := make([]int, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(n)
	for i := range n {
		go func() {
			defer wg.Done()
			<-start
			results[i] = s.Get()
		}()
	}
	close(start)
	wg.Wait()

	require.Equal(t, int64(2), calls.Load(), "an expiration must trigger exactly one recomputation")
	for i, v := range results {
		require.Equal(t, 2, v, "result %d", i)
	}
}

func TestConcurrentMixedOperationsStayConsistent(t *testing.T) {
	clock := newFakeClock()
	s := memoize.MemoizeExpiring(func() int { return 42 }, time.Minute, memoize.WithClock(clock.Now))

	const workers = 32
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for i := range 200 {
				switch i % 3 {
				case 0:
					assert.Equal(t, 42, s.Get())
				case 1:
					s.Invalidate()
				case 2:
					clock.Advance(time.Second)
				}
			}
		}()
	}
	wg.Wait()
	require.Equal(t, 42, s.Get())
}

func TestRepeatedPanicIsNeverCached(t *testing.T) {
	var calls atomic.Int64
	s := memoize.Memoize(func() int {
		calls.Add(1)
		panic("always")
	})
	for range 3 {
		require.PanicsWithValue(t, "always", func() { s.Get() })
	}
	require.Equal(t, int64(3), calls.Load(), "every attempt must retry the function")
	require.Equal(t, memoize.Stats{Hits: 0, Misses: 3}, s.Stats())
}

func TestPanicRecoversAndKeepsMutexUsable(t *testing.T) {
	var trigger atomic.Bool
	trigger.Store(true)
	s := memoize.Memoize(func() int {
		if trigger.Load() {
			panic("boom")
		}
		return 5
	})
	require.Panics(t, func() { s.Get() })
	trigger.Store(false)
	require.Equal(t, 5, s.Get(), "the supplier must stay usable after a panicking fn")
	require.Equal(t, 5, s.Get())
}

func TestGetHitPathDoesNotAllocate(t *testing.T) {
	s := memoize.Memoize(func() int { return 7 })
	require.Equal(t, 7, s.Get()) // warm the cache

	allocs := testing.AllocsPerRun(1000, func() { _ = s.Get() })
	require.Zero(t, allocs, "the cached Get path must not allocate")
}

func TestExpiringGetHitPathDoesNotAllocate(t *testing.T) {
	clock := newFakeClock()
	s := memoize.MemoizeExpiring(func() int { return 7 }, time.Hour, memoize.WithClock(clock.Now))
	require.Equal(t, 7, s.Get()) // warm the cache

	allocs := testing.AllocsPerRun(1000, func() { _ = s.Get() })
	require.Zero(t, allocs, "the cached Get path must not allocate")
}

func TestZeroTTLIsRejectedButTinyTTLWorks(t *testing.T) {
	require.PanicsWithValue(t, "memoize: ttl must be positive",
		func() { memoize.MemoizeExpiring(func() int { return 1 }, 0) })

	clock := newFakeClock()
	s := memoize.MemoizeExpiring(func() int { return 1 }, time.Nanosecond, memoize.WithClock(clock.Now))
	s.Get()
	clock.Advance(time.Nanosecond)
	s.Get()
	require.Equal(t, memoize.Stats{Hits: 0, Misses: 2}, s.Stats())
}

func TestSupplierTypeIsGenericOverAny(t *testing.T) {
	type config struct {
		Name  string
		Ports []int
	}
	s := memoize.Memoize(func() *config { return &config{Name: "svc", Ports: []int{80, 443}} })
	cfg := s.Get()
	require.Equal(t, "svc", cfg.Name)
	require.Same(t, cfg, s.Get())
}
