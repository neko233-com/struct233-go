package bloomfilter_test

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/bloomfilter"
)

// bfModel is the reference model for the differential tests: the set of
// elements added since the last Clear plus the Add-call count. The filter may
// report false positives, but it must never report a false negative and its
// Count must match the model exactly.
type bfModel struct {
	present map[string]bool
	count   uint64
}

func newBFModel() *bfModel { return &bfModel{present: map[string]bool{}} }

func (m *bfModel) add(key string) {
	m.present[key] = true
	m.count++
}

func (m *bfModel) clear() {
	m.present = map[string]bool{}
	m.count = 0
}

// bfUniverse bounds the element pool of the randomized and fuzzed operation
// streams so that hash collisions between modelled elements really happen and
// the no-false-negative property gets a real workout.
const bfUniverse = 16

// The three element kinds hash differently (the second base hash mixes in a
// kind-specific seed), so the same payload is a distinct element per kind and
// gets its own model key.
func bfStringKey(i int) string { return "s" + strconv.Itoa(i) }
func bfBytesKey(i int) string  { return "b" + strconv.Itoa(i) }
func bfUint64Key(i int) string { return "u" + strconv.Itoa(i) }

func bfStringVal(i int) string { return "item-" + strconv.Itoa(i) }
func bfBytesVal(i int) []byte  { return []byte("item-" + strconv.Itoa(i)) }
func bfUint64Val(i int) uint64 { return uint64(i) }

// bfOp is one replayable mutation of the bloom-filter model test.
type bfOp struct {
	kind byte // 0 add string, 1 add bytes, 2 add uint64, 3 probe string, 4 clear
	idx  int
}

// decodeBFOps turns a byte stream into an operation sequence, three bytes per
// operation. It is shared by the seeded differential test and the fuzz target.
func decodeBFOps(data []byte) []bfOp {
	ops := make([]bfOp, 0, 64)
	for i := 0; i+2 < len(data) && len(ops) < 64; i += 3 {
		ops = append(ops, bfOp{kind: data[i] % 5, idx: int(data[i+1]) % bfUniverse})
	}
	return ops
}

// runBFModel replays ops against a real filter and the reference model. After
// every operation the Count must match and the whole universe is swept for
// false negatives; false positives are allowed and never asserted against here
// (the false-positive budget is exercised by TestFalsePositiveStatistics).
func runBFModel(t *testing.T, ops []bfOp) {
	t.Helper()
	// deliberately small and permissive: plenty of bit collisions, the
	// properties under test must hold regardless
	b := bloomfilter.New(64, 0.5)
	m := newBFModel()
	for i, o := range ops {
		switch o.kind {
		case 0:
			b.AddString(bfStringVal(o.idx))
			m.add(bfStringKey(o.idx))
		case 1:
			b.AddBytes(bfBytesVal(o.idx))
			m.add(bfBytesKey(o.idx))
		case 2:
			b.AddUint64(bfUint64Val(o.idx))
			m.add(bfUint64Key(o.idx))
		case 3:
			if m.present[bfStringKey(o.idx)] {
				require.True(t, b.TestString(bfStringVal(o.idx)),
					"op %d: false negative on string %d", i, o.idx)
			}
		case 4:
			b.Clear()
			m.clear()
		}
		require.Equal(t, m.count, b.Count(), "op %d: Count", i)
		require.Equal(t, m.count == 0, b.FalsePositiveRate() == 0,
			"op %d: FalsePositiveRate must be zero exactly on an empty filter", i)
		for j := range bfUniverse {
			if m.present[bfStringKey(j)] {
				require.True(t, b.TestString(bfStringVal(j)), "op %d: false negative on string %d", i, j)
			}
			if m.present[bfBytesKey(j)] {
				require.True(t, b.TestBytes(bfBytesVal(j)), "op %d: false negative on bytes %d", i, j)
			}
			if m.present[bfUint64Key(j)] {
				require.True(t, b.TestUint64(bfUint64Val(j)), "op %d: false negative on uint64 %d", i, j)
			}
		}
	}
}

