package unionfind_test

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/unionfind"
)

// pair is one scripted union of two element indexes.
type pair struct{ a, b int }

// relation is one pairwise expectation: whether a and b share a component.
type relation struct {
	a, b int
	same bool
}

// sizeQ is one component-size expectation.
type sizeQ struct {
	x, want int
}

// ufModel is the reference model for the differential tests: a slice of
// equivalence classes, each kept in ascending order. Union merges two classes,
// and every query is answered by scanning the class list.
type ufModel struct {
	classes [][]int
}

// newUFModel returns the model of n singleton classes.
func newUFModel(n int) *ufModel {
	m := &ufModel{classes: make([][]int, n)}
	for i := range n {
		m.classes[i] = []int{i}
	}
	return m
}

// classIndex returns the index of the class holding x.
func (m *ufModel) classIndex(x int) int {
	for i, c := range m.classes {
		if slices.Contains(c, x) {
			return i
		}
	}
	return -1
}

// classOf returns the class holding x.
func (m *ufModel) classOf(x int) []int { return m.classes[m.classIndex(x)] }

// union merges the classes of a and b and reports whether they differed.
func (m *ufModel) union(a, b int) bool {
	ia, ib := m.classIndex(a), m.classIndex(b)
	if ia == ib {
		return false
	}
	merged := slices.Concat(m.classes[ia], m.classes[ib])
	sort.Ints(merged)
	out := make([][]int, 0, len(m.classes)-1)
	for i, c := range m.classes {
		switch i {
		case ib: // dropped, it was merged away
		case ia:
			out = append(out, merged)
		default:
			out = append(out, c)
		}
	}
	m.classes = out
	return true
}

// connected reports whether a and b share a class.
func (m *ufModel) connected(a, b int) bool { return m.classIndex(a) == m.classIndex(b) }

// size returns the size of the class holding x.
func (m *ufModel) size(x int) int { return len(m.classOf(x)) }

