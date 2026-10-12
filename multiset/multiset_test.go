package multiset_test

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/multiset"
)

// assertInvariants re-checks the structural contract of m; every mutation test
// calls it so a broken count can never hide behind a single assertion.
func assertInvariants[T comparable](t *testing.T, m *multiset.Multiset[T]) {
	t.Helper()

	distinct := m.DistinctElements()
	require.Equal(t, m.Distinct(), len(distinct), "Distinct() must equal len(DistinctElements())")
	require.Equal(t, m.Distinct(), len(m.Counts()), "Distinct() must equal len(Counts())")
	seen := make(map[T]struct{}, len(distinct))
	for _, elem := range distinct {
		_, dup := seen[elem]
		require.False(t, dup, "insertion order must hold no duplicate")
		seen[elem] = struct{}{}
	}

	sum := 0
	for _, elem := range distinct {
		n := m.Count(elem)
		require.Greater(t, n, 0, "stored element %v must have a positive count", elem)
		require.True(t, m.Contains(elem), "stored element %v must be contained", elem)
		sum += n
	}
	require.Equal(t, m.TotalCount(), sum, "TotalCount() must equal sum(Count(e))")
	require.Equal(t, m.TotalCount(), len(m.Elements()), "Elements() must hold one entry per occurrence")
	require.Equal(t, m.IsEmpty(), m.TotalCount() == 0, "IsEmpty() must reflect TotalCount()")

	for elem, n := range m.Counts() {
		require.Greater(t, n, 0, "Counts() must never expose a zero count")
		require.Equal(t, n, m.Count(elem))
	}

	elem, n, ok := m.MostCommon()
	if m.IsEmpty() {
		var zero T
		require.False(t, ok, "MostCommon must report ok=false on an empty multiset")
		require.Equal(t, zero, elem)
		require.Zero(t, n)
		return
	}
	require.True(t, ok)
	require.Equal(t, m.Count(elem), n, "MostCommon must agree with Count")
	for _, other := range distinct {
		require.LessOrEqual(t, m.Count(other), n, "MostCommon must pick a maximal count")
	}
}

// model is a plain reference implementation of the multiset semantics used for
// differential testing.
type model struct {
	counts map[int]int
	order  []int
}

// newModel returns an empty reference model.
func newModel() *model {
	return &model{counts: map[int]int{}, order: []int{}}
}

// add counts one more occurrence of elem.
func (mdl *model) add(elem int) int { return mdl.addN(elem, 1) }

// addN counts n more occurrences of elem and returns the new count. A zero count
// is never stored, mirroring the implementation's documented behaviour.
func (mdl *model) addN(elem int, n int) int {
	if n == 0 {
		return mdl.counts[elem]
	}
	if _, ok := mdl.counts[elem]; !ok {
		mdl.order = append(mdl.order, elem)
	}
	mdl.counts[elem] += n
	return mdl.counts[elem]
}

// remove counts one occurrence of elem away.
func (mdl *model) remove(elem int) int { return mdl.removeN(elem, 1) }

// removeN counts n occurrences of elem away and returns the new count.
func (mdl *model) removeN(elem int, n int) int {
	cur := mdl.counts[elem]
	if n >= cur {
		mdl.drop(elem)
		return 0
	}
	mdl.counts[elem] = cur - n
	return cur - n
}

// removeAll drops every occurrence of elem and returns the removed count.
func (mdl *model) removeAll(elem int) int {
	n := mdl.counts[elem]
	if n == 0 {
		return 0
	}
	mdl.drop(elem)
	return n
}

// setCount stores an exact count for elem.
func (mdl *model) setCount(elem int, n int) {
	if n == 0 {
		mdl.drop(elem)
		return
	}
	if _, ok := mdl.counts[elem]; !ok {
		mdl.order = append(mdl.order, elem)
	}
	mdl.counts[elem] = n
}

// drop deletes elem from the model map and inserts order.
func (mdl *model) drop(elem int) {
	if _, ok := mdl.counts[elem]; !ok {
		return
	}
	delete(mdl.counts, elem)
	for i, e := range mdl.order {
		if e == elem {
			mdl.order = append(mdl.order[:i], mdl.order[i+1:]...)
			return
		}
	}
}

// total returns the sum of all counts.
func (mdl *model) total() int {
	total := 0
	for _, n := range mdl.counts {
		total += n
	}
	return total
}

// elements returns one entry per occurrence in first-insertion order.
func (mdl *model) elements() []int {
	out := make([]int, 0, mdl.total())
	for _, elem := range mdl.order {
		for range mdl.counts[elem] {
			out = append(out, elem)
		}
	}
	return out
}

// mostCommon returns the model counterpart of MostCommon.
func (mdl *model) mostCommon() (elem int, count int, ok bool) {
	for _, e := range mdl.order {
		if n := mdl.counts[e]; n > count {
			elem, count, ok = e, n, true
		}
	}
	return elem, count, ok
}

// assertMatchesModel compares the whole observable multiset state with the
// reference model.
func assertMatchesModel(t *testing.T, m *multiset.Multiset[int], mdl *model) {
	t.Helper()
	require.Equal(t, mdl.counts, m.Counts(), "counts")
	require.Equal(t, mdl.order, m.DistinctElements(), "insertion order")
	require.Equal(t, mdl.total(), m.TotalCount(), "total count")
	require.Equal(t, len(mdl.order), m.Distinct(), "distinct")
	require.Equal(t, mdl.total() == 0, m.IsEmpty(), "empty")
	require.Equal(t, mdl.elements(), m.Elements(), "elements")
	mdlElem, mdlCount, mdlOK := mdl.mostCommon()
	elem, count, ok := m.MostCommon()
	require.Equal(t, mdlOK, ok, "most common ok")
	require.Equal(t, mdlElem, elem, "most common element")
	require.Equal(t, mdlCount, count, "most common count")
	assertInvariants(t, m)
}

