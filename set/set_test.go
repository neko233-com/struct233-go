package set_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/set"
)

func TestHashSetBasics(t *testing.T) {
	s := set.NewHashSet(1, 2, 3)
	require.Equal(t, 3, s.Len())

	require.False(t, s.Add(3), "re-adding an existing element must report false")
	require.True(t, s.Add(4), "adding a new element must report true")
	require.True(t, s.ContainsAll(1, 2, 3, 4))
	require.False(t, s.Contains(5))

	require.True(t, s.Remove(4))
	require.False(t, s.Remove(4), "Remove must report true exactly once")

	items := s.ToSlice()
	slices.Sort(items)
	require.Equal(t, []int{1, 2, 3}, items)

	s.Clear()
	require.Zero(t, s.Len())
	require.False(t, s.Contains(1))
}

func TestHashSetWithCapacity(t *testing.T) {
	s := set.NewHashSetWithCapacity(64, 1, 2, 3)
	require.Equal(t, 3, s.Len())
	require.True(t, s.ContainsAll(1, 2, 3))

	for i := range 100 {
		s.Add(i)
	}
	require.Equal(t, 100, s.Len())

	empty := set.NewHashSetWithCapacity[int](8)
	require.Zero(t, empty.Len())
	require.False(t, empty.Contains(0))
}

func TestHashSetAlgebra(t *testing.T) {
	a := set.NewHashSet(1, 2, 3, 4)
	b := set.NewHashSet(3, 4, 5)

	require.True(t, a.Union(b).Equal(set.NewHashSet(1, 2, 3, 4, 5)))
	require.True(t, a.Intersection(b).Equal(set.NewHashSet(3, 4)))
	require.True(t, a.Difference(b).Equal(set.NewHashSet(1, 2)))
	require.True(t, a.SymmetricDifference(b).Equal(set.NewHashSet(1, 2, 5)))

	require.True(t, a.Intersection(b).IsSubsetOf(a))
	require.True(t, a.IsSupersetOf(a.Intersection(b)))
	require.False(t, a.IsSubsetOf(b))
	require.True(t, a.IsSubsetOf(a))

	require.Equal(t, 4, a.Len(), "algebra operations must not mutate the receiver")
	require.Equal(t, 3, b.Len(), "algebra operations must not mutate the argument")
}

func TestHashSetClone(t *testing.T) {
	a := set.NewHashSet("x", "y")
	c := a.Clone()
	c.Add("z")

	require.False(t, a.Contains("z"), "Clone must be independent of the original")
	require.True(t, c.Contains("z"))
	require.True(t, c.IsSupersetOf(a))
}

func TestHashSetAll(t *testing.T) {
	s := set.NewHashSet(1, 2, 3)
	var iterated []int
	for v := range s.All() {
		iterated = append(iterated, v)
	}
	slices.Sort(iterated)
	require.Equal(t, []int{1, 2, 3}, iterated)

	count := 0
	for range s.All() {
		count++
		break
	}
	require.Equal(t, 1, count, "break must stop the iteration")
}

func TestContainsDoesNotAllocate(t *testing.T) {
	s := set.NewHashSetWithCapacity[int](128)
	for i := range 64 {
		s.Add(i)
	}

	allocs := testing.AllocsPerRun(100, func() { s.Contains(32) })
	require.Zero(t, allocs, "HashSet.Contains must not allocate")
}

func TestTreeSetOrdered(t *testing.T) {
	ts := set.NewTreeSet(5, 1, 9, 3, 7)
	require.Equal(t, []int{1, 3, 5, 7, 9}, ts.ToSlice())

	var iterated []int
	for k := range ts.All() {
		iterated = append(iterated, k)
	}
	require.Equal(t, []int{1, 3, 5, 7, 9}, iterated)

	first, ok := ts.First()
	require.True(t, ok)
	require.Equal(t, 1, first)
	last, ok := ts.Last()
	require.True(t, ok)
	require.Equal(t, 9, last)

	require.True(t, ts.Add(0))
	require.False(t, ts.Add(0), "duplicate Add must report false")
	require.True(t, ts.Remove(0))
	require.False(t, ts.Remove(0))
	require.Equal(t, 5, ts.Len())
}