// TestNewSizing pins the m/k sizing formula on its well-known sweet spots.
func TestNewSizing(t *testing.T) {
	tests := []struct {
		name    string
		n       int
		rate    float64
		wantK   int
		wantLen uint64
		delta   uint64
	}{
		{"one element 50%", 1, 0.5, 44, 64, 0},
		{"one element 1%", 1, 0.01, 44, 64, 0},
		{"100 elements 50%", 100, 0.5, 1, 145, 2},
		{"1000 elements 1%", 1000, 0.01, 7, 9586, 2},
		{"1000 elements 10%", 1000, 0.1, 3, 4793, 2},
		{"10000 elements 1%", 10000, 0.01, 7, 95851, 4},
		{"10000 elements 0.1%", 10000, 0.001, 10, 143774, 4},
		{"million elements 1e-6", 1 << 20, 1e-6, 20, 30152000, 100000},
		{"rate near one", 500, 0.999, 1, 64, 0},
		{"tiny rate", 50, 1e-9, 30, 2157, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bloomfilter.New(tt.n, tt.rate)
			require.Equal(t, tt.wantK, b.K())
			require.GreaterOrEqual(t, b.K(), 1)
			require.LessOrEqual(t, b.K(), 64)
			require.InDelta(t, tt.wantLen, b.BitLen(), float64(tt.delta))
			require.GreaterOrEqual(t, b.BitLen(), uint64(64), "at least one word of storage")
			require.Zero(t, b.Count())
			require.Zero(t, b.PopCount())
			require.Zero(t, b.FalsePositiveRate(), "an empty filter has no positives")
		})
	}
}

// TestNewPanics pins the exact panic messages of the rejected arguments.
func TestNewPanics(t *testing.T) {
	tests := []struct {
		name  string
		n     int
		rate  float64
		panic string
	}{
		{"zero items", 0, 0.01, "bloomfilter: expectedItems must be positive"},
		{"negative items", -1, 0.01, "bloomfilter: expectedItems must be positive"},
		{"min int items", math.MinInt, 0.5, "bloomfilter: expectedItems must be positive"},
		{"zero rate", 100, 0, "bloomfilter: falsePositiveRate must be within (0, 1)"},
		{"negative rate", 100, -0.5, "bloomfilter: falsePositiveRate must be within (0, 1)"},
		{"rate of one", 100, 1, "bloomfilter: falsePositiveRate must be within (0, 1)"},
		{"rate above one", 100, 1.5, "bloomfilter: falsePositiveRate must be within (0, 1)"},
		{"rate of +inf", 100, math.Inf(1), "bloomfilter: falsePositiveRate must be within (0, 1)"},
		{"rate of -inf", 100, math.Inf(-1), "bloomfilter: falsePositiveRate must be within (0, 1)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.PanicsWithValue(t, tt.panic, func() { bloomfilter.New(tt.n, tt.rate) })
		})
	}
}

// TestNaNRateBypassesValidation characterises a gap in the argument check: a
// NaN false-positive rate is not rejected by New because both comparisons in
// the check are false for NaN. The returned filter is unusable (its bit count
// degenerates to an implementation-dependent huge value), so this test only
// pins the missing validation and deliberately never touches the filter; the
// defect is reported separately and the source is not modified here.
func TestNaNRateBypassesValidation(t *testing.T) {
	require.NotPanics(t, func() { bloomfilter.New(100, math.NaN()) },
		"a NaN false-positive rate must be rejected but currently is not")
}

// TestExtremeSizingIsAcceptedButUnsafe pins that New accepts extreme design
// sizes without panicking: m is computed in float64 and converted to uint64
// without an overflow check, so huge expectedItems yield an unusable filter
// (probing would request astronomically large bit indexes). Only construction
// is exercised here; the missing bound is reported separately.
func TestExtremeSizingIsAcceptedButUnsafe(t *testing.T) {
	tests := []struct {
		name string
		n    int
		rate float64
	}{
		{"max int items", math.MaxInt, 1e-9},
		{"max int items loose rate", math.MaxInt, 0.5},
		{"billion items", 1 << 30, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotPanics(t, func() {
				b := bloomfilter.New(tt.n, tt.rate)
				// construction must leave the filter empty and consistent
				require.Zero(t, b.Count())
				require.Zero(t, b.PopCount())
				require.Zero(t, b.FalsePositiveRate())
			})
		})
	}
}

