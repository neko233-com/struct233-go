// Package struct233 is the unified facade of struct233-go: all containers,
// single-threaded and goroutine-safe, are reachable from a single import.
//
//	import "github.com/neko233-com/struct233-go"
//
//	s := struct233.NewHashSet(1, 2, 3)
//	q := struct233.NewHeapPriorityQueue[int]()
//	c := struct233.NewConcurrentSkipListMap[string, int]()
//
// The types below are aliases of the implementations in the subpackages
// (set, skiplist, pqueue, deque, bitset, lru, ttlcache, linkedmap, ringbuffer,
// unionfind, bloomfilter, concurrent, delayqueue, pool, uuid, snowflake,
// workerpool, ratelimit, semaphore, latch, barrier, retry, breaker, timewheel,
// random, multimap, multiset, text, verify, order, hash, rangeset, rangemap,
// table; the utility aliases are in util.go), so the facade adds no
// indirection: values returned by the factories are the concrete types and
// all of their methods stay available.
//
// The plain containers are not safe for concurrent use (like java.util);
// use the Concurrent* types for cross-goroutine sharing.
package struct233

import (
	"cmp"
	"context"
	"time"

	"github.com/neko233-com/struct233-go/barrier"
	"github.com/neko233-com/struct233-go/bitset"
	"github.com/neko233-com/struct233-go/bloomfilter"
	"github.com/neko233-com/struct233-go/breaker"
	"github.com/neko233-com/struct233-go/concurrent"
	"github.com/neko233-com/struct233-go/delayqueue"
	"github.com/neko233-com/struct233-go/deque"
	"github.com/neko233-com/struct233-go/latch"
	"github.com/neko233-com/struct233-go/linkedmap"
	"github.com/neko233-com/struct233-go/lru"
	"github.com/neko233-com/struct233-go/pool"
	"github.com/neko233-com/struct233-go/pqueue"
	"github.com/neko233-com/struct233-go/random"
	"github.com/neko233-com/struct233-go/ratelimit"
	"github.com/neko233-com/struct233-go/retry"
	"github.com/neko233-com/struct233-go/ringbuffer"
	"github.com/neko233-com/struct233-go/semaphore"
	"github.com/neko233-com/struct233-go/set"
	"github.com/neko233-com/struct233-go/skiplist"
	"github.com/neko233-com/struct233-go/snowflake"
	"github.com/neko233-com/struct233-go/timewheel"
	"github.com/neko233-com/struct233-go/ttlcache"
	"github.com/neko233-com/struct233-go/unionfind"
	"github.com/neko233-com/struct233-go/uuid"
	"github.com/neko233-com/struct233-go/workerpool"
)

// Comparator orders two keys: negative if a < b, zero if a == b, and
// positive if a > b (same contract as java.util.Comparator).
type Comparator[K any] = skiplist.Comparator[K]

