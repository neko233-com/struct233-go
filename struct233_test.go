package struct233_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	struct233 "github.com/neko233-com/struct233-go"
	"github.com/neko233-com/struct233-go/workerpool"
)

// errTransient stands in for a flaky dependency in the retry and breaker tests.
var errTransient = errors.New("transient failure")

// scriptedSource is a deterministic RandomSource: it hands out the queued values
// in order, so the random helpers can be asserted exactly.
type scriptedSource struct {
	mu     sync.Mutex
	ints   []int
	floats []float64
}

func (s *scriptedSource) IntN(n int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.ints[0]
	s.ints = s.ints[1:]
	if v >= n {
		v = n - 1
	}
	return v
}

func (s *scriptedSource) Float64() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.floats[0]
	s.floats = s.floats[1:]
	return v
}

func TestNewHashSet(t *testing.T) {
	s := struct233.NewHashSet(1, 2, 3)
	require.Equal(t, 3, s.Len())
	require.True(t, s.Contains(2))
	require.False(t, s.Add(3))
	require.True(t, s.Union(struct233.NewHashSet(4)).Equal(struct233.NewHashSet(1, 2, 3, 4)))

	capped := struct233.NewHashSetWithCapacity[string](32, "a", "b")
	require.Equal(t, 2, capped.Len())
	require.True(t, capped.ContainsAll("a", "b"))
}

func TestNewSkipListSet(t *testing.T) {
	ts := struct233.NewSkipListSet(3, 1, 2)
	require.Equal(t, []int{1, 2, 3}, ts.ToSlice())

	k, ok := ts.Ceiling(2)
	require.True(t, ok)
	require.Equal(t, 2, k)

	desc := struct233.NewSkipListSetFunc(func(a, b int) int { return b - a }, 1, 3, 2)
	require.Equal(t, []int{3, 2, 1}, desc.ToSlice())
}

func TestNewLinkedHashSet(t *testing.T) {
	s := struct233.NewLinkedHashSet("c", "a", "b")
	require.Equal(t, []string{"c", "a", "b"}, s.ToSlice(), "insertion order must be preserved")
}

func TestNewSkipListMap(t *testing.T) {
	sl := struct233.NewSkipListMap[int, string]()
	sl.Put(10, "a")
	sl.Put(20, "b")

	v, ok := sl.Get(20)
	require.True(t, ok)
	require.Equal(t, "b", v)

	k, _, ok := sl.Floor(15)
	require.True(t, ok)
	require.Equal(t, 10, k)

	rev := struct233.NewSkipListMapFunc[string, int](func(a, b string) int { return strings.Compare(b, a) })
	rev.Put("a", 1)
	rev.Put("c", 3)
	revKey, _, revOK := rev.First()
	require.True(t, revOK)
	require.Equal(t, "c", revKey, "custom comparator must be honoured")
}

func TestSkipListMapOrdering(t *testing.T) {
	tm := struct233.NewSkipListMap[int, string]()
	tm.Put(5, "e")
	tm.Put(1, "a")

	require.Equal(t, []int{1, 5}, tm.Keys())

	tmFunc := struct233.NewSkipListMapFunc[int, string](func(a, b int) int { return b - a })
	tmFunc.Put(1, "a")
	tmFunc.Put(5, "e")
	require.Equal(t, []int{5, 1}, tmFunc.Keys(), "reversed comparator must be honoured")
}

func TestNewLinkedHashMap(t *testing.T) {
	m := struct233.NewLinkedHashMap[string, int]()
	m.Put("b", 1)
	m.Put("a", 2)
	m.Put("b", 3) // update must not reorder

	require.Equal(t, []string{"b", "a"}, m.Keys())
	require.Equal(t, []int{3, 2}, m.Values())
}

func TestNewHeapPriorityQueue(t *testing.T) {
	q := struct233.NewHeapPriorityQueue[int]()
	q.Push(3)
	q.Push(1)
	q.Push(2)

	v, ok := q.Pop()
	require.True(t, ok)
	require.Equal(t, 1, v)

	maxQ := struct233.NewHeapPriorityQueueFunc(func(a, b int) int { return b - a })
	maxQ.Push(1)
	maxQ.Push(9)
	v, ok = maxQ.Pop()
	require.True(t, ok)
	require.Equal(t, 9, v, "custom comparator must be honoured")

	fromSlice := struct233.NewHeapPriorityQueueFromSlice([]int{5, 1, 4}, func(a, b int) int { return a - b })
	v, ok = fromSlice.Pop()
	require.True(t, ok)
	require.Equal(t, 1, v)
}

