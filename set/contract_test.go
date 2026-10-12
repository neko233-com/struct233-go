package set_test

import (
	"slices"
	"testing"

	"github.com/neko233-com/struct233-go/set"
	"github.com/stretchr/testify/require"
)

type setFactory struct {
	name  string
	new   func(...int) set.Set[int]
	order []int // nil means unspecified order
}

func setFactories() []setFactory {
	return []setFactory{
		{"HashSet", func(items ...int) set.Set[int] { return set.NewHashSet(items...) }, nil},
		{"LinkedHashSet", func(items ...int) set.Set[int] { return set.NewLinkedHashSet(items...) }, []int{3, 2, 4, 1}},
		{"SkipListSet", func(items ...int) set.Set[int] { return set.NewSkipListSet(items...) }, []int{1, 2, 3, 4}},
	}
}

func TestSetInterfaceContract(t *testing.T) {
	for _, factory := range setFactories() {
		t.Run(factory.name, func(t *testing.T) {
			s := factory.new()
			require.True(t, s.IsEmpty())
			require.Zero(t, s.Len())
			require.True(t, s.ContainsAll())
			require.False(t, s.Contains(0))
			require.False(t, s.Remove(0))
			require.Empty(t, slices.Collect(s.All()))
			s.AddAll(3, 1, 2, 3)
			require.Equal(t, 3, s.Len())
			require.False(t, s.IsEmpty())
			require.False(t, s.Add(3))
			require.True(t, s.Add(4))
			require.True(t, s.Remove(1))
			require.False(t, s.Remove(1))
			require.True(t, s.Add(1))
			require.True(t, s.ContainsAll(1, 2, 3, 4, 1))
			require.False(t, s.ContainsAll(1, 9))
			snapshot := s.ToSlice()
			if factory.order == nil {
				require.ElementsMatch(t, []int{1, 2, 3, 4}, snapshot)
				require.ElementsMatch(t, []int{1, 2, 3, 4}, slices.Collect(s.All()))
			} else {
				require.Equal(t, factory.order, snapshot)
				require.Equal(t, factory.order, slices.Collect(s.All()))
			}
			snapshot[0] = 99
			require.False(t, s.Contains(99), "ToSlice must be detached")
			visits := 0
			for range s.All() {
				visits++
				break
			}
			require.Equal(t, 1, visits, "iterator must respect early termination")
			s.Clear()
			require.True(t, s.IsEmpty())
			require.Zero(t, s.Len())
			require.Empty(t, s.ToSlice())
			require.Empty(t, slices.Collect(s.All()))
			require.False(t, s.Contains(1))
			require.True(t, s.Add(7), "Clear must leave the set reusable")
			require.Equal(t, []int{7}, s.ToSlice())
		})
	}
}

func TestSetAlgebraInteroperatesAcrossImplementations(t *testing.T) {
	for _, factory := range setFactories() {
		t.Run(factory.name, func(t *testing.T) {
			other := factory.new(4, 2, 3)
			hash := set.NewHashSet(1, 2, 3)
			require.True(t, hash.Union(other).Equal(set.NewHashSet(1, 2, 3, 4)))
			require.True(t, hash.Intersection(other).Equal(set.NewHashSet(2, 3)))
			require.True(t, hash.Difference(other).Equal(set.NewHashSet(1)))
			require.True(t, hash.SymmetricDifference(other).Equal(set.NewHashSet(1, 4)))
			require.False(t, hash.Equal(other))
			require.False(t, hash.IsSubsetOf(other))
			require.False(t, hash.IsSupersetOf(other))
			require.True(t, set.NewHashSet(2, 3).IsSubsetOf(other))
			require.True(t, set.NewHashSet(1, 2, 3, 4).IsSupersetOf(other))
			require.True(t, set.NewHashSet(1, 2, 3, 4).Intersection(factory.new(3)).Equal(set.NewHashSet(3)))

			sorted := set.NewSkipListSet(3, 2, 1)
			require.Equal(t, []int{1, 2, 3, 4}, sorted.Union(other).ToSlice())
			require.Equal(t, []int{2, 3}, sorted.Intersection(other).ToSlice())
			require.Equal(t, []int{1}, sorted.Difference(other).ToSlice())
			require.False(t, sorted.Equal(other))
			for _, equivalent := range []set.Set[int]{factory.new(1, 2, 3), factory.new(3, 2, 1)} {
				require.True(t, hash.Equal(equivalent))
				require.True(t, sorted.Equal(equivalent))
				require.True(t, set.NewLinkedHashSet(2, 1, 3).Equal(equivalent))
			}
			require.False(t, set.NewLinkedHashSet(1, 2, 3).Equal(other))
			require.True(t, hash.Equal(set.NewHashSet(1, 2, 3)), "algebra must not mutate the receiver")
			require.Equal(t, []int{1, 2, 3}, sorted.ToSlice())
			require.Equal(t, 3, other.Len())
			require.True(t, other.ContainsAll(2, 3, 4), "algebra must not mutate the argument")
		})
	}
}

func TestNavigableSetSupportsNonComparableElements(t *testing.T) {
	compare := slices.Compare[[]int, int]
	concrete := set.NewSkipListSetFunc(compare, []int{5}, []int{1}, []int{3})
	var s set.NavigableSet[[]int] = concrete
	require.False(t, s.Add([]int{3}), "comparator equality must collapse distinct slices")
	require.True(t, s.ContainsAll([]int{1}, []int{3}))
	require.False(t, s.ContainsAll([]int{2}))
	require.Equal(t, [][]int{{1}, {3}, {5}}, s.ToSlice())
	for _, tc := range []struct {
		name string
		call func() ([]int, bool)
		want []int
	}{
		{"First", s.First, []int{1}},
		{"Last", s.Last, []int{5}},
		{"Floor", func() ([]int, bool) { return s.Floor([]int{4}) }, []int{3}},
		{"Ceiling", func() ([]int, bool) { return s.Ceiling([]int{4}) }, []int{5}},
		{"Lower", func() ([]int, bool) { return s.Lower([]int{3}) }, []int{1}},
		{"Higher", func() ([]int, bool) { return s.Higher([]int{3}) }, []int{5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, ok := tc.call()
			require.True(t, ok)
			require.Equal(t, tc.want, value)
		})
	}
	require.Equal(t, [][]int{{3}}, slices.Collect(s.Range([]int{1}, []int{5}, false, false)))
	var other set.Set[[]int] = set.NewSkipListSetFunc(compare, []int{3}, []int{7})
	require.Equal(t, [][]int{{1}, {3}, {5}, {7}}, concrete.Union(other).ToSlice())
	require.Equal(t, [][]int{{3}}, concrete.Intersection(other).ToSlice())
	require.Equal(t, [][]int{{1}, {5}}, concrete.Difference(other).ToSlice())
	require.True(t, s.Remove([]int{3}))
	require.False(t, s.Contains([]int{3}))
	s.Clear()
	require.True(t, s.IsEmpty())
	_, ok := s.First()
	require.False(t, ok)
	_, ok = s.Last()
	require.False(t, ok)
}
