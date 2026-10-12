package lru_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/lru"
)

// TestLRUOpsMatrix drives Put/Get/Peek/Contains/Remove/Keys/Len through
// scripted operation tables and checks the full observable state after every
// script. Script steps: "p<k>:<v>" Put, "g<k>" Get, "e<k>" Peek, "c<k>"
// Contains, "r<k>" Remove, "x" Clear. Expected eviction is folded into the
// wantEvicted slice (empty marker when nothing was evicted).
func TestLRUOpsMatrix(t *testing.T) {
	tests := []struct {
		name    string
		cap     int
		script  []string
		want    []int // key order LRU -> MRU afterwards
		evicted []int // evicted keys in order (0 marker: no eviction)
	}{
		{
			"fill exactly", 2,
			[]string{"p1:10", "p2:20"},
			[]int{1, 2}, nil,
		},
		{
			"first eviction", 2,
			[]string{"p1:10", "p2:20", "p3:30"},
			[]int{2, 3}, []int{1},
		},
		{
			"update does not evict", 2,
			[]string{"p1:10", "p2:20", "p1:11"},
			[]int{2, 1}, nil,
		},
		{
			"update then evict other", 2,
			[]string{"p1:10", "p2:20", "p1:11", "p3:30"},
			[]int{1, 3}, []int{2},
		},
		{
			"get promotes", 2,
			[]string{"p1:10", "p2:20", "g1", "p3:30"},
			[]int{1, 3}, []int{2},
		},
		{
			"get miss promotes nothing", 2,
			[]string{"p1:10", "p2:20", "g9", "p3:30"},
			[]int{2, 3}, []int{1},
		},
		{
			"peek does not promote", 2,
			[]string{"p1:10", "p2:20", "e1", "p3:30"},
			[]int{2, 3}, []int{1},
		},
		{
			"contains does not promote", 2,
			[]string{"p1:10", "p2:20", "c1", "p3:30"},
			[]int{2, 3}, []int{1},
		},
		{
			"contains miss", 2,
			[]string{"p1:10", "c9"},
			[]int{1}, nil,
		},
		{
			"remove frees a slot", 2,
			[]string{"p1:10", "p2:20", "r1", "p3:30"},
			[]int{2, 3}, nil,
		},
		{
			"remove missing is a no-op", 2,
			[]string{"p1:10", "r9"},
			[]int{1}, nil,
		},
		{
			"remove twice reports once", 1,
			[]string{"p1:10", "r1", "r1"},
			nil, nil,
		},
		{
			"capacity one churn", 1,
			[]string{"p1:10", "p2:20", "p3:30"},
			[]int{3}, []int{1, 2},
		},
		{
			"capacity one update", 1,
			[]string{"p1:10", "p1:11", "p1:12"},
			[]int{1}, nil,
		},
		{
			"clear empties", 3,
			[]string{"p1:10", "p2:20", "x"},
			nil, nil,
		},
		{
			"clear then reuse", 3,
			[]string{"p1:10", "x", "p2:20", "p3:30"},
			[]int{2, 3}, nil,
		},
		{
			"eviction cascades", 3,
			[]string{"p1:1", "p2:2", "p3:3", "p4:4", "p5:5"},
			[]int{3, 4, 5}, []int{1, 2},
		},
		{
			"get on most recent still fine", 3,
			[]string{"p1:1", "p2:2", "p3:3", "g3", "p4:4"},
			[]int{2, 3, 4}, []int{1},
		},
		{
			"promotion chain", 3,
			[]string{"p1:1", "p2:2", "p3:3", "g1", "g2", "p4:4"},
			[]int{1, 2, 4}, []int{3},
		},
		{
			"remove victim then insert", 2,
			[]string{"p1:1", "p2:2", "r2", "p3:3", "p4:4"},
			[]int{3, 4}, []int{1},
		},
		{
			"zero values", 2,
			[]string{"p1:0", "p2:0", "p3:0"},
			[]int{2, 3}, []int{1},
		},
		{
			"negative keys", 3,
			[]string{"p-1:1", "p-2:2", "p-3:3", "p-4:4"},
			[]int{-2, -3, -4}, []int{-1},
		},
		{
			"reinsert evicted", 2,
			[]string{"p1:1", "p2:2", "p3:3", "p1:9"},
			[]int{3, 1}, []int{1, 2},
		},
		{
			"update keeps value fresh", 2,
			[]string{"p1:1", "p1:2", "p1:3", "g1"},
			[]int{1}, nil,
		},
		{
			"peek miss no promotion", 2,
			[]string{"p1:1", "p2:2", "e9", "p3:3"},
			[]int{2, 3}, []int{1},
		},
		{
			"remove lru head", 3,
			[]string{"p1:1", "p2:2", "p3:3", "r1", "p4:4"},
			[]int{2, 3, 4}, nil,
		},
		{
			"remove mru tail", 3,
			[]string{"p1:1", "p2:2", "p3:3", "r3", "p4:4"},
			[]int{1, 2, 4}, nil,
		},
		{
			"update promotes like get", 2,
			[]string{"p1:1", "p2:2", "p1:9", "p3:3"},
			[]int{1, 3}, []int{2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := lru.New[int, int](tt.cap)
			var evicted []int
			for _, step := range tt.script {
				switch step[0] {
				case 'p':
					k, v := parseKV(step[1:])
					ek, _, ev := c.Put(k, v)
					if ev {
						evicted = append(evicted, ek)
					}
				case 'g':
					k := atoi(step[1:])
					_, ok := c.Get(k)
					_ = ok
				case 'e':
					k := atoi(step[1:])
					_, ok := c.Peek(k)
					_ = ok
				case 'c':
					k := atoi(step[1:])
					_ = c.Contains(k)
				case 'r':
					k := atoi(step[1:])
					c.Remove(k)
				case 'x':
					c.Clear()
				}
			}
			require.Equal(t, tt.evicted, orNil(evicted), "eviction report sequence")
			require.Equal(t, tt.want, orNil(c.Keys()), "key order LRU -> MRU")
			require.Equal(t, len(tt.want), c.Len())

			// every key in want must be contained, and the cache must not hold
			// anything else
			for _, k := range tt.want {
				require.True(t, c.Contains(k), "Contains(%d)", k)
			}
		})
	}
}