// Container aliases, mirroring the JDK collection names.
type (
	// Set is the common collection contract implemented by each set style.
	Set[T any] = set.Set[T]
	// NavigableSet adds ordered boundary and range queries to Set.
	NavigableSet[T any] = set.NavigableSet[T]
	// HashSet is an unordered hash set (java.util.HashSet).
	HashSet[T comparable] = set.HashSet[T]
	// SkipListSet is a skip-list backed sorted set (java.util.TreeSet).
	SkipListSet[T any] = set.SkipListSet[T]
	// LinkedHashSet keeps insertion order (java.util.LinkedHashSet).
	LinkedHashSet[T comparable] = set.LinkedHashSet[T]
	// SkipListMap is an ordered map with NavigableMap operations
	// (java.util.concurrent.ConcurrentSkipListMap, single-threaded variant).
	SkipListMap[K any, V any] = skiplist.SkipListMap[K, V]
	// LinkedHashMap preserves insertion order (java.util.LinkedHashMap).
	LinkedHashMap[K comparable, V any] = linkedmap.LinkedHashMap[K, V]
	// HeapPriorityQueue is a binary-heap priority queue (java.util.PriorityQueue).
	HeapPriorityQueue[T any] = pqueue.HeapPriorityQueue[T]
	// ArrayDeque is a circular-array double-ended queue (java.util.ArrayDeque).
	ArrayDeque[T any] = deque.ArrayDeque[T]
	// RingBuffer keeps the newest N elements, overwriting the oldest.
	RingBuffer[T any] = ringbuffer.RingBuffer[T]
	// BitSet is a growable bit vector (java.util.BitSet).
	BitSet = bitset.BitSet
	// LRU is a fixed-capacity least-recently-used cache.
	LRU[K comparable, V any] = lru.LRU[K, V]
	// TTLCache is a goroutine-safe LRU cache with per-entry expiry.
	TTLCache[K comparable, V any] = ttlcache.Cache[K, V]
	// UnionFind partitions the indexes 0..N-1 into disjoint components.
	UnionFind = unionfind.Set
	// BloomFilter is a probabilistic set with no false negatives.
	BloomFilter = bloomfilter.BloomFilter

	// ConcurrentHashSet is a goroutine-safe unordered set.
	ConcurrentHashSet[T comparable] = concurrent.HashSet[T]
	// ConcurrentHashMap is a goroutine-safe map with an atomic GetOrPut.
	ConcurrentHashMap[K comparable, V any] = concurrent.HashMap[K, V]
	// ConcurrentSkipListMap is a goroutine-safe ordered map
	// (java.util.concurrent.ConcurrentSkipListMap).
	ConcurrentSkipListMap[K any, V any] = concurrent.SkipListMap[K, V]
	// ConcurrentLRU is a goroutine-safe LRU cache.
	ConcurrentLRU[K comparable, V any] = concurrent.LRU[K, V]
	// ArrayBlockingQueue is a bounded blocking FIFO queue
	// (java.util.concurrent.ArrayBlockingQueue).
	ArrayBlockingQueue[T any] = concurrent.ArrayBlockingQueue[T]
	// HeapBlockingPriorityQueue is a blocking priority queue whose head is
	// always the smallest element per the comparator
	// (java.util.concurrent.PriorityBlockingQueue). The plain HeapPriorityQueue
	// is single-threaded; this one is safe for cross-goroutine use.
	HeapBlockingPriorityQueue[T any] = concurrent.HeapBlockingPriorityQueue[T]
)

