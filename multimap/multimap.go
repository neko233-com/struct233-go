// Package multimap provides multimaps: containers that map one key to many
// values.
//
// ListMultimap keeps every value, in key and value insertion order, mirroring
// com.google.common.collect.ListMultimap. SetMultimap keeps at most one copy of
// each value per key, mirroring com.google.common.collect.SetMultimap. Both are
// plain single-threaded containers: they hold no locks, so the race-safe
// variants belong to the concurrent package.
//
//	list := multimap.NewList[string, int]()
//	list.Put("odd", 1)
//	list.PutAll("odd", 3, 5)
//	list.Put("even", 2)
//	list.Keys()          // [odd even]
//	list.Values()        // [1 3 5 2]
//	list.Size()          // 4
//	list.Get("odd")      // [1 3 5]
//	list.Invert().Get(3) // [odd]
//
//	set := multimap.NewSet[string, int]()
//	set.PutAll("a", 1, 1, 2) // 2 values added, the duplicate is ignored
//	set.Get("a")             // [1 2]
//
// Keys, Values, Entries and AsMap always return copies. The only slices handed
// out that are not copies are those detached by RemoveAll and Replace, which no
// longer belong to the multimap.
package multimap

import (
	"iter"
	"slices"
)

// Entry is one key/value pair.
type Entry[K comparable, V any] struct {
	// Key is the key the value is stored under.
	Key K
	// Value is the stored value.
	Value V
}

// ListMultimap maps each key to a list of values, keeping key and value
// insertion order (com.google.common.collect.ListMultimap).
//
// Keys are ordered by their first Put. Removing the last value of a key drops
// the key from that order, and a later Put inserts it again at the end. The keys
// reserved by NewListWithCapacity keep their slot until a value is Put. Values
// are compared with == and become keys after Invert, so V is comparable. Not
// safe for concurrent use.
type ListMultimap[K comparable, V comparable] struct {
	slots    []listSlot[K, V] // live and reserved keys, in order
	index    map[K]int        // key -> position in slots
	size     int              // number of values
	keyCount int              // number of keys holding at least one value
}

// listSlot is one ordered key with its values. A reserved slot has none and
// stays invisible to every reader until Put gives it one.
type listSlot[K comparable, V comparable] struct {
	key    K
	values []V
}

// NewList creates an empty ListMultimap.
func NewList[K comparable, V comparable]() *ListMultimap[K, V] {
	return &ListMultimap[K, V]{index: make(map[K]int)}
}

// NewListWithCapacity creates an empty ListMultimap pre-sized for capacity keys.
// The optional keys become reserved slots: they take part in the key order but
// carry no values, so Size, KeyCount, IsEmpty, Keys and every other reader
// ignore them until a value is Put. Such a Put activates the slot in place, and
// keys repeated in the variadic list keep only their first slot. It panics if
// capacity is negative.
func NewListWithCapacity[K comparable, V comparable](capacity int, keys ...K) *ListMultimap[K, V] {
	if capacity < 0 {
		panic("multimap: negative capacity")
	}
	m := &ListMultimap[K, V]{
		slots: make([]listSlot[K, V], 0, capacity+len(keys)),
		index: make(map[K]int, capacity+len(keys)),
	}
	m.reserve(keys)
	return m
}

// Put appends v to the values of k.
func (m *ListMultimap[K, V]) Put(k K, v V) {
	i, ok := m.index[k]
	if !ok {
		m.ensure()
		m.index[k] = len(m.slots)
		m.slots = append(m.slots, listSlot[K, V]{key: k, values: []V{v}})
		m.keyCount++
	} else {
		s := &m.slots[i]
		if len(s.values) == 0 {
			m.keyCount++ // a reserved key becomes live in its reserved slot
		}
		s.values = append(s.values, v)
	}
	m.size++
}

// PutAll appends every value to the values of k.
func (m *ListMultimap[K, V]) PutAll(k K, vs ...V) {
	for _, v := range vs {
		m.Put(k, v)
	}
}

