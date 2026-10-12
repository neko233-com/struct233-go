package pqueue_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/pqueue"
)

func ExamplePriorityQueue() {
	q := pqueue.New[int]() // min-heap: the smallest element pops first
	q.Push(3)
	q.Push(1)
	q.Push(2)

	fmt.Println(q.Len())
	v, _ := q.Peek()
	fmt.Println(v) // Peek does not remove

	for q.Len() > 0 {
		v, _ := q.Pop()
		fmt.Println(v)
	}
	// Output:
	// 3
	// 1
	// 1
	// 2
	// 3
}

func ExamplePriorityQueue_maxQueue() {
	// a reversed comparator turns the queue into a max-queue
	q := pqueue.NewFunc(func(a, b int) int { return b - a })
	q.Push(5)
	q.Push(9)
	q.Push(1)

	for q.Len() > 0 {
		v, _ := q.Pop()
		fmt.Println(v)
	}
	// Output:
	// 9
	// 5
	// 1
}

func ExampleNewFromSlice() {
	q := pqueue.NewFromSlice([]int{9, 4, 7, 1, 8}, func(a, b int) int { return a - b })
	fmt.Println(q.Len())

	v, ok := q.Pop()
	fmt.Println(v, ok)
	v, ok = q.Peek()
	fmt.Println(v, ok)
	// Output:
	// 5
	// 1 true
	// 4 true
}
