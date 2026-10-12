package semaphore_test

import (
	"context"
	"math"
	"testing"

	"github.com/neko233-com/struct233-go/semaphore"
	"github.com/stretchr/testify/require"
)

func TestCancelledHeadGrantsAvailablePermitsToNextWaiter(t *testing.T) {
	s := semaphore.New(3)
	require.True(t, s.TryAcquireN(2))
	headCtx, cancelHead := context.WithCancel(context.Background())
	defer cancelHead()
	head := asyncAcquire(s, headCtx, 3)
	requireWaiters(t, s, 1, "head must be queued")
	nextCtx, cancelNext := context.WithCancel(context.Background())
	defer cancelNext()
	next := asyncAcquire(s, nextCtx, 1)
	requireWaiters(t, s, 2, "smaller request must wait behind the head")
	cancelHead()
	require.ErrorIs(t, awaitAcquire(t, head, "head did not cancel"), context.Canceled)
	require.NoError(t, awaitAcquire(t, next, "available permit was stranded after head cancelled"))
	require.Zero(t, s.Permits())
	require.Zero(t, s.Waiters())
}

func TestReleaseOverflowPreservesAvailablePermits(t *testing.T) {
	s := semaphore.New(math.MaxInt64)
	require.Panics(t, func() { s.Release(1) })
	require.Equal(t, int64(math.MaxInt64), s.Permits())
}
