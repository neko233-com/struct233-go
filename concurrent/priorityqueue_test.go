package concurrent_test

import (
	"cmp"
	"math/rand"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/concurrent"
)

// pqTask is a prioritized value whose identity (name) is distinct from its
// priority, so FIFO tie-breaks are observable in tests.
type pqTask struct {
	pri  int
	name string
}

func pqCmpPri(a, b pqTask) int { return a.pri - b.pri }

func pqCmpPriDesc(a, b pqTask) int { return b.pri - a.pri }

func pqCmpNameLen(a, b pqTask) int { return len(a.name) - len(b.name) }

func pqCmpZero(a, b pqTask) int { return 0 }

func TestPriorityQueueOrdering(t *testing.T) {
	tests := []struct {
		name   string
		cmp    func(a, b pqTask) int
		pushes []pqTask
		want   []string // names in pop order
	}{
		{
			name:   "min ascending priorities",
			cmp:    pqCmpPri,
			pushes: []pqTask{{2, "b"}, {1, "a"}, {3, "c"}},
			want:   []string{"a", "b", "c"},
		},
		{
			name:   "min descending priorities",
			cmp:    pqCmpPri,
			pushes: []pqTask{{3, "c"}, {2, "b"}, {1, "a"}},
			want:   []string{"a", "b", "c"},
		},
		{
			name:   "max heap via reversed comparator",
			cmp:    pqCmpPriDesc,
			pushes: []pqTask{{2, "b"}, {1, "a"}, {3, "c"}},
			want:   []string{"c", "b", "a"},
		},
		{
			name:   "single element",
			cmp:    pqCmpPri,
			pushes: []pqTask{{7, "solo"}},
			want:   []string{"solo"},
		},
		{
			name:   "all equal keeps FIFO",
			cmp:    pqCmpPri,
			pushes: []pqTask{{1, "e1"}, {1, "e2"}, {1, "e3"}, {1, "e4"}, {1, "e5"}},
			want:   []string{"e1", "e2", "e3", "e4", "e5"},
		},
		{
			name:   "ties at multiple priorities",
			cmp:    pqCmpPri,
			pushes: []pqTask{{2, "b1"}, {1, "a1"}, {2, "b2"}, {1, "a2"}, {2, "b3"}},
			want:   []string{"a1", "a2", "b1", "b2", "b3"},
		},
		{
			name:   "equal tails after smaller late arrival",
			cmp:    pqCmpPri,
			pushes: []pqTask{{5, "e1"}, {5, "e2"}, {1, "x"}, {5, "e3"}},
			want:   []string{"x", "e1", "e2", "e3"},
		},
		{
			name:   "alternating two priorities",
			cmp:    pqCmpPri,
			pushes: []pqTask{{2, "h1"}, {1, "l1"}, {2, "h2"}, {1, "l2"}, {2, "h3"}, {1, "l3"}},
			want:   []string{"l1", "l2", "l3", "h1", "h2", "h3"},
		},
		{
			name:   "constant comparator degenerates to FIFO",
			cmp:    pqCmpZero,
			pushes: []pqTask{{9, "n1"}, {1, "n2"}, {5, "n3"}, {3, "n4"}},
			want:   []string{"n1", "n2", "n3", "n4"},
		},
		{
			name:   "constant comparator long run",
			cmp:    pqCmpZero,
			pushes: []pqTask{{0, "f1"}, {0, "f2"}, {0, "f3"}, {0, "f4"}, {0, "f5"}, {0, "f6"}},
			want:   []string{"f1", "f2", "f3", "f4", "f5", "f6"},
		},
		{
			name:   "name length ordering",
			cmp:    pqCmpNameLen,
			pushes: []pqTask{{0, "abcd"}, {0, "x"}, {0, "mno"}, {0, "yz"}},
			want:   []string{"x", "yz", "mno", "abcd"},
		},
		{
			name:   "name length ties keep FIFO",
			cmp:    pqCmpNameLen,
			pushes: []pqTask{{0, "ab"}, {0, "cd"}, {0, "e"}, {0, "fg"}, {0, "h"}, {0, "ij"}},
			want:   []string{"e", "h", "ab", "cd", "fg", "ij"},
		},
		{
			name:   "shuffled one to ten",
			cmp:    pqCmpPri,
			pushes: []pqTask{{7, "p7"}, {3, "p3"}, {10, "p10"}, {1, "p1"}, {8, "p8"}, {2, "p2"}, {9, "p9"}, {4, "p4"}, {6, "p6"}, {5, "p5"}},
			want:   []string{"p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9", "p10"},
		},
		{
			name:   "reverse sorted input",
			cmp:    pqCmpPri,
			pushes: []pqTask{{5, "p5"}, {4, "p4"}, {3, "p3"}, {2, "p2"}, {1, "p1"}},
			want:   []string{"p1", "p2", "p3", "p4", "p5"},
		},
		{
			name:   "already sorted input",
			cmp:    pqCmpPri,
			pushes: []pqTask{{1, "p1"}, {2, "p2"}, {3, "p3"}, {4, "p4"}, {5, "p5"}},
			want:   []string{"p1", "p2", "p3", "p4", "p5"},
		},
		{
			name:   "zigzag input",
			cmp:    pqCmpPri,
			pushes: []pqTask{{1, "p1"}, {5, "p5"}, {2, "p2"}, {4, "p4"}, {3, "p3"}},
			want:   []string{"p1", "p2", "p3", "p4", "p5"},
		},
		{
			name:   "negative priorities",
			cmp:    pqCmpPri,
			pushes: []pqTask{{-1, "n-1"}, {-5, "n-5"}, {0, "n0"}, {-3, "n-3"}},
			want:   []string{"n-5", "n-3", "n-1", "n0"},
		},
		{
			name:   "negatives with ties",
			cmp:    pqCmpPri,
			pushes: []pqTask{{-2, "t1"}, {-2, "t2"}, {-1, "s"}, {-2, "t3"}},
			want:   []string{"t1", "t2", "t3", "s"},
		},
		{
			name:   "reversed comparator with ties",
			cmp:    pqCmpPriDesc,
			pushes: []pqTask{{1, "a1"}, {3, "c1"}, {1, "a2"}, {3, "c2"}, {2, "b"}},
			want:   []string{"c1", "c2", "b", "a1", "a2"},
		},
		{
			name:   "interleaved equals with distinct center",
			cmp:    pqCmpPri,
			pushes: []pqTask{{4, "d1"}, {4, "d2"}, {2, "b"}, {4, "d3"}, {2, "b2"}, {4, "d4"}},
			want:   []string{"b", "b2", "d1", "d2", "d3", "d4"},
		},
		{
			name:   "all same priority long FIFO run",
			cmp:    pqCmpPri,
			pushes: []pqTask{{3, "s1"}, {3, "s2"}, {3, "s3"}, {3, "s4"}, {3, "s5"}, {3, "s6"}, {3, "s7"}, {3, "s8"}},
			want:   []string{"s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8"},
		},
		{
			name:   "two groups reversed insertion",
			cmp:    pqCmpPri,
			pushes: []pqTask{{2, "g2a"}, {2, "g2b"}, {1, "g1a"}, {1, "g1b"}},
			want:   []string{"g1a", "g1b", "g2a", "g2b"},
		},
		{
			name:   "max heap full tie run",
			cmp:    pqCmpPriDesc,
			pushes: []pqTask{{1, "m1"}, {1, "m2"}, {1, "m3"}, {1, "m4"}},
			want:   []string{"m1", "m2", "m3", "m4"},
		},
		{
			name:   "sparse priorities",
			cmp:    pqCmpPri,
			pushes: []pqTask{{100, "hi"}, {-100, "lo"}, {0, "mid"}, {-100, "lo2"}},
			want:   []string{"lo", "lo2", "mid", "hi"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := concurrent.NewPriorityQueue[pqTask](tt.cmp)
			for _, task := range tt.pushes {
				require.True(t, q.TryPut(task))
			}
			require.Equal(t, len(tt.pushes), q.Len())

			wantTasks := make(map[string]pqTask, len(tt.pushes))
			for _, task := range tt.pushes {
				wantTasks[task.name] = task
			}
			for i, wantName := range tt.want {
				head, ok := q.Peek()
				require.True(t, ok, "Peek must see element %d", i)
				require.Equal(t, wantName, head.name, "Peek must expose the next element %d", i)
				require.Equal(t, len(tt.pushes)-i, q.Len())

				got, ok := q.Take()
				require.True(t, ok)
				require.Equal(t, wantName, got.name, "pop order must follow (cmp, seq)")
				require.Equal(t, wantTasks[wantName], got, "elements must survive unchanged")
			}

			_, ok := q.TryTake()
			require.False(t, ok, "queue must be empty after draining")
			_, ok = q.Peek()
			require.False(t, ok)
			require.Zero(t, q.Len())
		})
	}
}

