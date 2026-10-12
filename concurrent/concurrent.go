// Package concurrent provides goroutine-safe containers, mirroring the
// java.util.concurrent collections on top of the single-threaded
// implementations of this module.
//
// Every method acquires the internal lock for the duration of the operation.
// The iteration methods (All, Range, Keys, Values, ToSlice, Snapshot) return
// a snapshot taken under the lock, so callers never range while holding it and
// concurrent writers are never blocked by slow consumers.
package concurrent

import (
	"cmp"
	"iter"
	"sync"

	"github.com/neko233-com/struct233-go/lru"
	"github.com/neko233-com/struct233-go/skiplist"
)

// HashSet is a goroutine-safe unordered set, similar to a synchronized
// java.util.HashSet / ConcurrentHashMap-based key set.
// The zero value is an empty set ready for use.
type HashSet[T comparable] struct {
	mu sync.RWMutex
	m  map[T]struct{}
}

// NewHashSet creates a HashSet pre-filled with items.
func NewHashSet[T comparable](items ...T) *HashSet[T] {
	s := NewHashSetWithCapacity[T](len(items))
	s.AddAll(items...)
	return s
}

// NewHashSetWithCapacity creates an empty HashSet whose map is pre-sized.
func NewHashSetWithCapacity[T comparable](capacity int) *HashSet[T] {
	return &HashSet[T]{m: make(map[T]struct{}, capacity)}
}

// Add inserts item and reports whether it was newly added.
func (s *HashSet[T]) Add(item T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[item]; ok {
		return false
	}
	if s.m == nil {
		s.m = make(map[T]struct{})
	}
	s.m[item] = struct{}{}
	return true
}

// AddAll inserts every item atomically with respect to other set operations.
func (s *HashSet[T]) AddAll(items ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(items) == 0 {
		return
	}
	if s.m == nil {
		s.m = make(map[T]struct{}, len(items))
	}
	for _, item := range items {
		s.m[item] = struct{}{}
	}
}

// Remove deletes item and reports whether it existed.
func (s *HashSet[T]) Remove(item T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[item]; !ok {
		return false
	}
	delete(s.m, item)
	return true
}

// Contains reports whether item is present.
func (s *HashSet[T]) Contains(item T) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.m[item]
	return ok
}

// ContainsAll reports whether every item is present in one consistent snapshot.
// An empty list of items always returns true.
func (s *HashSet[T]) ContainsAll(items ...T) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range items {
		if _, ok := s.m[item]; !ok {
			return false
		}
	}
	return true
}

// Len returns the number of elements.
func (s *HashSet[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m)
}

// IsEmpty reports whether the set has no elements.
func (s *HashSet[T]) IsEmpty() bool { return s.Len() == 0 }

// Clear removes all elements.
func (s *HashSet[T]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.m)
}

// ToSlice returns a snapshot of the elements in unspecified order.
func (s *HashSet[T]) ToSlice() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]T, 0, len(s.m))
	for item := range s.m {
		out = append(out, item)
	}
	return out
}

// All iterates over a snapshot of the elements.
func (s *HashSet[T]) All() iter.Seq[T] {
	snapshot := s.ToSlice()
	return func(yield func(T) bool) {
		for _, item := range snapshot {
			if !yield(item) {
				return
			}
		}
	}
}

// HashMap is a goroutine-safe map with an atomic GetOrPut,
// similar to java.util.concurrent.ConcurrentHashMap.
type HashMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

// NewHashMap creates an empty HashMap.
func NewHashMap[K comparable, V any]() *HashMap[K, V] {
	return NewHashMapWithCapacity[K, V](0)
}

// NewHashMapWithCapacity creates an empty HashMap pre-sized for capacity entries.
func NewHashMapWithCapacity[K comparable, V any](capacity int) *HashMap[K, V] {
	return &HashMap[K, V]{m: make(map[K]V, capacity)}
}

// Get returns the value for key.
func (m *HashMap[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.m[key]
	return v, ok
}

// Put inserts or updates key, returning the previous value if present.
func (m *HashMap[K, V]) Put(key K, value V) (old V, existed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, existed = m.m[key]
	m.m[key] = value
	return old, existed
}

// GetOrPut returns the value for key, computing and storing it with fn when
// the key is absent. The whole operation is atomic, so fn runs at most once
// per key even under concurrent callers; loaded reports whether the value was
// already present.
// fn runs under the map's write lock and must not call methods on the same HashMap.
func (m *HashMap[K, V]) GetOrPut(key K, fn func() V) (value V, loaded bool) {
	m.mu.RLock()
	if v, ok := m.m[key]; ok {
		m.mu.RUnlock()
		return v, true
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.m[key]; ok { // another goroutine won the race
		return v, true
	}
	v := fn()
	m.m[key] = v
	return v, false
}

// Delete removes key and returns its previous value.
func (m *HashMap[K, V]) Delete(key K) (old V, existed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, existed = m.m[key]
	delete(m.m, key)
	return old, existed
}

// Len returns the number of entries.
func (m *HashMap[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.m)
}

// Clear removes all entries.
func (m *HashMap[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	clear(m.m)
}

// Keys returns a snapshot of the keys in unspecified order.
func (m *HashMap[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]K, 0, len(m.m))
	for k := range m.m {
		out = append(out, k)
	}
	return out
}

// Snapshot returns a copy of the current entries.
func (m *HashMap[K, V]) Snapshot() map[K]V {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[K]V, len(m.m))
	for k, v := range m.m {
		out[k] = v
	}
	return out
}

// All iterates over a snapshot of the entries.
func (m *HashMap[K, V]) All() iter.Seq2[K, V] {
	snapshot := m.Snapshot()
	return func(yield func(K, V) bool) {
		for k, v := range snapshot {
			if !yield(k, v) {
				return
			}
		}
	}
}

// SkipListMap is a goroutine-safe ordered map, the analog of
// java.util.concurrent.ConcurrentSkipListMap.
type SkipListMap[K any, V any] struct {
	mu sync.RWMutex
	sl *skiplist.SkipListMap[K, V]
}

// NewSkipListMap creates an ordered map using the natural ordering of K.
func NewSkipListMap[K cmp.Ordered, V any]() *SkipListMap[K, V] {
	return &SkipListMap[K, V]{sl: skiplist.New[K, V]()}
}

// NewSkipListMapFunc creates an ordered map using a custom comparator.
func NewSkipListMapFunc[K any, V any](compare skiplist.Comparator[K]) *SkipListMap[K, V] {
	return &SkipListMap[K, V]{sl: skiplist.NewFunc[K, V](compare)}
}

// Put inserts or updates key, returning the previous value if present.
func (s *SkipListMap[K, V]) Put(key K, val V) (old V, existed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sl.Put(key, val)
}

// Get returns the value stored for key.
func (s *SkipListMap[K, V]) Get(key K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sl.Get(key)
}

// Remove deletes key and returns its previous value.
func (s *SkipListMap[K, V]) Remove(key K) (old V, removed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sl.Remove(key)
}

// ContainsKey reports whether key is present.
func (s *SkipListMap[K, V]) ContainsKey(key K) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sl.ContainsKey(key)
}

