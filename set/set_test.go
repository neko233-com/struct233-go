package set_test

import (
	"fmt"
	"math/rand/v2"
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

func TestLinkedHashSetWithCapacity(t *testing.T) {
	for _, capacity := range []int{-1, 0, 2, 64} {
		t.Run(fmt.Sprintf("capacity=%d", capacity), func(t *testing.T) {
			s := set.NewLinkedHashSetWithCapacity(capacity, 3, 1, 3, 2)
			require.Equal(t, []int{3, 1, 2}, s.ToSlice())
			require.False(t, s.Add(1))
			require.True(t, s.Remove(1))
			require.True(t, s.Add(1))
			require.Equal(t, []int{3, 2, 1}, s.ToSlice())
			s.Clear()
			require.True(t, s.Add(9))
			require.Equal(t, []int{9}, s.ToSlice())
		})
	}
	includeNone := set.NewLinkedHashSetWithCapacity[int](64)
	require.Zero(t, includeNone.Len())
	require.True(t, includeNone.Add(1))
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

func TestSkipListSetOrdered(t *testing.T) {
	ts := set.NewSkipListSet(5, 1, 9, 3, 7)
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

func TestSkipListSetNavigable(t *testing.T) {
	ts := set.NewSkipListSet(10, 20, 30, 40, 50)

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

	empty := set.NewSkipListSet[int]()
	_, ok = empty.First()
	require.False(t, ok, "First on empty set must miss")
	_, ok = empty.Last()
	require.False(t, ok, "Last on empty set must miss")
}

func TestSkipListSetRange(t *testing.T) {
	ts := set.NewSkipListSet(10, 20, 30, 40, 50)

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

func TestSkipListSetCustomComparator(t *testing.T) {
	ts := set.NewSkipListSetFunc(func(a, b int) int { return b - a }, 3, 1, 2)
	require.Equal(t, []int{3, 2, 1}, ts.ToSlice(), "descending comparator order")

	first, ok := ts.First()
	require.True(t, ok)
	require.Equal(t, 3, first, "reversed comparator must flip First")
}

func TestSkipListSetAlgebraAndClone(t *testing.T) {
	a := set.NewSkipListSet(1, 2, 3)
	b := set.NewSkipListSet(2, 3, 4)

	require.True(t, a.Union(b).Equal(set.NewSkipListSet(1, 2, 3, 4)))
	require.True(t, a.Intersection(b).Equal(set.NewSkipListSet(2, 3)))
	require.True(t, a.Difference(b).Equal(set.NewSkipListSet(1)))

	c := a.Clone()
	c.Add(9)
	require.False(t, a.Contains(9), "Clone must be independent")
	require.Equal(t, []int{1, 2, 3, 9}, c.ToSlice())
}

func TestSkipListSetPanicsOnNilComparator(t *testing.T) {
	require.Panics(t, func() { set.NewSkipListSetFunc[int](nil) })
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

// TestHashSet_TableDriven covers the exported HashSet methods through normal,
// boundary and empty cases in one table.
func TestHashSet_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"new with no items is empty", func(t *testing.T) {
			s := set.NewHashSet[int]()
			require.Zero(t, s.Len())
			require.False(t, s.Contains(0))
		}},
		{"new with items pre-fills", func(t *testing.T) {
			s := set.NewHashSet(1, 2, 2, 3)
			require.Equal(t, 3, s.Len(), "duplicate items must collapse")
		}},
		{"new with zero value item", func(t *testing.T) {
			s := set.NewHashSet(0)
			require.True(t, s.Contains(0))
			require.Equal(t, 1, s.Len())
		}},
		{"add new returns true", func(t *testing.T) {
			s := set.NewHashSet[int]()
			require.True(t, s.Add(5))
		}},
		{"add duplicate returns false", func(t *testing.T) {
			s := set.NewHashSet(5)
			require.False(t, s.Add(5))
			require.Equal(t, 1, s.Len())
		}},
		{"add zero value", func(t *testing.T) {
			s := set.NewHashSet[int]()
			require.True(t, s.Add(0))
			require.False(t, s.Add(0))
		}},
		{"add string key", func(t *testing.T) {
			s := set.NewHashSet[string]()
			require.True(t, s.Add("a"))
			require.True(t, s.Contains("a"))
		}},
		{"add all with duplicates", func(t *testing.T) {
			s := set.NewHashSet[int]()
			s.AddAll(1, 1, 2, 3, 2)
			require.Equal(t, 3, s.Len())
		}},
		{"add all with no items", func(t *testing.T) {
			s := set.NewHashSet[int]()
			s.AddAll()
			require.Zero(t, s.Len())
		}},
		{"remove hit returns true", func(t *testing.T) {
			s := set.NewHashSet(1, 2)
			require.True(t, s.Remove(1))
			require.Equal(t, 1, s.Len())
		}},
		{"remove miss returns false", func(t *testing.T) {
			s := set.NewHashSet[int]()
			require.False(t, s.Remove(7))
		}},
		{"remove twice reports only once", func(t *testing.T) {
			s := set.NewHashSet(7)
			require.True(t, s.Remove(7))
			require.False(t, s.Remove(7))
		}},
		{"contains all with empty list is true", func(t *testing.T) {
			s := set.NewHashSet[int]()
			require.True(t, s.ContainsAll())
		}},
		{"contains all on empty set is false", func(t *testing.T) {
			s := set.NewHashSet[int]()
			require.False(t, s.ContainsAll(1))
		}},
		{"contains all with mixed hit and miss", func(t *testing.T) {
			s := set.NewHashSet(1, 2, 3)
			require.True(t, s.ContainsAll(1, 3))
			require.False(t, s.ContainsAll(1, 4))
		}},
		{"clear empties the set", func(t *testing.T) {
			s := set.NewHashSet(1, 2, 3)
			s.Clear()
			require.Zero(t, s.Len())
			require.False(t, s.Contains(1))
		}},
		{"clear then add works", func(t *testing.T) {
			s := set.NewHashSet(1)
			s.Clear()
			require.True(t, s.Add(2))
			require.Equal(t, 1, s.Len())
		}},
		{"clone of empty set", func(t *testing.T) {
			s := set.NewHashSet[int]()
			c := s.Clone()
			require.Zero(t, c.Len())
			require.True(t, s.Equal(c))
		}},
		{"to slice of empty set", func(t *testing.T) {
			require.Empty(t, set.NewHashSet[int]().ToSlice())
		}},
		{"to slice contains every element", func(t *testing.T) {
			s := set.NewHashSet(3, 1, 2)
			items := s.ToSlice()
			slices.Sort(items)
			require.Equal(t, []int{1, 2, 3}, items)
		}},
		{"equal of identical sets", func(t *testing.T) {
			a := set.NewHashSet(1, 2)
			b := set.NewHashSet(2, 1)
			require.True(t, a.Equal(b))
			require.True(t, b.Equal(a))
		}},
		{"equal with size mismatch", func(t *testing.T) {
			a := set.NewHashSet(1, 2)
			b := set.NewHashSet(1, 2, 3)
			require.False(t, a.Equal(b))
			require.False(t, b.Equal(a))
		}},
		{"equal with same size but different members", func(t *testing.T) {
			a := set.NewHashSet(1, 2)
			b := set.NewHashSet(1, 3)
			require.False(t, a.Equal(b))
		}},
		{"union with disjoint sets", func(t *testing.T) {
			a := set.NewHashSet(1)
			b := set.NewHashSet(2)
			u := a.Union(b)
			require.True(t, u.Equal(set.NewHashSet(1, 2)))
		}},
		{"union with empty set", func(t *testing.T) {
			a := set.NewHashSet(1)
			require.True(t, a.Union(set.NewHashSet[int]()).Equal(a))
		}},
		{"intersection with disjoint sets is empty", func(t *testing.T) {
			a := set.NewHashSet(1)
			b := set.NewHashSet(2)
			require.Zero(t, a.Intersection(b).Len())
		}},
		{"intersection with itself is itself", func(t *testing.T) {
			a := set.NewHashSet(1, 2, 3)
			require.True(t, a.Intersection(a).Equal(a))
		}},
		{"difference with itself is empty", func(t *testing.T) {
			a := set.NewHashSet(1, 2)
			require.Zero(t, a.Difference(a).Len())
		}},
		{"difference keeps the receiver unchanged", func(t *testing.T) {
			a := set.NewHashSet(1, 2)
			b := set.NewHashSet(2)
			a.Difference(b)
			require.Equal(t, 2, a.Len())
		}},
		{"symmetric difference of identical sets is empty", func(t *testing.T) {
			a := set.NewHashSet(1, 2)
			require.Zero(t, a.SymmetricDifference(a).Len())
		}},
		{"symmetric difference is commutative", func(t *testing.T) {
			a := set.NewHashSet(1, 2, 3)
			b := set.NewHashSet(3, 4)
			require.True(t, a.SymmetricDifference(b).Equal(b.SymmetricDifference(a)))
		}},
		{"subset of empty set", func(t *testing.T) {
			require.True(t, set.NewHashSet[int]().IsSubsetOf(set.NewHashSet(1)))
			require.False(t, set.NewHashSet(1).IsSubsetOf(set.NewHashSet[int]()))
		}},
		{"superset of empty set", func(t *testing.T) {
			require.True(t, set.NewHashSet(1).IsSupersetOf(set.NewHashSet[int]()))
			require.False(t, set.NewHashSet[int]().IsSupersetOf(set.NewHashSet(1)))
		}},
		{"subset with equal size but missing element", func(t *testing.T) {
			a := set.NewHashSet(1, 2)
			b := set.NewHashSet(1, 3)
			require.False(t, a.IsSubsetOf(b))
		}},
		{"capacity hint larger than items", func(t *testing.T) {
			s := set.NewHashSetWithCapacity(1024, 1, 2)
			require.Equal(t, 2, s.Len())
		}},
		{"capacity hint zero", func(t *testing.T) {
			s := set.NewHashSetWithCapacity(0, 1)
			require.Equal(t, 1, s.Len())
		}},
		{"all on empty set yields nothing", func(t *testing.T) {
			count := 0
			for range set.NewHashSet[int]().All() {
				count++
			}
			require.Zero(t, count)
		}},
		{"all supports early break", func(t *testing.T) {
			s := set.NewHashSet(1, 2, 3, 4)
			count := 0
			for range s.All() {
				count++
				if count == 2 {
					break
				}
			}
			require.Equal(t, 2, count)
		}},
		{"union of two empty sets", func(t *testing.T) {
			a := set.NewHashSet[int]()
			b := set.NewHashSet[int]()
			require.Zero(t, a.Union(b).Len())
		}},
		{"intersection of two empty sets", func(t *testing.T) {
			a := set.NewHashSet[int]()
			b := set.NewHashSet[int]()
			require.Zero(t, a.Intersection(b).Len())
		}},
		{"algebra never mutates operands", func(t *testing.T) {
			a := set.NewHashSet(1, 2, 3)
			b := set.NewHashSet(3, 4)
			a.Union(b)
			a.Intersection(b)
			a.Difference(b)
			a.SymmetricDifference(b)
			require.Equal(t, 3, a.Len())
			require.Equal(t, 2, b.Len())
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// TestLinkedHashSet_TableDriven covers the exported LinkedHashSet methods.
func TestLinkedHashSet_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"new with no items is empty", func(t *testing.T) {
			s := set.NewLinkedHashSet[int]()
			require.Zero(t, s.Len())
			require.Empty(t, s.ToSlice())
		}},
		{"new with items keeps first-insertion order", func(t *testing.T) {
			s := set.NewLinkedHashSet(3, 1, 2, 1)
			require.Equal(t, []int{3, 1, 2}, s.ToSlice())
		}},
		{"new with zero value", func(t *testing.T) {
			s := set.NewLinkedHashSet(0, 1)
			require.Equal(t, []int{0, 1}, s.ToSlice())
		}},
		{"add new appends at the end", func(t *testing.T) {
			s := set.NewLinkedHashSet(1)
			require.True(t, s.Add(2))
			require.Equal(t, []int{1, 2}, s.ToSlice())
		}},
		{"add duplicate keeps position", func(t *testing.T) {
			s := set.NewLinkedHashSet(1, 2)
			require.False(t, s.Add(1))
			require.Equal(t, []int{1, 2}, s.ToSlice())
		}},
		{"add all keeps order", func(t *testing.T) {
			s := set.NewLinkedHashSet[int]()
			s.AddAll(2, 1, 3, 1)
			require.Equal(t, []int{2, 1, 3}, s.ToSlice())
		}},
		{"add all with no items", func(t *testing.T) {
			s := set.NewLinkedHashSet[int]()
			s.AddAll()
			require.Zero(t, s.Len())
		}},
		{"remove middle keeps relative order", func(t *testing.T) {
			s := set.NewLinkedHashSet(1, 2, 3)
			require.True(t, s.Remove(2))
			require.Equal(t, []int{1, 3}, s.ToSlice())
		}},
		{"remove miss returns false", func(t *testing.T) {
			s := set.NewLinkedHashSet(1)
			require.False(t, s.Remove(9))
		}},
		{"remove then re-add moves to the end", func(t *testing.T) {
			s := set.NewLinkedHashSet(1, 2, 3)
			s.Remove(1)
			s.Add(1)
			require.Equal(t, []int{2, 3, 1}, s.ToSlice())
		}},
		{"contains hit and miss", func(t *testing.T) {
			s := set.NewLinkedHashSet(1)
			require.True(t, s.Contains(1))
			require.False(t, s.Contains(2))
		}},
		{"len after add and remove", func(t *testing.T) {
			s := set.NewLinkedHashSet(1, 2, 3)
			s.Remove(2)
			require.Equal(t, 2, s.Len())
		}},
		{"clear resets order", func(t *testing.T) {
			s := set.NewLinkedHashSet(1, 2)
			s.Clear()
			require.Zero(t, s.Len())
			s.Add(3)
			require.Equal(t, []int{3}, s.ToSlice())
		}},
		{"clone preserves order", func(t *testing.T) {
			s := set.NewLinkedHashSet(3, 1, 2)
			require.Equal(t, []int{3, 1, 2}, s.Clone().ToSlice())
		}},
		{"clone is independent", func(t *testing.T) {
			s := set.NewLinkedHashSet(1, 2)
			c := s.Clone()
			c.Remove(1)
			require.Equal(t, []int{1, 2}, s.ToSlice(), "clone mutations must not leak")
		}},
		{"all walks insertion order", func(t *testing.T) {
			s := set.NewLinkedHashSet(2, 1, 3)
			var got []int
			for v := range s.All() {
				got = append(got, v)
			}
			require.Equal(t, []int{2, 1, 3}, got)
		}},
		{"all supports early break", func(t *testing.T) {
			s := set.NewLinkedHashSet(1, 2, 3)
			count := 0
			for range s.All() {
				count++
				break
			}
			require.Equal(t, 1, count)
		}},
		{"equal ignores insertion order", func(t *testing.T) {
			a := set.NewLinkedHashSet(1, 2)
			b := set.NewLinkedHashSet(2, 1)
			require.True(t, a.Equal(b))
			require.True(t, b.Equal(a))
		}},
		{"equal of identical order", func(t *testing.T) {
			a := set.NewLinkedHashSet(1, 2)
			b := set.NewLinkedHashSet(1, 2)
			require.True(t, a.Equal(b))
		}},
		{"equal with empty set", func(t *testing.T) {
			require.True(t, set.NewLinkedHashSet[int]().Equal(set.NewLinkedHashSet[int]()))
			require.False(t, set.NewLinkedHashSet(1).Equal(set.NewLinkedHashSet[int]()))
		}},
		{"to slice hands out a copy", func(t *testing.T) {
			s := set.NewLinkedHashSet(1)
			items := s.ToSlice()
			items[0] = 99
			require.Equal(t, []int{1}, s.ToSlice())
		}},
		{"string elements order", func(t *testing.T) {
			s := set.NewLinkedHashSet("b", "a", "c")
			require.Equal(t, []string{"b", "a", "c"}, s.ToSlice())
		}},
		{"re-add after clear keeps fresh order", func(t *testing.T) {
			s := set.NewLinkedHashSet(1, 2)
			s.Clear()
			s.AddAll(2, 1)
			require.Equal(t, []int{2, 1}, s.ToSlice())
		}},
		{"remove head and tail", func(t *testing.T) {
			s := set.NewLinkedHashSet(1, 2, 3)
			s.Remove(1)
			s.Remove(3)
			require.Equal(t, []int{2}, s.ToSlice())
		}},
		{"duplicate-heavy construction", func(t *testing.T) {
			s := set.NewLinkedHashSet(1, 1, 1, 1)
			require.Equal(t, []int{1}, s.ToSlice())
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// TestSkipListSet_TableDriven covers the exported SkipListSet methods.
func TestSkipListSet_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"new with no items is empty", func(t *testing.T) {
			ts := set.NewSkipListSet[int]()
			require.Zero(t, ts.Len())
			require.Empty(t, ts.ToSlice())
		}},
		{"new sorts the items", func(t *testing.T) {
			ts := set.NewSkipListSet(3, 1, 2)
			require.Equal(t, []int{1, 2, 3}, ts.ToSlice())
		}},
		{"new collapses duplicates", func(t *testing.T) {
			ts := set.NewSkipListSet(1, 1, 1)
			require.Equal(t, 1, ts.Len())
		}},
		{"add new returns true", func(t *testing.T) {
			ts := set.NewSkipListSet[int]()
			require.True(t, ts.Add(5))
		}},
		{"add duplicate returns false", func(t *testing.T) {
			ts := set.NewSkipListSet(5)
			require.False(t, ts.Add(5))
		}},
		{"add keeps sorted order", func(t *testing.T) {
			ts := set.NewSkipListSet(2)
			ts.Add(3)
			ts.Add(1)
			require.Equal(t, []int{1, 2, 3}, ts.ToSlice())
		}},
		{"add all with duplicates", func(t *testing.T) {
			ts := set.NewSkipListSet[int]()
			ts.AddAll(2, 1, 2, 3)
			require.Equal(t, []int{1, 2, 3}, ts.ToSlice())
		}},
		{"add all with no items", func(t *testing.T) {
			ts := set.NewSkipListSet[int]()
			ts.AddAll()
			require.Zero(t, ts.Len())
		}},
		{"remove hit returns true", func(t *testing.T) {
			ts := set.NewSkipListSet(1, 2, 3)
			require.True(t, ts.Remove(2))
			require.Equal(t, []int{1, 3}, ts.ToSlice())
		}},
		{"remove miss returns false", func(t *testing.T) {
			ts := set.NewSkipListSet(1)
			require.False(t, ts.Remove(9))
		}},
		{"remove twice reports only once", func(t *testing.T) {
			ts := set.NewSkipListSet(1)
			require.True(t, ts.Remove(1))
			require.False(t, ts.Remove(1))
		}},
		{"contains hit and miss", func(t *testing.T) {
			ts := set.NewSkipListSet(1, 2)
			require.True(t, ts.Contains(2))
			require.False(t, ts.Contains(3))
		}},
		{"len tracks add and remove", func(t *testing.T) {
			ts := set.NewSkipListSet(1, 2, 3)
			ts.Remove(3)
			require.Equal(t, 2, ts.Len())
		}},
		{"clear empties the set", func(t *testing.T) {
			ts := set.NewSkipListSet(1, 2)
			ts.Clear()
			require.Zero(t, ts.Len())
			require.False(t, ts.Contains(1))
		}},
		{"first and last on empty set miss", func(t *testing.T) {
			ts := set.NewSkipListSet[int]()
			_, ok := ts.First()
			require.False(t, ok)
			_, ok = ts.Last()
			require.False(t, ok)
		}},
		{"first and last on single element", func(t *testing.T) {
			ts := set.NewSkipListSet(7)
			k, ok := ts.First()
			require.True(t, ok)
			require.Equal(t, 7, k)
			k, ok = ts.Last()
			require.True(t, ok)
			require.Equal(t, 7, k)
		}},
		{"floor hit exact", func(t *testing.T) {
			ts := set.NewSkipListSet(10, 20, 30)
			k, ok := ts.Floor(20)
			require.True(t, ok)
			require.Equal(t, 20, k)
		}},
		{"floor hit below an element", func(t *testing.T) {
			ts := set.NewSkipListSet(10, 20, 30)
			k, ok := ts.Floor(25)
			require.True(t, ok)
			require.Equal(t, 20, k)
		}},
		{"floor below the minimum misses", func(t *testing.T) {
			ts := set.NewSkipListSet(10, 20)
			_, ok := ts.Floor(5)
			require.False(t, ok)
		}},
		{"ceiling hit exact", func(t *testing.T) {
			ts := set.NewSkipListSet(10, 20, 30)
			k, ok := ts.Ceiling(20)
			require.True(t, ok)
			require.Equal(t, 20, k)
		}},
		{"ceiling hit above an element", func(t *testing.T) {
			ts := set.NewSkipListSet(10, 20, 30)
			k, ok := ts.Ceiling(15)
			require.True(t, ok)
			require.Equal(t, 20, k)
		}},
		{"ceiling above the maximum misses", func(t *testing.T) {
			ts := set.NewSkipListSet(10, 20)
			_, ok := ts.Ceiling(25)
			require.False(t, ok)
		}},
		{"lower hit", func(t *testing.T) {
			ts := set.NewSkipListSet(10, 20, 30)
			k, ok := ts.Lower(20)
			require.True(t, ok)
			require.Equal(t, 10, k)
		}},
		{"lower at the minimum misses", func(t *testing.T) {
			ts := set.NewSkipListSet(10, 20)
			_, ok := ts.Lower(10)
			require.False(t, ok)
		}},
		{"higher hit", func(t *testing.T) {
			ts := set.NewSkipListSet(10, 20, 30)
			k, ok := ts.Higher(20)
			require.True(t, ok)
			require.Equal(t, 30, k)
		}},
		{"higher at the maximum misses", func(t *testing.T) {
			ts := set.NewSkipListSet(10, 20)
			_, ok := ts.Higher(20)
			require.False(t, ok)
		}},
		{"floor ceiling lower higher on empty set miss", func(t *testing.T) {
			ts := set.NewSkipListSet[int]()
			_, ok := ts.Floor(1)
			require.False(t, ok)
			_, ok = ts.Ceiling(1)
			require.False(t, ok)
			_, ok = ts.Lower(1)
			require.False(t, ok)
			_, ok = ts.Higher(1)
			require.False(t, ok)
		}},
		{"range inclusive both ends", func(t *testing.T) {
			ts := set.NewSkipListSet(1, 2, 3, 4, 5)
			var got []int
			for k := range ts.Range(2, 4, true, true) {
				got = append(got, k)
			}
			require.Equal(t, []int{2, 3, 4}, got)
		}},
		{"range exclusive both ends", func(t *testing.T) {
			ts := set.NewSkipListSet(1, 2, 3, 4, 5)
			var got []int
			for k := range ts.Range(2, 4, false, false) {
				got = append(got, k)
			}
			require.Equal(t, []int{3}, got)
		}},
		{"range with no interior points", func(t *testing.T) {
			ts := set.NewSkipListSet(1, 2, 3)
			var got []int
			for k := range ts.Range(1, 3, false, false) {
				got = append(got, k)
			}
			require.Equal(t, []int{2}, got)
		}},
		{"range over empty set yields nothing", func(t *testing.T) {
			ts := set.NewSkipListSet[int]()
			count := 0
			for range ts.Range(0, 10, true, true) {
				count++
			}
			require.Zero(t, count)
		}},
		{"to slice is ascending", func(t *testing.T) {
			ts := set.NewSkipListSet(5, 1, 3, 2, 4)
			require.Equal(t, []int{1, 2, 3, 4, 5}, ts.ToSlice())
		}},
		{"clone preserves order and is independent", func(t *testing.T) {
			ts := set.NewSkipListSet(3, 1, 2)
			c := ts.Clone()
			c.Add(0)
			require.Equal(t, []int{1, 2, 3}, ts.ToSlice())
			require.Equal(t, []int{0, 1, 2, 3}, c.ToSlice())
		}},
		{"union intersection difference", func(t *testing.T) {
			a := set.NewSkipListSet(1, 2, 3)
			b := set.NewSkipListSet(2, 3, 4)
			require.Equal(t, []int{1, 2, 3, 4}, a.Union(b).ToSlice())
			require.Equal(t, []int{2, 3}, a.Intersection(b).ToSlice())
			require.Equal(t, []int{1}, a.Difference(b).ToSlice())
		}},
		{"algebra with empty sets", func(t *testing.T) {
			a := set.NewSkipListSet(1, 2)
			e := set.NewSkipListSet[int]()
			require.Equal(t, []int{1, 2}, a.Union(e).ToSlice())
			require.Zero(t, a.Intersection(e).Len())
			require.Equal(t, []int{1, 2}, a.Difference(e).ToSlice())
		}},
		{"equal compares contents", func(t *testing.T) {
			a := set.NewSkipListSet(1, 2)
			b := set.NewSkipListSet(2, 1)
			require.True(t, a.Equal(b))
			require.True(t, b.Equal(a))
			require.False(t, a.Equal(set.NewSkipListSet(1)))
		}},
		{"equal of empty sets", func(t *testing.T) {
			require.True(t, set.NewSkipListSet[int]().Equal(set.NewSkipListSet[int]()))
		}},
		{"nil comparator panics with prefix", func(t *testing.T) {
			require.PanicsWithValue(t, "set: nil comparator", func() {
				set.NewSkipListSetFunc[int](nil)
			})
		}},
		{"custom comparator with items", func(t *testing.T) {
			ts := set.NewSkipListSetFunc(func(a, b string) int {
				if a < b {
					return -1
				}
				if a > b {
					return 1
				}
				return 0
			}, "b", "a")
			require.Equal(t, []string{"a", "b"}, ts.ToSlice())
		}},
		{"algebra never mutates operands", func(t *testing.T) {
			a := set.NewSkipListSet(1, 2, 3)
			b := set.NewSkipListSet(3, 4)
			a.Union(b)
			a.Intersection(b)
			a.Difference(b)
			require.Equal(t, []int{1, 2, 3}, a.ToSlice())
			require.Equal(t, []int{3, 4}, b.ToSlice())
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// setModel is the reference model for differential testing: a plain
// map[T]struct{} with the LinkedHashSet insertion order alongside it.
type setModel struct {
	m     map[int]struct{}
	order []int
}

func newSetModel() *setModel {
	return &setModel{m: map[int]struct{}{}}
}

func (mdl *setModel) add(v int) bool {
	if _, ok := mdl.m[v]; ok {
		return false
	}
	mdl.m[v] = struct{}{}
	mdl.order = append(mdl.order, v)
	return true
}

func (mdl *setModel) remove(v int) bool {
	if _, ok := mdl.m[v]; !ok {
		return false
	}
	delete(mdl.m, v)
	for i, x := range mdl.order {
		if x == v {
			mdl.order = append(mdl.order[:i], mdl.order[i+1:]...)
			break
		}
	}
	return true
}

// keys returns the members sorted ascending, for order-agnostic comparisons.
func (mdl *setModel) keys() []int {
	out := make([]int, 0, len(mdl.m))
	for v := range mdl.m {
		out = append(out, v)
	}
	slices.Sort(out)
	return out
}

// TestSetAgainstModel is a differential test: random Add/Remove traffic
// replayed against the map model for all three set types, with the set algebra
// cross-checked against modelled union/intersection/difference/symmetric
// difference after every step.
func TestSetAgainstModel(t *testing.T) {
	for _, seed := range [][2]uint64{{2026, 1012}, {7, 11}, {99, 4242}} {
		t.Run(fmt.Sprintf("seed-%d-%d", seed[0], seed[1]), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(seed[0], seed[1]))
			hs := set.NewHashSet[int]()
			lhs := set.NewLinkedHashSet[int]()
			ts := set.NewSkipListSet[int]()
			mdl := newSetModel()
			for step := range 200 {
				v := rng.IntN(20)
				if rng.IntN(3) > 0 {
					want := mdl.add(v)
					require.Equal(t, want, hs.Add(v), "step %d HashSet.Add(%d)", step, v)
					require.Equal(t, want, lhs.Add(v), "step %d LinkedHashSet.Add(%d)", step, v)
					require.Equal(t, want, ts.Add(v), "step %d SkipListSet.Add(%d)", step, v)
				} else {
					want := mdl.remove(v)
					require.Equal(t, want, hs.Remove(v), "step %d HashSet.Remove(%d)", step, v)
					require.Equal(t, want, lhs.Remove(v), "step %d LinkedHashSet.Remove(%d)", step, v)
					require.Equal(t, want, ts.Remove(v), "step %d SkipListSet.Remove(%d)", step, v)
				}

				want := mdl.keys()
				got := hs.ToSlice()
				slices.Sort(got)
				require.Equal(t, want, got, "step %d HashSet contents", step)
				require.True(t, slices.Equal(mdl.order, lhs.ToSlice()), "step %d LinkedHashSet order: %v vs %v", step, lhs.ToSlice(), mdl.order)
				require.Equal(t, want, ts.ToSlice(), "step %d SkipListSet contents", step)
				require.Equal(t, len(mdl.m), hs.Len(), "step %d HashSet.Len", step)
				require.Equal(t, len(mdl.m), lhs.Len(), "step %d LinkedHashSet.Len", step)
				require.Equal(t, len(mdl.m), ts.Len(), "step %d SkipListSet.Len", step)

				// membership agrees everywhere
				for i := range 20 {
					_, want := mdl.m[i]
					require.Equal(t, want, hs.Contains(i), "step %d HashSet.Contains(%d)", step, i)
					require.Equal(t, want, lhs.Contains(i), "step %d LinkedHashSet.Contains(%d)", step, i)
					require.Equal(t, want, ts.Contains(i), "step %d SkipListSet.Contains(%d)", step, i)
				}
			}
		})
	}
}

// TestSetAlgebraAgainstModel checks HashSet union/intersection/difference/
// symmetric difference against plain map[T]struct{} set algebra.
func TestSetAlgebraAgainstModel(t *testing.T) {
	for _, seed := range [][2]uint64{{2026, 1012}, {7, 11}, {99, 4242}} {
		t.Run(fmt.Sprintf("seed-%d-%d", seed[0], seed[1]), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(seed[0], seed[1]))
			for round := range 50 {
				a := set.NewHashSet[int]()
				b := set.NewHashSet[int]()
				ma, mb := map[int]struct{}{}, map[int]struct{}{}
				for range 8 {
					v := rng.IntN(16)
					a.Add(v)
					ma[v] = struct{}{}
					v = rng.IntN(16)
					b.Add(v)
					mb[v] = struct{}{}
				}
				check := func(name string, got *set.HashSet[int], want map[int]struct{}) {
					t.Helper()
					gotKeys := got.ToSlice()
					slices.Sort(gotKeys)
					wantKeys := make([]int, 0, len(want))
					for v := range want {
						wantKeys = append(wantKeys, v)
					}
					slices.Sort(wantKeys)
					require.Equal(t, wantKeys, gotKeys, "round %d %s", round, name)
				}
				union := map[int]struct{}{}
				for v := range ma {
					union[v] = struct{}{}
				}
				for v := range mb {
					union[v] = struct{}{}
				}
				inter := map[int]struct{}{}
				for v := range ma {
					if _, ok := mb[v]; ok {
						inter[v] = struct{}{}
					}
				}
				diff := map[int]struct{}{}
				for v := range ma {
					if _, ok := mb[v]; !ok {
						diff[v] = struct{}{}
					}
				}
				sym := map[int]struct{}{}
				for v := range ma {
					if _, ok := mb[v]; !ok {
						sym[v] = struct{}{}
					}
				}
				for v := range mb {
					if _, ok := ma[v]; !ok {
						sym[v] = struct{}{}
					}
				}
				check("union", a.Union(b), union)
				check("intersection", a.Intersection(b), inter)
				check("difference", a.Difference(b), diff)
				check("symmetric difference", a.SymmetricDifference(b), sym)
				// subset/superset agree with the model
				aSubsetB, bSubsetA := true, true
				for v := range ma {
					if _, ok := mb[v]; !ok {
						aSubsetB = false
					}
				}
				for v := range mb {
					if _, ok := ma[v]; !ok {
						bSubsetA = false
					}
				}
				require.Equal(t, aSubsetB, a.IsSubsetOf(b), "round %d IsSubsetOf", round)
				require.Equal(t, bSubsetA, a.IsSupersetOf(b), "round %d IsSupersetOf", round)
				require.Equal(t, len(ma) == len(mb) && aSubsetB, a.Equal(b), "round %d Equal", round)
			}
		})
	}
}

// TestSkipListSetAgainstSortedSliceModel checks SkipListSet ordering, navigation and
// Range against a sorted-deduplicated slice model.
func TestSkipListSetAgainstSortedSliceModel(t *testing.T) {
	for _, seed := range [][2]uint64{{2026, 1012}, {7, 11}, {99, 4242}} {
		t.Run(fmt.Sprintf("seed-%d-%d", seed[0], seed[1]), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(seed[0], seed[1]))
			ts := set.NewSkipListSet[int]()
			model := make([]int, 0, 32) // sorted, no duplicates
			for step := range 200 {
				v := rng.IntN(30)
				if rng.IntN(3) > 0 {
					ts.Add(v)
					i, found := slices.BinarySearch(model, v)
					if !found {
						model = slices.Insert(model, i, v)
					}
				} else {
					ts.Remove(v)
					if i, found := slices.BinarySearch(model, v); found {
						model = slices.Delete(model, i, i+1)
					}
				}
				require.Equal(t, model, ts.ToSlice(), "step %d ToSlice", step)
				require.Equal(t, len(model), ts.Len(), "step %d Len", step)

				// First / Last
				if len(model) == 0 {
					_, ok := ts.First()
					require.False(t, ok, "step %d First on empty", step)
					_, ok = ts.Last()
					require.False(t, ok, "step %d Last on empty", step)
				} else {
					k, ok := ts.First()
					require.True(t, ok, "step %d First", step)
					require.Equal(t, model[0], k, "step %d First", step)
					k, ok = ts.Last()
					require.True(t, ok, "step %d Last", step)
					require.Equal(t, model[len(model)-1], k, "step %d Last", step)
				}

				// Floor / Ceiling / Lower / Higher against the model
				q := rng.IntN(30)
				wantFloor, wantFloorOK := modelProbe(model, q, true, false, true)
				gotFloor, gotOK := ts.Floor(q)
				require.Equal(t, wantFloorOK, gotOK, "step %d Floor(%d) ok", step, q)
				require.Equal(t, wantFloor, gotFloor, "step %d Floor(%d)", step, q)
				wantCeil, wantCeilOK := modelProbe(model, q, true, true, true)
				gotCeil, gotOK := ts.Ceiling(q)
				require.Equal(t, wantCeilOK, gotOK, "step %d Ceiling(%d) ok", step, q)
				require.Equal(t, wantCeil, gotCeil, "step %d Ceiling(%d)", step, q)
				wantLower, wantLowerOK := modelProbe(model, q, false, false, true)
				gotLower, gotOK := ts.Lower(q)
				require.Equal(t, wantLowerOK, gotOK, "step %d Lower(%d) ok", step, q)
				require.Equal(t, wantLower, gotLower, "step %d Lower(%d)", step, q)
				wantHigher, wantHigherOK := modelProbe(model, q, false, true, true)
				gotHigher, gotOK := ts.Higher(q)
				require.Equal(t, wantHigherOK, gotOK, "step %d Higher(%d) ok", step, q)
				require.Equal(t, wantHigher, gotHigher, "step %d Higher(%d)", step, q)
			}
		})
	}
}

// modelProbe finds the nearest model element to q: includeEqual decides
// whether q itself is a valid answer, up selects the direction, and the bool
// reports whether any element qualifies.
func modelProbe(model []int, q int, includeEqual, up, _ bool) (int, bool) {
	if up {
		for _, v := range model {
			if v > q || (v == q && includeEqual) {
				return v, true
			}
		}
		return 0, false
	}
	for i := len(model) - 1; i >= 0; i-- {
		if model[i] < q || (model[i] == q && includeEqual) {
			return model[i], true
		}
	}
	return 0, false
}