// TestNewVariants covers the two constructors and their pre-fill behaviour.
func TestNewVariants(t *testing.T) {
	tests := []struct {
		name     string
		build    func() *multiset.Multiset[string]
		distinct []string
		total    int
	}{
		{
			name:  "New with no elements is empty",
			build: func() *multiset.Multiset[string] { return multiset.New[string]() },
		},
		{
			name:     "New with one element",
			build:    func() *multiset.Multiset[string] { return multiset.New("a") },
			distinct: []string{"a"},
			total:    1,
		},
		{
			name:     "New keeps first insertion order across duplicates",
			build:    func() *multiset.Multiset[string] { return multiset.New("b", "a", "b", "c", "a") },
			distinct: []string{"b", "a", "c"},
			total:    5,
		},
		{
			name:     "NewWithCapacity zero still stores elements",
			build:    func() *multiset.Multiset[string] { return multiset.NewWithCapacity(0, "x", "x") },
			distinct: []string{"x"},
			total:    2,
		},
		{
			name:     "NewWithCapacity larger than the elements",
			build:    func() *multiset.Multiset[string] { return multiset.NewWithCapacity(64, "y", "z") },
			distinct: []string{"y", "z"},
			total:    2,
		},
		{
			name:     "NewWithCapacity with no elements is empty",
			build:    func() *multiset.Multiset[string] { return multiset.NewWithCapacity[string](8) },
			distinct: nil,
			total:    0,
		},
		{
			name:     "NewWithCapacity clamps a negative capacity",
			build:    func() *multiset.Multiset[string] { return multiset.NewWithCapacity(-3, "q") },
			distinct: []string{"q"},
			total:    1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.build()
			require.Equal(t, tc.total, m.TotalCount())
			require.Equal(t, len(tc.distinct), m.Distinct())
			if len(tc.distinct) == 0 {
				require.Empty(t, m.DistinctElements())
			} else {
				require.Equal(t, tc.distinct, m.DistinctElements())
			}
			assertInvariants(t, m)
		})
	}
}

// TestAdd covers the resulting state of repeated Add calls.
func TestAdd(t *testing.T) {
	tests := []struct {
		name   string
		elems  []string
		ops    []string
		counts map[string]int
	}{
		{
			name:   "single occurrence",
			ops:    []string{"a"},
			counts: map[string]int{"a": 1},
		},
		{
			name:   "repeated occurrence accumulates",
			ops:    []string{"a", "a", "a"},
			counts: map[string]int{"a": 3},
		},
		{
			name:   "distinct elements each counted once",
			ops:    []string{"b", "a", "c"},
			counts: map[string]int{"b": 1, "a": 1, "c": 1},
		},
		{
			name:   "mixed duplicates",
			ops:    []string{"a", "b", "a", "b", "b"},
			counts: map[string]int{"a": 2, "b": 3},
		},
		{
			name:   "pre-filled then grown",
			elems:  []string{"a", "a"},
			ops:    []string{"a", "b"},
			counts: map[string]int{"a": 3, "b": 1},
		},
		{
			name:   "many elements",
			ops:    []string{"a", "b", "c", "d", "e", "f", "g", "h"},
			counts: map[string]int{"a": 1, "b": 1, "c": 1, "d": 1, "e": 1, "f": 1, "g": 1, "h": 1},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := multiset.New(tc.elems...)
			for _, elem := range tc.ops {
				m.Add(elem)
				assertInvariants(t, m)
			}
			require.Equal(t, tc.counts, m.Counts())
		})
	}
}

// TestAddReturnsNewCount pins the return value of Add.
func TestAddReturnsNewCount(t *testing.T) {
	tests := []struct {
		name string
		ops  []string
		want []int
	}{
		{name: "same element", ops: []string{"a", "a", "a"}, want: []int{1, 2, 3}},
		{name: "two elements", ops: []string{"a", "b", "a"}, want: []int{1, 1, 2}},
		{name: "interleaved", ops: []string{"x", "y", "y", "x", "x"}, want: []int{1, 1, 2, 2, 3}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := multiset.New[string]()
			got := make([]int, 0, len(tc.ops))
			for _, elem := range tc.ops {
				got = append(got, m.Add(elem))
			}
			require.Equal(t, tc.want, got)
		})
	}
}

// TestAddN covers bulk additions, including the zero count.
func TestAddN(t *testing.T) {
	tests := []struct {
		name   string
		elems  []string
		elem   string
		n      int
		want   int
		counts map[string]int
	}{
		{
			name:   "large n on an absent element",
			elem:   "a",
			n:      100,
			want:   100,
			counts: map[string]int{"a": 100},
		},
		{
			name:   "n of one equals Add",
			elem:   "a",
			n:      1,
			want:   1,
			counts: map[string]int{"a": 1},
		},
		{
			name:   "n of zero does not insert",
			elem:   "a",
			n:      0,
			want:   0,
			counts: map[string]int{},
		},
		{
			name:   "n of zero on an existing element is a no-op",
			elems:  []string{"a", "a"},
			elem:   "a",
			n:      0,
			want:   2,
			counts: map[string]int{"a": 2},
		},
		{
			name:   "accumulates onto an existing count",
			elems:  []string{"a", "b"},
			elem:   "a",
			n:      4,
			want:   5,
			counts: map[string]int{"a": 5, "b": 1},
		},
		{
			name:   "other elements are untouched",
			elems:  []string{"a", "b", "c"},
			elem:   "b",
			n:      3,
			want:   4,
			counts: map[string]int{"a": 1, "b": 4, "c": 1},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := multiset.New(tc.elems...)
			require.Equal(t, tc.want, m.AddN(tc.elem, tc.n))
			require.Equal(t, tc.counts, m.Counts())
			assertInvariants(t, m)
		})
	}
}

