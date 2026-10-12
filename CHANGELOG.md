# Changelog

All notable changes to this project are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.4.0] - 2026-10-12

### Changed

- Breaking collection naming: explicit `HashSet`, `LinkedHashSet`,
  `SkipListSet`, `SkipListMap`, `LinkedHashMap`, `HeapPriorityQueue`,
  `ArrayDeque`, concrete concurrent variants, and `TokenBucketLimiter`.
  Removed the misleading TreeMap aliases and old constructors without
  compatibility wrappers; see `MIGRATION.md`.
- `Set[T]` is a shared interface implemented by all four set styles;
  `NavigableSet[T]` adds ordered queries. Set algebra accepts different
  implementations with compatible equality. `LinkedHashSet.Equal` compares
  members independently of insertion order.

### Fixed

- Semaphore head-waiter cancellation and release overflow; blocking queue
  enqueue-after-close races; delay queue FIFO when an Element is reused.
- Timing-wheel cancellation/Stop reference retention, negative timestamps,
  and duration/hierarchy overflow.
- Circuit-breaker panic accounting and stale completions across state resets.
- Retry cancellation between attempts and delay overflow; token-bucket clock
  rollback, non-finite rates, and sub-nanosecond reservation rounding.
- Bloom-filter NaN and index overflow; weighted-random non-finite inputs and
  zero-weight selection at subnormal totals; multiset total overflow.
- TTL Peek/Contains expiry cleanup; stale references in skip lists, range
  sets, hash rings, memoized suppliers, and timers; supplier clock-panic locking.

### Performance and validation

- RangeSet algebra uses ordered scans; RangeMap writes preserve order without
  per-entry temporary sets or full sorting. Multiset TotalCount is O(1).
- UUID entropy is fetched in a bounded crypto/rand buffer; UUID generation
  remains allocation-free on Go 1.24 including Linux race builds.
- Added regression, shared-interface, differential, concurrent, allocation,
  and benchmark coverage. CI tests Go 1.24.x and the current stable toolchain
  with race detection, vet, build, formatting and benchmark smoke checks.

## [0.3.0] - 2026-10-12

### Added

- `table`: Guava-style two-key table with row and column lookups, stable cell
  insertion order, row/column removal, nested map snapshots, and a usable zero
  value. Root facade aliases and capacity constructors are included.
- `rangemap`: Guava-style interval-to-value map built on `rangeset.Range`.
  `Put` and `Remove` split old intervals at exact open/closed boundaries;
  lookup uses binary search, and `GetEntry`, `SubRangeMap`, `Span`, ordered
  snapshots and `AsMapOfRanges` are available from both the package and facade.

- `pool`: bounded, goroutine-safe LIFO object pool with an optional reset hook,
  a maximum idle-set size, `Prealloc` warm-up and reuse counters (`Stats`,
  `ReuseRate`) — a typed replacement for `sync.Pool` that cannot grow without
  bound.
- `uuid`: dependency-free RFC 9562 identifiers. Version 4 (random) and version
  7 (time-ordered) generation, a monotonic `Generator` with an injectable
  clock, `Parse` (canonical, bare hex, `urn:uuid:`, braced, any case),
  `String`/`AppendText`, `MarshalText`/`UnmarshalText` (JSON values and keys),
  `MarshalBinary`/`UnmarshalBinary`, `Version`, `Time`, `Compare`, `Nil`, `Max`
  and `ErrInvalidFormat`.
- `snowflake`: 64-bit time-ordered ID generator with a configurable epoch,
  configurable node/sequence bit split, injectable clock, sequence rollover
  that waits for the next millisecond, `ErrClockBackwards` on a backwards
  clock, and `TimeOf`/`NodeOf`/`SequenceOf` decoders.
- `workerpool`: fixed-goroutine pool with `Submit` (backpressure),
  `TrySubmit` (`ErrQueueFull`), `SubmitWait`, idempotent draining `Close`,
  panic isolation with an optional handler, and a `Metrics` snapshot.
- `ratelimit`: goroutine-safe token bucket with `Allow`, `AllowN`, `Reserve`,
  `Wait(ctx)`, `Tokens` and an injectable clock, so refill is deterministic in
  tests.
