package deque_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/deque"
)

// NOTE: deque documents "Not safe for concurrent use", so no race/concurrency
// test is provided for this package (see latch and lru for the concurrent
// containers).

// TestDequeOpsMatrix drives PushFront/PushBack/PopFront/PopBack/Front/Back/
// Len/Cap/Clear/ToSlice through scripted operation tables and checks the full
// observable state after every script.
func TestDequeOpsMatrix(t *testing.T) {
	tests := []struct {
		name   string
		script []string // "+1" PushBack, "-1" PushFront, ">v" PopFront, "<v" PopBack,
		//        "f" Front, "b" Back, "c" Clear
		want  []int // contents front-to-back afterwards
		popF  []int // PopFront results (0 marker on miss)
		popB  []int // PopBack results (0 marker on miss)
		front []int // Front results (0 marker on miss)
		back  []int // Back results (0 marker on miss)
	}{
		{"empty pop front", []string{">"}, nil, []int{0}, nil, nil, nil},
		{"empty pop back", []string{"<"}, nil, nil, []int{0}, nil, nil},
		{"empty front back", []string{"f", "b"}, nil, nil, nil, []int{0}, []int{0}},
		{"single push back pop front", []string{"+1", ">"}, nil, []int{1}, nil, nil, nil},
		{"single push back pop back", []string{"+1", "<"}, nil, nil, []int{1}, nil, nil},
		{"single push front pop front", []string{"-1", ">"}, nil, []int{1}, nil, nil, nil},
		{"single push front pop back", []string{"-1", "<"}, nil, nil, []int{1}, nil, nil},
		{"fifo", []string{"+1", "+2", "+3", ">", ">", ">"}, nil, []int{1, 2, 3}, nil, nil, nil},
		{"lifo", []string{"+1", "+2", "+3", "<", "<", "<"}, nil, nil, []int{3, 2, 1}, nil, nil},
		{"push front order", []string{"-1", "-2", "-3"}, []int{3, 2, 1}, nil, nil, nil, nil},
		{"both ends mixed", []string{"-2", "+3", "-1", "+4"}, []int{1, 2, 3, 4}, nil, nil, nil, nil},
		{"pop front miss after drain", []string{"+1", ">", ">"}, nil, []int{1, 0}, nil, nil, nil},
		{"pop back miss after drain", []string{"+1", "<", "<"}, nil, nil, []int{1, 0}, nil, nil},
		{"front back peek", []string{"+1", "+2", "f", "b"}, []int{1, 2}, nil, nil, []int{1}, []int{2}},
		{"front is not pop", []string{"+1", "f", "f"}, []int{1}, nil, nil, []int{1, 1}, nil},
		{"back is not pop", []string{"+1", "+2", "b", "b"}, []int{1, 2}, nil, nil, nil, []int{2, 2}},
		{"clear empties", []string{"+1", "+2", "c"}, nil, nil, nil, nil, nil},
		{"clear then reuse", []string{"+1", "c", "+5", "-4"}, []int{4, 5}, nil, nil, nil, nil},
		{"clear on empty", []string{"c"}, nil, nil, nil, nil, nil},
		{"alternating push pop", []string{"+1", ">", "+2", "<", "+3", ">"}, nil, []int{1, 3}, []int{2}, nil, nil},
		{"wrap then drain", []string{"+1", "+2", "+3", "+4", ">", ">", "+5", "+6"}, []int{3, 4, 5, 6}, []int{1, 2}, nil, nil, nil},
		{"grow via push front", []string{"-1", "-2", "-3", "-4", "-5", "-6", "-7", "-8", "-9", "-10"},
			[]int{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}, nil, nil, nil, nil},
		{"duplicates", []string{"+7", "+7", "-7"}, []int{7, 7, 7}, nil, nil, nil, nil},
		{"zeros", []string{"+0", "-0", "<", ">"}, nil, []int{0}, []int{0}, nil, nil},
		{"interleaved ends", []string{"+2", "-1", "+3", ">", "<", ">", "<"}, nil,
			[]int{1, 2}, []int{3, 0}, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := deque.New[int]()
			var popF, popB, front, back []int
			for _, step := range tt.script {
				switch step[0] {
				case '+':
					d.PushBack(atoi(step[1:]))
				case '-':
					d.PushFront(atoi(step[1:]))
				case '>':
					v, ok := d.PopFront()
					if ok {
						popF = append(popF, v)
					} else {
						popF = append(popF, 0)
					}
				case '<':
					v, ok := d.PopBack()
					if ok {
						popB = append(popB, v)
					} else {
						popB = append(popB, 0)
					}
				case 'f':
					v, ok := d.Front()
					if ok {
						front = append(front, v)
					} else {
						front = append(front, 0)
					}
				case 'b':
					v, ok := d.Back()
					if ok {
						back = append(back, v)
					} else {
						back = append(back, 0)
					}
				case 'c':
					d.Clear()
				}
			}
			require.Equal(t, tt.popF, popF, "PopFront results")
			require.Equal(t, tt.popB, popB, "PopBack results")
			require.Equal(t, tt.front, front, "Front results")
			require.Equal(t, tt.back, back, "Back results")
			require.Equal(t, tt.want, orNil(d.ToSlice()), "final contents")
			require.Equal(t, len(tt.want), d.Len())
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

// atoi parses a (possibly negative) decimal literal from the script step.
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

// TestDequeConstructionMatrix covers the constructor surface: capacity
// clamping to minCapacity and the exact requested capacity.
func TestDequeConstructionMatrix(t *testing.T) {
	tests := []struct {
		name    string
		build   func() *deque.ArrayDeque[int]
		wantCap int
	}{
		{"default", func() *deque.ArrayDeque[int] { return deque.New[int]() }, 8},
		{"zero capacity", func() *deque.ArrayDeque[int] { return deque.New[int](0) }, 8},
		{"negative capacity", func() *deque.ArrayDeque[int] { return deque.New[int](-5) }, 8},
		{"tiny capacity", func() *deque.ArrayDeque[int] { return deque.New[int](4) }, 8},
		{"exact min", func() *deque.ArrayDeque[int] { return deque.New[int](8) }, 8},
		{"above min", func() *deque.ArrayDeque[int] { return deque.New[int](16) }, 16},
		{"large", func() *deque.ArrayDeque[int] { return deque.New[int](1024) }, 1024},
		{"extra args ignored", func() *deque.ArrayDeque[int] { return deque.New[int](16, 32) }, 16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tt.build()
			require.Equal(t, tt.wantCap, d.Cap())
			require.Zero(t, d.Len())
			require.Empty(t, d.ToSlice())
			_, ok := d.PopFront()
			require.False(t, ok)
			_, ok = d.PopBack()
			require.False(t, ok)
		})
	}
}

// TestDequeAllMatrix checks the iteration order in various wrap states.
func TestDequeAllMatrix(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *deque.ArrayDeque[int]
		want  []int
	}{
		{"empty", func() *deque.ArrayDeque[int] { return deque.New[int]() }, nil},
		{"push back", func() *deque.ArrayDeque[int] {
			d := deque.New[int]()
			d.PushBack(1)
			d.PushBack(2)
			d.PushBack(3)
			return d
		}, []int{1, 2, 3}},
		{"push front", func() *deque.ArrayDeque[int] {
			d := deque.New[int]()
			d.PushFront(3)
			d.PushFront(2)
			d.PushFront(1)
			return d
		}, []int{1, 2, 3}},
		{"wrapped head", func() *deque.ArrayDeque[int] {
			d := deque.New[int](8)
			for i := range 6 {
				d.PushBack(i)
			}
			for range 4 {
				d.PopFront()
			}
			d.PushBack(6)
			d.PushBack(7)
			return d
		}, []int{4, 5, 6, 7}},
		{"wrapped back pushes", func() *deque.ArrayDeque[int] {
			d := deque.New[int](8)
			for i := range 4 {
				d.PushFront(i)
			}
			for i := range 4 {
				d.PushBack(10 + i)
			}
			return d
		}, []int{3, 2, 1, 0, 10, 11, 12, 13}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tt.setup()
			var got []int
			for v := range d.All() {
				got = append(got, v)
			}
			require.Equal(t, tt.want, orNil(got), "All must walk front to back")
			require.Equal(t, orNil(d.ToSlice()), orNil(got), "All must agree with ToSlice")

			// early break must stop iteration
			count := 0
			for range d.All() {
				count++
				break
			}
			if len(tt.want) == 0 {
				require.Zero(t, count)
			} else {
				require.Equal(t, 1, count)
			}
		})
	}
}

