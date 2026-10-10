package pqueue_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/pqueue"
)

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

func TestMinQueueCustomComparator(t *testing.T) {
	q := pqueue.NewFunc(func(a, b int) int { return a - b })
	q.Push(3)
	q.Push(1)
	q.Push(2)

	var got []int
	for {
		v, ok := q.Pop()
		if !ok {
			break
		}
		got = append(got, v)
	}
	require.Equal(t, []int{1, 2, 3}, got)
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

func TestNewFuncPanicsOnNilComparator(t *testing.T) {
	require.Panics(t, func() { pqueue.NewFunc[int](nil) })
}