// Utility aliases: the everyday primitives that these containers are usually
// surrounded by. They are not JDK collections, but they share the same
// "one import" story.
type (
	// ObjectPool recycles short-lived allocations (bounded, goroutine-safe).
	ObjectPool[T any] = pool.Pool[T]
	// ObjectPoolStats reports the counters of an ObjectPool.
	ObjectPoolStats = pool.Stats
	// UUID is a 128-bit identifier (RFC 9562), comparable and sortable.
	UUID = uuid.UUID
	// UUIDGenerator issues strictly increasing version 7 UUIDs.
	UUIDGenerator = uuid.Generator
	// Snowflake generates 64-bit, cluster-unique, time-ordered IDs.
	Snowflake = snowflake.Generator
	// SnowflakeID is a 64-bit snowflake identifier.
	SnowflakeID = snowflake.ID
	// WorkerPool runs tasks on a fixed set of worker goroutines
	// (java.util.concurrent.ThreadPoolExecutor).
	WorkerPool = workerpool.Pool
	// WorkerPoolTask is a unit of work submitted to a WorkerPool.
	WorkerPoolTask = workerpool.Task
	// WorkerPoolMetrics is a snapshot of a WorkerPool's counters.
	WorkerPoolMetrics = workerpool.Metrics
	// TokenBucketLimiter is a goroutine-safe token-bucket limiter.
	TokenBucketLimiter = ratelimit.Limiter
	// SlidingWindowLimiter admits at most Limit events in any Window.
	SlidingWindowLimiter = ratelimit.SlidingWindow
	// DelayQueue is an unbounded blocking queue whose elements become
	// available when their deadline passes (java.util.concurrent.DelayQueue).
	DelayQueue[T any] = delayqueue.Queue[T]
	// DelayElement is a queued value with the deadline it becomes available at.
	DelayElement[T any] = delayqueue.Element[T]
	// Latch is a count-down latch (java.util.concurrent.CountDownLatch).
	Latch = latch.Latch
	// Barrier is a cyclic barrier (java.util.concurrent.CyclicBarrier).
	Barrier = barrier.Barrier
	// RetryPolicy is a reusable retry configuration with exponential backoff.
	RetryPolicy = retry.Policy
	// Semaphore is a weighted counting semaphore
	// (java.util.concurrent.Semaphore).
	Semaphore = semaphore.Semaphore
	// Breaker is a circuit breaker with closed, open and half-open states.
	Breaker = breaker.Breaker
	// BreakerState is the circuit state of a Breaker.
	BreakerState = breaker.State
	// BreakerCounts is a snapshot of a Breaker's counters.
	BreakerCounts = breaker.Counts
	// Random draws random values from a source, for the helpers that need an
	// explicit generator (or a deterministic one in tests).
	Random = random.Generator
	// RandomSource is the random source a Random generator draws from.
	RandomSource = random.Rand
	// WeightedTable is a precomputed weighted picker whose Pick does not
	// allocate.
	WeightedTable[T any] = random.WeightedTable[T]
	// TimeWheel is a hierarchical timing wheel for scheduling callbacks.
	TimeWheel = timewheel.Wheel
	// TimeWheelTimer is a scheduled callback in a TimeWheel.
	TimeWheelTimer = timewheel.Timer
)

// Breaker states, mirroring their subpackage constants.
const (
	// BreakerClosed lets every call through and counts failures.
	BreakerClosed = breaker.StateClosed
	// BreakerOpen refuses every call until the open timeout elapses.
	BreakerOpen = breaker.StateOpen
	// BreakerHalfOpen admits a limited number of probe calls.
	BreakerHalfOpen = breaker.StateHalfOpen
)

// Option aliases, one per configurable utility. Each package has its own
// WithClock-style helper, so the facade prefixes them to stay unambiguous.
type (
	// ObjectPoolOption configures an ObjectPool.
	ObjectPoolOption = pool.Option
	// UUIDOption configures a UUIDGenerator.
	UUIDOption = uuid.Option
	// SnowflakeOption configures a Snowflake generator.
	SnowflakeOption = snowflake.Option
	// WorkerPoolOption configures a WorkerPool.
	WorkerPoolOption = workerpool.Option
	// RateLimitOption configures a TokenBucketLimiter.
	RateLimitOption = ratelimit.Option
	// RetryOption configures a RetryPolicy.
	RetryOption = retry.Option
	// BreakerOption configures a Breaker.
	BreakerOption = breaker.Option
	// TimeWheelOption configures a TimeWheel.
	TimeWheelOption = timewheel.Option
)

// NilUUID is the all-zero UUID.
var NilUUID = uuid.Nil

// Sentinel errors, aliased from their subpackages so a facade-level call can be
// compared with errors.Is without a second import.
var (
	// ErrInvalidUUID is returned when UUID text or binary input is malformed.
	ErrInvalidUUID = uuid.ErrInvalidFormat
	// ErrClockBackwards is returned when the snowflake clock moved backwards.
	ErrClockBackwards = snowflake.ErrClockBackwards
	// ErrWorkerPoolClosed is returned when submitting to a closed WorkerPool.
	ErrWorkerPoolClosed = workerpool.ErrClosed
	// ErrWorkerPoolQueueFull is returned by TrySubmit when the queue is full.
	ErrWorkerPoolQueueFull = workerpool.ErrQueueFull
	// ErrBreakerOpen is returned by Execute while the circuit is open.
	ErrBreakerOpen = breaker.ErrOpen
	// ErrBarrierBroken is returned to waiters of a broken barrier generation.
	ErrBarrierBroken = barrier.ErrBroken
)