// TestDequeGrowthInvariant checks Cap never shrinks and grows on demand.
func TestDequeGrowthInvariant(t *testing.T) {
	d := deque.New[int](8)
	require.Equal(t, 8, d.Cap())

	for i := range 8 {
		d.PushBack(i)
	}
	require.Equal(t, 8, d.Cap(), "filling to capacity must not grow yet")

	d.PushBack(8)
	require.Equal(t, 16, d.Cap(), "the next push must double the buffer")

	// draining keeps the capacity
	for range 9 {
		d.PopFront()
	}
	require.Equal(t, 16, d.Cap())
	require.Zero(t, d.Len())

	// push front growth
	for i := range 16 {
		d.PushFront(i)
	}
	require.Equal(t, 16, d.Cap())
	d.PushFront(99)
	require.Equal(t, 32, d.Cap())
	require.Equal(t, 17, d.Len())
}

func TestFIFO(t *testing.T) {
	d := deque.New[int]()
	for i := range 5 {
		d.PushBack(i)
	}
	for i := range 5 {
		v, ok := d.PopFront()
		require.True(t, ok)
		require.Equal(t, i, v, "PopFront #%d", i)
	}

	_, ok := d.PopFront()
	require.False(t, ok, "PopFront on empty deque must miss")
}

func TestLIFO(t *testing.T) {
	d := deque.New[string]()
	d.PushBack("a")
	d.PushBack("b")
	d.PushBack("c")

	v, ok := d.PopBack()
	require.True(t, ok)
	require.Equal(t, "c", v)
	v, ok = d.PopBack()
	require.True(t, ok)
	require.Equal(t, "b", v)
}

