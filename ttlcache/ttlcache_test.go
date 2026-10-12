package ttlcache_test

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/ttlcache"
)

// fakeClock is a deterministic, race-free clock for tests.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func TestPutGetPeek(t *testing.T) {
	clock := newFakeClock()
	c := ttlcache.NewWithClock[string, int](4, time.Minute, clock.Now)
	require.Equal(t, 4, c.Capacity())
	require.Equal(t, time.Minute, c.TTL())

	c.Put("a", 1)
	require.Equal(t, 1, c.Len())

	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, 1, v)

	v, ok = c.Peek("a")
	require.True(t, ok)
	require.Equal(t, 1, v)

	_, ok = c.Get("missing")
	require.False(t, ok)
	require.False(t, c.Contains("missing"))
	require.True(t, c.Contains("a"))
}

func TestExpiry(t *testing.T) {
	clock := newFakeClock()
	c := ttlcache.NewWithClock[string, int](4, time.Minute, clock.Now)
	c.Put("a", 1)

	clock.Advance(59 * time.Second)
	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, 1, v)

	clock.Advance(2 * time.Second) // now 61s > 60s TTL
	_, ok = c.Get("a")
	require.False(t, ok, "entry must expire after its TTL")

	require.False(t, c.Contains("a"))
	require.Zero(t, c.Len(), "expired entry must be dropped on access")
}

func TestPeekAndContainsDoNotReviveExpired(t *testing.T) {
	clock := newFakeClock()
	c := ttlcache.NewWithClock[string, int](4, time.Second, clock.Now)
	c.Put("a", 1)
	clock.Advance(2 * time.Second)

	_, ok := c.Peek("a")
	require.False(t, ok, "Peek must not return expired values")
	require.False(t, c.Contains("a"))
	require.Equal(t, 1, c.Len(), "Peek/Contains must not mutate the map")
}

func TestPutWithTTLOverrides(t *testing.T) {
	clock := newFakeClock()
	c := ttlcache.NewWithClock[string, int](4, time.Minute, clock.Now)

	c.PutWithTTL("short", 1, 5*time.Second)
	c.PutWithTTL("long", 2, 10*time.Minute)

	clock.Advance(6 * time.Second)
	_, ok := c.Get("short")
	require.False(t, ok, "short TTL must expire first")
	v, ok := c.Get("long")
	require.True(t, ok)
	require.Equal(t, 2, v)

	// non-positive TTL removes the entry
	c.PutWithTTL("long", 3, 0)
	require.False(t, c.Contains("long"))
}

func TestPutRefreshExtendsTTL(t *testing.T) {
	clock := newFakeClock()
	c := ttlcache.NewWithClock[string, int](4, time.Minute, clock.Now)

	c.Put("a", 1)
	clock.Advance(50 * time.Second)
	c.Put("a", 2) // refresh

	clock.Advance(20 * time.Second) // 70s after the first put, 20s after the refresh
	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, 2, v, "updating a key must reset its TTL")
}

func TestLRUEviction(t *testing.T) {
	clock := newFakeClock()
	c := ttlcache.NewWithClock[string, int](2, time.Hour, clock.Now)

	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a") // promote a; b becomes the eviction candidate
	c.Put("c", 3)

	require.False(t, c.Contains("b"))
	require.True(t, c.Contains("a"))
	require.True(t, c.Contains("c"))
	require.Equal(t, []string{"a", "c"}, c.Keys(), "Keys go from LRU to MRU")
}

func TestCapacityOne(t *testing.T) {
	clock := newFakeClock()
	c := ttlcache.NewWithClock[string, int](1, time.Hour, clock.Now)

	c.Put("a", 1)
	c.Put("a", 2)
	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, 2, v, "capacity 1 must still allow in-place updates")

	c.Put("b", 3)
	require.False(t, c.Contains("a"))
	require.True(t, c.Contains("b"))
}

func TestCleanup(t *testing.T) {
	clock := newFakeClock()
	c := ttlcache.NewWithClock[string, int](8, 10*time.Second, clock.Now)

	c.Put("a", 1)
	c.Put("b", 2)
	clock.Advance(5 * time.Second)
	c.Put("c", 3) // expires 5s later than a and b

	require.Equal(t, 0, c.Cleanup(), "nothing expired yet")
	clock.Advance(6 * time.Second) // 11s: a and b expired, c still alive

	require.Equal(t, 2, c.Cleanup())
	require.Equal(t, 1, c.Len())
	require.True(t, c.Contains("c"))
	require.Equal(t, 0, c.Cleanup(), "second sweep finds nothing")
}

func TestRemoveClearAndKeys(t *testing.T) {
	clock := newFakeClock()
	c := ttlcache.NewWithClock[int, int](4, time.Minute, clock.Now)
	for i := range 4 {
		c.Put(i, i)
	}

	require.True(t, c.Remove(1))
	require.False(t, c.Remove(1), "Remove must report true exactly once")
	require.Equal(t, []int{0, 2, 3}, c.Keys())

	clock.Advance(2 * time.Minute)
	require.Empty(t, c.Keys(), "Keys must skip expired entries")

	c.Clear()
	require.Zero(t, c.Len())
	c.Put(9, 9)
	require.True(t, c.Contains(9), "cache must stay usable after Clear")
}

func TestConcurrentAccess(t *testing.T) {
	c := ttlcache.New[int, int](64, time.Minute)

	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 500 {
				key := (g*500 + i) % 128
				c.Put(key, key)
				c.Get(key)
				c.Contains(key)
				c.Len()
				c.Keys()
			}
		}()
	}
	wg.Wait()

	require.Equal(t, 64, c.Len(), "capacity must hold under concurrent writers")
}

