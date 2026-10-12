# struct233-go

Common data structures for Go, modeled after the JDK collection framework
(`java.util` / `java.util.concurrent`). Java ships a rich toolbox of
collections; this library brings the same vocabulary to Go with generics.

- **One entry point** — `struct233.NewSet`, `struct233.NewSkipList`,
  `struct233.NewConcurrentLRU`, …
- **High performance, low memory** — zero-allocation read paths, no per-write
  scratch allocations (see [Performance](#performance)).
- **Goroutine-safe variants** — `Concurrent*` containers plus a blocking queue
  and a TTL cache, mirroring `java.util.concurrent`.
- **Utilities as well** — a bounded object pool, RFC 9562 UUIDs, snowflake IDs,
  a worker pool, two rate limiters, a delay queue, a timing wheel, a circuit
  breaker, retry with backoff, weighted random picks and the
  `java.util.concurrent` synchronizers (semaphore, latch, barrier), all behind
  the same import.
- **Verified** — 600+ exported symbols documented, testify suite with
  allocation guards, fuzz targets comparing containers against reference
  implementations, and CI running `gofmt` / `go vet` / `go test -race`.

## Structures

| Facade alias         | Package       | JDK analog                                                   |
| -------------------- | ------------- | ------------------------------------------------------------ |
| `Set`                | `set`         | `HashSet`                                                    |
| `LinkedSet`          | `set`         | `LinkedHashSet` (insertion order)                            |
| `SortedSet`          | `set`         | `TreeSet` (NavigableSet semantics)                           |
| `SkipList`           | `skiplist`    | `ConcurrentSkipListMap` / `TreeMap` (NavigableMap semantics)  |
| `TreeMap`            | `skiplist`    | `TreeMap` (alias of `SkipList`)                              |
| `LinkedMap`          | `linkedmap`   | `LinkedHashMap` (insertion order)                            |
| `BiMap`              | `bidimap`     | Guava `BiMap` (bidirectional map, live inverse view)         |
| `PriorityQueue`      | `pqueue`      | `PriorityQueue` (binary heap)                                |
| `Deque`              | `deque`       | `ArrayDeque` (FIFO queue + LIFO stack)                       |
| `RingBuffer`         | `ringbuffer`  | — (keep-newest-N buffer)                                     |
| `BitSet`             | `bitset`      | `BitSet` (bit vector)                                        |
| `BloomFilter`        | `bloomfilter` | — (probabilistic membership, no false negatives)             |
| `UnionFind`          | `unionfind`  | — (disjoint-set union)                                       |
| `Table`              | `table`      | Guava `Table` (row / column / value map)                       |
| `RangeSet`           | `rangeset`   | — (Guava `RangeSet`: disjoint, coalesced intervals)           |
| `RangeMap`           | `rangemap`   | Guava `RangeMap` (disjoint intervals mapped to values)         |
| `LRU`                | `lru`         | `LinkedHashMap(accessOrder=true)` + `removeEldestEntry`      |
| `TTLCache`           | `ttlcache`    | — (LRU + per-entry expiry, goroutine-safe)                   |
| `ConcurrentSet`      | `concurrent`  | `ConcurrentHashMap` key set / synchronized `HashSet`         |
| `ConcurrentMap`      | `concurrent`  | `ConcurrentHashMap` (atomic `GetOrPut`)                      |
| `ConcurrentSkipList` | `concurrent`  | `ConcurrentSkipListMap`                                      |
| `ConcurrentLRU`      | `concurrent`  | synchronized LRU cache                                       |
| `ConcurrentQueue`    | `concurrent`  | `ArrayBlockingQueue` (bounded, blocking)                     |
| `ConcurrentPriorityQueue` | `concurrent` | `PriorityBlockingQueue` (blocking, priority-ordered)     |

## Utilities

The same import also covers the everyday primitives that these containers are
usually surrounded by.

| Facade alias          | Package      | Purpose                                                        |
| --------------------- | ------------ | -------------------------------------------------------------- |
| `ObjectPool`          | `pool`       | bounded, goroutine-safe LIFO object pool with reuse counters    |
| `UUID`                | `uuid`       | RFC 9562 identifiers: v4, v7, parse, format, JSON/binary text   |
| `UUIDGenerator`       | `uuid`       | strictly increasing v7 UUIDs, injectable clock                  |
| `Snowflake`           | `snowflake`  | 64-bit time-ordered IDs without a coordinator                   |
| `WorkerPool`          | `workerpool` | fixed goroutine pool with backpressure, metrics, panic safety   |
| `RateLimiter`         | `ratelimit`  | token bucket (`Allow` / `Reserve` / `Wait`), injectable clock    |
| `SlidingWindowLimiter`| `ratelimit`  | exact "N events per window" counter (`Remaining`/`RetryAfter`)  |
| `DelayQueue`          | `delayqueue` | `DelayQueue`: elements become takeable at their deadline         |
| `Semaphore`           | `semaphore`  | weighted `Semaphore` with context-aware acquire                  |
| `Latch`               | `latch`      | `CountDownLatch`                                                 |
| `Barrier`             | `barrier`    | `CyclicBarrier` with arrival indexes and breakage                |
| `RetryPolicy`         | `retry`      | exponential backoff with jitter, `RetryIf`, injectable sleep     |
| `Breaker`             | `breaker`    | circuit breaker: closed / open / half-open, probe limits          |
| `TimeWheel`           | `timewheel`  | hierarchical timing wheel: O(1) schedule and cancel              |
| `Random`              | `random`     | weighted picks, random strings/ids, shuffles                     |
| `WeightedTable`       | `random`     | precomputed weighted picker with an allocation-free `Pick`        |
| `Multiset`            | `multiset`   | counting bag: `Count`, `TotalCount`, `MostCommon`, `Elements`     |
| `ListMultimap`        | `multimap`   | one key to many values, insertion ordered, duplicates allowed    |
| `SetMultimap`         | `multimap`   | one key to a set of values, insertion ordered                    |
| `Striped` / `StripedRW` | `striped`  | hashed mutex/RWMutex stripes, contention sharding                |
| `AtomicLongMap`       | `atomiclongmap` | Guava `AtomicLongMap`: concurrent key → int64 counters       |
| `Stopwatch`           | `stopwatch`  | Guava `Stopwatch`: accumulated start/stop segments               |
| `Supplier`            | `memoize`    | Guava `Suppliers.memoize`: lazy one-shot value, optional TTL     |
| `Joiner` / `Splitter` | `text`       | fluent join/split with skip / trim / omit-empty / limit           |
| — (free functions)    | `text`       | `JoinStrings`, `PadStart`, `Truncate`, `ToSnakeCase`, …           |
| — (free functions)    | `order`      | comparator combinators: `Natural`, `Reverse`, `Then`, `ByKey`, …  |
| — (free functions)    | `hash`       | `Sum64`, Murmur3, FNV-1a, CRC-32, CRC-32C, Adler-32               |
| `HashRing`            | `hashring`   | consistent hashing with virtual nodes for sharding                |
| — (free functions)    | `verify`     | `RequireTrue`, `RequireNotNil`, … panic-on-failure preconditions  |

## Install

```
go get github.com/neko233-com/struct233-go@v0.2.0
```

Requires Go 1.24+ (generic type aliases and range-over-func iterators).

## Usage

Everything is reachable from the root package:

```go
import "github.com/neko233-com/struct233-go"

s := struct233.NewSet(1, 2, 3)
s.Add(4)
s.Union(struct233.NewSet(5))        // {1, 2, 3, 4, 5}
s.Intersection(struct233.NewSet(2)) // {2}

sorted := struct233.NewSortedSet(50, 10, 30)
sorted.Ceiling(20) // 30
sorted.Higher(30)  // 50

ordered := struct233.NewLinkedSet("a", "b", "c") // iteration: a, b, c

sl := struct233.NewSkipList[int, string]()
sl.Put(30, "c")
sl.Put(10, "a")
if k, v, ok := sl.Floor(25); ok { // greatest key <= 25
    fmt.Println(k, v) // 10 a
}
for k, v := range sl.Range(10, 30, true, false) { // [10, 30)
    fmt.Println(k, v)
}

lm := struct233.NewLinkedMap[string, int]() // java.util.LinkedHashMap
lm.Put("b", 1)
lm.Put("a", 2)
lm.Keys() // [b a]

q := struct233.NewPriorityQueue[int]()   // min-heap on natural order
q.Push(3)
v, _ := q.Pop()                          // 3 -> 1 after pushing 1
maxQ := struct233.NewPriorityQueueFunc(func(a, b int) int { return b - a })

d := struct233.NewDeque[int]()
d.PushBack(2)  // queue: offer
d.PushFront(1)
d.PopFront()   // 1 (queue: poll)
d.PopBack()    // 2 (stack: pop)

recent := struct233.NewRingBuffer[Damage](4096) // keeps the newest 4096 hits
if evicted, ok := recent.Push(hit); ok {
    recycle(evicted)
}

b := struct233.NewBitSet()
b.Set(1000)
b.Cardinality() // 1
b.NextSetBit(0) // 1000
b.Flip(1000)
b.IsEmpty()     // true

cache := struct233.NewLRU[string, []byte](128)
cache.Put("key", data)
if v, ok := cache.Get("key"); ok { // promotes to most-recently-used
    use(v)
}
if ek, _, evicted := cache.Put("other", data2); evicted {
    log.Printf("evicted %s", ek)
}

spans := struct233.NewRangeSet[int]() // Guava RangeSet
spans.Add(struct233.RangeClosedOpen(2, 4))
spans.Add(struct233.RangeAtLeast(6))    // [2,4) [6,+inf)
spans.Contains(3)                       // true
spans.Complement().Spans()              // (-inf,2) [4,6)

// table: values addressed by both a row and column key
scores := struct233.NewTable[string, string, int]()
scores.Put("ada", "math", 98)
scores.Put("ada", "physics", 94)
scores.Get("ada", "math")                // 98, true
scores.Row("ada")                        // detached map of Ada's scores

// rangemap: overwrite parts of a domain while retaining the outer mappings
prices := struct233.NewRangeMap[int, string]()
prices.Put(struct233.RangeClosed(1, 10), "standard")
prices.Put(struct233.RangeClosedOpen(4, 7), "discount")
prices.Get(5)                             // "discount", true
prices.Entries()                          // [1,4), [4,7), [7,10]
```

The `struct233` types are **aliases** of the implementations in the
subpackages, so the facade adds no indirection and the advanced API stays
available. For custom ordering use `NewSortedSetFunc`, `NewSkipListFunc` /
`NewTreeMapFunc`, `NewPriorityQueueFunc`, `NewConcurrentSkipListFunc`;
`struct233.Comparator[K]` documents the comparator contract (negative / zero /
positive, same as `java.util.Comparator`).

### Probabilistic and structural helpers

```go
// bloomfilter: cheap "have I seen this?" filter with no false negatives
seen := struct233.NewBloomFilter(1_000_000, 0.01) // 1M items, 1% false positives
if seen.TestString(token) { /* may be a duplicate */ }
seen.AddString(token)

// unionfind: merge groups (guilds, territories, clusters)
groups := struct233.NewUnionFind(playerCount)
groups.Union(a, b)
groups.Connected(a, b) // true
groups.Count()         // remaining component count
groups.Size(a)         // members in a's component

// ttlcache: LRU + per-entry expiry, goroutine-safe
tokens := struct233.NewTTLCache[string, Session](10_000, 30*time.Minute)
tokens.Put(token, session)
tokens.PutWithTTL(token, session, time.Minute) // per-entry override
sess, ok := tokens.Get(token)                  // expired entries report false
tokens.Cleanup()                               // optional eager sweep

// bidimap: both directions are functions (Guava BiMap)
names := struct233.NewBiMap[string, int]()
names.Put("one", 1)                            // true
names.Put("uno", 1)                            // false: 1 is already bound
names.ForcePut("uno", 1)                       // steals the value, drops "one"
id, _ := names.Get("uno")                      // 1
name, _ := names.GetKeyOf(1)                   // "uno"
names.Inverse().Get(1)                         // live view over the same storage
```

### Goroutine-safe containers

The plain containers are unsynchronized (like `java.util`). Share state across
goroutines with the `Concurrent*` types:

```go
ranking := struct233.NewConcurrentSkipList[int64, string]() // score -> player
ranking.Put(1200, "p1")
ranking.Put(1500, "p2")
top, name, _ := ranking.Last() // 1500 p2
for score, player := range ranking.Range(1000, 2000, true, true) {
    fmt.Println(score, player)
}

seen := struct233.NewConcurrentSetWithCapacity[int64](1 << 16)
if seen.Add(uid) { /* first time */ }

sessions := struct233.NewConcurrentMapWithCapacity[int64, *Session](1024)
sess, _ := sessions.GetOrPut(uid, func() *Session { return newSession(uid) }) // atomic

sessCache := struct233.NewConcurrentLRU[int64, *Session](4096)

// bounded blocking queue: producers block when full, consumers when empty
jobs := struct233.NewConcurrentQueue[Job](1024)
go func() { jobs.Put(job) }()                    // blocks while full
if job, ok := jobs.Poll(50 * time.Millisecond); ok { handle(job) }
jobs.Close()                                     // unblocks every waiter

// blocking priority queue (PriorityBlockingQueue): head is the smallest
// element per the comparator, equal elements leave FIFO
tasks := struct233.NewConcurrentPriorityQueue(func(a, b Task) int { return a.Deadline.Compare(b.Deadline) })
tasks.Put(task)                                  // blocks while a bounded queue is full
t, ok := tasks.Take()                            // blocks while empty
```

All methods take the internal lock for one operation; iteration methods
(`All`, `Range`, `Keys`, `Values`, `ToSlice`, `Snapshot`) return a **snapshot**
taken under the lock, so ranging never blocks writers.

### Pre-sized construction

When the element count is known up front, skip the rehashing:

```go
s := struct233.NewSetWithCapacity[int](1024)          // map pre-sized for 1024 keys
m := struct233.NewConcurrentMapWithCapacity[int64, *Session](4096)
q := struct233.NewPriorityQueueFromSlice(items, cmp)  // O(n) heapify
d := struct233.NewDeque[int](4096)                    // pre-allocated ring buffer
```

### Reuse, identity and bounded concurrency

```go
// pool: recycle expensive, short-lived objects instead of re-allocating them
packets := struct233.NewObjectPool(func() *Packet { return new(Packet) },
    struct233.WithObjectPoolReset(func(p *Packet) { p.Reset() }),
    struct233.WithObjectPoolMaxSize(1024),
)
p := packets.Get()
p.Write(frame)
packets.Put(p)                      // reset, then stored for the next caller
packets.Stats().ReuseRate()         // 0..1, how often Get hit the pool
packets.Prealloc(256)               // warm the pool during startup

// uuid: v4 for randomness, v7 for sortable keys (time-ordered, k-sortable)
session := struct233.NewUUID()      // version 4, 122 random bits
key := struct233.NewUUID7()         // version 7, strictly increasing
if parsed, err := struct233.ParseUUID(key.String()); err == nil {
    when, _ := parsed.Time()        // the embedded millisecond timestamp
}
struct233.NewUUID7At(deadline)      // v7 with an explicit timestamp
struct233.NilUUID                   // the all-zero UUID

// snowflake: 64-bit, time-ordered IDs from a node id, no coordination
ids := struct233.NewSnowflake(7)              // 10 node bits by default
id, err := ids.Next()                         // err is snowflake.ErrClockBackwards
if err == nil {
    fmt.Println(ids.TimeOf(id), ids.NodeOf(id), ids.SequenceOf(id))
}

// workerpool: run at most N tasks at a time, with backpressure
jobs := struct233.NewWorkerPool(4, struct233.WithWorkerPoolQueueSize(1024))
defer jobs.Close()                            // drains and waits, idempotent
jobs.Submit(func() { handle() })              // blocks while the queue is full
jobs.TrySubmit(func() { handle() })           // reports workerpool.ErrQueueFull
jobs.SubmitWait(func() { handle() })          // waits for the task to finish
jobs.Metrics().Completed                      // counters, queue depth, active workers

// ratelimit: token bucket per client, per endpoint, per anything
limit := struct233.NewRateLimiter(100, 10)    // 100 events/s, bursts of 10
if limit.Allow() {
    serve()
}
limit.AllowN(5)                               // all-or-nothing
limit.Reserve()                               // how long until the next token
limit.Wait(ctx)                               // block until a token is free

// ratelimit (windowed): exact "at most N in any window" quota
window := struct233.NewSlidingWindowLimiter(1000, time.Second)
window.AllowN(10)                             // all-or-nothing
window.Remaining()                            // slots left in the window
window.RetryAfter()                           // how long until the next slot frees

// delayqueue: elements become takeable when their deadline passes
delayed := struct233.NewDelayQueue[Order]()
delayed.Put(struct233.NewDelayElement(order, time.Now().Add(30*time.Second)))
if item, ok := delayed.Take(); ok {           // blocks until the order is due
    handle(item.Value())
}
delayed.Poll(50 * time.Millisecond)           // or wait at most a moment
delayed.Close()                               // unblocks the takers

// timewheel: thousands of timers with O(1) schedule/cancel
wheel := struct233.NewTimeWheel(10*time.Millisecond, 512)
wheel.Schedule(5*time.Second, reconnect)      // fires within one tick of 5s
timer := wheel.ScheduleAt(deadline, expire)
timer.Cancel()                                // true when still pending
wheel.Tick()                                  // drive from a game loop
go wheel.Run(ctx)                             // or tick on a real clock

// semaphore / latch / barrier: the java.util.concurrent synchronizers
slots := struct233.NewSemaphore(16)           // weighted permits
if err := slots.AcquireN(ctx, 4); err == nil {
    defer slots.Release(4)
}
ready := struct233.NewLatch(3)                // CountDownLatch
ready.CountDown()                             // third call opens it
err := ready.Await(ctx)

round := struct233.NewBarrier(4)              // CyclicBarrier, reusable
index, err := round.Await(ctx)                // 0..3, arrival order per round

// retry: exponential backoff with jitter, context aware
err = struct233.Retry(ctx, callFlakyService,
    struct233.WithRetryMaxAttempts(5),
    struct233.WithRetryBaseDelay(100*time.Millisecond),
    struct233.WithRetryIf(func(err error) bool { return isRetryable(err) }),
)
value, err := struct233.RetryValue(ctx, fetchCount)   // for a value

// breaker: stop hammering a failing dependency
call := struct233.NewBreaker(
    struct233.WithBreakerFailureThreshold(5),
    struct233.WithBreakerOpenTimeout(30*time.Second),
)
err = call.Execute(func() error { return callRemote() })  // ErrBreakerOpen when open
call.State()                                             // closed / open / half-open

// random: weighted drop tables, ids, shuffles
table := struct233.NewWeightedTable(nil, drops, func(d Drop) float64 { return d.Chance })
drop, _ := table.Pick()                                  // allocation-free
token := struct233.DefaultRandom().String(24)            // alphanumeric id
struct233.Shuffle(nil, deck)

// multiset: counts with a deterministic iteration order
seen := struct233.NewMultiset("a", "b", "a")
seen.AddN("b", 2)                                        // exact counts
seen.MostCommon()                                        // b 3 true
seen.Elements()                                          // [a a b b b]

// multimap: one key to many values (list) or to a set of values (set)
invites := struct233.NewListMultimap[string, int]()      // duplicates allowed
invites.Put("raid", 1)
invites.Put("raid", 2)
invites.Get("raid")                                      // [1 2]
tags := struct233.NewSetMultimap[string, int]()          // unique per key
tags.Put("p1", 7)                                        // false on a duplicate
tags.Invert().Get(7)                                     // [p1]

// striped: spread lock contention over independent mutex stripes
stripes := struct233.NewStriped(16, hashTaskKey)          // rounded up to a power of two
mu := stripes.Get(key)                                   // same key -> same mutex, zero allocs
mu.Lock()
criticalSection()
mu.Unlock()

// atomiclongmap: concurrent key -> int64 counters, no external lock
hits := struct233.NewAtomicLongMap[string]()
hits.Increment("login")                                  // 1, hot path allocates nothing
hits.Add("login", 9)                                     // 10
hits.Sum()                                               // zero-valued entries are kept

// stopwatch: accumulate busy time across start/stop segments
sw := struct233.NewStartedStopwatch()
work()
sw.Stop()                                                // the gap is free time
sw.Elapsed()                                             // only running segments count

// memoize: compute an expensive value lazily, exactly once
cfg := struct233.Memoize(func() *Config { return loadConfig() })
cfg.Get()                                                // computed here...
cfg.Get()                                                // ...and reused forever
expiring := struct233.MemoizeExpiring(fetchQuote, time.Minute) // recomputes after the TTL

// text: join and split with a policy, plus case conversion
struct233.NewJoiner(", ").SkipEmpty().Join("ada", "", "grace")   // "ada, grace"
struct233.NewSplitter(",").Trim().OmitEmpty().Split(" a , , b ") // [a b]
struct233.ToSnakeCase("HTTPServer")                              // http_server
struct233.Truncate("hello world", 8, "…")                        // hello…

// verify: preconditions that panic with a classifiable error
struct233.RequirePositive(n, "n %d must be positive", n)  // errors.Is(err, ErrInvalidArgument)
cfg := struct233.RequireNotNil(cfg, "config must be set") // returns the value

// hash: sharding-friendly hashes for ring placement and dedup keys
struct233.Sum64([]byte("user:42"))                        // MurmurHash3, seed 0
struct233.FNV1a64([]byte("user:42"))

// order: comparator combinators that plug straight into the containers
byScoreDesc := struct233.Reverse(struct233.Natural[int]())
leaderboard := struct233.NewSortedSetFunc(byScoreDesc, 10, 30, 20) // 30, 20, 10
byNameThenAge := struct233.Then(
    struct233.ByKey(func(p Player) string { return p.Name }, struct233.Natural[string]()),
    struct233.ByKey(func(p Player) int { return p.Age }, struct233.Natural[int]()),
)
struct233.Sorted(byNameThenAge, roster)                   // stable, input untouched
```

Every utility takes an injectable clock (`struct233.WithUUIDClock`,
`WithSnowflakeClock`, `WithRateLimitClock`, `WithBreakerClock`,
`WithTimeWheelClock`, `WithStopwatchClock`, `WithMemoizeClock`), so expiry,
ordering, refill, measurement and breaker transitions are testable without
sleeping. The sentinel errors are re-exported by the facade
(`struct233.ErrInvalidUUID`, `ErrClockBackwards`, `ErrWorkerPoolClosed`,
`ErrWorkerPoolQueueFull`, `ErrBreakerOpen`, `ErrBarrierBroken`) and stay
identical to the subpackage values, so `errors.Is` works either way.

## Performance

`go test -bench=Benchmark -benchtime=100ms ./...` on AMD Ryzen 9 9950X,
Windows, Go 1.27 (`B/op` = bytes allocated per operation):

| Operation                            | ns/op | B/op  | allocs/op |
| ------------------------------------ | ----- | ----- | --------- |
| `BitSet.Get`                         | 0.58  | 0     | 0         |
| `RingBuffer.Push` (with eviction)    | 0.79  | 0     | 0         |
| `BitSet.Set`                         | 0.82  | 0     | 0         |
| `PriorityQueue.Pop`                  | 0.86  | 0     | 0         |
| `UnionFind.Find`                     | 0.94  | 0     | 0         |
| `Deque.PushFront` + `PopBack`        | 1.66  | 0     | 0         |
| `UnionFind.Union`                    | 1.98  | 0     | 0         |
| `Deque.PushBack` + `PopFront`        | 2.10  | 0     | 0         |
| `HashSet.Contains`                   | 3.63  | 0     | 0         |
| `Deque.PushBack`                     | 3.70  | 0     | 0         |
| `LinkedMap.Get`                      | 4.07  | 0     | 0         |
| `LRU.Peek`                           | 4.49  | 0     | 0         |
| `LinkedMap.Put` (update)             | 4.90  | 0     | 0         |
| `PriorityQueue.Push`                 | 8.51  | 46    | 0         |
| `LRU.Get` (promotes)                 | 8.84  | 0     | 0         |
| `BloomFilter.TestUint64`             | 8.38  | 0     | 0         |
| `LRU.Put` (update)                   | 11.28 | 0     | 0         |
| `BitSet.Cardinality` (4k bits)       | 23.31 | 0     | 0         |
| `BloomFilter.AddString`              | 23.61 | 0     | 0         |
| `BitSet.Or`                          | 29.12 | 0     | 0         |
| `PriorityQueue.Push` + `Pop`         | 30.67 | 0     | 0         |
| `LinkedHashSet.Add`                  | 81.32 | 78    | 1         |
| `LinkedMap.Put` (insert)             | 104.9 | 107   | 1         |
| `SkipList.Get`                       | 134.2 | 0     | 0         |
| `TreeSet.Add`                        | 140.7 | 42    | 2         |
| `SkipList.Put`                       | 154.6 | 58    | 2         |
| `LRU.Put` (insert + evict)           | 59.52 | 32    | 1         |
| `SkipList.Range` (1k keys)           | 2499  | 0     | 0         |
| `PriorityQueue.NewFromSlice` (1024)  | 3899  | 8224  | 2         |
| `HashSet.Union` (512 + 512)          | 18002 | 37000 | 7         |

Goroutine-safe containers, all zero-allocation:

| Operation                                     | ns/op (32 threads) |
| --------------------------------------------- | ------------------ |
| `ConcurrentSkipList.Get`                      | 23.6               |
| `ConcurrentMap.Get`                           | 26.7               |
| `ConcurrentSet.Contains`                      | 28.4               |
| `ConcurrentLRU.Get`                           | 56.4               |
| `ConcurrentSet.Add`                           | 63.5               |
| `TTLCache.Get` (mutex, incl. expiry check)    | 21.2               |
| `TTLCache.Put` (update)                       | 21.9               |
| `ConcurrentQueue.Put` + `Take`                | 20.9               |

Allocation design (asserted by tests such as `TestReadPathsDoNotAllocate`,
`TestPushPopDoesNotAllocate`, `TestReadsDoNotAllocate`,
`TestPushWithEvictionDoesNotAllocate`):

- reads (`Get` / `Contains` / `Peek` / `Front` / `Cardinality`) never allocate;
- writes reuse internal buffers — the skip list keeps a per-instance update
  buffer and the bit set grows its word slice in place, so `Put` / `Set` cost
  one allocation for the new entry itself and nothing else;
- steady-state queue/deque/ring-buffer traffic (`Push` + `Pop`) is
  allocation-free once the backing array has grown;
- set algebra ships a pre-sized result map instead of intermediate slices;
- union-find, bloom filters and the TTL cache are allocation-free on their
  hot paths.

## Design notes

- **Generics first.** Ordered containers use `cmp.Ordered` for natural
  ordering and accept a `Comparator` for custom ordering.
- **Iterators.** Containers expose `All()` / `Range(...)` returning
  `iter.Seq` / `iter.Seq2`, usable with `range` and early `break`.
- **Concurrency.** Plain containers are unsynchronized; `concurrent.*`
  (`Concurrent*` aliases) wraps them with an `RWMutex`/`Mutex` and snapshots
  every iteration, so callers never hold the lock while consuming. The
  blocking `Queue` uses condition variables with deadline-aware waits.
- **Injectable clock.** Every time-based utility (`ttlcache`, `delayqueue`,
  `timewheel`, `ratelimit`, `breaker`, `uuid`, `snowflake`, `stopwatch`,
  `memoize`) takes its time source as an option, so expiry, ordering, refill
  and breaker transitions are tested without sleeping.
- **Verification.** testify assertions, per-package benchmarks, allocation
  guards, fuzz targets against reference models (containers, plus UUID text and
  binary codecs), an AST-based Godoc coverage check (640/640 exported symbols
  documented, struct fields included), and CI running
  `gofmt` + `go vet` + `go test -race` + benchmark compilation.

## Roadmap

- Further blocking containers on top of the existing primitives.
- Lock-free concurrent skip list (CAS-based, as in the JDK) in place of the
  mutex-wrapped variant.
- Generated API documentation on `pkg.go.dev`.

## License

[MIT](LICENSE)