// TestRemove covers single removals and the drop-on-zero boundary.
func TestRemove(t *testing.T) {
	tests := []struct {
		name   string
		elems  []string
		elem   string
		want   int
		counts map[string]int
	}{
		{
			name:   "absent element stays absent",
			elems:  []string{"a"},
			elem:   "zzz",
			want:   0,
			counts: map[string]int{"a": 1},
		},
		{
			name:   "count one drops the element",
			elems:  []string{"a", "b"},
			elem:   "a",
			want:   0,
			counts: map[string]int{"b": 1},
		},
		{
			name:   "count above one only decrements",
			elems:  []string{"a", "a", "a"},
			elem:   "a",
			want:   2,
			counts: map[string]int{"a": 2},
		},
		{
			name:   "last element of the multiset is dropped",
			elems:  []string{"only"},
			elem:   "only",
			want:   0,
			counts: map[string]int{},
		},
		{
			name:   "repeated removals walk down to zero",
			elems:  []string{"a", "a"},
			elem:   "a",
			want:   1,
			counts: map[string]int{"a": 1},
		},
		{
			name:   "elements after the removed one keep their order",
			elems:  []string{"a", "b", "c"},
			elem:   "a",
			want:   0,
			counts: map[string]int{"b": 1, "c": 1},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := multiset.New(tc.elems...)
			require.Equal(t, tc.want, m.Remove(tc.elem))
			require.Equal(t, tc.counts, m.Counts())
			assertInvariants(t, m)
		})
	}
}

// TestRemoveDropsFromInsertionOrder documents that a dropped element leaves the
// insertion order and a later Add re-inserts it at the end.
func TestRemoveDropsFromInsertionOrder(t *testing.T) {
	t.Run("drop then re-add moves the element to the end", func(t *testing.T) {
		m := multiset.New("a", "b", "c")
		require.Equal(t, []string{"a", "b", "c"}, m.DistinctElements())
		require.Zero(t, m.Remove("a"))
		require.Equal(t, []string{"b", "c"}, m.DistinctElements())
		require.Equal(t, 1, m.Add("a"))
		require.Equal(t, []string{"b", "c", "a"}, m.DistinctElements())
		require.Equal(t, []string{"b", "c", "a"}, m.Elements())
		assertInvariants(t, m)
	})

	t.Run("re-add after RemoveN lands at the end", func(t *testing.T) {
		m := multiset.New("a", "a", "b")
		require.Zero(t, m.RemoveN("a", 2))
		require.Equal(t, []string{"b"}, m.DistinctElements())
		require.Equal(t, 2, m.AddN("a", 2))
		require.Equal(t, []string{"b", "a"}, m.DistinctElements())
		assertInvariants(t, m)
	})

	t.Run("RemoveAll also drops the element from the order", func(t *testing.T) {
		m := multiset.New("a", "b")
		require.Equal(t, 1, m.RemoveAll("a"))
		require.Equal(t, []string{"b"}, m.DistinctElements())
		assertInvariants(t, m)
	})

	t.Run("SetCount zero then a positive SetCount lands at the end", func(t *testing.T) {
		m := multiset.New("a", "b")
		m.SetCount("a", 0)
		require.Equal(t, []string{"b"}, m.DistinctElements())
		m.SetCount("a", 7)
		require.Equal(t, []string{"b", "a"}, m.DistinctElements())
		assertInvariants(t, m)
	})

	t.Run("dropping every element leaves an empty order", func(t *testing.T) {
		m := multiset.New("a", "b", "c")
		for _, elem := range []string{"a", "b", "c"} {
			m.Remove(elem)
			assertInvariants(t, m)
		}
		require.Empty(t, m.DistinctElements())
		require.Empty(t, m.Elements())
		require.True(t, m.IsEmpty())
	})
}

// TestRemoveN covers bulk removals and the over-removal boundary.
func TestRemoveN(t *testing.T) {
	tests := []struct {
		name    string
		elems   []string
		elem    string
		n       int
		want    int
		total   int
		present bool
	}{
		{
			name:    "partial removal",
			elems:   []string{"a", "a", "a", "a", "a"},
			elem:    "a",
			n:       2,
			want:    3,
			total:   3,
			present: true,
		},
		{
			name:    "exact removal drops the element",
			elems:   []string{"a", "a"},
			elem:    "a",
			n:       2,
			want:    0,
			total:   0,
			present: false,
		},
		{
			name:    "removing more occurrences than exist drops the element",
			elems:   []string{"a", "a"},
			elem:    "a",
			n:       99,
			want:    0,
			total:   0,
			present: false,
		},
		{
			name:    "n of zero is a no-op",
			elems:   []string{"a", "a"},
			elem:    "a",
			n:       0,
			want:    2,
			total:   2,
			present: true,
		},
		{
			name:    "n of zero on an absent element",
			elems:   []string{"a"},
			elem:    "zzz",
			n:       0,
			want:    0,
			total:   1,
			present: false,
		},
		{
			name:    "absent element with a positive n",
			elems:   []string{"a"},
			elem:    "zzz",
			n:       5,
			want:    0,
			total:   1,
			present: false,
		},
		{
			name:    "other elements are untouched",
			elems:   []string{"a", "b", "b"},
			elem:    "b",
			n:       1,
			want:    1,
			total:   2,
			present: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := multiset.New(tc.elems...)
			require.Equal(t, tc.want, m.RemoveN(tc.elem, tc.n))
			require.Equal(t, tc.want, m.Count(tc.elem))
			require.Equal(t, tc.present, m.Contains(tc.elem))
			require.Equal(t, tc.total, m.TotalCount())
			assertInvariants(t, m)
		})
	}
}

