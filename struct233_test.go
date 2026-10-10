package struct233_test

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	struct233 "github.com/neko233-com/struct233-go"
)

func TestNewSet(t *testing.T) {
	s := struct233.NewSet(1, 2, 3)
	require.Equal(t, 3, s.Len())
	require.True(t, s.Contains(2))
	require.False(t, s.Add(3))
	require.True(t, s.Union(struct233.NewSet(4)).Equal(struct233.NewSet(1, 2, 3, 4)))

	capped := struct233.NewSetWithCapacity[string](32, "a", "b")
	require.Equal(t, 2, capped.Len())
	require.True(t, capped.ContainsAll("a", "b"))
}

func TestNewSortedSet(t *testing.T) {
	ts := struct233.NewSortedSet(3, 1, 2)
	require.Equal(t, []int{1, 2, 3}, ts.ToSlice())

	k, ok := ts.Ceiling(2)
	require.True(t, ok)
	require.Equal(t, 2, k)

	desc := struct233.NewSortedSetFunc(func(a, b int) int { return b - a }, 1, 3, 2)
	require.Equal(t, []int{3, 2, 1}, desc.ToSlice())
}

func TestNewLinkedSet(t *testing.T) {
	s := struct233.NewLinkedSet("c", "a", "b")
	require.Equal(t, []string{"c", "a", "b"}, s.ToSlice(), "insertion order must be preserved")
}

func TestNewSkipList(t *testing.T) {
	sl := struct233.NewSkipList[int, string]()
	sl.Put(10, "a")
	sl.Put(20, "b")

	v, ok := sl.Get(20)
	require.True(t, ok)
	require.Equal(t, "b", v)

	k, _, ok := sl.Floor(15)
	require.True(t, ok)
	require.Equal(t, 10, k)

	rev := struct233.NewSkipListFunc[string, int](func(a, b string) int { return strings.Compare(b, a) })
	rev.Put("a", 1)
	rev.Put("c", 3)
	revKey, _, revOK := rev.First()
	require.True(t, revOK)
	require.Equal(t, "c", revKey, "custom comparator must be honoured")
}

func TestNewTreeMap(t *testing.T) {
	tm := struct233.NewTreeMap[int, string]()
	tm.Put(5, "e")
	tm.Put(1, "a")

	require.Equal(t, []int{1, 5}, tm.Keys())

	tmFunc := struct233.NewTreeMapFunc[int, string](func(a, b int) int { return b - a })
	tmFunc.Put(1, "a")
	tmFunc.Put(5, "e")
	require.Equal(t, []int{5, 1}, tmFunc.Keys(), "reversed comparator must be honoured")
}

func TestNewLinkedMap(t *testing.T) {
	m := struct233.NewLinkedMap[string, int]()
	m.Put("b", 1)
	m.Put("a", 2)
	m.Put("b", 3) // update must not reorder

	require.Equal(t, []string{"b", "a"}, m.Keys())
	require.Equal(t, []int{3, 2}, m.Values())
}

func TestNewPriorityQueue(t *testing.T) {
	q := struct233.NewPriorityQueue[int]()
	q.Push(3)
	q.Push(1)
	q.Push(2)

	v, ok := q.Pop()
	require.True(t, ok)
	require.Equal(t, 1, v)

	maxQ := struct233.NewPriorityQueueFunc(func(a, b int) int { return b - a })
	maxQ.Push(1)
	maxQ.Push(9)
	v, ok = maxQ.Pop()
	require.True(t, ok)
	require.Equal(t, 9, v, "custom comparator must be honoured")

	fromSlice := struct233.NewPriorityQueueFromSlice([]int{5, 1, 4}, func(a, b int) int { return a - b })
	v, ok = fromSlice.Pop()
	require.True(t, ok)
	require.Equal(t, 1, v)
}

func TestNewDeque(t *testing.T) {
	d := struct233.NewDeque[int](16)
	d.PushBack(1)
	d.PushFront(0)
	d.PushBack(2)

	require.Equal(t, []int{0, 1, 2}, d.ToSlice())

	back, ok := d.PopBack()
	require.True(t, ok)
	require.Equal(t, 2, back)

	require.Equal(t, []int{0, 1}, d.ToSlice())

	noCap := struct233.NewDeque[string]()
	require.GreaterOrEqual(t, noCap.Cap(), 8)
}

