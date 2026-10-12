// Package skiplist provides an ordered map built on a skip list.
//
// It exposes the NavigableMap-style operations known from the JDK
// (java.util.TreeMap / java.util.concurrent.ConcurrentSkipListMap):
// First, Last, Floor, Ceiling, Lower, Higher and ordered range iteration.
package skiplist

import (
	"cmp"
	"iter"
	"math/rand/v2"
)

// Comparator returns a negative number if a < b, zero if a == b,
// and a positive number if a > b (same contract as java.util.Comparator).
type Comparator[K any] func(a, b K) int

const (
	defaultMaxLevel = 32
	defaultP        = 0.25
)

type node[K any, V any] struct {
	key     K
	val     V
	forward []*node[K, V]
}

// SkipListMap is an ordered map: entries are kept sorted by the comparator.
//
// A SkipListMap is not safe for concurrent use; guard it externally
// (for example with a sync.Mutex) when shared across goroutines.
type SkipListMap[K any, V any] struct {
	cmp      Comparator[K]
	maxLevel int
	p        float64
	head     *node[K, V]
	size     int
	// update is a scratch buffer reused by writes, so Put/Remove do not
	// allocate per operation.
	update []*node[K, V]
}

// New creates a SkipListMap ordered by the natural ordering of K.
func New[K cmp.Ordered, V any]() *SkipListMap[K, V] {
	return NewFunc[K, V](cmp.Compare[K])
}

// NewFunc creates a SkipListMap ordered by a custom comparator.
// It panics if compare is nil.
func NewFunc[K any, V any](compare Comparator[K]) *SkipListMap[K, V] {
	if compare == nil {
		panic("skiplist: nil comparator")
	}
	return &SkipListMap[K, V]{
		cmp:      compare,
		maxLevel: defaultMaxLevel,
		p:        defaultP,
		head:     &node[K, V]{forward: make([]*node[K, V], defaultMaxLevel)},
		update:   make([]*node[K, V], defaultMaxLevel),
	}
}

// Len returns the number of entries.
func (s *SkipListMap[K, V]) Len() int { return s.size }

func (s *SkipListMap[K, V]) randomLevel() int {
	level := 1
	for level < s.maxLevel && rand.Float64() < s.p {
		level++
	}
	return level
}

// predecessors returns, for every level, the rightmost node whose key is
// strictly smaller than key. The result aliases the internal scratch
// buffer and is valid until the next write.
func (s *SkipListMap[K, V]) predecessors(key K) []*node[K, V] {
	update := s.update
	cur := s.head
	for level := s.maxLevel - 1; level >= 0; level-- {
		for next := cur.forward[level]; next != nil && s.cmp(next.key, key) < 0; next = cur.forward[level] {
			cur = next
		}
		update[level] = cur
	}
	return update
}

// lowerBound returns the first node whose key is >= target (inclusive)
// or > target (exclusive), or nil when no such node exists.
func (s *SkipListMap[K, V]) lowerBound(target K, inclusive bool) *node[K, V] {
	cur := s.head
	for level := s.maxLevel - 1; level >= 0; level-- {
		for next := cur.forward[level]; next != nil; next = cur.forward[level] {
			c := s.cmp(next.key, target)
			if c < 0 || (c == 0 && !inclusive) {
				cur = next
			} else {
				break
			}
		}
	}
	return cur.forward[0]
}

// floorNode returns the last node whose key is <= target (inclusive)
// or < target (exclusive), or nil when no such node exists.
func (s *SkipListMap[K, V]) floorNode(target K, inclusive bool) *node[K, V] {
	cur := s.head
	for level := s.maxLevel - 1; level >= 0; level-- {
		for next := cur.forward[level]; next != nil; next = cur.forward[level] {
			c := s.cmp(next.key, target)
			if c < 0 || (c == 0 && inclusive) {
				cur = next
			} else {
				break
			}
		}
	}
	if cur == s.head {
		return nil
	}
	return cur
}

