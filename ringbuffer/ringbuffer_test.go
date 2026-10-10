package ringbuffer_test

import (
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
	require.Panics(t, func() { ringbuffer.New[int](0) })
	require.Panics(t, func() { ringbuffer.New[int](-3) })
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
