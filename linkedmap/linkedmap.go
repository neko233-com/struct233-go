// Package linkedmap provides a map that preserves insertion order,
// similar to java.util.LinkedHashMap in insertion-order mode.
//
// Updating an existing key keeps its original position; removing and
// re-adding a key moves it to the end. Not safe for concurrent use.
package linkedmap

import "iter"

// Map is a map with deterministic insertion-order iteration.
type Map[K comparable, V any] struct {
	m          map[K]*node[K, V]
	head, tail *node[K, V] // sentinels: head.next is the first, tail.prev the last
}

type node[K comparable, V any] struct {
	key        K
	value      V
	prev, next *node[K, V]
}

// New creates an empty Map.
func New[K comparable, V any]() *Map[K, V] {
	m := &Map[K, V]{
		m:    make(map[K]*node[K, V]),
		head: &node[K, V]{},
		tail: &node[K, V]{},
	}
	m.head.next = m.tail
	m.tail.prev = m.head
	return m
}

// Put inserts or updates key, returning the previous value if present.
// Updating an existing key does not change its position.
func (m *Map[K, V]) Put(key K, value V) (old V, existed bool) {
	if n, ok := m.m[key]; ok {
		old, existed = n.value, true
		n.value = value
		return old, existed
	}
	n := &node[K, V]{key: key, value: value}
	n.prev = m.tail.prev
	n.next = m.tail
	m.tail.prev.next = n
	m.tail.prev = n
	m.m[key] = n
	return old, false
}

// Get returns the value for key.
func (m *Map[K, V]) Get(key K) (V, bool) {
	n, ok := m.m[key]
	if !ok {
		var zero V
		return zero, false
	}
	return n.value, true
}

// Remove deletes key and returns its previous value.
func (m *Map[K, V]) Remove(key K) (old V, removed bool) {
	n, ok := m.m[key]
	if !ok {
		return old, false
	}
	n.prev.next = n.next
	n.next.prev = n.prev
	delete(m.m, key)
	return n.value, true
}

// Contains reports whether key is present.
func (m *Map[K, V]) Contains(key K) bool {
	_, ok := m.m[key]
	return ok
}

// Len returns the number of entries.
func (m *Map[K, V]) Len() int { return len(m.m) }

// Clear removes all entries.
func (m *Map[K, V]) Clear() {
	clear(m.m)
	m.head.next = m.tail
	m.tail.prev = m.head
}

// First returns the entry that was inserted first.
func (m *Map[K, V]) First() (K, V, bool) {
	n := m.head.next
	if n == m.tail {
		var (
			zeroK K
			zeroV V
		)
		return zeroK, zeroV, false
	}
	return n.key, n.value, true
}

// Last returns the most recently inserted entry.
func (m *Map[K, V]) Last() (K, V, bool) {
	n := m.tail.prev
	if n == m.head {
		var (
			zeroK K
			zeroV V
		)
		return zeroK, zeroV, false
	}
	return n.key, n.value, true
}

// Keys returns the keys in insertion order.
func (m *Map[K, V]) Keys() []K {
	out := make([]K, 0, len(m.m))
	for n := m.head.next; n != m.tail; n = n.next {
		out = append(out, n.key)
	}
	return out
}

// Values returns the values in insertion order of their keys.
func (m *Map[K, V]) Values() []V {
	out := make([]V, 0, len(m.m))
	for n := m.head.next; n != m.tail; n = n.next {
		out = append(out, n.value)
	}
	return out
}

// All iterates over the entries in insertion order.
func (m *Map[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for n := m.head.next; n != m.tail; n = n.next {
			if !yield(n.key, n.value) {
				return
			}
		}
	}
}

// Clone returns a shallow copy with the same entries in the same order.
func (m *Map[K, V]) Clone() *Map[K, V] {
	c := New[K, V]()
	for n := m.head.next; n != m.tail; n = n.next {
		c.Put(n.key, n.value)
	}
	return c
}