// Get returns a copy of the values of k, or nil when k holds none.
func (m *ListMultimap[K, V]) Get(k K) []V {
	i, ok := m.index[k]
	if !ok || len(m.slots[i].values) == 0 {
		return nil
	}
	return slices.Clone(m.slots[i].values)
}

// Remove removes the first occurrence of v under k and reports whether it was
// there. Removing the last value of k drops the key from the key order.
func (m *ListMultimap[K, V]) Remove(k K, v V) bool {
	i, ok := m.index[k]
	if !ok {
		return false
	}
	vs := m.slots[i].values
	j := slices.Index(vs, v)
	if j < 0 {
		return false
	}
	vs = slices.Delete(vs, j, j+1)
	m.size--
	if len(vs) == 0 {
		m.keyCount--
		m.dropAt(i)
		return true
	}
	m.slots[i].values = vs
	return true
}

// RemoveAll removes k with all of its values and returns them. The returned
// slice is detached from the multimap; nil means k held no values.
func (m *ListMultimap[K, V]) RemoveAll(k K) []V {
	i, ok := m.index[k]
	if !ok {
		return nil
	}
	vs := m.slots[i].values
	if len(vs) == 0 {
		return nil // a reserved key keeps its slot
	}
	m.size -= len(vs)
	m.keyCount--
	m.dropAt(i)
	return vs
}

// Replace sets the values of k to vs and returns the previous values, which are
// detached from the multimap. An absent key with at least one value is appended
// at the end of the key order; replacing with no values drops the key, and a
// reserved key only becomes live once it holds a value.
func (m *ListMultimap[K, V]) Replace(k K, vs ...V) []V {
	i, ok := m.index[k]
	if !ok {
		if len(vs) == 0 {
			return nil
		}
		m.ensure()
		m.index[k] = len(m.slots)
		m.slots = append(m.slots, listSlot[K, V]{key: k, values: slices.Clone(vs)})
		m.size += len(vs)
		m.keyCount++
		return nil
	}
	s := &m.slots[i]
	old := s.values
	live := len(old) > 0
	m.size -= len(old)
	s.values = slices.Clone(vs)
	m.size += len(vs)
	switch {
	case live && len(vs) == 0:
		m.keyCount--
		m.dropAt(i)
	case !live && len(vs) > 0:
		m.keyCount++
	}
	return old
}

// ContainsKey reports whether k holds at least one value.
func (m *ListMultimap[K, V]) ContainsKey(k K) bool {
	i, ok := m.index[k]
	return ok && len(m.slots[i].values) > 0
}

// ContainsEntry reports whether v is one of the values of k.
func (m *ListMultimap[K, V]) ContainsEntry(k K, v V) bool {
	i, ok := m.index[k]
	return ok && slices.Index(m.slots[i].values, v) >= 0
}

// Size returns the total number of values.
func (m *ListMultimap[K, V]) Size() int { return m.size }

// KeyCount returns the number of keys holding at least one value.
func (m *ListMultimap[K, V]) KeyCount() int { return m.keyCount }

// IsEmpty reports whether the multimap holds no values.
func (m *ListMultimap[K, V]) IsEmpty() bool { return m.size == 0 }

// Keys returns a copy of the live keys in insertion order.
func (m *ListMultimap[K, V]) Keys() []K {
	out := make([]K, 0, m.keyCount)
	for _, s := range m.slots {
		if len(s.values) > 0 {
			out = append(out, s.key)
		}
	}
	return out
}

// Values returns a copy of every value, grouped by key in key order.
func (m *ListMultimap[K, V]) Values() []V {
	out := make([]V, 0, m.size)
	for _, s := range m.slots {
		out = append(out, s.values...)
	}
	return out
}

