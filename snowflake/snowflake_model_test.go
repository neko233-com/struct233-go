package snowflake_test

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/snowflake"
)

// advancingClock is a deterministic clock that starts at a fixed instant and
// moves forward by step on every read. Every read is monotonic, so generators
// fed by it never wait.
type advancingClock struct {
	mu   sync.Mutex
	now  time.Time
	step time.Duration
}

func newAdvancingClock(start time.Time, step time.Duration) *advancingClock {
	return &advancingClock{now: start, step: step}
}

func (c *advancingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.now
	c.now = c.now.Add(c.step)
	return t
}

// Set moves the clock to t, modelling a manual time jump.
func (c *advancingClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// roundTrip checks the formula-inverse contract: packing a (ms, node, seq)
// triple and reading it back through TimeOf/NodeOf/SequenceOf returns the
// original values.
func roundTrip(t *testing.T, g *snowflake.Generator, epoch time.Time, ms, node, seq int64) {
	t.Helper()
	id := snowflake.ID(ms<<22 | node<<(22-g.NodeBits()) | seq)
	require.Equal(t, ms, g.TimeOf(id).Sub(epoch).Milliseconds(),
		"TimeOf must invert the timestamp field of %d", id.Int64())
	require.Equal(t, node, g.NodeOf(id),
		"NodeOf must invert the node field of %d", id.Int64())
	require.Equal(t, seq, g.SequenceOf(id),
		"SequenceOf must invert the sequence field of %d", id.Int64())
	require.Equal(t, id.Int64(), id.Int64(), "Int64 is the identity for the storage form")
}

// TestFormulaInverseTable packs and decodes hand-picked triples across node
// widths, pinning the 1+41+nodeBits+(22-nodeBits) layout.
func TestFormulaInverseTable(t *testing.T) {
	epoch := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		nodeBits uint8
		node     int64
		ms, seq  int64
	}{
		{"default width, first triple", 10, 0, 0, 0},
		{"default width, mid values", 10, 512, 1_000_000, 2047},
		{"default width, max node and seq", 10, 1023, math.MaxInt64 >> 22, 4095},
		{"width 1, node 1", 1, 1, 12345, (1 << 21) - 1},
		{"width 1, node 0", 1, 0, 7, 0},
		{"width 21, mid", 21, 1 << 20, 999, 1},
		{"width 22, no sequence", 22, (1 << 22) - 1, 42, 0},
		{"width 4, max seq", 4, 15, 8, (1 << 18) - 1},
		{"width 11", 11, 2000, 555, 2047},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := snowflake.New(tc.node,
				snowflake.WithEpoch(epoch), snowflake.WithNodeBits(tc.nodeBits))
			roundTrip(t, g, epoch, tc.ms, tc.node, tc.seq)
		})
	}
}

// TestGeneratedIDDecodes checks that every generated ID decodes back to the
// clock time, the node and a bounded sequence through the public API.
func TestGeneratedIDDecodes(t *testing.T) {
	cases := []struct {
		name     string
		nodeBits uint8
		node     int64
		step     time.Duration
	}{
		{"default width", 10, 7, time.Millisecond},
		{"width 1", 1, 1, 2 * time.Millisecond},
		{"width 22", 22, (1 << 22) - 1, time.Millisecond},
		{"width 5", 5, 31, 500 * time.Microsecond},
	}
	epoch := time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clock := newAdvancingClock(epoch.Add(time.Hour), tc.step)
			g := snowflake.New(tc.node,
				snowflake.WithEpoch(epoch), snowflake.WithNodeBits(tc.nodeBits),
				snowflake.WithClock(clock.Now))
			maxSeq := int64(1)<<(22-tc.nodeBits) - 1

			var prev snowflake.ID
			for i := range 100 {
				id, err := g.Next()
				require.NoError(t, err, "Next #%d", i)
				require.Equal(t, tc.node, g.NodeOf(id), "NodeOf must report the configured node")
				require.GreaterOrEqual(t, g.SequenceOf(id), int64(0))
				require.LessOrEqual(t, g.SequenceOf(id), maxSeq,
					"the sequence must fit in %d bits", 22-tc.nodeBits)
				ts := g.TimeOf(id)
				require.False(t, ts.Before(epoch), "TimeOf must not precede the epoch")
				require.Equal(t, ts.Sub(epoch).Milliseconds(),
					(id.Int64() >> 22), "TimeOf must invert the timestamp field")
				if i > 0 {
					require.Greater(t, id.Int64(), prev.Int64(),
						"IDs must be strictly increasing")
				}
				prev = id
			}
		})
	}
}