// WithObjectPoolMaxSize caps how many idle objects an ObjectPool retains.
func WithObjectPoolMaxSize(max int) ObjectPoolOption {
	return pool.WithMaxSize(max)
}

// WithObjectPoolReset sets the hook that ObjectPool runs on every Put, so the
// next Get hands out a clean object.
func WithObjectPoolReset[T any](reset func(T)) ObjectPoolOption {
	return pool.WithReset(reset)
}

// WithUUIDClock sets the time source of a UUIDGenerator.
func WithUUIDClock(now func() time.Time) UUIDOption {
	return uuid.WithClock(now)
}

// WithSnowflakeEpoch sets the epoch snowflake timestamps are measured from.
func WithSnowflakeEpoch(t time.Time) SnowflakeOption {
	return snowflake.WithEpoch(t)
}

// WithSnowflakeNodeBits sets the width of the snowflake node-id field.
func WithSnowflakeNodeBits(bits uint8) SnowflakeOption {
	return snowflake.WithNodeBits(bits)
}

// WithSnowflakeClock sets the time source of a Snowflake generator.
func WithSnowflakeClock(now func() time.Time) SnowflakeOption {
	return snowflake.WithClock(now)
}

// WithWorkerPoolQueueSize sets how many tasks a WorkerPool queues before
// Submit starts to block.
func WithWorkerPoolQueueSize(n int) WorkerPoolOption {
	return workerpool.WithQueueSize(n)
}

// WithWorkerPoolPanicHandler sets the function a WorkerPool calls with the
// recovered value when a task panics.
func WithWorkerPoolPanicHandler(fn func(recovered any)) WorkerPoolOption {
	return workerpool.WithPanicHandler(fn)
}

// WithRateLimitClock sets the time source a TokenBucketLimiter refills from.
func WithRateLimitClock(now func() time.Time) RateLimitOption {
	return ratelimit.WithClock(now)
}

// WithRetryMaxAttempts sets the total number of attempts, including the first.
func WithRetryMaxAttempts(n int) RetryOption {
	return retry.WithMaxAttempts(n)
}

// WithRetryBaseDelay sets the delay before the second attempt.
func WithRetryBaseDelay(d time.Duration) RetryOption {
	return retry.WithBaseDelay(d)
}

// WithRetryMaxDelay caps the backoff delay.
func WithRetryMaxDelay(d time.Duration) RetryOption {
	return retry.WithMaxDelay(d)
}

// WithRetryMultiplier sets the exponential growth factor of the backoff.
func WithRetryMultiplier(f float64) RetryOption {
	return retry.WithMultiplier(f)
}

// WithRetryJitter sets how much the backoff delay is randomised in [1-j, 1+j].
func WithRetryJitter(j float64) RetryOption {
	return retry.WithJitter(j)
}

// WithRetrySleep replaces the real backoff wait, which keeps tests
// deterministic.
func WithRetrySleep(fn func(ctx context.Context, d time.Duration) error) RetryOption {
	return retry.WithSleep(fn)
}

// WithRetryIf sets the predicate that decides whether an error is worth
// retrying.
func WithRetryIf(pred func(err error) bool) RetryOption {
	return retry.RetryIf(pred)
}

// WithRetryOnRetry registers an observer called before each backoff wait.
func WithRetryOnRetry(fn func(attempt int, err error, delay time.Duration)) RetryOption {
	return retry.WithOnRetry(fn)
}

// WithBreakerFailureThreshold sets how many consecutive failures open the
// circuit.
func WithBreakerFailureThreshold(n int) BreakerOption {
	return breaker.WithFailureThreshold(n)
}