// Put inserts or updates key, returning the previous value if present.
func (s *SkipListMap[K, V]) Put(key K, val V) (old V, existed bool) {
	update := s.predecessors(key)
	if found := update[0].forward[0]; found != nil && s.cmp(found.key, key) == 0 {
		old, existed = found.val, true
		found.val = val
		return old, existed
	}
	level := s.randomLevel()
	n := &node[K, V]{key: key, val: val, forward: make([]*node[K, V], level)}
	for i := 0; i < level; i++ {
		n.forward[i] = update[i].forward[i]
		update[i].forward[i] = n
	}
	s.size++
	return old, false
}

// Get returns the value stored for key.
func (s *SkipListMap[K, V]) Get(key K) (val V, ok bool) {
	found := s.lowerBound(key, true)
	if found != nil && s.cmp(found.key, key) == 0 {
		return found.val, true
	}
	return val, false
}

// ContainsKey reports whether key is present.
func (s *SkipListMap[K, V]) ContainsKey(key K) bool {
	_, ok := s.Get(key)
	return ok
}

// Remove deletes key and returns its previous value.
func (s *SkipListMap[K, V]) Remove(key K) (old V, removed bool) {
	update := s.predecessors(key)
	found := update[0].forward[0]
	if found == nil || s.cmp(found.key, key) != 0 {
		return old, false
	}
	for i := range found.forward {
		update[i].forward[i] = found.forward[i]
	}
	s.size--
	return found.val, true
}

// Clear removes all entries.
func (s *SkipListMap[K, V]) Clear() {
	clear(s.head.forward)
	clear(s.update)
	s.size = 0
}

// First returns the entry with the smallest key.
func (s *SkipListMap[K, V]) First() (key K, val V, ok bool) {
	n := s.head.forward[0]
	if n == nil {
		return key, val, false
	}
	return n.key, n.val, true
}

// Last returns the entry with the greatest key.
func (s *SkipListMap[K, V]) Last() (key K, val V, ok bool) {
	cur := s.head
	for level := s.maxLevel - 1; level >= 0; level-- {
		for cur.forward[level] != nil {
			cur = cur.forward[level]
		}
	}
	if cur == s.head {
		return key, val, false
	}
	return cur.key, cur.val, true
}

// Floor returns the entry with the greatest key <= key.
func (s *SkipListMap[K, V]) Floor(key K) (k K, val V, ok bool) {
	n := s.floorNode(key, true)
	if n == nil {
		return k, val, false
	}
	return n.key, n.val, true
}

// Ceiling returns the entry with the smallest key >= key.
func (s *SkipListMap[K, V]) Ceiling(key K) (k K, val V, ok bool) {
	n := s.lowerBound(key, true)
	if n == nil {
		return k, val, false
	}
	return n.key, n.val, true
}

// Lower returns the entry with the greatest key < key.
func (s *SkipListMap[K, V]) Lower(key K) (k K, val V, ok bool) {
	n := s.floorNode(key, false)
	if n == nil {
		return k, val, false
	}
	return n.key, n.val, true
}

// Higher returns the entry with the smallest key > key.
func (s *SkipListMap[K, V]) Higher(key K) (k K, val V, ok bool) {
	n := s.lowerBound(key, false)
	if n == nil {
		return k, val, false
	}
	return n.key, n.val, true
}

// All iterates over all entries in ascending key order.
// It supports range-over-func: `for k, v := range sl.All()`.
func (s *SkipListMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for n := s.head.forward[0]; n != nil; n = n.forward[0] {
			if !yield(n.key, n.val) {
				return
			}
		}
	}
}

// Range iterates over the entries whose keys fall in [from, to],
// honoring the inclusivity flags, in ascending order.
func (s *SkipListMap[K, V]) Range(from, to K, fromInclusive, toInclusive bool) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for n := s.lowerBound(from, fromInclusive); n != nil; n = n.forward[0] {
			c := s.cmp(n.key, to)
			if c > 0 || (c == 0 && !toInclusive) {
				return
			}
			if !yield(n.key, n.val) {
				return
			}
		}
	}
}

// Keys returns all keys in ascending order.
func (s *SkipListMap[K, V]) Keys() []K {
	keys := make([]K, 0, s.size)
	for n := s.head.forward[0]; n != nil; n = n.forward[0] {
		keys = append(keys, n.key)
	}
	return keys
}
