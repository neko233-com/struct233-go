package multimap_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/multimap"
)

func ExampleListMultimap() {
	m := multimap.NewList[string, int]()
	m.Put("odd", 1)
	m.PutAll("odd", 3, 5)
	m.Put("even", 2)
	m.Put("even", 4)

	fmt.Println(m.Keys())
	fmt.Println(m.Get("odd"))
	fmt.Println(m.Size(), m.KeyCount(), m.IsEmpty())
	fmt.Println(m.ContainsEntry("odd", 5), m.ContainsEntry("even", 5))
	// Output:
	// [odd even]
	// [1 3 5]
	// 5 2 false
	// true false
}

func ExampleListMultimap_All() {
	m := multimap.NewList[string, int]()
	m.PutAll("a", 1, 2)
	m.Put("b", 3)

	for k, v := range m.All() {
		fmt.Printf("%s=%d\n", k, v)
	}
	// Output:
	// a=1
	// a=2
	// b=3
}

func ExampleListMultimap_Invert() {
	m := multimap.NewList[string, string]()
	m.PutAll("fruit", "apple", "banana")
	m.PutAll("colour", "banana")

	inverted := m.Invert()
	fmt.Println(inverted.Keys())
	fmt.Println(inverted.Get("banana"))
	// Output:
	// [apple banana]
	// [fruit colour]
}

func ExampleListMultimap_Replace() {
	m := multimap.NewList[string, int]()
	m.PutAll("a", 1, 2)
	m.Put("b", 3)

	// Replace hands back the values it dropped, detached from the multimap.
	fmt.Println(m.Replace("a", 9))
	fmt.Println(m.Entries())

	// Removing the last value of a key drops the key, and a later Put appends it
	// at the end of the key order.
	m.Replace("a")
	m.Put("a", 4)
	fmt.Println(m.Keys())
	// Output:
	// [1 2]
	// [{a 9} {b 3}]
	// [b a]
}

func ExampleSetMultimap() {
	m := multimap.NewSet[string, int]()
	fmt.Println(m.Put("a", 1))
	fmt.Println(m.Put("a", 1), "duplicate is ignored")
	fmt.Println(m.PutAll("a", 2, 3, 3), "values added")
	fmt.Println(m.Get("a"))
	fmt.Println(m.Remove("a", 2), m.Remove("a", 2))
	fmt.Println(m.AsMap())
	// Output:
	// true
	// false duplicate is ignored
	// 2 values added
	// [1 2 3]
	// true false
	// map[a:[1 3]]
}