// WithBreakerSuccessThreshold sets how many half-open successes close the
// circuit again.
func WithBreakerSuccessThreshold(n int) BreakerOption {
	return breaker.WithSuccessThreshold(n)
}

// WithBreakerOpenTimeout sets how long the circuit stays open before probes are
// allowed.
func WithBreakerOpenTimeout(d time.Duration) BreakerOption {
	return breaker.WithOpenTimeout(d)
}

// WithBreakerHalfOpenMaxCalls sets how many half-open probes may run at once.
func WithBreakerHalfOpenMaxCalls(n int) BreakerOption {
	return breaker.WithHalfOpenMaxCalls(n)
}

// WithBreakerClock sets the time source a Breaker measures the open timeout
// with.
func WithBreakerClock(now func() time.Time) BreakerOption {
	return breaker.WithClock(now)
}

// WithTimeWheelClock sets the time source a TimeWheel reads in Tick and Run.
func WithTimeWheelClock(now func() time.Time) TimeWheelOption {
	return timewheel.WithClock(now)
}

// WithTimeWheelLevels sets how many hierarchy levels a TimeWheel uses.
func WithTimeWheelLevels(levels int) TimeWheelOption {
	return timewheel.WithLevels(levels)
}

// NewHashSet creates an unordered hash set, optionally pre-filled.
func NewHashSet[T comparable](items ...T) *HashSet[T] {
	return set.NewHashSet(items...)
}

// NewHashSetWithCapacity creates an unordered hash set whose map is pre-sized
// for capacity elements, avoiding rehashing while filling it.
func NewHashSetWithCapacity[T comparable](capacity int, items ...T) *HashSet[T] {
	return set.NewHashSetWithCapacity(capacity, items...)
}

// NewSkipListSet creates a sorted set ordered by the natural ordering of T.
func NewSkipListSet[T cmp.Ordered](items ...T) *SkipListSet[T] {
	return set.NewSkipListSet(items...)
}

// NewSkipListSetFunc creates a sorted set ordered by a custom comparator.
func NewSkipListSetFunc[T any](compare Comparator[T], items ...T) *SkipListSet[T] {
	return set.NewSkipListSetFunc(compare, items...)
}

// NewLinkedHashSet creates a set that keeps insertion order.
func NewLinkedHashSet[T comparable](items ...T) *LinkedHashSet[T] {
	return set.NewLinkedHashSet(items...)
}

// NewLinkedHashSetWithCapacity creates an insertion-ordered hash set with a
// map pre-sized for capacity elements.
func NewLinkedHashSetWithCapacity[T comparable](capacity int, items ...T) *LinkedHashSet[T] {
	return set.NewLinkedHashSetWithCapacity(capacity, items...)
}

// NewSkipListMap creates an ordered map ordered by the natural ordering of K.
func NewSkipListMap[K cmp.Ordered, V any]() *SkipListMap[K, V] {
	return skiplist.New[K, V]()
}

// NewSkipListMapFunc creates an ordered map ordered by a custom comparator.
func NewSkipListMapFunc[K any, V any](compare Comparator[K]) *SkipListMap[K, V] {
	return skiplist.NewFunc[K, V](compare)
}

// NewLinkedHashMap creates a map that preserves insertion order.
func NewLinkedHashMap[K comparable, V any]() *LinkedHashMap[K, V] {
	return linkedmap.New[K, V]()
}

// NewHeapPriorityQueue creates a min-queue ordered by the natural ordering of T.
func NewHeapPriorityQueue[T cmp.Ordered]() *HeapPriorityQueue[T] {
	return pqueue.New[T]()
}

// NewHeapPriorityQueueFunc creates a queue ordered by a custom comparator.
func NewHeapPriorityQueueFunc[T any](compare func(a, b T) int) *HeapPriorityQueue[T] {
	return pqueue.NewFunc(compare)
}