func TestNewArrayDeque(t *testing.T) {
	d := struct233.NewArrayDeque[int](16)
	d.PushBack(1)
	d.PushFront(0)
	d.PushBack(2)

	require.Equal(t, []int{0, 1, 2}, d.ToSlice())

	back, ok := d.PopBack()
	require.True(t, ok)
	require.Equal(t, 2, back)

	require.Equal(t, []int{0, 1}, d.ToSlice())

	noCap := struct233.NewArrayDeque[string]()
	require.GreaterOrEqual(t, noCap.Cap(), 8)
}

func TestNewRingBuffer(t *testing.T) {
	r := struct233.NewRingBuffer[int](3)
	require.Equal(t, 3, r.Cap())

	r.Push(1)
	r.Push(2)
	r.Push(3)

	evicted, ok := r.Push(4)
	require.True(t, ok)
	require.Equal(t, 1, evicted, "the oldest element must be overwritten")
	require.Equal(t, []int{2, 3, 4}, r.ToSlice())

	require.Panics(t, func() { struct233.NewRingBuffer[int](0) })
}

func TestNewBitSet(t *testing.T) {
	b := struct233.NewBitSet()
	b.Set(7)
	b.Set(70)

	require.True(t, b.Get(70))
	require.Equal(t, 2, b.Cardinality())
	require.Equal(t, 71, b.Len())
	require.Equal(t, 7, b.NextSetBit(0))
	require.Equal(t, 70, b.NextSetBit(8))
}

func TestNewLRU(t *testing.T) {
	c := struct233.NewLRU[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)

	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, 1, v)

	ek, _, ev := c.Put("c", 3)
	require.True(t, ev)
	require.Equal(t, "b", ek, "Get must have promoted a")
	require.Equal(t, 2, c.Len())
}

func TestNewConcurrentContainers(t *testing.T) {
	cs := struct233.NewConcurrentHashSetWithCapacity[int](8)
	require.True(t, cs.Add(1))
	require.True(t, cs.Contains(1))

	cm := struct233.NewConcurrentHashMapWithCapacity[string, int](8)
	cm.Put("a", 1)
	v, loaded := cm.GetOrPut("a", func() int { return 99 })
	require.True(t, loaded)
	require.Equal(t, 1, v, "GetOrPut must keep the existing value")

	// the no-capacity constructors must work too
	require.True(t, struct233.NewConcurrentHashSet[int]().Add(1))
	require.Zero(t, struct233.NewConcurrentHashMap[int, int]().Len())

	csl := struct233.NewConcurrentSkipListMap[int, string]()
	csl.Put(2, "b")
	csl.Put(1, "a")
	require.Equal(t, []int{1, 2}, csl.Keys())

	cslFunc := struct233.NewConcurrentSkipListMapFunc[string, int](func(a, b string) int { return strings.Compare(b, a) })
	cslFunc.Put("a", 1)
	cslFunc.Put("c", 3)
	k, _, ok := cslFunc.First()
	require.True(t, ok)
	require.Equal(t, "c", k)

	cl := struct233.NewConcurrentLRU[int, int](2)
	cl.Put(1, 1)
	cl.Put(2, 2)
	cl.Put(3, 3)
	require.Equal(t, 2, cl.Len())
	require.False(t, cl.Contains(1), "the least recently used entry must be evicted")
}

// TestAliasesAreTheImplementationTypes proves the facade returns the concrete
// container types, so no wrapper indirection is introduced.
func TestAliasesAreTheImplementationTypes(t *testing.T) {
	var s *struct233.HashSet[int] = struct233.NewHashSet[int]()
	s.Add(1)
	require.True(t, s.Contains(1))

	var d *struct233.ArrayDeque[int] = struct233.NewArrayDeque[int]()
	d.PushBack(1)
	require.Equal(t, 1, d.Len())

	var tm *struct233.SkipListMap[int, int] = struct233.NewSkipListMap[int, int]()
	tm.Put(1, 1)
	require.Equal(t, 1, tm.Len())
}