// TestRemoveAll covers dropping an element entirely.
func TestRemoveAll(t *testing.T) {
	tests := []struct {
		name   string
		elems  []string
		elem   string
		want   int
		counts map[string]int
	}{
		{
			name:   "absent element removes nothing",
			elems:  []string{"a"},
			elem:   "zzz",
			want:   0,
			counts: map[string]int{"a": 1},
		},
		{
			name:   "single occurrence",
			elems:  []string{"a", "b"},
			elem:   "a",
			want:   1,
			counts: map[string]int{"b": 1},
		},
		{
			name:   "many occurrences",
			elems:  []string{"a", "a", "a", "b"},
			elem:   "a",
			want:   3,
			counts: map[string]int{"b": 1},
		},
		{
			name:   "last element empties the multiset",
			elems:  []string{"only", "only"},
			elem:   "only",
			want:   2,
			counts: map[string]int{},
		},
		{
			name:   "second RemoveAll returns zero",
			elems:  []string{"a"},
			elem:   "a",
			want:   1,
			counts: map[string]int{},
		},
		{
			name:   "other elements keep their counts",
			elems:  []string{"a", "b", "b", "c"},
			elem:   "b",
			want:   2,
			counts: map[string]int{"a": 1, "c": 1},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := multiset.New(tc.elems...)
			require.Equal(t, tc.want, m.RemoveAll(tc.elem))
			require.Zero(t, m.Count(tc.elem))
			require.Equal(t, tc.counts, m.Counts())
			assertInvariants(t, m)
		})
	}
}

// TestSetCount covers storing exact counts.
func TestSetCount(t *testing.T) {
	tests := []struct {
		name   string
		elems  []string
		elem   string
		n      int
		counts map[string]int
	}{
		{
			name:   "upward on an existing element",
			elems:  []string{"a"},
			elem:   "a",
			n:      10,
			counts: map[string]int{"a": 10},
		},
		{
			name:   "downward on an existing element",
			elems:  []string{"a", "a", "a"},
			elem:   "a",
			n:      1,
			counts: map[string]int{"a": 1},
		},
		{
			name:   "zero drops the element",
			elems:  []string{"a", "b"},
			elem:   "a",
			n:      0,
			counts: map[string]int{"b": 1},
		},
		{
			name:   "zero on an absent element is a no-op",
			elems:  []string{"a"},
			elem:   "zzz",
			n:      0,
			counts: map[string]int{"a": 1},
		},
		{
			name:   "positive on an absent element inserts it",
			elems:  []string{"a"},
			elem:   "zzz",
			n:      4,
			counts: map[string]int{"a": 1, "zzz": 4},
		},
		{
			name:   "same value again is idempotent",
			elems:  []string{"a", "a"},
			elem:   "a",
			n:      2,
			counts: map[string]int{"a": 2},
		},
		{
			name:   "zero on the last element empties the multiset",
			elems:  []string{"only"},
			elem:   "only",
			n:      0,
			counts: map[string]int{},
		},
		{
			name:   "large count",
			elems:  []string{"a", "b"},
			elem:   "b",
			n:      1 << 20,
			counts: map[string]int{"a": 1, "b": 1 << 20},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := multiset.New(tc.elems...)
			m.SetCount(tc.elem, tc.n)
			require.Equal(t, tc.counts, m.Counts())
			assertInvariants(t, m)
		})
	}
}

// TestQueries covers Count, Contains, TotalCount, Distinct and IsEmpty.
func TestQueries(t *testing.T) {
	tests := []struct {
		name      string
		elems     []string
		elem      string
		wantCount int
		contains  bool
		total     int
		distinct  int
		empty     bool
	}{
		{
			name:      "empty multiset",
			elem:      "a",
			wantCount: 0,
			total:     0,
			distinct:  0,
			empty:     true,
		},
		{
			name:      "absent element of a populated multiset",
			elems:     []string{"a", "a"},
			elem:      "b",
			wantCount: 0,
			total:     2,
			distinct:  1,
		},
		{
			name:      "present element",
			elems:     []string{"a", "a", "a"},
			elem:      "a",
			wantCount: 3,
			contains:  true,
			total:     3,
			distinct:  1,
		},
		{
			name:      "several distinct elements",
			elems:     []string{"a", "b", "a", "c", "b", "b"},
			elem:      "b",
			wantCount: 3,
			contains:  true,
			total:     6,
			distinct:  3,
		},
		{
			name:      "single element",
			elems:     []string{"x"},
			elem:      "x",
			wantCount: 1,
			contains:  true,
			total:     1,
			distinct:  1,
		},
		{
			name:      "empty string element",
			elems:     []string{"", "", "a"},
			elem:      "",
			wantCount: 2,
			contains:  true,
			total:     3,
			distinct:  2,
		},
		{
			name:      "case sensitive",
			elems:     []string{"A"},
			elem:      "a",
			wantCount: 0,
			total:     1,
			distinct:  1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := multiset.New(tc.elems...)
			require.Equal(t, tc.wantCount, m.Count(tc.elem))
			require.Equal(t, tc.contains, m.Contains(tc.elem))
			require.Equal(t, tc.total, m.TotalCount())
			require.Equal(t, tc.distinct, m.Distinct())
			require.Equal(t, tc.empty, m.IsEmpty())
			assertInvariants(t, m)
		})
	}
}