- Facade aliases, factories and option helpers for all of the above:
  `ObjectPool`, `UUID`, `UUIDGenerator`, `Snowflake`, `SnowflakeID`,
  `WorkerPool`, `RateLimiter`, `NewObjectPool`, `NewUUID`, `NewUUID4`,
  `NewUUID7`, `NewUUID7At`, `NewUUIDGenerator`, `ParseUUID`, `MustParseUUID`,
  `NewSnowflake`, `NewWorkerPool`, `NewRateLimiter`, `NilUUID` and the
  `WithObjectPool*` / `WithUUIDClock` / `WithSnowflake*` / `WithWorkerPool*` /
  `WithRateLimitClock` option helpers.
- Fuzz targets for the UUID text and binary codecs, alongside the existing
  container fuzzers.
- Godoc coverage check: 435 exported symbols, 0 undocumented.

### Added (second batch)

- `delayqueue`: unbounded blocking delay queue
  (`java.util.concurrent.DelayQueue`), heap-ordered on per-element deadlines
  with a FIFO tie-break, `Take`/`Poll`/`TryTake`/`Peek`/`Drain`, an injectable
  clock and `Close`.
- `semaphore`: weighted counting semaphore (`java.util.concurrent.Semaphore`)
  with context-aware `Acquire`/`AcquireN`, `TryAcquire`/`TryAcquireN`,
  `AcquireTimeout`, FIFO waiter queueing and `Permits`/`Limit`/`Waiters`.
- `latch`: count-down latch (`java.util.concurrent.CountDownLatch`) with
  `CountDown`, context-aware `Await`, `AwaitTimeout` and an allocation-free
  `Open` fast path.
- `barrier`: cyclic barrier (`java.util.concurrent.CyclicBarrier`) with arrival
  indexes, generation-safe breakage (`ErrBroken`), `Reset`, `Waiting` and
  `Broken`.
- `retry`: exponential backoff with jitter, `RetryIf`, `WithOnRetry`, a
  reusable `Policy`, `Do`/`DoValue` and an injectable sleep hook, so retry
  behaviour is testable without sleeping.
- `breaker`: circuit breaker with closed/open/half-open states, failure and
  success thresholds, an open timeout, limited half-open probes, `Execute`,
  `Counts` and an injectable clock.
- `timewheel`: hierarchical timing wheel with O(1) `Schedule`/`Cancel`,
  `Advance`/`AdvanceBy`/`Tick` (injectable clock, deterministic) and a
  real-time `Run(ctx)` driver; callbacks run outside the wheel lock.
- `random`: bounded ints, random strings/digits, `Shuffle`/`Choice`/`Sample`,
  an O(n) weighted pick and an allocation-free precomputed `WeightedTable`,
  with a deterministic `Generator` over an injectable `Rand` source.
- `ratelimit`: added `SlidingWindow`, an exact sliding-window counter limiter
  (`Allow`/`AllowN`/`Remaining`/`RetryAfter`/`Wait`/`Reset`), and reworked
  `Option` so both limiters share the injectable clock.
- Facade aliases and factories for the batch: `DelayQueue`, `DelayElement`,
  `Latch`, `Barrier`, `SlidingWindowLimiter`, `Semaphore`, `RetryPolicy`,
  `Breaker`, `TimeWheel`, `Random` and their `New*` constructors, plus the
  matching `With*` option helpers. Sentinel errors are re-exported by the
  facade (`ErrInvalidUUID`, `ErrClockBackwards`, `ErrWorkerPoolClosed`,
  `ErrWorkerPoolQueueFull`, `ErrBreakerOpen`, `ErrBarrierBroken`).
- Godoc coverage check: 640 exported symbols, 0 undocumented.

### Added (Guava-inspired batch)

- `multimap`: `ListMultimap` and `SetMultimap`
  (`com.google.common.collect`), both keeping key and value insertion order,
  with `Put`/`PutAll`/`Get`/`Remove`/`RemoveAll`/`Replace`/`ContainsEntry`/
  `Entries`/`AsMap`/`Invert` and an `iter.Seq2` view; the capacity constructors
  keep reserved, value-less key slots. The value type is `comparable` so that
  `Invert` can swap the roles.
- `multiset`: counting bag (`com.google.common.collect.Multiset`) with
  `Add`/`AddN`/`Remove`/`RemoveN`/`RemoveAll`/`SetCount`/`Count`/
  `TotalCount`/`Distinct`/`Elements`/`Counts`/`MostCommon`/`Equal`, keeping
  first-insertion order so every observation is deterministic.