// TestAddThenTest covers the normal round trip of every element kind.
func TestAddThenTest(t *testing.T) {
	long := strings.Repeat("x", 256)
	tests := []struct {
		name  string
		add   func(b *bloomfilter.BloomFilter)
		probe func(b *bloomfilter.BloomFilter) bool
	}{
		{
			name:  "string hit",
			add:   func(b *bloomfilter.BloomFilter) { b.AddString("alpha") },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestString("alpha") },
		},
		{
			name:  "string hit repeated add",
			add:   func(b *bloomfilter.BloomFilter) { b.AddString("alpha"); b.AddString("alpha") },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestString("alpha") },
		},
		{
			name:  "empty string hit",
			add:   func(b *bloomfilter.BloomFilter) { b.AddString("") },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestString("") },
		},
		{
			name:  "unicode string hit",
			add:   func(b *bloomfilter.BloomFilter) { b.AddString("玩家-42-🎮") },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestString("玩家-42-🎮") },
		},
		{
			name:  "long string hit",
			add:   func(b *bloomfilter.BloomFilter) { b.AddString(long) },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestString(long) },
		},
		{
			name:  "string with NUL byte hit",
			add:   func(b *bloomfilter.BloomFilter) { b.AddString("a\x00b") },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestString("a\x00b") },
		},
		{
			name:  "bytes hit",
			add:   func(b *bloomfilter.BloomFilter) { b.AddBytes([]byte("network-session-token")) },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestBytes([]byte("network-session-token")) },
		},
		{
			name:  "empty bytes hit",
			add:   func(b *bloomfilter.BloomFilter) { b.AddBytes([]byte{}) },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestBytes([]byte{}) },
		},
		{
			name:  "nil bytes hit",
			add:   func(b *bloomfilter.BloomFilter) { b.AddBytes(nil) },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestBytes(nil) },
		},
		{
			name: "bytes hashed at add time",
			add: func(b *bloomfilter.BloomFilter) {
				p := []byte("mutable")
				b.AddBytes(p)
				p[0] = 'M' // mutating after Add must not change membership
			},
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestBytes([]byte("mutable")) },
		},
		{
			name:  "uint64 zero hit",
			add:   func(b *bloomfilter.BloomFilter) { b.AddUint64(0) },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestUint64(0) },
		},
		{
			name:  "uint64 max hit",
			add:   func(b *bloomfilter.BloomFilter) { b.AddUint64(math.MaxUint64) },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestUint64(math.MaxUint64) },
		},
		{
			name:  "uint64 mid hit",
			add:   func(b *bloomfilter.BloomFilter) { b.AddUint64(1 << 63) },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestUint64(1 << 63) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bloomfilter.New(1000, 0.001)
			tt.add(b)
			require.True(t, tt.probe(b), "an added element must be reported present")
			require.GreaterOrEqual(t, b.PopCount(), 1, "adding must set bits")
			require.LessOrEqual(t, b.PopCount(), int(b.BitLen()))
		})
	}
}