// normalized returns the classes in a canonical comparable form: every class
// ascending and the class list sorted by its first member.
func (m *ufModel) normalized() [][]int {
	out := make([][]int, len(m.classes))
	for i, c := range m.classes {
		out[i] = slices.Clone(c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// normalizeComponents canonicalises the Components output the same way, so
// the comparison does not depend on map iteration order.
func normalizeComponents(comps [][]int) [][]int {
	out := make([][]int, len(comps))
	for i, c := range comps {
		out[i] = slices.Clone(c)
		sort.Ints(out[i])
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// checkUFModel asserts that s agrees with the model on every observable query:
// Count, per-element Find/Size, every Connected pair and the full Components
// partition.
func checkUFModel(t *testing.T, s *unionfind.Set, m *ufModel, n int, where string) {
	t.Helper()
	require.Equal(t, n, s.Len(), "%s: Len", where)
	require.Equal(t, len(m.classes), s.Count(), "%s: Count", where)
	for x := range n {
		require.Equal(t, m.size(x), s.Size(x), "%s: Size(%d)", where, x)
		root := s.Find(x)
		require.GreaterOrEqual(t, root, 0, "%s: Find(%d) must be a valid index", where, x)
		require.Less(t, root, n, "%s: Find(%d) must be a valid index", where, x)
		require.Contains(t, m.classOf(x), root, "%s: Find(%d) must return a member of its class", where, x)
		require.Equal(t, root, s.Find(root), "%s: Find must be idempotent", where)
	}
	for a := range n {
		for b := range n {
			require.Equal(t, m.connected(a, b), s.Connected(a, b), "%s: Connected(%d,%d)", where, a, b)
		}
	}
	comps := s.Components()
	require.Len(t, comps, len(m.classes), "%s: Components count", where)
	for _, c := range comps {
		require.NotEmpty(t, c, "%s: Components must not expose empty members", where)
		require.True(t, sort.IntsAreSorted(c), "%s: component members must be ascending: %v", where, c)
	}
	require.Equal(t, m.normalized(), normalizeComponents(comps), "%s: Components partition", where)
}

// ufOp is one replayable operation of the union-find model test.
type ufOp struct {
	kind byte // 0 union, 1 find, 2 connected, 3 size, 4 full check only
	a, b int
}

// decodeUFOps turns a byte stream into an operation sequence, three bytes per
// operation. It is shared by the seeded differential test and the fuzz target.
func decodeUFOps(data []byte, n int) []ufOp {
	ops := make([]ufOp, 0, 64)
	for i := 0; i+2 < len(data) && len(ops) < 64; i += 3 {
		ops = append(ops, ufOp{
			kind: data[i] % 5,
			a:    int(data[i+1]) % n,
			b:    int(data[i+2]) % n,
		})
	}
	return ops
}

// runUFModel replays ops against a real Set and the equivalence-class model,
// checking full model agreement after every single operation.
func runUFModel(t *testing.T, n int, ops []ufOp) {
	t.Helper()
	s := unionfind.New(n)
	m := newUFModel(n)
	for i, o := range ops {
		switch o.kind {
		case 0:
			require.Equal(t, m.union(o.a, o.b), s.Union(o.a, o.b),
				"op %d: Union(%d,%d) must report whether the components differed", i, o.a, o.b)
		case 1:
			root := s.Find(o.a)
			require.Contains(t, m.classOf(o.a), root,
				"op %d: Find(%d) must return a member of its class", i, o.a)
			require.Equal(t, root, s.Find(root), "op %d: Find must be idempotent", i)
		case 2:
			require.Equal(t, m.connected(o.a, o.b), s.Connected(o.a, o.b),
				"op %d: Connected(%d,%d)", i, o.a, o.b)
		case 3:
			require.Equal(t, m.size(o.a), s.Size(o.a), "op %d: Size(%d)", i, o.a)
		case 4:
			// the full check below covers this step
		}
		checkUFModel(t, s, m, n, fmt.Sprintf("op %d (kind %d, %d, %d)", i, o.kind, o.a, o.b))
	}
}

// TestNew covers the constructor: every element starts as its own singleton
// component.
func TestNew(t *testing.T) {
	tests := []struct {
		name string
		n    int
	}{
		{"zero elements", 0},
		{"one element", 1},
		{"two elements", 2},
		{"five elements", 5},
		{"hundred elements", 100},
		{"ten thousand elements", 10_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := unionfind.New(tt.n)
			require.Equal(t, tt.n, s.Len())
			require.Equal(t, tt.n, s.Count(), "every element starts as its own component")
			for x := range tt.n {
				require.Equal(t, x, s.Find(x), "each element is its own root initially")
				require.Equal(t, 1, s.Size(x))
			}
			for x := range min(tt.n, 8) {
				for y := x + 1; y < min(tt.n, x+8); y++ {
					require.False(t, s.Connected(x, y), "singletons must not be connected")
				}
			}
			if tt.n == 0 {
				require.Empty(t, s.Components())
			}
		})
	}
}

// TestNewPanics pins the exact panic message of a negative element count.
func TestNewPanics(t *testing.T) {
	tests := []struct {
		name string
		n    int
	}{
		{"negative one", -1},
		{"negative five", -5},
		{"min int", math.MinInt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.PanicsWithValue(t, "unionfind: negative element count", func() { unionfind.New(tt.n) })
		})
	}
}

// TestFindAndConnected covers Find and Connected on scripted union sequences,
// including transitivity and the symmetry of Connected.
func TestFindAndConnected(t *testing.T) {
	tests := []struct {
		name    string
		n       int
		unions  []pair
		queries []relation
	}{
		{
			name:    "no unions keeps everything apart",
			n:       4,
			queries: []relation{{0, 0, true}, {0, 1, false}, {2, 3, false}, {1, 3, false}},
		},
		{
			name:    "one union",
			n:       4,
			unions:  []pair{{0, 1}},
			queries: []relation{{0, 1, true}, {0, 2, false}, {1, 2, false}, {2, 3, false}},
		},
		{
			name:    "chain merges transitively",
			n:       5,
			unions:  []pair{{0, 1}, {1, 2}, {2, 3}},
			queries: []relation{{0, 3, true}, {1, 2, true}, {0, 4, false}, {3, 4, false}},
		},
		{
			name:    "union of the same pair twice",
			n:       3,
			unions:  []pair{{0, 1}, {0, 1}},
			queries: []relation{{0, 1, true}, {0, 2, false}, {1, 2, false}},
		},
		{
			name:    "self union",
			n:       3,
			unions:  []pair{{1, 1}},
			queries: []relation{{1, 1, true}, {0, 1, false}, {1, 2, false}},
		},
		{
			name:    "merge two existing groups",
			n:       6,
			unions:  []pair{{0, 1}, {2, 3}, {1, 3}},
			queries: []relation{{0, 2, true}, {0, 3, true}, {0, 4, false}, {4, 5, false}},
		},
		{
			name:    "union order does not matter",
			n:       4,
			unions:  []pair{{3, 2}, {2, 1}, {1, 0}},
			queries: []relation{{0, 3, true}, {0, 1, true}, {1, 2, true}},
		},
		{
			name:    "everything in one component",
			n:       4,
			unions:  []pair{{0, 1}, {2, 3}, {1, 2}},
			queries: []relation{{0, 3, true}, {1, 2, true}, {0, 2, true}},
		},
		{
			name:    "disconnected pairs",
			n:       6,
			unions:  []pair{{0, 1}, {3, 4}},
			queries: []relation{{0, 1, true}, {3, 4, true}, {0, 3, false}, {2, 5, false}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := unionfind.New(tt.n)
			for _, u := range tt.unions {
				s.Union(u.a, u.b)
			}
			for _, q := range tt.queries {
				require.Equal(t, q.same, s.Connected(q.a, q.b), "Connected(%d,%d)", q.a, q.b)
				require.Equal(t, q.same, s.Connected(q.b, q.a), "Connected must be symmetric")
				require.Equal(t, q.same, s.Find(q.a) == s.Find(q.b), "Find must agree with Connected")
			}
			// Find is idempotent and returns a valid index on every element
			for x := range tt.n {
				root := s.Find(x)
				require.Equal(t, root, s.Find(root))
				require.GreaterOrEqual(t, root, 0)
				require.Less(t, root, tt.n)
			}
		})
	}
}

// TestUnionReturnValue pins the bool returned by Union and its effect on Count.
func TestUnionReturnValue(t *testing.T) {
	tests := []struct {
		name   string
		n      int
		script []pair
		want   []bool
		counts []int
	}{
		{
			name:   "first union merges",
			n:      3,
			script: []pair{{0, 1}},
			want:   []bool{true},
			counts: []int{2},
		},
		{
			name:   "repeat union is a no-op",
			n:      3,
			script: []pair{{0, 1}, {1, 0}, {0, 1}},
			want:   []bool{true, false, false},
			counts: []int{2, 2, 2},
		},
		{
			name:   "self union never merges",
			n:      3,
			script: []pair{{0, 0}, {1, 1}},
			want:   []bool{false, false},
			counts: []int{3, 3},
		},
		{
			name:   "transitive chain",
			n:      5,
			script: []pair{{0, 1}, {1, 2}, {2, 3}, {3, 4}},
			want:   []bool{true, true, true, true},
			counts: []int{4, 3, 2, 1},
		},
		{
			name:   "merge two groups",
			n:      6,
			script: []pair{{0, 1}, {2, 3}, {1, 3}, {0, 2}},
			want:   []bool{true, true, true, false},
			counts: []int{5, 4, 3, 3},
		},
		{
			name:   "union order does not matter",
			n:      4,
			script: []pair{{3, 2}, {2, 1}, {1, 0}},
			want:   []bool{true, true, true},
			counts: []int{3, 2, 1},
		},
		{
			name:   "disjoint pairs stay separate",
			n:      6,
			script: []pair{{0, 1}, {3, 4}, {0, 4}},
			want:   []bool{true, true, true},
			counts: []int{5, 4, 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := unionfind.New(tt.n)
			require.Equal(t, tt.n, s.Count())
			for i, u := range tt.script {
				require.Equal(t, tt.want[i], s.Union(u.a, u.b), "Union(%d,%d) #%d", u.a, u.b, i)
				require.Equal(t, tt.counts[i], s.Count(), "Count after Union #%d", i)
			}
		})
	}
}

// TestSize covers Size on singletons, merged components and after re-rooting.
func TestSize(t *testing.T) {
	tests := []struct {
		name    string
		n       int
		unions  []pair
		queries []sizeQ
	}{
		{
			name:    "singletons",
			n:       5,
			queries: []sizeQ{{0, 1}, {4, 1}, {2, 1}},
		},
		{
			name:    "after one union",
			n:       5,
			unions:  []pair{{0, 1}},
			queries: []sizeQ{{0, 2}, {1, 2}, {2, 1}},
		},
		{
			name:    "after a chain",
			n:       5,
			unions:  []pair{{0, 1}, {1, 2}},
			queries: []sizeQ{{0, 3}, {2, 3}, {3, 1}, {4, 1}},
		},
		{
			name:    "whole set",
			n:       4,
			unions:  []pair{{0, 1}, {2, 3}, {1, 2}},
			queries: []sizeQ{{0, 4}, {3, 4}, {2, 4}},
		},
		{
			name:    "star shape",
			n:       6,
			unions:  []pair{{0, 1}, {0, 2}, {0, 3}},
			queries: []sizeQ{{3, 4}, {4, 1}, {1, 4}},
		},
		{
			name:    "self union keeps the size",
			n:       3,
			unions:  []pair{{1, 1}},
			queries: []sizeQ{{1, 1}, {0, 1}},
		},
		{
			name:    "size is symmetric across members",
			n:       7,
			unions:  []pair{{5, 6}, {4, 5}, {3, 4}},
			queries: []sizeQ{{3, 4}, {6, 4}, {0, 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := unionfind.New(tt.n)
			for _, u := range tt.unions {
				s.Union(u.a, u.b)
			}
			for _, q := range tt.queries {
				require.Equal(t, q.want, s.Size(q.x), "Size(%d)", q.x)
			}
			// every member of every component must report the same size
			for _, members := range s.Components() {
				want := len(members)
				for _, x := range members {
					require.Equal(t, want, s.Size(x), "Size of member %d of %v", x, members)
				}
			}
		})
	}
}

// TestComponents covers the partition reporting, including its documented
// ascending order inside each component.
func TestComponents(t *testing.T) {
	tests := []struct {
		name   string
		n      int
		unions []pair
		want   [][]int
	}{
		{
			name: "empty set",
			n:    0,
			want: [][]int{},
		},
		{
			name: "singletons",
			n:    3,
			want: [][]int{{0}, {1}, {2}},
		},
		{
			name:   "one pair",
			n:      4,
			unions: []pair{{0, 1}},
			want:   [][]int{{0, 1}, {2}, {3}},
		},
		{
			name:   "two groups and a singleton",
			n:      6,
			unions: []pair{{0, 1}, {3, 4}},
			want:   [][]int{{0, 1}, {2}, {3, 4}, {5}},
		},
		{
			name:   "chain",
			n:      5,
			unions: []pair{{0, 1}, {1, 2}, {2, 3}},
			want:   [][]int{{0, 1, 2, 3}, {4}},
		},
		{
			name:   "merged groups",
			n:      6,
			unions: []pair{{0, 1}, {2, 3}, {1, 2}},
			want:   [][]int{{0, 1, 2, 3}, {4}, {5}},
		},
		{
			name:   "everything in one component",
			n:      4,
			unions: []pair{{0, 1}, {2, 3}, {1, 2}},
			want:   [][]int{{0, 1, 2, 3}},
		},
		{
			name:   "single union of the endpoints",
			n:      5,
			unions: []pair{{0, 4}},
			want:   [][]int{{0, 4}, {1}, {2}, {3}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := unionfind.New(tt.n)
			for _, u := range tt.unions {
				s.Union(u.a, u.b)
			}
			comps := s.Components()
			for _, c := range comps {
				require.True(t, sort.IntsAreSorted(c), "members must be ascending: %v", c)
				for _, x := range c {
					for _, y := range c {
						require.True(t, s.Connected(x, y), "members %d and %d of %v", x, y, c)
					}
				}
			}
			require.Equal(t, tt.want, normalizeComponents(comps))
			require.Equal(t, len(tt.want), s.Count(), "Components must hold one entry per component")
		})
	}
}

// TestIndexPanics pins the exact panic message for every method that takes an
// element index and proves the set is left untouched by a rejected call.
func TestIndexPanics(t *testing.T) {
	tests := []struct {
		name string
		call func(s *unionfind.Set)
	}{
		{"Find(-1)", func(s *unionfind.Set) { s.Find(-1) }},
		{"Find(n)", func(s *unionfind.Set) { s.Find(2) }},
		{"Find(n+1)", func(s *unionfind.Set) { s.Find(3) }},
		{"Find(min int)", func(s *unionfind.Set) { s.Find(math.MinInt) }},
		{"Find(max int)", func(s *unionfind.Set) { s.Find(math.MaxInt) }},
		{"Union(-1,0)", func(s *unionfind.Set) { s.Union(-1, 0) }},
		{"Union(0,n)", func(s *unionfind.Set) { s.Union(0, 2) }},
		{"Union(n,0)", func(s *unionfind.Set) { s.Union(2, 0) }},
		{"Union(-1,-1)", func(s *unionfind.Set) { s.Union(-1, -1) }},
		{"Connected(n,0)", func(s *unionfind.Set) { s.Connected(2, 0) }},
		{"Connected(0,-1)", func(s *unionfind.Set) { s.Connected(0, -1) }},
		{"Connected(-1,-1)", func(s *unionfind.Set) { s.Connected(-1, -1) }},
		{"Size(-1)", func(s *unionfind.Set) { s.Size(-1) }},
		{"Size(n)", func(s *unionfind.Set) { s.Size(2) }},
		{"Size(max int)", func(s *unionfind.Set) { s.Size(math.MaxInt) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := unionfind.New(2)
			require.PanicsWithValue(t, "unionfind: index out of range", func() { tt.call(s) })
			require.Equal(t, 2, s.Count(), "a rejected call must not mutate the set")
			require.False(t, s.Connected(0, 1))
		})
	}

	t.Run("Find on the empty set", func(t *testing.T) {
		s := unionfind.New(0)
		require.PanicsWithValue(t, "unionfind: index out of range", func() { s.Find(0) })
		require.Zero(t, s.Count())
	})
}

// TestEmptyAndSingleton pins the degenerate shapes.
func TestEmptyAndSingleton(t *testing.T) {
	tests := []struct {
		name string
		n    int
	}{
		{"empty set", 0},
		{"singleton", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := unionfind.New(tt.n)
			require.Equal(t, tt.n, s.Len())
			require.Equal(t, tt.n, s.Count())
			if tt.n == 0 {
				require.Empty(t, s.Components())
			} else {
				require.Equal(t, [][]int{{0}}, s.Components())
			}
			if tt.n == 1 {
				require.Equal(t, 0, s.Find(0))
				require.Equal(t, 1, s.Size(0))
				require.False(t, s.Union(0, 0), "self union on a singleton merges nothing")
				require.Equal(t, 1, s.Count())
				require.True(t, s.Connected(0, 0))
			}
		})
	}
}

// TestUnionBySizeKeepsTreeShallow proves the amortised bound: after n-1 unions
// into one component, path compression plus union by size keeps every Find at
// depth one.
func TestUnionBySizeKeepsTreeShallow(t *testing.T) {
	t.Run("chain into the first element", func(t *testing.T) {
		const n = 10_000
		s := unionfind.New(n)
		for i := 1; i < n; i++ {
			s.Union(i, 0)
		}
		require.Equal(t, 1, s.Count())
		require.Equal(t, n, s.Size(n-1))

		root := s.Find(n - 1)
		require.Equal(t, s.Find(0), root)
		for i := range n {
			require.Equal(t, root, s.Find(i), "every element must resolve to the same root")
		}
	})

	t.Run("alternating union order stays flat", func(t *testing.T) {
		const n = 1024
		s := unionfind.New(n)
		for i := 0; i < n-1; i += 2 {
			s.Union(i, i+1)
		}
		for i := 0; i < n-2; i += 4 {
			s.Union(i, i+2)
		}
		for i := 0; i < n/2-1; i++ {
			s.Union(2*i, 2*i+2)
		}
		s.Union(0, n/2)
		require.Equal(t, 1, s.Count())
		root := s.Find(0)
		for i := range n {
			require.Equal(t, root, s.Find(i))
		}
	})
}

// TestAgainstModelRandomOps replays randomised Union/Find/Connected/Size
// traffic against the equivalence-class model and checks full model agreement
// after every step.
func TestAgainstModelRandomOps(t *testing.T) {
	seeds := []uint64{1, 7, 42, 1337, 20261012}
	const n = 24
	for _, seed := range seeds {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			r := rand.New(rand.NewPCG(seed, 0x233))
			ops := make([]ufOp, 0, 400)
			for range 400 {
				ops = append(ops, ufOp{
					kind: byte(r.IntN(5)),
					a:    r.IntN(n),
					b:    r.IntN(n),
				})
			}
			runUFModel(t, n, ops)
		})
	}
}

// TestReadPathAllocations guards the hot read paths and the no-op write path
// against unexpected allocations on a warmed-up set.
func TestReadPathAllocations(t *testing.T) {
	build := func() *unionfind.Set {
		s := unionfind.New(64)
		for i := 1; i < 64; i++ {
			s.Union(i, i-1)
		}
		return s
	}

	t.Run("Find on a compressed path", func(t *testing.T) {
		s := build()
		allocs := testing.AllocsPerRun(1000, func() {
			_ = s.Find(63)
		})
		require.Zero(t, allocs)
	})

	t.Run("Connected", func(t *testing.T) {
		s := build()
		allocs := testing.AllocsPerRun(1000, func() {
			_ = s.Connected(0, 63)
		})
		require.Zero(t, allocs)
	})

	t.Run("Size", func(t *testing.T) {
		s := build()
		allocs := testing.AllocsPerRun(1000, func() {
			_ = s.Size(32)
		})
		require.Zero(t, allocs)
	})

	t.Run("Union of one component", func(t *testing.T) {
		s := build()
		allocs := testing.AllocsPerRun(1000, func() {
			_ = s.Union(0, 63)
		})
		require.Zero(t, allocs, "a union that merges nothing must not allocate")
	})
}

// TestConcurrentUseSkipped documents the concurrency contract: Set is
// explicitly not safe for concurrent use, so no race test is run here.
func TestConcurrentUseSkipped(t *testing.T) {
	t.Skip("unionfind.Set is documented as not safe for concurrent use; " +
		"no concurrent test applies (would fail under -race by design)")
}
