package rangemap_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/rangemap"
	"github.com/neko233-com/struct233-go/rangeset"
)

func ExampleRangeMap() {
	m := rangemap.New[int, string]()
	m.Put(rangeset.Closed(1, 10), "old")
	m.Put(rangeset.ClosedOpen(4, 7), "new")

	fmt.Println(m.Entries())
	v2, ok2 := m.Get(2)
	v5, ok5 := m.Get(5)
	fmt.Println(v2, ok2, v5, ok5)
	// Output:
	// [{[1,4) old} {[4,7) new} {[7,10] old}]
	// old true new true
}

func ExampleRangeMap_Remove() {
	m := rangemap.New[int, string]()
	m.Put(rangeset.Closed(0, 20), "x")
	m.Remove(rangeset.Closed(5, 15)) // the outer portions survive
	fmt.Println(m.Entries())
	fmt.Println(m.Span())
	// Output:
	// [{[0,5) x} {(15,20] x}]
	// [0,20] true
}

func ExampleRangeMap_SubRangeMap() {
	m := rangemap.New[int, string]()
	m.Put(rangeset.Closed(0, 10), "a")
	sub := m.SubRangeMap(rangeset.Closed(3, 7))
	fmt.Println(sub.Entries())

	sub.Put(rangeset.All[int](), "detached")
	fmt.Println(m.Get(5))
	// Output:
	// [{[3,7] a}]
	// a true
}

func ExampleRangeMap_unbounded() {
	m := rangemap.New[int, string]()
	m.Put(rangeset.All[int](), "all")
	m.Put(rangeset.OpenClosed(-2, 2), "middle")
	fmt.Println(m.Entries())
	v1, ok1 := m.Get(-3)
	v2, ok2 := m.Get(0)
	fmt.Println(v1, ok1, v2, ok2)
	// Output:
	// [{(-inf,-2] all} {(-2,2] middle} {(2,+inf) all}]
	// all true middle true
}
