package semaphore_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/semaphore"
)

// asyncAcquire runs AcquireN in a goroutine so a test can observe blocking
// without racing on the assertion helpers.
func asyncAcquire(s *semaphore.Semaphore, ctx context.Context, n int64) <-chan error {
	errs := make(chan error, 1)
	go func() { errs <- s.AcquireN(ctx, n) }()
	return errs
}

// requireWaiters waits for the blocked-caller count to reach want, which is how
// the tests synchronise on a queued waiter instead of sleeping.
func requireWaiters(t *testing.T, s *semaphore.Semaphore, want int, msg string) {
	t.Helper()
	require.Eventually(t, func() bool { return s.Waiters() == want }, time.Second, time.Millisecond, msg)
}

// requireStillBlocked fails when errs produces a result within the guard
// window, i.e. when the caller was not actually blocked.
func requireStillBlocked(t *testing.T, errs <-chan error, msg string) {
	t.Helper()
	select {
	case err := <-errs:
		require.NoError(t, err)
		t.Fatal(msg)
	case <-time.After(20 * time.Millisecond):
	}
}

// awaitAcquire fails when the blocked caller does not return in time.
func awaitAcquire(t *testing.T, errs <-chan error, msg string) error {
	t.Helper()
	select {
	case err := <-errs:
		return err
	case <-time.After(time.Second):
		t.Fatal(msg)
		return nil
	}
}

func TestNewStartsWithAllPermitsFree(t *testing.T) {
	sem := semaphore.New(3)

	require.Equal(t, int64(3), sem.Limit())
	require.Equal(t, int64(3), sem.Permits(), "New must hand out the full permit count")
	require.Zero(t, sem.Waiters())
}

func TestPermitsAccountingRoundTrip(t *testing.T) {
	ctx := context.Background()
	sem := semaphore.New(3)

	require.NoError(t, sem.Acquire(ctx))
	require.Equal(t, int64(2), sem.Permits())

	require.NoError(t, sem.AcquireN(ctx, 2))
	require.Zero(t, sem.Permits())
	require.False(t, sem.TryAcquire(), "an exhausted semaphore must refuse a fast-path take")

	sem.Release(3)
	require.Equal(t, int64(3), sem.Permits(), "releasing must return every permit")
	require.Equal(t, int64(3), sem.Limit(), "Release must not change the configured limit")
}

func TestAcquireBlocksUntilRelease(t *testing.T) {
	ctx := context.Background()
	sem := semaphore.New(1)
	require.NoError(t, sem.Acquire(ctx), "the test holds the only permit")

	errs := asyncAcquire(sem, ctx, 1)
	requireWaiters(t, sem, 1, "Acquire must queue while no permit is free")
	requireStillBlocked(t, errs, "Acquire must block instead of taking a permit that is not free")

	sem.Release(1)
	require.NoError(t, awaitAcquire(t, errs, "Acquire was not unblocked by Release"))
	require.Zero(t, sem.Permits(), "the woken caller must own the permit")
	require.Zero(t, sem.Waiters(), "a served caller must leave the queue")
}

func TestAcquireNIsAllOrNothing(t *testing.T) {
	ctx := context.Background()
	sem := semaphore.New(10)
	require.True(t, sem.TryAcquireN(7), "7 of 10 permits must be free")
	require.Equal(t, int64(3), sem.Permits())

	errs := asyncAcquire(sem, ctx, 4)
	requireWaiters(t, sem, 1, "AcquireN(4) must block while only 3 permits are free")
	requireStillBlocked(t, errs, "AcquireN must not take a partial 3 of the 4 requested permits")
	require.Equal(t, int64(3), sem.Permits(), "a blocked AcquireN must not hold the free permits")

	sem.Release(1) // 4 free permits make the whole request satisfiable
	require.NoError(t, awaitAcquire(t, errs, "AcquireN(4) was not unblocked once 4 permits were free"))
	require.Zero(t, sem.Permits(), "all 10 permits must be out on loan")
}

func TestAcquireNReturnsNilForNonPositiveCount(t *testing.T) {
	ctx := context.Background()
	sem := semaphore.New(1)

	require.NoError(t, sem.AcquireN(ctx, 0))
	require.NoError(t, sem.AcquireN(ctx, -3))
	require.Equal(t, int64(1), sem.Permits(), "taking no permits must not consume anything")
	require.Zero(t, sem.Waiters())
}

func TestAcquireNHonoursContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sem := semaphore.New(2)
	require.NoError(t, sem.AcquireN(ctx, 2))

	errs := asyncAcquire(sem, ctx, 2)
	requireWaiters(t, sem, 1, "AcquireN must queue while no permit is free")

	cancel()
	err := awaitAcquire(t, errs, "AcquireN did not return when the context was cancelled")
	require.ErrorIs(t, err, context.Canceled)

	require.Eventually(t, func() bool { return sem.Waiters() == 0 }, time.Second, time.Millisecond,
		"a cancelled caller must leave the waiter queue")
	require.Zero(t, sem.Permits(), "the cancelled request must not consume a permit")

	sem.Release(2)
	require.Equal(t, int64(2), sem.Permits(), "the still-held permits must be intact")
}

func TestAcquireNReturnsDeadlineExceeded(t *testing.T) {
	ctx := context.Background()
	sem := semaphore.New(1)
	require.NoError(t, sem.Acquire(ctx))

	deadline, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	err := sem.AcquireN(deadline, 1)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	require.Zero(t, sem.Permits(), "a deadline must not consume a permit")
	require.Zero(t, sem.Waiters(), "a timed-out caller must leave the waiter queue")
}

func TestAcquireReturnsContextErrorBeforeBlocking(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sem := semaphore.New(1)

	err := sem.Acquire(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, int64(1), sem.Permits(), "a cancelled caller must not take a free permit")
	require.Zero(t, sem.Waiters())
}

func TestTryAcquireReportsFalseWhenEmpty(t *testing.T) {
	sem := semaphore.New(1)

	require.True(t, sem.TryAcquire(), "the first try must take the free permit")
	require.Zero(t, sem.Permits())
	require.False(t, sem.TryAcquire(), "TryAcquire must not block or succeed when empty")
	require.False(t, sem.TryAcquireN(1), "TryAcquireN must not block either")
	require.Zero(t, sem.Waiters(), "a failed try must not queue the caller")
}

func TestTryAcquireNRejectsMoreThanLimit(t *testing.T) {
	sem := semaphore.New(3)

	require.False(t, sem.TryAcquireN(4), "a 3-permit semaphore can never satisfy a request for 4")
	require.Equal(t, int64(3), sem.Permits(), "a failed try must not consume permits")

	sem.Release(10)
	require.Equal(t, int64(13), sem.Permits())
	require.False(t, sem.TryAcquireN(4), "extra released permits must not raise Limit")
	require.True(t, sem.TryAcquireN(3))
}

func TestTryAcquireNZeroIsAlwaysSatisfiable(t *testing.T) {
	sem := semaphore.New(1)

	require.True(t, sem.TryAcquireN(0), "taking no permits must succeed")
	require.True(t, sem.TryAcquireN(-2), "taking no permits must succeed")
	require.Equal(t, int64(1), sem.Permits(), "taking no permits must not consume anything")
}

func TestTryAcquireBargesPastQueuedWaiters(t *testing.T) {
	ctx := context.Background()
	sem := semaphore.New(4)
	require.True(t, sem.TryAcquireN(3), "leave one permit free")

	errs := asyncAcquire(sem, ctx, 4)
	requireWaiters(t, sem, 1, "the 4-permit request must queue")

	require.True(t, sem.TryAcquire(), "TryAcquire does not check the queue, so it may barge")
	require.Zero(t, sem.Permits())
	require.Equal(t, 1, sem.Waiters(), "barging must not remove the queued caller")

	sem.Release(4)
	require.NoError(t, awaitAcquire(t, errs, "the queued caller must be served once 4 permits are free"))
}

func TestAcquireTimeoutReportsDeadlineWithoutPermit(t *testing.T) {
	sem := semaphore.New(1)
	require.True(t, sem.TryAcquire(), "exhaust the semaphore")

	start := time.Now()
	require.False(t, sem.AcquireTimeout(1, 20*time.Millisecond),
		"no permit can appear, so AcquireTimeout must report false")
	require.GreaterOrEqual(t, time.Since(start), 10*time.Millisecond,
		"AcquireTimeout must wait for the deadline instead of failing immediately")
	require.Equal(t, int64(1), sem.Limit())
	require.Zero(t, sem.Waiters(), "the timed-out waiter must leave the queue")
}

func TestAcquireTimeoutTakesAFreePermit(t *testing.T) {
	sem := semaphore.New(1)

	require.True(t, sem.AcquireTimeout(1, time.Second), "a free permit must be taken well before the deadline")
	require.Zero(t, sem.Permits())
	require.True(t, sem.AcquireTimeout(0, time.Second), "taking no permits must succeed immediately")
}