// Len returns the number of entries.
func (s *SkipListMap[K, V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sl.Len()
}

// Clear removes all entries.
func (s *SkipListMap[K, V]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sl.Clear()
}

// First returns the entry with the smallest key.
func (s *SkipListMap[K, V]) First() (K, V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sl.First()
}

// Last returns the entry with the greatest key.
func (s *SkipListMap[K, V]) Last() (K, V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sl.Last()
}

// Floor returns the entry with the greatest key <= key.
func (s *SkipListMap[K, V]) Floor(key K) (K, V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sl.Floor(key)
}

// Ceiling returns the entry with the smallest key >= key.
func (s *SkipListMap[K, V]) Ceiling(key K) (K, V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sl.Ceiling(key)
}

// Lower returns the entry with the greatest key < key.
func (s *SkipListMap[K, V]) Lower(key K) (K, V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sl.Lower(key)
}

// Higher returns the entry with the smallest key > key.
func (s *SkipListMap[K, V]) Higher(key K) (K, V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sl.Higher(key)
}

// Keys returns a snapshot of the keys in ascending order.
func (s *SkipListMap[K, V]) Keys() []K {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sl.Keys()
}

// All iterates over a snapshot of the entries in ascending key order.
func (s *SkipListMap[K, V]) All() iter.Seq2[K, V] {
	snapshot := s.snapshot()
	return func(yield func(K, V) bool) {
		for _, e := range snapshot {
			if !yield(e.key, e.val) {
				return
			}
		}
	}
}

// Range iterates over a snapshot of the entries whose keys fall in
// [from, to], honoring the inclusivity flags.
func (s *SkipListMap[K, V]) Range(from, to K, fromInclusive, toInclusive bool) iter.Seq2[K, V] {
	s.mu.RLock()
	var snapshot []entry[K, V]
	for k, v := range s.sl.Range(from, to, fromInclusive, toInclusive) {
		snapshot = append(snapshot, entry[K, V]{key: k, val: v})
	}
	s.mu.RUnlock()
	return func(yield func(K, V) bool) {
		for _, e := range snapshot {
			if !yield(e.key, e.val) {
				return
			}
		}
	}
}

type entry[K any, V any] struct {
	key K
	val V
}

func (s *SkipListMap[K, V]) snapshot() []entry[K, V] {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]entry[K, V], 0, s.sl.Len())
	for k, v := range s.sl.All() {
		out = append(out, entry[K, V]{key: k, val: v})
	}
	return out
}

// LRU is a goroutine-safe fixed-capacity least-recently-used cache.
type LRU[K comparable, V any] struct {
	mu sync.Mutex
	l  *lru.LRU[K, V]
}

// NewLRU creates an LRU cache with the given capacity (> 0).
func NewLRU[K comparable, V any](capacity int) *LRU[K, V] {
	return &LRU[K, V]{l: lru.New[K, V](capacity)}
}

// Get returns the value for key and promotes the entry to most-recently-used.
func (c *LRU[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.l.Get(key)
}

// Peek returns the value for key without changing the recency order.
func (c *LRU[K, V]) Peek(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.l.Peek(key)
}

// Contains reports whether key is cached, without affecting recency.
func (c *LRU[K, V]) Contains(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.l.Contains(key)
}

// Put inserts or updates key, evicting the least-recently-used entry when a
// new key would exceed the capacity.
func (c *LRU[K, V]) Put(key K, value V) (evictedKey K, evictedValue V, evicted bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.l.Put(key, value)
}

// Remove deletes key and reports whether it was cached.
func (c *LRU[K, V]) Remove(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.l.Remove(key)
}

// Len returns the number of cached entries.
func (c *LRU[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.l.Len()
}

// Capacity returns the maximum number of entries.
func (c *LRU[K, V]) Capacity() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.l.Capacity()
}

// Clear removes all entries.
func (c *LRU[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.l.Clear()
}

// Keys returns a snapshot of the keys from least- to most-recently-used.
func (c *LRU[K, V]) Keys() []K {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.l.Keys()
}
