package latch_test

import (
	"context"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/latch"
)

// TestLatchStateMatrix drives the Count/CountDown/Open/Await/AwaitTimeout
// matrix against the hand-written count-down state machine (see latchModel).
// Every row checks the observable state, both deterministic Await branches and
// the timeout branch.
func TestLatchStateMatrix(t *testing.T) {
	tests := []struct {
		name      string
		count     int
		downs     int
		wantCount int
		wantOpen  bool
	}{
		{"zero opens at once", 0, 0, 0, true},
		{"zero ignores one countdown", 0, 1, 0, true},
		{"zero ignores many countdowns", 0, 4, 0, true},
		{"single stays closed", 1, 0, 1, false},
		{"single opens on one down", 1, 1, 0, true},
		{"single overshoot", 1, 2, 0, true},
		{"single heavy overshoot", 1, 9, 0, true},
		{"two fresh", 2, 0, 2, false},
		{"two still closed after one", 2, 1, 1, false},
		{"two opens on both", 2, 2, 0, true},
		{"two overshoot", 2, 3, 0, true},
		{"three fresh", 3, 0, 3, false},
		{"three one down", 3, 1, 2, false},
		{"three two down", 3, 2, 1, false},
		{"three opens", 3, 3, 0, true},
		{"three overshoot", 3, 4, 0, true},
		{"four fresh", 4, 0, 4, false},
		{"four mid", 4, 2, 2, false},
		{"four shy", 4, 3, 1, false},
		{"four opens", 4, 4, 0, true},
		{"four overshoot", 4, 6, 0, true},
		{"five fresh", 5, 0, 5, false},
		{"five mid", 5, 3, 2, false},
		{"five opens", 5, 5, 0, true},
		{"five overshoot", 5, 7, 0, true},
		{"eight quarter", 8, 2, 6, false},
		{"eight half", 8, 4, 4, false},
		{"eight shy", 8, 7, 1, false},
		{"eight opens", 8, 8, 0, true},
		{"eight overshoot", 8, 10, 0, true},
		{"ten fifth", 10, 2, 8, false},
		{"ten half", 10, 5, 5, false},
		{"ten shy", 10, 9, 1, false},
		{"ten opens", 10, 10, 0, true},
		{"ten overshoot", 10, 11, 0, true},
		{"big shy", 100, 99, 1, false},
		{"big opens", 100, 100, 0, true},
		{"big overshoot", 100, 120, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := latch.New(tt.count)
			m := newLatchModel(tt.count)
			for range tt.downs {
				l.CountDown()
				m.countDown()
			}

			require.Equal(t, tt.wantCount, l.Count())
			require.Equal(t, tt.wantOpen, l.Open())
			require.Equal(t, m.count, l.Count(), "Count must agree with the state machine")
			require.Equal(t, m.opened, l.Open(), "Open must agree with the state machine")

			// Deterministic Await branches: the open fast path wins even over an
			// already-canceled context; a closed latch always observes the
			// cancellation.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if tt.wantOpen {
				require.NoError(t, l.Await(ctx), "the open fast path must win over a canceled context")
				require.NoError(t, l.Await(context.Background()))
			} else {
				require.ErrorIs(t, l.Await(ctx), context.Canceled)
			}
			require.Equal(t, tt.wantOpen, l.AwaitTimeout(2*time.Millisecond))
		})
	}
}

// TestLatchAwaitContextMatrix pins the Await error contract for the
// deterministic context states.
func TestLatchAwaitContextMatrix(t *testing.T) {
	tests := []struct {
		name    string
		open    bool
		ctx     func() (context.Context, context.CancelFunc)
		wantErr error
	}{
		{
			name: "open background", open: true,
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			wantErr: nil,
		},
		{
			name: "open beats canceled", open: true,
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			wantErr: nil,
		},
		{
			name: "open beats expired deadline", open: true,
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Unix(0, 0))
			},
			wantErr: nil,
		},
		{
			name: "closed observes cancel", open: false,
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			wantErr: context.Canceled,
		},
		{
			name: "closed observes expired deadline", open: false,
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Unix(0, 0))
			},
			wantErr: context.DeadlineExceeded,
		},
		{
			name: "closed with timeout context", open: false,
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), time.Millisecond)
			},
			wantErr: context.DeadlineExceeded,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := latch.New(1)
			if tt.open {
				l.CountDown()
			}
			ctx, cancel := tt.ctx()
			defer cancel()

			err := l.Await(ctx)
			if tt.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

// TestLatchAwaitTimeoutMatrix pins the AwaitTimeout boundary contract.
func TestLatchAwaitTimeoutMatrix(t *testing.T) {
	tests := []struct {
		name string
		open bool
		d    time.Duration
		want bool
	}{
		{"closed times out", false, time.Millisecond, false},
		{"closed zero duration", false, 0, false},
		{"closed negative duration", false, -time.Second, false},
		{"open returns at once", true, 0, true},
		{"open ignores duration", true, time.Hour, true},
		{"open negative duration", true, -time.Second, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := latch.New(1)
			if tt.open {
				l.CountDown()
			}
			require.Equal(t, tt.want, l.AwaitTimeout(tt.d))
		})
	}
}

