package set

import (
	"iter"
	"slices"
)

// LinkedHashSet is a set that preserves insertion order,
// similar to java.util.LinkedHashSet. Re-adding an existing element
// does not change its position.
type LinkedHashSet[K comparable] struct {
	m          map[K]*lhsEntry[K]
	head, tail *lhsEntry[K] // sentinels: head.next is the first, tail.prev the last
}

type lhsEntry[K comparable] struct {
	key        K
	prev, next *lhsEntry[K]
}

// NewLinkedHashSet creates a LinkedHashSet pre-filled with items.
func NewLinkedHashSet[K comparable](items ...K) *LinkedHashSet[K] {
	s := &LinkedHashSet[K]{
		m:    make(map[K]*lhsEntry[K], len(items)),
		head: &lhsEntry[K]{},
		tail: &lhsEntry[K]{},
	}
	s.head.next = s.tail
	s.tail.prev = s.head
	s.AddAll(items...)
	return s
}

// Add inserts item and reports whether it was newly added.
func (s *LinkedHashSet[K]) Add(item K) bool {
	if _, ok := s.m[item]; ok {
		return false
	}
	e := &lhsEntry[K]{key: item}
	e.prev = s.tail.prev
	e.next = s.tail
	s.tail.prev.next = e
	s.tail.prev = e
	s.m[item] = e
	return true
}

// AddAll inserts every item.
func (s *LinkedHashSet[K]) AddAll(items ...K) {
	for _, item := range items {
		s.Add(item)
	}
}

// Remove deletes item and reports whether it existed.
func (s *LinkedHashSet[K]) Remove(item K) bool {
	e, ok := s.m[item]
	if !ok {
		return false
	}
	e.prev.next = e.next
	e.next.prev = e.prev
	delete(s.m, item)
	return true
}

// Contains reports whether item is present.
func (s *LinkedHashSet[K]) Contains(item K) bool {
	_, ok := s.m[item]
	return ok
}

// Len returns the number of elements.
func (s *LinkedHashSet[K]) Len() int { return len(s.m) }

// Clear removes all elements.
func (s *LinkedHashSet[K]) Clear() {
	clear(s.m)
	s.head.next = s.tail
	s.tail.prev = s.head
}

// Clone returns a shallow copy with the same elements in the same order.
func (s *LinkedHashSet[K]) Clone() *LinkedHashSet[K] {
	c := NewLinkedHashSet[K]()
	c.AddAll(s.ToSlice()...)
	return c
}

// All iterates over the elements in insertion order.
func (s *LinkedHashSet[K]) All() iter.Seq[K] {
	return func(yield func(K) bool) {
		for e := s.head.next; e != s.tail; e = e.next {
			if !yield(e.key) {
				return
			}
		}
	}
}

// ToSlice returns the elements in insertion order.
func (s *LinkedHashSet[K]) ToSlice() []K {
	out := make([]K, 0, len(s.m))
	for e := s.head.next; e != s.tail; e = e.next {
		out = append(out, e.key)
	}
	return out
}

// Equal reports whether both sets contain the same elements in the same order.
func (s *LinkedHashSet[K]) Equal(other *LinkedHashSet[K]) bool {
	return slices.Equal(s.ToSlice(), other.ToSlice())
}
