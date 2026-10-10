package lru_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/lru"
)

func TestEviction(t *testing.T) {
	c := lru.New[string, int](2)

	_, _, evicted := c.Put("a", 1)
	require.False(t, evicted)
	_, _, evicted = c.Put("b", 2)
	require.False(t, evicted)

	key, value, evicted := c.Put("c", 3)
	require.True(t, evicted)
	require.Equal(t, "a", key, "the least recently used entry must be evicted")
	require.Equal(t, 1, value)

	require.Equal(t, 2, c.Len())
	require.False(t, c.Contains("a"))
	require.True(t, c.Contains("b"))
	require.True(t, c.Contains("c"))
}

func TestGetPromotes(t *testing.T) {
	c := lru.New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)

	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, 1, v)

	ek, _, ev := c.Put("c", 3)
	require.True(t, ev)
	require.Equal(t, "b", ek, "Get must promote the accessed entry")
	require.True(t, c.Contains("a"))
	require.True(t, c.Contains("c"))
}

func TestPeekDoesNotPromote(t *testing.T) {
	c := lru.New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)

	v, ok := c.Peek("a")
	require.True(t, ok)
	require.Equal(t, 1, v)

	ek, _, ev := c.Put("c", 3)
	require.True(t, ev)
	require.Equal(t, "a", ek, "Peek must not change the recency order")
}

func TestContainsDoesNotPromote(t *testing.T) {
	c := lru.New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)

	require.True(t, c.Contains("a"))

	ek, _, ev := c.Put("c", 3)
	require.True(t, ev)
	require.Equal(t, "a", ek, "Contains must not change the recency order")
}

func TestPutUpdateNeverEvicts(t *testing.T) {
	c := lru.New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)

	_, _, evicted := c.Put("a", 11)
	require.False(t, evicted, "updating an existing key must not evict")
	require.True(t, c.Contains("b"))

	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, 11, v)

	// a was promoted by the update, so b becomes the eviction candidate
	ek, _, ev := c.Put("c", 3)
	require.True(t, ev)
	require.Equal(t, "b", ek)
}

func TestCapacityOne(t *testing.T) {
	c := lru.New[string, int](1)

	_, _, evicted := c.Put("a", 1)
	require.False(t, evicted)
	_, _, evicted = c.Put("a", 2)
	require.False(t, evicted, "capacity 1 must still allow in-place updates")

	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, 2, v)

	ek, _, ev := c.Put("b", 3)
	require.True(t, ev)
	require.Equal(t, "a", ek)
	require.False(t, c.Contains("a"))
}

func TestKeysOrder(t *testing.T) {
	c := lru.New[int, int](4)
	for i := range 4 {
		c.Put(i, i)
	}

	c.Get(1) // promotes 1
	c.Get(0) // promotes 0
	require.Equal(t, []int{2, 3, 1, 0}, c.Keys(), "Keys must go from LRU to MRU")

	var iterated []int
	for k := range c.All() {
		iterated = append(iterated, k)
	}
	require.Equal(t, []int{2, 3, 1, 0}, iterated)
}

func TestRemoveAndClear(t *testing.T) {
	c := lru.New[int, int](3)
	c.Put(1, 1)
	c.Put(2, 2)

	require.True(t, c.Remove(1))
	require.False(t, c.Remove(1), "Remove must report true exactly once")

	c.Put(3, 3)
	require.Equal(t, []int{2, 3}, c.Keys())

	m, ok := c.Get(3)
	require.True(t, ok)
	require.Equal(t, 3, m)

	c.Clear()
	require.Zero(t, c.Len())
	require.False(t, c.Contains(2))
	require.Empty(t, c.Keys())

	c.Put(9, 9)
	v, ok := c.Get(9)
	require.True(t, ok)
	require.Equal(t, 9, v, "cache must stay usable after Clear")
}

func TestCapacityAndAccessors(t *testing.T) {
	c := lru.New[string, int](4)
	require.Equal(t, 4, c.Capacity())

	_, ok := c.Get("missing")
	require.False(t, ok)
	_, ok = c.Peek("missing")
	require.False(t, ok)
	require.Zero(t, c.Len())
}

func TestNewPanicsOnBadCapacity(t *testing.T) {
	require.Panics(t, func() { lru.New[string, int](0) })
	require.Panics(t, func() { lru.New[string, int](-1) })
}

func TestGetDoesNotAllocate(t *testing.T) {
	c := lru.New[int, int](128)
	for i := range 128 {
		c.Put(i, i)
	}

	allocs := testing.AllocsPerRun(100, func() { c.Get(64) })
	require.Zero(t, allocs, "Get must not allocate (node is moved, not recreated)")
}

func TestKeysAreIndependentSnapshot(t *testing.T) {
	c := lru.New[int, int](4)
	c.Put(1, 1)
	c.Put(2, 2)

	keys := c.Keys()
	c.Put(3, 3)

	require.Equal(t, []int{1, 2}, keys, "Keys must return a snapshot")
	require.True(t, slices.Equal(keys, []int{1, 2}))
}