// TestEmptyMultisetStatesEveryObservationOfAnEmptyMultiset.
func TestEmptyMultisetStatesEveryObservationOfAnEmptyMultiset(t *testing.T) {
	m := multiset.New[string]()
	require.Zero(t, m.Count("absent"))
	require.False(t, m.Contains("absent"))
	require.Zero(t, m.TotalCount())
	require.Zero(t, m.Distinct())
	require.True(t, m.IsEmpty())
	require.Empty(t, m.Elements())
	require.Empty(t, m.DistinctElements())
	require.Empty(t, m.Counts())
	require.NotNil(t, m.Counts(), "Counts must return a usable empty map")
	require.NotNil(t, m.Elements(), "Elements returns a usable empty slice so callers can range it")

	_, count, ok := m.MostCommon()
	require.False(t, ok, "MostCommon reports ok=false when empty")
	require.Zero(t, count)

	require.Zero(t, m.Remove("absent"))
	require.Zero(t, m.RemoveN("absent", 3))
	require.Zero(t, m.RemoveAll("absent"))
	require.Equal(t, 0, m.AddN("absent", 0))
	require.True(t, m.IsEmpty(), "read-only and zero-count calls must not insert")

	m.SetCount("absent", 0)
	require.True(t, m.IsEmpty(), "SetCount to zero must not insert")

	m.Clear()
	require.True(t, m.IsEmpty())

	require.True(t, m.Equal(multiset.New[string]()))
	assertInvariants(t, m)
}

// TestElementsAndDistinctElementsOrder pins the observable ordering.
func TestElementsAndDistinctElementsOrder(t *testing.T) {
	tests := []struct {
		name     string
		build    func() *multiset.Multiset[string]
		elements []string
		distinct []string
	}{
		{
			name:  "empty",
			build: func() *multiset.Multiset[string] { return multiset.New[string]() },
		},
		{
			name:     "single",
			build:    func() *multiset.Multiset[string] { return multiset.New("a") },
			elements: []string{"a"},
			distinct: []string{"a"},
		},
		{
			name:     "occurrences are grouped by first insertion",
			build:    func() *multiset.Multiset[string] { return multiset.New("b", "a", "b") },
			elements: []string{"b", "b", "a"},
			distinct: []string{"b", "a"},
		},
		{
			name: "order is not alphabetical",
			build: func() *multiset.Multiset[string] {
				return multiset.New("zeta", "alpha", "mu")
			},
			elements: []string{"zeta", "alpha", "mu"},
			distinct: []string{"zeta", "alpha", "mu"},
		},
		{
			name: "after SetCount",
			build: func() *multiset.Multiset[string] {
				m := multiset.New("a", "b")
				m.SetCount("a", 3)
				return m
			},
			elements: []string{"a", "a", "a", "b"},
			distinct: []string{"a", "b"},
		},
		{
			name: "after a drop and re-add",
			build: func() *multiset.Multiset[string] {
				m := multiset.New("a", "b", "c")
				m.RemoveAll("a")
				m.Add("a")
				return m
			},
			elements: []string{"b", "c", "a"},
			distinct: []string{"b", "c", "a"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.build()
			if len(tc.elements) == 0 {
				require.Empty(t, m.Elements())
			} else {
				require.Equal(t, tc.elements, m.Elements())
			}
			if len(tc.distinct) == 0 {
				require.Empty(t, m.DistinctElements())
			} else {
				require.Equal(t, tc.distinct, m.DistinctElements())
			}
			assertInvariants(t, m)
		})
	}
}

// TestSnapshotSlicesAreCopies checks that query results never alias live state.
func TestSnapshotSlicesAreCopies(t *testing.T) {
	t.Run("DistinctElements does not alias the insertion order", func(t *testing.T) {
		m := multiset.New("a", "b")
		got := m.DistinctElements()
		got[0] = "mutated"
		require.Equal(t, []string{"a", "b"}, m.DistinctElements())
	})

	t.Run("Elements does not alias live state", func(t *testing.T) {
		m := multiset.New("a", "b")
		got := m.Elements()
		got[0] = "mutated"
		require.Equal(t, []string{"a", "b"}, m.Elements())
	})

	t.Run("Counts does not alias the internal map", func(t *testing.T) {
		m := multiset.New("a", "a")
		got := m.Counts()
		got["a"] = 100
		delete(got, "a")
		got["ghost"] = 1
		require.Equal(t, map[string]int{"a": 2}, m.Counts())
		require.Equal(t, 2, m.Count("a"))
		require.False(t, m.Contains("ghost"))
	})

	t.Run("mutating the multiset does not change an old snapshot", func(t *testing.T) {
		m := multiset.New("a")
		snapshot := m.Counts()
		m.Add("a")
		m.Add("b")
		require.Equal(t, map[string]int{"a": 1}, snapshot)
	})

	t.Run("Counts of an empty multiset is empty but non-nil", func(t *testing.T) {
		got := multiset.New[string]().Counts()
		require.NotNil(t, got)
		require.Empty(t, got)
	})
}