func TestPanicsOnBadArguments(t *testing.T) {
	clock := newFakeClock()
	require.Panics(t, func() { ttlcache.NewWithClock[string, int](0, time.Minute, clock.Now) })
	require.Panics(t, func() { ttlcache.NewWithClock[string, int](-1, time.Minute, clock.Now) })
	require.Panics(t, func() { ttlcache.NewWithClock[string, int](4, 0, clock.Now) })
	require.Panics(t, func() { ttlcache.NewWithClock[string, int](4, -time.Second, clock.Now) })
	require.Panics(t, func() { ttlcache.NewWithClock[string, int](4, time.Minute, nil) })
}

// ---------------------------------------------------------------------------
// Table-driven coverage of every exported method: normal, boundary and invalid
// arguments.
// ---------------------------------------------------------------------------

// tcStep is one scripted action of a table-driven cache scenario.
type tcStep struct {
	kind     string // put, putTTL, get, peek, remove, advance, cleanup, clear
	key, val int
	ttl, d   time.Duration
}

// tcCase is one scenario: scripted steps applied to a fresh cache, then
// whole-state expectations.
type tcCase struct {
	name     string
	capacity int
	ttl      time.Duration
	steps    []tcStep
	wantLen  int
	wantKeys []int // LRU to MRU, unexpired only; nil means "expect empty"
	query    int
	wantVal  int
	wantOK   bool
}

// runTCCase replays the scenario against a real cache driven by a fake clock
// and asserts the final state.
func runTCCase(t *testing.T, tt tcCase) {
	t.Helper()
	clock := newFakeClock()
	capacity, ttl := tt.capacity, tt.ttl
	if capacity == 0 {
		capacity = 4
	}
	if ttl == 0 {
		ttl = time.Minute
	}
	c := ttlcache.NewWithClock[int, int](capacity, ttl, clock.Now)
	for _, s := range tt.steps {
		switch s.kind {
		case "put":
			c.Put(s.key, s.val)
		case "putTTL":
			c.PutWithTTL(s.key, s.val, s.ttl)
		case "get":
			_, _ = c.Get(s.key)
		case "peek":
			_, _ = c.Peek(s.key)
		case "remove":
			c.Remove(s.key)
		case "advance":
			clock.Advance(s.d)
		case "cleanup":
			c.Cleanup()
		case "clear":
			c.Clear()
		default:
			t.Fatalf("unknown step kind %q", s.kind)
		}
	}
	require.Equal(t, tt.wantLen, c.Len(), "Len")
	switch {
	case tt.wantKeys != nil:
		require.Equal(t, tt.wantKeys, c.Keys(), "Keys (LRU to MRU)")
	case tt.wantLen == 0:
		require.Empty(t, c.Keys(), "Keys")
	}
	gotVal, gotOK := c.Peek(tt.query)
	require.Equal(t, tt.wantOK, gotOK, "Peek(%d) ok", tt.query)
	require.Equal(t, tt.wantVal, gotVal, "Peek(%d) value", tt.query)
	gotVal, gotOK = c.Get(tt.query)
	require.Equal(t, tt.wantOK, gotOK, "Get(%d) ok", tt.query)
	require.Equal(t, tt.wantVal, gotVal, "Get(%d) value", tt.query)
	require.Equal(t, tt.wantOK, c.Contains(tt.query), "Contains(%d)", tt.query)
}

// TestConstructorPanicsExactMessages pins the exact panic message of every
// rejected constructor argument, for both constructors.
func TestConstructorPanicsExactMessages(t *testing.T) {
	clock := newFakeClock()
	tests := []struct {
		name  string
		call  func()
		panic string
	}{
		{"New zero capacity", func() { ttlcache.New[int, int](0, time.Minute) },
			"ttlcache: capacity must be positive"},
		{"New negative capacity", func() { ttlcache.New[int, int](-1, time.Minute) },
			"ttlcache: capacity must be positive"},
		{"New min int capacity", func() { ttlcache.New[int, int](int(^uint(0)>>1)*-1-1, time.Minute) },
			"ttlcache: capacity must be positive"},
		{"New zero ttl", func() { ttlcache.New[int, int](4, 0) },
			"ttlcache: ttl must be positive"},
		{"New negative ttl", func() { ttlcache.New[int, int](4, -time.Second) },
			"ttlcache: ttl must be positive"},
		{"NewWithClock zero capacity", func() { ttlcache.NewWithClock[int, int](0, time.Minute, clock.Now) },
			"ttlcache: capacity must be positive"},
		{"NewWithClock negative capacity", func() { ttlcache.NewWithClock[int, int](-5, time.Minute, clock.Now) },
			"ttlcache: capacity must be positive"},
		{"NewWithClock zero ttl", func() { ttlcache.NewWithClock[int, int](4, 0, clock.Now) },
			"ttlcache: ttl must be positive"},
		{"NewWithClock negative ttl", func() { ttlcache.NewWithClock[int, int](4, -time.Nanosecond, clock.Now) },
			"ttlcache: ttl must be positive"},
		{"NewWithClock min duration ttl", func() { ttlcache.NewWithClock[int, int](4, -1<<63, clock.Now) },
			"ttlcache: ttl must be positive"},
		{"NewWithClock nil clock", func() { ttlcache.NewWithClock[int, int](4, time.Minute, nil) },
			"ttlcache: nil clock"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.PanicsWithValue(t, tt.panic, tt.call)
		})
	}
}