// TestMisses covers unadded elements: distinct payloads and the three kinds
// are separate elements even when their payload text is identical.
func TestMisses(t *testing.T) {
	tests := []struct {
		name  string
		add   func(b *bloomfilter.BloomFilter)
		probe func(b *bloomfilter.BloomFilter) bool
	}{
		{
			name:  "string miss",
			add:   func(b *bloomfilter.BloomFilter) { b.AddString("alpha") },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestString("beta") },
		},
		{
			name:  "string prefix miss",
			add:   func(b *bloomfilter.BloomFilter) { b.AddString("alphabet") },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestString("alpha") },
		},
		{
			name:  "string suffix miss",
			add:   func(b *bloomfilter.BloomFilter) { b.AddString("alpha") },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestString("alphabet") },
		},
		{
			name:  "case sensitive miss",
			add:   func(b *bloomfilter.BloomFilter) { b.AddString("Alpha") },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestString("alpha") },
		},
		{
			name:  "bytes miss",
			add:   func(b *bloomfilter.BloomFilter) { b.AddBytes([]byte("session-token-a")) },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestBytes([]byte("session-token-b")) },
		},
		{
			name:  "uint64 adjacent miss",
			add:   func(b *bloomfilter.BloomFilter) { b.AddUint64(42) },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestUint64(43) },
		},
		{
			name:  "string added is not seen as bytes",
			add:   func(b *bloomfilter.BloomFilter) { b.AddString("42") },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestBytes([]byte("42")) },
		},
		{
			name:  "string added is not seen as uint64",
			add:   func(b *bloomfilter.BloomFilter) { b.AddString("42") },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestUint64(42) },
		},
		{
			name:  "bytes added is not seen as string",
			add:   func(b *bloomfilter.BloomFilter) { b.AddBytes([]byte("42")) },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestString("42") },
		},
		{
			name:  "uint64 added is not seen as string",
			add:   func(b *bloomfilter.BloomFilter) { b.AddUint64(42) },
			probe: func(b *bloomfilter.BloomFilter) bool { return b.TestString("42") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bloomfilter.New(1<<20, 1e-9) // false positives practically impossible
			tt.add(b)
			require.False(t, tt.probe(b), "an unadded element must not collide here")
		})
	}
}

// TestCountAndClear covers the Add-call bookkeeping and the reset.
func TestCountAndClear(t *testing.T) {
	tests := []struct {
		name       string
		script     string // comma separated: add:<key>, addbytes:<key>, addu:<n>, clear
		wantCount  uint64
		wantAlive  []string // strings added with AddString, probed via TestString
		aliveBytes []string // payloads added with AddBytes, probed via TestBytes
		aliveU64   []uint64 // values added with AddUint64, probed via TestUint64
	}{
		{
			name:      "single add",
			script:    "add:a",
			wantCount: 1,
			wantAlive: []string{"a"},
		},
		{
			name:      "double add counts twice",
			script:    "add:a,add:a",
			wantCount: 2,
			wantAlive: []string{"a"},
		},
		{
			name:      "three distinct",
			script:    "add:a,add:b,add:c",
			wantCount: 3,
			wantAlive: []string{"a", "b", "c"},
		},
		{
			name:      "clear resets count",
			script:    "add:a,add:b,clear",
			wantCount: 0,
			wantAlive: nil,
		},
		{
			name:      "clear then add restarts count",
			script:    "add:a,clear,add:b",
			wantCount: 1,
			wantAlive: []string{"b"},
		},
		{
			name:      "double clear is a no-op",
			script:    "add:a,clear,clear",
			wantCount: 0,
			wantAlive: nil,
		},
		{
			name:       "mixed kinds count together",
			script:     "add:a,addbytes:b,addu:7",
			wantCount:  3,
			wantAlive:  []string{"a"},
			aliveBytes: []string{"b"},
			aliveU64:   []uint64{7},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bloomfilter.New(1000, 0.001)
			for _, step := range strings.Split(tt.script, ",") {
				switch {
				case step == "clear":
					b.Clear()
				case strings.HasPrefix(step, "add:"):
					b.AddString(strings.TrimPrefix(step, "add:"))
				case strings.HasPrefix(step, "addbytes:"):
					b.AddBytes([]byte(strings.TrimPrefix(step, "addbytes:")))
				case strings.HasPrefix(step, "addu:"):
					v, err := strconv.ParseUint(strings.TrimPrefix(step, "addu:"), 10, 64)
					require.NoError(t, err)
					b.AddUint64(v)
				default:
					t.Fatalf("unknown script step %q", step)
				}
			}
			require.Equal(t, tt.wantCount, b.Count())
			for _, k := range tt.wantAlive {
				require.True(t, b.TestString(k), "key %q must survive the script", k)
			}
			for _, k := range tt.aliveBytes {
				require.True(t, b.TestBytes([]byte(k)), "bytes %q must survive the script", k)
			}
			for _, v := range tt.aliveU64 {
				require.True(t, b.TestUint64(v), "uint64 %d must survive the script", v)
			}
			if tt.wantCount == 0 {
				require.Zero(t, b.PopCount(), "a cleared filter must hold no bits")
				require.Zero(t, b.FalsePositiveRate())
			}
		})
	}
}

