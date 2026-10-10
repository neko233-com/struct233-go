package set_test

import (
	"slices"
	"testing"

	"github.com/neko233-com/go-data-struct-lib/set"
)

func TestHashSetBasics(t *testing.T) {
	s := set.NewHashSet(1, 2, 3)
	if s.Len() != 3 {
		t.Fatalf("Len = %d; want 3", s.Len())
	}
	if s.Add(3) {
		t.Fatal("re-adding an existing element should report false")
	}
	if !s.Add(4) {
		t.Fatal("adding a new element should report true")
	}
	if !s.ContainsAll(1, 2, 3, 4) {
		t.Fatal("ContainsAll mismatch")
	}
	if s.Contains(5) {
		t.Fatal("Contains(5) should be false")
	}
	if !s.Remove(4) || s.Remove(4) {
		t.Fatal("Remove should report true exactly once")
	}
	s.Clear()
	if s.Len() != 0 || s.Contains(1) {
		t.Fatal("set should be empty after Clear")
	}
}

func TestHashSetAlgebra(t *testing.T) {
	a := set.NewHashSet(1, 2, 3, 4)
	b := set.NewHashSet(3, 4, 5)

	if got := a.Union(b); !got.Equal(set.NewHashSet(1, 2, 3, 4, 5)) {
		t.Fatalf("Union = %v", got.ToSlice())
	}
	if got := a.Intersection(b); !got.Equal(set.NewHashSet(3, 4)) {
		t.Fatalf("Intersection = %v", got.ToSlice())
	}
	if got := a.Difference(b); !got.Equal(set.NewHashSet(1, 2)) {
		t.Fatalf("Difference = %v", got.ToSlice())
	}
	if got := a.SymmetricDifference(b); !got.Equal(set.NewHashSet(1, 2, 5)) {
		t.Fatalf("SymmetricDifference = %v", got.ToSlice())
	}
	if !a.Intersection(b).IsSubsetOf(a) || !a.IsSupersetOf(a.Intersection(b)) {
		t.Fatal("subset/superset check failed")
	}
	if a.IsSubsetOf(b) {
		t.Fatal("a is not a subset of b")
	}
	// algebra ops must not mutate receivers
	if a.Len() != 4 || b.Len() != 3 {
		t.Fatal("algebra operations mutated their receivers")
	}
}

func TestHashSetClone(t *testing.T) {
	a := set.NewHashSet("x", "y")
	c := a.Clone()
	c.Add("z")
	if a.Contains("z") {
		t.Fatal("Clone should be independent of the original")
	}
}

func TestTreeSetOrdered(t *testing.T) {
	ts := set.NewTreeSet(5, 1, 9, 3, 7)
	if got := ts.ToSlice(); !slices.Equal(got, []int{1, 3, 5, 7, 9}) {
		t.Fatalf("ToSlice = %v; want ascending", got)
	}
	var iterated []int
	for k := range ts.All() {
		iterated = append(iterated, k)
	}
	if !slices.Equal(iterated, []int{1, 3, 5, 7, 9}) {
		t.Fatalf("All() = %v", iterated)
	}
	if k, ok := ts.First(); !ok || k != 1 {
		t.Fatalf("First = %d, %v", k, ok)
	}
	if k, ok := ts.Last(); !ok || k != 9 {
		t.Fatalf("Last = %d, %v", k, ok)
	}
}

func TestTreeSetNavigable(t *testing.T) {
	ts := set.NewTreeSet(10, 20, 30, 40, 50)

	if k, ok := ts.Ceiling(35); !ok || k != 40 {
		t.Fatalf("Ceiling(35) = %d, %v", k, ok)
	}
	if k, ok := ts.Lower(30); !ok || k != 20 {
		t.Fatalf("Lower(30) = %d, %v", k, ok)
	}
	if k, ok := ts.Higher(30); !ok || k != 40 {
		t.Fatalf("Higher(30) = %d, %v", k, ok)
	}
	if k, ok := ts.Floor(35); !ok || k != 30 {
		t.Fatalf("Floor(35) = %d, %v", k, ok)
	}
	if _, ok := ts.Lower(10); ok {
		t.Fatal("Lower(10) should miss")
	}
	if _, ok := ts.Higher(50); ok {
		t.Fatal("Higher(50) should miss")
	}
	if _, ok := ts.Ceiling(51); ok {
		t.Fatal("Ceiling(51) should miss")
	}
}