// TestUtilityAliasesAreTheImplementationTypes proves the facade adds no
// wrapper around the utility packages either.
func TestUtilityAliasesAreTheImplementationTypes(t *testing.T) {
	var p *struct233.ObjectPool[*strings.Builder] = struct233.NewObjectPool(func() *strings.Builder {
		return new(strings.Builder)
	})
	require.Zero(t, p.Len())

	var gen *struct233.UUIDGenerator = struct233.NewUUIDGenerator()
	require.Equal(t, 7, gen.Next().Version())

	var sf *struct233.Snowflake = struct233.NewSnowflake(0)
	require.Equal(t, int64(0), sf.Node())

	wp := struct233.NewWorkerPool(1)
	wp.Close()
	var _ *struct233.WorkerPool = wp

	var rl *struct233.TokenBucketLimiter = struct233.NewTokenBucketLimiter(1, 1)
	require.True(t, rl.Allow())

	var u struct233.UUID = struct233.NilUUID
	require.True(t, u.IsNil())

	var id struct233.SnowflakeID = 42
	require.Equal(t, int64(42), id.Int64())
}

func TestNewObjectPool(t *testing.T) {
	p := struct233.NewObjectPool(func() *strings.Builder { return new(strings.Builder) },
		struct233.WithObjectPoolReset(func(b *strings.Builder) { b.Reset() }),
		struct233.WithObjectPoolMaxSize(2),
	)
	require.Equal(t, 2, p.Cap())

	b := p.Get()
	b.WriteString("first")
	p.Put(b)

	again := p.Get()
	require.Zero(t, again.Len(), "the reset hook must hand out a clean object")

	stats := p.Stats()
	require.Equal(t, int64(2), stats.Gets)
	require.Equal(t, int64(1), stats.Hits)
	require.InDelta(t, 0.5, stats.ReuseRate(), 1e-9)
}

func TestNewUUID(t *testing.T) {
	v4 := struct233.NewUUID4()
	require.Equal(t, 4, v4.Version())
	require.Equal(t, 4, struct233.NewUUID().Version())
	require.True(t, struct233.NilUUID.IsNil())

	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	when, ok := struct233.NewUUID7At(at).Time()
	require.True(t, ok)
	require.Equal(t, at.UnixMilli(), when.UnixMilli())
	require.Equal(t, 7, struct233.NewUUID7().Version())

	parsed, err := struct233.ParseUUID(strings.ToUpper(v4.String()))
	require.NoError(t, err)
	require.Equal(t, v4, parsed, "parsing is case-insensitive")
	require.Equal(t, v4, struct233.MustParseUUID(v4.String()))

	_, err = struct233.ParseUUID("not-a-uuid")
	require.Error(t, err)
	require.Panics(t, func() { struct233.MustParseUUID("not-a-uuid") })

	g := struct233.NewUUIDGenerator(struct233.WithUUIDClock(func() time.Time { return at }))
	first, second := g.Next(), g.Next()
	require.Negative(t, first.Compare(second), "an injected clock must keep v7 ids ordered")
}

func TestNewSnowflake(t *testing.T) {
	g := struct233.NewSnowflake(3, struct233.WithSnowflakeNodeBits(4))
	require.Equal(t, int64(3), g.Node())
	require.Equal(t, uint8(4), g.NodeBits())

	first, err := g.Next()
	require.NoError(t, err)
	second, err := g.Next()
	require.NoError(t, err)

	require.Greater(t, second.Int64(), first.Int64())
	require.Equal(t, int64(3), g.NodeOf(second))
	require.Zero(t, g.SequenceOf(first), "the first id of a millisecond uses sequence 0")

	fixed := time.Now()
	clocked := struct233.NewSnowflake(1,
		struct233.WithSnowflakeClock(func() time.Time { return fixed }),
		struct233.WithSnowflakeEpoch(fixed.Add(-time.Hour)),
	)
	id, err := clocked.Next()
	require.NoError(t, err)
	require.Equal(t, fixed.UnixMilli(), clocked.TimeOf(id).UnixMilli())

	require.Panics(t, func() { struct233.NewSnowflake(-1) })
}

