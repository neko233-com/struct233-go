package barrier_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/barrier"
)

// testTimeout bounds every wait in this file so a broken rendezvous fails the
// test instead of hanging it.
const testTimeout = 2 * time.Second

// waitGroupTimeout waits for wg and fails the test when it does not finish in
// time.
func waitGroupTimeout(t *testing.T, wg *sync.WaitGroup, what string) {
	t.Helper()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal(what)
	}
}

// rendezvousInOrder runs one generation of b, starting the parties one at a
// time and waiting for every arrival to be registered first, so the arrival
// indices are deterministic. It returns each party's index and error.
func rendezvousInOrder(t *testing.T, b *barrier.Barrier) ([]int, []error) {
	t.Helper()
	parties := b.Parties()
	indices := make([]int, parties)
	errs := make([]error, parties)

	var wg sync.WaitGroup
	for i := range parties {
		wg.Add(1)
		go func() {
			defer wg.Done()
			indices[i], errs[i] = b.Await(context.Background())
		}()
		if i+1 < parties {
			require.Eventually(t, func() bool { return b.Waiting() == i+1 },
				testTimeout, time.Millisecond,
				"party %d must be waiting before party %d starts", i, i+1)
		}
	}
	waitGroupTimeout(t, &wg, "the generation did not complete in time")
	return indices, errs
}

// breakGeneration breaks the current generation of b by arriving with a context
// that is already done. It needs parties >= 2, otherwise the single arrival
// completes the generation.
func breakGeneration(t *testing.T, b *barrier.Barrier) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := b.Await(ctx)
	require.ErrorIs(t, err, context.Canceled, "the canceled arrival must break the generation")
}

func TestAwaitRendezvousReturnsArrivalIndices(t *testing.T) {
	b := barrier.New(3)

	indices, errs := rendezvousInOrder(t, b)

	for i, err := range errs {
		require.NoError(t, err, "party %d must rendezvous", i)
	}
	require.Equal(t, []int{0, 1, 2}, indices, "indices must follow the arrival order")
	require.Zero(t, b.Waiting(), "a completed generation has no waiting parties")
	require.False(t, b.Broken(), "a completed generation is not broken")
}

func TestBarrierIsCyclicAcrossGenerations(t *testing.T) {
	b := barrier.New(3)

	for round := range 3 {
		indices, errs := rendezvousInOrder(t, b)
		for i, err := range errs {
			require.NoError(t, err, "round %d party %d", round, i)
		}
		require.Equal(t, []int{0, 1, 2}, indices, "round %d must repeat the arrival order", round)
		require.Zero(t, b.Waiting(), "round %d must leave no waiter behind", round)
	}
}

func TestAwaitContextCancelBreaksTheGeneration(t *testing.T) {
	b := barrier.New(3)

	var (
		wg       sync.WaitGroup
		firstErr error
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, firstErr = b.Await(context.Background())
	}()
	require.Eventually(t, func() bool { return b.Waiting() == 1 },
		testTimeout, time.Millisecond, "party 0 must park before party 1 arrives")

	ctx, cancel := context.WithCancel(context.Background())
	var cancelErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, cancelErr = b.Await(ctx)
	}()
	require.Eventually(t, func() bool { return b.Waiting() == 2 },
		testTimeout, time.Millisecond, "party 1 must park before the generation is broken")

	cancel()
	waitGroupTimeout(t, &wg, "cancelling one waiter must wake the whole generation")

	require.ErrorIs(t, cancelErr, context.Canceled,
		"the canceling party must observe its own context error")
	require.ErrorIs(t, firstErr, barrier.ErrBroken,
		"the other waiter must observe ErrBroken")
	require.True(t, b.Broken(), "the broken generation must stay broken")
}

func TestAwaitWhileBrokenReturnsErrBroken(t *testing.T) {
	b := barrier.New(2)
	breakGeneration(t, b)
	require.True(t, b.Broken())

	_, err := b.Await(context.Background())
	require.ErrorIs(t, err, barrier.ErrBroken, "a broken barrier must reject arrivals until Reset")
}

func TestResetClearsBrokenAndLetsANewGenerationComplete(t *testing.T) {
	b := barrier.New(2)
	breakGeneration(t, b)
	require.True(t, b.Broken())

	b.Reset()
	require.False(t, b.Broken(), "Reset must clear the broken state")
	require.Zero(t, b.Waiting())

	indices, errs := rendezvousInOrder(t, b)
	for i, err := range errs {
		require.NoError(t, err, "party %d must rendezvous after Reset", i)
	}
	require.Equal(t, []int{0, 1}, indices)
}

func TestResetWakesWaitingPartiesWithErrBroken(t *testing.T) {
	b := barrier.New(2)

	var (
		wg  sync.WaitGroup
		err error
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err = b.Await(context.Background())
	}()
	require.Eventually(t, func() bool { return b.Waiting() == 1 },
		testTimeout, time.Millisecond, "the party must park before Reset")

	b.Reset()
	waitGroupTimeout(t, &wg, "Reset must wake the waiting parties")

	require.ErrorIs(t, err, barrier.ErrBroken,
		"Reset must report the abandoned generation as broken")
	require.False(t, b.Broken(), "the fresh generation must not be broken")
}

func TestAccessorsReportTheCurrentGeneration(t *testing.T) {
	b := barrier.New(2)
	require.Equal(t, 2, b.Parties())
	require.Zero(t, b.Waiting(), "a fresh barrier has no waiting parties")
	require.False(t, b.Broken(), "a fresh barrier is not broken")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.Await(context.Background())
	}()
	require.Eventually(t, func() bool { return b.Waiting() == 1 },
		testTimeout, time.Millisecond, "the party must park before it is counted")

	require.Equal(t, 1, b.Waiting(), "the parked party must be counted")
	require.False(t, b.Broken(), "waiting alone must not break the generation")

	b.Reset()
	waitGroupTimeout(t, &wg, "Reset must release the parked party")
	require.Zero(t, b.Waiting())
}

func TestPanicsOnInvalidParties(t *testing.T) {
	require.Panics(t, func() { barrier.New(0) })
	require.Panics(t, func() { barrier.New(-1) })
}