// TestPutGetTable covers the normal and boundary round trips of Put, Get,
// Peek and Contains on a query key.
func TestPutGetTable(t *testing.T) {
	tests := []tcCase{
		{
			name:  "get on the empty cache misses",
			steps: nil,
			query: 1, wantVal: 0, wantOK: false,
		},
		{
			name:  "put then get hits",
			steps: []tcStep{{kind: "put", key: 1, val: 10}},
			query: 1, wantVal: 10, wantOK: true,
		},
		{
			name: "repeated get keeps hitting",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "get", key: 1}, {kind: "get", key: 1}, {kind: "get", key: 1},
			},
			query: 1, wantVal: 10, wantOK: true,
		},
		{
			name: "overwrite wins",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 1, val: 20},
			},
			query: 1, wantVal: 20, wantOK: true,
		},
		{
			name:  "zero value is a hit",
			steps: []tcStep{{kind: "put", key: 1, val: 0}},
			query: 1, wantVal: 0, wantOK: true,
		},
		{
			name:  "absent key next to a present one",
			steps: []tcStep{{kind: "put", key: 1, val: 10}},
			query: 2, wantVal: 0, wantOK: false,
		},
		{
			name: "removed key misses afterwards",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "remove", key: 1},
			},
			query: 1, wantVal: 0, wantOK: false,
		},
		{
			name: "cleared cache misses afterwards",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "clear"},
			},
			query: 1, wantVal: 0, wantOK: false,
		},
		{
			name:  "negative keys work",
			steps: []tcStep{{kind: "put", key: -7, val: 77}},
			query: -7, wantVal: 77, wantOK: true,
		},
		{
			name:  "zero key works",
			steps: []tcStep{{kind: "put", key: 0, val: 5}},
			query: 0, wantVal: 5, wantOK: true,
		},
		{
			name: "peek does not affect the value",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "peek", key: 1}, {kind: "peek", key: 1},
			},
			query: 1, wantVal: 10, wantOK: true,
		},
		{
			name: "put after remove re-inserts",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "remove", key: 1},
				{kind: "put", key: 1, val: 11},
			},
			query: 1, wantVal: 11, wantOK: true,
		},
	}
	for _, tt := range tests {
		tt.wantLen = 0
		if tt.steps != nil && tt.name != "removed key misses afterwards" && tt.name != "cleared cache misses afterwards" {
			tt.wantLen = 1
		}
		if tt.name == "removed key misses afterwards" || tt.name == "cleared cache misses afterwards" {
			tt.wantLen = 0
		}
		t.Run(tt.name, func(t *testing.T) { runTCCase(t, tt) })
	}
}

// TestExpiryBoundaryTable pins lazy expiry down to the exact boundary: an
// entry expires exactly at now+ttl, not one nanosecond later.
func TestExpiryBoundaryTable(t *testing.T) {
	const ttl = 10 * time.Second
	tests := []tcCase{
		{
			name: "alive at zero advance",
			ttl:  ttl,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "advance", d: 0},
			},
			wantLen: 1, wantKeys: []int{1},
			query: 1, wantVal: 10, wantOK: true,
		},
		{
			name: "alive just before the boundary",
			ttl:  ttl,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "advance", d: 9999 * time.Millisecond},
			},
			wantLen: 1, wantKeys: []int{1},
			query: 1, wantVal: 10, wantOK: true,
		},
		{
			name: "expired exactly at the boundary",
			ttl:  ttl,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "advance", d: ttl},
			},
			wantLen: 1, // expired but not yet swept: Peek/Get do not drop it here
			query:   1, wantVal: 0, wantOK: false,
		},
		{
			name: "expired one nanosecond past the boundary",
			ttl:  ttl,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "advance", d: ttl + time.Nanosecond},
			},
			wantLen: 1,
			query:   1, wantVal: 0, wantOK: false,
		},
		{
			name: "get drops the expired entry",
			ttl:  ttl,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "advance", d: 2 * ttl},
				{kind: "get", key: 1},
			},
			wantLen: 0,
			query:   1, wantVal: 0, wantOK: false,
		},
		{
			name: "peek keeps the expired entry (documented leak)",
			ttl:  ttl,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "advance", d: 2 * ttl},
				{kind: "peek", key: 1},
			},
			wantLen: 1,
			query:   1, wantVal: 0, wantOK: false,
		},
		{
			name: "keys skips the expired entry",
			ttl:  ttl,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "advance", d: ttl + time.Second},
				{kind: "putTTL", key: 2, val: 20, ttl: time.Hour},
			},
			wantLen: 2, wantKeys: []int{2},
			query: 2, wantVal: 20, wantOK: true,
		},
		{
			name: "cleanup sweeps the expired entry",
			ttl:  ttl,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "advance", d: 2 * ttl},
				{kind: "cleanup"},
			},
			wantLen: 0,
			query:   1, wantVal: 0, wantOK: false,
		},
		{
			name: "shorter per-key TTL expires first",
			ttl:  ttl,
			steps: []tcStep{
				{kind: "putTTL", key: 1, val: 10, ttl: time.Second},
				{kind: "put", key: 2, val: 20},
				{kind: "advance", d: 2 * time.Second},
			},
			wantLen: 2, wantKeys: []int{2},
			query: 2, wantVal: 20, wantOK: true,
		},
		{
			name: "longer per-key TTL outlives the default window",
			ttl:  ttl,
			steps: []tcStep{
				{kind: "putTTL", key: 1, val: 10, ttl: time.Hour},
				{kind: "advance", d: 2 * ttl},
			},
			wantLen: 1, wantKeys: []int{1},
			query: 1, wantVal: 10, wantOK: true,
		},
		{
			name: "re-put refreshes the expiry",
			ttl:  ttl,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "advance", d: 8 * time.Second},
				{kind: "put", key: 1, val: 11},
				{kind: "advance", d: 8 * time.Second}, // 16s after the first put
			},
			wantLen: 1, wantKeys: []int{1},
			query: 1, wantVal: 11, wantOK: true,
		},
		{
			name: "expired entry still counts for Len until swept",
			ttl:  ttl,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "advance", d: 2 * ttl},
			},
			wantLen: 2,
			query:   1, wantVal: 0, wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runTCCase(t, tt) })
	}
}

