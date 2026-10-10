package ttlcache_test

import (
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

func BenchmarkTTLCacheGet(b *testing.B) {
	clock := newFakeClock()
	c := ttlcache.NewWithClock[int, int](4096, time.Hour, clock.Now)
	for i := range 4096 {
		c.Put(i, i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		c.Get(i & 4095)
	}
}

func BenchmarkTTLCachePutUpdate(b *testing.B) {
	clock := newFakeClock()
	c := ttlcache.NewWithClock[int, int](4096, time.Hour, clock.Now)
	for i := range 4096 {
		c.Put(i, i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		c.Put(i&4095, i)
	}
}
