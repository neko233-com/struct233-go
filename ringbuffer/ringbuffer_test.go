package ringbuffer_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/ringbuffer"
)

func TestPushThenEvictOldest(t *testing.T) {
	r := ringbuffer.New[int](3)
	require.Equal(t, 3, r.Cap())
	require.Zero(t, r.Len())

	for i := 1; i <= 3; i++ {
		evicted, ok := r.Push(i)
		require.False(t, ok, "push #%d must not evict", i)
		require.Zero(t, evicted)
	}
	require.Equal(t, []int{1, 2, 3}, r.ToSlice())

	evicted, ok := r.Push(4)
	require.True(t, ok, "push into a full buffer must evict")
	require.Equal(t, 1, evicted)
	require.Equal(t, []int{2, 3, 4}, r.ToSlice())
	require.Equal(t, 3, r.Len())

	for i := 5; i <= 100; i++ {
		r.Push(i)
	}
	require.Equal(t, []int{98, 99, 100}, r.ToSlice(), "only the newest Cap elements survive")
	require.Equal(t, 3, r.Len())
}

func TestPopAndPeek(t *testing.T) {
	r := ringbuffer.New[int](3)

	_, ok := r.Pop()
	require.False(t, ok, "Pop on empty buffer must miss")
	_, ok = r.Peek()
	require.False(t, ok, "Peek on empty buffer must miss")

	r.Push(1)
	r.Push(2)

	v, ok := r.Peek()
	require.True(t, ok)
	require.Equal(t, 1, v, "Peek returns the oldest")

	v, ok = r.Pop()
	require.True(t, ok)
	require.Equal(t, 1, v)

	r.Push(3)
	require.Equal(t, []int{2, 3}, r.ToSlice())
}

func TestAt(t *testing.T) {
	r := ringbuffer.New[int](3)
	for _, v := range []int{10, 20, 30} {
		r.Push(v)
	}
	r.Push(40) // 10 evicted

	for i, want := range []int{20, 30, 40} {
		v, ok := r.At(i)
		require.True(t, ok)
		require.Equal(t, want, v, "At(%d)", i)
	}

	_, ok := r.At(3)
	require.False(t, ok, "At beyond Len must miss")
	_, ok = r.At(-1)
	require.False(t, ok, "negative index must miss")
}

func TestAll(t *testing.T) {
	r := ringbuffer.New[int](3)
	r.Push(1)
	r.Push(2)
	r.Push(3)
	r.Push(4) // 1 evicted

	var got []int
	for v := range r.All() {
		got = append(got, v)
	}
	require.Equal(t, []int{2, 3, 4}, got)

	count := 0
	for range r.All() {
		count++
		break
	}
	require.Equal(t, 1, count, "break must stop the iteration")
}

func TestClearAndReuse(t *testing.T) {
	r := ringbuffer.New[int](4)
	r.Push(1)
	r.Push(2)

	r.Clear()
	require.Zero(t, r.Len())
	require.Equal(t, 4, r.Cap(), "Clear must keep the capacity")
	require.Empty(t, r.ToSlice())

	evicted, ok := r.Push(9)
	require.False(t, ok)
	require.Zero(t, evicted)
	require.Equal(t, []int{9}, r.ToSlice())
}

func TestWraparound(t *testing.T) {
	r := ringbuffer.New[int](4)
	// drive the head around the end of the backing array several times
	for i := range 10 {
		r.Push(i)
	}
	require.Equal(t, []int{6, 7, 8, 9}, r.ToSlice())

	require.Equal(t, 6, mustPop(t, r))
	require.Equal(t, 7, mustPop(t, r))
	r.Push(10)
	r.Push(11)
	require.Equal(t, []int{8, 9, 10, 11}, r.ToSlice())
}

func mustPop(t *testing.T, r *ringbuffer.RingBuffer[int]) int {
	t.Helper()
	v, ok := r.Pop()
	require.True(t, ok)
	return v
}

func TestPanicsOnBadCapacity(t *testing.T) {
	require.PanicsWithValue(t, "ringbuffer: capacity must be positive", func() { ringbuffer.New[int](0) })
	require.PanicsWithValue(t, "ringbuffer: capacity must be positive", func() { ringbuffer.New[int](-3) })
}

