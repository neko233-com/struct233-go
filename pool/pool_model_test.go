package pool_test

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/pool"
)

// poolModel is the reference implementation of the pool semantics: a LIFO
// stack with a maximum idle size and cumulative counters. Every differential
// test replays an operation sequence against both the model and a real pool
// and compares the results after every step.
type poolModel struct {
	stack    []int // object ids, top = the next one out (LIFO)
	max      int   // 0 = unbounded
	gets     int64
	hits     int64
	misses   int64
	puts     int64
	discards int64
	nextID   int
}

func newPoolModel(max int) *poolModel { return &poolModel{max: max} }

// alloc hands out one fresh id.
func (m *poolModel) alloc() int { m.nextID++; return m.nextID }

// getHit pops the top id, counting a hit.
func (m *poolModel) getHit() int {
	m.gets++
	m.hits++
	id := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	return id
}

// getMiss counts a miss and returns the fresh id a factory build would get.
func (m *poolModel) getMiss() int {
	m.gets++
	m.misses++
	return m.alloc()
}

// put returns an id to the stack, dropping it when the idle set is full.
func (m *poolModel) put(id int) {
	if m.max > 0 && len(m.stack) >= m.max {
		m.discards++
		return
	}
	m.stack = append(m.stack, id)
	m.puts++
}

// prealloc allocates n fresh ids (the factory builds every one) and stores
// only as many as fit, mirroring the source. It returns every built id so the
// real factory can be staged with the same values.
func (m *poolModel) prealloc(n int) []int {
	ids := make([]int, 0, n)
	for range n {
		ids = append(ids, m.alloc())
	}
	for _, id := range ids {
		if m.max > 0 && len(m.stack) >= m.max {
			break
		}
		m.stack = append(m.stack, id)
	}
	return ids
}

// clear drops every idle id without touching the counters.
func (m *poolModel) clear() { m.stack = nil }

// stats mirrors pool.Stats for comparison.
func (m *poolModel) stats() pool.Stats {
	return pool.Stats{
		Gets:     m.gets,
		Hits:     m.hits,
		Misses:   m.misses,
		Puts:     m.puts,
		Discards: m.discards,
	}
}

// tracked wraps a pooled value with the model id so both sides can be
// compared after Get/Put sequences.
type tracked struct {
	id  int
	buf []byte
}

// dualPool runs one real pool and one reference model in lockstep. Ids for
// factory-built objects are staged into pending before the operation that
// triggers the build, so both sides always agree on object identity.
type dualPool struct {
	t       *testing.T
	p       *pool.Pool[*tracked]
	m       *poolModel
	pending []int // ids staged for the factory to consume, in order
}

func newDualPool(t *testing.T, max int) *dualPool {
	d := &dualPool{t: t, m: newPoolModel(max)}
	d.p = pool.New(func() *tracked {
		if len(d.pending) == 0 {
			t.Error("the factory ran without a staged id")
			return &tracked{id: -1}
		}
		id := d.pending[0]
		d.pending = d.pending[1:]
		return &tracked{id: id}
	}, pool.WithMaxSize(max), pool.WithReset(func(tr *tracked) { tr.buf = tr.buf[:0] }))
	return d
}

// compare asserts the observable state of both sides is identical.
func (d *dualPool) compare() {
	d.t.Helper()
	require.Equal(d.t, d.m.stats(), d.p.Stats(), "stats must match the model")
	require.Equal(d.t, len(d.m.stack), d.p.Len(), "idle count must match the model")
	require.Equal(d.t, d.m.stats().ReuseRate(), d.p.Stats().ReuseRate(),
		"reuse rate must match the model")
	require.Empty(d.t, d.pending, "every staged id must have been consumed")
}

// runOps replays a script of 'G' (Get), 'P' (Put), 'C' (Clear), 'X' (Prealloc
// one) and 'B' (Prealloc five) against both sides.
func (d *dualPool) runOps(ops string) {
	d.t.Helper()
	for step, op := range ops {
		switch op {
		case 'G':
			var wantID int
			if len(d.m.stack) > 0 {
				wantID = d.m.getHit()
			} else {
				wantID = d.m.getMiss()
				d.pending = append(d.pending, wantID)
			}
			got := d.p.Get()
			require.Equal(d.t, wantID, got.id,
				"step %d: Get must serve the model's object (LIFO on hits, fresh on misses)", step)
		case 'P':
			id := d.m.alloc()
			d.m.put(id)
			d.p.Put(&tracked{id: id})
		case 'C':
			d.m.clear()
			d.p.Clear()
		case 'X':
			d.pending = append(d.pending, d.m.prealloc(1)...)
			d.p.Prealloc(1)
		case 'B':
			d.pending = append(d.pending, d.m.prealloc(5)...)
			d.p.Prealloc(5)
		default:
			d.t.Fatalf("unknown op %q", op)
		}
		d.compare()
	}
}