func TestTreeSetRange(t *testing.T) {
	ts := set.NewTreeSet(10, 20, 30, 40, 50)
	var got []int
	for k := range ts.Range(20, 40, false, false) {
		got = append(got, k)
	}
	if !slices.Equal(got, []int{30}) {
		t.Fatalf("exclusive Range = %v; want [30]", got)
	}
	got = nil
	for k := range ts.Range(15, 45, true, true) {
		got = append(got, k)
	}
	if !slices.Equal(got, []int{20, 30, 40}) {
		t.Fatalf("inclusive Range = %v; want [20 30 40]", got)
	}
}

func TestTreeSetCustomComparator(t *testing.T) {
	// descending order
	ts := set.NewTreeSetFunc(func(a, b int) int { return b - a }, 3, 1, 2)
	if got := ts.ToSlice(); !slices.Equal(got, []int{3, 2, 1}) {
		t.Fatalf("ToSlice = %v; want descending", got)
	}
	if k, ok := ts.First(); !ok || k != 3 {
		t.Fatalf("First = %d, %v; want 3", k, ok)
	}
}

func TestTreeSetAlgebra(t *testing.T) {
	a := set.NewTreeSet(1, 2, 3)
	b := set.NewTreeSet(2, 3, 4)

	if got := a.Union(b); !got.Equal(set.NewTreeSet(1, 2, 3, 4)) {
		t.Fatalf("Union = %v", got.ToSlice())
	}
	if got := a.Intersection(b); !got.Equal(set.NewTreeSet(2, 3)) {
		t.Fatalf("Intersection = %v", got.ToSlice())
	}
	if got := a.Difference(b); !got.Equal(set.NewTreeSet(1)) {
		t.Fatalf("Difference = %v", got.ToSlice())
	}
	c := a.Clone()
	c.Add(9)
	if a.Contains(9) {
		t.Fatal("Clone should be independent")
	}
}

func TestLinkedHashSetOrder(t *testing.T) {
	s := set.NewLinkedHashSet("a", "b", "c")
	if got := s.ToSlice(); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Fatalf("initial order = %v", got)
	}
	// re-add existing: order unchanged (JDK LinkedHashSet semantics)
	s.Add("a")
	if got := s.ToSlice(); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Fatalf("re-add changed order: %v", got)
	}
	// remove middle, then re-add: goes to the end
	s.Remove("b")
	s.Add("b")
	if got := s.ToSlice(); !slices.Equal(got, []string{"a", "c", "b"}) {
		t.Fatalf("order after remove+re-add = %v", got)
	}
	// iteration matches ToSlice
	var iterated []string
	for k := range s.All() {
		iterated = append(iterated, k)
	}
	if !slices.Equal(iterated, s.ToSlice()) {
		t.Fatalf("All() = %v, ToSlice = %v", iterated, s.ToSlice())
	}
	s.Clear()
	if s.Len() != 0 || !s.Equal(set.NewLinkedHashSet[string]()) {
		t.Fatal("set should be empty after Clear")
	}
}

func TestLinkedHashSetClone(t *testing.T) {
	s := set.NewLinkedHashSet(1, 2, 3)
	c := s.Clone()
	c.Remove(1)
	if !s.Contains(1) {
		t.Fatal("Clone should be independent of the original")
	}
	if !c.Equal(set.NewLinkedHashSet(2, 3)) {
		t.Fatalf("clone contents = %v", c.ToSlice())
	}
}