func TestPushPopDoesNotAllocate(t *testing.T) {
	r := ringbuffer.New[int](64)
	r.Push(1) // reach steady state

	allocs := testing.AllocsPerRun(100, func() {
		r.Push(2)
		r.Pop()
	})
	require.Zero(t, allocs, "Push+Pop on a full-capacity window must not allocate")
}

func TestZeroValueIsTracked(t *testing.T) {
	r := ringbuffer.New[int](2)
	r.Push(0)
	r.Push(0)

	v, ok := r.Peek()
	require.True(t, ok)
	require.Zero(t, v)
	require.Equal(t, 2, r.Len(), "zero values must still count as elements")
}

// TestRingBuffer_TableDriven covers every exported method through normal,
// boundary and empty/full cases in one table.
func TestRingBuffer_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"new capacity one", func(t *testing.T) {
			r := ringbuffer.New[int](1)
			require.Equal(t, 1, r.Cap())
			require.Zero(t, r.Len())
		}},
		{"new large capacity", func(t *testing.T) {
			r := ringbuffer.New[int](1024)
			require.Equal(t, 1024, r.Cap())
		}},
		{"cap is stable after pushes", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			for i := range 10 {
				r.Push(i)
			}
			require.Equal(t, 3, r.Cap())
		}},
		{"len tracks pushes and pops", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			r.Push(1)
			require.Equal(t, 1, r.Len())
			r.Push(2)
			r.Push(3)
			require.Equal(t, 3, r.Len())
			r.Pop()
			require.Equal(t, 2, r.Len())
		}},
		{"len saturates at capacity", func(t *testing.T) {
			r := ringbuffer.New[int](2)
			r.Push(1)
			r.Push(2)
			r.Push(3)
			require.Equal(t, 2, r.Len())
		}},
		{"push into non-full buffer does not evict", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			evicted, ok := r.Push(5)
			require.False(t, ok)
			require.Zero(t, evicted)
		}},
		{"push into full buffer evicts oldest", func(t *testing.T) {
			r := ringbuffer.New[int](2)
			r.Push(1)
			r.Push(2)
			evicted, ok := r.Push(3)
			require.True(t, ok)
			require.Equal(t, 1, evicted)
		}},
		{"push zero value into full buffer reports zero eviction", func(t *testing.T) {
			r := ringbuffer.New[int](1)
			r.Push(0)
			evicted, ok := r.Push(7)
			require.True(t, ok)
			require.Zero(t, evicted, "evicted zero value is still reported")
		}},
		{"push of negative values", func(t *testing.T) {
			r := ringbuffer.New[int](2)
			r.Push(-1)
			r.Push(-2)
			require.Equal(t, []int{-1, -2}, r.ToSlice())
		}},
		{"pop on empty misses", func(t *testing.T) {
			r := ringbuffer.New[int](2)
			v, ok := r.Pop()
			require.False(t, ok)
			require.Zero(t, v)
		}},
		{"pop drains in FIFO order", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			r.Push(1)
			r.Push(2)
			v, ok := r.Pop()
			require.True(t, ok)
			require.Equal(t, 1, v)
			v, ok = r.Pop()
			require.True(t, ok)
			require.Equal(t, 2, v)
			_, ok = r.Pop()
			require.False(t, ok)
		}},
		{"pop then push reuses the slot", func(t *testing.T) {
			r := ringbuffer.New[int](2)
			r.Push(1)
			r.Push(2)
			r.Pop()
			evicted, ok := r.Push(3)
			require.False(t, ok, "there is a free slot after Pop")
			require.Zero(t, evicted)
			require.Equal(t, []int{2, 3}, r.ToSlice())
		}},
		{"peek on empty misses", func(t *testing.T) {
			r := ringbuffer.New[int](2)
			v, ok := r.Peek()
			require.False(t, ok)
			require.Zero(t, v)
		}},
		{"peek does not remove", func(t *testing.T) {
			r := ringbuffer.New[int](2)
			r.Push(1)
			v, ok := r.Peek()
			require.True(t, ok)
			require.Equal(t, 1, v)
			require.Equal(t, 1, r.Len(), "Peek must not consume")
		}},
		{"peek sees the evicted-out head", func(t *testing.T) {
			r := ringbuffer.New[int](2)
			r.Push(1)
			r.Push(2)
			r.Push(3) // 1 evicted
			v, ok := r.Peek()
			require.True(t, ok)
			require.Equal(t, 2, v)
		}},
		{"at zero is the oldest", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			r.Push(9)
			r.Push(8)
			v, ok := r.At(0)
			require.True(t, ok)
			require.Equal(t, 9, v)
		}},
		{"at len-1 is the newest", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			r.Push(9)
			r.Push(8)
			v, ok := r.At(1)
			require.True(t, ok)
			require.Equal(t, 8, v)
		}},
		{"at len misses", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			r.Push(1)
			_, ok := r.At(1)
			require.False(t, ok)
		}},
		{"at negative misses", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			r.Push(1)
			_, ok := r.At(-1)
			require.False(t, ok)
		}},
		{"at negative on empty misses", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			_, ok := r.At(-5)
			require.False(t, ok)
		}},
		{"at beyond capacity misses after wraparound", func(t *testing.T) {
			r := ringbuffer.New[int](2)
			for i := range 7 {
				r.Push(i)
			}
			_, ok := r.At(2)
			require.False(t, ok)
			v, ok := r.At(1)
			require.True(t, ok)
			require.Equal(t, 6, v)
		}},
		{"at on empty buffer misses", func(t *testing.T) {
			r := ringbuffer.New[int](2)
			_, ok := r.At(0)
			require.False(t, ok)
		}},
		{"clear on empty buffer", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			r.Clear()
			require.Zero(t, r.Len())
			require.Equal(t, 3, r.Cap())
		}},
		{"clear resets wraparound state", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			for i := range 8 {
				r.Push(i)
			}
			r.Clear()
			require.Zero(t, r.Len())
			r.Push(42)
			require.Equal(t, []int{42}, r.ToSlice())
		}},
		{"to slice on empty buffer", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			require.Empty(t, r.ToSlice())
		}},
		{"to slice copies", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			r.Push(1)
			s := r.ToSlice()
			s[0] = 99
			v, _ := r.At(0)
			require.Equal(t, 1, v, "ToSlice must hand out a copy")
		}},
		{"all on empty buffer yields nothing", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			count := 0
			for range r.All() {
				count++
			}
			require.Zero(t, count)
		}},
		{"all walks oldest to newest", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			r.Push(3)
			r.Push(1)
			r.Push(2)
			var got []int
			for v := range r.All() {
				got = append(got, v)
			}
			require.Equal(t, []int{3, 1, 2}, got)
		}},
		{"all supports early break after wraparound", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			for i := range 7 {
				r.Push(i)
			}
			count := 0
			for range r.All() {
				count++
				if count == 1 {
					break
				}
			}
			require.Equal(t, 1, count)
		}},
		{"wraparound head index at exact capacity", func(t *testing.T) {
			r := ringbuffer.New[int](4)
			for i := range 4 {
				r.Push(i)
			}
			r.Pop()
			r.Pop()
			r.Push(4)
			r.Push(5) // head wrapped to 0
			require.Equal(t, []int{2, 3, 4, 5}, r.ToSlice())
			require.Equal(t, 2, mustPop(t, r))
		}},
		{"interleaved push pop keeps order", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			r.Push(1)
			r.Push(2)
			require.Equal(t, 1, mustPop(t, r))
			r.Push(3)
			r.Push(4)
			require.Equal(t, 2, mustPop(t, r))
			require.Equal(t, []int{3, 4}, r.ToSlice())
		}},
		{"capacity one overwrite loop", func(t *testing.T) {
			r := ringbuffer.New[int](1)
			for i := range 5 {
				evicted, ok := r.Push(i)
				require.Equal(t, i > 0, ok)
				if ok {
					require.Equal(t, i-1, evicted)
				}
			}
			require.Equal(t, []int{4}, r.ToSlice())
		}},
		{"string elements", func(t *testing.T) {
			r := ringbuffer.New[string](2)
			r.Push("a")
			r.Push("b")
			r.Push("c")
			require.Equal(t, []string{"b", "c"}, r.ToSlice())
		}},
		{"empty string is a real element", func(t *testing.T) {
			r := ringbuffer.New[string](2)
			r.Push("")
			v, ok := r.Peek()
			require.True(t, ok)
			require.Empty(t, v)
			require.Equal(t, 1, r.Len())
		}},
		{"at with wraparound index mapping", func(t *testing.T) {
			r := ringbuffer.New[int](4)
			for i := range 6 {
				r.Push(i * 10)
			}
			// head is at index 2; logical 0..3 -> 20 30 40 50
			for i, want := range []int{20, 30, 40, 50} {
				v, ok := r.At(i)
				require.True(t, ok)
				require.Equal(t, want, v, "At(%d)", i)
			}
		}},
		{"many overwrites only keep last cap", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			for i := range 1000 {
				r.Push(i)
			}
			require.Equal(t, []int{997, 998, 999}, r.ToSlice())
		}},
		{"peek after full drain misses", func(t *testing.T) {
			r := ringbuffer.New[int](2)
			r.Push(1)
			r.Push(2)
			r.Pop()
			r.Pop()
			_, ok := r.Peek()
			require.False(t, ok)
			require.Zero(t, r.Len())
		}},
		{"evicted value equals the oldest in ToSlice", func(t *testing.T) {
			r := ringbuffer.New[int](3)
			for i := 1; i <= 3; i++ {
				r.Push(i)
			}
			oldest := r.ToSlice()[0]
			evicted, ok := r.Push(4)
			require.True(t, ok)
			require.Equal(t, oldest, evicted)
		}},
		{"len cap agree across drift", func(t *testing.T) {
			r := ringbuffer.New[int](5)
			for i := range 37 {
				r.Push(i)
				if i%3 == 0 {
					r.Pop()
				}
			}
			require.LessOrEqual(t, r.Len(), r.Cap())
			require.Equal(t, r.Len(), len(r.ToSlice()))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// TestRingBufferAgainstSliceModel is a differential test: random Push/Pop/Peek
// traffic replayed against a plain slice queue with the same overwrite-oldest
// rule, exercising wraparound repeatedly.
func TestRingBufferAgainstSliceModel(t *testing.T) {
	for _, seed := range [][2]uint64{{2026, 1012}, {7, 11}, {99, 4242}} {
		for _, capacity := range []int{1, 2, 3, 5} {
			t.Run(fmt.Sprintf("seed-%d-%d-cap-%d", seed[0], seed[1], capacity), func(t *testing.T) {
				rng := rand.New(rand.NewPCG(seed[0], seed[1]))
				r := ringbuffer.New[int](capacity)
				ref := make([]int, 0, capacity)
				for step := range 400 {
					switch rng.IntN(4) {
					case 0, 1: // push
						v := rng.IntN(100)
						evicted, ok := r.Push(v)
						if len(ref) == capacity {
							require.True(t, ok, "step %d full push must evict", step)
							require.Equal(t, ref[0], evicted, "step %d evicted", step)
							ref = append(ref[1:], v)
						} else {
							require.False(t, ok, "step %d non-full push must not evict", step)
							ref = append(ref, v)
						}
					case 2: // pop
						v, ok := r.Pop()
						if len(ref) == 0 {
							require.False(t, ok, "step %d empty pop must miss", step)
						} else {
							require.True(t, ok, "step %d pop must hit", step)
							require.Equal(t, ref[0], v, "step %d popped", step)
							ref = ref[1:]
						}
					case 3: // peek
						v, ok := r.Peek()
						require.Equal(t, len(ref) > 0, ok, "step %d peek ok", step)
						if ok {
							require.Equal(t, ref[0], v, "step %d peeked", step)
						}
					}
					require.Equal(t, len(ref), r.Len(), "step %d len", step)
					require.Equal(t, ref, r.ToSlice(), "step %d toSlice", step)
					for i, want := range ref {
						got, ok := r.At(i)
						require.True(t, ok, "step %d At(%d) ok", step, i)
						require.Equal(t, want, got, "step %d At(%d)", step, i)
					}
					var iterated []int
					for v := range r.All() {
						iterated = append(iterated, v)
					}
					require.True(t, slices.Equal(ref, iterated), "step %d All: %v vs %v", step, iterated, ref)
				}
			})
		}
	}
}