func TestBothEnds(t *testing.T) {
	d := deque.New[int]()
	d.PushFront(2)
	d.PushFront(1)
	d.PushBack(3)

	require.Equal(t, []int{1, 2, 3}, d.ToSlice())

	front, ok := d.Front()
	require.True(t, ok)
	require.Equal(t, 1, front)

	back, ok := d.Back()
	require.True(t, ok)
	require.Equal(t, 3, back)

	require.Equal(t, 3, d.Len())
}

func TestGrowth(t *testing.T) {
	d := deque.New[int]() // starts at minCapacity
	for i := range 1000 {
		d.PushFront(i)
		d.PushBack(i)
	}
	require.Equal(t, 2000, d.Len())

	want := make([]int, 0, 2000)
	for i := 999; i >= 0; i-- {
		want = append(want, i)
	}
	for i := range 1000 {
		want = append(want, i)
	}
	require.Equal(t, want, d.ToSlice(), "content must survive repeated growth")

	for i := range 500 {
		front, okF := d.PopFront()
		back, okB := d.PopBack()
		require.True(t, okF)
		require.True(t, okB)
		require.Equal(t, 999-i, front)
		require.Equal(t, 999-i, back)
	}
	require.Equal(t, 1000, d.Len())
}

func TestEmptyAccessors(t *testing.T) {
	d := deque.New[int]()

	_, ok := d.Front()
	require.False(t, ok, "Front on empty deque must miss")
	_, ok = d.Back()
	require.False(t, ok, "Back on empty deque must miss")
	_, ok = d.PopBack()
	require.False(t, ok, "PopBack on empty deque must miss")
}

func TestClearAndReuse(t *testing.T) {
	d := deque.New[int](64)
	d.PushBack(1)
	d.PushFront(0)
	capacityBefore := d.Cap()

	d.Clear()
	require.Zero(t, d.Len())
	require.Equal(t, capacityBefore, d.Cap(), "Clear must keep the allocated capacity")

	d.PushBack(7)
	v, ok := d.PopFront()
	require.True(t, ok)
	require.Equal(t, 7, v)
}