// TestPutWithTTLTable covers the explicit-TTL variant, above all the
// non-positive TTL remove semantics.
func TestPutWithTTLTable(t *testing.T) {
	tests := []tcCase{
		{
			name: "zero TTL removes a present key",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "putTTL", key: 1, val: 11, ttl: 0},
			},
			wantLen: 0,
			query:   1, wantVal: 0, wantOK: false,
		},
		{
			name: "negative TTL removes a present key",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "putTTL", key: 1, val: 11, ttl: -time.Second},
			},
			wantLen: 0,
			query:   1, wantVal: 0, wantOK: false,
		},
		{
			name: "min duration TTL removes a present key",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "putTTL", key: 1, val: 11, ttl: -1 << 63},
			},
			wantLen: 0,
			query:   1, wantVal: 0, wantOK: false,
		},
		{
			name:    "zero TTL on an absent key is a no-op",
			steps:   []tcStep{{kind: "putTTL", key: 1, val: 10, ttl: 0}},
			wantLen: 0,
			query:   1, wantVal: 0, wantOK: false,
		},
		{
			name:    "negative TTL on an absent key is a no-op",
			steps:   []tcStep{{kind: "putTTL", key: 1, val: 10, ttl: -time.Hour}},
			wantLen: 0,
			query:   1, wantVal: 0, wantOK: false,
		},
		{
			name: "nanosecond TTL expires on the next advance",
			steps: []tcStep{
				{kind: "putTTL", key: 1, val: 10, ttl: time.Nanosecond},
				{kind: "advance", d: time.Millisecond},
			},
			wantLen: 1,
			query:   1, wantVal: 0, wantOK: false,
		},
		{
			name: "per-key TTL replaces the default",
			steps: []tcStep{
				{kind: "putTTL", key: 1, val: 10, ttl: time.Hour},
				{kind: "advance", d: 2 * time.Minute},
			},
			wantLen: 1, wantKeys: []int{1},
			query: 1, wantVal: 10, wantOK: true,
		},
		{
			name: "update with a fresh TTL refreshes value and expiry",
			steps: []tcStep{
				{kind: "putTTL", key: 1, val: 10, ttl: time.Second},
				{kind: "putTTL", key: 1, val: 11, ttl: time.Hour},
				{kind: "advance", d: time.Minute},
			},
			wantLen: 1, wantKeys: []int{1},
			query: 1, wantVal: 11, wantOK: true,
		},
		{
			name:     "update of a present key never evicts",
			capacity: 2,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "putTTL", key: 1, val: 11, ttl: time.Hour},
			},
			wantLen: 2, wantKeys: []int{2, 1},
			query: 1, wantVal: 11, wantOK: true,
		},
		{
			name:     "insert on a full cache evicts the LRU",
			capacity: 2,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "putTTL", key: 3, val: 30, ttl: time.Hour},
			},
			wantLen: 2, wantKeys: []int{2, 3},
			query: 3, wantVal: 30, wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runTCCase(t, tt) })
	}
}

// TestLRUEvictionTable covers capacity bound and recency order, including the
// difference between Get (promotes) and Peek (does not promote).
func TestLRUEvictionTable(t *testing.T) {
	tests := []tcCase{
		{
			name:     "filling to capacity evicts nothing",
			capacity: 3,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "put", key: 3, val: 30},
			},
			wantLen: 3, wantKeys: []int{1, 2, 3},
			query: 1, wantVal: 10, wantOK: true,
		},
		{
			name:     "one past capacity evicts the oldest",
			capacity: 3,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "put", key: 3, val: 30},
				{kind: "put", key: 4, val: 40},
			},
			wantLen: 3, wantKeys: []int{2, 3, 4},
			query: 1, wantVal: 0, wantOK: false,
		},
		{
			name:     "get promotes and changes the eviction victim",
			capacity: 3,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "put", key: 3, val: 30},
				{kind: "get", key: 1},
				{kind: "put", key: 4, val: 40},
			},
			wantLen: 3, wantKeys: []int{3, 1, 4},
			query: 2, wantVal: 0, wantOK: false,
		},
		{
			name:     "peek does not promote",
			capacity: 3,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "put", key: 3, val: 30},
				{kind: "peek", key: 1},
				{kind: "put", key: 4, val: 40},
			},
			wantLen: 3, wantKeys: []int{2, 3, 4},
			query: 1, wantVal: 0, wantOK: false,
		},
		{
			name:     "put update promotes",
			capacity: 3,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "put", key: 3, val: 30},
				{kind: "put", key: 1, val: 11},
				{kind: "put", key: 4, val: 40},
			},
			wantLen: 3, wantKeys: []int{3, 1, 4},
			query: 1, wantVal: 11, wantOK: true,
		},
		{
			name:     "put same key never grows beyond one entry",
			capacity: 3,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 1, val: 11},
				{kind: "put", key: 1, val: 12},
				{kind: "put", key: 1, val: 13},
			},
			wantLen: 1, wantKeys: []int{1},
			query: 1, wantVal: 13, wantOK: true,
		},
		{
			name:     "capacity one churn",
			capacity: 1,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "put", key: 3, val: 30},
			},
			wantLen: 1, wantKeys: []int{3},
			query: 3, wantVal: 30, wantOK: true,
		},
		{
			name:     "capacity two with interleaved promotion",
			capacity: 2,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "get", key: 1},
				{kind: "put", key: 3, val: 30},
				{kind: "get", key: 3},
				{kind: "put", key: 4, val: 40},
			},
			wantLen: 2, wantKeys: []int{3, 4},
			query: 1, wantVal: 0, wantOK: false,
		},
		{
			name:     "remove of the LRU leaves the order intact",
			capacity: 3,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "put", key: 3, val: 30},
				{kind: "remove", key: 1},
				{kind: "put", key: 4, val: 40},
			},
			wantLen: 3, wantKeys: []int{2, 3, 4},
			query: 2, wantVal: 20, wantOK: true,
		},
		{
			name:     "remove of the MRU leaves the order intact",
			capacity: 3,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "put", key: 3, val: 30},
				{kind: "remove", key: 3},
			},
			wantLen: 2, wantKeys: []int{1, 2},
			query: 1, wantVal: 10, wantOK: true,
		},
		{
			name:     "expired entries also serve as eviction victims",
			capacity: 2,
			ttl:      time.Minute,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "advance", d: 2 * time.Minute},
				{kind: "put", key: 3, val: 30},
			},
			wantLen: 2,
			query:   3, wantVal: 30, wantOK: true,
		},
		{
			name:     "clear then reuse keeps working",
			capacity: 2,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "clear"},
				{kind: "put", key: 3, val: 30},
				{kind: "put", key: 4, val: 40},
			},
			wantLen: 2, wantKeys: []int{3, 4},
			query: 3, wantVal: 30, wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runTCCase(t, tt) })
	}
}

