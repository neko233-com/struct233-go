package skiplist_test

import (
	"iter"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/neko233-com/go-data-struct-lib/skiplist"
)

func TestPutGet(t *testing.T) {
	sl := skiplist.New[int, string]()
	if _, ok := sl.Get(1); ok {
		t.Fatal("Get on empty skiplist should miss")
	}
	if old, existed := sl.Put(1, "a"); existed {
		t.Fatalf("first Put should report no old value, got %q", old)
	}
	if v, ok := sl.Get(1); !ok || v != "a" {
		t.Fatalf("Get(1) = %q, %v; want \"a\", true", v, ok)
	}
	if old, existed := sl.Put(1, "b"); !existed || old != "a" {
		t.Fatalf("overwriting Put: old=%q existed=%v; want \"a\", true", old, existed)
	}
	if v, ok := sl.Get(1); !ok || v != "b" {
		t.Fatalf("Get(1) after overwrite = %q, %v", v, ok)
	}
	if sl.Len() != 1 {
		t.Fatalf("Len() = %d after overwrite; want 1", sl.Len())
	}
	if !sl.ContainsKey(1) || sl.ContainsKey(2) {
		t.Fatal("ContainsKey mismatch")
	}
}

func TestRemove(t *testing.T) {
	sl := skiplist.New[int, int]()
	for k := range 10 {
		sl.Put(k, k*10)
	}
	if old, removed := sl.Remove(3); !removed || old != 30 {
		t.Fatalf("Remove(3) = %d, %v; want 30, true", old, removed)
	}
	if _, removed := sl.Remove(3); removed {
		t.Fatal("removing an absent key should report false")
	}
	if sl.Len() != 9 {
		t.Fatalf("Len() = %d; want 9", sl.Len())
	}
	// The chain must stay traversable and ordered after deletion.
	want := []int{0, 1, 2, 4, 5, 6, 7, 8, 9}
	if got := sl.Keys(); !slices.Equal(got, want) {
		t.Fatalf("keys after remove = %v; want %v", got, want)
	}
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

	if sl.Len() != len(ref) {
		t.Fatalf("Len() = %d; want %d", sl.Len(), len(ref))
	}
	wantKeys := make([]int, 0, len(ref))
	for k := range ref {
		wantKeys = append(wantKeys, k)
	}
	slices.Sort(wantKeys)
	if got := sl.Keys(); !slices.Equal(got, wantKeys) {
		t.Fatalf("keys diverge from reference (len %d vs %d)", len(got), len(wantKeys))
	}
	for k, want := range ref {
		if v, ok := sl.Get(k); !ok || v != want {
			t.Fatalf("Get(%d) = %d, %v; want %d", k, v, ok, want)
		}
	}
}

func TestFirstLast(t *testing.T) {
	sl := skiplist.New[int, string]()
	if _, _, ok := sl.First(); ok {
		t.Fatal("First on empty skiplist should miss")
	}
	if _, _, ok := sl.Last(); ok {
		t.Fatal("Last on empty skiplist should miss")
	}
	sl.Put(5, "e")
	sl.Put(1, "a")
	sl.Put(9, "i")
	if k, v, ok := sl.First(); !ok || k != 1 || v != "a" {
		t.Fatalf("First() = %d, %q, %v; want 1, \"a\", true", k, v, ok)
	}
	if k, v, ok := sl.Last(); !ok || k != 9 || v != "i" {
		t.Fatalf("Last() = %d, %q, %v; want 9, \"i\", true", k, v, ok)
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
		floor, ceiling                     int
		lower, higher                      int
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
			if k, _, ok := sl.Floor(tc.key); ok != tc.floorOK || (ok && k != tc.floor) {
				t.Fatalf("Floor(%d) = %d, %v; want %d, %v", tc.key, k, ok, tc.floor, tc.floorOK)
			}
			if k, _, ok := sl.Ceiling(tc.key); ok != tc.ceilOK || (ok && k != tc.ceiling) {
				t.Fatalf("Ceiling(%d) = %d, %v; want %d, %v", tc.key, k, ok, tc.ceiling, tc.ceilOK)
			}
			if k, _, ok := sl.Lower(tc.key); ok != tc.lowerOK || (ok && k != tc.lower) {
				t.Fatalf("Lower(%d) = %d, %v; want %d, %v", tc.key, k, ok, tc.lower, tc.lowerOK)
			}
			if k, _, ok := sl.Higher(tc.key); ok != tc.higherOK || (ok && k != tc.higher) {
				t.Fatalf("Higher(%d) = %d, %v; want %d, %v", tc.key, k, ok, tc.higher, tc.higherOK)
			}
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

	if got := collect(sl.Range(30, 70, true, true)); !slices.Equal(got, []int{30, 40, 50, 60, 70}) {
		t.Fatalf("inclusive Range = %v", got)
	}
	if got := collect(sl.Range(30, 70, false, false)); !slices.Equal(got, []int{40, 50, 60}) {
		t.Fatalf("exclusive Range = %v", got)
	}
	if got := collect(sl.Range(-5, 5, true, true)); !slices.Equal(got, []int{0}) {
		t.Fatalf("clamped-low Range = %v", got)
	}
	if got := collect(sl.Range(70, 30, true, true)); len(got) != 0 {
		t.Fatalf("inverted Range should be empty, got %v", got)
	}
	// early termination: stop after first yield
	count := 0
	for range sl.Range(0, 90, true, true) {
		count++
		break
	}
	if count != 1 {
		t.Fatalf("break inside range visited %d entries; want 1", count)
	}
}

func TestAll(t *testing.T) {
	sl := skiplist.New[string, int]()
	for _, w := range []string{"mango", "apple", "kiwi", "banana"} {
		sl.Put(w, len(w))
	}
	var got []string
	for k, v := range sl.All() {
		if v != len(k) {
			t.Fatalf("value mismatch for %q", k)
		}
		got = append(got, k)
	}
	if !slices.Equal(got, []string{"apple", "banana", "kiwi", "mango"}) {
		t.Fatalf("All() = %v; want ascending order", got)
	}
}

func TestCustomComparator(t *testing.T) {
	// reverse lexicographic order
	rev := skiplist.NewFunc[string, int](func(a, b string) int { return strings.Compare(b, a) })
	for i, w := range []string{"a", "b", "c"} {
		rev.Put(w, i)
	}
	if k, _, ok := rev.First(); !ok || k != "c" {
		t.Fatalf("First() with reversed comparator = %q, %v; want \"c\"", k, ok)
	}
	if k, _, ok := rev.Last(); !ok || k != "a" {
		t.Fatalf("Last() with reversed comparator = %q, %v; want \"a\"", k, ok)
	}
	if got := rev.Keys(); !slices.Equal(got, []string{"c", "b", "a"}) {
		t.Fatalf("Keys() = %v", got)
	}
}

func TestClear(t *testing.T) {
	sl := skiplist.New[int, int]()
	for k := range 100 {
		sl.Put(k, k)
	}
	sl.Clear()
	if sl.Len() != 0 {
		t.Fatalf("Len() after Clear = %d", sl.Len())
	}
	if sl.ContainsKey(50) {
		t.Fatal("skiplist should be empty after Clear")
	}
	if got := sl.Keys(); len(got) != 0 {
		t.Fatalf("Keys() after Clear = %v", got)
	}
	sl.Put(7, 7)
	if v, ok := sl.Get(7); !ok || v != 7 {
		t.Fatal("skiplist unusable after Clear")
	}
}

func TestNewFuncPanicsOnNilComparator(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewFunc with nil comparator should panic")
		}
	}()
	skiplist.NewFunc[int, int](nil)
}
