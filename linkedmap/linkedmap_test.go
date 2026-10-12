package linkedmap_test

import (
	"fmt"
	"math/rand/v2"
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

// TestLinkedMap_TableDriven walks every exported method through normal,
// boundary and missing-key cases with one table.
func TestLinkedMap_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"put new key returns zero and false", func(t *testing.T) {
			m := linkedmap.New[string, int]()
			old, existed := m.Put("k", 7)
			require.False(t, existed)
			require.Zero(t, old)
		}},
		{"put existing key returns old value and true", func(t *testing.T) {
			m := linkedmap.New[string, int]()
			m.Put("k", 7)
			old, existed := m.Put("k", 8)
			require.True(t, existed)
			require.Equal(t, 7, old)
		}},
		{"put keeps insertion position on update", func(t *testing.T) {
			m := linkedmap.New[string, int]()
			m.Put("a", 1)
			m.Put("b", 2)
			m.Put("a", 3)
			require.Equal(t, []string{"a", "b"}, m.Keys())
		}},
		{"put after remove appends at the end", func(t *testing.T) {
			m := linkedmap.New[string, int]()
			m.Put("a", 1)
			m.Put("b", 2)
			m.Remove("a")
			m.Put("a", 3)
			require.Equal(t, []string{"b", "a"}, m.Keys())
		}},
		{"put zero key is a normal key", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(0, 5)
			v, ok := m.Get(0)
			require.True(t, ok)
			require.Equal(t, 5, v)
		}},
		{"put zero value is distinguishable from missing", func(t *testing.T) {
			m := linkedmap.New[string, int]()
			m.Put("k", 0)
			v, ok := m.Get("k")
			require.True(t, ok)
			require.Zero(t, v)
		}},
		{"put string key and value", func(t *testing.T) {
			m := linkedmap.New[string, string]()
			m.Put("greeting", "hello")
			v, ok := m.Get("greeting")
			require.True(t, ok)
			require.Equal(t, "hello", v)
		}},
		{"get hit", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(1, 10)
			v, ok := m.Get(1)
			require.True(t, ok)
			require.Equal(t, 10, v)
		}},
		{"get miss returns zero value", func(t *testing.T) {
			m := linkedmap.New[int, string]()
			v, ok := m.Get(42)
			require.False(t, ok)
			require.Empty(t, v)
		}},
		{"get on empty map misses", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			_, ok := m.Get(0)
			require.False(t, ok)
		}},
		{"get after clear misses", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(1, 1)
			m.Clear()
			_, ok := m.Get(1)
			require.False(t, ok)
		}},
		{"remove hit", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(1, 10)
			old, removed := m.Remove(1)
			require.True(t, removed)
			require.Equal(t, 10, old)
			require.Zero(t, m.Len())
		}},
		{"remove miss returns zero and false", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			old, removed := m.Remove(7)
			require.False(t, removed)
			require.Zero(t, old)
		}},
		{"remove twice reports only once", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(1, 1)
			_, removed := m.Remove(1)
			require.True(t, removed)
			_, removed = m.Remove(1)
			require.False(t, removed)
		}},
		{"remove head keeps order", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(1, 1)
			m.Put(2, 2)
			m.Put(3, 3)
			m.Remove(1)
			require.Equal(t, []int{2, 3}, m.Keys())
		}},
		{"remove middle keeps order", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(1, 1)
			m.Put(2, 2)
			m.Put(3, 3)
			m.Remove(2)
			require.Equal(t, []int{1, 3}, m.Keys())
		}},
		{"remove tail keeps order", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(1, 1)
			m.Put(2, 2)
			m.Put(3, 3)
			m.Remove(3)
			require.Equal(t, []int{1, 2}, m.Keys())
		}},
		{"contains hit and miss", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(1, 1)
			require.True(t, m.Contains(1))
			require.False(t, m.Contains(2))
		}},
		{"len grows and shrinks", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			require.Zero(t, m.Len())
			m.Put(1, 1)
			m.Put(2, 2)
			require.Equal(t, 2, m.Len())
			m.Remove(1)
			require.Equal(t, 1, m.Len())
		}},
		{"len counts distinct keys only", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(1, 1)
			m.Put(1, 2)
			require.Equal(t, 1, m.Len())
		}},
		{"clear on empty map is a no-op", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Clear()
			require.Zero(t, m.Len())
		}},
		{"clear then reuse works", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(1, 1)
			m.Put(2, 2)
			m.Clear()
			m.Put(3, 3)
			require.Equal(t, []int{3}, m.Keys())
			require.Equal(t, 1, m.Len())
		}},
		{"clear resets first and last", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(1, 1)
			m.Clear()
			_, _, ok := m.First()
			require.False(t, ok)
			_, _, ok = m.Last()
			require.False(t, ok)
		}},
		{"first on empty map", func(t *testing.T) {
			m := linkedmap.New[string, int]()
			k, v, ok := m.First()
			require.False(t, ok)
			require.Empty(t, k)
			require.Zero(t, v)
		}},
		{"first is the oldest key", func(t *testing.T) {
			m := linkedmap.New[string, int]()
			m.Put("z", 1)
			m.Put("a", 2)
			k, v, ok := m.First()
			require.True(t, ok)
			require.Equal(t, "z", k)
			require.Equal(t, 1, v)
		}},
		{"first with one element", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(5, 50)
			k, v, ok := m.First()
			require.True(t, ok)
			require.Equal(t, 5, k)
			require.Equal(t, 50, v)
		}},
		{"last on empty map", func(t *testing.T) {
			m := linkedmap.New[string, int]()
			k, v, ok := m.Last()
			require.False(t, ok)
			require.Empty(t, k)
			require.Zero(t, v)
		}},
		{"last is the newest key", func(t *testing.T) {
			m := linkedmap.New[string, int]()
			m.Put("z", 1)
			m.Put("a", 2)
			k, v, ok := m.Last()
			require.True(t, ok)
			require.Equal(t, "a", k)
			require.Equal(t, 2, v)
		}},
		{"last follows update position", func(t *testing.T) {
			m := linkedmap.New[string, int]()
			m.Put("a", 1)
			m.Put("b", 2)
			m.Put("a", 3) // update, does not move
			k, _, ok := m.Last()
			require.True(t, ok)
			require.Equal(t, "b", k)
		}},
		{"keys on empty map", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			require.Empty(t, m.Keys())
		}},
		{"values on empty map", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			require.Empty(t, m.Values())
		}},
		{"keys and values stay aligned after update", func(t *testing.T) {
			m := linkedmap.New[string, int]()
			m.Put("a", 1)
			m.Put("b", 2)
			m.Put("a", 3)
			require.Equal(t, []string{"a", "b"}, m.Keys())
			require.Equal(t, []int{3, 2}, m.Values())
		}},
		{"all on empty map yields nothing", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			count := 0
			for range m.All() {
				count++
			}
			require.Zero(t, count)
		}},
		{"all yields keys and values together", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(1, 10)
			m.Put(2, 20)
			sum := 0
			for k, v := range m.All() {
				sum += k + v
			}
			require.Equal(t, 33, sum)
		}},
		{"all supports early break", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			for i := range 5 {
				m.Put(i, i)
			}
			count := 0
			for range m.All() {
				count++
				if count == 2 {
					break
				}
			}
			require.Equal(t, 2, count)
		}},
		{"clone of empty map is empty", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			c := m.Clone()
			require.Zero(t, c.Len())
			require.Empty(t, c.Keys())
		}},
		{"clone preserves insertion order", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(3, 1)
			m.Put(1, 2)
			m.Put(2, 3)
			c := m.Clone()
			require.Equal(t, []int{3, 1, 2}, c.Keys())
		}},
		{"clone is independent for values", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(1, 1)
			c := m.Clone()
			c.Put(1, 99)
			v, _ := m.Get(1)
			require.Equal(t, 1, v, "clone writes must not leak into the original")
		}},
		{"clone is independent for removal", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(1, 1)
			c := m.Clone()
			c.Remove(1)
			require.True(t, m.Contains(1))
			require.False(t, c.Contains(1))
		}},
		{"mixed key types struct key", func(t *testing.T) {
			type pair struct{ a, b int }
			m := linkedmap.New[pair, string]()
			m.Put(pair{1, 2}, "x")
			v, ok := m.Get(pair{1, 2})
			require.True(t, ok)
			require.Equal(t, "x", v)
		}},
		{"remove single element empties the list", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			m.Put(1, 1)
			m.Remove(1)
			require.Empty(t, m.Keys())
			require.Empty(t, m.Values())
			_, _, ok := m.First()
			require.False(t, ok)
			_, _, ok = m.Last()
			require.False(t, ok)
		}},
		{"many insertions keep strict order", func(t *testing.T) {
			m := linkedmap.New[int, int]()
			for i := range 100 {
				m.Put(i, i)
			}
			keys := m.Keys()
			require.Len(t, keys, 100)
			for i, k := range keys {
				require.Equal(t, i, k)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// linkedMapModel is the reference implementation: a value map plus the
// insertion order of its keys, mirroring the Put/Remove semantics.
type linkedMapModel struct {
	values map[int]int
	order  []int
}

func newLinkedMapModel() *linkedMapModel {
	return &linkedMapModel{values: map[int]int{}}
}

func (mdl *linkedMapModel) put(key, value int) (old int, existed bool) {
	old, existed = mdl.values[key]
	mdl.values[key] = value
	if !existed {
		mdl.order = append(mdl.order, key)
	}
	return old, existed
}

func (mdl *linkedMapModel) remove(key int) (old int, removed bool) {
	old, removed = mdl.values[key]
	if !removed {
		return 0, false
	}
	delete(mdl.values, key)
	for i, k := range mdl.order {
		if k == key {
			mdl.order = append(mdl.order[:i], mdl.order[i+1:]...)
			break
		}
	}
	return old, true
}

// TestLinkedMapAgainstModel is a differential test: random Put/Remove traffic
// replayed against the map+insertion-order model, checking every observable
// after each operation.
func TestLinkedMapAgainstModel(t *testing.T) {
	for _, seed := range [][2]uint64{{2026, 1012}, {7, 11}, {42, 4242}} {
		t.Run(fmt.Sprintf("seed-%d-%d", seed[0], seed[1]), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(seed[0], seed[1]))
			m := linkedmap.New[int, int]()
			mdl := newLinkedMapModel()
			for step := range 300 {
				key := rng.IntN(16)
				if rng.IntN(3) > 0 {
					old, existed := m.Put(key, step)
					refOld, refExisted := mdl.put(key, step)
					require.Equal(t, refExisted, existed, "step %d Put(%d) existed", step, key)
					require.Equal(t, refOld, old, "step %d Put(%d) old", step, key)
				} else {
					old, removed := m.Remove(key)
					refOld, refRemoved := mdl.remove(key)
					require.Equal(t, refRemoved, removed, "step %d Remove(%d) removed", step, key)
					require.Equal(t, refOld, old, "step %d Remove(%d) old", step, key)
				}
				require.Equal(t, mdl.order, m.Keys(), "step %d keys order", step)
				wantValues := make([]int, 0, len(mdl.order))
				for _, k := range mdl.order {
					wantValues = append(wantValues, mdl.values[k])
				}
				require.Equal(t, wantValues, m.Values(), "step %d values order", step)
				require.Equal(t, len(mdl.values), m.Len(), "step %d len", step)
				for i := range 16 {
					want, wantOK := mdl.values[i]
					got, gotOK := m.Get(i)
					require.Equal(t, wantOK, gotOK, "step %d Get(%d) ok", step, i)
					require.Equal(t, want, got, "step %d Get(%d) value", step, i)
					require.Equal(t, wantOK, m.Contains(i), "step %d Contains(%d)", step, i)
				}
			}
		})
	}
}
