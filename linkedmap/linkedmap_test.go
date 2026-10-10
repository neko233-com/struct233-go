package linkedmap_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/linkedmap"
)

func TestInsertionOrder(t *testing.T) {
	m := linkedmap.New[string, int]()

	_, existed := m.Put("b", 1)
	require.False(t, existed)
	m.Put("a", 2)
	m.Put("c", 3)
	require.Equal(t, []string{"b", "a", "c"}, m.Keys())
	require.Equal(t, []int{1, 2, 3}, m.Values())
	require.Equal(t, 3, m.Len())

	// updating keeps the original position
	old, existed := m.Put("b", 10)
	require.True(t, existed)
	require.Equal(t, 1, old)
	require.Equal(t, []string{"b", "a", "c"}, m.Keys(), "update must not reorder")
	require.Equal(t, []int{10, 2, 3}, m.Values())

	// remove + re-add moves the key to the end
	old, removed := m.Remove("b")
	require.True(t, removed)
	require.Equal(t, 10, old)
	m.Put("b", 11)
	require.Equal(t, []string{"a", "c", "b"}, m.Keys())
}

func TestGetAndContains(t *testing.T) {
	m := linkedmap.New[string, int]()
	m.Put("a", 1)

	v, ok := m.Get("a")
	require.True(t, ok)
	require.Equal(t, 1, v)

	_, ok = m.Get("missing")
	require.False(t, ok)
	require.True(t, m.Contains("a"))
	require.False(t, m.Contains("missing"))
}

func TestFirstLast(t *testing.T) {
	m := linkedmap.New[string, int]()

	_, _, ok := m.First()
	require.False(t, ok, "First on empty map must miss")
	_, _, ok = m.Last()
	require.False(t, ok, "Last on empty map must miss")

	m.Put("a", 1)
	m.Put("b", 2)
	m.Put("c", 3)

	k, v, ok := m.First()
	require.True(t, ok)
	require.Equal(t, "a", k)
	require.Equal(t, 1, v)

	k, v, ok = m.Last()
	require.True(t, ok)
	require.Equal(t, "c", k)
	require.Equal(t, 3, v)
}

func TestAll(t *testing.T) {
	m := linkedmap.New[int, string]()
	m.Put(3, "c")
	m.Put(1, "a")
	m.Put(2, "b")

	var keys []int
	for k, v := range m.All() {
		require.Equal(t, m.Keys()[len(keys)], k)
		require.NotEmpty(t, v)
		keys = append(keys, k)
	}
	require.Equal(t, []int{3, 1, 2}, keys, "All must follow insertion order")

	count := 0
	for range m.All() {
		count++
		break
	}
	require.Equal(t, 1, count, "break must stop the iteration")
}

func TestClearAndClone(t *testing.T) {
	m := linkedmap.New[int, int]()
	m.Put(1, 1)
	m.Put(2, 2)

	c := m.Clone()
	c.Put(3, 3)
	require.Equal(t, 2, m.Len(), "Clone must be independent")
	require.Equal(t, []int{1, 2, 3}, c.Keys())

	m.Clear()
	require.Zero(t, m.Len())
	require.Empty(t, m.Keys())
	require.False(t, m.Contains(1))

	m.Put(9, 9)
	require.Equal(t, []int{9}, m.Keys(), "map must stay usable after Clear")
}