// TestLatchNewPanics asserts the documented panic on a negative count.
func TestLatchNewPanics(t *testing.T) {
	tests := []struct {
		name  string
		count int
		msg   string
	}{
		{"negative one", -1, "latch: count must not be negative"},
		{"large negative", -100, "latch: count must not be negative"},
		{"min int", -1 << 40, "latch: count must not be negative"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.PanicsWithValue(t, tt.msg, func() { latch.New(tt.count) })
		})
	}
}

func TestOpensAfterConfiguredCountDowns(t *testing.T) {
	l := latch.New(3)
	require.Equal(t, 3, l.Count())
	require.False(t, l.Open(), "a fresh latch must be closed")

	for i := 1; i <= 3; i++ {
		l.CountDown()
		require.Equal(t, 3-i, l.Count(), "after CountDown #%d", i)
		if i < 3 {
			require.False(t, l.Open(), "the latch must stay closed until the count reaches zero")
		}
	}

	require.True(t, l.Open(), "the last CountDown must open the latch")
	require.NoError(t, l.Await(context.Background()), "Await on an open latch must return nil")
}

func TestAwaitReturnsImmediatelyWhenAlreadyOpen(t *testing.T) {
	l := latch.New(1)
	l.CountDown()

	done := make(chan error, 1)
	go func() { done <- l.Await(context.Background()) }()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Await must return immediately on an already-open latch")
	}
}

func TestCountNeverGoesNegative(t *testing.T) {
	l := latch.New(2)
	for range 5 {
		l.CountDown()
	}

	require.Zero(t, l.Count(), "CountDown calls past zero must be no-ops")
	require.True(t, l.Open())
}

func TestAwaitReturnsContextErrorWhenContextAlreadyDone(t *testing.T) {
	l := latch.New(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.ErrorIs(t, l.Await(ctx), context.Canceled)
}

func TestAwaitUnblocksOnContextCancel(t *testing.T) {
	l := latch.New(1)
	ctx, cancel := context.WithCancel(context.Background())
	released := make(chan error, 1)
	go func() { released <- l.Await(ctx) }()

	cancel()

	select {
	case err := <-released:
		require.ErrorIs(t, err, context.Canceled, "a blocked waiter must observe the cancellation")
	case <-time.After(time.Second):
		t.Fatal("Await must not block forever once the context is done")
	}
	require.False(t, l.Open(), "cancelling a waiter must not open the latch")
}

func TestAwaitTimeoutReportsWhetherTheLatchOpened(t *testing.T) {
	l := latch.New(1)

	require.False(t, l.AwaitTimeout(20*time.Millisecond), "a closed latch must report a timeout")
	l.CountDown()
	require.True(t, l.AwaitTimeout(time.Second), "an open latch must report true at once")
}

func TestWaitersAreReleasedTogetherByOneCountDown(t *testing.T) {
	const waiters = 8
	l := latch.New(1)

	var (
		wg   sync.WaitGroup
		errs = make([]error, waiters)
		open = make([]bool, waiters)
	)
	ready := make(chan struct{}, waiters)
	for i := range waiters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ready <- struct{}{}
			errs[i] = l.Await(context.Background())
			open[i] = l.Open()
		}()
	}
	for range waiters {
		<-ready // every waiter has reached Await
	}
	require.Equal(t, 1, l.Count(), "waiters must not open the latch")

	l.CountDown()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("one CountDown must release every waiter")
	}

	for i := range waiters {
		require.NoError(t, errs[i], "waiter %d must be released by CountDown", i)
		require.True(t, open[i], "waiter %d must observe the open latch", i)
	}
}

func TestZeroIsOpenImmediately(t *testing.T) {
	l := latch.New(0)
	require.Zero(t, l.Count())
	require.True(t, l.Open(), "a zero latch starts open")
	require.NoError(t, l.Await(context.Background()))
	require.True(t, l.AwaitTimeout(0))

	l.CountDown()
	require.Zero(t, l.Count(), "CountDown on an open latch must be a no-op")
	require.True(t, l.Open())
}

func TestPanicsOnNegativeCount(t *testing.T) {
	require.Panics(t, func() { latch.New(-1) })
	require.Panics(t, func() { latch.New(-100) })
}

