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
- **Verified** — 300+ exported symbols documented, testify suite with
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
| `PriorityQueue`      | `pqueue`      | `PriorityQueue` (binary heap)                                |
| `Deque`              | `deque`       | `ArrayDeque` (FIFO queue + LIFO stack)                       |
| `RingBuffer`         | `ringbuffer`  | — (keep-newest-N buffer)                                     |
| `BitSet`             | `bitset`      | `BitSet` (bit vector)                                        |
| `BloomFilter`        | `bloomfilter` | — (probabilistic membership, no false negatives)             |
| `UnionFind`          | `unionfind`   | — (disjoint-set union)                                       |
| `LRU`                | `lru`         | `LinkedHashMap(accessOrder=true)` + `removeEldestEntry`      |
| `TTLCache`           | `ttlcache`    | — (LRU + per-entry expiry, goroutine-safe)                   |
| `ConcurrentSet`      | `concurrent`  | `ConcurrentHashMap` key set / synchronized `HashSet`         |
| `ConcurrentMap`      | `concurrent`  | `ConcurrentHashMap` (atomic `GetOrPut`)                      |
| `ConcurrentSkipList` | `concurrent`  | `ConcurrentSkipListMap`                                      |
| `ConcurrentLRU`      | `concurrent`  | synchronized LRU cache                                       |
| `ConcurrentQueue`    | `concurrent`  | `ArrayBlockingQueue` (bounded, blocking)                     |

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
- **Injectable clock.** `ttlcache.NewWithClock` keeps expiry tests
  deterministic — no sleeps.
- **Verification.** testify assertions, per-package benchmarks, allocation
  guards, fuzz targets against reference models, an AST-based Godoc coverage
  check (304/304 exported symbols documented), and CI running
  `gofmt` + `go vet` + `go test -race` + benchmark compilation.

## Roadmap

- `DelayQueue` and `PriorityBlockingQueue` style containers.
- Lock-free concurrent skip list (CAS-based, as in the JDK) in place of the
  mutex-wrapped variant.
- Generated API documentation on `pkg.go.dev`.

## License

[MIT](LICENSE)
