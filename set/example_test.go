package set_test

import (
	"fmt"
	"slices"

	"github.com/neko233-com/struct233-go/set"
)

func sorted(s *set.HashSet[int]) []int {
	items := s.ToSlice()
	slices.Sort(items)
	return items
}

func ExampleHashSet() {
	s := set.NewHashSet(3, 1, 2, 1)
	fmt.Println(s.Len())
	fmt.Println(s.Contains(2), s.Contains(9))
	s.Add(4)
	s.Remove(1)
	fmt.Println(sorted(s))
	// Output:
	// 3
	// true false
	// [2 3 4]
}

func ExampleHashSet_algebra() {
	a := set.NewHashSet(1, 2, 3)
	b := set.NewHashSet(2, 3, 4)
	fmt.Println(sorted(a.Intersection(b)))
	fmt.Println(sorted(a.Union(b)))
	fmt.Println(sorted(a.Difference(b)))
	fmt.Println(sorted(a.SymmetricDifference(b)))
	// Output:
	// [2 3]
	// [1 2 3 4]
	// [1]
	// [1 4]
}

func ExampleLinkedHashSet() {
	s := set.NewLinkedHashSet(3, 1, 2)
	fmt.Println(s.ToSlice())
	s.Add(1) // re-adding keeps the position
	fmt.Println(s.ToSlice())
	s.Remove(3)
	s.Add(3) // re-adding after remove moves to the end
	fmt.Println(s.ToSlice())
	// Output:
	// [3 1 2]
	// [3 1 2]
	// [1 2 3]
}

func ExampleSkipListSet() {
	ts := set.NewSkipListSet(5, 1, 9, 3, 7)
	fmt.Println(ts.ToSlice())
	first, _ := ts.First()
	last, _ := ts.Last()
	fmt.Println(first, last)
	ceil, _ := ts.Ceiling(4)
	floor, _ := ts.Floor(4)
	fmt.Println(ceil, floor)
	// Output:
	// [1 3 5 7 9]
	// 1 9
	// 5 3
}

func ExampleSkipListSet_Range() {
	ts := set.NewSkipListSet(10, 20, 30, 40, 50)
	for v := range ts.Range(20, 40, true, false) {
		fmt.Println(v)
	}
	// Output:
	// 20
	// 30
}
