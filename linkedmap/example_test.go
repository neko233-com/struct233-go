package linkedmap_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/linkedmap"
)

func ExampleLinkedHashMap() {
	m := linkedmap.New[string, int]()
	m.Put("b", 2)
	m.Put("a", 1)
	m.Put("c", 3)
	fmt.Println(m.Keys())
	fmt.Println(m.Values())
	// Output:
	// [b a c]
	// [2 1 3]
}

func ExampleLinkedHashMap_Put() {
	m := linkedmap.New[string, int]()
	m.Put("a", 1)
	m.Put("b", 2)
	// updating keeps the original position
	m.Put("a", 10)
	fmt.Println(m.Keys(), m.Values())
	old, existed := m.Put("a", 11)
	fmt.Println(old, existed)
	// Output:
	// [a b] [10 2]
	// 10 true
}

func ExampleLinkedHashMap_Remove() {
	m := linkedmap.New[string, int]()
	m.Put("a", 1)
	m.Put("b", 2)
	m.Put("c", 3)
	m.Remove("b")
	// re-adding moves the key to the end
	m.Put("b", 20)
	fmt.Println(m.Keys())
	// Output:
	// [a c b]
}

func ExampleLinkedHashMap_First() {
	m := linkedmap.New[string, int]()
	m.Put("first", 1)
	m.Put("last", 2)
	k, v, ok := m.First()
	fmt.Println(k, v, ok)
	_, _, ok = linkedmap.New[string, int]().First()
	fmt.Println(ok)
	// Output:
	// first 1 true
	// false
}

func ExampleLinkedHashMap_All() {
	m := linkedmap.New[int, string]()
	m.Put(2, "b")
	m.Put(1, "a")
	for k, v := range m.All() {
		fmt.Println(k, v)
	}
	// Output:
	// 2 b
	// 1 a
}