// orNil normalizes an empty slice to nil so require.Equal can compare it
// against the table's nil want values.
func orNil(s []int) []int {
	if len(s) == 0 {
		return nil
	}
	return s
}

// parseKV parses "k:v" into ints.
func parseKV(s string) (int, int) {
	for i := range s {
		if s[i] == ':' {
			return atoi(s[:i]), atoi(s[i+1:])
		}
	}
	return atoi(s), 0
}

// atoi parses a (possibly negative) decimal literal.
func atoi(s string) int {
	neg := false
	if s == "" {
		return 0
	}
	if s[0] == '-' {
		neg, s = true, s[1:]
	}
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	if neg {
		return -n
	}
	return n
}

// TestLRUCapacityAndConstructor pins the constructor and accessor contracts,
// including the panic on a non-positive capacity.
func TestLRUCapacityAndConstructor(t *testing.T) {
	tests := []struct {
		name string
		cap  int
	}{
		{"one", 1},
		{"two", 2},
		{"eight", 8},
		{"large", 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := lru.New[int, string](tt.cap)
			require.Equal(t, tt.cap, c.Capacity())
			require.Zero(t, c.Len())
			require.Empty(t, c.Keys())

			_, ok := c.Get(0)
			require.False(t, ok, "Get on empty cache must miss")
			_, ok = c.Peek(0)
			require.False(t, ok, "Peek on empty cache must miss")
			require.False(t, c.Contains(0))
			require.False(t, c.Remove(0), "Remove on empty cache must miss")
		})
	}
}