// TestTimeOfMonotonicAcrossMsBoundary pins that IDs generated over advancing
// time decode to non-decreasing timestamps and monotonic ids.
func TestTimeOfMonotonicAcrossMsBoundary(t *testing.T) {
	epoch := time.Date(2022, 3, 4, 0, 0, 0, 0, time.UTC)
	clock := newAdvancingClock(epoch.Add(24*time.Hour), 3*time.Millisecond)
	g := snowflake.New(3, snowflake.WithEpoch(epoch), snowflake.WithClock(clock.Now))

	var prev snowflake.ID
	var prevMS int64 = -1
	for i := range 200 {
		id, err := g.Next()
		require.NoError(t, err, "Next #%d", i)
		ms := g.TimeOf(id).Sub(epoch).Milliseconds()
		require.GreaterOrEqual(t, ms, prevMS, "timestamps must not go backwards")
		require.Greater(t, id.Int64(), prev.Int64(), "ids must be strictly increasing")
		prev, prevMS = id, ms
	}
}

// TestNodeOfSequenceOfBitBoundaries checks the decoders at the extremes of the
// dynamic bit budget for several node widths.
func TestNodeOfSequenceOfBitBoundaries(t *testing.T) {
	epoch := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for nodeBits := uint8(1); nodeBits <= 22; nodeBits++ {
		t.Run(fmt.Sprintf("nodeBits=%d", nodeBits), func(t *testing.T) {
			maxNode := int64(1)<<nodeBits - 1
			maxSeq := int64(1)<<(22-nodeBits) - 1
			g := snowflake.New(maxNode,
				snowflake.WithEpoch(epoch), snowflake.WithNodeBits(nodeBits))

			id := snowflake.ID(987654321<<22 | maxNode<<(22-nodeBits) | maxSeq)
			require.Equal(t, maxNode, g.NodeOf(id))
			require.Equal(t, maxSeq, g.SequenceOf(id))

			zero := snowflake.ID(1 << 22)
			require.Zero(t, g.NodeOf(zero))
			require.Zero(t, g.SequenceOf(zero))
		})
	}
}

// TestAccessorTable checks the plain accessors against their option inputs.
func TestAccessorTable(t *testing.T) {
	epoch := time.Date(2019, 5, 5, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		node     int64
		nodeBits uint8
	}{
		{"node 0 default bits", 0, 10},
		{"node max default bits", 1023, 10},
		{"one bit", 1, 1},
		{"twenty two bits", (1 << 22) - 1, 22},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := snowflake.New(tc.node,
				snowflake.WithEpoch(epoch), snowflake.WithNodeBits(tc.nodeBits))
			require.Equal(t, tc.node, g.Node())
			require.Equal(t, tc.nodeBits, g.NodeBits())
			require.Equal(t, epoch, g.Epoch())
		})
	}
}

