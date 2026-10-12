// Package ttlcache provides a goroutine-safe cache with both a capacity bound
// (least-recently-used eviction) and a per-entry time-to-live.
//
// Expiry is evaluated lazily on access; Cleanup sweeps the expired entries
// eagerly. The clock is injectable, which keeps tests deterministic.
package ttlcache

import (
	"sync"
	"time"
)

type entry[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
	prev      *entry[K, V]
	next      *entry[K, V]
}

// Cache is an LRU cache whose entries also expire.
type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	ttl      time.Duration
	now      func() time.Time
	m        map[K]*entry[K, V]
	head     *entry[K, V] // sentinel: head.next is the most recently used
	tail     *entry[K, V] // sentinel: tail.prev is the least recently used
}

// New creates a cache with the given capacity and default TTL. It panics if
// capacity <= 0 or ttl <= 0.
func New[K comparable, V any](capacity int, ttl time.Duration) *Cache[K, V] {
	return NewWithClock[K, V](capacity, ttl, time.Now)
}

// NewWithClock creates a cache that reads the current time from now, which is
// handy for deterministic tests. It panics on invalid arguments.
// now runs under the cache's lock and must not call methods on the same Cache.
// Cleanup and Keys each evaluate all entries against one clock reading.
func NewWithClock[K comparable, V any](capacity int, ttl time.Duration, now func() time.Time) *Cache[K, V] {
	if capacity <= 0 {
		panic("ttlcache: capacity must be positive")
	}
	if ttl <= 0 {
		panic("ttlcache: ttl must be positive")
	}
	if now == nil {
		panic("ttlcache: nil clock")
	}
	c := &Cache[K, V]{
		capacity: capacity,
		ttl:      ttl,
		now:      now,
		m:        make(map[K]*entry[K, V], capacity),
		head:     &entry[K, V]{},
		tail:     &entry[K, V]{},
	}
	c.head.next = c.tail
	c.tail.prev = c.head
	return c
}

// Capacity returns the maximum number of entries.
func (c *Cache[K, V]) Capacity() int { return c.capacity }

// TTL returns the default time-to-live.
func (c *Cache[K, V]) TTL() time.Duration { return c.ttl }

// Len returns the number of stored entries, including not-yet-swept expired
// ones. Use Cleanup to remove them eagerly.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

// Put stores key with the default TTL, evicting the least-recently-used entry
// when the capacity is exceeded.
func (c *Cache[K, V]) Put(key K, value V) {
	c.PutWithTTL(key, value, c.ttl)
}

// PutWithTTL stores key with an explicit TTL, evicting the least-recently-used
// entry when the capacity is exceeded.
func (c *Cache[K, V]) PutWithTTL(key K, value V, ttl time.Duration) {
	if ttl <= 0 {
		c.Remove(key)
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.m[key]; ok {
		e.value = value
		e.expiresAt = c.now().Add(ttl)
		c.moveToFront(e)
		return
	}
	for len(c.m) >= c.capacity {
		victim := c.tail.prev
		if victim == c.head {
			break
		}
		c.unlink(victim)
		delete(c.m, victim.key)
	}
	e := &entry[K, V]{key: key, value: value, expiresAt: c.now().Add(ttl)}
	c.m[key] = e
	c.linkFront(e)
}

// Get returns the value for key, promoting the entry to most-recently-used.
// Expired entries are removed and reported as absent.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		var zero V
		return zero, false
	}
	if c.expired(e) {
		c.unlink(e)
		delete(c.m, key)
		var zero V
		return zero, false
	}
	c.moveToFront(e)
	return e.value, true
}

// Peek returns the value for key without changing the recency order.
// Expired entries are removed and reported as absent.
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		var zero V
		return zero, false
	}
	if c.expired(e) {
		c.unlink(e)
		delete(c.m, key)
		var zero V
		return zero, false
	}
	return e.value, true
}

// Contains reports whether key is present and unexpired, without affecting
// recency.
func (c *Cache[K, V]) Contains(key K) bool {
	_, ok := c.Peek(key)
	return ok
}

// Remove deletes key and reports whether it was present.
func (c *Cache[K, V]) Remove(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		return false
	}
	c.unlink(e)
	delete(c.m, key)
	return true
}

// Cleanup removes all expired entries and returns how many were dropped.
func (c *Cache[K, V]) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	removed := 0
	if len(c.m) == 0 {
		return removed
	}
	now := c.now()
	for e := c.tail.prev; e != c.head; {
		prev := e.prev
		if !now.Before(e.expiresAt) {
			c.unlink(e)
			delete(c.m, e.key)
			removed++
		}
		e = prev
	}
	return removed
}

// Clear removes all entries.
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.m)
	c.head.next = c.tail
	c.tail.prev = c.head
}

// Keys returns the unexpired keys from least- to most-recently-used.
func (c *Cache[K, V]) Keys() []K {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]K, 0, len(c.m))
	if len(c.m) == 0 {
		return out
	}
	now := c.now()
	for e := c.tail.prev; e != c.head; e = e.prev {
		if now.Before(e.expiresAt) {
			out = append(out, e.key)
		}
	}
	return out
}

func (c *Cache[K, V]) expired(e *entry[K, V]) bool {
	return !c.now().Before(e.expiresAt)
}

func (c *Cache[K, V]) linkFront(e *entry[K, V]) {
	e.prev = c.head
	e.next = c.head.next
	c.head.next.prev = e
	c.head.next = e
}

func (c *Cache[K, V]) unlink(e *entry[K, V]) {
	e.prev.next = e.next
	e.next.prev = e.prev
	e.prev, e.next = nil, nil
}

func (c *Cache[K, V]) moveToFront(e *entry[K, V]) {
	e.prev.next = e.next
	e.next.prev = e.prev
	c.linkFront(e)
}
