package concurrent_test

import (
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/concurrent"
)

func TestSet(t *testing.T) {
	s := concurrent.NewHashSet(1, 2, 3)
	require.Equal(t, 3, s.Len())

	require.False(t, s.Add(3), "re-adding must report false")
	require.True(t, s.Add(4))
	require.True(t, s.Contains(4))
	require.False(t, s.Contains(5))

	require.True(t, s.Remove(4))
	require.False(t, s.Remove(4))

	items := s.ToSlice()
	require.Len(t, items, 3)
	slices.Sort(items)
	require.Equal(t, []int{1, 2, 3}, items)

	iterated := 0
	for range s.All() {
		iterated++
	}
	require.Equal(t, 3, iterated)

	s.Clear()
	require.Zero(t, s.Len())
}

func TestSetConcurrent(t *testing.T) {
	const goroutines, perGoroutine = 8, 500
	s := concurrent.NewHashSetWithCapacity[int](goroutines * perGoroutine)

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perGoroutine {
				key := g*perGoroutine + i
				s.Add(key)
				s.Contains(key)
				s.Len()
			}
		}()
	}
	wg.Wait()
	require.Equal(t, goroutines*perGoroutine, s.Len())

	for g := range goroutines / 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perGoroutine {
				s.Remove(g*perGoroutine + i)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, goroutines*perGoroutine/2, s.Len())
}

func TestMap(t *testing.T) {
	m := concurrent.NewHashMapWithCapacity[string, int](4)
	require.Zero(t, m.Len())

	old, existed := m.Put("a", 1)
	require.False(t, existed)
	require.Zero(t, old)

	old, existed = m.Put("a", 2)
	require.True(t, existed)
	require.Equal(t, 1, old)

	v, ok := m.Get("a")
	require.True(t, ok)
	require.Equal(t, 2, v)

	_, ok = m.Get("missing")
	require.False(t, ok)

	require.Equal(t, []string{"a"}, m.Keys())
	require.Equal(t, map[string]int{"a": 2}, m.Snapshot())

	count := 0
	for k, val := range m.All() {
		count++
		require.Equal(t, "a", k)
		require.Equal(t, 2, val)
	}
	require.Equal(t, 1, count)

	old, existed = m.Delete("a")
	require.True(t, existed)
	require.Equal(t, 2, old)
	_, existed = m.Delete("a")
	require.False(t, existed)

	m.Put("b", 1)
	m.Clear()
	require.Zero(t, m.Len())
}

func TestMapGetOrPut(t *testing.T) {
	m := concurrent.NewHashMap[string, int]()
	calls := 0

	v, loaded := m.GetOrPut("k", func() int { calls++; return 1 })
	require.False(t, loaded, "first GetOrPut must compute")
	require.Equal(t, 1, v)
	require.Equal(t, 1, calls)

	v, loaded = m.GetOrPut("k", func() int { calls++; return 2 })
	require.True(t, loaded, "second GetOrPut must hit the cache")
	require.Equal(t, 1, v)
	require.Equal(t, 1, calls, "fn must not run when the key already exists")
}

func TestMapGetOrPutConcurrent(t *testing.T) {
	m := concurrent.NewHashMap[int, int]()
	const goroutines = 16

	var (
		computed atomic.Int64
		wg       sync.WaitGroup
	)
	got := make([]int, goroutines)

	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, _ := m.GetOrPut(7, func() int {
				computed.Add(1)
				return 42
			})
			got[g] = v
		}()
	}
	wg.Wait()

	require.Equal(t, int64(1), computed.Load(), "fn must run exactly once under concurrency")
	require.Equal(t, 1, m.Len())
	for _, v := range got {
		require.Equal(t, 42, v)
	}
}

func TestSkipList(t *testing.T) {
	sl := concurrent.NewSkipListMap[int, string]()
	sl.Put(30, "c")
	sl.Put(10, "a")
	sl.Put(20, "b")

	v, ok := sl.Get(20)
	require.True(t, ok)
	require.Equal(t, "b", v)
	require.True(t, sl.ContainsKey(10))
	require.False(t, sl.ContainsKey(99))
	require.Equal(t, 3, sl.Len())

	k, _, ok := sl.Floor(25)
	require.True(t, ok)
	require.Equal(t, 20, k)

	k, _, ok = sl.Ceiling(25)
	require.True(t, ok)
	require.Equal(t, 30, k)

	k, _, ok = sl.Lower(10)
	require.False(t, ok)

	k, _, ok = sl.Higher(20)
	require.True(t, ok)
	require.Equal(t, 30, k)

	_, _, ok = sl.First()
	require.True(t, ok)
	_, _, ok = sl.Last()
	require.True(t, ok)

	require.Equal(t, []int{10, 20, 30}, sl.Keys())

	var keys []int
	for k := range sl.All() {
		keys = append(keys, k)
	}
	require.Equal(t, []int{10, 20, 30}, keys)

	keys = nil
	for k := range sl.Range(10, 30, false, true) {
		keys = append(keys, k)
	}
	require.Equal(t, []int{20, 30}, keys)

	old, removed := sl.Remove(20)
	require.True(t, removed)
	require.Equal(t, "b", old)

	sl.Clear()
	require.Zero(t, sl.Len())
}

func TestSkipListCustomComparatorConcurrent(t *testing.T) {
	sl := concurrent.NewSkipListMapFunc[string, int](func(a, b string) int { return -strings.Compare(a, b) })
	sl.Put("a", 1)
	sl.Put("c", 3)

	k, _, ok := sl.First()
	require.True(t, ok)
	require.Equal(t, "c", k, "reversed comparator must be honoured")
}

func TestLRU(t *testing.T) {
	c := concurrent.NewLRU[string, int](2)
	require.Equal(t, 2, c.Capacity())

	c.Put("a", 1)
	c.Put("b", 2)
	require.Equal(t, 2, c.Len())

	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, 1, v)

	ek, _, evicted := c.Put("c", 3)
	require.True(t, evicted)
	require.Equal(t, "b", ek, "Get must have promoted a")
	require.Equal(t, []string{"a", "c"}, c.Keys())

	_, ok = c.Peek("a")
	require.True(t, ok)
	require.True(t, c.Contains("c"))
	require.True(t, c.Remove("c"))
	require.False(t, c.Contains("c"))

	c.Clear()
	require.Zero(t, c.Len())
}

func TestLRUConcurrent(t *testing.T) {
	c := concurrent.NewLRU[int, int](64)

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
			}
		}()
	}
	wg.Wait()

	require.Equal(t, 64, c.Len(), "capacity must hold under concurrent writers")
	require.Len(t, c.Keys(), 64)
}