// TestRemoveTable covers the Remove return value and its effect on the order.
func TestRemoveTable(t *testing.T) {
	tests := []struct {
		name   string
		steps  []tcStep
		key    int
		wantOK bool
	}{
		{
			name: "remove on the empty cache", key: 1, wantOK: false,
		},
		{
			name:  "remove a present key",
			steps: []tcStep{{kind: "put", key: 1, val: 10}},
			key:   1, wantOK: true,
		},
		{
			name: "second remove reports false",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "remove", key: 1},
			},
			key: 1, wantOK: false,
		},
		{
			name: "remove an absent key of a populated cache",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
			},
			key: 3, wantOK: false,
		},
		{
			name: "remove an expired key reports true (it was stored)",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "advance", d: 2 * time.Minute},
			},
			key: 1, wantOK: true,
		},
		{
			name: "remove preserves the other entries",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "put", key: 3, val: 30},
				{kind: "remove", key: 2},
			},
			key: 1, wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := newFakeClock()
			c := ttlcache.NewWithClock[int, int](4, time.Minute, clock.Now)
			for _, s := range tt.steps {
				switch s.kind {
				case "put":
					c.Put(s.key, s.val)
				case "remove":
					c.Remove(s.key)
				case "advance":
					clock.Advance(s.d)
				}
			}
			require.Equal(t, tt.wantOK, c.Remove(tt.key), "Remove(%d)", tt.key)
		})
	}
}

// TestCleanupTable covers the eager sweep: counts, survivors and idempotence.
func TestCleanupTable(t *testing.T) {
	tests := []struct {
		name      string
		steps     []tcStep
		wantCount int
		wantLen   int
		wantKeys  []int
	}{
		{
			name:      "nothing expired",
			steps:     []tcStep{{kind: "put", key: 1, val: 10}},
			wantCount: 0, wantLen: 1, wantKeys: []int{1},
		},
		{
			name: "everything expired",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "advance", d: 2 * time.Minute},
			},
			wantCount: 2, wantLen: 0,
		},
		{
			name: "partial expiry",
			steps: []tcStep{
				{kind: "putTTL", key: 1, val: 10, ttl: time.Second},
				{kind: "putTTL", key: 2, val: 20, ttl: time.Hour},
				{kind: "advance", d: time.Minute},
			},
			wantCount: 1, wantLen: 1, wantKeys: []int{2},
		},
		{
			name: "second sweep finds nothing",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "advance", d: 2 * time.Minute},
				{kind: "cleanup"},
			},
			wantCount: 0, wantLen: 0,
		},
		{
			name: "cleanup on the empty cache",
			steps: []tcStep{
				{kind: "clear"},
			},
			wantCount: 0, wantLen: 0,
		},
		{
			name: "cleanup preserves the LRU order of survivors",
			steps: []tcStep{
				{kind: "putTTL", key: 1, val: 10, ttl: time.Second},
				{kind: "put", key: 2, val: 20},
				{kind: "putTTL", key: 3, val: 30, ttl: time.Second},
				{kind: "put", key: 4, val: 40},
				{kind: "advance", d: 2 * time.Second},
			},
			wantCount: 2, wantLen: 2, wantKeys: []int{2, 4},
		},
		{
			name: "cleanup after clear",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "clear"},
			},
			wantCount: 0, wantLen: 0,
		},
		{
			name: "mixed expiry in the middle of the order",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "putTTL", key: 2, val: 20, ttl: time.Second},
				{kind: "put", key: 3, val: 30},
				{kind: "advance", d: 2 * time.Second},
			},
			wantCount: 1, wantLen: 2, wantKeys: []int{1, 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := newFakeClock()
			c := ttlcache.NewWithClock[int, int](8, time.Minute, clock.Now)
			for _, s := range tt.steps {
				switch s.kind {
				case "put":
					c.Put(s.key, s.val)
				case "putTTL":
					c.PutWithTTL(s.key, s.val, s.ttl)
				case "advance":
					clock.Advance(s.d)
				case "cleanup":
					c.Cleanup()
				case "clear":
					c.Clear()
				}
			}
			require.Equal(t, tt.wantCount, c.Cleanup(), "Cleanup removed")
			require.Equal(t, tt.wantLen, c.Len(), "Len after Cleanup")
			if tt.wantKeys == nil {
				require.Empty(t, c.Keys())
			} else {
				require.Equal(t, tt.wantKeys, c.Keys())
			}
		})
	}
}

