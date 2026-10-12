package struct233_test

import (
	"fmt"
	"time"

	struct233 "github.com/neko233-com/struct233-go"
)

func ExampleNewListMultimap() {
	// a key may repeat a value, and both key and value order are kept
	m := struct233.NewListMultimap[string, int]()
	m.Put("fruit", 1)
	m.Put("fruit", 1)
	m.Put("veg", 2)

	fmt.Println(m.Get("fruit"), m.Size(), m.KeyCount())
	fmt.Println(m.Entries())
	// Output:
	// [1 1] 3 2
	// [{fruit 1} {fruit 1} {veg 2}]
}

func ExampleNewSetMultimap() {
	// values are unique per key, so Put reports whether it was new
	m := struct233.NewSetMultimap[string, int]()
	fmt.Println(m.Put("a", 1), m.Put("a", 1))
	fmt.Println(m.PutAll("a", 1, 2, 3), m.Get("a"))
	fmt.Println(m.Invert().Get(2))
	// Output:
	// true false
	// 2 [1 2 3]
	// [a]
}

func ExampleNewMultiset() {
	m := struct233.NewMultiset("a", "b", "a")
	m.AddN("b", 2)

	elem, count, _ := m.MostCommon()
	fmt.Println(elem, count, m.Distinct(), m.TotalCount())
	// Output:
	// b 3 2 5
}

func ExampleNewJoiner() {
	fmt.Println(struct233.NewJoiner(", ").SkipEmpty().Join("ada", "", "grace"))
	fmt.Println(struct233.NewJoiner(", ").UseForEmpty("-").Join("ada", "", "grace"))
	// Output:
	// ada, grace
	// ada, -, grace
}

func ExampleNewSplitter() {
	fmt.Println(struct233.NewSplitter(",").Trim().OmitEmpty().Split(" ada , , grace "))
	fmt.Println(struct233.NewSplitter(",").Limit(2).Split("a,b,c"))
	// Output:
	// [ada grace]
	// [a b,c]
}

func ExampleToSnakeCase() {
	fmt.Println(struct233.ToSnakeCase("HTTPServer"))
	fmt.Println(struct233.ToLowerCamel("user_id"))
	// Output:
	// http_server
	// userId
}

func ExampleRequirePositive() {
	check := func(n int) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err, _ = r.(error)
			}
		}()
		struct233.RequirePositive(n, "count %d must be positive", n)
		return nil
	}

	fmt.Println(check(3))
	fmt.Println(check(0))
	// Output:
	// <nil>
	// invalid argument: count 0 must be positive
}

func ExampleNewRangeSet() {
	s := struct233.NewRangeSet[int]()
	s.Add(struct233.RangeClosed(1, 3))
	s.Add(struct233.RangeClosed(3, 6)) // closed-touching: coalesces with [1,3]
	fmt.Println(s.Spans())
	s.Remove(struct233.RangeSingleton(2))
	fmt.Println(s.Spans())
	fmt.Println(s.Contains(2))
	// Output:
	// [[1,6]]
	// [[1,2) (2,6]]
	// false
}

func ExampleRangeSet_Complement() {
	s := struct233.NewRangeSet[int]()
	s.Add(struct233.RangeClosedOpen(2, 4))
	fmt.Println(s.Complement().Spans())
	// Output:
	// [(-inf,2) [4,+inf)]
}

func ExampleNewTable() {
	scores := struct233.NewTable[string, string, int]()
	scores.Put("ada", "math", 98)
	scores.Put("ada", "physics", 94)

	math, _ := scores.Get("ada", "math")
	fmt.Println(math, scores.RowKeys(), scores.ColumnKeys())
	// Output:
	// 98 [ada] [math physics]
}

func ExampleNewRangeMap() {
	prices := struct233.NewRangeMap[int, string]()
	prices.Put(struct233.RangeClosed(1, 10), "standard")
	prices.Put(struct233.RangeClosedOpen(4, 7), "discount")

	value, _ := prices.Get(5)
	fmt.Println(value, prices.Entries())
	// Output:
	// discount [{[1,4) standard} {[4,7) discount} {[7,10] standard}]
}

func ExampleSum32() {
	fmt.Println(struct233.Sum32(nil))
	fmt.Printf("%08x\n", struct233.CRC32([]byte("123456789")))
	// Output:
	// 0
	// cbf43926
}

func ExampleNewBiMap() {
	// both directions are functions: a value is bound to at most one key
	b := struct233.NewBiMap[string, int]()
	fmt.Println(b.Put("one", 1))
	fmt.Println(b.Put("uno", 1)) // 1 is already bound to "one"

	b.ForcePut("uno", 1) // rebinds 1, dropping "one"
	v, _ := b.Get("uno")
	k, _ := b.GetKeyOf(1) // reverse lookup
	fmt.Println(v, k, b.Len())
	// Output:
	// true
	// false
	// 1 uno 1
}

func ExampleNewStriped() {
	// spread lock contention over independent stripes instead of one mutex
	s := struct233.NewStriped(4, func(k int) uint64 { return uint64(k) })
	fmt.Println(s.Get(7) == s.Get(7)) // same key, same mutex
	fmt.Println(s.Size())
	// Output:
	// true
	// 4
}

func ExampleNewAtomicLongMap() {
	counters := struct233.NewAtomicLongMap[string]()
	fmt.Println(counters.Increment("hits"))
	fmt.Println(counters.Add("hits", 9))
	fmt.Println(counters.Sum(), counters.Len())

	counters.Put("errors", 0)
	fmt.Println(counters.RemoveIfZero("errors"))
	// Output:
	// 1
	// 10
	// 10 1
	// true
}

func ExampleNewStartedStopwatch() {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	sw := struct233.NewStartedStopwatch(struct233.WithStopwatchClock(func() time.Time { return now }))

	now = now.Add(1500 * time.Millisecond)
	sw.Stop()
	fmt.Println(sw.Elapsed()) // the stopped gap is never counted

	now = now.Add(time.Hour)
	sw.Restart()
	now = now.Add(250 * time.Millisecond)
	fmt.Println(sw)
	// Output:
	// 1.5s
	// 250ms (running)
}

func ExampleMemoizeExpiring() {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var calls int
	s := struct233.MemoizeExpiring(func() int {
		calls++
		return calls
	}, time.Minute, struct233.WithMemoizeClock(func() time.Time { return now }))

	fmt.Println(s.Get()) // computed here
	fmt.Println(s.Get()) // served from the cache

	now = now.Add(time.Minute) // the TTL elapses
	fmt.Println(s.Get())       // recomputed
	fmt.Println("fn calls:", calls)
	// Output:
	// 1
	// 1
	// 2
	// fn calls: 2
}
