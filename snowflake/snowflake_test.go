package snowflake_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/snowflake"
)

// fakeClock is a manual, race-free clock for tests.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// scriptedClock returns the same millisecond for step consecutive reads, then
// advances by one millisecond. A generator that runs out of sequence numbers
// waits for the clock to move on, so a step larger than the sequence budget
// exercises that wait without spinning forever.
type scriptedClock struct {
	mu     sync.Mutex
	epoch  time.Time
	baseMS int64
	step   int
	reads  int
}

func (c *scriptedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	ms := c.baseMS + int64(c.reads/c.step)
	c.reads++
	return c.epoch.Add(time.Duration(ms) * time.Millisecond)
}

func TestNextIsMonotonic(t *testing.T) {
	g := snowflake.New(7)

	var prev snowflake.ID
	for i := range 1000 {
		id, err := g.Next()
		require.NoError(t, err, "Next #%d", i)
		require.Greater(t, id.Int64(), prev.Int64(), "Next #%d must be strictly greater", i)
		prev = id
	}
}

func TestDecodersRoundTrip(t *testing.T) {
	epoch := time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC)
	g := snowflake.New(42, snowflake.WithEpoch(epoch), snowflake.WithNodeBits(12))
	require.Equal(t, int64(42), g.Node())
	require.Equal(t, uint8(12), g.NodeBits())
	require.Equal(t, epoch, g.Epoch())

	id, err := g.Next()
	require.NoError(t, err)
	require.Equal(t, int64(id), id.Int64())
	require.Equal(t, int64(42), g.NodeOf(id))
	require.Zero(t, g.SequenceOf(id), "the first ID of a millisecond uses sequence 0")

	generated := g.TimeOf(id)
	require.False(t, generated.Before(epoch), "TimeOf must not precede the epoch")
	require.Less(t, time.Since(generated), time.Hour, "TimeOf must be close to the current time")
}

func TestSequenceIncrementsWithinMillisecond(t *testing.T) {
	clock := newFakeClock()
	g := snowflake.New(9, snowflake.WithClock(clock.Now))

	first, err := g.Next()
	require.NoError(t, err)

	for want := int64(1); want <= 4; want++ {
		id, err := g.Next()
		require.NoError(t, err)
		require.Equal(t, int64(9), g.NodeOf(id))
		require.Equal(t, want, g.SequenceOf(id), "the sequence must advance within one millisecond")
		require.Equal(t, g.TimeOf(first), g.TimeOf(id), "the millisecond must stay the same")
	}
}

func TestConcurrentIDsAreUnique(t *testing.T) {
	const goroutines, perGoroutine = 8, 2000
	g := snowflake.New(3)

	var wg sync.WaitGroup
	ids := make([][]snowflake.ID, goroutines)
	errs := make([]error, goroutines)
	for w := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out := make([]snowflake.ID, 0, perGoroutine)
			for range perGoroutine {
				id, err := g.Next()
				if err != nil {
					errs[w] = err
					return
				}
				out = append(out, id)
			}
			ids[w] = out
		}()
	}
	wg.Wait()

	for w, err := range errs {
		require.NoError(t, err, "goroutine %d", w)
	}
	seen := make(map[snowflake.ID]struct{}, goroutines*perGoroutine)
	for w, batch := range ids {
		require.Len(t, batch, perGoroutine, "goroutine %d", w)
		for i, id := range batch {
			_, dup := seen[id]
			require.False(t, dup, "goroutine %d id #%d repeated %d", w, i, id.Int64())
			seen[id] = struct{}{}
		}
	}
	require.Len(t, seen, goroutines*perGoroutine, "every generated id must be distinct")
}

func TestClockRollbackReturnsError(t *testing.T) {
	clock := newFakeClock()
	g := snowflake.New(1, snowflake.WithClock(clock.Now))

	id, err := g.Next()
	require.NoError(t, err)

	clock.Set(clock.Now().Add(-time.Second)) // the clock jumped backwards
	_, err = g.Next()
	require.ErrorIs(t, err, snowflake.ErrClockBackwards)

	// the clock catches up again, so ids keep flowing
	clock.Set(clock.Now().Add(2 * time.Second))
	recovered, err := g.Next()
	require.NoError(t, err)
	require.Greater(t, recovered.Int64(), id.Int64(), "ids must stay monotonic after the clock recovers")
}