// TestPoolModelLIFOSequences replays fixed Get/Put patterns against the stack
// model and checks hit order, eviction and the counters.
func TestPoolModelLIFOSequences(t *testing.T) {
	cases := []struct {
		name string
		max  int
		ops  string
	}{
		{"get miss then put hit", 0, "GPG"},
		{"lifo order", 0, "XXXGGG"},
		{"more gets than puts", 0, "GPGGG"},
		{"clear resets the stack", 0, "XXCGG"},
		{"max one discards", 1, "PPPG"},
		{"max two eviction", 2, "BBPPP"},
		{"prealloc overflow", 2, "BXB"},
		{"mixed churn", 4, "GXPGPXCBPG"},
		{"clear then reuse", 0, "XPCGPG"},
		{"empty get is a miss", 0, "GGG"},
		{"put after clear", 2, "XCPCG"},
		{"discard keeps the hot object", 2, "PGPPPGG"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newDualPool(t, tc.max).runOps(tc.ops)
		})
	}
}

// TestPoolModelRandomSeeds replays randomised operation scripts against the
// reference stack model.
func TestPoolModelRandomSeeds(t *testing.T) {
	opLetters := []rune("GPXCB")
	for seed := uint64(0); seed < 20; seed++ {
		rng := rand.New(rand.NewPCG(seed, seed*13+5))
		max := rng.IntN(4) // 0..3, 0 = unbounded
		ops := make([]rune, 24)
		for i := range ops {
			ops[i] = opLetters[rng.IntN(len(opLetters))]
		}
		t.Run(fmt.Sprintf("seed=%d/max=%d/%s", seed, max, string(ops)), func(t *testing.T) {
			newDualPool(t, max).runOps(string(ops))
		})
	}
}

// TestStatsAndReuseRateTable pins the counters and ReuseRate over scripted
// Get/Put histories.
func TestStatsAndReuseRateTable(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T, p *pool.Pool[*tracked])
		want pool.Stats
	}{
		{
			name: "fresh pool",
			run:  func(t *testing.T, p *pool.Pool[*tracked]) {},
			want: pool.Stats{},
		},
		{
			name: "one miss",
			run:  func(t *testing.T, p *pool.Pool[*tracked]) { p.Get() },
			want: pool.Stats{Gets: 1, Misses: 1},
		},
		{
			name: "miss then hit",
			run: func(t *testing.T, p *pool.Pool[*tracked]) {
				it := p.Get()
				p.Put(it)
				p.Get()
			},
			want: pool.Stats{Gets: 2, Hits: 1, Misses: 1, Puts: 1},
		},
		{
			name: "put over cap discards",
			run: func(t *testing.T, p *pool.Pool[*tracked]) {
				p.Put(&tracked{})
				p.Put(&tracked{})
				p.Put(&tracked{})
			},
			want: pool.Stats{Puts: 1, Discards: 2},
		},
		{
			name: "clear keeps counters and empties the idle set",
			run: func(t *testing.T, p *pool.Pool[*tracked]) {
				p.Put(&tracked{})
				p.Clear()
				p.Get()
			},
			want: pool.Stats{Gets: 1, Misses: 1, Puts: 1},
		},
		{
			// the pool has cap 1, so Prealloc(2) stores only one object:
			// the second Get is a miss, and no Put was ever counted
			name: "prealloc does not count puts",
			run: func(t *testing.T, p *pool.Pool[*tracked]) {
				p.Prealloc(2)
				p.Get()
				p.Get()
			},
			want: pool.Stats{Gets: 2, Hits: 1, Misses: 1},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := pool.New(func() *tracked { return &tracked{} }, pool.WithMaxSize(1))
			tc.run(t, p)
			got := p.Stats()
			require.Equal(t, tc.want, got)
			if got.Gets == 0 {
				require.Zero(t, got.ReuseRate(), "ReuseRate is 0 before the first Get")
			} else {
				require.InDelta(t, float64(got.Hits)/float64(got.Gets), got.ReuseRate(), 1e-9)
			}
		})
	}
}