func TestNewWorkerPool(t *testing.T) {
	p := struct233.NewWorkerPool(2, struct233.WithWorkerPoolQueueSize(4))

	var ran atomic.Int64
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		require.NoError(t, p.Submit(func() {
			defer wg.Done()
			ran.Add(1)
		}))
	}
	wg.Wait()

	require.NoError(t, p.SubmitWait(func() { ran.Add(1) }))
	require.NoError(t, p.SubmitWait(func() { panic("boom") }), "a panic must not surface to the submitter")
	p.Close()

	require.True(t, p.Closed())
	require.Equal(t, int64(9), ran.Load())

	m := p.Metrics()
	require.Equal(t, 2, m.Workers)
	require.Equal(t, int64(10), m.Submitted)
	require.Equal(t, int64(10), m.Completed)
	require.Equal(t, int64(1), m.Panics)

	require.ErrorIs(t, p.Submit(func() {}), workerpool.ErrClosed,
		"the facade returns the subpackage sentinel, so errors.Is works")
}

func TestNewTokenBucketLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := struct233.NewTokenBucketLimiter(2, 2, struct233.WithRateLimitClock(func() time.Time { return now }))
	require.Equal(t, 2.0, l.Rate())
	require.Equal(t, 2, l.Burst())

	require.True(t, l.Allow())
	require.True(t, l.Allow())
	require.False(t, l.Allow(), "the burst must be exhausted")
	require.False(t, l.AllowN(2))
	require.Positive(t, l.Reserve(), "an empty bucket must report a wait")

	now = now.Add(time.Second) // two tokens refill at 2/s
	require.InDelta(t, 2.0, l.Tokens(), 1e-9)
	require.True(t, l.AllowN(2))
	require.InDelta(t, 0.0, l.Tokens(), 1e-9)

	full := struct233.NewTokenBucketLimiter(1, 1)
	require.NoError(t, full.Wait(context.Background()), "a full bucket must not block")
	require.Panics(t, func() { struct233.NewTokenBucketLimiter(0, 1) })
	require.Panics(t, func() { struct233.NewTokenBucketLimiter(1, 0) })
}

func TestNewDelayQueue(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	q := struct233.NewDelayQueueWithClock[string](func() time.Time { return now })

	require.True(t, q.Put(struct233.NewDelayElement("later", now.Add(time.Minute))))

	_, ok := q.TryTake()
	require.False(t, ok, "nothing is due yet")
	require.Equal(t, 1, q.Len())

	now = now.Add(time.Minute) // the injected clock reaches the deadline
	item, ok := q.TryTake()
	require.True(t, ok)
	require.Equal(t, "later", item.Value())
	require.Equal(t, now, item.Deadline())

	q.Close()
	require.True(t, q.Closed())

	require.True(t, struct233.NewDelayQueue[int]().IsEmpty())
}

func TestNewLatchAndBarrier(t *testing.T) {
	l := struct233.NewLatch(2)
	require.Equal(t, 2, l.Count())
	require.False(t, l.Open())

	l.CountDown()
	require.False(t, l.Open(), "the latch opens only after the last CountDown")
	l.CountDown()
	require.True(t, l.Open())
	require.Zero(t, l.Count())
	require.NoError(t, l.Await(context.Background()), "an open latch must not block")
	require.True(t, l.AwaitTimeout(time.Millisecond))

	b := struct233.NewBarrier(2)
	require.Equal(t, 2, b.Parties())

	type arrival struct {
		index int
		err   error
	}
	arrived := make(chan arrival, 1)
	go func() {
		index, err := b.Await(context.Background())
		arrived <- arrival{index, err}
	}()

	index, err := b.Await(context.Background())
	require.NoError(t, err)
	require.Zero(t, index, "the party that completes the generation arrives last")

	select {
	case other := <-arrived:
		require.NoError(t, other.err)
		require.Equal(t, 1, other.index)
	case <-time.After(time.Second):
		t.Fatal("the barrier did not release the waiting party")
	}
	require.False(t, b.Broken())
	require.Zero(t, b.Waiting())
}

func TestNewSemaphore(t *testing.T) {
	s := struct233.NewSemaphore(2)
	require.Equal(t, int64(2), s.Limit())
	require.Equal(t, int64(2), s.Permits())

	require.True(t, s.TryAcquire())
	require.True(t, s.TryAcquire())
	require.False(t, s.TryAcquire(), "the permits are exhausted")
	require.Zero(t, s.Permits())
	require.Zero(t, s.Waiters(), "TryAcquire never queues a waiter")

	s.Release(2)
	require.Equal(t, int64(2), s.Permits())

	require.NoError(t, s.Acquire(context.Background()))
	require.NoError(t, s.AcquireN(context.Background(), 1))
	require.Zero(t, s.Permits())

	require.False(t, s.AcquireTimeout(1, 10*time.Millisecond), "the permits are gone")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, s.Acquire(ctx), context.DeadlineExceeded)

	require.Panics(t, func() { struct233.NewSemaphore(0) })
	require.Panics(t, func() { s.Release(-1) })
}

func TestNewSlidingWindowLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	w := struct233.NewSlidingWindowLimiter(2, time.Minute, struct233.WithRateLimitClock(func() time.Time { return now }))
	require.Equal(t, 2, w.Limit())
	require.Equal(t, time.Minute, w.Window())

	require.True(t, w.Allow())
	require.True(t, w.Allow())
	require.False(t, w.Allow(), "the window is full")
	require.Zero(t, w.Remaining())
	require.Equal(t, time.Minute, w.RetryAfter())

	now = now.Add(time.Minute)
	require.Equal(t, 2, w.Remaining())
	require.Zero(t, w.RetryAfter())
	require.True(t, w.AllowN(2))
	require.False(t, w.AllowN(1))

	w.Reset()
	require.Equal(t, 2, w.Remaining())
}

func TestRetry(t *testing.T) {
	attempts := 0
	var delays []time.Duration

	err := struct233.Retry(context.Background(), func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errTransient
		}
		return nil
	},
		struct233.WithRetryMaxAttempts(5),
		struct233.WithRetryBaseDelay(10*time.Millisecond),
		struct233.WithRetryMultiplier(3),
		struct233.WithRetryJitter(0),
		struct233.WithRetrySleep(func(_ context.Context, d time.Duration) error {
			delays = append(delays, d)
			return nil
		}),
	)
	require.NoError(t, err)
	require.Equal(t, 3, attempts)
	require.Equal(t, []time.Duration{10 * time.Millisecond, 30 * time.Millisecond}, delays,
		"the backoff must grow exponentially")

	// the final error is returned unwrapped, so errors.Is keeps working
	failing := struct233.Retry(context.Background(), func(context.Context) error { return errTransient },
		struct233.WithRetryMaxAttempts(2),
		struct233.WithRetrySleep(func(context.Context, time.Duration) error { return nil }),
	)
	require.ErrorIs(t, failing, errTransient)

	// a policy is reusable
	policy := struct233.NewRetryPolicy(struct233.WithRetryMaxAttempts(1))
	require.ErrorIs(t, policy.Do(context.Background(), func(context.Context) error {
		return errTransient
	}), errTransient)

	value, err := struct233.RetryValue(context.Background(), func(context.Context) (int, error) {
		return 42, nil
	})
	require.NoError(t, err)
	require.Equal(t, 42, value)
}

func TestNewBreaker(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	b := struct233.NewBreaker(
		struct233.WithBreakerFailureThreshold(2),
		struct233.WithBreakerSuccessThreshold(1),
		struct233.WithBreakerOpenTimeout(time.Minute),
		struct233.WithBreakerClock(func() time.Time { return now }),
	)
	require.Equal(t, struct233.BreakerClosed, b.State())

	b.Failure()
	require.Equal(t, struct233.BreakerClosed, b.State(), "one failure is below the threshold")
	b.Failure()
	require.Equal(t, struct233.BreakerOpen, b.State(), "the threshold must open the circuit")

	calls := 0
	require.ErrorIs(t, b.Execute(func() error { calls++; return nil }), struct233.ErrBreakerOpen)
	require.Zero(t, calls, "Execute must not call fn while the circuit is open")
	require.Equal(t, int64(1), b.Counts().Rejections)

	now = now.Add(2 * time.Minute)
	require.Equal(t, struct233.BreakerHalfOpen, b.State(), "the open timeout must allow probes")
	require.True(t, b.Allow())
	b.Success()
	require.Equal(t, struct233.BreakerClosed, b.State(), "a successful probe closes the circuit")

	b.Reset()
	require.Zero(t, b.Counts().Failures)
}