- `text`: ergonomic string helpers — `Join`/`JoinSlice`/`JoinAny`/`JoinMap`,
  the fluent `Joiner` (skip or substitute empty values) and `Splitter` (trim,
  omit empty, limit), `Repeat`/`PadStart`/`PadEnd`/`Truncate`, the blank/empty
  predicates, and the four case conversions (lower/upper camel, snake, kebab)
  with one documented word-splitting rule.
- `verify`: precondition checks (`True`/`False`/`State`/`NotNil`/`Nil`/`Index`/
  `Position`/`InRange`/`Positive`/`NonNegative`) in the spirit of
  `com.google.common.base.Preconditions`; every failure panics with an error
  wrapping `ErrInvalidArgument` or `ErrInvalidState`, so `errors.Is` still works
  after recovering, and a passing check without arguments does not allocate.
- `hash`: `Hasher` plus MurmurHash3 (32 and x64 128), FNV-1a 32/64, CRC-32,
  CRC-32C and Adler-32, with the standard-library-backed ones cross-checked
  against `hash/fnv`, `hash/crc32` and `hash/adler32` in the tests.
- `order`: comparator combinators in the spirit of
  `com.google.common.collect.Ordering` — `Natural`, `Reverse`, `Then`
  (lexicographic), `NullsFirst`/`NullsLast`, `ByKey`, the `By` less-function
  adapter, `Max`/`Min`/`MaxOf`/`MinOf`, and a stable `Sorted` plus `IsSorted`.
  `Comparator` is an alias of the container type, so a combinator plugs straight
  into `NewSortedSetFunc`, `NewSkipListFunc` and `NewPriorityQueueFunc`.
- `hashring`: consistent hash ring with virtual nodes — `Add`/`Remove`/`Get`/
  `GetN`/`Nodes`/`Replicas`/`Len`, an O(log n) binary-search lookup, a
  goroutine-safe circle, and the consistent-hashing guarantee (adding or
  removing a node moves only the keys it owns, and every key that moves moves to
  the new node). The tests also show that FNV-1a distributes keys too unevenly
  for a ring, so the default hasher is MurmurHash3 (`hash.Sum64`); a key is
  hashed without copying, which keeps `Get` allocation-free.
- Deep test suites for the batch: every package ships 40+ table-driven cases
  with subtests, model-based differential tests against plain reference
  implementations, fuzz targets replaying randomised operations (and, for
  `text`, property targets for the text codecs), race-clean concurrency tests,
  allocation guards and runnable examples. The `text` fuzz targets found and
  pinned down two real edge cases: `ToSnakeCase` is not idempotent for invalid
  UTF-8 or for runes that are upper case without a lower-case mapping (it
  converges on the second pass), and camelCase cannot carry the word boundary
  between adjacent capitalised words.
- Facade aliases and factories for the batch in `util.go`: `Multiset`,
  `NewMultiset`, `NewMultisetWithCapacity`, `Joiner`/`Splitter` (+
  `NewJoiner`/`NewSplitter`), `JoinStrings`/`JoinStringsAny`/`JoinStringMap`/
  `SplitString`/`RepeatString`/`PadStart`/`PadEnd`/`Truncate`, the blank
  predicates, the case conversions, `Hasher` with the hash functions, the
  `Require*` precondition wrappers, and the `ErrInvalidArgument`/
  `ErrInvalidState` sentinels.
- Godoc coverage of the new packages: every exported symbol carries a doc
  comment starting with its name (the AST check counts struct fields too).

### Added (interval algebra batch)

- `rangeset`: intervals and interval sets over ordered types
  (com.google.common.collect.Range / RangeSet). A `Range` has closed, open or
  unbounded ends expressed as flags rather than sentinel values, so every
  ordered type (strings included) works; the constructors cover the usual
  vocabulary (`Closed`, `Open`, `OpenClosed`, `ClosedOpen`, `AtLeast`, `AtMost`,
  `GreaterThan`, `LessThan`, `Singleton`, `All`) and `IsEmpty`/`Contains`/
  `Encloses`/`IsConnected`/`Intersection`/`Span`/`Equal` do the interval
  algebra. A `RangeSet` stores its values as disjoint, ascending, coalesced
  spans: `Add` merges closed-touching ranges, `Remove` cuts spans apart,
  membership is a binary search, and `Encloses`/`Intersects`/`Intersection`/
  `Union`/`Difference`/`Complement` do the set algebra. `Len` counts spans, so
  unbounded sets stay finite.