func TestNewRingBuffer(t *testing.T) {
	r := struct233.NewRingBuffer[int](3)
	require.Equal(t, 3, r.Cap())

	r.Push(1)
	r.Push(2)
	r.Push(3)

	evicted, ok := r.Push(4)
	require.True(t, ok)
	require.Equal(t, 1, evicted, "the oldest element must be overwritten")
	require.Equal(t, []int{2, 3, 4}, r.ToSlice())

	require.Panics(t, func() { struct233.NewRingBuffer[int](0) })
}

func TestNewBitSet(t *testing.T) {
	b := struct233.NewBitSet()
	b.Set(7)
	b.Set(70)

	require.True(t, b.Get(70))
	require.Equal(t, 2, b.Cardinality())
	require.Equal(t, 71, b.Len())
	require.Equal(t, 7, b.NextSetBit(0))
	require.Equal(t, 70, b.NextSetBit(8))
}

func TestNewLRU(t *testing.T) {
	c := struct233.NewLRU[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)

	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, 1, v)

	ek, _, ev := c.Put("c", 3)
	require.True(t, ev)
	require.Equal(t, "b", ek, "Get must have promoted a")
	require.Equal(t, 2, c.Len())
}

func TestNewConcurrentContainers(t *testing.T) {
	cs := struct233.NewConcurrentSetWithCapacity[int](8)
	require.True(t, cs.Add(1))
	require.True(t, cs.Contains(1))

	cm := struct233.NewConcurrentMapWithCapacity[string, int](8)
	cm.Put("a", 1)
	v, loaded := cm.GetOrPut("a", func() int { return 99 })
	require.True(t, loaded)
	require.Equal(t, 1, v, "GetOrPut must keep the existing value")

	// the no-capacity constructors must work too
	require.True(t, struct233.NewConcurrentSet[int]().Add(1))
	require.Zero(t, struct233.NewConcurrentMap[int, int]().Len())

	csl := struct233.NewConcurrentSkipList[int, string]()
	csl.Put(2, "b")
	csl.Put(1, "a")
	require.Equal(t, []int{1, 2}, csl.Keys())

	cslFunc := struct233.NewConcurrentSkipListFunc[string, int](func(a, b string) int { return strings.Compare(b, a) })
	cslFunc.Put("a", 1)
	cslFunc.Put("c", 3)
	k, _, ok := cslFunc.First()
	require.True(t, ok)
	require.Equal(t, "c", k)

	cl := struct233.NewConcurrentLRU[int, int](2)
	cl.Put(1, 1)
	cl.Put(2, 2)
	cl.Put(3, 3)
	require.Equal(t, 2, cl.Len())
	require.False(t, cl.Contains(1), "the least recently used entry must be evicted")
}

// TestAliasesAreTheImplementationTypes proves the facade returns the concrete
// container types, so no wrapper indirection is introduced.
func TestAliasesAreTheImplementationTypes(t *testing.T) {
	var s *struct233.Set[int] = struct233.NewSet[int]()
	s.Add(1)
	require.True(t, s.Contains(1))

	var d *struct233.Deque[int] = struct233.NewDeque[int]()
	d.PushBack(1)
	require.Equal(t, 1, d.Len())

	var tm *struct233.TreeMap[int, int] = struct233.NewTreeMap[int, int]()
	tm.Put(1, 1)
	require.Equal(t, 1, tm.Len())
}

func BenchmarkFacadeNewSetAndAdd(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		s := struct233.NewSetWithCapacity[int](4)
		s.Add(i & 3)
	}
}

func BenchmarkFacadeGetSkipList(b *testing.B) {
	sl := struct233.NewSkipList[int, int]()
	rng := rand.New(rand.NewPCG(1, 2))
	for range 1024 {
		k := rng.IntN(1024)
		sl.Put(k, k)
	}
	keys := make([]int, 1024)
	for i := range keys {
		keys[i] = rng.IntN(1024)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		sl.Get(keys[i&1023])
	}
}
