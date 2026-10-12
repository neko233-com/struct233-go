package barrier_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/barrier"
)

// barrierResult is what one Await call reported.
type barrierResult struct {
	index int
	err   error
}

// barrierHarness drives a real Barrier and a reference cyclic-generation state
// machine in lockstep. The model is deliberately sequential: arrivals register
// one at a time (the harness waits for Waiting() to match the model before the
// next step), so every expected outcome is deterministic.
//
// The model states mirror the source semantics of one generation:
//
//	broken                -> every arrival answers (0, ErrBroken)
//	last arrival          -> completes the generation with (index, nil), even
//	                         with a cancelled ctx, waking parked waiters with
//	                         (their index, nil)
//	cancelled non-last    -> answers (0, ctx.Err()), parked waiters wake with
//	                         (0, ErrBroken), the barrier stays broken
//	plain arrival         -> parks with its 0-based arrival index
//	Reset                 -> parked waiters wake with (0, ErrBroken) and the
//	                         state returns to a fresh unbroken generation
type barrierHarness struct {
	t *testing.T
	b *barrier.Barrier

	parties int
	waiting int
	broken  bool
	waiters []chan barrierResult // parked arrivals, in arrival order
}

func newBarrierHarness(t *testing.T, parties int) *barrierHarness {
	return &barrierHarness{t: t, b: barrier.New(parties), parties: parties}
}

// recv waits for one Await outcome, failing the test when it never arrives.
func (h *barrierHarness) recv(ch chan barrierResult, what string) barrierResult {
	h.t.Helper()
	select {
	case got := <-ch:
		return got
	case <-time.After(testTimeout):
		h.t.Fatal(what)
		return barrierResult{}
	}
}

// drainWaiters checks that every parked arrival woke with the modeled outcome.
func (h *barrierHarness) drainWaiters(broken bool) {
	h.t.Helper()
	for i, ch := range h.mWaiters() {
		got := h.recv(ch, "a parked waiter never woke")
		if broken {
			require.Zero(h.t, got.index, "a broken waiter gets index 0")
			require.ErrorIs(h.t, got.err, barrier.ErrBroken, "waiter %d", i)
		} else {
			require.Equal(h.t, i, got.index, "waiter %d index", i)
			require.NoError(h.t, got.err, "waiter %d", i)
		}
	}
	h.waiters = nil
}

func (h *barrierHarness) mWaiters() []chan barrierResult { return h.waiters }

// assertState checks the observable state against the model.
func (h *barrierHarness) assertState() {
	h.t.Helper()
	require.Equal(h.t, h.waiting, h.b.Waiting(), "Waiting must match the model")
	require.Equal(h.t, h.broken, h.b.Broken(), "Broken must match the model")
}

// arrive runs one Await (plain or with an already cancelled ctx) and checks
// the outcome and the woken waiters against the model.
func (h *barrierHarness) arrive(cancelled bool) {
	h.t.Helper()
	ctx := context.Background()
	if cancelled {
		cctx, cancel := context.WithCancel(context.Background())
		cancel()
		ctx = cctx
	}
	ch := make(chan barrierResult, 1)
	go func() {
		idx, err := h.b.Await(ctx)
		ch <- barrierResult{idx, err}
	}()

	if h.broken {
		got := h.recv(ch, "an arrival on a broken barrier must answer immediately")
		require.Zero(h.t, got.index)
		require.ErrorIs(h.t, got.err, barrier.ErrBroken)
		return
	}

	if h.waiting+1 == h.parties {
		// the last arrival completes the generation regardless of ctx
		got := h.recv(ch, "the completing arrival must answer")
		require.Equal(h.t, h.waiting, got.index, "the last arrival reports its index")
		require.NoError(h.t, got.err, "completing a generation is never an error")
		h.drainWaiters(false)
		h.waiting = 0
		return
	}

	if cancelled {
		got := h.recv(ch, "the cancelled arrival must answer")
		require.Zero(h.t, got.index)
		require.ErrorIs(h.t, got.err, context.Canceled, "the cancelling party sees its own ctx error")
		h.drainWaiters(true)
		h.waiting = 0
		h.broken = true
		return
	}

	// plain arrival: it parks with the next index
	index := h.waiting
	h.waiters = append(h.waiters, ch)
	h.waiting++
	require.Eventually(h.t, func() bool { return h.b.Waiting() == h.waiting },
		testTimeout, time.Millisecond,
		"arrival %d must register in the current generation", index)
}