// NewHeapPriorityQueueFromSlice builds a queue with all items in O(n).
func NewHeapPriorityQueueFromSlice[T any](items []T, compare func(a, b T) int) *HeapPriorityQueue[T] {
	return pqueue.NewFromSlice(items, compare)
}

// NewArrayDeque creates an empty double-ended queue. An optional initial
// capacity may be given.
func NewArrayDeque[T any](initialCapacity ...int) *ArrayDeque[T] {
	return deque.New[T](initialCapacity...)
}

// NewRingBuffer creates a ring buffer of the given capacity (> 0) that keeps
// the newest elements and overwrites the oldest once full.
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	return ringbuffer.New[T](capacity)
}

// NewBitSet creates an empty bit vector.
func NewBitSet() *BitSet {
	return bitset.New()
}

// NewLRU creates an LRU cache with the given capacity (> 0).
func NewLRU[K comparable, V any](capacity int) *LRU[K, V] {
	return lru.New[K, V](capacity)
}

// NewConcurrentHashSet creates a goroutine-safe set, optionally pre-filled.
func NewConcurrentHashSet[T comparable](items ...T) *ConcurrentHashSet[T] {
	return concurrent.NewHashSet(items...)
}

// NewConcurrentHashSetWithCapacity creates an empty goroutine-safe set whose map
// is pre-sized.
func NewConcurrentHashSetWithCapacity[T comparable](capacity int) *ConcurrentHashSet[T] {
	return concurrent.NewHashSetWithCapacity[T](capacity)
}

// NewConcurrentHashMap creates an empty goroutine-safe map.
func NewConcurrentHashMap[K comparable, V any]() *ConcurrentHashMap[K, V] {
	return concurrent.NewHashMap[K, V]()
}

// NewConcurrentHashMapWithCapacity creates an empty goroutine-safe map pre-sized
// for capacity entries.
func NewConcurrentHashMapWithCapacity[K comparable, V any](capacity int) *ConcurrentHashMap[K, V] {
	return concurrent.NewHashMapWithCapacity[K, V](capacity)
}

// NewConcurrentSkipListMap creates a goroutine-safe ordered map using the
// natural ordering of K.
func NewConcurrentSkipListMap[K cmp.Ordered, V any]() *ConcurrentSkipListMap[K, V] {
	return concurrent.NewSkipListMap[K, V]()
}

// NewConcurrentSkipListMapFunc creates a goroutine-safe ordered map using a
// custom comparator.
func NewConcurrentSkipListMapFunc[K any, V any](compare Comparator[K]) *ConcurrentSkipListMap[K, V] {
	return concurrent.NewSkipListMapFunc[K, V](compare)
}

// NewConcurrentLRU creates a goroutine-safe LRU cache with the given
// capacity (> 0).
func NewConcurrentLRU[K comparable, V any](capacity int) *ConcurrentLRU[K, V] {
	return concurrent.NewLRU[K, V](capacity)
}

// NewArrayBlockingQueue creates a bounded blocking FIFO queue holding at most
// capacity elements (> 0).
func NewArrayBlockingQueue[T any](capacity int) *ArrayBlockingQueue[T] {
	return concurrent.NewArrayBlockingQueue[T](capacity)
}

// NewHeapBlockingPriorityQueue creates an unbounded blocking priority queue
// ordered by compare (negative when a < b, zero when equal, positive
// otherwise). Equal elements leave the queue in insertion order.
func NewHeapBlockingPriorityQueue[T any](compare func(a, b T) int) *HeapBlockingPriorityQueue[T] {
	return concurrent.NewHeapBlockingPriorityQueue[T](compare)
}

// NewHeapBlockingPriorityQueueWithCapacity creates a blocking priority queue
// holding at most capacity elements (> 0): Put blocks while the queue is
// full and Offer waits at most its timeout for space.
func NewHeapBlockingPriorityQueueWithCapacity[T any](compare func(a, b T) int, capacity int) *HeapBlockingPriorityQueue[T] {
	return concurrent.NewHeapBlockingPriorityQueueWithCapacity[T](compare, capacity)
}