- Facade aliases for the batch: `Range`, `RangeSet`, `NewRangeSet` and the
  `Range*` constructor wrappers (`RangeClosed` … `RangeAll`) — generic
  functions cannot be aliased, so the constructors are thin wrappers.
- Test suite: table-driven cases for every constructor and operation,
  invariant checks that the stored spans stay non-empty, ascending and
  disjoint, differential tests against a point-wise brute-force model, and a
  fuzzer replaying random `Add`/`Remove` traffic against the model.

### Added (fourth batch)

- `bidimap`: bidirectional map (com.google.common.collect.BiMap) keeping a
  key→value mapping and its value→key inverse in sync: every key binds at most
  one value and every value at most one key. `Put` reports a value collision as
  `false` instead of throwing, `ForcePut` overwrites both conflicting bindings,
  `GetKeyOf` does the reverse lookup, and `Inverse` returns a live swapped view
  sharing the same storage. The zero value is ready to use; `Keys`, `Values` and
  `Entries` (`iter.Seq2`) iterate in unspecified order. Tests: 40+ table-driven
  scenarios, a differential run against a reference map model, live-view
  invariant checks, a fuzz target and a read-path zero-allocation guard.
- `striped`: striped locks (com.google.common.util.concurrent.Striped) — a
  fixed-size array of independent mutexes (`Striped`) or RWMutexes (`StripedRW`)
  addressed by a caller-supplied 64-bit key hash. The stripe count is rounded up
  to a power of two so `Get`/`GetRW`/`GetAt` use a bit mask and allocate
  nothing. Tests: power-of-two rounding and panic contracts, same-key pointer
  stability, distribution checks, reference-model differential tests, two fuzz
  targets, contention tests showing same-stripe serialisation and cross-stripe
  independence, and allocation guards on every lookup path.
- `atomiclongmap`: concurrent map from keys to int64 counters
  (com.google.common.util.concurrent.AtomicLongMap). `Add`/`Increment`/
  `Decrement` on an existing key allocate nothing and never block readers;
  values may go negative and are kept at zero until `RemoveIfZero`.
  `GetOrAdd` hands out a shared counter cell, `Range` iterates a snapshot,
  `AsMap`/`Keys` return snapshots, and `Sum` totals the counters. Tests:
  table-driven cases for every operation, a reference-model differential run,
  precise concurrent-increment checks, shared-cell `GetOrAdd` races, a fuzz
  target and a hot-path zero-allocation guard.
- `stopwatch`: Guava-style stopwatch (com.google.common.base.Stopwatch) that
  accumulates every `Start`..`Stop` segment and ignores the gaps in between.
  `Restart`, `Reset`, `Elapsed`, `IsRunning`, `String`, an injectable clock
  (`WithClock`) and panics on invalid transitions ("already started",
  "not running") in the repository style where Guava would throw. Tests: a
  40+ case table-driven lifecycle suite over a fake clock, randomised and fuzz
  runs against a reference model, concurrent reader/controller tests, and an
  `Elapsed` zero-allocation guard.
- `memoize`: lazy memoizing suppliers
  (com.google.common.base.Suppliers.memoize / memoizeWithExpiration).
  `Memoize` computes the value exactly once (singleflight: concurrent first
  calls run `fn` once), `MemoizeExpiring` recomputes it after a TTL, a
  panicking `fn` is never cached, `Invalidate` forces a recomputation, and
  `Stats` reports hit/miss counters. The clock is injectable. Tests: table
  suites for the value, TTL, panic and invalidation semantics, differential
  runs against a reference model (sequential and parallel), a fuzz target and
  zero-allocation hit paths.
- `concurrent.PriorityQueue`: blocking priority queue
  (java.util.concurrent.PriorityBlockingQueue) whose head is always the
  smallest element per the comparator, with a FIFO tie-break on equal
  elements. `Put`/`Offer` block on a bounded queue, `Take`/`Poll` block while
  empty, `Close` unblocks every waiter and leaves queued values drainable.
  Includes a fix carried in this new code: on a bounded queue, a blocked
  producer is now woken as soon as a consumer frees a slot (the full-queue
  wakeup path used to leave waiters sleeping). Tests: ordering suites, slice
  model and stable-order differential tests, blocking/timeout/close
  concurrency scenarios, a fuzz target against a model and zero-allocation
  `Peek`/`Len`.