func TestNewPanicsOnBadCapacity(t *testing.T) {
	tests := []struct {
		name string
		cap  int
		msg  string
	}{
		{"zero", 0, "lru: capacity must be positive"},
		{"negative", -1, "lru: capacity must be positive"},
		{"large negative", -100, "lru: capacity must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.PanicsWithValue(t, tt.msg, func() { lru.New[string, int](tt.cap) })
		})
	}
}

// TestLRUAllIteration checks the iteration order and early break of All.
func TestLRUAllIteration(t *testing.T) {
	c := lru.New[int, int](4)
	for i := range 4 {
		c.Put(i, i*10)
	}
	c.Get(1)
	c.Get(0)

	type kv struct {
		k, v int
	}
	var got []kv
	for k, v := range c.All() {
		got = append(got, kv{k, v})
	}
	require.Equal(t, []kv{{2, 20}, {3, 30}, {1, 10}, {0, 0}}, got,
		"All must walk LRU -> MRU")

	count := 0
	for range c.All() {
		count++
		break
	}
	require.Equal(t, 1, count, "break must stop the iteration")
}

// TestLRUAllEarlyStopViaValue requires the value side of All to carry the
// stored value, and a false yield to stop the walk.
func TestLRUAllEarlyStopViaValue(t *testing.T) {
	c := lru.New[string, int](3)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)

	seen := 0
	for k, v := range c.All() {
		seen++
		require.Equal(t, v, map[string]int{"a": 1, "b": 2, "c": 3}[k])
		if seen == 2 {
			break
		}
	}
	require.Equal(t, 2, seen)
}

// TestLRUGenericValueTypes exercises non-int values.
func TestLRUGenericValueTypes(t *testing.T) {
	c := lru.New[string, []int](2)
	c.Put("a", []int{1, 2})
	c.Put("b", []int{3})

	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, []int{1, 2}, v)

	c.Put("c", nil)
	_, ok = c.Get("b")
	require.False(t, ok, "b must be the eviction victim")

	v, ok = c.Get("c")
	require.True(t, ok)
	require.Nil(t, v, "a nil value must round-trip")
}

// ---- differential test against a map + recency-slice model -----------------

// lruModel is the reference model: a map for values and an ordered slice of
// keys from LRU to MRU.
type lruModel struct {
	cap int
	m   map[int]int
	ord []int // keys, LRU first
}

// newLRUModel returns an empty model with the given capacity.
func newLRUModel(capacity int) *lruModel {
	return &lruModel{cap: capacity, m: map[int]int{}}
}

// put mirrors Put and reports the evicted key/value.
func (m *lruModel) put(k, v int) (int, int, bool) {
	if _, ok := m.m[k]; ok {
		m.m[k] = v
		m.promote(k)
		return 0, 0, false
	}
	var ek, ev int
	var did bool
	if len(m.m) >= m.cap {
		victim := m.ord[0]
		ek, ev, did = victim, m.m[victim], true
		delete(m.m, victim)
		m.ord = m.ord[1:]
	}
	m.m[k] = v
	m.ord = append(m.ord, k)
	return ek, ev, did
}

// get mirrors Get: promotes on hit.
func (m *lruModel) get(k int) (int, bool) {
	v, ok := m.m[k]
	if ok {
		m.promote(k)
	}
	return v, ok
}

// peek mirrors Peek: no promotion.
func (m *lruModel) peek(k int) (int, bool) {
	v, ok := m.m[k]
	return v, ok
}

// remove mirrors Remove.
func (m *lruModel) remove(k int) bool {
	if _, ok := m.m[k]; !ok {
		return false
	}
	delete(m.m, k)
	m.ord = slices.DeleteFunc(m.ord, func(x int) bool { return x == k })
	return true
}

// clear mirrors Clear.
func (m *lruModel) clear() {
	m.m = map[int]int{}
	m.ord = nil
}

// promote moves k to the MRU end.
func (m *lruModel) promote(k int) {
	m.ord = slices.DeleteFunc(m.ord, func(x int) bool { return x == k })
	m.ord = append(m.ord, k)
}