func TestTreeSetNavigable(t *testing.T) {
	ts := set.NewTreeSet(10, 20, 30, 40, 50)

	k, ok := ts.Ceiling(35)
	require.True(t, ok)
	require.Equal(t, 40, k)

	k, ok = ts.Floor(35)
	require.True(t, ok)
	require.Equal(t, 30, k)

	k, ok = ts.Lower(30)
	require.True(t, ok)
	require.Equal(t, 20, k)

	k, ok = ts.Higher(30)
	require.True(t, ok)
	require.Equal(t, 40, k)

	_, ok = ts.Lower(10)
	require.False(t, ok, "Lower(min) must miss")
	_, ok = ts.Higher(50)
	require.False(t, ok, "Higher(max) must miss")
	_, ok = ts.Ceiling(51)
	require.False(t, ok, "Ceiling(above max) must miss")

	empty := set.NewTreeSet[int]()
	_, ok = empty.First()
	require.False(t, ok, "First on empty set must miss")
	_, ok = empty.Last()
	require.False(t, ok, "Last on empty set must miss")
}

func TestTreeSetRange(t *testing.T) {
	ts := set.NewTreeSet(10, 20, 30, 40, 50)

	var exclusive []int
	for k := range ts.Range(20, 40, false, false) {
		exclusive = append(exclusive, k)
	}
	require.Equal(t, []int{30}, exclusive)

	var inclusive []int
	for k := range ts.Range(15, 45, true, true) {
		inclusive = append(inclusive, k)
	}
	require.Equal(t, []int{20, 30, 40}, inclusive)
}

func TestTreeSetCustomComparator(t *testing.T) {
	ts := set.NewTreeSetFunc(func(a, b int) int { return b - a }, 3, 1, 2)
	require.Equal(t, []int{3, 2, 1}, ts.ToSlice(), "descending comparator order")

	first, ok := ts.First()
	require.True(t, ok)
	require.Equal(t, 3, first, "reversed comparator must flip First")
}

func TestTreeSetAlgebraAndClone(t *testing.T) {
	a := set.NewTreeSet(1, 2, 3)
	b := set.NewTreeSet(2, 3, 4)

	require.True(t, a.Union(b).Equal(set.NewTreeSet(1, 2, 3, 4)))
	require.True(t, a.Intersection(b).Equal(set.NewTreeSet(2, 3)))
	require.True(t, a.Difference(b).Equal(set.NewTreeSet(1)))

	c := a.Clone()
	c.Add(9)
	require.False(t, a.Contains(9), "Clone must be independent")
	require.Equal(t, []int{1, 2, 3, 9}, c.ToSlice())
}

func TestTreeSetPanicsOnNilComparator(t *testing.T) {
	require.Panics(t, func() { set.NewTreeSetFunc[int](nil) })
}

func TestLinkedHashSetOrder(t *testing.T) {
	s := set.NewLinkedHashSet("a", "b", "c")
	require.Equal(t, []string{"a", "b", "c"}, s.ToSlice())

	// re-adding an existing element keeps its position (JDK semantics)
	require.False(t, s.Add("a"))
	require.Equal(t, []string{"a", "b", "c"}, s.ToSlice())

	// remove then re-add moves the element to the end
	require.True(t, s.Remove("b"))
	require.True(t, s.Add("b"))
	require.Equal(t, []string{"a", "c", "b"}, s.ToSlice())

	var iterated []string
	for k := range s.All() {
		iterated = append(iterated, k)
	}
	require.Equal(t, s.ToSlice(), iterated, "All must follow ToSlice order")

	s.Clear()
	require.Zero(t, s.Len())
	require.True(t, s.Equal(set.NewLinkedHashSet[string]()))
}

func TestLinkedHashSetClone(t *testing.T) {
	s := set.NewLinkedHashSet(1, 2, 3)
	c := s.Clone()
	require.True(t, c.Remove(1))

	require.True(t, s.Contains(1), "Clone must be independent")
	require.False(t, c.Equal(s), "Equal is order sensitive")
	require.True(t, c.Equal(set.NewLinkedHashSet(2, 3)))
}
