package deque_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/go-data-struct-lib/deque"
)

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
