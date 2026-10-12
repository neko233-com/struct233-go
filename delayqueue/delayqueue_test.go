package delayqueue_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/delayqueue"
)

// clock is a manually advanced time source, which keeps the deadline decisions
// deterministic instead of racing the real clock.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock() *clock {
	return &clock{now: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type takeResult struct {
	item *delayqueue.Element[string]
	ok   bool
}

// startTake runs a blocking Take in the background so the test can observe when
// it returns.
func startTake(q *delayqueue.Queue[string]) <-chan takeResult {
	ch := make(chan takeResult, 1)
	go func() {
		item, ok := q.Take()
		ch <- takeResult{item, ok}
	}()
	return ch
}

func TestTryTakeOnlyReturnsDueElements(t *testing.T) {
	c := newClock()
	q := delayqueue.NewWithClock[string](c.Now)

	require.True(t, q.Put(delayqueue.NewElement("soon", c.Now().Add(time.Second))))
	require.True(t, q.Put(delayqueue.NewElement("late", c.Now().Add(time.Hour))))
	require.Equal(t, 2, q.Len())

	_, ok := q.TryTake()
	require.False(t, ok, "nothing is due yet")

	c.Advance(2 * time.Second)
	item, ok := q.TryTake()
	require.True(t, ok)
	require.Equal(t, "soon", item.Value(), "the earliest deadline comes out first")

	_, ok = q.TryTake()
	require.False(t, ok, "the second element is still an hour away")

	c.Advance(time.Hour)
	item, ok = q.TryTake()
	require.True(t, ok)
	require.Equal(t, "late", item.Value())
	require.True(t, q.IsEmpty())
}

func TestPeekIgnoresTheDeadline(t *testing.T) {
	c := newClock()
	q := delayqueue.NewWithClock[string](c.Now)

	_, ok := q.Peek()
	require.False(t, ok, "Peek on an empty queue must miss")

	q.Put(delayqueue.NewElement("far", c.Now().Add(time.Hour)))
	q.Put(delayqueue.NewElement("near", c.Now().Add(time.Minute)))

	item, ok := q.Peek()
	require.True(t, ok)
	require.Equal(t, "near", item.Value(), "Peek returns the earliest element even when it is not due")
	require.Equal(t, 2, q.Len(), "Peek must not remove anything")
}

func TestEqualDeadlinesKeepFIFOOrder(t *testing.T) {
	c := newClock()
	q := delayqueue.NewWithClock[int](c.Now)
	deadline := c.Now().Add(time.Second)

	for i := range 4 {
		q.Put(delayqueue.NewElement(i, deadline))
	}
	c.Advance(time.Second)

	var got []int
	for range 4 {
		item, ok := q.TryTake()
		require.True(t, ok)
		got = append(got, item.Value())
	}
	require.Equal(t, []int{0, 1, 2, 3}, got, "equal deadlines must be served in insertion order")
}

func TestTakeBlocksUntilTheDeadlinePasses(t *testing.T) {
	c := newClock()
	q := delayqueue.NewWithClock[string](c.Now)
	q.Put(delayqueue.NewElement("later", c.Now().Add(50*time.Millisecond)))

	results := startTake(q)

	select {
	case r := <-results:
		t.Fatalf("Take returned %v before the deadline", r)
	case <-time.After(20 * time.Millisecond):
	}

	c.Advance(50 * time.Millisecond)

	select {
	case r := <-results:
		require.True(t, r.ok)
		require.Equal(t, "later", r.item.Value())
	case <-time.After(time.Second):
		t.Fatal("Take did not return after the deadline passed")
	}
}

func TestTakeWakesWhenAnEarlierElementArrives(t *testing.T) {
	// the system clock drives this one: the point is that a Put with an earlier
	// deadline wakes a blocked taker instead of leaving it to wait out the
	// first element's deadline
	q := delayqueue.New[string]()
	q.Put(delayqueue.NewElement("in an hour", time.Now().Add(time.Hour)))

	results := startTake(q)
	time.Sleep(10 * time.Millisecond) // let the taker block on the far deadline

	q.Put(delayqueue.NewElement("now", time.Now()))

	select {
	case r := <-results:
		require.True(t, r.ok)
		require.Equal(t, "now", r.item.Value())
	case <-time.After(time.Second):
		t.Fatal("Take stayed blocked after an earlier element was queued")
	}
}

func TestCloseUnblocksWaiters(t *testing.T) {
	q := delayqueue.New[string]()
	results := startTake(q)

	time.Sleep(10 * time.Millisecond)
	q.Close()

	select {
	case r := <-results:
		require.False(t, r.ok, "a closed, empty queue reports false")
		require.Nil(t, r.item)
	case <-time.After(time.Second):
		t.Fatal("Close did not unblock Take")
	}
	require.True(t, q.Closed())
}

func TestTakeStillDrainsAfterClose(t *testing.T) {
	c := newClock()
	q := delayqueue.NewWithClock[string](c.Now)
	q.Put(delayqueue.NewElement("due", c.Now()))
	q.Put(delayqueue.NewElement("future", c.Now().Add(time.Hour)))
	q.Close()

	item, ok := q.Take()
	require.True(t, ok, "elements already due stay drainable after Close")
	require.Equal(t, "due", item.Value())

	_, ok = q.Take()
	require.False(t, ok, "the future element is not due, and the queue is closed")

	require.False(t, q.Put(delayqueue.NewElement("late", c.Now())), "Put after Close must report false")
}

func TestPollTimesOut(t *testing.T) {
	c := newClock()
	q := delayqueue.NewWithClock[string](c.Now)

	_, ok := q.Poll(10 * time.Millisecond)
	require.False(t, ok, "an empty queue must time out")

	q.Put(delayqueue.NewElement("far", c.Now().Add(time.Hour)))
	start := time.Now()
	_, ok = q.Poll(20 * time.Millisecond)
	require.False(t, ok, "an element that is not due must time out")
	require.Less(t, time.Since(start), time.Second, "Poll must not wait much longer than its timeout")

	// an element that is already due comes out immediately
	q.Put(delayqueue.NewElement("due", c.Now()))
	item, ok := q.Poll(0)
	require.True(t, ok)
	require.Equal(t, "due", item.Value())
}

func TestDrainAndClear(t *testing.T) {
	c := newClock()
	q := delayqueue.NewWithClock[int](c.Now)
	for i := range 5 {
		q.Put(delayqueue.NewElement(i, c.Now().Add(time.Duration(i)*time.Second)))
	}
	c.Advance(10 * time.Second)

	only := q.Drain(2)
	require.Len(t, only, 2, "Drain(n) removes at most n elements")
	require.Equal(t, []int{0, 1}, []int{only[0].Value(), only[1].Value()})
	require.Equal(t, 3, q.Len())

	q.Clear()
	require.Zero(t, q.Len())
	require.Empty(t, q.Drain(0), "nothing is left to drain")
}

func TestElementAccessors(t *testing.T) {
	c := newClock()
	deadline := c.Now().Add(time.Minute)
	e := delayqueue.NewElement(42, deadline)

	require.Equal(t, 42, e.Value())
	require.Equal(t, deadline, e.Deadline())
	require.Equal(t, time.Minute, e.Delay(c.Now()))
	require.False(t, e.Expired(c.Now()))
	require.True(t, e.Expired(deadline), "a zero remaining delay counts as due")
	require.Zero(t, e.Delay(deadline))
	require.Negative(t, e.Delay(deadline.Add(time.Second)))
}

func TestZeroDeadlineIsDueImmediately(t *testing.T) {
	q := delayqueue.New[string]()
	q.Put(delayqueue.NewElement("zero", time.Time{}))

	item, ok := q.TryTake()
	require.True(t, ok, "the zero time is in the past, so the element is due")
	require.Equal(t, "zero", item.Value())
}

func TestPanicsOnBadInput(t *testing.T) {
	require.Panics(t, func() { delayqueue.New[string]().Put(nil) })
	require.Panics(t, func() { delayqueue.NewWithClock[string](nil) })
}

func TestPutAndTryTakeDoNotAllocate(t *testing.T) {
	q := delayqueue.New[int]()
	e := delayqueue.NewElement(7, time.Now())

	q.Put(e)
	q.TryTake() // reach steady state: the heap keeps its capacity

	allocs := testing.AllocsPerRun(100, func() {
		q.Put(e)
		q.TryTake()
	})
	require.Zero(t, allocs, "Put+TryTake on a warm queue must not allocate")
}

func TestConcurrentProducersAndConsumers(t *testing.T) {
	const (
		producers = 4
		consumers = 4
		perWorker = 250
	)

	q := delayqueue.New[int]()

	var produced atomic.Int64
	var wg sync.WaitGroup
	for p := range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perWorker {
				// spread the deadlines around the moment of insertion
				d := time.Duration(i%5) * time.Millisecond
				q.Put(delayqueue.NewElement(p*perWorker+i, time.Now().Add(d)))
				produced.Add(1)
			}
		}()
	}

	var consumed atomic.Int64
	seen := make([][]bool, producers)
	for i := range seen {
		seen[i] = make([]bool, perWorker)
	}
	var seenMu sync.Mutex
	for range consumers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				e, ok := q.Take()
				if !ok {
					return
				}
				v := e.Value()
				seenMu.Lock()
				seen[v/perWorker][v%perWorker] = true
				seenMu.Unlock()
				consumed.Add(1)
			}
		}()
	}

	require.Eventually(t, func() bool {
		return produced.Load() == int64(producers*perWorker)
	}, time.Second, time.Millisecond, "the producers must finish")

	// let the consumers drain, then close to release them
	require.Eventually(t, func() bool {
		return consumed.Load() == int64(producers*perWorker)
	}, 5*time.Second, time.Millisecond, "every element must be consumed")

	q.Close()
	wg.Wait()

	require.Equal(t, int64(producers*perWorker), consumed.Load())
	for p, batch := range seen {
		for i, got := range batch {
			require.True(t, got, "producer %d element %d was never consumed", p, i)
		}
	}
}
