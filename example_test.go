package struct233_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	struct233 "github.com/neko233-com/struct233-go"
)

func ExampleNewHashSet() {
	s := struct233.NewHashSet(1, 2, 3)
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

func ExampleNewSkipListSet() {
	s := struct233.NewSkipListSet(30, 10, 20) // asserts ascending order, not insertion
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

func ExampleNewSkipListMap() {
	sl := struct233.NewSkipListMap[int, string]()
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

func ExampleNewLinkedHashMap() {
	m := struct233.NewLinkedHashMap[string, int]()
	for i, name := range []string{"gamma", "alpha", "beta"} {
		m.Put(name, i)
	}
	fmt.Println(m.Keys())
	fmt.Println(m.Values())
	// Output:
	// [gamma alpha beta]
	// [0 1 2]
}

func ExampleNewHeapPriorityQueue() {
	q := struct233.NewHeapPriorityQueue[int]()
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

func ExampleNewArrayDeque() {
	d := struct233.NewArrayDeque[int]()
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

func ExampleNewConcurrentSkipListMap() {
	ranking := struct233.NewConcurrentSkipListMap[int, string]() // goroutine-safe
	ranking.Put(100, "p1")
	ranking.Put(250, "p2")

	top, name, _ := ranking.Last()
	fmt.Println(name, top)
	fmt.Println(ranking.Keys())
	// Output:
	// p2 250
	// [100 250]
}

func ExampleNewArrayBlockingQueue() {
	jobs := struct233.NewArrayBlockingQueue[int](2) // bounded blocking queue
	jobs.Put(1)
	jobs.Put(2)

	fmt.Println(jobs.TryPut(3)) // no space left

	first, _ := jobs.Take()
	fmt.Println(first, jobs.Len())
	// Output:
	// false
	// 1 1
}

func ExampleNewHeapBlockingPriorityQueue() {
	tasks := struct233.NewHeapBlockingPriorityQueue(func(a, b string) int {
		return len(a) - len(b) // shortest task first
	})
	tasks.Put("cccc")
	tasks.Put("bb")
	tasks.Put("aaa")

	for tasks.Len() > 0 {
		v, _ := tasks.Take()
		fmt.Print(v, " ")
	}
	// Output:
	// bb aaa cccc
}

func ExampleNewTTLCache() {
	cache := struct233.NewTTLCache[string, int](128, time.Minute)
	cache.Put("session", 7)

	v, ok := cache.Get("session")
	fmt.Println(v, ok)
	fmt.Println(cache.Contains("missing"), cache.Len())
	// Output:
	// 7 true
	// false 1
}

func ExampleNewUnionFind() {
	groups := struct233.NewUnionFind(5)
	groups.Union(0, 1)
	groups.Union(1, 2)

	fmt.Println(groups.Connected(0, 2))
	fmt.Println(groups.Size(0), groups.Count())
	// Output:
	// true
	// 3 3
}

func ExampleNewBloomFilter() {
	seen := struct233.NewBloomFilter(1000, 0.01) // 1% false positives
	seen.AddString("token-1")
	seen.AddUint64(42)

	fmt.Println(seen.TestString("token-1"))
	fmt.Println(seen.TestUint64(42))
	fmt.Println(seen.Count(), seen.K())
	// Output:
	// true
	// true
	// 2 7
}

func ExampleNewObjectPool() {
	buffers := struct233.NewObjectPool(func() *strings.Builder { return new(strings.Builder) },
		struct233.WithObjectPoolReset(func(b *strings.Builder) { b.Reset() }),
	)

	b := buffers.Get()
	b.WriteString("hello")
	buffers.Put(b)

	reused := buffers.Get() // the reset hook already cleaned the object
	fmt.Printf("%q %d\n", reused.String(), buffers.Stats().Hits)
	// Output:
	// "" 1
}

func ExampleNewUUID7At() {
	id := struct233.NewUUID7At(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	when, _ := id.Time()

	fmt.Println(id.Version(), when.Format(time.RFC3339))
	fmt.Println(id.String() == struct233.MustParseUUID(id.String()).String())
	// Output:
	// 7 2026-01-02T03:04:05Z
	// true
}

func ExampleNewSnowflake() {
	ids := struct233.NewSnowflake(7, struct233.WithSnowflakeNodeBits(4))

	first, _ := ids.Next()
	second, _ := ids.Next()

	fmt.Println(second > first, ids.NodeOf(first), ids.NodeBits())
	// Output:
	// true 7 4
}

func ExampleNewWorkerPool() {
	jobs := struct233.NewWorkerPool(2, struct233.WithWorkerPoolQueueSize(4))
	defer jobs.Close()

	done := make(chan string, 1)
	_ = jobs.Submit(func() { done <- "job done" })
	fmt.Println(<-done)

	jobs.Close()
	fmt.Println(jobs.Closed(), jobs.Metrics().Completed)
	// Output:
	// job done
	// true 1
}

func ExampleNewTokenBucketLimiter() {
	now := time.Unix(0, 0)
	limiter := struct233.NewTokenBucketLimiter(2, 2, struct233.WithRateLimitClock(func() time.Time { return now }))

	fmt.Println(limiter.Allow(), limiter.Allow(), limiter.Allow())
	fmt.Println(limiter.AllowN(2))

	now = now.Add(time.Second) // two tokens are refilled
	fmt.Println(limiter.Tokens(), limiter.AllowN(2))
	// Output:
	// true true false
	// false
	// 2 true
}

func ExampleNewSlidingWindowLimiter() {
	now := time.Unix(0, 0)
	window := struct233.NewSlidingWindowLimiter(2, time.Minute,
		struct233.WithRateLimitClock(func() time.Time { return now }))

	fmt.Println(window.Allow(), window.Allow(), window.Allow())
	fmt.Println(window.Remaining(), window.RetryAfter())

	now = now.Add(time.Minute) // the whole window expires
	fmt.Println(window.Remaining(), window.Allow())
	// Output:
	// true true false
	// 0 1m0s
	// 2 true
}

func ExampleRetry() {
	attempts := 0
	err := struct233.Retry(context.Background(), func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("try again")
		}
		return nil
	},
		struct233.WithRetryMaxAttempts(5),
		struct233.WithRetryJitter(0),
		// inject the sleep so the example does not really wait
		struct233.WithRetrySleep(func(context.Context, time.Duration) error { return nil }),
	)

	fmt.Println(err, attempts)
	// Output:
	// <nil> 3
}

func ExampleNewBreaker() {
	b := struct233.NewBreaker(
		struct233.WithBreakerFailureThreshold(2),
		struct233.WithBreakerOpenTimeout(time.Hour),
	)

	b.Failure()
	b.Failure()
	fmt.Println(b.State())
	fmt.Println(b.Execute(func() error { return nil }))
	fmt.Println(b.Counts().Rejections)
	// Output:
	// open
	// breaker: circuit is open
	// 1
}

func ExampleNewSemaphore() {
	s := struct233.NewSemaphore(2)

	fmt.Println(s.TryAcquire(), s.TryAcquire(), s.TryAcquire())
	fmt.Println(s.Permits())

	s.Release(1)
	fmt.Println(s.Permits())
	// Output:
	// true true false
	// 0
	// 1
}

func ExampleNewLatch() {
	l := struct233.NewLatch(2)

	l.CountDown()
	fmt.Println(l.Open())
	l.CountDown()
	fmt.Println(l.Open(), l.Count())
	// Output:
	// false
	// true 0
}

func ExampleNewDelayQueue() {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q := struct233.NewDelayQueueWithClock[string](func() time.Time { return now })
	q.Put(struct233.NewDelayElement("job", now.Add(time.Minute)))

	_, ok := q.TryTake()
	fmt.Println(ok)

	now = now.Add(time.Minute) // the clock reaches the deadline
	item, ok := q.TryTake()
	fmt.Println(ok, item.Value())
	// Output:
	// false
	// true job
}

func ExampleNewTimeWheel() {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	w := struct233.NewTimeWheel(10*time.Millisecond, 8,
		struct233.WithTimeWheelClock(func() time.Time { return now }))

	fired := 0
	w.Schedule(30*time.Millisecond, func() { fired++ })
	fmt.Println(w.Len())

	now = now.Add(30 * time.Millisecond)
	fmt.Println(w.Advance(now), fired)
	// Output:
	// 1
	// 1 1
}

func ExampleNewWeightedTable() {
	table := struct233.NewWeightedTable(nil, []string{"common", "rare"},
		func(item string) float64 {
			if item == "rare" {
				return 1 // a third of the picks
			}
			return 2
		})

	picked, ok := table.Pick()
	fmt.Println(table.Items(), ok, picked != "")
	// Output:
	// 2 true true
}
