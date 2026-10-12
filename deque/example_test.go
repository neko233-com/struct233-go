package deque_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/deque"
)

func ExampleArrayDeque() {
	d := deque.New[int]()
	d.PushBack(1) // queue from the back...
	d.PushBack(2)
	d.PushFront(0) // ...and from the front at once

	fmt.Println(d.Len(), d.ToSlice())

	v, _ := d.PopFront() // FIFO when paired with PushBack
	fmt.Println(v)
	v, _ = d.PopBack() // LIFO when paired with PushBack
	fmt.Println(v)
	// Output:
	// 3 [0 1 2]
	// 0
	// 2
}

func ExampleArrayDeque_All() {
	d := deque.New[string]()
	d.PushFront("b")
	d.PushFront("a")
	d.PushBack("c")

	for v := range d.All() {
		fmt.Println(v)
	}
	// Output:
	// a
	// b
	// c
}

func ExampleArrayDeque_Clear() {
	d := deque.New[int](16)
	d.PushBack(1)
	d.PushBack(2)
	fmt.Println(d.Len(), d.Cap())

	d.Clear() // keeps the allocated capacity
	fmt.Println(d.Len(), d.Cap())
	// Output:
	// 2 16
	// 0 16
}