// NewTTLCache creates a goroutine-safe cache with the given capacity (> 0)
// and default time-to-live (> 0).
func NewTTLCache[K comparable, V any](capacity int, ttl time.Duration) *TTLCache[K, V] {
	return ttlcache.New[K, V](capacity, ttl)
}

// NewTTLCacheWithClock creates a TTL cache reading the current time from now,
// which keeps tests deterministic.
func NewTTLCacheWithClock[K comparable, V any](capacity int, ttl time.Duration, now func() time.Time) *TTLCache[K, V] {
	return ttlcache.NewWithClock[K, V](capacity, ttl, now)
}

// NewUnionFind creates a disjoint-set union over the indexes 0..n-1.
func NewUnionFind(n int) *UnionFind {
	return unionfind.New(n)
}

// NewBloomFilter creates a probabilistic set sized for expectedItems elements
// with the given false-positive rate (0 < rate < 1).
func NewBloomFilter(expectedItems int, falsePositiveRate float64) *BloomFilter {
	return bloomfilter.New(expectedItems, falsePositiveRate)
}

// NewObjectPool creates an object pool that builds new objects with newFn.
// Reuse makes sense for objects that are expensive to allocate and cheap to
// reset, such as byte buffers or packets.
func NewObjectPool[T any](newFn func() T, opts ...ObjectPoolOption) *ObjectPool[T] {
	return pool.New(newFn, opts...)
}

// NewUUID creates a random version 4 UUID.
func NewUUID() UUID {
	return uuid.NewV4()
}

// NewUUID4 creates a random version 4 UUID.
func NewUUID4() UUID {
	return uuid.NewV4()
}

// NewUUID7 creates a strictly increasing version 7 UUID for the current time.
// Prefer it over NewUUID for keys: v7 UUIDs sort by creation time.
func NewUUID7() UUID {
	return uuid.NewV7()
}

// NewUUID7At creates a version 7 UUID carrying the given timestamp.
func NewUUID7At(t time.Time) UUID {
	return uuid.NewV7At(t)
}

// NewUUIDGenerator creates a version 7 generator, optionally with an injected
// clock.
func NewUUIDGenerator(opts ...UUIDOption) *UUIDGenerator {
	return uuid.NewGenerator(opts...)
}

// ParseUUID parses any accepted UUID text form (canonical, bare hex, urn
// prefix, braces) and reports uuid.ErrInvalidFormat otherwise.
func ParseUUID(s string) (UUID, error) {
	return uuid.Parse(s)
}

// MustParseUUID is ParseUUID, panicking on invalid input.
func MustParseUUID(s string) UUID {
	return uuid.MustParse(s)
}

// NewSnowflake creates a generator for the given node id (0 <= node <
// 1<<nodeBits, 10 bits by default). It panics on invalid options.
func NewSnowflake(node int64, opts ...SnowflakeOption) *Snowflake {
	return snowflake.New(node, opts...)
}

// NewWorkerPool creates a pool of workers goroutines (workers > 0) that run
// submitted tasks. Call Close to drain it and release the goroutines.
func NewWorkerPool(workers int, opts ...WorkerPoolOption) *WorkerPool {
	return workerpool.New(workers, opts...)
}

// NewTokenBucketLimiter creates a token-bucket limiter admitting rate events per
// second in bursts of at most burst.
func NewTokenBucketLimiter(rate float64, burst int, opts ...RateLimitOption) *TokenBucketLimiter {
	return ratelimit.New(rate, burst, opts...)
}

// NewSlidingWindowLimiter creates a limiter admitting at most limit events in
// any window, which is exact where a fixed window would burst at its boundary.
func NewSlidingWindowLimiter(limit int, window time.Duration, opts ...RateLimitOption) *SlidingWindowLimiter {
	return ratelimit.NewSlidingWindow(limit, window, opts...)
}

