package atomiclongmap_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/atomiclongmap"
)

func ExampleAtomicLongMap() {
	m := atomiclongmap.New[string]()

	fmt.Println(m.Increment("hits"))
	fmt.Println(m.Add("hits", 9))
	fmt.Println(m.Decrement("hits"))
	fmt.Println(m.Get("hits"))

	m.Put("errors", 0)
	fmt.Println(m.RemoveIfZero("errors"))

	m.Put("errors", 2)
	fmt.Println(m.Remove("errors"), m.Len())

	m.Put("a", 1)
	m.Put("b", 2)
	fmt.Println(m.Sum())
	fmt.Println(m.AsMap())

	m.Clear()
	fmt.Println(m.Len())

	// Output:
	// 1
	// 10
	// 9
	// 9
	// true
	// 2 1
	// 12
	// map[a:1 b:2 hits:9]
	// 0
}

func ExampleAtomicLongMap_Range() {
	m := atomiclongmap.New[string]()
	m.Put("hits", 3)
	m.Put("misses", 1)

	sum := int64(0)
	m.Range(func(key string, value int64) bool {
		sum += value
		return true
	})
	fmt.Println(sum)

	// Stop early after the first pair.
	m.Range(func(key string, value int64) bool {
		fmt.Println("first:", key)
		return false
	})

	// Output:
	// 4
	// first: hits
}