func TestAll(t *testing.T) {
	d := deque.New[int]()
	d.PushBack(1)
	d.PushBack(2)
	d.PushBack(3)

	var got []int
	for v := range d.All() {
		got = append(got, v)
	}
	require.Equal(t, []int{1, 2, 3}, got)

	count := 0
	for range d.All() {
		count++
		break
	}
	require.Equal(t, 1, count, "break must stop the iteration")
}

func TestInitialCapacity(t *testing.T) {
	d := deque.New[int](1)
	require.GreaterOrEqual(t, d.Cap(), 8, "capacity must never drop below minCapacity")

	big := deque.New[int](128)
	require.Equal(t, 128, big.Cap())
}

func TestPushPopDoesNotAllocate(t *testing.T) {
	d := deque.New[int](64)

	d.PushBack(1) // warm up
	allocs := testing.AllocsPerRun(100, func() {
		d.PushBack(2)
		d.PopFront()
	})
	require.Zero(t, allocs, "steady-state PushBack+PopFront must not allocate")
	require.Equal(t, 1, d.Len())
}

// TestFrontBackDoNotAllocate guards the read paths: Front and Back on a
// non-empty deque must not allocate.
func TestFrontBackDoNotAllocate(t *testing.T) {
	d := deque.New[int](64)
	d.PushBack(1)
	d.PushBack(2)

	allocs := testing.AllocsPerRun(100, func() {
		d.Front()
		d.Back()
	})
	require.Zero(t, allocs, "Front/Back must not allocate")
	require.Equal(t, 2, d.Len())
}

func TestToSliceIsIndependent(t *testing.T) {
	d := deque.New[int]()
	d.PushBack(1)

	s := d.ToSlice()
	s[0] = 42

	v, _ := d.Front()
	require.Equal(t, 1, v, "ToSlice must return an independent copy")
}

func TestWraparound(t *testing.T) {
	d := deque.New[int](8)
	// repeatedly wrap the head around the end of the buffer
	for i := range 8 {
		d.PushBack(i)
	}
	for i := range 4 {
		v, ok := d.PopFront()
		require.True(t, ok)
		require.Equal(t, i, v)
	}
	for i := range 4 {
		d.PushBack(100 + i)
	}
	require.Equal(t, []int{4, 5, 6, 7, 100, 101, 102, 103}, d.ToSlice())

	// same for the back index
	wantBack := []int{103, 102, 101, 100, 7, 6}
	for i, want := range wantBack {
		v, ok := d.PopBack()
		require.True(t, ok)
		require.Equal(t, want, v, "PopBack #%d", i)
	}
	require.Equal(t, []int{4, 5}, d.ToSlice())
}

// ---- differential test against a slice model -------------------------------

// sliceModel is the reference model: a plain Go slice used as a deque.
type sliceModel struct {
	live []int
}

// pushBack mirrors PushBack.
func (m *sliceModel) pushBack(v int) { m.live = append(m.live, v) }

// pushFront mirrors PushFront.
func (m *sliceModel) pushFront(v int) { m.live = append([]int{v}, m.live...) }

// popFront mirrors PopFront.
func (m *sliceModel) popFront() (int, bool) {
	if len(m.live) == 0 {
		return 0, false
	}
	v := m.live[0]
	m.live = m.live[1:]
	return v, true
}

// popBack mirrors PopBack.
func (m *sliceModel) popBack() (int, bool) {
	if len(m.live) == 0 {
		return 0, false
	}
	v := m.live[len(m.live)-1]
	m.live = m.live[:len(m.live)-1]
	return v, true
}

// dequeOpKind enumerates the operations replayed by the randomized
// differential test and the fuzz target.
const (
	dequeOpPushBack = iota
	dequeOpPushFront
	dequeOpPopFront
	dequeOpPopBack
	dequeOpFront
	dequeOpBack
	dequeOpClear
	dequeOpKindCount
)