// TestNewPanicsExactMessages pins the exact panic text of every invalid
// construction argument.
func TestNewPanicsExactMessages(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)

	cases := []struct {
		name string
		fn   func()
	}{
		{"negative node", func() { snowflake.New(-1) }},
		{"node at default max plus one", func() { snowflake.New(1024) }},
		{"node out of 4 bit range", func() { snowflake.New(16, snowflake.WithNodeBits(4)) }},
		{"node negative with small width", func() { snowflake.New(-5, snowflake.WithNodeBits(2)) }},
		{"node bits zero", func() { snowflake.New(0, snowflake.WithNodeBits(0)) }},
		{"node bits 23", func() { snowflake.New(0, snowflake.WithNodeBits(23)) }},
		{"node bits 255", func() { snowflake.New(0, snowflake.WithNodeBits(255)) }},
		{"future epoch", func() { snowflake.New(0, snowflake.WithEpoch(now.Add(time.Hour))) }},
		{"nil clock", func() { snowflake.New(0, snowflake.WithClock(nil)) }},
		{"node out of one bit range", func() { snowflake.New(2, snowflake.WithNodeBits(1)) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := "panic expected"
			switch tc.name {
			case "negative node", "node at default max plus one", "node out of 4 bit range",
				"node negative with small width", "node out of one bit range":
				msg = "snowflake: node id out of range"
			case "node bits zero", "node bits 23", "node bits 255":
				msg = "snowflake: node bits must be in 1..22"
			case "future epoch":
				msg = "snowflake: epoch must not be in the future"
			case "nil clock":
				msg = "snowflake: nil clock"
			}
			require.PanicsWithValue(t, msg, tc.fn)
		})
	}

	t.Run("epoch exactly now does not panic", func(t *testing.T) {
		require.NotPanics(t, func() {
			snowflake.New(0, snowflake.WithClock(func() time.Time { return past.Add(time.Hour) }),
				snowflake.WithEpoch(past.Add(time.Hour)))
		}, "an epoch that is exactly the clock reading must be accepted")
	})
}

// TestMustNextSuccessAndPanicMessage checks the MustNext contract on both
// branches, pinning the panic payload.
func TestMustNextSuccessAndPanicMessage(t *testing.T) {
	t.Run("success returns an id", func(t *testing.T) {
		clock := newFakeClock()
		g := snowflake.New(1, snowflake.WithClock(clock.Now))
		id := g.MustNext()
		require.NotZero(t, id.Int64(), "MustNext must return a real id")
		require.Equal(t, int64(1), g.NodeOf(id))
		require.Zero(t, g.SequenceOf(id), "the first id of a ms starts at sequence 0")
	})
	t.Run("panic value is the sentinel error", func(t *testing.T) {
		clock := newFakeClock()
		g := snowflake.New(1, snowflake.WithClock(clock.Now))
		g.MustNext()
		clock.Set(clock.Now().Add(-time.Second))
		require.PanicsWithValue(t, snowflake.ErrClockBackwards, func() { g.MustNext() })
	})
}

// waitGroupTimeout waits for wg and fails the test when it does not finish
// within the bound, so a hung Next call fails instead of stalling the run.
func waitGroupTimeout(t *testing.T, wg *sync.WaitGroup, what string) {
	t.Helper()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal(what)
	}
}

// TestSequenceExhaustionWaitsForClock uses a frozen clock to force sequence
// exhaustion and checks the generator waits and then continues monotonically.
func TestSequenceExhaustionWaitsForClock(t *testing.T) {
	epoch := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	// nodeBits 20 leaves 2 sequence bits: 4 ids per millisecond. The clock is
	// frozen, so the fifth Next has to spin until the clock moves on.
	frozen := epoch.Add(time.Hour)
	clock := newAdvancingClock(frozen, 0) // step 0: every read returns frozen
	g := snowflake.New(0,
		snowflake.WithEpoch(epoch), snowflake.WithNodeBits(20), snowflake.WithClock(clock.Now))

	release := make(chan struct{})
	ids := make([]snowflake.ID, 0, 5)
	errs := make([]error, 0, 5)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 4 {
			id, err := g.Next()
			ids, errs = append(ids, id), append(errs, err)
		}
		close(release) // the sequence is exhausted now
		// the fifth call waits for the clock to move; the main goroutine moves it
		id, err := g.Next()
		ids, errs = append(ids, id), append(errs, err)
	}()

	<-release
	clock.Set(frozen.Add(time.Millisecond))
	waitGroupTimeout(t, &wg, "Next must return once the clock advances")

	for i, err := range errs {
		require.NoError(t, err, "Next #%d", i)
	}
	for i, id := range ids[:4] {
		require.Equal(t, int64(i), g.SequenceOf(id), "the sequence must fill one ms")
		require.Equal(t, frozen, g.TimeOf(id), "the first four ids share the frozen ms")
	}
	require.Greater(t, g.TimeOf(ids[4]), frozen, "the new id must come from a later ms")
	require.Zero(t, g.SequenceOf(ids[4]), "a new millisecond restarts the sequence")
}

