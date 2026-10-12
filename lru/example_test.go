package lru_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/lru"
)

func ExampleLRU() {
	c := lru.New[string, int](2) // keep at most two entries
	c.Put("a", 1)
	c.Put("b", 2)
	fmt.Println(c.Len(), c.Keys()) // LRU -> MRU

	// inserting a third key evicts the least recently used one
	evictedKey, evictedValue, evicted := c.Put("c", 3)
	fmt.Println(evicted, evictedKey, evictedValue)
	fmt.Println(c.Keys())
	// Output:
	// 2 [a b]
	// true a 1
	// [b c]
}

func ExampleLRU_Get() {
	c := lru.New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)

	v, ok := c.Get("a") // Get promotes a to most-recently-used
	fmt.Println(v, ok)

	c.Put("c", 3) // evicts b, not a
	fmt.Println(c.Contains("a"), c.Contains("b"))

	_, ok = c.Get("missing")
	fmt.Println(ok)
	// Output:
	// 1 true
	// true false
	// false
}

func ExampleLRU_Keys() {
	c := lru.New[int, string](3)
	c.Put(1, "x")
	c.Put(2, "y")
	c.Put(3, "z")

	c.Get(1) // promote 1
	fmt.Println(c.Keys())

	for k, v := range c.All() { // same order: LRU -> MRU
		fmt.Println(k, v)
	}
	// Output:
	// [2 3 1]
	// 2 y
	// 3 z
	// 1 x
}
