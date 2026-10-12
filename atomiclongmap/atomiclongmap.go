// Package atomiclongmap provides AtomicLongMap, a concurrent map from
// comparable keys to int64 counters (a Go port of Guava's
// com.google.common.util.concurrent.AtomicLongMap).
//
// Unlike a map[K]int64 guarded by a mutex, per-key updates need no external
// lock: the hot path (Add on an existing key) allocates nothing and never
// blocks concurrent readers. Counter values may be negative and are kept when
// they reach zero; call RemoveIfZero to drop a zero-valued entry.
//
//	m := atomiclongmap.New[string]()
//	m.Increment("hits")                 // 1
//	m.Add("hits", 9)                    // 10
//	m.Decrement("hits")                 // 9
//	m.Put("errors", 0)                  // 0
//	removed := m.RemoveIfZero("errors") // true: value was exactly 0
//	println(m.Get("hits"), m.Sum(), m.Len())
//	// 9 9 1
package atomiclongmap

import (
	"sync"
	"sync/atomic"
)

// AtomicLongMap is a concurrent map from K keys to int64 counters.
// The zero value is not ready for use; create one with New.
type AtomicLongMap[K comparable] struct {
	// mu guards data against structural changes (insert and delete);
	// counter updates on existing entries run under a read lock only.
	mu sync.RWMutex
	// data holds one stable counter cell per key; a cell pointer never
	// changes for the lifetime of its entry, so readers may keep it.
	data map[K]*atomic.Int64
}

// New creates an empty AtomicLongMap.
func New[K comparable]() *AtomicLongMap[K] {
	return &AtomicLongMap[K]{data: make(map[K]*atomic.Int64)}
}

// Get returns the current value for key, or 0 when key is absent.
func (m *AtomicLongMap[K]) Get(key K) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c, ok := m.data[key]; ok {
		return c.Load()
	}
	return 0
}

// GetOrAdd returns the counter cell for key, creating an empty cell when key
// is absent. Concurrent GetOrAdd calls for the same missing key share a
// single cell, so racing callers never lose updates. A cell stays readable
// after its entry is removed, but writing to a detached cell does not
// resurrect the entry; use Add for map-visible updates.
func (m *AtomicLongMap[K]) GetOrAdd(key K) *atomic.Int64 {
	m.mu.RLock()
	if c, ok := m.data[key]; ok {
		m.mu.RUnlock()
		return c
	}
	m.mu.RUnlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.data[key]; ok {
		return c
	}
	c := &atomic.Int64{}
	m.data[key] = c
	return c
}

// Add adds delta to the counter for key and returns the new value. A missing
// key starts at 0, so Add creates the entry even when delta is 0. Adding to an
// existing key allocates nothing.
func (m *AtomicLongMap[K]) Add(key K, delta int64) int64 {
	m.mu.RLock()
	if c, ok := m.data[key]; ok {
		v := c.Add(delta)
		m.mu.RUnlock()
		return v
	}
	m.mu.RUnlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.data[key]; ok {
		return c.Add(delta)
	}
	c := &atomic.Int64{}
	c.Store(delta)
	m.data[key] = c
	return delta
}

// Increment adds 1 to the counter for key and returns the new value.
func (m *AtomicLongMap[K]) Increment(key K) int64 {
	return m.Add(key, 1)
}

// Decrement subtracts 1 from the counter for key and returns the new value.
func (m *AtomicLongMap[K]) Decrement(key K) int64 {
	return m.Add(key, -1)
}

// Put sets the counter for key to value and returns the previous value,
// or 0 when key was absent.
func (m *AtomicLongMap[K]) Put(key K, value int64) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.data[key]; ok {
		return c.Swap(value)
	}
	c := &atomic.Int64{}
	c.Store(value)
	m.data[key] = c
	return 0
}

// Remove deletes key and returns its previous value, or 0 when key was
// absent. Unlike RemoveIfZero it removes regardless of the current value.
func (m *AtomicLongMap[K]) Remove(key K) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.data[key]
	if !ok {
		return 0
	}
	delete(m.data, key)
	return c.Load()
}

// RemoveIfZero deletes key only when its current value is exactly 0 and
// reports whether an entry was removed. Absent keys and non-zero values
// (including negative ones) are left untouched.
func (m *AtomicLongMap[K]) RemoveIfZero(key K) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.data[key]
	if !ok {
		return false
	}
	if c.Load() != 0 {
		return false
	}
	delete(m.data, key)
	return true
}

// Sum returns the sum of all counter values (0 for an empty map).
func (m *AtomicLongMap[K]) Sum() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var sum int64
	for _, c := range m.data {
		sum += c.Load()
	}
	return sum
}

// Len returns the number of entries, counting zero-valued entries that have
// not been removed yet.
func (m *AtomicLongMap[K]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

// Keys returns a snapshot slice of all keys in unspecified order.
func (m *AtomicLongMap[K]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]K, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys
}

// AsMap returns a snapshot copy of the counters as a plain map[K]int64.
// Later updates to the AtomicLongMap do not affect the returned map.
func (m *AtomicLongMap[K]) AsMap() map[K]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := make(map[K]int64, len(m.data))
	for k, c := range m.data {
		snapshot[k] = c.Load()
	}
	return snapshot
}

// Range calls f for every key/value pair in unspecified order until f returns
// false. Range iterates over a snapshot, so f may safely call methods on m.
// Range panics with "atomiclongmap: ..." when f is nil.
func (m *AtomicLongMap[K]) Range(f func(key K, value int64) bool) {
	if f == nil {
		panic("atomiclongmap: Range func must not be nil")
	}
	type entry struct {
		key K
		val int64
	}
	m.mu.RLock()
	entries := make([]entry, 0, len(m.data))
	for k, c := range m.data {
		entries = append(entries, entry{key: k, val: c.Load()})
	}
	m.mu.RUnlock()
	for _, e := range entries {
		if !f(e.key, e.val) {
			return
		}
	}
}

// Clear removes all entries.
func (m *AtomicLongMap[K]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	clear(m.data)
}