func TestNextWaitsForNextMillisecond(t *testing.T) {
	epoch := time.Date(2022, 3, 4, 0, 0, 0, 0, time.UTC)
	// nodeBits 20 leaves seqBits 2, so only 4 ids fit in a millisecond while
	// the scripted clock holds each millisecond for 6 reads: the fifth Next
	// has to block until the clock moves on.
	clock := &scriptedClock{epoch: epoch, baseMS: 1000, step: 6}
	g := snowflake.New(5, snowflake.WithEpoch(epoch), snowflake.WithNodeBits(20), snowflake.WithClock(clock.Now))

	const calls = 8
	got := make([]snowflake.ID, 0, calls)
	for i := range calls {
		id, err := g.Next()
		require.NoError(t, err, "Next #%d must succeed while the clock advances", i)
		got = append(got, id)
	}
	require.Len(t, got, calls, "every call must produce an id")

	seen := make(map[snowflake.ID]struct{}, calls)
	for i, id := range got {
		require.Equal(t, int64(5), g.NodeOf(id))
		require.LessOrEqual(t, g.SequenceOf(id), int64(3), "the sequence must fit in 2 bits")
		_, dup := seen[id]
		require.False(t, dup, "Next #%d must not repeat an id", i)
		seen[id] = struct{}{}
		if i > 0 {
			require.Greater(t, id.Int64(), got[i-1].Int64(), "Next #%d must advance", i)
		}
	}

	require.Greater(t, g.TimeOf(got[len(got)-1]).Sub(epoch).Milliseconds(), int64(1000),
		"the generator must have waited for a later millisecond")
}

func TestNodeBitsLayout(t *testing.T) {
	g := snowflake.New(15, snowflake.WithNodeBits(4))
	require.Equal(t, uint8(4), g.NodeBits())

	id, err := g.Next()
	require.NoError(t, err)
	require.Equal(t, int64(15), g.NodeOf(id), "the largest 4-bit node id must round-trip")
	require.LessOrEqual(t, g.SequenceOf(id), int64((1<<18)-1), "the sequence must fit in 18 bits")
}

func TestMaxNodeBitsLeavesNoSequenceBits(t *testing.T) {
	node := int64((1 << 22) - 1)
	g := snowflake.New(node, snowflake.WithNodeBits(22))

	id, err := g.Next()
	require.NoError(t, err)
	require.Equal(t, node, g.NodeOf(id))
	require.Zero(t, g.SequenceOf(id), "22 node bits leave no room for a sequence")
}

func TestPanicsOnInvalidOptions(t *testing.T) {
	require.Panics(t, func() { snowflake.New(-1) }, "negative node id")
	require.Panics(t, func() { snowflake.New(16, snowflake.WithNodeBits(4)) }, "node id past 1<<nodeBits")
	require.Panics(t, func() { snowflake.New(0, snowflake.WithNodeBits(0)) }, "node bits below 1")
	require.Panics(t, func() { snowflake.New(0, snowflake.WithNodeBits(23)) }, "node bits above 22")
	require.Panics(t, func() { snowflake.New(0, snowflake.WithEpoch(time.Now().Add(time.Hour))) }, "epoch in the future")
	require.Panics(t, func() { snowflake.New(0, snowflake.WithClock(nil)) }, "nil clock")
}

func TestMustNextPanicsOnClockRollback(t *testing.T) {
	clock := newFakeClock()
	g := snowflake.New(2, snowflake.WithClock(clock.Now))
	require.NotPanics(t, func() { g.MustNext() }, "MustNext must not panic while the clock advances")

	clock.Set(clock.Now().Add(-time.Second))
	require.PanicsWithValue(t, snowflake.ErrClockBackwards, func() { g.MustNext() },
		"MustNext must panic with ErrClockBackwards")
}