// Entries returns a copy of every key/value pair in key order and then value
// order.
func (m *ListMultimap[K, V]) Entries() []Entry[K, V] {
	out := make([]Entry[K, V], 0, m.size)
	for _, s := range m.slots {
		for _, v := range s.values {
			out = append(out, Entry[K, V]{Key: s.key, Value: v})
		}
	}
	return out
}

// AsMap returns a snapshot as a plain map of copied value lists.
func (m *ListMultimap[K, V]) AsMap() map[K][]V {
	out := make(map[K][]V, m.keyCount)
	for _, s := range m.slots {
		if len(s.values) > 0 {
			out[s.key] = slices.Clone(s.values)
		}
	}
	return out
}

// Invert returns a new ListMultimap with keys and values swapped. The inverted
// keys follow the order in which their values were first seen, and duplicate
// pairs are preserved as duplicates.
func (m *ListMultimap[K, V]) Invert() *ListMultimap[V, K] {
	out := NewList[V, K]()
	for _, s := range m.slots {
		for _, v := range s.values {
			out.Put(v, s.key)
		}
	}
	return out
}

// Clear removes every key and value while keeping the allocated storage.
func (m *ListMultimap[K, V]) Clear() {
	clear(m.slots)
	m.slots = m.slots[:0]
	clear(m.index)
	m.size = 0
	m.keyCount = 0
}

// All iterates every key/value pair in key order and then value order.
func (m *ListMultimap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, s := range m.slots {
			for _, v := range s.values {
				if !yield(s.key, v) {
					return
				}
			}
		}
	}
}

// reserve appends keys without values, keeping the order they are given in.
func (m *ListMultimap[K, V]) reserve(keys []K) {
	for _, k := range keys {
		if _, ok := m.index[k]; ok {
			continue
		}
		m.index[k] = len(m.slots)
		m.slots = append(m.slots, listSlot[K, V]{key: k})
	}
}

// ensure creates the key index on demand so the zero value is usable.
func (m *ListMultimap[K, V]) ensure() {
	if m.index == nil {
		m.index = make(map[K]int)
	}
}

// dropAt removes the slot at i and re-indexes the keys behind it.
func (m *ListMultimap[K, V]) dropAt(i int) {
	delete(m.index, m.slots[i].key)
	m.slots = slices.Delete(m.slots, i, i+1)
	for j := i; j < len(m.slots); j++ {
		m.index[m.slots[j].key] = j
	}
}

// SetMultimap maps each key to a set of values, keeping key and value insertion
// order (com.google.common.collect.SetMultimap).
//
// The values of a key are unique. Keys are ordered by their first Put, removing
// the last value of a key drops the key from that order, and the keys reserved
// by NewSetWithCapacity keep their slot until a value is Put. Not safe for
// concurrent use.
type SetMultimap[K comparable, V comparable] struct {
	slots    []setSlot[K, V] // live and reserved keys, in order
	index    map[K]int       // key -> position in slots
	size     int             // number of values
	keyCount int             // number of keys holding at least one value
}

// setSlot is one ordered key with its unique values and their membership set. A
// reserved slot has neither and stays invisible to every reader until Put gives
// it a value.
type setSlot[K comparable, V comparable] struct {
	key    K
	values []V
	seen   map[V]struct{}
}

// NewSet creates an empty SetMultimap.
func NewSet[K comparable, V comparable]() *SetMultimap[K, V] {
	return &SetMultimap[K, V]{index: make(map[K]int)}
}

// NewSetWithCapacity creates an empty SetMultimap pre-sized for capacity keys.
// The optional keys become reserved slots: they take part in the key order but
// carry no values, so Size, KeyCount, IsEmpty, Keys and every other reader
// ignore them until a value is Put. Such a Put activates the slot in place, and
// keys repeated in the variadic list keep only their first slot. It panics if
// capacity is negative.
func NewSetWithCapacity[K comparable, V comparable](capacity int, keys ...K) *SetMultimap[K, V] {
	if capacity < 0 {
		panic("multimap: negative capacity")
	}
	m := &SetMultimap[K, V]{
		slots: make([]setSlot[K, V], 0, capacity+len(keys)),
		index: make(map[K]int, capacity+len(keys)),
	}
	m.reserve(keys)
	return m
}