// reset runs Reset and checks the parked waiters wake broken.
func (h *barrierHarness) reset() {
	h.t.Helper()
	h.b.Reset()
	h.drainWaiters(true)
	h.waiting = 0
	h.broken = false
}

// runBarrierScript replays ops ('A' plain arrival, 'C' cancelled arrival,
// 'R' reset) against the harness and its model.
func runBarrierScript(t *testing.T, parties int, ops string) {
	t.Helper()
	h := newBarrierHarness(t, parties)
	for _, op := range ops {
		switch op {
		case 'A':
			h.arrive(false)
		case 'C':
			h.arrive(true)
		case 'R':
			h.reset()
		default:
			t.Fatalf("unknown op %q", op)
		}
		h.assertState()
	}
	// cleanup: never leave a parked waiter behind, so no script can hang
	if h.waiting > 0 {
		h.reset()
	}
}

// TestNewPanicsExactMessages pins the panic text of an invalid party count.
func TestNewPanicsExactMessages(t *testing.T) {
	cases := []struct {
		name    string
		parties int
	}{
		{"zero", 0},
		{"negative one", -1},
		{"large negative", -1 << 30},
		{"min int", math.MinInt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.PanicsWithValue(t, "barrier: parties must be at least 1",
				func() { barrier.New(tc.parties) })
		})
	}
}

// TestPartiesTable checks the accessors of a fresh barrier over party counts.
func TestPartiesTable(t *testing.T) {
	for _, parties := range []int{1, 2, 3, 17, 100} {
		t.Run(fmt.Sprintf("parties=%d", parties), func(t *testing.T) {
			b := barrier.New(parties)
			require.Equal(t, parties, b.Parties())
			require.Zero(t, b.Waiting(), "a fresh barrier has no waiters")
			require.False(t, b.Broken(), "a fresh barrier is not broken")
		})
	}
}

// TestSinglePartyAwaitTable runs a party of one through several rounds: every
// arrival is the last arrival, so every round completes with index 0.
func TestSinglePartyAwaitTable(t *testing.T) {
	cases := []struct {
		name    string
		parties int
		rounds  int
	}{
		{"one round", 1, 1},
		{"two rounds", 1, 2},
		{"five rounds", 1, 5},
		{"two parties two rounds", 2, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newBarrierHarness(t, tc.parties)
			for round := range tc.rounds {
				// complete one generation with exactly parties arrivals
				for range tc.parties {
					h.arrive(false)
				}
				h.assertState()
				require.Zero(t, h.b.Waiting(), "round %d must leave no waiter", round)
			}
		})
	}
}

// TestBrokenStateTable covers the broken contract: after a break every arrival
// is rejected and only Reset repairs the barrier.
func TestBrokenStateTable(t *testing.T) {
	cases := []struct {
		name    string
		parties int
		ops     string
	}{
		{"break then arrive", 2, "CR"},
		{"break then arrive twice", 2, "CAAR"},
		{"double break", 2, "CC"},
		{"break then reset then break", 2, "CRC"},
		{"parked then break then reset", 3, "ACR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runBarrierScript(t, tc.parties, tc.ops)
		})
	}
}

// TestBarrierModelScenarios enumerates every 3-step script over arrivals,
// cancellations and resets for parties 1 and 2 and replays it against the
// reference generation state machine.
func TestBarrierModelScenarios(t *testing.T) {
	letters := []byte("ACR")
	var scripts []string
	for _, a := range letters {
		for _, b := range letters {
			for _, c := range letters {
				scripts = append(scripts, string([]byte{a, b, c}))
			}
		}
	}
	for _, parties := range []int{1, 2} {
		for _, script := range scripts {
			t.Run(fmt.Sprintf("parties=%d/%s", parties, script), func(t *testing.T) {
				runBarrierScript(t, parties, script)
			})
		}
	}
}