// NewDelayQueue creates an empty delay queue driven by the system clock.
func NewDelayQueue[T any]() *DelayQueue[T] {
	return delayqueue.New[T]()
}

// NewDelayQueueWithClock creates an empty delay queue reading the current time
// from now, which keeps deadline tests deterministic.
func NewDelayQueueWithClock[T any](now func() time.Time) *DelayQueue[T] {
	return delayqueue.NewWithClock[T](now)
}

// NewDelayElement wraps value with the deadline at which a delay queue may hand
// it out.
func NewDelayElement[T any](value T, deadline time.Time) *DelayElement[T] {
	return delayqueue.NewElement(value, deadline)
}

// NewLatch creates a latch that opens after count CountDown calls.
func NewLatch(count int) *Latch {
	return latch.New(count)
}

// NewBarrier creates a cyclic barrier over parties goroutines.
func NewBarrier(parties int) *Barrier {
	return barrier.New(parties)
}

// NewSemaphore creates a counting semaphore holding permits permits.
func NewSemaphore(permits int64) *Semaphore {
	return semaphore.New(permits)
}

// NewRetryPolicy creates a reusable retry policy.
func NewRetryPolicy(opts ...RetryOption) *RetryPolicy {
	return retry.New(opts...)
}

// Retry runs fn until it succeeds, the attempt budget runs out, RetryIf rejects
// the error or ctx is done, using a policy built from opts.
func Retry(ctx context.Context, fn func(ctx context.Context) error, opts ...RetryOption) error {
	return retry.Do(ctx, fn, opts...)
}

// RetryValue is Retry for a function that returns a value.
func RetryValue[T any](ctx context.Context, fn func(ctx context.Context) (T, error), opts ...RetryOption) (T, error) {
	return retry.DoValue(ctx, fn, opts...)
}

// NewBreaker creates a circuit breaker around a call that can fail.
func NewBreaker(opts ...BreakerOption) *Breaker {
	return breaker.New(opts...)
}

// NewRandom wraps the given random source. It panics when src is nil.
func NewRandom(src RandomSource) *Random {
	return random.New(src)
}

// DefaultRandom returns the process-wide random generator, backed by
// math/rand/v2's goroutine-safe global source.
func DefaultRandom() *Random {
	return random.Default()
}

// Shuffle shuffles items in place. A nil g means DefaultRandom().
func Shuffle[T any](g *Random, items []T) {
	random.Shuffle(g, items)
}

// Choice returns a random element of items; ok is false when items is empty.
// A nil g means DefaultRandom().
func Choice[T any](g *Random, items []T) (T, bool) {
	return random.Choice(g, items)
}

// Sample returns up to n distinct elements of items in random order. A nil g
// means DefaultRandom().
func Sample[T any](g *Random, items []T, n int) []T {
	return random.Sample(g, items, n)
}

// Weighted returns an element picked proportionally to weight; ok is false when
// items is empty or every weight is <= 0. A nil g means DefaultRandom().
func Weighted[T any](g *Random, items []T, weight func(T) float64) (T, bool) {
	return random.Weighted(g, items, weight)
}

// NewWeightedTable builds a reusable weighted picker (drop tables, spawn
// tables). A nil g means DefaultRandom().
func NewWeightedTable[T any](g *Random, items []T, weight func(T) float64) *WeightedTable[T] {
	return random.NewWeightedTable(g, items, weight)
}

// NewTimeWheel creates a timing wheel with the given tick resolution (> 0) and
// slots per level (>= 2). Drive it with Advance/AdvanceBy/Tick, or with Run.
func NewTimeWheel(tick time.Duration, wheelSize int, opts ...TimeWheelOption) *TimeWheel {
	return timewheel.New(tick, wheelSize, opts...)
}
