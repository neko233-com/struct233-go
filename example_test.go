package struct233_test

import (
	"fmt"
	"sort"

	struct233 "github.com/neko233-com/struct233-go"
)

func ExampleNewSet() {
	s := struct233.NewSet(1, 2, 3)
	s.Add(4)
	s.Remove(1)

	got := s.ToSlice()
	sort.Ints(got)
	fmt.Println(got)
	fmt.Println(s.Contains(2))
	// Output:
	// [2 3 4]
	// true
}

func ExampleNewSortedSet() {
	s := struct233.NewSortedSet(30, 10, 20) // asserts ascending order, not insertion
	fmt.Println(s.ToSlice())
	fmt.Println(s.Contains(20))

	floor, _ := s.Floor(25)
	ceiling, _ := s.Ceiling(25)
	fmt.Println(floor, ceiling)
	// Output:
	// [10 20 30]
	// true
	// 20 30
}

func ExampleNewSkipList() {
	sl := struct233.NewSkipList[int, string]()
	sl.Put(30, "c")
	sl.Put(10, "a")
	sl.Put(20, "b")

	if k, v, ok := sl.Floor(25); ok { // greatest key <= 25
		fmt.Println(k, v)
	}
	for k := range sl.Range(10, 20, true, true) {
		fmt.Print(k, " ")
	}
	// Output:
	// 20 b
	// 10 20
}

func ExampleNewLinkedMap() {
	m := struct233.NewLinkedMap[string, int]()
	for i, name := range []string{"gamma", "alpha", "beta"} {
		m.Put(name, i)
	}
	fmt.Println(m.Keys())
	fmt.Println(m.Values())
	// Output:
	// [gamma alpha beta]
	// [0 1 2]
}

func ExampleNewPriorityQueue() {
	q := struct233.NewPriorityQueue[int]()
	for _, v := range []int{5, 1, 3} {
		q.Push(v)
	}
	for {
		v, ok := q.Pop()
		if !ok {
			break
		}
		fmt.Print(v, " ")
	}
	// Output:
	// 1 3 5
}

func ExampleNewDeque() {
	d := struct233.NewDeque[int]()
	d.PushBack(2)
	d.PushFront(1)
	fmt.Println(d.ToSlice())
	fmt.Println(d.PopFront())
	fmt.Println(d.PopBack())
	// Output:
	// [1 2]
	// 1 true
	// 2 true
}

func ExampleNewRingBuffer() {
	recent := struct233.NewRingBuffer[int](3)
	for i := 1; i <= 5; i++ {
		if evicted, ok := recent.Push(i); ok {
			fmt.Println("evicted", evicted)
		}
	}
	fmt.Println(recent.ToSlice())
	// Output:
	// evicted 1
	// evicted 2
	// [3 4 5]
}

func ExampleNewBitSet() {
	b := struct233.NewBitSet()
	b.Set(64)
	b.Set(70)
	fmt.Println(b.Cardinality(), b.Len())
	fmt.Println(b.NextSetBit(0), b.NextSetBit(65))
	b.Flip(64)
	fmt.Println(b.Get(64))
	// Output:
	// 2 71
	// 64 70
	// false
}

func ExampleNewLRU() {
	cache := struct233.NewLRU[string, int](2)
	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Get("a") // promotes a; b becomes the eviction candidate

	evicted, _, ok := cache.Put("c", 3)
	fmt.Println(evicted, ok)
	fmt.Println(cache.Keys())
	// Output:
	// b true
	// [a c]
}

func ExampleNewConcurrentSkipList() {
	ranking := struct233.NewConcurrentSkipList[int, string]() // goroutine-safe
	ranking.Put(100, "p1")
	ranking.Put(250, "p2")

	top, name, _ := ranking.Last()
	fmt.Println(name, top)
	fmt.Println(ranking.Keys())
	// Output:
	// p2 250
	// [100 250]
}
