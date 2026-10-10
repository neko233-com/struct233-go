# go-data-struct-lib

Common data structures for Go, modeled after the JDK collection framework
(`java.util` / `java.util.concurrent`). Java ships a rich toolbox of
collections; this library brings the same vocabulary to Go with generics.

- **One entry point** — `struct233.NewSet`, `struct233.NewSkipList`, …
- **High performance, low memory** — zero-allocation read paths, no per-write
  scratch allocations (see [Performance](#performance)).
- **testify test suite** — every container is covered by
  [`testify`](https://github.com/stretchr/testify) assertions, including
  allocation guards for hot paths.

## Structures

| Package / alias     | Type                        | JDK analog                                                   |
| ------------------- | --------------------------- | ------------------------------------------------------------ |
| `struct233`         | facade (factories + aliases)| —                                                             |
| `set` / `Set`       | `HashSet[K]`                | `HashSet`                                                     |
| `set` / `LinkedSet` | `LinkedHashSet[K]`          | `LinkedHashSet` (insertion order)                             |
| `set` / `SortedSet` | `TreeSet[K]`                | `TreeSet` (NavigableSet semantics)                            |
| `skiplist`          | `SkipList[K, V]`            | `TreeMap` / `ConcurrentSkipListMap` (NavigableMap semantics)  |
| `pqueue`            | `PriorityQueue[T]`          | `PriorityQueue` (binary heap)                                 |
| `deque`             | `ArrayDeque[T]`             | `ArrayDeque` (FIFO queue + LIFO stack)                        |
| `bitset`            | `BitSet`                    | `BitSet` (bit vector)                                         |
| `lru`               | `LRU[K, V]`                 | `LinkedHashMap(accessOrder=true)` + `removeEldestEntry`       |

## Install

```
go get github.com/neko233-com/go-data-struct-lib@v0.1.0
```

Requires Go 1.24+.

## Usage

Everything is reachable from the root package:

```go
import "github.com/neko233-com/go-data-struct-lib"

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

q := struct233.NewPriorityQueue[int]()   // min-heap on natural order
q.Push(3)
v, _ := q.Pop()                          // 3 -> 1 after pushing 1
maxQ := struct233.NewPriorityQueueFunc(func(a, b int) int { return b - a })

d := struct233.NewDeque[int]()
d.PushBack(2)  // queue: offer
d.PushFront(1)
d.PopFront()   // 1 (queue: poll)
d.PopBack()    // 2 (stack: pop)

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
available. For custom ordering use `NewSortedSetFunc`, `NewSkipListFunc`,
`NewPriorityQueueFunc`; `struct233.Comparator[K]` documents the comparator
contract (negative / zero / positive, same as `java.util.Comparator`).

### Pre-sized construction

When the element count is known up front, skip the rehashing:

```go
s := struct233.NewSetWithCapacity[int](1024) // map pre-sized for 1024 keys
q := struct233.NewPriorityQueueFromSlice(items, cmp) // O(n) heapify
d := struct233.NewDeque[int](4096) // pre-allocated ring buffer
```

## Performance

`go test -bench=Benchmark -benchtime=100ms ./...` on AMD Ryzen 9 9950X,
Windows, Go 1.27 (`B/op` = bytes allocated per operation):

| Operation                            | ns/op | B/op  | allocs/op |
| ------------------------------------ | ----- | ----- | --------- |
| `BitSet.Get`                         | 0.58  | 0     | 0         |
| `BitSet.Set`                         | 0.82  | 0     | 0         |
| `PriorityQueue.Pop`                  | 0.86  | 0     | 0         |
| `Deque.PushFront` + `PopBack`        | 1.66  | 0     | 0         |
| `Deque.PushBack` + `PopFront`        | 2.10  | 0     | 0         |
| `HashSet.Contains`                   | 3.63  | 0     | 0         |
| `Deque.PushBack`                     | 3.70  | 31    | 0         |
| `LRU.Peek`                           | 4.49  | 0     | 0         |
| `PriorityQueue.Push`                 | 8.51  | 46    | 0         |
| `LRU.Get` (promotes)                 | 8.84  | 0     | 0         |
| `LRU.Put` (update)                   | 11.28 | 0     | 0         |
| `BitSet.Cardinality` (4k bits)       | 23.31 | 0     | 0         |
| `BitSet.Or`                          | 29.12 | 0     | 0         |
| `PriorityQueue.Push` + `Pop`         | 30.67 | 0     | 0         |
| `LinkedHashSet.Add`                  | 81.32 | 78    | 1         |
| `SkipList.Get`                       | 134.2 | 0     | 0         |
| `TreeSet.Add`                        | 140.7 | 42    | 2         |
| `SkipList.Put`                       | 154.6 | 58    | 2         |
| `LRU.Put` (insert + evict)           | 59.52 | 32    | 1         |
| `SkipList.Range` (1k keys)           | 2499  | 0     | 0         |
| `PriorityQueue.NewFromSlice` (1024)  | 3899  | 8224  | 2         |
| `HashSet.Union` (512 + 512)          | 18002 | 37000 | 7         |

Allocation design (asserted by tests such as `TestReadPathsDoNotAllocate`,
`TestPushPopDoesNotAllocate`, `TestReadsDoNotAllocate`):

- reads (`Get` / `Contains` / `Peek` / `Front` / `Cardinality`) never allocate;
- writes reuse internal buffers — the skip list keeps a per-instance update
  buffer and the bit set grows its word slice in place, so `Put` / `Set` cost
  one allocation for the new entry itself and nothing else;
- steady-state queue/deque traffic (`Push`+`Pop`) is allocation-free once the
  backing array has grown;
- set algebra ships a pre-sized result map instead of intermediate slices.

## Design notes

- **Generics first.** Ordered containers use `cmp.Ordered` for natural
  ordering and accept a `Comparator` for custom ordering.
- **Iterators.** Containers expose `All()` / `Range(...)` returning
  `iter.Seq` / `iter.Seq2`, usable with `range` and early `break`.
- **Not concurrency-safe.** Like `java.util` collections, instances are
  unsynchronized; guard them externally when shared.
- **Tests.** The suite uses `testify` (`require` / `assert`) and includes
  benchmarks per package plus allocation guards for hot paths.

## License

[MIT](LICENSE)