// TestKeysOrderTable pins the documented least- to most-recently-used order.
func TestKeysOrderTable(t *testing.T) {
	tests := []tcCase{
		{
			name:  "empty cache",
			steps: nil,
			query: 0,
		},
		{
			name:    "single key",
			steps:   []tcStep{{kind: "put", key: 1, val: 1}},
			wantLen: 1, wantKeys: []int{1},
			query: 1, wantVal: 1, wantOK: true,
		},
		{
			name: "insertion order",
			steps: []tcStep{
				{kind: "put", key: 3, val: 30},
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
			},
			wantLen: 3, wantKeys: []int{3, 1, 2},
			query: 3, wantVal: 30, wantOK: true,
		},
		{
			name: "get reorders to MRU",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "put", key: 3, val: 30},
				{kind: "get", key: 1},
			},
			wantLen: 3, wantKeys: []int{2, 3, 1},
			query: 1, wantVal: 10, wantOK: true,
		},
		{
			name: "peek does not reorder",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "peek", key: 1},
			},
			wantLen: 2, wantKeys: []int{1, 2},
			query: 1, wantVal: 10, wantOK: true,
		},
		{
			name: "update reorders to MRU",
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "put", key: 1, val: 11},
			},
			wantLen: 2, wantKeys: []int{2, 1},
			query: 1, wantVal: 11, wantOK: true,
		},
		{
			name: "expired keys are skipped",
			steps: []tcStep{
				{kind: "putTTL", key: 1, val: 10, ttl: time.Second},
				{kind: "put", key: 2, val: 20},
				{kind: "advance", d: 2 * time.Second},
			},
			wantLen: 2, wantKeys: []int{2},
			query: 2, wantVal: 20, wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runTCCase(t, tt) })
	}
}

// TestCapacityEdgeTable pins the capacity bound on the degenerate shapes.
func TestCapacityEdgeTable(t *testing.T) {
	tests := []tcCase{
		{
			name:     "capacity one holds exactly one",
			capacity: 1,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 1, val: 11},
			},
			wantLen: 1, wantKeys: []int{1},
			query: 1, wantVal: 11, wantOK: true,
		},
		{
			name:     "capacity two holds exactly two",
			capacity: 2,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
			},
			wantLen: 2, wantKeys: []int{1, 2},
			query: 1, wantVal: 10, wantOK: true,
		},
		{
			name:     "many unique keys stay bounded",
			capacity: 3,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "put", key: 2, val: 20},
				{kind: "put", key: 3, val: 30},
				{kind: "put", key: 4, val: 40},
				{kind: "put", key: 5, val: 50},
				{kind: "put", key: 6, val: 60},
			},
			wantLen: 3, wantKeys: []int{4, 5, 6},
			query: 4, wantVal: 40, wantOK: true,
		},
		{
			name:     "expired entries free no capacity until swept or evicted",
			capacity: 2,
			ttl:      time.Minute,
			steps: []tcStep{
				{kind: "put", key: 1, val: 10},
				{kind: "advance", d: 2 * time.Minute},
				{kind: "put", key: 2, val: 20},
			},
			wantLen: 2, // 1 is expired but still counted until eviction/sweep
			query:   2, wantVal: 20, wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runTCCase(t, tt) })
	}
}

// ---------------------------------------------------------------------------
// Differential testing against a map + explicit expiry timestamp model.
// ---------------------------------------------------------------------------

// The reference model mirrors the observable ttlcache semantics: a map with an
// explicit expiry timestamp per entry plus an LRU order slice (least recent
// first). It deliberately mirrors the current Peek behaviour on expired
// entries (report absent, keep the entry): the leak is pinned by
// TestPeekAndContainsDoNotReviveExpired and reported separately, so the model
// tracks the observable behaviour rather than hiding it.

const (
	tcCapacity  = 3
	tcTTL       = time.Second
	tcUniverse  = 8 // key pool of the randomized/fuzzed traffic
	tcMaxOps    = 64
	tcProbeKeys = tcUniverse + 2
)

// tcModelEntry is one modelled cache entry.
type tcModelEntry struct {
	val       int
	expiresAt time.Time
}

// tcModel is the reference implementation: map + explicit expiry + LRU order.
type tcModel struct {
	cap   int
	ttl   time.Duration
	clock *fakeClock
	m     map[int]tcModelEntry
	order []int // least recently used first
}

// newTCModel returns an empty model with the given capacity and default TTL.
func newTCModel(capacity int, ttl time.Duration, clock *fakeClock) *tcModel {
	return &tcModel{cap: capacity, ttl: ttl, clock: clock, m: map[int]tcModelEntry{}}
}

// now returns the current modelled time.
func (m *tcModel) now() time.Time { return m.clock.Now() }

// expired mirrors the cache's expiry predicate (expiresAt is exclusive).
func (m *tcModel) expired(e tcModelEntry) bool { return !m.now().Before(e.expiresAt) }

