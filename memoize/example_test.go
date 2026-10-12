package memoize_test

import (
	"fmt"
	"time"

	"github.com/neko233-com/struct233-go/memoize"
)

// ExampleMemoize shows the one-shot lazy computation: the expensive function
// runs on the first Get and every later call reuses the cached value.
func ExampleMemoize() {
	var calls int
	s := memoize.Memoize(func() string {
		calls++
		return "config-loaded"
	})

	fmt.Println(s.Get())
	fmt.Println(s.Get())
	fmt.Println("fn calls:", calls)
	// Output:
	// config-loaded
	// config-loaded
	// fn calls: 1
}

// ExampleMemoizeExpiring shows TTL-based recomputation with an injected
// clock: the value is recomputed on the first Get after the TTL elapses.
func ExampleMemoizeExpiring() {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var calls int
	s := memoize.MemoizeExpiring(func() int {
		calls++
		return calls
	}, time.Minute, memoize.WithClock(func() time.Time { return now }))

	fmt.Println(s.Get()) // computed
	fmt.Println(s.Get()) // still fresh
	now = now.Add(30 * time.Second)
	fmt.Println(s.Get()) // still fresh
	now = now.Add(31 * time.Second)
	fmt.Println(s.Get()) // TTL (1m) elapsed -> recomputed
	fmt.Println("fn calls:", calls)
	// Output:
	// 1
	// 1
	// 1
	// 2
	// fn calls: 2
}

// ExampleSupplier_Invalidate shows forcing a recomputation with Invalidate.
func ExampleSupplier_Invalidate() {
	var calls int
	s := memoize.Memoize(func() int {
		calls++
		return calls
	})

	fmt.Println(s.Get())
	s.Invalidate()
	fmt.Println(s.Get())
	fmt.Println("fn calls:", calls)
	// Output:
	// 1
	// 2
	// fn calls: 2
}

// ExampleSupplier_Stats shows the hit/miss counters.
func ExampleSupplier_Stats() {
	s := memoize.Memoize(func() int { return 42 })
	s.Get() // miss: computes
	s.Get() // hit
	s.Get() // hit

	st := s.Stats()
	fmt.Println("misses:", st.Misses, "hits:", st.Hits)
	// Output:
	// misses: 1 hits: 2
}