// pqRefModel is the reference model: a slice kept sorted by (cmp, seq), i.e.
// stable sort by comparator with insertion-order tie-break.
type pqRefModel struct {
	cmp      func(a, b int) int
	capacity int // 0 means unbounded
	seq      uint64
	items    []pqRefEntry
}

type pqRefEntry struct {
	v   int
	seq uint64
}

func newPQRefModel(cmp func(a, b int) int, capacity int) *pqRefModel {
	return &pqRefModel{cmp: cmp, capacity: capacity}
}

func (m *pqRefModel) put(v int) bool {
	if m.capacity > 0 && len(m.items) == m.capacity {
		return false
	}
	m.seq++
	m.items = append(m.items, pqRefEntry{v: v, seq: m.seq})
	slices.SortFunc(m.items, func(a, b pqRefEntry) int {
		if c := m.cmp(a.v, b.v); c != 0 {
			return c
		}
		return cmp.Compare(a.seq, b.seq)
	})
	return true
}

func (m *pqRefModel) pop() (int, bool) {
	if len(m.items) == 0 {
		return 0, false
	}
	v := m.items[0].v
	m.items = m.items[1:]
	return v, true
}

func (m *pqRefModel) peek() (int, bool) {
	if len(m.items) == 0 {
		return 0, false
	}
	return m.items[0].v, true
}

