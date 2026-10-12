package ringbuffer_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/ringbuffer"
)

func ExampleRingBuffer() {
	r := ringbuffer.New[int](3)
	for _, v := range []int{1, 2, 3} {
		r.Push(v)
	}
	evicted, ok := r.Push(4) // full: the oldest is overwritten
	fmt.Println(evicted, ok)
	fmt.Println(r.ToSlice())
	// Output:
	// 1 true
	// [2 3 4]
}

func ExampleRingBuffer_Pop() {
	r := ringbuffer.New[string](2)
	r.Push("a")
	r.Push("b")
	v, ok := r.Peek()
	fmt.Println(v, ok)
	v, ok = r.Pop()
	fmt.Println(v, ok)
	fmt.Println(r.ToSlice())
	// Output:
	// a true
	// a true
	// [b]
}

func ExampleRingBuffer_At() {
	r := ringbuffer.New[int](3)
	for i := 0; i < 4; i++ {
		r.Push(i * 10) // 0 evicted once the buffer fills
	}
	for i := range r.Len() {
		v, _ := r.At(i)
		fmt.Println(v)
	}
	// Output:
	// 10
	// 20
	// 30
}
