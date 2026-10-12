package rangeset_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/rangeset"
)

func ExampleRange() {
	r := rangeset.ClosedOpen(1, 5)
	fmt.Println(r, r.Contains(1), r.Contains(5), r.IsEmpty())
	// Output:
	// [1,5) true false false
}

func ExampleRange_IsConnected() {
	fmt.Println(rangeset.Closed(1, 4).IsConnected(rangeset.Closed(4, 7)))
	fmt.Println(rangeset.Open(1, 4).IsConnected(rangeset.Open(4, 7)))
	// Output:
	// true
	// false
}

func ExampleRangeSet() {
	s := rangeset.NewSet[int]()
	s.Add(rangeset.Closed(1, 3))
	s.Add(rangeset.Closed(3, 6)) // closed-touching spans coalesce
	fmt.Println(s.Spans())
	s.Remove(rangeset.Singleton(2))
	fmt.Println(s.Spans())
	fmt.Println(s.Contains(5), s.Contains(2))
	// Output:
	// [[1,6]]
	// [[1,2) (2,6]]
	// true false
}

func ExampleRangeSet_Complement() {
	s := rangeset.NewSet[int]()
	s.Add(rangeset.Closed(1, 3))
	fmt.Println(s.Complement().Spans())
	fmt.Println(rangeset.All[int]().Encloses(rangeset.Closed(1, 3)))
	// Output:
	// [(-inf,1) (3,+inf)]
	// true
}

func ExampleRangeSet_algebra() {
	a := rangeset.NewSet[int]()
	a.Add(rangeset.Closed(1, 5))
	b := rangeset.NewSet[int]()
	b.Add(rangeset.Closed(3, 7))
	fmt.Println(a.Intersection(b).Spans())
	fmt.Println(a.Difference(b).Spans())
	fmt.Println(a.Union(b).Spans())
	// Output:
	// [[3,5]]
	// [[1,3)]
	// [[1,7]]
}