// lruOpKind enumerates the operations replayed by the randomized differential
// test and the fuzz target.
const (
	lruOpPut = iota
	lruOpGet
	lruOpPeek
	lruOpContains
	lruOpRemove
	lruOpClear
	lruOpKindCount
)

// applyLRUOp replays one operation on the real cache and the model and asserts
// they agree: return values, key order and length.
func applyLRUOp(t testing.TB, c *lru.LRU[int, int], m *lruModel, kind byte, k, v int) {
	t.Helper()
	switch kind % lruOpKindCount {
	case lruOpPut:
		ek, ev, e := c.Put(k, v)
		mek, mev, me := m.put(k, v)
		require.Equal(t, me, e, "Put evicted flag")
		if me {
			require.Equal(t, mek, ek, "Put evicted key")
			require.Equal(t, mev, ev, "Put evicted value")
		}
	case lruOpGet:
		got, gotOK := c.Get(k)
		want, wantOK := m.get(k)
		require.Equal(t, wantOK, gotOK, "Get ok")
		require.Equal(t, want, got, "Get value")
	case lruOpPeek:
		got, gotOK := c.Peek(k)
		want, wantOK := m.peek(k)
		require.Equal(t, wantOK, gotOK, "Peek ok")
		require.Equal(t, want, got, "Peek value")
	case lruOpContains:
		require.Equal(t, has(m, k), c.Contains(k), "Contains")
	case lruOpRemove:
		require.Equal(t, m.remove(k), c.Remove(k), "Remove")
	case lruOpClear:
		c.Clear()
		m.clear()
	}
	require.Equal(t, len(m.m), c.Len(), "Len")
	require.Equal(t, orNil(m.ord), orNil(c.Keys()), "key order must track the model")
}

// has reports whether the model contains k, even with a zero value.
func has(m *lruModel, k int) bool {
	_, ok := m.m[k]
	return ok
}

// runLRUOps replays data as an operation stream against a cache of the given
// capacity and the reference model, checking agreement after every step.
func runLRUOps(t testing.TB, capacity int, data []byte) {
	t.Helper()
	if capacity < 1 {
		capacity = 1
	}
	if capacity > 16 {
		capacity = 16
	}
	if len(data) > 256 {
		data = data[:256] // keep individual runs fast
	}
	c := lru.New[int, int](capacity)
	m := newLRUModel(capacity)
	for i := 0; i+3 <= len(data); i += 3 {
		applyLRUOp(t, c, m, data[i], int(data[i+1])%12, int(data[i+2])%8)
	}
}

// TestLRUModelRandomized replays random traffic against the map + recency
// model across several capacities.
func TestLRUModelRandomized(t *testing.T) {
	for _, seed := range [][2]uint64{{2026, 1012}, {7, 11}, {99, 4242}} {
		rng := rand.New(rand.NewPCG(seed[0], seed[1]))
		for _, cap := range []int{1, 2, 3, 8} {
			data := make([]byte, 240)
			for i := range data {
				data[i] = byte(rng.IntN(256))
			}
			runLRUOps(t, cap, data)
		}
	}
}

// TestLRUModelScripts replays fixed adversarial scripts against the model.
func TestLRUModelScripts(t *testing.T) {
	scripts := map[string][]byte{
		"eviction churn": {
			lruOpPut, 1, 1, lruOpPut, 2, 2, lruOpPut, 3, 3,
			lruOpGet, 1, 0, lruOpPut, 4, 4, lruOpPeek, 2, 0,
			lruOpPut, 5, 5, lruOpRemove, 3, 0,
		},
		"update heavy": {
			lruOpPut, 1, 1, lruOpPut, 1, 2, lruOpPut, 1, 3,
			lruOpGet, 1, 0, lruOpPut, 2, 7, lruOpPut, 1, 9,
		},
		"clear cycles": {
			lruOpPut, 1, 1, lruOpClear, 0, 0, lruOpPut, 2, 2,
			lruOpGet, 1, 0, lruOpPut, 3, 3, lruOpClear, 0, 0,
			lruOpPeek, 3, 0,
		},
	}
	for name, data := range scripts {
		t.Run(name, func(t *testing.T) {
			runLRUOps(t, 3, data)
		})
	}
}

