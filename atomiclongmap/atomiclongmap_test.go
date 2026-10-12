package atomiclongmap_test

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/atomiclongmap"
)

// referenceModel is the naive map[K]int64 + mutex model used for differential
// testing against AtomicLongMap.
type referenceModel struct {
	mu sync.Mutex
	m  map[string]int64
}

func newReferenceModel() *referenceModel {
	return &referenceModel{m: make(map[string]int64)}
}

func (r *referenceModel) Get(key string) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.m[key]
}

func (r *referenceModel) Add(key string, delta int64) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[key] += delta
	return r.m[key]
}

func (r *referenceModel) Put(key string, value int64) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.m[key]
	r.m[key] = value
	return old
}

func (r *referenceModel) Remove(key string) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.m[key]
	delete(r.m, key)
	return old
}

func (r *referenceModel) RemoveIfZero(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v, ok := r.m[key]; ok && v == 0 {
		delete(r.m, key)
		return true
	}
	return false
}

func (r *referenceModel) Snapshot() map[string]int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]int64, len(r.m))
	for k, v := range r.m {
		out[k] = v
	}
	return out
}

func TestNewIsEmpty(t *testing.T) {
	m := atomiclongmap.New[string]()
	require.Equal(t, 0, m.Len())
	require.Equal(t, int64(0), m.Sum())
	require.Equal(t, int64(0), m.Get("missing"))
	require.Empty(t, m.Keys())
	require.Empty(t, m.AsMap())
}

func TestGetTableDriven(t *testing.T) {
	m := atomiclongmap.New[string]()
	m.Put("a", 5)
	m.Put("b", -7)
	m.Put("z", 0)

	tests := []struct {
		name string
		key  string
		want int64
	}{
		{"existing positive", "a", 5},
		{"existing negative", "b", -7},
		{"existing zero", "z", 0},
		{"missing key", "nope", 0},
		{"empty key on non-empty map", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, m.Get(tt.key))
		})
	}
}

func TestAddTableDriven(t *testing.T) {
	tests := []struct {
		name      string
		steps     []int64 // deltas applied in order
		wantFinal int64
	}{
		{"single positive add", []int64{3}, 3},
		{"single negative add", []int64{-4}, -4},
		{"zero delta creates entry", []int64{0}, 0},
		{"two adds accumulate", []int64{2, 3}, 5},
		{"add and subtract", []int64{10, -10}, 0},
		{"many small adds", []int64{1, 1, 1, 1, 1}, 5},
		{"overflow wraps like int64", []int64{math.MaxInt64, 1}, math.MinInt64},
		{"underflow wraps like int64", []int64{math.MinInt64, -1}, math.MaxInt64},
		{"large delta", []int64{1 << 40, 1 << 40}, 1 << 41},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := atomiclongmap.New[string]()
			var got int64
			for _, d := range tt.steps {
				got = m.Add("k", d)
			}
			require.Equal(t, tt.wantFinal, got, "Add must return the new value")
			require.Equal(t, tt.wantFinal, m.Get("k"))
			require.Equal(t, 1, m.Len(), "all deltas hit the same key")
		})
	}
}

func TestIncrementDecrementReturnNewValue(t *testing.T) {
	tests := []struct {
		name       string
		ops        []string // "inc" or "dec"
		wantFinal  int64
		wantValues []int64
	}{
		{"single increment", []string{"inc"}, 1, []int64{1}},
		{"single decrement below zero", []string{"dec"}, -1, []int64{-1}},
		{"inc inc dec", []string{"inc", "inc", "dec"}, 1, []int64{1, 2, 1}},
		{"dec to exact zero stays", []string{"inc", "dec"}, 0, []int64{1, 0}},
		{"inc to overflow wraps", []string{"inc"}, 1, []int64{1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := atomiclongmap.New[string]()
			var values []int64
			for _, op := range tt.ops {
				if op == "inc" {
					values = append(values, m.Increment("k"))
				} else {
					values = append(values, m.Decrement("k"))
				}
			}
			require.Equal(t, tt.wantValues, values, "each call must return the new value")
			require.Equal(t, tt.wantFinal, m.Get("k"))
		})
	}

	t.Run("increment from overflow boundary", func(t *testing.T) {
		m := atomiclongmap.New[string]()
		m.Put("k", math.MaxInt64)
		require.Equal(t, int64(math.MinInt64), m.Increment("k"))
	})
}

