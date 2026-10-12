package pqueue_test

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/pqueue"
)

// NOTE: pqueue documents "Not safe for concurrent use", so no race/concurrency
// test is provided for this package (see latch and lru for the concurrent
// containers).

func intCmp(a, b int) int { return cmp.Compare(a, b) }

// TestPriorityQueueOpsMatrix drives Push/Pop/Peek/Len/Clear/ToSlice through a
// scripted operation table and checks the full observable state after every
// script. Script steps: "p" pushes value, "o" pops, "k" peeks, "c" clears.
func TestPriorityQueueOpsMatrix(t *testing.T) {
	tests := []struct {
		name   string
		script []string // e.g. "p1", "p2", "o", "k", "c"
		want   []int    // remaining contents in pop order
		pops   []int    // values popped by "o" steps (0 marker on empty miss)
	}{
		{"empty pop", []string{"o"}, nil, []int{0}},
		{"empty peek", []string{"k"}, nil, nil},
		{"single push pop", []string{"p1", "o"}, nil, []int{1}},
		{"single push peek", []string{"p1", "k"}, []int{1}, nil},
		{"push pop push pop", []string{"p1", "o", "p2", "o"}, nil, []int{1, 2}},
		{"two ascending", []string{"p1", "p2"}, []int{1, 2}, nil},
		{"two descending", []string{"p2", "p1"}, []int{1, 2}, nil},
		{"three ascending", []string{"p1", "p2", "p3"}, []int{1, 2, 3}, nil},
		{"three descending", []string{"p3", "p2", "p1"}, []int{1, 2, 3}, nil},
		{"three zigzag", []string{"p2", "p3", "p1"}, []int{1, 2, 3}, nil},
		{"pop all then empty", []string{"p2", "p1", "o", "o", "o"}, nil, []int{1, 2, 0}},
		{"peek is not pop", []string{"p2", "p1", "k", "k"}, []int{1, 2}, nil},
		{"clear empties", []string{"p1", "p2", "p3", "c"}, nil, nil},
		{"clear then reuse", []string{"p1", "c", "p5", "p4"}, []int{4, 5}, nil},
		{"clear on empty", []string{"c"}, nil, nil},
		{"duplicates", []string{"p2", "p2", "p1", "p2"}, []int{1, 2, 2, 2}, nil},
		{"duplicates pop", []string{"p2", "p1", "p2", "o", "o"}, []int{2}, []int{1, 2}},
		{"zero values", []string{"p0", "p0", "o"}, []int{0}, []int{0}},
		{"negatives", []string{"p-1", "p-5", "p-3"}, []int{-5, -3, -1}, nil},
		{"mixed signs", []string{"p0", "p-2", "p7", "p-2", "p3"}, []int{-2, -2, 0, 3, 7}, nil},
		{"grow beyond small", []string{"p5", "p4", "p3", "p2", "p1", "p0"}, []int{0, 1, 2, 3, 4, 5}, nil},
		{"push pop interleaved", []string{"p3", "o", "p1", "o", "p2", "o"}, nil, []int{3, 1, 2}},
		{"peek after pop", []string{"p3", "p1", "o", "k"}, []int{3}, []int{1}},
		{"pop then clear then pop", []string{"p1", "o", "c", "o"}, nil, []int{1, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := pqueue.New[int]()
			var pops []int
			for _, step := range tt.script {
				switch step[0] {
				case 'p':
					v := atoi(step[1:])
					q.Push(v)
				case 'o':
					v, ok := q.Pop()
					if ok {
						pops = append(pops, v)
					} else {
						pops = append(pops, 0) // zero value marker
					}
				case 'k':
					v, ok := q.Peek()
					if q.Len() == 0 {
						require.False(t, ok, "Peek on empty queue must miss")
					} else {
						require.True(t, ok, "peek on non-empty queue must hit")
						require.Equal(t, mustMin(t, q), v, "Peek must return the current minimum")
					}
				case 'c':
					q.Clear()
					require.Zero(t, q.Len())
				}
			}
			require.Equal(t, tt.pops, pops)

			// remaining contents must drain in sorted order
			var got []int
			for {
				v, ok := q.Pop()
				if !ok {
					break
				}
				got = append(got, v)
			}
			require.Equal(t, tt.want, got)
			require.Zero(t, q.Len())
		})
	}
}

