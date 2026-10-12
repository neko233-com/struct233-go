package striped_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/striped"
)

// Example shows the typical shard-your-map-across-stripes pattern: guard a
// plain map with a striped mutex instead of one global lock.
func Example() {
	s := striped.New(4, func(k string) uint64 {
		h := uint64(0xcbf29ce484222325)
		for i := 0; i < len(k); i++ {
			h = (h ^ uint64(k[i])) * 0x100000001b3
		}
		return h
	})

	counts := map[string]int{}
	bump := func(key string) {
		l := s.Get(key)
		l.Lock()
		defer l.Unlock()
		counts[key]++
	}

	bump("alpha")
	bump("alpha")
	bump("beta")

	fmt.Println(counts["alpha"], counts["beta"])
	fmt.Println(s.Size())
	// Output:
	// 2 1
	// 4
}

// ExampleStriped_Get demonstrates key stability: the same key always maps to
// the same *sync.Mutex.
func ExampleStriped_Get() {
	s := striped.New(8, func(k int) uint64 { return uint64(k) })
	a := s.Get(42)
	b := s.Get(42)
	fmt.Println(a == b)
	// Output:
	// true
}

// ExampleStripedRW_GetRW shows the read-write variant: shared reads on the
// same key proceed in parallel, writers stay exclusive.
func ExampleStripedRW_GetRW() {
	rw := striped.NewRW(4, func(k int) uint64 { return uint64(k) })

	var shared int
	mu := rw.GetRW(7)
	mu.RLock()
	_ = shared // readers may overlap
	mu.RUnlock()

	mu.Lock()
	shared = 1 // writers are exclusive
	mu.Unlock()

	fmt.Println(shared)
	// Output:
	// 1
}