func TestPutTableDriven(t *testing.T) {
	tests := []struct {
		name     string
		steps    []int64
		wantOld  []int64 // previous value returned by each Put
		wantLast int64
	}{
		{"put on missing key", []int64{5}, []int64{0}, 5},
		{"put overwrites previous", []int64{5, 9}, []int64{0, 5}, 9},
		{"put zero then zero", []int64{0, 0}, []int64{0, 0}, 0},
		{"put negative", []int64{-3}, []int64{0}, -3},
		{"put negative then positive", []int64{-3, 7}, []int64{0, -3}, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := atomiclongmap.New[string]()
			var olds []int64
			for _, v := range tt.steps {
				olds = append(olds, m.Put("k", v))
			}
			require.Equal(t, tt.wantOld, olds, "Put must return the previous value")
			require.Equal(t, tt.wantLast, m.Get("k"))
			require.Equal(t, 1, m.Len())
		})
	}
}

func TestRemoveTableDriven(t *testing.T) {
	tests := []struct {
		name      string
		initial   int64
		hasKey    bool
		wantOld   int64
		wantAfter int64
		wantLen   int
	}{
		{"remove positive value", 5, true, 5, 0, 0},
		{"remove negative value", -8, true, -8, 0, 0},
		{"remove zero value", 0, true, 0, 0, 0},
		{"remove missing key", 0, false, 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := atomiclongmap.New[string]()
			if tt.hasKey {
				m.Put("k", tt.initial)
			}
			require.Equal(t, tt.wantOld, m.Remove("k"), "Remove must return the previous value")
			require.Equal(t, tt.wantAfter, m.Get("k"))
			require.Equal(t, tt.wantLen, m.Len())
		})
	}
}

func TestRemoveIfZeroBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		initial   int64
		hasKey    bool
		wantGone  bool
		wantAfter int64
		wantLen   int
	}{
		{"exactly zero is removed", 0, true, true, 0, 0},
		{"zero reached by add -x +x", 0, true, true, 0, 0},
		{"positive value is kept", 1, true, false, 1, 1},
		{"negative value is kept", -1, true, false, -1, 1},
		{"missing key is a no-op", 0, false, false, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := atomiclongmap.New[string]()
			if tt.hasKey {
				m.Add("k", tt.initial)
			}
			require.Equal(t, tt.wantGone, m.RemoveIfZero("k"))
			require.Equal(t, tt.wantAfter, m.Get("k"))
			require.Equal(t, tt.wantLen, m.Len())
		})
	}

	t.Run("removed zero entry reappears only on new write", func(t *testing.T) {
		m := atomiclongmap.New[string]()
		m.Add("k", 3)
		m.Add("k", -3)
		require.True(t, m.RemoveIfZero("k"))
		require.Equal(t, 0, m.Len())
		require.Equal(t, int64(1), m.Increment("k"), "a later Add must create the entry again")
		require.Equal(t, 1, m.Len())
	})

	t.Run("exactly zero after decrement", func(t *testing.T) {
		m := atomiclongmap.New[string]()
		m.Increment("k")
		require.Equal(t, int64(0), m.Decrement("k"))
		require.True(t, m.RemoveIfZero("k"))
		require.Equal(t, 0, m.Len())
	})
}