// applyDequeOp replays one operation on the real deque and the model and
// asserts they agree.
func applyDequeOp(t testing.TB, d *deque.ArrayDeque[int], m *sliceModel, kind byte, v int) {
	t.Helper()
	switch kind % dequeOpKindCount {
	case dequeOpPushBack:
		d.PushBack(v)
		m.pushBack(v)
	case dequeOpPushFront:
		d.PushFront(v)
		m.pushFront(v)
	case dequeOpPopFront:
		got, gotOK := d.PopFront()
		want, wantOK := m.popFront()
		require.Equal(t, wantOK, gotOK, "PopFront ok")
		require.Equal(t, want, got, "PopFront value")
	case dequeOpPopBack:
		got, gotOK := d.PopBack()
		want, wantOK := m.popBack()
		require.Equal(t, wantOK, gotOK, "PopBack ok")
		require.Equal(t, want, got, "PopBack value")
	case dequeOpFront:
		if len(m.live) == 0 {
			_, ok := d.Front()
			require.False(t, ok, "Front on empty must miss")
		} else {
			got, ok := d.Front()
			require.True(t, ok)
			require.Equal(t, m.live[0], got, "Front value")
		}
	case dequeOpBack:
		if len(m.live) == 0 {
			_, ok := d.Back()
			require.False(t, ok, "Back on empty must miss")
		} else {
			got, ok := d.Back()
			require.True(t, ok)
			require.Equal(t, m.live[len(m.live)-1], got, "Back value")
		}
	case dequeOpClear:
		d.Clear()
		m.live = nil
	}
}

// runDequeOps replays data as an operation stream against a deque and the
// slice model, checking agreement after every step and comparing the final
// contents.
func runDequeOps(t testing.TB, data []byte) {
	t.Helper()
	if len(data) > 256 {
		data = data[:256] // keep individual runs fast
	}
	d := deque.New[int]()
	m := &sliceModel{}
	for i := 0; i+2 <= len(data); i += 2 {
		applyDequeOp(t, d, m, data[i], int(data[i+1])%16-8)
		require.Equal(t, len(m.live), d.Len(), "after op %d", i/2)
		require.Equal(t, orNil(m.live), orNil(d.ToSlice()), "after op %d", i/2)
	}
}

// TestDequeModelRandomized replays random two-ended traffic against the slice
// reference model.
func TestDequeModelRandomized(t *testing.T) {
	for _, seed := range [][2]uint64{{2026, 1012}, {7, 11}, {99, 4242}} {
		rng := rand.New(rand.NewPCG(seed[0], seed[1]))
		data := make([]byte, 256)
		for i := range data {
			data[i] = byte(rng.IntN(256))
		}
		runDequeOps(t, data)
	}
}

// TestDequeModelScripts replays fixed adversarial scripts against the model.
func TestDequeModelScripts(t *testing.T) {
	scripts := map[string][]byte{
		"grow and shrink": {
			dequeOpPushBack, 1, dequeOpPushBack, 2, dequeOpPushFront, 3,
			dequeOpPushFront, 4, dequeOpPopBack, 0, dequeOpPopFront, 0,
			dequeOpPushBack, 5, dequeOpClear, 0, dequeOpPushFront, 6,
		},
		"pop heavy": {
			dequeOpPopFront, 0, dequeOpPopBack, 0, dequeOpFront, 0,
			dequeOpBack, 0, dequeOpPushBack, 7, dequeOpPopFront, 0,
			dequeOpPopFront, 0,
		},
		"alternating ends": {
			dequeOpPushBack, 1, dequeOpPushFront, 2, dequeOpPopBack, 0,
			dequeOpPushBack, 3, dequeOpPopFront, 0, dequeOpPushFront, 4,
			dequeOpBack, 0, dequeOpFront, 0,
		},
	}
	for name, data := range scripts {
		t.Run(name, func(t *testing.T) {
			runDequeOps(t, data)
		})
	}
}

// TestDequeToSliceMatchesAll is a small consistency check on a big deque.
func TestDequeToSliceMatchesAll(t *testing.T) {
	d := deque.New[int]()
	rng := rand.New(rand.NewPCG(1, 2))
	for range 300 {
		switch rng.IntN(4) {
		case 0:
			d.PushBack(rng.IntN(1000))
		case 1:
			d.PushFront(rng.IntN(1000))
		case 2:
			d.PopFront()
		case 3:
			d.PopBack()
		}
	}

	var viaAll []int
	for v := range d.All() {
		viaAll = append(viaAll, v)
	}
	require.Equal(t, orNil(d.ToSlice()), orNil(viaAll))
	require.Equal(t, orNil(slices.Collect(d.All())), orNil(d.ToSlice()))
}
