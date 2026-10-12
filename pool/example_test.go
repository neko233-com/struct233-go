package pool_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/pool"
)

func ExamplePool() {
	packets := pool.New(func() *item { return &item{} },
		pool.WithReset(func(p *item) { p.buf = p.buf[:0] }),
		pool.WithMaxSize(64),
	)

	p := packets.Get() // a miss: the factory builds the object
	packets.Put(p)
	q := packets.Get() // a hit: the same object comes back (LIFO)
	fmt.Println(p == q)
	// Output:
	// true
}

func ExamplePool_Stats() {
	p := pool.New(func() *item { return &item{} }, pool.WithMaxSize(1))

	p.Put(&item{})
	p.Put(&item{}) // the idle set is full: this one is discarded

	s := p.Stats()
	fmt.Println(s.Gets, s.Hits, s.Misses, s.Puts, s.Discards)

	q := p.Get() // the only idle object: a hit
	p.Get()      // a miss: the factory runs again
	fmt.Println(s.ReuseRate())
	_ = q
	// Output:
	// 0 0 0 1 1
	// 0
}

func ExamplePool_Prealloc() {
	p := pool.New(func() *item { return &item{} }, pool.WithMaxSize(2))
	p.Prealloc(5) // 3 objects do not fit and are dropped
	fmt.Println(p.Len(), p.Cap())

	p.Clear()
	fmt.Println(p.Len())
	// Output:
	// 2 2
	// 0
}
