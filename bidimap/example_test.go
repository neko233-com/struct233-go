package bidimap_test

import (
	"fmt"
	"slices"

	"github.com/neko233-com/struct233-go/bidimap"
)

func ExampleBiMap() {
	b := bidimap.New[string, int]()
	fmt.Println(b.Put("one", 1)) // true: value 1 is still free
	fmt.Println(b.Put("uno", 1)) // false: 1 is already bound to "one"
	b.Put("two", 2)

	v, ok := b.Get("two")
	fmt.Println(v, ok)
	k, ok := b.GetKeyOf(1)
	fmt.Println(k, ok)
	fmt.Println(b.Len())
	// Output:
	// true
	// false
	// 2 true
	// one true
	// 2
}

func ExampleBiMap_Inverse() {
	b := bidimap.New[string, int]()
	b.Put("one", 1)
	b.Put("two", 2)

	inv := b.Inverse() // live reverse view over the same storage
	v, _ := inv.Get(1)
	fmt.Println(v)
	inv.ForcePut(3, "three") // writes through the view...
	got, _ := b.Get("three") // ...and are visible on the origin
	fmt.Println(got)
	// Output:
	// one
	// 3
}

func ExampleBiMap_ForcePut() {
	b := bidimap.New[string, int]()
	b.Put("one", 1)
	b.ForcePut("two", 1) // steals value 1 from "one"

	v, ok := b.Get("one")
	fmt.Println(v, ok)
	v, ok = b.Get("two")
	fmt.Println(v, ok)
	// Output:
	// 0 false
	// 1 true
}

func ExampleBiMap_Entries() {
	b := bidimap.New[string, int]()
	b.Put("one", 1)
	b.Put("two", 2)

	pairs := []string{}
	for k, v := range b.Entries() {
		pairs = append(pairs, fmt.Sprintf("%s=%d", k, v))
	}
	slices.Sort(pairs) // Entries iterate in unspecified order
	fmt.Println(pairs)
	// Output:
	// [one=1 two=2]
}