// ---- concurrency -----------------------------------------------------------

// NOTE: lru documents "Not safe for concurrent use", so the concurrent tests
// below exercise Get/Put eviction traffic from many goroutines through a
// guarding mutex: they are race-clean workloads that keep CI's race detector
// honest about the container's internals while following the documented
// access pattern.

// TestConcurrentGetPutEviction runs Get/Put traffic from many goroutines
// against a small shared cache (serialized by a mutex, as the API requires)
// and checks the capacity invariant after every operation and the retrievability
// of the surviving keys at the end.
func TestConcurrentGetPutEviction(t *testing.T) {
	const (
		workers  = 8
		ops      = 200
		capacity = 16
	)
	c := lru.New[int, int](capacity)
	var mu sync.Mutex

	var (
		wg      sync.WaitGroup
		muErr   error
		errOnce sync.Once
	)
	fail := func(err error) { errOnce.Do(func() { muErr = err }) }

	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range ops {
				k := (w*ops + i) % (capacity * 2)
				mu.Lock()
				c.Put(k, i)
				c.Get(k)
				c.Peek(k)
				c.Contains(k)
				if c.Len() > capacity {
					fail(fmt.Errorf("Len() = %d exceeds capacity %d", c.Len(), capacity))
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	require.NoError(t, muErr)

	mu.Lock()
	defer mu.Unlock()
	require.LessOrEqual(t, c.Len(), capacity)
	for _, k := range c.Keys() {
		_, ok := c.Get(k)
		require.True(t, ok, "every key reported by Keys must be retrievable")
	}
}

// TestConcurrentReadersDuringEviction interleaves evicting writers with
// readers in lock-step batches; every observed Keys() snapshot is internally
// consistent (no duplicates, no dangling entries, length equals Len()).
func TestConcurrentReadersDuringEviction(t *testing.T) {
	const (
		writers  = 4
		readers  = 4
		rounds   = 100
		capacity = 8
	)
	c := lru.New[int, int](capacity)
	var mu sync.Mutex

	var (
		wg      sync.WaitGroup
		muErr   error
		errOnce sync.Once
	)
	fail := func(err error) { errOnce.Do(func() { muErr = err }) }

	for w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range rounds {
				mu.Lock()
				c.Put(w*rounds+i, i)
				if c.Len() > capacity {
					fail(fmt.Errorf("Len() = %d exceeds capacity %d", c.Len(), capacity))
				}
				mu.Unlock()
			}
		}()
	}
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				mu.Lock()
				keys := c.Keys()
				if len(keys) != c.Len() {
					fail(fmt.Errorf("len(Keys()) = %d != Len() = %d", len(keys), c.Len()))
				}
				seen := map[int]bool{}
				for _, k := range keys {
					if seen[k] {
						fail(fmt.Errorf("Keys() reported duplicate key %d", k))
					}
					seen[k] = true
					if !c.Contains(k) {
						fail(fmt.Errorf("key %d from Keys() is not contained", k))
					}
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	require.NoError(t, muErr)

	mu.Lock()
	defer mu.Unlock()
	require.LessOrEqual(t, c.Len(), capacity)
}

func TestEviction(t *testing.T) {
	c := lru.New[string, int](2)

	_, _, evicted := c.Put("a", 1)
	require.False(t, evicted)
	_, _, evicted = c.Put("b", 2)
	require.False(t, evicted)

	key, value, evicted := c.Put("c", 3)
	require.True(t, evicted)
	require.Equal(t, "a", key, "the least recently used entry must be evicted")
	require.Equal(t, 1, value)

	require.Equal(t, 2, c.Len())
	require.False(t, c.Contains("a"))
	require.True(t, c.Contains("b"))
	require.True(t, c.Contains("c"))
}

func TestGetPromotes(t *testing.T) {
	c := lru.New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)

	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, 1, v)

	ek, _, ev := c.Put("c", 3)
	require.True(t, ev)
	require.Equal(t, "b", ek, "Get must promote the accessed entry")
	require.True(t, c.Contains("a"))
	require.True(t, c.Contains("c"))
}