// touch moves k to the most-recently-used end of the order.
func (m *tcModel) touch(k int) {
	for i, x := range m.order {
		if x == k {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	m.order = append(m.order, k)
}

// put stores k under the default TTL.
func (m *tcModel) put(k, v int) { m.putWithTTL(k, v, m.ttl) }

// putWithTTL stores k under an explicit TTL; a non-positive TTL removes k.
func (m *tcModel) putWithTTL(k, v int, ttl time.Duration) {
	if ttl <= 0 {
		m.remove(k)
		return
	}
	if e, ok := m.m[k]; ok {
		e.val = v
		e.expiresAt = m.now().Add(ttl)
		m.m[k] = e
		m.touch(k)
		return
	}
	for len(m.m) >= m.cap {
		if len(m.order) == 0 {
			break
		}
		victim := m.order[0]
		m.order = m.order[1:]
		delete(m.m, victim)
	}
	m.m[k] = tcModelEntry{val: v, expiresAt: m.now().Add(ttl)}
	m.order = append(m.order, k)
}

// get returns the value of k and promotes it; an expired entry is dropped and
// reported absent, exactly like the implementation.
func (m *tcModel) get(k int) (int, bool) {
	e, ok := m.m[k]
	if !ok {
		return 0, false
	}
	if m.expired(e) {
		m.remove(k)
		return 0, false
	}
	m.touch(k)
	return e.val, true
}

// peek returns the value of k without touching the order and without dropping
// an expired entry, mirroring the implementation including its leak.
func (m *tcModel) peek(k int) (int, bool) {
	e, ok := m.m[k]
	if !ok || m.expired(e) {
		return 0, false
	}
	return e.val, true
}

// contains reports whether k is present and unexpired.
func (m *tcModel) contains(k int) bool { _, ok := m.peek(k); return ok }

// remove deletes k and reports whether it was stored (even when expired).
func (m *tcModel) remove(k int) bool {
	if _, ok := m.m[k]; !ok {
		return false
	}
	delete(m.m, k)
	for i, x := range m.order {
		if x == k {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	return true
}

// cleanup sweeps all expired entries and returns how many were dropped.
func (m *tcModel) cleanup() int {
	removed := 0
	for k, e := range m.m {
		if m.expired(e) {
			m.remove(k)
			removed++
		}
	}
	return removed
}

// clear drops everything.
func (m *tcModel) clear() {
	m.m = map[int]tcModelEntry{}
	m.order = nil
}

// len returns the stored entry count including not-yet-swept expired ones.
func (m *tcModel) len() int { return len(m.m) }

// keys returns the unexpired keys from least- to most-recently-used.
func (m *tcModel) keys() []int {
	out := []int{}
	for _, k := range m.order {
		if e, ok := m.m[k]; ok && !m.expired(e) {
			out = append(out, k)
		}
	}
	return out
}

// tcOp is one replayable operation of the cache model test.
type tcOp struct {
	kind byte // 0 put, 1 putWithTTL, 2 get, 3 peek, 4 contains, 5 remove, 6 cleanup, 7 advance, 8 clear, 9 keys check
	key  int
	val  int
	ttl  time.Duration
	d    time.Duration
}

var (
	tcTTLOptions     = []time.Duration{0, -time.Second, time.Millisecond, time.Minute}
	tcAdvanceOptions = []time.Duration{time.Millisecond, 500 * time.Millisecond, 2 * time.Second, time.Minute}
)

// decodeTCOps turns a byte stream into an operation sequence, four bytes per
// operation. It is shared by the seeded differential test and the fuzz target,
// and with the injected clock the replay is fully deterministic.
func decodeTCOps(data []byte) []tcOp {
	ops := make([]tcOp, 0, tcMaxOps)
	for i := 0; i+3 < len(data) && len(ops) < tcMaxOps; i += 4 {
		ops = append(ops, tcOp{
			kind: data[i] % 10,
			key:  int(data[i+1]) % tcUniverse,
			val:  int(data[i+2]),
			ttl:  tcTTLOptions[int(data[i+3])%len(tcTTLOptions)],
			d:    tcAdvanceOptions[int(data[i+3])%len(tcAdvanceOptions)],
		})
	}
	return ops
}

// checkTCModel asserts the full observable state of the cache against the
// model: Len, Keys order and Peek/Contains over the whole key probe range.
func checkTCModel(t *testing.T, c *ttlcache.Cache[int, int], m *tcModel, where string) {
	t.Helper()
	require.Equal(t, len(m.m), len(m.order), "%s: model invariant", where)
	require.Equal(t, m.len(), c.Len(), "%s: Len", where)
	require.Equal(t, m.keys(), c.Keys(), "%s: Keys (LRU to MRU)", where)
	for k := 0; k < tcProbeKeys; k++ {
		wantVal, wantOK := m.peek(k)
		gotVal, gotOK := c.Peek(k)
		require.Equal(t, wantOK, gotOK, "%s: Peek(%d) ok", where, k)
		require.Equal(t, wantVal, gotVal, "%s: Peek(%d) value", where, k)
		require.Equal(t, wantOK, c.Contains(k), "%s: Contains(%d)", where, k)
	}
}

// runTCModel replays ops against a real cache and the reference model, both
// driven by the same injected fake clock, and checks full model agreement
// after every single operation.
func runTCModel(t *testing.T, ops []tcOp) {
	t.Helper()
	clock := newFakeClock()
	c := ttlcache.NewWithClock[int, int](tcCapacity, tcTTL, clock.Now)
	m := newTCModel(tcCapacity, tcTTL, clock)
	for i, o := range ops {
		where := fmt.Sprintf("op %d (kind %d, key %d)", i, o.kind, o.key)
		switch o.kind {
		case 0:
			c.Put(o.key, o.val)
			m.put(o.key, o.val)
		case 1:
			c.PutWithTTL(o.key, o.val, o.ttl)
			m.putWithTTL(o.key, o.val, o.ttl)
		case 2:
			gotVal, gotOK := c.Get(o.key)
			wantVal, wantOK := m.get(o.key)
			require.Equal(t, wantOK, gotOK, "%s: Get ok", where)
			require.Equal(t, wantVal, gotVal, "%s: Get value", where)
		case 3:
			gotVal, gotOK := c.Peek(o.key)
			wantVal, wantOK := m.peek(o.key)
			require.Equal(t, wantOK, gotOK, "%s: Peek ok", where)
			require.Equal(t, wantVal, gotVal, "%s: Peek value", where)
		case 4:
			require.Equal(t, m.contains(o.key), c.Contains(o.key), "%s: Contains", where)
		case 5:
			require.Equal(t, m.remove(o.key), c.Remove(o.key), "%s: Remove", where)
		case 6:
			require.Equal(t, m.cleanup(), c.Cleanup(), "%s: Cleanup", where)
		case 7:
			clock.Advance(o.d)
		case 8:
			c.Clear()
			m.clear()
		case 9:
			// Keys/order is verified by the full check below
		}
		checkTCModel(t, c, m, where)
	}
}

// TestAgainstModelRandomOps replays randomised Put/PutWithTTL/Get/Peek/
// Contains/Remove/Cleanup/advance/Clear traffic against the map + expiry
// model and checks full agreement after every step.
func TestAgainstModelRandomOps(t *testing.T) {
	seeds := []uint64{1, 7, 42, 1337, 20261012}
	for _, seed := range seeds {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			r := rand.New(rand.NewPCG(seed, 0x233))
			ops := make([]tcOp, 0, 300)
			for range 300 {
				ops = append(ops, tcOp{
					kind: byte(r.IntN(10)),
					key:  r.IntN(tcUniverse),
					val:  r.IntN(1000),
					ttl:  tcTTLOptions[r.IntN(len(tcTTLOptions))],
					d:    tcAdvanceOptions[r.IntN(len(tcAdvanceOptions))],
				})
			}
			runTCModel(t, ops)
		})
	}
}

// TestConcurrentMixedWorkload hammers every method from eight goroutines at
// once. Cache is documented as goroutine-safe, so this must be race-clean
// (CI runs it under -race) and must keep the capacity bound.
func TestConcurrentMixedWorkload(t *testing.T) {
	c := ttlcache.New[int, int](64, time.Hour)

	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := range 500 {
				key := (id*500 + i) % 200
				switch i % 8 {
				case 0:
					c.Put(key, i)
				case 1:
					c.PutWithTTL(key, i, time.Minute)
				case 2:
					_, _ = c.Get(key)
				case 3:
					_, _ = c.Peek(key)
				case 4:
					_ = c.Contains(key)
				case 5:
					c.Remove(key)
				case 6:
					_ = c.Len()
					_ = c.Keys()
				case 7:
					c.Cleanup()
				}
			}
		}(g)
	}
	wg.Wait()

	require.LessOrEqual(t, c.Len(), 64, "capacity must hold under concurrent writers")
	require.LessOrEqual(t, len(c.Keys()), 64)
	for _, k := range c.Keys() {
		require.True(t, c.Contains(k), "every listed key must still be present")
	}
}

