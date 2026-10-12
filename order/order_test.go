package order_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/order"
	"github.com/neko233-com/struct233-go/pqueue"
	"github.com/neko233-com/struct233-go/skiplist"
)

func sample() []int { return []int{3, 1, 4, 1, 5, 9, 2, 6} }

func TestNatural(t *testing.T) {
	require.Equal(t, -1, order.Natural[int]()(1, 2))
	require.Equal(t, 0, order.Natural[int]()(2, 2))
	require.Equal(t, 1, order.Natural[int]()(3, 2))
	require.Equal(t, -1, order.Natural[string]()("a", "b"))
	require.Equal(t, 0, order.Natural[string]()("b", "b"))
	require.Equal(t, 1, order.Natural[float64]()(2.5, 2.4))
}

func TestReverse(t *testing.T) {
	asc := order.Natural[int]()
	desc := order.Reverse(asc)
	require.Equal(t, 1, desc(1, 2))
	require.Equal(t, 0, desc(2, 2))
	require.Equal(t, -1, desc(3, 2))

	t.Run("double reverse matches the original", func(t *testing.T) {
		twice := order.Reverse(order.Reverse(asc))
		for _, pair := range [][2]int{{1, 2}, {2, 2}, {3, 2}, {0, -1}} {
			require.Equal(t, asc(pair[0], pair[1]), twice(pair[0], pair[1]), "pair %v", pair)
		}
	})

	t.Run("a large result is normalised so composition cannot overflow", func(t *testing.T) {
		huge := func(a, b int) int {
			if a < b {
				return math.MinInt
			}
			return math.MaxInt
		}
		require.Equal(t, 1, order.Reverse(huge)(1, 2), "the sign must be flipped, not negated")
	})
}

func TestThen(t *testing.T) {
	type item struct {
		group string
		score int
	}
	byGroup := order.ByKey(func(i item) string { return i.group }, order.Natural[string]())
	byScoreDesc := order.Reverse(order.ByKey(func(i item) int { return i.score }, order.Natural[int]()))

	cmp := order.Then(byGroup, byScoreDesc)
	a := item{"a", 10}
	b := item{"b", 1}

	require.Equal(t, -1, cmp(a, b), "the first comparator decides")
	require.Equal(t, 1, cmp(b, a))
	require.Equal(t, -1, cmp(item{"a", 10}, item{"a", 5}), "the second breaks the tie")
	require.Equal(t, 0, cmp(item{"a", 5}, item{"a", 5}))

	t.Run("one comparator", func(t *testing.T) {
		require.Equal(t, -1, order.Then(byGroup)(a, b))
		require.Equal(t, 0, order.Then[int]()(1, 2), "no comparator means everything is equal")
	})

	t.Run("three comparators", func(t *testing.T) {
		chained := order.Then(byGroup, byScoreDesc, order.Reverse(byScoreDesc))
		require.Equal(t, 1, chained(item{"a", 1}, item{"a", 2}),
			"in descending score order the lower score comes second")
		require.Equal(t, 0, chained(item{"a", 1}, item{"a", 1}))
	})
}

func TestNullsFirstAndLast(t *testing.T) {
	one, two := 1, 2
	nilPtr := (*int)(nil)
	n := order.Natural[int]()

	require.Equal(t, 0, order.NullsFirst(n)(nil, nil))
	require.Equal(t, -1, order.NullsFirst(n)(nil, &one))
	require.Equal(t, 1, order.NullsFirst(n)(&one, nil))
	require.Equal(t, -1, order.NullsFirst(n)(&one, &two))
	require.Equal(t, 1, order.NullsFirst(n)(&two, &one))

	require.Equal(t, 0, order.NullsLast(n)(nilPtr, nil))
	require.Equal(t, 1, order.NullsLast(n)(nilPtr, &one))
	require.Equal(t, -1, order.NullsLast(n)(&one, nil))
	require.Equal(t, -1, order.NullsLast(n)(&one, &two))
}

func TestByKey(t *testing.T) {
	byLen := order.ByKey(func(s string) int { return len(s) }, order.Natural[int]())
	require.Equal(t, -1, byLen("a", "bb"))
	require.Equal(t, 1, byLen("bbb", "bb"))
	require.Equal(t, 0, byLen("ab", "cd"), "equal keys compare equal")

	t.Run("the key function runs once per operand", func(t *testing.T) {
		calls := 0
		cmp := order.ByKey(func(v int) int { calls++; return v % 10 }, order.Natural[int]())
		require.Equal(t, 0, cmp(11, 21))
		require.Equal(t, 2, calls)
	})
}

func TestByAdapter(t *testing.T) {
	asc := order.By(func(a, b int) bool { return a < b })
	require.Equal(t, -1, asc(1, 2))
	require.Equal(t, 1, asc(2, 1))
	require.Equal(t, 0, asc(2, 2))
	require.True(t, order.IsSorted(asc, []int{1, 2, 2, 3}))
	require.False(t, order.IsSorted(asc, []int{1, 3, 2}))
}