func TestPeekDoesNotPromote(t *testing.T) {
	c := lru.New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)

	v, ok := c.Peek("a")
	require.True(t, ok)
	require.Equal(t, 1, v)

	ek, _, ev := c.Put("c", 3)
	require.True(t, ev)
	require.Equal(t, "a", ek, "Peek must not change the recency order")
}

func TestContainsDoesNotPromote(t *testing.T) {
	c := lru.New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)

	require.True(t, c.Contains("a"))

	ek, _, ev := c.Put("c", 3)
	require.True(t, ev)
	require.Equal(t, "a", ek, "Contains must not change the recency order")
}

func TestPutUpdateNeverEvicts(t *testing.T) {
	c := lru.New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)

	_, _, evicted := c.Put("a", 11)
	require.False(t, evicted, "updating an existing key must not evict")
	require.True(t, c.Contains("b"))

	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, 11, v)

	// a was promoted by the update, so b becomes the eviction candidate
	ek, _, ev := c.Put("c", 3)
	require.True(t, ev)
	require.Equal(t, "b", ek)
}

func TestCapacityOne(t *testing.T) {
	c := lru.New[string, int](1)

	_, _, evicted := c.Put("a", 1)
	require.False(t, evicted)
	_, _, evicted = c.Put("a", 2)
	require.False(t, evicted, "capacity 1 must still allow in-place updates")

	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, 2, v)

	ek, _, ev := c.Put("b", 3)
	require.True(t, ev)
	require.Equal(t, "a", ek)
	require.False(t, c.Contains("a"))
}

func TestKeysOrder(t *testing.T) {
	c := lru.New[int, int](4)
	for i := range 4 {
		c.Put(i, i)
	}

	c.Get(1) // promotes 1
	c.Get(0) // promotes 0
	require.Equal(t, []int{2, 3, 1, 0}, c.Keys(), "Keys must go from LRU to MRU")

	var iterated []int
	for k := range c.All() {
		iterated = append(iterated, k)
	}
	require.Equal(t, []int{2, 3, 1, 0}, iterated)
}

func TestRemoveAndClear(t *testing.T) {
	c := lru.New[int, int](3)
	c.Put(1, 1)
	c.Put(2, 2)

	require.True(t, c.Remove(1))
	require.False(t, c.Remove(1), "Remove must report true exactly once")

	c.Put(3, 3)
	require.Equal(t, []int{2, 3}, c.Keys())

	m, ok := c.Get(3)
	require.True(t, ok)
	require.Equal(t, 3, m)

	c.Clear()
	require.Zero(t, c.Len())
	require.False(t, c.Contains(2))
	require.Empty(t, c.Keys())

	c.Put(9, 9)
	v, ok := c.Get(9)
	require.True(t, ok)
	require.Equal(t, 9, v, "cache must stay usable after Clear")
}

func TestCapacityAndAccessors(t *testing.T) {
	c := lru.New[string, int](4)
	require.Equal(t, 4, c.Capacity())

	_, ok := c.Get("missing")
	require.False(t, ok)
	_, ok = c.Peek("missing")
	require.False(t, ok)
	require.Zero(t, c.Len())
}

func TestGetDoesNotAllocate(t *testing.T) {
	c := lru.New[int, int](128)
	for i := range 128 {
		c.Put(i, i)
	}

	allocs := testing.AllocsPerRun(100, func() { c.Get(64) })
	require.Zero(t, allocs, "Get must not allocate (node is moved, not recreated)")
}

func TestKeysAreIndependentSnapshot(t *testing.T) {
	c := lru.New[int, int](4)
	c.Put(1, 1)
	c.Put(2, 2)

	keys := c.Keys()
	c.Put(3, 3)

	require.Equal(t, []int{1, 2}, keys, "Keys must return a snapshot")
	require.True(t, slices.Equal(keys, []int{1, 2}))
}