// TestMostCommon covers the argmax and its tie-breaking rule.
func TestMostCommon(t *testing.T) {
	tests := []struct {
		name  string
		build func() *multiset.Multiset[string]
		want  string
		count int
		ok    bool
	}{
		{
			name:  "empty reports ok=false",
			build: func() *multiset.Multiset[string] { return multiset.New[string]() },
			want:  "",
			count: 0,
			ok:    false,
		},
		{
			name:  "single element",
			build: func() *multiset.Multiset[string] { return multiset.New("a") },
			want:  "a",
			count: 1,
			ok:    true,
		},
		{
			name:  "clear winner",
			build: func() *multiset.Multiset[string] { return multiset.New("a", "b", "b", "b") },
			want:  "b",
			count: 3,
			ok:    true,
		},
		{
			name:  "tie goes to the first inserted",
			build: func() *multiset.Multiset[string] { return multiset.New("b", "a") },
			want:  "b",
			count: 1,
			ok:    true,
		},
		{
			name:  "tie among heavy elements goes to the first inserted",
			build: func() *multiset.Multiset[string] { return multiset.New("a", "b", "b", "a") },
			want:  "a",
			count: 2,
			ok:    true,
		},
		{
			name: "dropping the leader promotes the runner up",
			build: func() *multiset.Multiset[string] {
				m := multiset.New("a", "a", "b")
				m.RemoveAll("a")
				return m
			},
			want:  "b",
			count: 1,
			ok:    true,
		},
		{
			name: "re-inserted element loses the tie",
			build: func() *multiset.Multiset[string] {
				m := multiset.New("a", "b")
				m.RemoveAll("a")
				m.Add("a")
				return m
			},
			want:  "b",
			count: 1,
			ok:    true,
		},
		{
			name: "SetCount decides the winner",
			build: func() *multiset.Multiset[string] {
				m := multiset.New("a", "b")
				m.SetCount("b", 5)
				return m
			},
			want:  "b",
			count: 5,
			ok:    true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			elem, count, ok := tc.build().MostCommon()
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, elem)
			require.Equal(t, tc.count, count)
		})
	}
}

// TestEqual covers the count-only comparison semantics.
func TestEqual(t *testing.T) {
	tests := []struct {
		name  string
		left  *multiset.Multiset[string]
		right *multiset.Multiset[string]
		want  bool
	}{
		{
			name:  "two empty multisets are equal",
			left:  multiset.New[string](),
			right: multiset.New[string](),
			want:  true,
		},
		{
			name:  "same counts in the same order",
			left:  multiset.New("a", "b"),
			right: multiset.New("a", "b"),
			want:  true,
		},
		{
			name:  "same counts in a different insertion order",
			left:  multiset.New("a", "b"),
			right: multiset.New("b", "a"),
			want:  true,
		},
		{
			name:  "different counts",
			left:  multiset.New("a", "a"),
			right: multiset.New("a"),
			want:  false,
		},
		{
			name:  "same total but different elements",
			left:  multiset.New("a", "b"),
			right: multiset.New("c", "d"),
			want:  false,
		},
		{
			name:  "empty against populated",
			left:  multiset.New[string](),
			right: multiset.New("a"),
			want:  false,
		},
		{
			name:  "the same counts built by different operations",
			left:  multiset.New("a", "a", "b"),
			right: multiset.NewWithCapacity[string](4),
			want:  false,
		},
		{
			name:  "nil is never equal",
			left:  multiset.New("a"),
			right: nil,
			want:  false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.left.Equal(tc.right))
			if tc.right != nil {
				require.Equal(t, tc.want, tc.right.Equal(tc.left), "Equal must be symmetric")
			}
		})
	}

	t.Run("multisets built differently but holding equal counts, empty", func(t *testing.T) {
		left := multiset.New("a", "a", "b")
		right := multiset.New[string]()
		right.Add("b")
		right.AddN("a", 2)
		require.True(t, left.Equal(right))
	})

	t.Run("multisets built differently but holding equal counts, non-empty", func(t *testing.T) {
		left := multiset.New("a", "a", "b", "c", "c", "c")
		right := multiset.New("b", "c")
		right.AddN("c", 2)
		right.AddN("a", 2)
		require.True(t, left.Equal(right))
		require.Equal(t, []string{"a", "b", "c"}, left.DistinctElements())
		require.Equal(t, []string{"b", "c", "a"}, right.DistinctElements())
	})
}

// TestClear covers the reset behaviour and the empty capacity it leaves behind.
func TestClear(t *testing.T) {
	t.Run("clear empties a populated multiset", func(t *testing.T) {
		m := multiset.New("a", "a", "b")
		m.Clear()
		require.True(t, m.IsEmpty())
		require.Zero(t, m.TotalCount())
		require.Zero(t, m.Distinct())
		require.Empty(t, m.DistinctElements())
		require.Empty(t, m.Elements())
		assertInvariants(t, m)
	})

	t.Run("clear on an empty multiset is a no-op", func(t *testing.T) {
		m := multiset.New[string]()
		m.Clear()
		m.Clear()
		require.True(t, m.IsEmpty())
		assertInvariants(t, m)
	})

	t.Run("clear resets the insertion order", func(t *testing.T) {
		m := multiset.New("a", "b")
		m.Clear()
		m.Add("b")
		m.Add("a")
		require.Equal(t, []string{"b", "a"}, m.DistinctElements())
	})

	t.Run("clear then reuse keeps the counts correct", func(t *testing.T) {
		m := multiset.New("a", "a")
		m.Clear()
		require.Equal(t, 3, m.AddN("a", 3))
		require.Equal(t, map[string]int{"a": 3}, m.Counts())
		assertInvariants(t, m)
	})
}

