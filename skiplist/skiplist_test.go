package skiplist_test

import (
	"iter"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/skiplist"
)

func mustGet[K comparable, V any](t *testing.T, sl *skiplist.SkipList[K, V], key K) V {
	t.Helper()
	v, ok := sl.Get(key)
	require.True(t, ok, "key %v must exist", key)
	return v
}

func TestPutGet(t *testing.T) {
	sl := skiplist.New[int, string]()

	_, ok := sl.Get(1)
	require.False(t, ok, "Get on empty skiplist must miss")

	old, existed := sl.Put(1, "a")
	require.False(t, existed, "first Put must report no previous value")
	require.Empty(t, old)
	require.Equal(t, "a", mustGet(t, sl, 1))

	old, existed = sl.Put(1, "b")
	require.True(t, existed)
	require.Equal(t, "a", old)
	require.Equal(t, "b", mustGet(t, sl, 1))
	require.Equal(t, 1, sl.Len())
	require.True(t, sl.ContainsKey(1))
	require.False(t, sl.ContainsKey(2))
}

func TestRemove(t *testing.T) {
	sl := skiplist.New[int, int]()
	for k := range 10 {
		sl.Put(k, k*10)
	}

	old, removed := sl.Remove(3)
	require.True(t, removed)
	require.Equal(t, 30, old)

	_, removed = sl.Remove(3)
	require.False(t, removed, "removing an absent key must report false")
	require.Equal(t, 9, sl.Len())

	require.Equal(t, []int{0, 1, 2, 4, 5, 6, 7, 8, 9}, sl.Keys(),
		"chain must stay ordered and traversable after deletion")
}

func TestRandomOpsAgainstReference(t *testing.T) {
	sl := skiplist.New[int, int]()
	ref := make(map[int]int)
	rng := rand.New(rand.NewPCG(42, 2026))

	for range 5000 {
		k := rng.IntN(1000)
		switch rng.IntN(3) {
		case 0, 1:
			sl.Put(k, k*3)
			ref[k] = k * 3
		case 2:
			sl.Remove(k)
			delete(ref, k)
		}
	}

	require.Equal(t, len(ref), sl.Len())

	wantKeys := make([]int, 0, len(ref))
	for k := range ref {
		wantKeys = append(wantKeys, k)
	}
	slices.Sort(wantKeys)
	require.Equal(t, wantKeys, sl.Keys(), "keys must match the sorted reference")

	for k, want := range ref {
		require.Equal(t, want, mustGet(t, sl, k))
	}
}

func TestFirstLast(t *testing.T) {
	sl := skiplist.New[int, string]()

	_, _, ok := sl.First()
	require.False(t, ok, "First on empty skiplist must miss")
	_, _, ok = sl.Last()
	require.False(t, ok, "Last on empty skiplist must miss")

	sl.Put(5, "e")
	sl.Put(1, "a")
	sl.Put(9, "i")

	k, v, ok := sl.First()
	require.True(t, ok)
	require.Equal(t, 1, k)
	require.Equal(t, "a", v)

	k, v, ok = sl.Last()
	require.True(t, ok)
	require.Equal(t, 9, k)
	require.Equal(t, "i", v)
}

func assertNav(t *testing.T, name string, key, gotKey int, gotOK bool, wantKey int, wantOK bool) {
	t.Helper()
	require.Equal(t, wantOK, gotOK, "%s(%d) presence", name, key)
	if gotOK {
		require.Equal(t, wantKey, gotKey, "%s(%d) key", name, key)
	}
}

func TestNavigableOps(t *testing.T) {
	sl := skiplist.New[int, int]()
	for k := range 10 {
		sl.Put(k*10, k) // keys: 0, 10, 20, ..., 90
	}

	cases := []struct {
		name                               string
		key                                int
		floor, ceiling, lower, higher      int
		floorOK, ceilOK, lowerOK, higherOK bool
	}{
		{"exact hit", 30, 30, 30, 20, 40, true, true, true, true},
		{"gap", 35, 30, 40, 30, 40, true, true, true, true},
		{"below range", 5, 0, 10, 0, 10, true, true, true, true},
		{"above range", 105, 90, 0, 90, 0, true, false, true, false},
		{"smallest key", 0, 0, 0, 0, 10, true, true, false, true},
		{"largest key", 90, 90, 90, 80, 90, true, true, true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k, _, ok := sl.Floor(tc.key)
			assertNav(t, "Floor", tc.key, k, ok, tc.floor, tc.floorOK)
			k, _, ok = sl.Ceiling(tc.key)
			assertNav(t, "Ceiling", tc.key, k, ok, tc.ceiling, tc.ceilOK)
			k, _, ok = sl.Lower(tc.key)
			assertNav(t, "Lower", tc.key, k, ok, tc.lower, tc.lowerOK)
			k, _, ok = sl.Higher(tc.key)
			assertNav(t, "Higher", tc.key, k, ok, tc.higher, tc.higherOK)
		})
	}
}

func TestRange(t *testing.T) {
	sl := skiplist.New[int, int]()
	for k := range 10 {
		sl.Put(k*10, k)
	}

	collect := func(seq iter.Seq2[int, int]) []int {
		var out []int
		for k := range seq {
			out = append(out, k)
		}
		return out
	}

	require.Equal(t, []int{30, 40, 50, 60, 70}, collect(sl.Range(30, 70, true, true)))
	require.Equal(t, []int{40, 50, 60}, collect(sl.Range(30, 70, false, false)))
	require.Equal(t, []int{0}, collect(sl.Range(-5, 5, true, true)))
	require.Empty(t, collect(sl.Range(70, 30, true, true)), "inverted range must be empty")

	count := 0
	for range sl.Range(0, 90, true, true) {
		count++
		break
	}
	require.Equal(t, 1, count, "break must stop the iteration")
}

func TestAll(t *testing.T) {
	sl := skiplist.New[string, int]()
	for _, w := range []string{"mango", "apple", "kiwi", "banana"} {
		sl.Put(w, len(w))
	}

	var got []string
	for k, v := range sl.All() {
		require.Equal(t, len(k), v)
		got = append(got, k)
	}
	require.Equal(t, []string{"apple", "banana", "kiwi", "mango"}, got)
}

func TestCustomComparator(t *testing.T) {
	rev := skiplist.NewFunc[string, int](func(a, b string) int { return strings.Compare(b, a) })
	for i, w := range []string{"a", "b", "c"} {
		rev.Put(w, i)
	}

	k, _, ok := rev.First()
	require.True(t, ok)
	require.Equal(t, "c", k, "reversed comparator must flip First")
	k, _, ok = rev.Last()
	require.True(t, ok)
	require.Equal(t, "a", k, "reversed comparator must flip Last")
	require.Equal(t, []string{"c", "b", "a"}, rev.Keys())
}

func TestClear(t *testing.T) {
	sl := skiplist.New[int, int]()
	for k := range 100 {
		sl.Put(k, k)
	}

	sl.Clear()
	require.Zero(t, sl.Len())
	require.False(t, sl.ContainsKey(50))
	require.Empty(t, sl.Keys())

	sl.Put(7, 7)
	require.Equal(t, 7, mustGet(t, sl, 7), "skiplist must stay usable after Clear")
}

func TestNewFuncPanicsOnNilComparator(t *testing.T) {
	require.Panics(t, func() { skiplist.NewFunc[int, int](nil) })
}