// TestBarrierModelRandomSeeds replays randomised operation sequences against
// the reference model.
func TestBarrierModelRandomSeeds(t *testing.T) {
	for seed := uint64(0); seed < 15; seed++ {
		rng := rand.New(rand.NewPCG(seed, seed*7+1))
		parties := 1 + rng.IntN(3)
		ops := make([]byte, 16)
		for i := range ops {
			ops[i] = "ACR"[rng.IntN(3)]
		}
		t.Run(fmt.Sprintf("seed=%d/parties=%d/%s", seed, parties, ops), func(t *testing.T) {
			runBarrierScript(t, parties, string(ops))
		})
	}
}

// TestConcurrentCyclicGenerations runs many goroutines through many rounds of
// the same barrier; every generation must complete with the full party count.
func TestConcurrentCyclicGenerations(t *testing.T) {
	const (
		parties = 4
		rounds  = 25
	)
	b := barrier.New(parties)
	seen := make([][]int, rounds)
	var mu sync.Mutex

	var wg sync.WaitGroup
	for p := range parties {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range rounds {
				idx, err := b.Await(context.Background())
				if err != nil {
					t.Errorf("party %d round %d: %v", p, r, err)
					return
				}
				mu.Lock()
				seen[r] = append(seen[r], idx)
				mu.Unlock()
			}
		}()
	}
	waitGroupTimeout(t, &wg, "the cyclic generations did not complete")

	for r, indices := range seen {
		slices.Sort(indices)
		require.Equal(t, []int{0, 1, 2, 3}, indices,
			"round %d must see every arrival index exactly once", r)
	}
}

// TestConcurrentResetDuringAwait lets a resetter break generations at random
// while workers keep arriving: every Await must answer nil or ErrBroken and
// the barrier must be usable again once the resetter stops.
func TestConcurrentResetDuringAwait(t *testing.T) {
	const workers = 4
	b := barrier.New(workers)

	stop := make(chan struct{})
	var resetter sync.WaitGroup
	resetter.Add(1)
	go func() {
		defer resetter.Done()
		for {
			select {
			case <-stop:
				return
			default:
				b.Reset()
				time.Sleep(time.Millisecond)
			}
		}
	}()

	var wg sync.WaitGroup
	for p := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				_, err := b.Await(context.Background())
				if err != nil && !errors.Is(err, barrier.ErrBroken) {
					t.Errorf("party %d got %v, want nil or ErrBroken", p, err)
					return
				}
			}
		}()
	}
	waitGroupTimeout(t, &wg, "the workers did not drain their arrivals")

	close(stop)
	resetter.Wait()

	// with the resetter gone a fresh generation completes normally
	b.Reset()
	indices, errs := rendezvousInOrder(t, b)
	for i, err := range errs {
		require.NoError(t, err, "party %d must rendezvous after the resets", i)
	}
	require.Equal(t, []int{0, 1, 2, 3}, indices)
}

// TestBarrierAccessorsDoNotAllocate asserts the zero cost of the read paths.
func TestBarrierAccessorsDoNotAllocate(t *testing.T) {
	b := barrier.New(2)
	allocs := testing.AllocsPerRun(1000, func() {
		_ = b.Parties()
		_ = b.Waiting()
		_ = b.Broken()
	})
	require.Zero(t, allocs, "the accessors must not allocate")
}

// TestAwaitAllocationBudget records what a completed generation costs: the
// barrier allocates one genState and one release channel per generation by
// design, so only a small budget is asserted, not zero.
func TestAwaitAllocationBudget(t *testing.T) {
	b := barrier.New(1)
	ctx := context.Background()
	allocs := testing.AllocsPerRun(100, func() {
		if _, err := b.Await(ctx); err != nil {
			t.Fatal(err)
		}
	})
	t.Logf("allocs per completed single-party generation: %v", allocs)
	require.LessOrEqual(t, allocs, 4.0,
		"a completed generation must only allocate its genState and channel")
}