func TestAcquireTimeoutWithNonPositiveDeadlineTriesImmediately(t *testing.T) {
	sem := semaphore.New(1)

	require.True(t, sem.AcquireTimeout(1, 0), "a zero deadline still takes an available permit")
	require.False(t, sem.AcquireTimeout(1, 0), "with nothing free and no time to wait it must report false")
	require.Zero(t, sem.Waiters())
}

func TestAcquireTimeoutSucceedsWhenPermitFreesInTime(t *testing.T) {
	ctx := context.Background()
	sem := semaphore.New(1)
	require.NoError(t, sem.Acquire(ctx))

	ok := make(chan bool, 1)
	go func() { ok <- sem.AcquireTimeout(1, time.Second) }()
	requireWaiters(t, sem, 1, "AcquireTimeout must queue while no permit is free")

	sem.Release(1)
	select {
	case got := <-ok:
		require.True(t, got, "AcquireTimeout must report true once the permit freed in time")
	case <-time.After(time.Second):
		t.Fatal("AcquireTimeout was not unblocked by Release")
	}
	require.Zero(t, sem.Permits(), "the woken caller must own the permit")
}

func TestWaitersAreServedInOrder(t *testing.T) {
	ctx := context.Background()
	sem := semaphore.New(1)
	require.NoError(t, sem.Acquire(ctx), "the test holds the only permit")

	const waiters = 3
	type grant struct {
		id  int
		err error
	}
	grants := make(chan grant, waiters)
	for i := range waiters {
		go func() { grants <- grant{id: i, err: sem.Acquire(ctx)} }()
		// wait for this waiter to be queued before starting the next one, so
		// arrival order is the order under test
		requireWaiters(t, sem, i+1, "waiters must queue in arrival order")
	}

	for i := range waiters {
		sem.Release(1)
		select {
		case g := <-grants:
			require.NoError(t, g.err)
			require.Equal(t, i, g.id, "releases must wake waiters in arrival order")
		case <-time.After(time.Second):
			t.Fatalf("waiter #%d was never served", i)
		}
	}
	require.Zero(t, sem.Permits())
	require.Zero(t, sem.Waiters())
}

func TestReleaseZeroIsNoOp(t *testing.T) {
	sem := semaphore.New(2)
	require.True(t, sem.TryAcquire())

	sem.Release(0)
	require.Equal(t, int64(1), sem.Permits(), "Release(0) must not change the permit count")
}

func TestReleaseAboveLimitIsAllowed(t *testing.T) {
	sem := semaphore.New(1)

	sem.Release(4)
	require.Equal(t, int64(5), sem.Permits(),
		"Release may push Permits above Limit; balancing it is the caller's responsibility")
	require.Equal(t, int64(1), sem.Limit(), "the configured limit must not change")
	require.True(t, sem.TryAcquire(), "the extra permits are usable")
}

func TestReleasePanicsOnNegativeCount(t *testing.T) {
	sem := semaphore.New(1)

	require.Panics(t, func() { sem.Release(-1) }, "a negative release must panic")
	require.Panics(t, func() { sem.Release(-100) })
	require.Equal(t, int64(1), sem.Permits(), "a rejected release must not change the permit count")
}

func TestNewPanicsOnNonPositivePermits(t *testing.T) {
	require.Panics(t, func() { semaphore.New(0) }, "a zero-permit semaphore can never be acquired")
	require.Panics(t, func() { semaphore.New(-1) })
}

func TestConcurrentAcquireNeverExceedsLimit(t *testing.T) {
	const (
		permits = 4
		workers = 32
		rounds  = 50
	)
	ctx := context.Background()
	sem := semaphore.New(permits)

	var (
		live     atomic.Int64 // callers currently holding a permit
		peak     atomic.Int64
		exceeded atomic.Bool
		wg       sync.WaitGroup
	)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				if err := sem.Acquire(ctx); err != nil {
					exceeded.Store(true) // recorded, asserted after the join
					return
				}
				held := live.Add(1)
				if held > permits {
					exceeded.Store(true)
				}
				if held > peak.Load() {
					peak.Store(held)
				}
				live.Add(-1)
				sem.Release(1)
			}
		}()
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the workers did not finish; the semaphore may be leaking permits")
	}

	require.False(t, exceeded.Load(), "more than %d callers held a permit at the same time", permits)
	require.LessOrEqual(t, peak.Load(), int64(permits), "peak concurrency must never exceed Limit")
	require.Equal(t, int64(permits), sem.Permits(), "every permit must be back once the workers finish")
	require.Zero(t, sem.Waiters())
}