// TestAllIterator covers the iter.Seq view, including early termination.
func TestAllIterator(t *testing.T) {
	tests := []struct {
		name  string
		elems []string
		want  []string
	}{
		{name: "empty yields nothing", elems: nil, want: []string{}},
		{name: "single element", elems: []string{"a"}, want: []string{"a"}},
		{name: "occurrences are grouped", elems: []string{"b", "a", "b"}, want: []string{"b", "b", "a"}},
		{name: "many distinct elements", elems: []string{"c", "b", "a"}, want: []string{"c", "b", "a"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := multiset.New(tc.elems...)
			got := []string{}
			for elem := range m.All() {
				got = append(got, elem)
			}
			require.Equal(t, tc.want, got)
		})
	}

	t.Run("All agrees with Elements", func(t *testing.T) {
		m := multiset.New("x", "y", "x", "z", "y", "y")
		got := []string{}
		for elem := range m.All() {
			got = append(got, elem)
		}
		require.Equal(t, m.Elements(), got)
	})

	t.Run("breaking out of All stops the iteration", func(t *testing.T) {
		m := multiset.New("a", "a", "b")
		seen := []string{}
		for elem := range m.All() {
			seen = append(seen, elem)
			break
		}
		require.Equal(t, []string{"a"}, seen)
	})
}

// TestInvalidArgumentsPanic pins the exported panic message and proves the
// multiset is left untouched by a rejected call.
func TestInvalidArgumentsPanic(t *testing.T) {
	tests := []struct {
		name string
		call func(m *multiset.Multiset[string])
	}{
		{
			name: "AddN with a negative count",
			call: func(m *multiset.Multiset[string]) { m.AddN("pre", -1) },
		},
		{
			name: "AddN with math.MinInt",
			call: func(m *multiset.Multiset[string]) { m.AddN("pre", math.MinInt) },
		},
		{
			name: "RemoveN with a negative count",
			call: func(m *multiset.Multiset[string]) { m.RemoveN("pre", -1) },
		},
		{
			name: "RemoveN with math.MinInt",
			call: func(m *multiset.Multiset[string]) { m.RemoveN("pre", math.MinInt) },
		},
		{
			name: "SetCount with a negative count",
			call: func(m *multiset.Multiset[string]) { m.SetCount("pre", -1) },
		},
		{
			name: "SetCount with math.MinInt",
			call: func(m *multiset.Multiset[string]) { m.SetCount("pre", math.MinInt) },
		},
		{
			name: "AddN on an absent element",
			call: func(m *multiset.Multiset[string]) { m.AddN("absent", -2) },
		},
		{
			name: "SetCount on an absent element",
			call: func(m *multiset.Multiset[string]) { m.SetCount("absent", -2) },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := multiset.New("pre")
			require.PanicsWithValue(t, "multiset: negative count", func() { tc.call(m) })
			require.Equal(t, map[string]int{"pre": 1}, m.Counts(), "a rejected call must not mutate the multiset")
			assertInvariants(t, m)
		})
	}
}

// TestInvariantsAfterEveryMutation re-checks the invariants after each mutation
// kind on a shared starting state.
func TestInvariantsAfterEveryMutation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, m *multiset.Multiset[string])
	}{
		{
			name:   "Add",
			mutate: func(t *testing.T, m *multiset.Multiset[string]) { m.Add("a") },
		},
		{
			name:   "AddN",
			mutate: func(t *testing.T, m *multiset.Multiset[string]) { m.AddN("a", 7) },
		},
		{
			name: "AddN zero",
			mutate: func(t *testing.T, m *multiset.Multiset[string]) {
				require.Zero(t, m.AddN("absent", 0), "a zero count must report the unchanged count")
				require.False(t, m.Contains("absent"), "a zero count must not insert")
				require.Equal(t, 3, m.AddN("a", 0), "a zero count must leave an existing count alone")
			},
		},
		{
			name:   "Remove",
			mutate: func(t *testing.T, m *multiset.Multiset[string]) { m.Remove("a") },
		},
		{
			name:   "RemoveN partial",
			mutate: func(t *testing.T, m *multiset.Multiset[string]) { m.RemoveN("a", 1) },
		},
		{
			name:   "RemoveN past zero",
			mutate: func(t *testing.T, m *multiset.Multiset[string]) { m.RemoveN("a", 10) },
		},
		{
			name:   "RemoveAll",
			mutate: func(t *testing.T, m *multiset.Multiset[string]) { m.RemoveAll("a") },
		},
		{
			name:   "RemoveAll of an absent element",
			mutate: func(t *testing.T, m *multiset.Multiset[string]) { m.RemoveAll("ghost") },
		},
		{
			name:   "SetCount positive",
			mutate: func(t *testing.T, m *multiset.Multiset[string]) { m.SetCount("a", 9) },
		},
		{
			name:   "SetCount zero",
			mutate: func(t *testing.T, m *multiset.Multiset[string]) { m.SetCount("a", 0) },
		},
		{
			name: "remove every element one by one",
			mutate: func(t *testing.T, m *multiset.Multiset[string]) {
				for range 3 {
					m.Remove("a")
					assertInvariants(t, m)
				}
			},
		},
		{
			name:   "Clear",
			mutate: func(t *testing.T, m *multiset.Multiset[string]) { m.Clear() },
		},
		{
			name: "clear then rebuild",
			mutate: func(t *testing.T, m *multiset.Multiset[string]) {
				m.Clear()
				m.Add("z")
				m.AddN("y", 2)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := multiset.New("a", "a", "a", "b", "c", "b")
			assertInvariants(t, m)
			tc.mutate(t, m)
			assertInvariants(t, m)
		})
	}
}