func TestSumTableDriven(t *testing.T) {
	tests := []struct {
		name  string
		pairs map[string]int64
		want  int64
	}{
		{"empty map", map[string]int64{}, 0},
		{"single positive", map[string]int64{"a": 4}, 4},
		{"single negative", map[string]int64{"a": -4}, -4},
		{"mixed signs cancel", map[string]int64{"a": 10, "b": -10}, 0},
		{"three counters", map[string]int64{"a": 1, "b": 2, "c": 3}, 6},
		{"zero value counted", map[string]int64{"a": 0, "b": 5}, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := atomiclongmap.New[string]()
			for k, v := range tt.pairs {
				m.Put(k, v)
			}
			require.Equal(t, tt.want, m.Sum())
		})
	}
}

func TestLenCountsZeroEntriesUntilRemoved(t *testing.T) {
	m := atomiclongmap.New[string]()
	m.Put("zero", 0)
	m.Put("one", 1)
	require.Equal(t, 2, m.Len(), "zero-valued entries still occupy the map")

	m.Remove("one")
	require.Equal(t, 1, m.Len())

	m.RemoveIfZero("zero")
	require.Equal(t, 0, m.Len())
}

func TestKeysSnapshot(t *testing.T) {
	m := atomiclongmap.New[string]()
	m.Put("a", 1)
	m.Put("b", 2)
	m.Put("c", 3)

	keys := m.Keys()
	require.Len(t, keys, 3)
	require.ElementsMatch(t, []string{"a", "b", "c"}, keys)

	keys[0] = "mutated"
	require.ElementsMatch(t, []string{"a", "b", "c"}, m.Keys(), "Keys must return an independent snapshot")
}

func TestAsMapSnapshotIsIndependent(t *testing.T) {
	m := atomiclongmap.New[string]()
	m.Put("a", 1)
	m.Put("b", -2)
	m.Put("z", 0)

	snap := m.AsMap()
	require.Equal(t, map[string]int64{"a": 1, "b": -2, "z": 0}, snap)

	// Later mutations must not leak into the snapshot, and vice versa.
	m.Add("a", 10)
	m.Put("new", 5)
	delete(snap, "b")
	snap["forged"] = 99

	require.Equal(t, map[string]int64{"a": 11, "b": -2, "z": 0, "new": 5}, m.AsMap())
}

func TestRangeTableDriven(t *testing.T) {
	tests := []struct {
		name      string
		pairs     map[string]int64
		stopAfter int // 0 means iterate everything
		wantSeen  map[string]int64
	}{
		{"empty map", map[string]int64{}, 0, map[string]int64{}},
		{"single pair", map[string]int64{"a": 7}, 0, map[string]int64{"a": 7}},
		{"all pairs visited", map[string]int64{"a": 1, "b": -2, "c": 0}, 0, map[string]int64{"a": 1, "b": -2, "c": 0}},
		{"early stop after one", map[string]int64{"a": 1, "b": 2, "c": 3}, 1, map[string]int64{"a": 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := atomiclongmap.New[string]()
			for k, v := range tt.pairs {
				m.Put(k, v)
			}
			seen := make(map[string]int64)
			count := 0
			m.Range(func(key string, value int64) bool {
				seen[key] = value
				count++
				return tt.stopAfter == 0 || count < tt.stopAfter
			})
			require.Equal(t, tt.wantSeen, seen)
		})
	}

	t.Run("Range may mutate the map", func(t *testing.T) {
		m := atomiclongmap.New[string]()
		m.Put("a", 1)
		m.Put("b", 2)
		var visited []string
		m.Range(func(key string, value int64) bool {
			visited = append(visited, key)
			m.Remove("b")
			return true
		})
		require.ElementsMatch(t, []string{"a", "b"}, visited, "Range iterates a snapshot")
		require.Equal(t, 1, m.Len())
	})
}

func TestRangeNilFuncPanics(t *testing.T) {
	m := atomiclongmap.New[string]()
	require.PanicsWithValue(t, "atomiclongmap: Range func must not be nil", func() {
		m.Range(nil)
	})
}