func TestNewRandomHelpers(t *testing.T) {
	src := &scriptedSource{
		ints:   []int{0, 2},
		floats: []float64{0.1, 0.0, 0.5, 0.99},
	}
	g := struct233.NewRandom(src)

	require.Equal(t, 10, g.Int(10, 20), "a zero draw maps to min")
	require.Equal(t, 12, g.Int(10, 20))
	require.InDelta(t, 0.1, g.Float64(), 1e-9)

	items := []string{"common", "rare"}
	table := struct233.NewWeightedTable(g, items, func(s string) float64 {
		if s == "rare" {
			return 1
		}
		return 3
	})
	require.Equal(t, 2, table.Items())

	// cumulative weights are [3, 4]: anything below 3/4 picks the common item
	picked, ok := table.Pick()
	require.True(t, ok)
	require.Equal(t, "common", picked)
	picked, ok = table.Pick()
	require.True(t, ok)
	require.Equal(t, "common", picked)
	picked, ok = table.Pick()
	require.True(t, ok)
	require.Equal(t, "rare", picked)

	require.Equal(t, 5, struct233.DefaultRandom().Int(5, 5))

	shuffled := []int{1, 2, 3, 4, 5}
	struct233.Shuffle(nil, shuffled)
	require.ElementsMatch(t, []int{1, 2, 3, 4, 5}, shuffled)

	pick, ok := struct233.Choice[string](nil, items)
	require.True(t, ok)
	require.Contains(t, items, pick)
	_, ok = struct233.Choice[string](nil, nil)
	require.False(t, ok, "an empty set has no choice")

	sample := struct233.Sample(nil, []int{1, 2, 3, 4, 5}, 3)
	require.Len(t, sample, 3)
	require.Subset(t, []int{1, 2, 3, 4, 5}, sample)

	only, ok := struct233.Weighted(nil, []string{"only"}, func(string) float64 { return 1 })
	require.True(t, ok)
	require.Equal(t, "only", only)
}

func TestNewTimeWheel(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	w := struct233.NewTimeWheel(10*time.Millisecond, 8,
		struct233.WithTimeWheelClock(func() time.Time { return now }),
		struct233.WithTimeWheelLevels(3),
	)

	var fired atomic.Int64
	timer := w.Schedule(30*time.Millisecond, func() { fired.Add(1) })
	require.Equal(t, 1, w.Len())

	require.Zero(t, w.AdvanceBy(29*time.Millisecond), "a callback must never run early")
	require.Equal(t, 1, w.AdvanceBy(time.Millisecond))
	require.Equal(t, int64(1), fired.Load())
	require.True(t, timer.Done())
	require.Zero(t, w.Len())

	cancelled := w.Schedule(time.Second, func() { fired.Add(1) })
	require.True(t, cancelled.Cancel())
	w.Stop()
	require.Zero(t, w.Len())
	require.Equal(t, int64(1), fired.Load())
}

func TestNewTableAndRangeMap(t *testing.T) {
	scores := struct233.NewTable[string, string, int]()
	scores.Put("ada", "math", 98)
	scores.Put("ada", "physics", 94)
	score, ok := scores.Get("ada", "math")
	require.True(t, ok)
	require.Equal(t, 98, score)
	require.Equal(t, map[string]int{"math": 98, "physics": 94}, scores.Row("ada"))

	prices := struct233.NewRangeMap[int, string]()
	prices.Put(struct233.RangeClosed(1, 10), "standard")
	prices.Put(struct233.RangeClosedOpen(4, 7), "discount")
	price, ok := prices.Get(5)
	require.True(t, ok)
	require.Equal(t, "discount", price)
	require.Equal(t, []struct233.RangeMapEntry[int, string]{
		{Range: struct233.RangeClosedOpen(1, 4), Value: "standard"},
		{Range: struct233.RangeClosedOpen(4, 7), Value: "discount"},
		{Range: struct233.RangeClosed(7, 10), Value: "standard"},
	}, prices.Entries())
}

func BenchmarkFacadeNewSetAndAdd(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		s := struct233.NewHashSetWithCapacity[int](4)
		s.Add(i & 3)
	}
}

func BenchmarkFacadeGetSkipList(b *testing.B) {
	sl := struct233.NewSkipListMap[int, int]()
	rng := rand.New(rand.NewPCG(1, 2))
	for range 1024 {
		k := rng.IntN(1024)
		sl.Put(k, k)
	}
	keys := make([]int, 1024)
	for i := range keys {
		keys[i] = rng.IntN(1024)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		sl.Get(keys[i&1023])
	}
}
