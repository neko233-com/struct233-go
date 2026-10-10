# go-data-struct-lib

Common data structures for Go, modeled after the JDK collection framework
(java.util / java.util.concurrent). Java ships a rich toolbox of collections;
this library brings the same vocabulary to Go with generics.

## Structures

| Package       | Type                             | JDK analog                                                       |
| ------------- | -------------------------------- | ---------------------------------------------------------------- |
| `skiplist`    | `SkipList[K, V]`                 | `TreeMap` / `ConcurrentSkipListMap` (NavigableMap semantics)     |
| `set`         | `HashSet[K]`                     | `HashSet`                                                        |
| `set`         | `LinkedHashSet[K]`               | `LinkedHashSet` (insertion order)                                |
| `set`         | `TreeSet[K]`                     | `TreeSet` (NavigableSet semantics)                               |
| `pqueue`      | `PriorityQueue[T]`               | `PriorityQueue` (binary heap)                                    |
| `deque`       | `ArrayDeque[T]`                  | `ArrayDeque` (FIFO queue + LIFO stack)                           |
| `bitset`      | `BitSet`                         | `BitSet` (bit vector)                                            |
| `lru`         | `LRU[K, V]`                      | `LinkedHashMap(accessOrder=true)` + `removeEldestEntry`          |

## Install

```
go get github.com/neko233-com/go-data-struct-lib
```

Requires Go 1.24+.

## Usage

### SkipList — ordered map with NavigableMap operations

```go
sl := skiplist.New[int, string]()
sl.Put(30, "c")
sl.Put(10, "a")
sl.Put(20, "b")

if k, v, ok := sl.Floor(25); ok { // greatest key <= 25
    fmt.Println(k, v) // 20 b
}
if k, _, ok := sl.Higher(20); ok { // smallest key > 20
    fmt.Println(k) // 30
}
for k, v := range sl.Range(10, 30, true, false) { // [10, 30)
    fmt.Println(k, v)
}
```

Custom ordering: `skiplist.NewFunc[K, V](compare)`.

### Sets

```go
hs := set.NewHashSet(1, 2, 3)
hs.Union(set.NewHashSet(3, 4))          // {1, 2, 3, 4}
hs.Intersection(set.NewHashSet(2, 3, 9)) // {2, 3}

lhs := set.NewLinkedHashSet("a", "b", "c") // iteration order: a, b, c

ts := set.NewTreeSet(50, 10, 30)
ts.Ceiling(20) // 30
ts.Higher(30)  // 50
for k := range ts.All() { // ascending
    fmt.Println(k)
}
```

### PriorityQueue

```go
q := pqueue.New[int]() // min-heap on natural order
q.Push(3)
q.Push(1)
v, _ := q.Pop() // 1

maxQ := pqueue.NewFunc(func(a, b int) int { return b - a }) // max-heap
```

### ArrayDeque — queue, stack or both ends

```go
d := deque.New[int]()
d.PushBack(2)  // queue: offer
d.PushFront(1)
d.PopFront() // 1 (queue: poll)
d.PopBack()  // 2 (stack: pop)
```

### BitSet

```go
b := bitset.New()
b.Set(1000)
b.Cardinality()  // 1
b.NextSetBit(0)  // 1000
b.Flip(1000)
b.IsEmpty()      // true
```

### LRU cache

```go
cache := lru.New[string, []byte](128)
cache.Put("key", data)
if v, ok := cache.Get("key"); ok { // promotes to most-recently-used
    use(v)
}
// Peek/Contains inspect without touching the recency order
if ek, ev, evicted := cache.Put("other", data2); evicted {
    log.Printf("evicted %s (%d bytes)", ek, len(ev))
}
```

## Design notes

- **Generics first.** All containers are generic; ordered containers use
  `cmp.Ordered` for natural ordering and accept a `Comparator` for custom
  ordering (same contract as `java.util.Comparator`).
- **Iterators.** Containers expose `All()` / `Range(...)` returning
  `iter.Seq` / `iter.Seq2`, usable with `range` and early `break`.
- **Not concurrency-safe.** Like most `java.util` collections, instances
  are unsynchronized; guard them externally when shared.

## License

[MIT](LICENSE)