- Facade aliases, factories and option helpers for the batch: `BiMap`
  (`NewBiMap`/`NewBiMapWithCapacity`), `Striped`/`StripedRW`
  (`NewStriped`/`NewStripedRW`), `AtomicLongMap` (`NewAtomicLongMap`),
  `Stopwatch` (`NewStopwatch`/`NewStartedStopwatch`, `StopwatchOption`,
  `WithStopwatchClock`), `Supplier`/`MemoizeStats`
  (`Memoize`/`MemoizeExpiring`, `WithMemoizeClock`) and
  `ConcurrentPriorityQueue`
  (`NewConcurrentPriorityQueue`/`NewConcurrentPriorityQueueCapacity`), each
  with a runnable example.
- Full-library test hardening: 40+ table-driven suites, differential tests
  against reference models, fuzz targets, zero-allocation guards, runnable
  examples and benchmarks now cover every package, and 14 previously thin
  packages were brought up to the delivery bar.

## [0.2.0] - 2026-10-10

### Added

- `bloomfilter`: space-efficient probabilistic set with no false negatives
  (string/bytes/uint64 keys, FNV-1a + splitmix64 double hashing, tunable
  false-positive rate).
- `unionfind`: disjoint-set union with path compression and union by size
  (`Find`, `Union`, `Connected`, `Size`, `Components`).
- `ttlcache`: goroutine-safe cache that combines LRU eviction with per-entry
  time-to-live and an injectable clock (`PutWithTTL`, lazy expiry, `Cleanup`).
- `concurrent.Queue`: bounded blocking FIFO queue with `ArrayBlockingQueue`
  semantics (`Put`, `Offer` with timeout, `Take`, `Poll`, `TryPut`,
  `TryTake`, `Close`).
- Facade factories and aliases for all of the above: `NewBloomFilter`,
  `NewUnionFind`, `NewTTLCache`, `NewTTLCacheWithClock`, `NewConcurrentQueue`.
- Fuzz targets comparing `skiplist`, `bitset`, `ringbuffer` and `linkedmap`
  against reference implementations.
- Godoc coverage check: 304 exported symbols, 0 undocumented.

## [0.1.0] - 2026-10-10

First release as **struct233-go** (formerly `go-data-struct-lib`).

### Added

- Unified facade `struct233`: factory functions (`NewSet`, `NewSortedSet`,
  `NewLinkedSet`, `NewSkipList`, `NewTreeMap`, `NewLinkedMap`,
  `NewPriorityQueue`, `NewDeque`, `NewRingBuffer`, `NewBitSet`, `NewLRU`,
  `NewConcurrentSet`, `NewConcurrentMap`, `NewConcurrentSkipList`,
  `NewConcurrentLRU`) plus type aliases.
- `skiplist`: ordered map with NavigableMap operations (`First`, `Last`,
  `Floor`, `Ceiling`, `Lower`, `Higher`, `Range`), generic comparator support.
- `set`: `HashSet`, `LinkedHashSet` (insertion order), `TreeSet` (skip-list
  backed, NavigableSet operations) and set algebra.
- `pqueue`: binary-heap priority queue with custom comparators and O(n)
  heapify.
- `deque`: circular-array double-ended queue (FIFO queue + LIFO stack).
- `bitset`: growable bit vector with `Cardinality`, `NextSetBit`,
  `PrevSetBit`, `And`, `Or`, `Xor`, `AndNot`.
- `lru`: fixed-capacity least-recently-used cache (`Get` promotes, `Peek`
  does not).
- `linkedmap`: insertion-ordered map (`java.util.LinkedHashMap` analog).
- `ringbuffer`: fixed-capacity ring buffer with overwrite-oldest semantics.
- `concurrent`: goroutine-safe `Set`, `Map` (atomic `GetOrPut`), `SkipList`
  and `LRU`, iterating over snapshots.
- Performance: zero-allocation read paths, per-instance scratch buffers for
  skip-list writes, in-place bit-set growth, pre-sized set algebra results.
  Benchmarks and allocation guards ship with every package.
- testify-based test suite, runnable examples, and GitHub Actions CI
  (format, vet, test with `-race`, benchmarks).

[Unreleased]: https://github.com/neko233-com/struct233-go/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/neko233-com/struct233-go/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/neko233-com/struct233-go/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/neko233-com/struct233-go/releases/tag/v0.1.0