func TestClear(t *testing.T) {
	m := atomiclongmap.New[string]()
	m.Put("a", 1)
	m.Put("b", 0)
	m.Put("c", -3)
	m.Clear()

	require.Equal(t, 0, m.Len())
	require.Equal(t, int64(0), m.Sum())
	require.Empty(t, m.AsMap())

	m.Increment("a")
	require.Equal(t, 1, m.Len(), "the map must stay usable after Clear")
}

func TestGenericIntKeys(t *testing.T) {
	m := atomiclongmap.New[int]()
	m.Add(42, 2)
	m.Increment(7)
	require.Equal(t, int64(2), m.Get(42))
	require.Equal(t, int64(1), m.Get(7))
	require.Equal(t, int64(3), m.Sum())
	require.ElementsMatch(t, []int{42, 7}, m.Keys())
}

func TestDifferentialAgainstReferenceModel(t *testing.T) {
	const rounds = 2000
	m := atomiclongmap.New[string]()
	ref := newReferenceModel()
	keys := []string{"a", "b", "c", "d", ""}

	for i := 0; i < rounds; i++ {
		key := keys[i%len(keys)]
		switch i % 8 {
		case 0, 1, 2:
			delta := int64(i%17) - 8
			wantNew := ref.Add(key, delta)
			require.Equal(t, wantNew, m.Add(key, delta), "round %d Add", i)
		case 3:
			require.Equal(t, ref.IncrementAndGet(key), m.Increment(key), "round %d Increment", i)
		case 4:
			wantNew := ref.Add(key, -1)
			require.Equal(t, wantNew, m.Decrement(key), "round %d Decrement", i)
		case 5:
			value := int64(i%5) - 2
			require.Equal(t, ref.Put(key, value), m.Put(key, value), "round %d Put", i)
		case 6:
			require.Equal(t, ref.RemoveIfZero(key), m.RemoveIfZero(key), "round %d RemoveIfZero", i)
		case 7:
			if i%64 == 7 {
				require.Equal(t, ref.Remove(key), m.Remove(key), "round %d Remove", i)
			}
		}
		require.Equal(t, ref.Get(key), m.Get(key), "round %d Get", i)
	}

	require.Equal(t, ref.Snapshot(), m.AsMap(), "final snapshots must match")

	var sum int64
	for _, v := range ref.Snapshot() {
		sum += v
	}
	require.Equal(t, sum, m.Sum())
	require.Equal(t, len(ref.Snapshot()), m.Len())
}

// helper on the reference model so the differential loop stays readable.
func (r *referenceModel) IncrementAndGet(key string) int64 {
	return r.Add(key, 1)
}

func TestConcurrentIncrementPreciseCount(t *testing.T) {
	const goroutines = 32
	const perGoroutine = 1000

	m := atomiclongmap.New[string]()
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				m.Increment("counter")
			}
		}()
	}
	wg.Wait()

	want := int64(goroutines * perGoroutine)
	require.Equal(t, want, m.Get("counter"), "no increment may be lost")
	require.Equal(t, want, m.Sum())
	require.Equal(t, 1, m.Len())
}

func TestConcurrentMixedOpsStayConsistent(t *testing.T) {
	const goroutines = 16
	const perGoroutine = 500

	m := atomiclongmap.New[string]()
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				switch (g + i) % 4 {
				case 0:
					m.Increment("shared")
				case 1:
					m.Decrement("shared")
				case 2:
					m.Get("shared")
				case 3:
					m.AsMap()
				}
			}
		}(g)
	}
	wg.Wait()

	// Every Increment is paired with a Decrement on another goroutine's mix,
	// but the mix is deterministic per (g, i): count exactly instead.
	incs := 0
	decs := 0
	for g := 0; g < goroutines; g++ {
		for i := 0; i < perGoroutine; i++ {
			switch (g + i) % 4 {
			case 0:
				incs++
			case 1:
				decs++
			}
		}
	}
	require.Equal(t, int64(incs-decs), m.Get("shared"))
	require.Equal(t, 1, m.Len())
}