func TestMaxAndMin(t *testing.T) {
	cmp := order.Natural[int]()
	require.Equal(t, 5, order.Max(cmp, 5, 3))
	require.Equal(t, 5, order.Max(cmp, 3, 5))
	require.Equal(t, 3, order.Min(cmp, 3, 5))
	require.Equal(t, 3, order.Min(cmp, 5, 3))

	t.Run("a tie prefers the first argument", func(t *testing.T) {
		first := []int{7}
		second := []int{7}
		sameLen := order.ByKey(func(v []int) int { return len(v) }, cmp)
		require.Same(t, &first[0], &order.Max(sameLen, first, second)[0])
		require.Same(t, &first[0], &order.Min(sameLen, first, second)[0])
	})
}

func TestMaxOfAndMinOf(t *testing.T) {
	cmp := order.Natural[int]()

	_, ok := order.MaxOf(cmp, nil)
	require.False(t, ok, "an empty slice has no maximum")
	_, ok = order.MinOf(cmp, nil)
	require.False(t, ok)

	max, ok := order.MaxOf(cmp, []int{4})
	require.True(t, ok)
	require.Equal(t, 4, max)
	min, ok := order.MinOf(cmp, []int{4})
	require.True(t, ok)
	require.Equal(t, 4, min)

	items := sample()
	max, _ = order.MaxOf(cmp, items)
	min, _ = order.MinOf(cmp, items)
	require.Equal(t, 9, max)
	require.Equal(t, 1, min)
}

func TestSorted(t *testing.T) {
	cmp := order.Natural[int]()
	items := sample()
	sorted := order.Sorted(cmp, items)

	require.Equal(t, []int{1, 1, 2, 3, 4, 5, 6, 9}, sorted)
	require.Equal(t, sample(), items, "the input must not be touched")
	require.True(t, order.IsSorted(cmp, sorted))

	t.Run("descending", func(t *testing.T) {
		require.Equal(t, []int{9, 6, 5, 4, 3, 2, 1, 1}, order.Sorted(order.Reverse(cmp), items))
	})

	t.Run("empty and single", func(t *testing.T) {
		require.Empty(t, order.Sorted(cmp, nil))
		require.Equal(t, []int{4}, order.Sorted(cmp, []int{4}))
	})

	t.Run("stable for equal keys", func(t *testing.T) {
		type pair struct{ key, seq int }
		byKey := order.ByKey(func(p pair) int { return p.key }, cmp)
		in := []pair{{2, 0}, {1, 1}, {2, 2}, {1, 3}}
		got := order.Sorted(byKey, in)
		require.Equal(t, []pair{{1, 1}, {1, 3}, {2, 0}, {2, 2}}, got, "equal keys keep their order")
	})
}

func TestIsSorted(t *testing.T) {
	cmp := order.Natural[int]()
	require.True(t, order.IsSorted(cmp, nil))
	require.True(t, order.IsSorted(cmp, []int{1}))
	require.True(t, order.IsSorted(cmp, []int{1, 1, 2}))
	require.True(t, order.IsSorted(cmp, []int{1, 2, 3}))
	require.False(t, order.IsSorted(cmp, []int{1, 3, 2}))
	require.False(t, order.IsSorted(cmp, []int{2, 1}))
	require.True(t, order.IsSorted(order.Reverse(cmp), []int{3, 2, 1}))
}

func TestComparatorsPlugIntoTheContainers(t *testing.T) {
	desc := order.Reverse(order.Natural[int]())

	sl := skiplist.NewFunc[int, string](desc)
	sl.Put(10, "a")
	sl.Put(30, "c")
	sl.Put(20, "b")
	require.Equal(t, []int{30, 20, 10}, sl.Keys(), "a comparator drives the ordered map")

	q := pqueue.NewFunc(desc)
	q.Push(10)
	q.Push(30)
	q.Push(20)
	first, ok := q.Pop()
	require.True(t, ok)
	require.Equal(t, 30, first, "the same comparator drives the priority queue")
}

func TestKeyFunctionsAreCalledDeterministically(t *testing.T) {
	// a comparator derived from a pure function must be pure itself: the same
	// pair always yields the same sign
	byMod := order.ByKey(func(v int) int { return v % 7 }, order.Natural[int]())
	for _, pair := range [][2]int{{1, 8}, {8, 1}, {14, 21}, {3, 4}} {
		first := byMod(pair[0], pair[1])
		for range 5 {
			require.Equal(t, first, byMod(pair[0], pair[1]))
		}
	}
}

func TestZeroValueComparators(t *testing.T) {
	// a nil comparator is not supported, but the combinators themselves never
	// panic on ordinary input and report equality for identical operands
	cmp := order.Then(order.Natural[int](), order.Reverse(order.Natural[int]()))
	require.Zero(t, cmp(5, 5), "both comparators agree")
	require.Equal(t, -1, cmp(4, 5), "the first comparator decides")

	var items []int
	require.True(t, order.IsSorted(cmp, items))
	require.Equal(t, fmt.Sprint([]int(nil)), fmt.Sprint(order.Sorted[int](cmp, items)))
}