// TestModelDifferentialRandomOps replays randomised operations against a plain
// map model and compares the whole observable state after every step.
func TestModelDifferentialRandomOps(t *testing.T) {
	seeds := []uint64{1, 7, 42, 1337, 20241012}
	for _, seed := range seeds {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			r := rand.New(rand.NewPCG(seed, 0x233))
			m := multiset.New[int]()
			mdl := newModel()

			for step := range 500 {
				elem := r.IntN(6)
				switch r.IntN(6) {
				case 0:
					require.Equal(t, mdl.add(elem), m.Add(elem), "step %d", step)
				case 1:
					n := r.IntN(5)
					require.Equal(t, mdl.addN(elem, n), m.AddN(elem, n), "step %d", step)
				case 2:
					require.Equal(t, mdl.remove(elem), m.Remove(elem), "step %d", step)
				case 3:
					n := r.IntN(6)
					require.Equal(t, mdl.removeN(elem, n), m.RemoveN(elem, n), "step %d", step)
				case 4:
					require.Equal(t, mdl.removeAll(elem), m.RemoveAll(elem), "step %d", step)
				case 5:
					n := r.IntN(5)
					mdl.setCount(elem, n)
					m.SetCount(elem, n)
				}
				assertMatchesModel(t, m, mdl)
			}

			// drain everything and confirm both implementations end empty
			for _, elem := range slices.Clone(mdl.order) {
				require.Equal(t, mdl.removeAll(elem), m.RemoveAll(elem))
				assertMatchesModel(t, m, mdl)
			}
			require.True(t, m.IsEmpty())
			require.Empty(t, m.Elements())

			// a fresh Add after the drain restarts the insertion order
			require.Equal(t, 1, m.Add(4))
			require.Equal(t, []int{4}, m.DistinctElements())
			assertInvariants(t, m)
		})
	}
}

// lockedMultiset serialises every access with one mutex, which is what makes
// the concurrency test race-clean by construction.
type lockedMultiset struct {
	mu    sync.Mutex
	inner *multiset.Multiset[string]
}

// add counts one more occurrence under the lock.
func (l *lockedMultiset) add(elem string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.inner.Add(elem)
}

// addN counts n more occurrences under the lock.
func (l *lockedMultiset) addN(elem string, n int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.inner.AddN(elem, n)
}

// remove counts one occurrence away under the lock.
func (l *lockedMultiset) remove(elem string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.inner.Remove(elem)
}

// count reads a count under the lock.
func (l *lockedMultiset) count(elem string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inner.Count(elem)
}

// totalCount reads the total under the lock.
func (l *lockedMultiset) totalCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inner.TotalCount()
}

// TestConcurrentAccess uses one mutex around every call, so the test is
// race-clean by construction; with -race it also proves the wrapper is needed.
func TestConcurrentAccess(t *testing.T) {
	const (
		goroutines = 8
		ops        = 200
		contended  = "shared"
		drained    = "drained"
	)

	m := multiset.New[string]()
	guarded := &lockedMultiset{inner: m}
	guarded.addN(drained, goroutines*ops)

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%02d", id)
			for range ops {
				guarded.add(key)
				guarded.add(contended)
				guarded.remove(drained)
				_ = guarded.count(key)
				_ = guarded.totalCount()
			}
		}(g)
	}
	wg.Wait()

	for g := range goroutines {
		require.Equal(t, ops, m.Count(fmt.Sprintf("key-%02d", g)), "per-goroutine key")
	}
	require.Equal(t, goroutines*ops, m.Count(contended), "contended key")
	require.Zero(t, m.Count(drained))
	require.False(t, m.Contains(drained), "fully drained element must leave the multiset")
	require.Equal(t, 2*goroutines*ops, m.TotalCount())
	require.Equal(t, goroutines+1, m.Distinct())
	assertInvariants(t, m)
}

// TestAllocationsOnHotPaths guards the steady-state hot paths against
// unexpected allocations: the element always exists and the map never grows.
func TestAllocationsOnHotPaths(t *testing.T) {
	t.Run("Add and Remove of a present element", func(t *testing.T) {
		m := multiset.New("hot")
		allocs := testing.AllocsPerRun(1000, func() {
			m.Add("hot")
			m.Remove("hot")
		})
		require.Zero(t, allocs)
	})

	t.Run("AddN of a present element", func(t *testing.T) {
		m := multiset.New("hot", "hot")
		allocs := testing.AllocsPerRun(1000, func() {
			m.AddN("hot", 2)
			m.RemoveN("hot", 2)
		})
		require.Zero(t, allocs)
	})

	t.Run("Count and Contains", func(t *testing.T) {
		m := multiset.New("hot")
		allocs := testing.AllocsPerRun(1000, func() {
			_ = m.Count("hot")
			_ = m.Contains("hot")
		})
		require.Zero(t, allocs)
	})

	t.Run("Count of an absent element", func(t *testing.T) {
		m := multiset.New("hot")
		allocs := testing.AllocsPerRun(1000, func() {
			_ = m.Count("cold")
		})
		require.Zero(t, allocs)
	})

	t.Run("Remove on an element that is re-added", func(t *testing.T) {
		m := multiset.New("dup", "dup")
		allocs := testing.AllocsPerRun(1000, func() {
			m.Remove("dup")
			m.Add("dup")
		})
		require.Zero(t, allocs)
	})

	t.Run("SetCount of a present element", func(t *testing.T) {
		m := multiset.New("hot")
		allocs := testing.AllocsPerRun(1000, func() {
			m.SetCount("hot", 1)
		})
		require.Zero(t, allocs)
	})
}