// TestPreallocTable checks Prealloc against the maximum and the factory count.
func TestPreallocTable(t *testing.T) {
	cases := []struct {
		name      string
		max       int
		n         int
		wantLen   int
		wantBuild int64
	}{
		{"zero", 0, 0, 0, 0},
		{"unbounded", 0, 5, 5, 5},
		{"fits exactly", 3, 3, 3, 3},
		{"truncated at cap", 2, 5, 2, 5},
		{"cap one", 1, 4, 1, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var built atomic.Int64
			p := pool.New(func() *tracked {
				built.Add(1)
				return &tracked{}
			}, pool.WithMaxSize(tc.max))
			p.Prealloc(tc.n)
			require.Equal(t, tc.wantLen, p.Len())
			require.Equal(t, tc.wantBuild, built.Load(), "every requested object is built")
		})
	}
}

// TestCapAndLenTable checks Cap/Len reporting across bounded and unbounded
// pools.
func TestCapAndLenTable(t *testing.T) {
	cases := []struct {
		name    string
		max     int
		fill    int
		wantCap int
		wantLen int
	}{
		{"unbounded empty", 0, 0, 0, 0},
		{"unbounded filled", 0, 7, 0, 7},
		{"bounded empty", 3, 0, 3, 0},
		{"bounded at cap", 3, 5, 3, 3},
		{"bounded below cap", 5, 2, 5, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := pool.New(func() *tracked { return &tracked{} }, pool.WithMaxSize(tc.max))
			for range tc.fill {
				p.Put(&tracked{})
			}
			require.Equal(t, tc.wantCap, p.Cap())
			require.Equal(t, tc.wantLen, p.Len())
		})
	}
}

// TestResetHookTable exercises the reset hook contract: it runs on Put and on
// Clear drops, and even on discarded objects, but never on a Get hit.
func TestResetHookTable(t *testing.T) {
	cases := []struct {
		name string
		max  int
		ops  func(t *testing.T, p *pool.Pool[*tracked])
	}{
		{
			name: "put runs the hook",
			max:  0,
			ops: func(t *testing.T, p *pool.Pool[*tracked]) {
				it := p.Get()
				it.buf = []byte{1}
				p.Put(it)
				require.Empty(t, it.buf, "Put must clean the object")
			},
		},
		{
			name: "clear runs the hook on drops",
			max:  0,
			ops: func(t *testing.T, p *pool.Pool[*tracked]) {
				it := &tracked{buf: []byte{1}}
				p.Put(it)
				p.Clear()
				require.Empty(t, it.buf, "Clear must clean the dropped object")
			},
		},
		{
			name: "discard still runs the hook",
			max:  1,
			ops: func(t *testing.T, p *pool.Pool[*tracked]) {
				p.Put(&tracked{buf: []byte{1}})
				it := &tracked{buf: []byte{2}}
				p.Put(it) // dropped: the hook must have run anyway
				require.Empty(t, it.buf, "the hook runs even for discarded objects")
			},
		},
		{
			name: "get hit is not re-reset",
			max:  0,
			ops: func(t *testing.T, p *pool.Pool[*tracked]) {
				it := p.Get()
				it.buf = []byte{9}
				p.Put(it) // hook clears buf
				back := p.Get()
				back.buf = []byte{7}
				require.Equal(t, []byte{7}, back.buf, "a hit hands back the object as stored")
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := pool.New(func() *tracked { return &tracked{} },
				pool.WithMaxSize(tc.max), pool.WithReset(func(tr *tracked) { tr.buf = nil }))
			tc.ops(t, p)
		})
	}
}

// TestConcurrentReuseCount checks that concurrent Get/Put traffic keeps the
// accounting exact: every Get is one hit or one miss, every Put is stored or
// discarded, and the idle set never exceeds the cap.
func TestConcurrentReuseCount(t *testing.T) {
	const (
		goroutines = 8
		perWorker  = 500
	)
	p := pool.New(func() *tracked { return &tracked{} }, pool.WithMaxSize(8))

	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perWorker {
				it := p.Get()
				it.buf = append(it.buf[:0], byte(i))
				p.Put(it)
			}
		}()
	}
	wg.Wait()

	stats := p.Stats()
	require.Equal(t, int64(goroutines*perWorker), stats.Gets)
	require.Equal(t, stats.Gets, stats.Hits+stats.Misses, "every Get is a hit or a miss")
	require.Equal(t, int64(goroutines*perWorker), stats.Puts+stats.Discards,
		"every Put is stored or discarded")
	require.LessOrEqual(t, p.Len(), 8, "the idle set honours the cap")
}