// Put adds v under k and reports whether it was newly added; a value that is
// already there changes nothing.
func (m *SetMultimap[K, V]) Put(k K, v V) bool {
	i, ok := m.index[k]
	if !ok {
		m.ensure()
		m.index[k] = len(m.slots)
		m.slots = append(m.slots, setSlot[K, V]{
			key:    k,
			values: []V{v},
			seen:   map[V]struct{}{v: {}},
		})
		m.size++
		m.keyCount++
		return true
	}
	s := &m.slots[i]
	if _, dup := s.seen[v]; dup {
		return false
	}
	if s.seen == nil {
		s.seen = make(map[V]struct{}, 1)
	}
	s.seen[v] = struct{}{}
	if len(s.values) == 0 {
		m.keyCount++ // a reserved key becomes live in its reserved slot
	}
	s.values = append(s.values, v)
	m.size++
	return true
}

// PutAll adds every value to k and returns how many were newly added.
func (m *SetMultimap[K, V]) PutAll(k K, vs ...V) int {
	added := 0
	for _, v := range vs {
		if m.Put(k, v) {
			added++
		}
	}
	return added
}

// Get returns a copy of the values of k, or nil when k holds none.
func (m *SetMultimap[K, V]) Get(k K) []V {
	i, ok := m.index[k]
	if !ok || len(m.slots[i].values) == 0 {
		return nil
	}
	return slices.Clone(m.slots[i].values)
}

// Remove removes v from the values of k and reports whether it was there.
// Removing the last value of k drops the key from the key order.
func (m *SetMultimap[K, V]) Remove(k K, v V) bool {
	i, ok := m.index[k]
	if !ok {
		return false
	}
	s := &m.slots[i]
	j := slices.Index(s.values, v)
	if j < 0 {
		return false
	}
	delete(s.seen, v)
	s.values = slices.Delete(s.values, j, j+1)
	m.size--
	if len(s.values) == 0 {
		m.keyCount--
		m.dropAt(i)
	}
	return true
}

// RemoveAll removes k with all of its values and returns them. The returned
// slice is detached from the multimap; nil means k held no values.
func (m *SetMultimap[K, V]) RemoveAll(k K) []V {
	i, ok := m.index[k]
	if !ok {
		return nil
	}
	vs := m.slots[i].values
	if len(vs) == 0 {
		return nil // a reserved key keeps its slot
	}
	m.size -= len(vs)
	m.keyCount--
	m.dropAt(i)
	return vs
}

// Replace sets the values of k to the unique values of vs, in first-occurrence
// order, and returns the previous values, which are detached from the multimap.
// An absent key with at least one value is appended at the end of the key order;
// replacing with no values drops the key, and a reserved key only becomes live
// once it holds a value.
func (m *SetMultimap[K, V]) Replace(k K, vs ...V) []V {
	unique, seen := dedupe(vs)
	i, ok := m.index[k]
	if !ok {
		if len(unique) == 0 {
			return nil
		}
		m.ensure()
		m.index[k] = len(m.slots)
		m.slots = append(m.slots, setSlot[K, V]{key: k, values: unique, seen: seen})
		m.size += len(unique)
		m.keyCount++
		return nil
	}
	s := &m.slots[i]
	old := s.values
	live := len(old) > 0
	m.size -= len(old)
	s.values = unique
	s.seen = seen
	m.size += len(unique)
	switch {
	case live && len(unique) == 0:
		m.keyCount--
		m.dropAt(i)
	case !live && len(unique) > 0:
		m.keyCount++
	}
	return old
}

// ContainsKey reports whether k holds at least one value.
func (m *SetMultimap[K, V]) ContainsKey(k K) bool {
	i, ok := m.index[k]
	return ok && len(m.slots[i].values) > 0
}

