package order_test

import (
	"cmp"
	"slices"
	"testing"

	"github.com/neko233-com/struct233-go/order"
)

// FuzzSortedIsAPermutationAndOrdered checks the promises of Sorted for any input
// and either direction: it returns a permutation of the input, ordered by the
// comparator, without touching the input, and MaxOf/MinOf agree with its ends.
func FuzzSortedIsAPermutationAndOrdered(f *testing.F) {
	f.Add([]byte{3, 1, 4, 1, 5}, false)
	f.Add([]byte{}, true)
	f.Add([]byte{7}, false)

	f.Fuzz(func(t *testing.T, raw []byte, descending bool) {
		items := make([]int, len(raw))
		for i, b := range raw {
			items[i] = int(b) - 128
		}

		cmp := order.Natural[int]()
		if descending {
			cmp = order.Reverse(cmp)
		}

		sorted := order.Sorted(cmp, items)
		if len(sorted) != len(items) {
			t.Fatalf("Sorted changed the length: %d then %d", len(items), len(sorted))
		}
		if !order.IsSorted(cmp, sorted) {
			t.Fatalf("Sorted(cmp, %v) = %v is not ordered", items, sorted)
		}

		want := slices.Clone(items)
		slices.SortFunc(want, cmp)
		if !slices.Equal(sorted, want) {
			t.Fatalf("Sorted(cmp, %v) = %v; want %v", items, sorted, want)
		}

		if len(sorted) > 0 {
			// "sorted by cmp" means the last element is the greatest under cmp and
			// the first is the least, whichever direction cmp orders in
			wantMax, wantMin := sorted[len(sorted)-1], sorted[0]
			if max, ok := order.MaxOf(cmp, sorted); !ok || max != wantMax {
				t.Fatalf("MaxOf(%v) = %d, %v; want %d", sorted, max, ok, wantMax)
			}
			if min, ok := order.MinOf(cmp, sorted); !ok || min != wantMin {
				t.Fatalf("MinOf(%v) = %d, %v; want %d", sorted, min, ok, wantMin)
			}
		} else if _, ok := order.MaxOf(cmp, sorted); ok {
			t.Fatal("MaxOf must report false for an empty slice")
		}

		if c := cmp(0, 0); c != 0 {
			t.Fatalf("a comparator must report equality for identical operands, got %d", c)
		}
		if sign, mirrored := cmp(1, 2), cmp(2, 1); sign != -mirrored {
			t.Fatalf("a comparator must be antisymmetric: %d vs %d", sign, -mirrored)
		}
	})
}

// FuzzThenIsLexicographic checks that Then decides with the first non-zero
// comparator and reports equality only when every comparator does.
func FuzzThenIsLexicographic(f *testing.F) {
	f.Add(3, 5)
	f.Add(4, 4)

	f.Fuzz(func(t *testing.T, a, b int) {
		byMod2 := order.ByKey(func(v int) int { return v % 2 }, order.Natural[int]())
		cmp2 := order.Then(byMod2, order.Natural[int]())

		got := cmp2(a, b)
		switch {
		case a%2 != b%2:
			if want := cmp.Compare(a%2, b%2); got != want {
				t.Fatalf("Then(%d, %d) = %d; want the first comparator's %d", a, b, got, want)
			}
		case a == b:
			if got != 0 {
				t.Fatalf("Then(%d, %d) = %d; want 0", a, b, got)
			}
		default:
			if want := cmp.Compare(a, b); got != want {
				t.Fatalf("Then(%d, %d) = %d; want the second comparator's %d", a, b, got, want)
			}
		}
	})
}

// FuzzByKeyMirrorsTheProjection checks that a key comparator answers exactly
// what the key comparator answers on the projected keys.
func FuzzByKeyMirrorsTheProjection(f *testing.F) {
	f.Add("abc", "abd")
	f.Add("", "x")

	f.Fuzz(func(t *testing.T, a, b string) {
		byLen := order.ByKey(func(s string) int { return len(s) }, order.Natural[int]())
		byText := order.ByKey(func(s string) string { return s }, order.Natural[string]())

		if got, want := byLen(a, b), cmp.Compare(len(a), len(b)); got != want {
			t.Fatalf("byLen(%q, %q) = %d; want %d", a, b, got, want)
		}
		if got, want := byText(a, b), cmp.Compare(a, b); got != want {
			t.Fatalf("byText(%q, %q) = %d; want %d", a, b, got, want)
		}
	})
}