func (m *pqRefModel) len() int { return len(m.items) }

func pqIntCmps() []func(a, b int) int {
	return []func(a, b int) int{
		func(a, b int) int { return a - b },
		func(a, b int) int { return b - a },
		func(a, b int) int {
			aa, bb := abs(a), abs(b)
			if aa != bb {
				return aa - bb
			}
			return a - b
		},
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func TestPriorityQueueAgainstSliceModel(t *testing.T) {
	for seed := int64(0); seed < 20; seed++ {
		t.Run("seed "+itoa(int(seed)), func(t *testing.T) {
			r := rand.New(rand.NewSource(seed))
			cmpFn := pqIntCmps()[seed%3]
			capacity := 0
			if seed%2 == 1 {
				capacity = 8
			}

			var q *concurrent.PriorityQueue[int]
			if capacity == 0 {
				q = concurrent.NewPriorityQueue[int](cmpFn)
			} else {
				q = concurrent.NewPriorityQueueCapacity[int](cmpFn, capacity)
			}
			ref := newPQRefModel(cmpFn, capacity)

			for i := 0; i < 400; i++ {
				v := r.Intn(40) - 20
				switch r.Intn(6) {
				case 0, 1: // TryPut
					got := q.TryPut(v)
					want := ref.put(v)
					require.Equal(t, want, got, "seed %d op %d: TryPut(%d)", seed, i, v)
				case 2: // TryTake
					gv, gok := q.TryTake()
					wv, wok := ref.pop()
					require.Equal(t, wok, gok, "seed %d op %d: TryTake hit", seed, i)
					if wok {
						require.Equal(t, wv, gv, "seed %d op %d: TryTake value", seed, i)
					}
				case 3: // Poll(0) behaves like TryTake
					gv, gok := q.Poll(0)
					wv, wok := ref.pop()
					require.Equal(t, wok, gok, "seed %d op %d: Poll hit", seed, i)
					if wok {
						require.Equal(t, wv, gv, "seed %d op %d: Poll value", seed, i)
					}
				case 4: // Peek
					gv, gok := q.Peek()
					wv, wok := ref.peek()
					require.Equal(t, wok, gok, "seed %d op %d: Peek hit", seed, i)
					if wok {
						require.Equal(t, wv, gv, "seed %d op %d: Peek value", seed, i)
					}
				case 5: // Offer with zero timeout behaves like TryPut
					got := q.Offer(v, 0)
					want := ref.put(v)
					require.Equal(t, want, got, "seed %d op %d: Offer(%d)", seed, i, v)
				}
				require.Equal(t, ref.len(), q.Len(), "seed %d op %d: Len", seed, i)
			}

			// final drain must agree element by element
			for i := 0; ; i++ {
				gv, gok := q.TryTake()
				wv, wok := ref.pop()
				require.Equal(t, wok, gok, "seed %d drain %d: hit", seed, i)
				if !gok {
					break
				}
				require.Equal(t, wv, gv, "seed %d drain %d: value", seed, i)
			}
		})
	}
}

func TestPriorityQueueStableOrderDifferential(t *testing.T) {
	for seed := int64(0); seed < 8; seed++ {
		t.Run("seed "+itoa(int(seed)), func(t *testing.T) {
			r := rand.New(rand.NewSource(1000 + seed))
			cmpFn := pqIntCmps()[seed%3]

			q := concurrent.NewPriorityQueue[int](cmpFn)
			pushes := make([]int, 64)
			for i := range pushes {
				pushes[i] = r.Intn(16) - 8 // heavy duplicates stress the tie-break
				require.True(t, q.TryPut(pushes[i]))
			}

			want := slices.Clone(pushes)
			slices.SortStableFunc(want, cmpFn) // stable sort = (cmp, seq) order

			got := make([]int, 0, len(pushes))
			for range pushes {
				v, ok := q.TryTake()
				require.True(t, ok)
				got = append(got, v)
			}
			require.Equal(t, want, got, "pop order must match stable sort by comparator")
		})
	}
}

func TestPriorityQueueFIFOTieBreak(t *testing.T) {
	q := concurrent.NewPriorityQueue[pqTask](pqCmpPri)

	// push equal priorities in a known order, interleaved with other priorities
	require.True(t, q.Put(pqTask{2, "tie1"}))
	require.True(t, q.Put(pqTask{1, "other1"}))
	require.True(t, q.Put(pqTask{2, "tie2"}))
	require.True(t, q.Put(pqTask{2, "tie3"}))
	require.True(t, q.Put(pqTask{1, "other2"}))
	require.True(t, q.Put(pqTask{2, "tie4"}))

	want := []string{"other1", "other2", "tie1", "tie2", "tie3", "tie4"}
	for _, name := range want {
		v, ok := q.Take()
		require.True(t, ok)
		require.Equal(t, name, v.name, "equal elements must pop in insertion order")
	}

	// seq keeps growing across drains: a new tie set must not reuse old order
	require.True(t, q.TryPut(pqTask{5, "late1"}))
	require.True(t, q.TryPut(pqTask{5, "late2"}))
	v, ok := q.Take()
	require.True(t, ok)
	require.Equal(t, "late1", v.name)
	v, ok = q.Take()
	require.True(t, ok)
	require.Equal(t, "late2", v.name)
}

func TestPriorityQueueUnboundedPutNeverBlocks(t *testing.T) {
	q := concurrent.NewPriorityQueue[int](func(a, b int) int { return a - b })

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 10_000; i++ {
			q.Put(i)
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Put on an unbounded queue must never block")
	}
	require.Equal(t, 10_000, q.Len())
}

func TestPriorityQueueBoundedPutBlocksUntilSpace(t *testing.T) {
	q := concurrent.NewPriorityQueueCapacity[int](func(a, b int) int { return a - b }, 2)
	require.True(t, q.TryPut(1))
	require.True(t, q.TryPut(2))
	require.False(t, q.TryPut(3), "TryPut must fail on a full bounded queue")

	putDone := make(chan bool, 1)
	go func() { putDone <- q.Put(3) }()
	time.Sleep(20 * time.Millisecond) // let the producer block on the full queue

	v, ok := q.Take()
	require.True(t, ok)
	require.Equal(t, 1, v)

	select {
	case enqueued := <-putDone:
		require.True(t, enqueued, "Put must succeed once space appears")
	case <-time.After(time.Second):
		t.Fatal("Put was not unblocked by Take")
	}

	require.Equal(t, 2, q.Len())
	v, ok = q.Take()
	require.True(t, ok)
	require.Equal(t, 2, v)
	v, ok = q.Take()
	require.True(t, ok)
	require.Equal(t, 3, v)
}

func TestPriorityQueueOfferTimeoutAndSuccess(t *testing.T) {
	q := concurrent.NewPriorityQueueCapacity[pqTask](pqCmpPri, 1)
	require.True(t, q.TryPut(pqTask{1, "held"}))

	require.False(t, q.Offer(pqTask{2, "x"}, 0), "Offer without timeout must fail on a full queue")
	require.False(t, q.Offer(pqTask{2, "x"}, 20*time.Millisecond), "Offer must time out on a full queue")

	offerDone := make(chan bool, 1)
	go func() { offerDone <- q.Offer(pqTask{9, "late"}, time.Second) }()

	time.Sleep(20 * time.Millisecond)
	v, ok := q.Take()
	require.True(t, ok)
	require.Equal(t, "held", v.name)

	select {
	case enqueued := <-offerDone:
		require.True(t, enqueued, "Offer must succeed once space appears")
	case <-time.After(time.Second):
		t.Fatal("Offer was not unblocked by Take")
	}

	v, ok = q.Take()
	require.True(t, ok)
	require.Equal(t, "late", v.name, "the higher priority element must pop first when both fit")
}

func TestPriorityQueueOfferOnUnboundedIgnoresTimeout(t *testing.T) {
	q := concurrent.NewPriorityQueue[int](func(a, b int) int { return a - b })
	require.True(t, q.Offer(1, 0))
	require.True(t, q.Offer(2, 20*time.Millisecond), "an unbounded queue always has room")
	require.Equal(t, 2, q.Len())
}

func TestPriorityQueueTakeBlocksUntilPut(t *testing.T) {
	q := concurrent.NewPriorityQueue[int](func(a, b int) int { return a - b })

	done := make(chan int, 1)
	go func() {
		v, ok := q.Take()
		if !ok {
			done <- -1
			return
		}
		done <- v
	}()

	time.Sleep(20 * time.Millisecond) // let the consumer block
	require.True(t, q.Put(7))

	select {
	case v := <-done:
		require.Equal(t, 7, v)
	case <-time.After(time.Second):
		t.Fatal("Take was not unblocked by Put")
	}
}

func TestPriorityQueuePollTimeoutAndSuccess(t *testing.T) {
	q := concurrent.NewPriorityQueue[int](func(a, b int) int { return a - b })

	v, ok := q.Poll(10 * time.Millisecond)
	require.False(t, ok, "Poll must time out on an empty queue")

	require.True(t, q.TryPut(5))
	v, ok = q.Poll(50 * time.Millisecond)
	require.True(t, ok)
	require.Equal(t, 5, v)

	require.True(t, q.TryPut(6))
	v, ok = q.Poll(0)
	require.True(t, ok, "Poll with zero timeout must still take a queued element")
	require.Equal(t, 6, v)

	v, ok = q.Poll(0)
	require.False(t, ok)
}

func TestPriorityQueuePollWakesOnPut(t *testing.T) {
	q := concurrent.NewPriorityQueue[int](func(a, b int) int { return a - b })

	done := make(chan int, 1)
	go func() {
		v, ok := q.Poll(time.Second)
		if !ok {
			done <- -1
			return
		}
		done <- v
	}()

	time.Sleep(20 * time.Millisecond) // let the consumer block
	require.True(t, q.TryPut(42))

	select {
	case v := <-done:
		require.Equal(t, 42, v)
	case <-time.After(time.Second):
		t.Fatal("Poll was not unblocked by Put")
	}
}

func TestPriorityQueuePeekAndLen(t *testing.T) {
	q := concurrent.NewPriorityQueueCapacity[pqTask](pqCmpPri, 4)

	_, ok := q.Peek()
	require.False(t, ok, "Peek on empty queue must miss")
	require.Zero(t, q.Len())

	require.True(t, q.TryPut(pqTask{3, "c"}))
	require.True(t, q.TryPut(pqTask{1, "a"}))
	require.True(t, q.TryPut(pqTask{2, "b"}))

	v, ok := q.Peek()
	require.True(t, ok)
	require.Equal(t, "a", v.name, "Peek must expose the smallest element")
	require.Equal(t, 3, q.Len(), "Peek must not remove the element")

	v, ok = q.Peek()
	require.True(t, ok)
	require.Equal(t, "a", v.name, "Peek must be repeatable")
}

func TestPriorityQueueCloseWakesBlockedTakers(t *testing.T) {
	q := concurrent.NewPriorityQueue[int](func(a, b int) int { return a - b })

	const waiters = 3
	takeDone := make(chan bool, waiters)
	for i := 0; i < waiters-1; i++ {
		go func() {
			_, ok := q.Take()
			takeDone <- ok
		}()
	}
	go func() {
		_, ok := q.Poll(time.Second)
		takeDone <- ok
	}()
	time.Sleep(20 * time.Millisecond) // let the consumers block on the empty queue

	q.Close()

	for i := 0; i < waiters; i++ {
		select {
		case ok := <-takeDone:
			require.False(t, ok, "blocked takers must report false once closed and drained")
		case <-time.After(time.Second):
			t.Fatal("Close must unblock every waiting taker")
		}
	}
}

func TestPriorityQueueCloseWakesBlockedPutters(t *testing.T) {
	q := concurrent.NewPriorityQueueCapacity[int](func(a, b int) int { return a - b }, 1)
	require.True(t, q.TryPut(1))
	require.False(t, q.TryPut(2), "queue is full")

	putDone := make(chan bool, 2)
	go func() { putDone <- q.Put(2) }()
	go func() { putDone <- q.Offer(3, time.Second) }()
	time.Sleep(20 * time.Millisecond) // let the producers block on the full queue

	q.Close()

	for i := 0; i < 2; i++ {
		select {
		case ok := <-putDone:
			require.False(t, ok, "blocked producers must report false once closed")
		case <-time.After(time.Second):
			t.Fatal("Close must unblock every waiting producer")
		}
	}

	require.False(t, q.Put(4))
	require.False(t, q.TryPut(4))
	require.False(t, q.Offer(4, 0))
	require.False(t, q.Offer(4, 10*time.Millisecond))
}

func TestPriorityQueueDrainAfterClose(t *testing.T) {
	q := concurrent.NewPriorityQueue[int](func(a, b int) int { return a - b })
	require.True(t, q.TryPut(2))
	require.True(t, q.TryPut(1))

	q.Close()

	v, ok := q.Take()
	require.True(t, ok)
	require.Equal(t, 1, v, "queued elements must stay drainable after Close")

	v, ok = q.Take()
	require.True(t, ok)
	require.Equal(t, 2, v)

	_, ok = q.Take()
	require.False(t, ok, "Take must miss once closed and drained")

	_, ok = q.Poll(10 * time.Millisecond)
	require.False(t, ok)
	_, ok = q.TryTake()
	require.False(t, ok)
}

func TestPriorityQueuePutFailsAfterClose(t *testing.T) {
	q := concurrent.NewPriorityQueueCapacity[int](func(a, b int) int { return a - b }, 4)
	q.Close()

	require.False(t, q.Put(1))
	require.False(t, q.TryPut(1))
	require.False(t, q.Offer(1, 0))
	require.False(t, q.Offer(1, 10*time.Millisecond))
	require.Zero(t, q.Len())
}

func TestPriorityQueueConcurrentProducersConsumers(t *testing.T) {
	const (
		producers = 4
		consumers = 4
		perWorker = 250
	)
	// each producer pushes its own strictly increasing range; value-to-insert
	// order is nondeterministic across producers, so the assertions below check
	// exactly-once delivery rather than global monotonicity
	q := concurrent.NewPriorityQueue[int](func(a, b int) int { return a - b })

	var (
		pwg sync.WaitGroup
		cwg sync.WaitGroup
	)
	for p := range producers {
		pwg.Add(1)
		go func() {
			defer pwg.Done()
			base := p * perWorker
			for i := range perWorker {
				q.Put(base + i)
			}
		}()
	}

	records := make([][]int, consumers)
	for c := range consumers {
		cwg.Add(1)
		go func() {
			defer cwg.Done()
			var rec []int
			for {
				v, ok := q.Take()
				if !ok {
					break
				}
				rec = append(rec, v)
			}
			records[c] = rec
		}()
	}

	producersDone := make(chan struct{})
	go func() { pwg.Wait(); close(producersDone) }()
	select {
	case <-producersDone:
	case <-time.After(10 * time.Second):
		t.Fatal("producers did not finish")
	}
	q.Close()

	consumersDone := make(chan struct{})
	go func() { cwg.Wait(); close(consumersDone) }()
	select {
	case <-consumersDone:
	case <-time.After(10 * time.Second):
		t.Fatal("consumers did not drain after Close")
	}

	// multi-producer values are assigned before the lock is won, so insert
	// order (heap seq) is not ticket order; only these invariants hold:
	// each consumer sees no duplicates, and the union is exactly 0..N-1
	seen := make([]int, 0, producers*perWorker)
	for c, rec := range records {
		uniq := make(map[int]struct{}, len(rec))
		for _, v := range rec {
			_, dup := uniq[v]
			require.False(t, dup, "consumer %d saw value %d twice", c, v)
			uniq[v] = struct{}{}
		}
		seen = append(seen, rec...)
	}
	require.Equal(t, producers*perWorker, len(seen), "every produced element must be consumed exactly once")

	slices.Sort(seen)
	want := make([]int, producers*perWorker)
	for i := range want {
		want[i] = i
	}
	require.Equal(t, want, seen, "consumers together must see each produced value exactly once")
}

func TestPriorityQueueConcurrentBoundedRoundTrip(t *testing.T) {
	const total = 2000
	q := concurrent.NewPriorityQueueCapacity[int](func(a, b int) int { return a - b }, 16)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range total {
			q.Put(i)
		}
	}()

	got := make([]int, 0, total)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range total {
			v, ok := q.Take()
			require.True(t, ok)
			got = append(got, v)
		}
	}()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("bounded round trip did not finish")
	}

	require.True(t, slices.IsSorted(got), "a single consumer must see non-decreasing priorities")
	require.Equal(t, total, len(got))
}

