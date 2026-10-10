// Package lru provides a fixed-capacity LRU (least-recently-used) cache,
// equivalent to java.util.LinkedHashMap with accessOrder=true plus a
// removeEldestEntry policy.
//
// Not safe for concurrent use.
package lru

import "iter"

// LRU evicts the least-recently-used entry when inserting a new key would
// exceed the capacity. Get promotes an entry to most-recently-used; Peek
// and Contains do not.
type LRU[K comparable, V any] struct {
	capacity int
	m        map[K]*entry[K, V]
	head     *entry[K, V] // sentinel: head.next is the most recently used
	tail     *entry[K, V] // sentinel: tail.prev is the least recently used
}

type entry[K comparable, V any] struct {
	key        K
	value      V
	prev, next *entry[K, V]
}

// New creates an LRU cache with the given capacity. It panics if
// capacity <= 0.
func New[K comparable, V any](capacity int) *LRU[K, V] {
	if capacity <= 0 {
		panic("lru: capacity must be positive")
	}
	l := &LRU[K, V]{
		capacity: capacity,
		m:        make(map[K]*entry[K, V], capacity),
		head:     &entry[K, V]{},
		tail:     &entry[K, V]{},
	}
	l.head.next = l.tail
	l.tail.prev = l.head
	return l
}

// Len returns the number of cached entries.
func (l *LRU[K, V]) Len() int { return len(l.m) }

// Capacity returns the maximum number of entries.
func (l *LRU[K, V]) Capacity() int { return l.capacity }

// Get returns the value for key and promotes the entry to
// most-recently-used.
func (l *LRU[K, V]) Get(key K) (V, bool) {
	e, ok := l.m[key]
	if !ok {
		var zero V
		return zero, false
	}
	l.moveToFront(e)
	return e.value, true
}

// Peek returns the value for key without changing the recency order.
func (l *LRU[K, V]) Peek(key K) (V, bool) {
	e, ok := l.m[key]
	if !ok {
		var zero V
		return zero, false
	}
	return e.value, true
}

// Contains reports whether key is cached, without affecting recency.
func (l *LRU[K, V]) Contains(key K) bool {
	_, ok := l.m[key]
	return ok
}

// Put inserts or updates key. Updating an existing key never evicts.
// Inserting a new key when the cache is full evicts the
// least-recently-used entry and reports it.
func (l *LRU[K, V]) Put(key K, value V) (evictedKey K, evictedValue V, evicted bool) {
	if e, ok := l.m[key]; ok {
		e.value = value
		l.moveToFront(e)
		return evictedKey, evictedValue, false
	}
	if len(l.m) >= l.capacity {
		victim := l.tail.prev
		l.unlink(victim)
		delete(l.m, victim.key)
		evictedKey, evictedValue, evicted = victim.key, victim.value, true
	}
	e := &entry[K, V]{key: key, value: value}
	l.m[key] = e
	l.linkFront(e)
	return evictedKey, evictedValue, evicted
}

// Remove deletes key and reports whether it was cached.
func (l *LRU[K, V]) Remove(key K) bool {
	e, ok := l.m[key]
	if !ok {
		return false
	}
	l.unlink(e)
	delete(l.m, key)
	return true
}

// Clear removes all entries.
func (l *LRU[K, V]) Clear() {
	l.m = make(map[K]*entry[K, V], l.capacity)
	l.head.next = l.tail
	l.tail.prev = l.head
}

// Keys returns the keys ordered from least- to most-recently-used.
func (l *LRU[K, V]) Keys() []K {
	keys := make([]K, 0, len(l.m))
	for e := l.tail.prev; e != l.head; e = e.prev {
		keys = append(keys, e.key)
	}
	return keys
}

// All iterates over the entries from least- to most-recently-used.
func (l *LRU[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for e := l.tail.prev; e != l.head; e = e.prev {
			if !yield(e.key, e.value) {
				return
			}
		}
	}
}

func (l *LRU[K, V]) linkFront(e *entry[K, V]) {
	e.prev = l.head
	e.next = l.head.next
	l.head.next.prev = e
	l.head.next = e
}

func (l *LRU[K, V]) unlink(e *entry[K, V]) {
	e.prev.next = e.next
	e.next.prev = e.prev
	e.prev, e.next = nil, nil
}

func (l *LRU[K, V]) moveToFront(e *entry[K, V]) {
	e.prev.next = e.next
	e.next.prev = e.prev
	l.linkFront(e)
}
