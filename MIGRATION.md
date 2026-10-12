# Migrating to v0.4.0

This is a breaking release. Update both the dependency and callers; old aliases
and constructors are removed rather than retained as wrappers.

## Collection names

| Previous root API | v0.4.0 root API |
| --- | --- |
| Concrete `Set`, `NewSet`, `NewSetWithCapacity` | `HashSet`, `NewHashSet`, `NewHashSetWithCapacity` |
| `LinkedSet`, `NewLinkedSet` | `LinkedHashSet`, `NewLinkedHashSet` |
| `SortedSet`, `NewSortedSet`, `NewSortedSetFunc` | `SkipListSet`, `NewSkipListSet`, `NewSkipListSetFunc` |
| `SkipList`, `NewSkipList`, `NewSkipListFunc` | `SkipListMap`, `NewSkipListMap`, `NewSkipListMapFunc` |
| `TreeMap`, `NewTreeMap`, `NewTreeMapFunc` | Use the actual `SkipListMap` implementation and constructors |
| `LinkedMap`, `NewLinkedMap` | `LinkedHashMap`, `NewLinkedHashMap` |
| `PriorityQueue`, `NewPriorityQueue*` | `HeapPriorityQueue`, `NewHeapPriorityQueue*` |
| `Deque`, `NewDeque` | `ArrayDeque`, `NewArrayDeque` |
| `ConcurrentSet`, `NewConcurrentSet*` | `ConcurrentHashSet`, `NewConcurrentHashSet*` |
| `ConcurrentMap`, `NewConcurrentMap*` | `ConcurrentHashMap`, `NewConcurrentHashMap*` |
| `ConcurrentSkipList`, `NewConcurrentSkipList*` | `ConcurrentSkipListMap`, `NewConcurrentSkipListMap*` |
| `ConcurrentQueue`, `NewConcurrentQueue` | `ArrayBlockingQueue`, `NewArrayBlockingQueue` |
| `ConcurrentPriorityQueue`, `NewConcurrentPriorityQueue` | `HeapBlockingPriorityQueue`, `NewHeapBlockingPriorityQueue` |
| `NewConcurrentPriorityQueueCapacity` | `NewHeapBlockingPriorityQueueWithCapacity` |
| `RateLimiter`, `NewRateLimiter` | `TokenBucketLimiter`, `NewTokenBucketLimiter` |

The `*` suffix above preserves constructor variants such as `Func`,
`FromSlice`, and `WithCapacity` unless a row explicitly says otherwise.

Subpackage users must also update `set.TreeSet` / `NewTreeSet*` to
`SkipListSet` / `NewSkipListSet*`, `skiplist.SkipList` to `SkipListMap`,
`linkedmap.Map` to `LinkedHashMap`, and `pqueue.PriorityQueue` to
`HeapPriorityQueue`. Their package-level `New` constructors remain idiomatic
Go constructors within the explicitly named packages.

Within `concurrent`, `Set` / `Map` / `SkipList` become `HashSet` / `HashMap` /
`SkipListMap`; constructor names follow those concrete types. `Queue` becomes
`ArrayBlockingQueue`; `PriorityQueue` becomes `HeapBlockingPriorityQueue` and
its bounded constructor is `NewHeapBlockingPriorityQueueWithCapacity`.

## Depend on the Set contract

`struct233.Set[T]` now aliases the `set.Set[T]` interface. Do not use a pointer
to this interface. Constructors still return concrete pointers:

```go
func hasAll(ids struct233.Set[int64], required ...int64) bool {
    return ids.ContainsAll(required...)
}

ids := struct233.NewHashSetWithCapacity[int64](128, 10, 20)
hasAll(ids, 10, 20)
```

`NavigableSet[T]` adds `First`, `Last`, `Floor`, `Ceiling`, `Lower`, `Higher`,
and `Range`. A `SkipListSet` with a comparator also supports non-comparable
element types.

`HashSet` and `SkipListSet` algebra accept the `Set` interface and return the
receiver's concrete set style. Operands must agree on element equality.
`LinkedHashSet.Equal` now compares membership independent of insertion order;
use `slices.Equal(a.ToSlice(), b.ToSlice())` for order-sensitive comparison.

## Corrected edge behavior

- Expired `TTLCache.Peek` / `Contains` entries are removed immediately.
- Non-finite Bloom-filter rates, random weights, token rates, and retry
  settings panic. Bloom-filter bit counts and multiset totals reject overflow.
- Weighted picks never select a zero-weight trailing entry through float rounding.
- Semaphore cancellation wakes eligible waiters, and closed blocking queues
  cannot accept an enqueue after a waiting producer wakes.
- `Breaker.Execute` records panics as failures and ignores completions from a
  previous circuit generation. Prefer it over manual feedback for concurrent work.
- Invalidating memoized values and cancelling timers release their references.
- Timing-wheel timestamps and rounded tick boundaries must fit in int64
  Unix nanoseconds; out-of-range dates panic before changing pending state.