func TestPriorityQueuePeekLenZeroAlloc(t *testing.T) {
	q := concurrent.NewPriorityQueue[int](func(a, b int) int { return a - b })
	for i := range 128 {
		q.Put(i)
	}

	allocs := testing.AllocsPerRun(1000, func() {
		_, _ = q.Peek()
		_ = q.Len()
	})
	require.Zero(t, allocs, "Peek and Len must not allocate")
}

func TestPriorityQueuePanicsOnBadArgs(t *testing.T) {
	cmpFn := func(a, b int) int { return a - b }

	require.PanicsWithValue(t, "concurrent: nil comparator", func() {
		concurrent.NewPriorityQueue[int](nil)
	})
	require.PanicsWithValue(t, "concurrent: nil comparator", func() {
		concurrent.NewPriorityQueueCapacity[int](nil, 4)
	})
	require.PanicsWithValue(t, "concurrent: priority queue capacity must be positive", func() {
		concurrent.NewPriorityQueueCapacity[int](cmpFn, 0)
	})
	require.PanicsWithValue(t, "concurrent: priority queue capacity must be positive", func() {
		concurrent.NewPriorityQueueCapacity[int](cmpFn, -1)
	})
}

func FuzzPriorityQueueAgainstModel(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2, 3, 0, 1, 2, 3})
	f.Add([]byte{0, 0, 0, 1, 1, 2, 3, 2, 0, 1})
	f.Add([]byte{255, 0, 128, 1, 2, 3, 4, 5, 6, 7})

	f.Fuzz(func(t *testing.T, ops []byte) {
		cmpFn := func(a, b int) int { return a - b }
		q := concurrent.NewPriorityQueue[int](cmpFn)
		ref := newPQRefModel(cmpFn, 0)

		for i, b := range ops {
			v := int(b) - 128
			switch b % 5 {
			case 0, 1: // TryPut
				got := q.TryPut(v)
				want := ref.put(v)
				require.Equal(t, want, got, "op %d: TryPut(%d)", i, v)
			case 2: // TryTake
				gv, gok := q.TryTake()
				wv, wok := ref.pop()
				require.Equal(t, wok, gok, "op %d: TryTake hit", i)
				if wok {
					require.Equal(t, wv, gv, "op %d: TryTake value", i)
				}
			case 3: // Peek
				gv, gok := q.Peek()
				wv, wok := ref.peek()
				require.Equal(t, wok, gok, "op %d: Peek hit", i)
				if wok {
					require.Equal(t, wv, gv, "op %d: Peek value", i)
				}
			case 4: // Len
				require.Equal(t, ref.len(), q.Len(), "op %d: Len", i)
			}
		}

		for i := 0; ; i++ {
			gv, gok := q.TryTake()
			wv, wok := ref.pop()
			require.Equal(t, wok, gok, "drain %d: hit", i)
			if !gok {
				break
			}
			require.Equal(t, wv, gv, "drain %d: value", i)
		}
	})
}

// itoa avoids importing strconv just for subtest names.
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