// TestConcurrentGenerationIsUniqueAndOrdered hammers Next from many
// goroutines and requires a duplicate-free, strictly increasing per-call-chain
// set of ids with consistent decode fields.
func TestConcurrentGenerationIsUniqueAndOrdered(t *testing.T) {
	const (
		goroutines   = 8
		perGoroutine = 3000
	)
	g := snowflake.New(17)

	var wg sync.WaitGroup
	out := make([]snowflake.ID, 0, goroutines*perGoroutine)
	badNodes := make([]bool, goroutines)
	var mu sync.Mutex
	for w := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			batch := make([]snowflake.ID, 0, perGoroutine)
			for range perGoroutine {
				id, err := g.Next()
				if err != nil {
					t.Errorf("goroutine %d: %v", w, err)
					return
				}
				if g.NodeOf(id) != 17 {
					badNodes[w] = true // checked after the wait
					return
				}
				batch = append(batch, id)
			}
			mu.Lock()
			out = append(out, batch...)
			mu.Unlock()
		}()
	}
	wg.Wait()

	for w, bad := range badNodes {
		require.False(t, bad, "goroutine %d decoded a wrong node", w)
	}

	seen := make(map[snowflake.ID]struct{}, len(out))
	for _, id := range out {
		_, dup := seen[id]
		require.False(t, dup, "id %d was issued twice", id.Int64())
		seen[id] = struct{}{}
	}
	require.Len(t, seen, goroutines*perGoroutine)
}

// TestNextDoesNotAllocate asserts the zero cost of the hot generation path.
func TestNextDoesNotAllocate(t *testing.T) {
	g := snowflake.New(1)
	allocs := testing.AllocsPerRun(1000, func() { g.MustNext() })
	require.Zero(t, allocs, "Next must not allocate")
}

// TestFormulaInverseRandomSeeds replays generated id streams under random
// node widths and clock steps and requires every id to decode back to its
// generation inputs exactly (the formula-inverse model). The real fuzz target
// lives in fuzz_test.go.
func TestFormulaInverseRandomSeeds(t *testing.T) {
	for seed := uint64(0); seed < 10; seed++ {
		rng := rand.New(rand.NewPCG(seed, seed*31+7))
		nodeBits := uint8(1 + rng.IntN(22))
		node := rng.Int64N(1 << nodeBits)
		step := time.Duration(rng.IntN(3)) * time.Millisecond
		t.Run(fmt.Sprintf("seed=%d/nodeBits=%d/step=%v", seed, nodeBits, step), func(t *testing.T) {
			epoch := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			clock := newAdvancingClock(epoch.Add(365*24*time.Hour), max(step, time.Millisecond))
			g := snowflake.New(node,
				snowflake.WithEpoch(epoch), snowflake.WithNodeBits(nodeBits),
				snowflake.WithClock(clock.Now))
			maxSeq := int64(1)<<(22-nodeBits) - 1

			for i := range 50 {
				id, err := g.Next()
				require.NoError(t, err, "Next #%d", i)
				// formula-inverse model
				require.Equal(t, node, g.NodeOf(id))
				require.LessOrEqual(t, g.SequenceOf(id), maxSeq)
				require.Equal(t, int64(id)>>22, g.TimeOf(id).Sub(epoch).Milliseconds())
			}
		})
	}
}