// TestFalsePositiveRateFormula pins the documented estimate against the
// closed-form (1 - e^(-k*n/m))^k.
func TestFalsePositiveRateFormula(t *testing.T) {
	tests := []struct {
		name string
		n    int
		rate float64
		adds int
	}{
		{"empty is zero", 1000, 0.01, 0},
		{"quarter filled", 1000, 0.01, 250},
		{"design size", 1000, 0.01, 1000},
		{"overfilled", 1000, 0.01, 4000},
		{"loose design", 500, 0.1, 500},
		{"tight design", 500, 0.001, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bloomfilter.New(tt.n, tt.rate)
			for i := range tt.adds {
				b.AddUint64(uint64(i))
			}
			var want float64
			if tt.adds > 0 {
				k, m := float64(b.K()), float64(b.BitLen())
				want = math.Pow(1-math.Exp(-k*float64(tt.adds)/m), k)
			}
			require.InDelta(t, want, b.FalsePositiveRate(), 1e-12)
			require.GreaterOrEqual(t, b.FalsePositiveRate(), 0.0)
			require.LessOrEqual(t, b.FalsePositiveRate(), 1.0)
		})
	}
}

// TestPopCountBounds pins the occupancy reporting.
func TestPopCountBounds(t *testing.T) {
	tests := []struct {
		name    string
		n       int
		rate    float64
		adds    int
		wantMin int
		wantMax int
	}{
		{"nothing added", 100, 0.01, 0, 0, 0},
		{"one add sets at most k bits", 100, 0.01, 1, 1, 7},
		{"few adds", 100, 0.01, 10, 5, 70},
		{"many adds bounded by bitlen", 100, 0.01, 100, 50, 960},
		{"tiny filter", 10, 0.5, 10, 5, 64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bloomfilter.New(tt.n, tt.rate)
			for i := range tt.adds {
				b.AddUint64(uint64(i))
			}
			require.GreaterOrEqual(t, b.PopCount(), tt.wantMin)
			require.LessOrEqual(t, b.PopCount(), tt.wantMax)
			require.LessOrEqual(t, b.PopCount(), int(b.BitLen()))
		})
	}
}

// TestClearBehaviour covers Clear on empty, populated and reused filters.
func TestClearBehaviour(t *testing.T) {
	tests := []struct {
		name   string
		before func(b *bloomfilter.BloomFilter)
	}{
		{"clear an empty filter", func(b *bloomfilter.BloomFilter) {}},
		{"clear after one add", func(b *bloomfilter.BloomFilter) { b.AddString("a") }},
		{"clear after many adds", func(b *bloomfilter.BloomFilter) {
			for i := range 50 {
				b.AddUint64(uint64(i))
			}
		}},
		{"clear after clear", func(b *bloomfilter.BloomFilter) {
			b.AddString("a")
			b.Clear()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bloomfilter.New(100, 0.01)
			tt.before(b)
			b.Clear()
			require.Zero(t, b.Count())
			require.Zero(t, b.PopCount())
			require.Zero(t, b.FalsePositiveRate())
			require.False(t, b.TestString("a"), "a cleared filter forgets everything")
			require.False(t, b.TestUint64(0))

			// the filter stays usable after Clear
			b.AddString("a")
			require.True(t, b.TestString("a"))
			require.Equal(t, uint64(1), b.Count())
		})
	}
}

// TestAccessorsConsistency checks the read-only accessors against each other.
func TestAccessorsConsistency(t *testing.T) {
	tests := []struct {
		name string
		n    int
		rate float64
		adds int
	}{
		{"small", 100, 0.01, 20},
		{"medium", 1000, 0.05, 250},
		{"large", 10_000, 0.001, 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bloomfilter.New(tt.n, tt.rate)
			for i := range tt.adds {
				b.AddString(fmt.Sprintf("k-%d", i))
			}
			require.Equal(t, uint64(tt.adds), b.Count())
			require.GreaterOrEqual(t, b.BitLen(), uint64(64))
			require.GreaterOrEqual(t, b.K(), 1)
			require.LessOrEqual(t, b.K(), 64)
			require.GreaterOrEqual(t, b.PopCount(), 1)
			require.LessOrEqual(t, b.PopCount(), int(b.BitLen()))
			require.Positive(t, b.FalsePositiveRate())
		})
	}
}

