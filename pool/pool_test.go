package pool_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/pool"
)

// item is the typical pooled object: a struct with a slice that the reset hook
// rewinds so the backing array is reused.
type item struct {
	buf []byte
}

// item2 is a second pooled type, used to prove that a reset hook of the wrong
// type is rejected at construction time.
type item2 struct{ n int }

func newCounter() (*pool.Pool[*item], *atomic.Int64) {
	var created atomic.Int64
	p := pool.New(func() *item {
		created.Add(1)
		return &item{buf: make([]byte, 0, 8)}
	}, pool.WithReset(func(it *item) { it.buf = it.buf[:0] }))
	return p, &created
}

func TestGetCreatesThenReuses(t *testing.T) {
	p, created := newCounter()

	first := p.Get()
	require.Equal(t, int64(1), created.Load(), "an empty pool must build an object")

	p.Put(first)
	second := p.Get()
	require.Same(t, first, second, "the pooled object must come back")
	require.Equal(t, int64(1), created.Load(), "a hit must not build a new object")

	stats := p.Stats()
	require.Equal(t, int64(2), stats.Gets)
	require.Equal(t, int64(1), stats.Hits)
	require.Equal(t, int64(1), stats.Misses)
	require.Equal(t, int64(1), stats.Puts)
	require.Zero(t, stats.Discards)
	require.InDelta(t, 0.5, stats.ReuseRate(), 1e-9)
}

func TestReuseRateBeforeAnyGet(t *testing.T) {
	p, _ := newCounter()
	require.Zero(t, p.Stats().ReuseRate(), "no Get call means no reuse rate")
}

func TestPutRunsResetHook(t *testing.T) {
	p := pool.New(func() *item { return &item{} },
		pool.WithReset(func(it *item) { it.buf = nil }))

	it := p.Get()
	it.buf = []byte{1, 2, 3}
	p.Put(it)

	require.Nil(t, it.buf, "Put must run the reset hook before storing the object")
	back := p.Get()
	require.Nil(t, back.buf, "a pooled object must be clean for the next caller")
}

func TestPutDropsObjectsPastMaxSize(t *testing.T) {
	p := pool.New(func() *item { return &item{} }, pool.WithMaxSize(2))
	require.Equal(t, 2, p.Cap())

	for range 5 {
		p.Put(&item{})
	}
	require.Equal(t, 2, p.Len(), "the idle set must not grow past the maximum")

	stats := p.Stats()
	require.Equal(t, int64(2), stats.Puts)
	require.Equal(t, int64(3), stats.Discards)
}

func TestUnboundedByDefault(t *testing.T) {
	p := pool.New(func() *item { return &item{} })
	require.Zero(t, p.Cap(), "0 means unbounded")

	for range 100 {
		p.Put(&item{})
	}
	require.Equal(t, 100, p.Len())

	p.Clear()
	require.Zero(t, p.Len())
	require.Zero(t, p.Cap())
}

func TestClearRunsResetOnEveryDroppedObject(t *testing.T) {
	var resets atomic.Int64
	p := pool.New(func() *item { return &item{} },
		pool.WithReset(func(*item) { resets.Add(1) }))

	for range 3 {
		p.Put(&item{})
	}
	before := resets.Load() // Put itself already ran the hook once per object
	p.Clear()
	require.Zero(t, p.Len())
	require.Equal(t, before+3, resets.Load(),
		"Clear must run the reset hook on every dropped object so references are released")

	// the pool stays usable after Clear
	p.Put(&item{})
	require.Equal(t, 1, p.Len())
	require.NotNil(t, p.Get())
}

func TestPreallocMakesTheFirstGetsHit(t *testing.T) {
	p, created := newCounter()
	p.Prealloc(4)
	require.Equal(t, 4, p.Len())
	require.Equal(t, int64(4), created.Load())

	for range 4 {
		require.NotNil(t, p.Get())
	}
	stats := p.Stats()
	require.Equal(t, int64(4), stats.Hits)
	require.Zero(t, stats.Misses, "Prealloc must satisfy the first n Get calls")
}

func TestPreallocRespectsMaxSize(t *testing.T) {
	p := pool.New(func() *item { return &item{} }, pool.WithMaxSize(2))
	p.Prealloc(10)
	require.Equal(t, 2, p.Len())
}

func TestPreallocPanicsOnNegativeCount(t *testing.T) {
	p := pool.New(func() *item { return &item{} })
	require.Panics(t, func() { p.Prealloc(-1) })
}

func TestFactoryMayCallBackIntoThePool(t *testing.T) {
	// the factory runs without the internal lock held, so touching the pool
	// from inside it must not deadlock
	var p *pool.Pool[int]
	p = pool.New(func() int {
		_ = p.Len()
		return 7
	})
	require.Equal(t, 7, p.Get())
}

func TestPanicsOnInvalidArguments(t *testing.T) {
	require.Panics(t, func() { pool.New[int](nil) })
	require.Panics(t, func() { pool.New(func() int { return 0 }, pool.WithMaxSize(-1)) })
	require.Panics(t, func() {
		// the hook type does not match the pooled type
		pool.New(func() *item { return &item{} }, pool.WithReset(func(other *item2) { _ = other }))
	})
	require.Panics(t, func() {
		pool.New(func() *item { return &item{} }, pool.WithReset[*item](nil))
	})
}

func TestConcurrentGetPut(t *testing.T) {
	const (
		goroutines = 8
		perWorker  = 200
	)
	p := pool.New(func() *item { return &item{} }, pool.WithMaxSize(4))

	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perWorker {
				it := p.Get()
				it.buf = append(it.buf[:0], 1, 2, 3)
				p.Put(it)
			}
		}()
	}
	wg.Wait()

	stats := p.Stats()
	require.Equal(t, int64(goroutines*perWorker), stats.Gets)
	require.Equal(t, stats.Gets, stats.Hits+stats.Misses, "every Get is either a hit or a miss")
	require.LessOrEqual(t, p.Len(), 4, "the idle set must honour the maximum")
}

func TestGetPutDoesNotAllocate(t *testing.T) {
	p := pool.New(func() *item {
		t := new(item)
		t.buf = make([]byte, 0, 64)
		return t
	})
	p.Prealloc(1)

	it := p.Get()
	p.Put(it) // reach steady state: the idle slice already has capacity

	allocs := testing.AllocsPerRun(100, func() {
		p.Put(p.Get())
	})
	require.Zero(t, allocs, "Get+Put on a warm pool must not allocate")
}