// ContainsEntry reports whether v is one of the values of k.
func (m *SetMultimap[K, V]) ContainsEntry(k K, v V) bool {
	i, ok := m.index[k]
	if !ok {
		return false
	}
	_, found := m.slots[i].seen[v]
	return found
}

// Size returns the total number of values.
func (m *SetMultimap[K, V]) Size() int { return m.size }

// KeyCount returns the number of keys holding at least one value.
func (m *SetMultimap[K, V]) KeyCount() int { return m.keyCount }

// IsEmpty reports whether the multimap holds no values.
func (m *SetMultimap[K, V]) IsEmpty() bool { return m.size == 0 }

// Keys returns a copy of the live keys in insertion order.
func (m *SetMultimap[K, V]) Keys() []K {
	out := make([]K, 0, m.keyCount)
	for _, s := range m.slots {
		if len(s.values) > 0 {
			out = append(out, s.key)
		}
	}
	return out
}

// Values returns a copy of every value, grouped by key in key order.
func (m *SetMultimap[K, V]) Values() []V {
	out := make([]V, 0, m.size)
	for _, s := range m.slots {
		out = append(out, s.values...)
	}
	return out
}

// Entries returns a copy of every key/value pair in key order and then value
// order.
func (m *SetMultimap[K, V]) Entries() []Entry[K, V] {
	out := make([]Entry[K, V], 0, m.size)
	for _, s := range m.slots {
		for _, v := range s.values {
			out = append(out, Entry[K, V]{Key: s.key, Value: v})
		}
	}
	return out
}

// AsMap returns a snapshot as a plain map of copied value lists.
func (m *SetMultimap[K, V]) AsMap() map[K][]V {
	out := make(map[K][]V, m.keyCount)
	for _, s := range m.slots {
		if len(s.values) > 0 {
			out[s.key] = slices.Clone(s.values)
		}
	}
	return out
}

// Invert returns a new SetMultimap with keys and values swapped. The inverted
// keys follow the order in which their values were first seen, and pairs that
// collide after swapping merge into one.
func (m *SetMultimap[K, V]) Invert() *SetMultimap[V, K] {
	out := NewSet[V, K]()
	for _, s := range m.slots {
		for _, v := range s.values {
			out.Put(v, s.key)
		}
	}
	return out
}

// Clear removes every key and value while keeping the allocated storage.
func (m *SetMultimap[K, V]) Clear() {
	clear(m.slots)
	m.slots = m.slots[:0]
	clear(m.index)
	m.size = 0
	m.keyCount = 0
}

// All iterates every key/value pair in key order and then value order.
func (m *SetMultimap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, s := range m.slots {
			for _, v := range s.values {
				if !yield(s.key, v) {
					return
				}
			}
		}
	}
}

// reserve appends keys without values, keeping the order they are given in.
func (m *SetMultimap[K, V]) reserve(keys []K) {
	for _, k := range keys {
		if _, ok := m.index[k]; ok {
			continue
		}
		m.index[k] = len(m.slots)
		m.slots = append(m.slots, setSlot[K, V]{key: k})
	}
}

// ensure creates the key index on demand so the zero value is usable.
func (m *SetMultimap[K, V]) ensure() {
	if m.index == nil {
		m.index = make(map[K]int)
	}
}

// dropAt removes the slot at i and re-indexes the keys behind it.
func (m *SetMultimap[K, V]) dropAt(i int) {
	delete(m.index, m.slots[i].key)
	m.slots = slices.Delete(m.slots, i, i+1)
	for j := i; j < len(m.slots); j++ {
		m.index[m.slots[j].key] = j
	}
}

// dedupe returns the unique values of vs in first-occurrence order together with
// their membership set.
func dedupe[V comparable](vs []V) ([]V, map[V]struct{}) {
	out := make([]V, 0, len(vs))
	seen := make(map[V]struct{}, len(vs))
	for _, v := range vs {
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out, seen
}