// TestConcurrentAwaitAndCountDown races many Await/AwaitTimeout waiters
// against many concurrent CountDown calls and requires every waiter to be
// released exactly when the count reaches zero. Race-clean by construction:
// the latch is the only shared state and it is documented as concurrency-safe.
func TestConcurrentAwaitAndCountDown(t *testing.T) {
	const workers = 16
	l := latch.New(workers)

	var (
		waiters sync.WaitGroup
		errs    = make([]error, workers)
		timedIn = make([]bool, workers)
	)
	for i := range workers {
		waiters.Add(1)
		go func() {
			defer waiters.Done()
			if i%2 == 0 {
				errs[i] = l.Await(context.Background())
			} else {
				timedIn[i] = l.AwaitTimeout(5 * time.Second)
			}
		}()
	}

	var counters sync.WaitGroup
	for range workers {
		counters.Add(1)
		go func() {
			defer counters.Done()
			l.CountDown()
		}()
	}

	done := make(chan struct{})
	go func() { waiters.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("every waiter must be released once the count reaches zero")
	}
	counters.Wait()

	for i := range workers {
		if i%2 == 0 {
			require.NoError(t, errs[i], "waiter %d", i)
		} else {
			require.True(t, timedIn[i], "timed waiter %d must observe the open latch", i)
		}
	}
	require.Zero(t, l.Count())
	require.True(t, l.Open())
}

// TestConcurrentCountDownIsIdempotentPastZero hammers an over-counted latch
// from many goroutines and must never panic (the channel is closed once).
func TestConcurrentCountDownIsIdempotentPastZero(t *testing.T) {
	l := latch.New(2)
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.CountDown()
			_ = l.Count()
			_ = l.Open()
		}()
	}
	wg.Wait()
	require.Zero(t, l.Count())
	require.True(t, l.Open())
}

// TestLatchCountDoesNotAllocate guards the Count fast read-out.
func TestLatchCountDoesNotAllocate(t *testing.T) {
	l := latch.New(4)
	l.CountDown()

	allocs := testing.AllocsPerRun(100, func() { _ = l.Count() })
	require.Zero(t, allocs, "Count must not allocate")
}

// ---- reference model -------------------------------------------------------

// latchModel is the hand-written count-down state machine used for the
// differential tests: a count that decrements towards zero and a one-way
// opened flag.
type latchModel struct {
	count  int
	opened bool
}

// newLatchModel returns the model state right after New(count).
func newLatchModel(count int) *latchModel {
	return &latchModel{count: count, opened: count == 0}
}

// countDown mirrors Latch.CountDown.
func (m *latchModel) countDown() {
	if m.count == 0 {
		return
	}
	m.count--
	if m.count == 0 {
		m.opened = true
	}
}

// latchOpKind enumerates the deterministic operations replayed by the
// randomized differential test and the fuzz target. Every branch of Await is
// observable without blocking: the open fast path and the already-canceled /
// already-expired context path.
const (
	latchOpCountDown     = iota // CountDown
	latchOpAwaitCanceled        // Await with an already-canceled context
	latchOpAwaitTimeout         // AwaitTimeout(0): true when open, false otherwise
	latchOpCount                // Count
	latchOpOpen                 // Open
	latchOpKindCount
)

// applyLatchOp replays one operation on the real latch and the model and
// asserts they agree.
func applyLatchOp(t testing.TB, l *latch.Latch, m *latchModel, kind byte) {
	t.Helper()
	switch kind % latchOpKindCount {
	case latchOpCountDown:
		l.CountDown()
		m.countDown()
	case latchOpAwaitCanceled:
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // already canceled: makes the branch deterministic and non-blocking
		err := l.Await(ctx)
		if m.opened {
			require.NoError(t, err, "an open latch must ignore a canceled context")
		} else {
			require.ErrorIs(t, err, context.Canceled, "a closed latch must observe the cancellation")
		}
	case latchOpAwaitTimeout:
		require.Equal(t, m.opened, l.AwaitTimeout(0), "AwaitTimeout branch must match the model")
	case latchOpCount:
		require.Equal(t, m.count, l.Count())
	case latchOpOpen:
		require.Equal(t, m.opened, l.Open())
	}
}

// runLatchOps replays data as an operation stream against a latch created with
// initialCount and the state machine, checking agreement after every step.
func runLatchOps(t testing.TB, initialCount int, data []byte) {
	t.Helper()
	if initialCount < 0 {
		initialCount = 0
	}
	if len(data) > 256 {
		data = data[:256] // keep individual runs fast
	}
	l := latch.New(initialCount)
	m := newLatchModel(initialCount)
	for i, b := range data {
		applyLatchOp(t, l, m, b)
		require.Equal(t, m.count, l.Count(), "after op %d", i)
		require.Equal(t, m.opened, l.Open(), "after op %d", i)
	}
}

// TestLatchModelRandomized replays random operation streams against the
// count-down state machine.
func TestLatchModelRandomized(t *testing.T) {
	for _, seed := range [][2]uint64{{2026, 1012}, {7, 11}, {99, 4242}} {
		rng := rand.New(rand.NewPCG(seed[0], seed[1]))
		for _, initial := range []int{0, 1, 3, 8} {
			data := make([]byte, 128)
			for i := range data {
				data[i] = byte(rng.IntN(256))
			}
			runLatchOps(t, initial, data)
		}
	}
}