// mustMin returns the smallest of the queue's current contents (via ToSlice).
func mustMin[T cmp.Ordered](t *testing.T, q *pqueue.PriorityQueue[T]) T {
	t.Helper()
	s := q.ToSlice()
	require.NotEmpty(t, s)
	return slices.Min(s)
}

// atoi parses a (possibly negative) decimal literal from the script step.
func atoi(s string) int {
	neg := false
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

// TestPriorityQueueConstructorsMatrix covers the constructor surface and its
// panic contract.
func TestPriorityQueueConstructorsMatrix(t *testing.T) {
	tests := []struct {
		name  string
		build func() *pqueue.PriorityQueue[int]
		want  []int // drain order
	}{
		{
			name:  "New natural min",
			build: func() *pqueue.PriorityQueue[int] { return pqueue.New[int]() },
		},
		{
			name: "NewFunc ascending",
			build: func() *pqueue.PriorityQueue[int] {
				q := pqueue.NewFunc(func(a, b int) int { return a - b })
				q.Push(3)
				q.Push(9)
				q.Push(5)
				return q
			},
			want: []int{3, 5, 9},
		},
		{
			name: "NewFunc descending max-queue",
			build: func() *pqueue.PriorityQueue[int] {
				q := pqueue.NewFunc(func(a, b int) int { return b - a })
				q.Push(3)
				q.Push(9)
				q.Push(5)
				return q
			},
			want: []int{9, 5, 3},
		},
		{
			name: "NewFromSlice empty",
			build: func() *pqueue.PriorityQueue[int] {
				return pqueue.NewFromSlice([]int{}, intCmp)
			},
			want: nil,
		},
		{
			name: "NewFromSlice nil",
			build: func() *pqueue.PriorityQueue[int] {
				return pqueue.NewFromSlice(nil, intCmp)
			},
			want: nil,
		},
		{
			name: "NewFromSlice sorted",
			build: func() *pqueue.PriorityQueue[int] {
				return pqueue.NewFromSlice([]int{1, 2, 3}, intCmp)
			},
			want: []int{1, 2, 3},
		},
		{
			name: "NewFromSlice reversed",
			build: func() *pqueue.PriorityQueue[int] {
				return pqueue.NewFromSlice([]int{9, 5, 3}, intCmp)
			},
			want: []int{3, 5, 9},
		},
		{
			name: "NewFromSlice duplicates",
			build: func() *pqueue.PriorityQueue[int] {
				return pqueue.NewFromSlice([]int{2, 1, 2, 1}, intCmp)
			},
			want: []int{1, 1, 2, 2},
		},
		{
			name: "NewFromSlice negatives",
			build: func() *pqueue.PriorityQueue[int] {
				return pqueue.NewFromSlice([]int{-1, -7, 0, 4}, intCmp)
			},
			want: []int{-7, -1, 0, 4},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := tt.build()
			require.Equal(t, len(tt.want), q.Len())

			var got []int
			for {
				v, ok := q.Pop()
				if !ok {
					break
				}
				got = append(got, v)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNewFuncPanicsOnNilComparator(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
		msg  string
	}{
		{"generic int nil", func() { pqueue.NewFunc[int](nil) }, "pqueue: nil comparator"},
		{"generic string nil", func() { pqueue.NewFunc[string](nil) }, "pqueue: nil comparator"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.PanicsWithValue(t, tt.msg, tt.fn)
		})
	}
}

// TestPriorityQueueGenericTypes exercises non-int element types and string
// comparators.
func TestPriorityQueueGenericTypes(t *testing.T) {
	t.Run("string min", func(t *testing.T) {
		q := pqueue.New[string]()
		for _, s := range []string{"pear", "apple", "mango"} {
			q.Push(s)
		}
		var got []string
		for {
			v, ok := q.Pop()
			if !ok {
				break
			}
			got = append(got, v)
		}
		require.Equal(t, []string{"apple", "mango", "pear"}, got)
	})

	t.Run("string by length", func(t *testing.T) {
		q := pqueue.NewFunc(func(a, b string) int { return len(a) - len(b) })
		for _, s := range []string{"aaaa", "b", "cc"} {
			q.Push(s)
		}
		got, _ := q.Pop()
		require.Equal(t, "b", got)
	})

	t.Run("struct element", func(t *testing.T) {
		type task struct {
			prio int
			name string
		}
		q := pqueue.NewFunc(func(a, b task) int { return cmp.Compare(a.prio, b.prio) })
		q.Push(task{2, "low"})
		q.Push(task{1, "high"})
		got, ok := q.Pop()
		require.True(t, ok)
		require.Equal(t, "high", got.name)
	})
}

func TestMinQueueDefaultOrder(t *testing.T) {
	q := pqueue.New[int]()

	_, ok := q.Pop()
	require.False(t, ok, "Pop on empty queue must miss")
	_, ok = q.Peek()
	require.False(t, ok, "Peek on empty queue must miss")

	items := make([]int, 1000)
	rng := rand.New(rand.NewPCG(7, 7))
	for i := range items {
		items[i] = rng.IntN(100000)
		q.Push(items[i])
	}
	require.Equal(t, len(items), q.Len())

	top, ok := q.Peek()
	require.True(t, ok)
	require.Equal(t, slices.Min(items), top)

	slices.Sort(items)
	for i, want := range items {
		got, ok := q.Pop()
		require.True(t, ok, "Pop #%d must succeed", i)
		require.Equal(t, want, got, "Pop #%d must follow the min-heap order", i)
	}
	require.Zero(t, q.Len())
}

func TestMaxQueueCustomComparator(t *testing.T) {
	q := pqueue.NewFunc(func(a, b int) int { return b - a })
	for _, v := range []int{5, 1, 9, 3} {
		q.Push(v)
	}

	var got []int
	for {
		v, ok := q.Pop()
		if !ok {
			break
		}
		got = append(got, v)
	}
	require.Equal(t, []int{9, 5, 3, 1}, got, "reversed comparator must yield a max-queue")
}

func TestNewFromSlice(t *testing.T) {
	src := []int{9, 4, 7, 1, 8}
	q := pqueue.NewFromSlice(src, func(a, b int) int { return a - b })
	require.Equal(t, len(src), q.Len())

	var got []int
	for {
		v, ok := q.Pop()
		if !ok {
			break
		}
		got = append(got, v)
	}
	require.Equal(t, []int{1, 4, 7, 8, 9}, got)
	require.Equal(t, []int{9, 4, 7, 1, 8}, src, "heap construction must not reorder the input slice")
}

func TestClear(t *testing.T) {
	q := pqueue.New[int]()
	q.Push(1)
	q.Push(2)

	q.Clear()
	require.Zero(t, q.Len())
	_, ok := q.Pop()
	require.False(t, ok)
	_, ok = q.Peek()
	require.False(t, ok)

	q.Push(3)
	v, ok := q.Peek()
	require.True(t, ok)
	require.Equal(t, 3, v, "queue must stay usable after Clear")
}

func TestToSliceIsIndependent(t *testing.T) {
	q := pqueue.New[int]()
	q.Push(1)

	s := q.ToSlice()
	s[0] = 99

	v, _ := q.Peek()
	require.Equal(t, 1, v, "ToSlice must return an independent copy")
}

// ---- differential test against a sort.Slice model --------------------------

// sortedModel is the reference model: elements are kept in a slice and the
// dequeue order is the stable sort of everything ever pushed.
type sortedModel struct {
	less func(a, b int) int
	all  []int // everything pushed, in push order
	live []int // elements not yet popped, in push order
}

// newSortedModel returns an empty model ordered by less.
func newSortedModel(less func(a, b int) int) *sortedModel {
	return &sortedModel{less: less}
}

// push records one element.
func (m *sortedModel) push(v int) {
	m.all = append(m.all, v)
	m.live = append(m.live, v)
}

// pop removes the first element of the stably sorted live slice.
func (m *sortedModel) pop() (int, bool) {
	if len(m.live) == 0 {
		return 0, false
	}
	v := m.liveOrder()[0]
	// drop the popped element from live (first occurrence of v in push order
	// among the minimal ones)
	for i, x := range m.live {
		if x == v {
			m.live = slices.Delete(m.live, i, i+1)
			break
		}
	}
	return v, true
}

// liveOrder returns the model's expected dequeue order of the live elements.
func (m *sortedModel) liveOrder() []int { return m.sortOrder(m.live) }

// drainOrder returns the full dequeue order model for m.all without mutating.
func (m *sortedModel) drainOrder() []int { return m.sortOrder(m.all) }

// sortOrder returns vals stably sorted by the comparator, tie-broken by
// position in vals (i.e. push order).
func (m *sortedModel) sortOrder(vals []int) []int {
	type item struct {
		v   int
		idx int
	}
	items := make([]item, len(vals))
	for i, v := range vals {
		items[i] = item{v, i}
	}
	slices.SortStableFunc(items, func(a, b item) int {
		if c := m.less(a.v, b.v); c != 0 {
			return c
		}
		return cmp.Compare(a.idx, b.idx)
	})
	out := make([]int, len(items))
	for i, it := range items {
		out[i] = it.v
	}
	return out
}

// pqOp is one replayable queue mutation: push (value set) or pop.
type pqOp struct {
	push bool
	v    int
}

// maxPQOps bounds the replay length so fuzzing stays fast.
const maxPQOps = 128

// decodePQOps turns arbitrary bytes into queue mutations, two bytes per
// mutation: the kind and the value. Values are small so the model's tie-break
// logic gets a real workout.
func decodePQOps(data []byte) []pqOp {
	ops := make([]pqOp, 0, maxPQOps)
	for i := 0; i+2 <= len(data) && len(ops) < maxPQOps; i += 2 {
		if data[i]%3 == 0 {
			ops = append(ops, pqOp{push: false})
		} else {
			ops = append(ops, pqOp{push: true, v: int(data[i+1])%8 - 4})
		}
	}
	return ops
}

// runPQModel replays ops against a real queue and the sort model: after every
// step Len must agree, Peek must match the model's smallest live element, and
// the final drain must equal the model's full dequeue order.
func runPQModel(t testing.TB, ops []pqOp, compare func(a, b int) int) {
	t.Helper()
	q := pqueue.NewFunc(compare)
	m := newSortedModel(compare)

	for i, o := range ops {
		if o.push {
			q.Push(o.v)
			m.push(o.v)
		} else {
			got, gotOK := q.Pop()
			want, wantOK := m.pop()
			require.Equal(t, wantOK, gotOK, "op %d: Pop ok", i)
			if wantOK {
				require.Equal(t, want, got, "op %d: Pop value", i)
			}
		}
		require.Equal(t, len(m.live), q.Len(), "op %d: Len", i)
		if len(m.live) > 0 {
			top, ok := q.Peek()
			require.True(t, ok, "op %d: Peek on non-empty", i)
			require.Equal(t, m.liveOrder()[0], top, "op %d: Peek must return the smallest", i)
		}
	}

	var got []int
	for {
		v, ok := q.Pop()
		if !ok {
			break
		}
		got = append(got, v)
	}
	require.Equal(t, orNil(m.liveOrder()), orNil(got), "final drain must equal the sorted live elements")
}

// orNil normalizes an empty slice to nil so require.Equal can compare it
// against model output without nil-vs-empty noise.
func orNil(s []int) []int {
	if len(s) == 0 {
		return nil
	}
	return s
}

// TestPriorityQueueModelRandomized replays random Push/Pop traffic against the
// sort.Slice reference model.
func TestPriorityQueueModelRandomized(t *testing.T) {
	compares := map[string]func(a, b int) int{
		"min": func(a, b int) int { return a - b },
		"max": func(a, b int) int { return b - a },
	}
	for name, compare := range compares {
		t.Run(name, func(t *testing.T) {
			for _, seed := range [][2]uint64{{2026, 1012}, {7, 11}, {99, 4242}} {
				rng := rand.New(rand.NewPCG(seed[0], seed[1]))
				data := make([]byte, 2*maxPQOps)
				for i := range data {
					data[i] = byte(rng.IntN(256))
				}
				runPQModel(t, decodePQOps(data), compare)
			}
		})
	}
}

// TestPriorityQueueModelScripts replays fixed adversarial scripts (duplicate
// heavy, pop-heavy, all-push) against the model.
func TestPriorityQueueModelScripts(t *testing.T) {
	scripts := map[string][]pqOp{
		"all pushes": {
			{push: true, v: 3}, {push: true, v: 1}, {push: true, v: 2},
			{push: true, v: 1}, {push: true, v: 3}, {push: true, v: 2},
		},
		"pop heavy": {
			{push: true, v: 0}, {push: false}, {push: false},
			{push: true, v: -1}, {push: true, v: 5}, {push: false},
			{push: false}, {push: false},
		},
		"alternating": {
			{push: true, v: 2}, {push: false}, {push: true, v: 2},
			{push: true, v: -3}, {push: false}, {push: true, v: -3},
			{push: false}, {push: false},
		},
	}
	for name, ops := range scripts {
		t.Run(name, func(t *testing.T) {
			runPQModel(t, ops, func(a, b int) int { return a - b })
		})
	}
}