func TestConcurrentGetOrAddNeverLosesUpdates(t *testing.T) {
	const goroutines = 32
	const perGoroutine = 200

	m := atomiclongmap.New[int]()
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				m.GetOrAdd(1).Add(1)
			}
		}()
	}
	wg.Wait()

	require.Equal(t, int64(goroutines*perGoroutine), m.Get(1), "GetOrAdd races must not lose updates")
	require.Equal(t, 1, m.Len())
}

func TestGetOrAddReturnsSharedCell(t *testing.T) {
	m := atomiclongmap.New[string]()
	c1 := m.GetOrAdd("k")
	c2 := m.GetOrAdd("k")
	require.Same(t, c1, c2, "existing keys must return the same cell")

	c1.Add(5)
	require.Equal(t, int64(5), m.Get("k"), "cell writes must be visible through the map")
}

func TestGetOrAddConcurrentSingleCell(t *testing.T) {
	const goroutines = 16
	m := atomiclongmap.New[string]()
	cells := make([]*atomic.Int64, goroutines)
	var wg sync.WaitGroup
	var start sync.WaitGroup
	start.Add(1)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			start.Wait()
			cells[g] = m.GetOrAdd("k")
		}(g)
	}
	start.Done()
	wg.Wait()

	for g := 1; g < goroutines; g++ {
		require.Same(t, cells[0], cells[g], "concurrent GetOrAdd must converge on one cell")
	}
	require.Equal(t, 1, m.Len())
}

func TestAddAllocatesZeroOnHotPath(t *testing.T) {
	m := atomiclongmap.New[string]()
	m.Add("k", 1) // pre-create the entry so Add takes the hot path

	allocs := testing.AllocsPerRun(1000, func() {
		m.Add("k", 1)
	})
	require.Zero(t, allocs, "Add on an existing key must allocate 0 times per run")

	allocs = testing.AllocsPerRun(1000, func() {
		m.Get("k")
	})
	require.Zero(t, allocs, "Get must allocate 0 times per run")

	allocs = testing.AllocsPerRun(1000, func() {
		m.Increment("k")
		m.Decrement("k")
	})
	require.Zero(t, allocs, "Increment/Decrement must allocate 0 times per run")
}

func FuzzAddPutRemoveRoundTrip(f *testing.F) {
	f.Add("k", int64(1), int64(2), uint8(0))
	f.Add("k", int64(-5), int64(0), uint8(3))
	f.Add("", int64(math.MaxInt64), int64(math.MinInt64), uint8(6))

	f.Fuzz(func(t *testing.T, key string, delta, value int64, op uint8) {
		m := atomiclongmap.New[string]()
		ref := newReferenceModel()

		switch op % 5 {
		case 0:
			require.Equal(t, ref.Add(key, delta), m.Add(key, delta))
		case 1:
			require.Equal(t, ref.Put(key, value), m.Put(key, value))
		case 2:
			require.Equal(t, ref.Add(key, delta), m.Add(key, delta))
			require.Equal(t, ref.RemoveIfZero(key), m.RemoveIfZero(key))
		case 3:
			require.Equal(t, ref.Remove(key), m.Remove(key))
			require.Equal(t, ref.Put(key, value), m.Put(key, value))
		case 4:
			require.Equal(t, ref.IncrementAndGet(key), m.Increment(key))
			require.Equal(t, ref.Add(key, -1), m.Decrement(key))
			require.Equal(t, ref.RemoveIfZero(key), m.RemoveIfZero(key))
		}

		require.Equal(t, ref.Snapshot(), m.AsMap())
		require.Equal(t, ref.Get(key), m.Get(key))
	})
}

func TestStringerFriendlyDebugging(t *testing.T) {
	m := atomiclongmap.New[string]()
	m.Put("hits", 3)
	m.Put("misses", -1)
	snap := m.AsMap()
	require.Equal(t, "map[hits:3 misses:-1]", fmt.Sprint(snap))
}