// TestAgainstSetModelRandomOps replays randomised Add/Test/Clear traffic
// against the map[string]bool model and requires zero false negatives.
func TestAgainstSetModelRandomOps(t *testing.T) {
	seeds := []uint64{1, 7, 42, 1337, 20261012}
	for _, seed := range seeds {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			r := rand.New(rand.NewPCG(seed, 0x233))
			ops := make([]bfOp, 0, 256)
			for range 256 {
				ops = append(ops, bfOp{kind: byte(r.IntN(5)), idx: r.IntN(bfUniverse)})
			}
			runBFModel(t, ops)
		})
	}
}

// TestFalsePositiveStatistics builds the statistical model: the filter must
// contain every key of the added map and its observed false-positive rate over
// unseen keys must stay within the design budget (generous margin: the FNV +
// double-hashing pair is not a perfect hash).
func TestFalsePositiveStatistics(t *testing.T) {
	const (
		n      = 20_000
		rate   = 0.01
		probes = 20_000
	)
	b := bloomfilter.New(n, rate)
	added := make(map[string]bool, n)
	for i := range n {
		k := fmt.Sprintf("uid:%d", i)
		b.AddString(k)
		added[k] = true
	}
	require.Equal(t, uint64(n), b.Count())

	// no false negatives against the model set
	for k := range added {
		require.True(t, b.TestString(k), "added key %q must never be reported absent", k)
	}

	falsePositives := 0
	for i := range probes {
		k := fmt.Sprintf("absent:%d", i)
		require.False(t, added[k], "probe key must not be in the model set")
		if b.TestString(k) {
			falsePositives++
		}
	}
	observed := float64(falsePositives) / float64(probes)
	require.Less(t, observed, rate*5,
		"observed false-positive rate %.4f is far above the design rate %.4f", observed, rate)
	require.InDelta(t, rate, b.FalsePositiveRate(), rate*0.5,
		"the estimate at the design size must stay near the design rate")
}

// TestReadPathAllocations guards the hot read paths (the TestXxx probes) and
// the Add path on a warm filter against unexpected allocations. The package
// exposes Test* rather than Has/MightContain; these are the read paths.
func TestReadPathAllocations(t *testing.T) {
	t.Run("TestString hit", func(t *testing.T) {
		b := bloomfilter.New(1024, 0.01)
		b.AddString("hot-path")
		allocs := testing.AllocsPerRun(1000, func() {
			_ = b.TestString("hot-path")
		})
		require.Zero(t, allocs)
	})

	t.Run("TestUint64 hit", func(t *testing.T) {
		b := bloomfilter.New(1024, 0.01)
		b.AddUint64(42)
		allocs := testing.AllocsPerRun(1000, func() {
			_ = b.TestUint64(42)
		})
		require.Zero(t, allocs)
	})

	t.Run("TestBytes hit and miss", func(t *testing.T) {
		b := bloomfilter.New(1024, 0.01)
		buf := []byte("hot-path-bytes")
		b.AddBytes(buf)
		allocs := testing.AllocsPerRun(1000, func() {
			_ = b.TestBytes(buf)
			_ = b.TestBytes(buf)
		})
		require.Zero(t, allocs)
	})

	t.Run("AddUint64 on a warm filter", func(t *testing.T) {
		b := bloomfilter.New(1024, 0.01)
		b.AddUint64(42) // warm the storage up to the probed bits
		allocs := testing.AllocsPerRun(1000, func() {
			b.AddUint64(42)
		})
		require.Zero(t, allocs, "re-adding within the grown bit vector must not allocate")
	})
}

// TestConcurrentUseSkipped documents the concurrency contract: BloomFilter is
// explicitly not safe for concurrent use, so no race test is run here.
func TestConcurrentUseSkipped(t *testing.T) {
	t.Skip("bloomfilter.BloomFilter is documented as not safe for concurrent use; " +
		"no concurrent test applies (would fail under -race by design)")
}