// TestConcurrentPutsAllVisible proves no concurrent Put is lost: disjoint
// keys written from eight goroutines must all be readable afterwards.
func TestConcurrentPutsAllVisible(t *testing.T) {
	const (
		goroutines = 8
		perG       = 125
	)
	c := ttlcache.New[int, int](goroutines*perG, time.Hour)

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := range perG {
				key := id*perG + i
				c.Put(key, key*2)
			}
		}(g)
	}
	wg.Wait()

	require.Equal(t, goroutines*perG, c.Len())
	for k := range goroutines * perG {
		v, ok := c.Get(k)
		require.True(t, ok, "key %d must survive concurrent writes", k)
		require.Equal(t, k*2, v)
	}
}

// TestHotPathAllocations guards the steady-state hot paths against unexpected
// allocations: all keys pre-exist so no entry or map growth is involved.
func TestHotPathAllocations(t *testing.T) {
	build := func() *ttlcache.Cache[int, int] {
		clock := newFakeClock()
		c := ttlcache.NewWithClock[int, int](16, time.Hour, clock.Now)
		for i := range 16 {
			c.Put(i, i)
		}
		return c
	}

	t.Run("Get hit", func(t *testing.T) {
		c := build()
		allocs := testing.AllocsPerRun(1000, func() {
			_, _ = c.Get(3)
		})
		require.Zero(t, allocs)
	})

	t.Run("Get miss", func(t *testing.T) {
		c := build()
		allocs := testing.AllocsPerRun(1000, func() {
			_, _ = c.Get(1000)
		})
		require.Zero(t, allocs)
	})

	t.Run("Peek hit", func(t *testing.T) {
		c := build()
		allocs := testing.AllocsPerRun(1000, func() {
			_, _ = c.Peek(3)
		})
		require.Zero(t, allocs)
	})

	t.Run("Contains hit", func(t *testing.T) {
		c := build()
		allocs := testing.AllocsPerRun(1000, func() {
			_ = c.Contains(3)
		})
		require.Zero(t, allocs)
	})

	t.Run("Put update of a present key", func(t *testing.T) {
		c := build()
		allocs := testing.AllocsPerRun(1000, func() {
			c.Put(3, 3)
		})
		require.Zero(t, allocs, "updating an existing entry must not allocate")
	})

	t.Run("Remove of a present key re-added", func(t *testing.T) {
		c := build()
		allocs := testing.AllocsPerRun(1000, func() {
			c.Remove(3)
			c.Put(3, 3)
		})
		// Put re-inserts a fresh entry each run: exactly one allocation
		require.Equal(t, 1.0, allocs, "remove + re-insert allocates exactly one entry")
	})
}
