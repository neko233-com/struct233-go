package set

import (
	"cmp"
	"iter"

	"github.com/neko233-com/struct233-go/skiplist"
)

// SkipListSet is an ordered set backed by a skip list.
type SkipListSet[K any] struct {
	cmp skiplist.Comparator[K]
	sl  *skiplist.SkipListMap[K, struct{}]
}

// NewSkipListSet creates a SkipListSet ordered by the natural ordering of K.
func NewSkipListSet[K cmp.Ordered](items ...K) *SkipListSet[K] {
	ts := &SkipListSet[K]{cmp: cmp.Compare[K], sl: skiplist.New[K, struct{}]()}
	ts.AddAll(items...)
	return ts
}

// NewSkipListSetFunc creates a SkipListSet ordered by a custom comparator.
// It panics if compare is nil.
func NewSkipListSetFunc[K any](compare skiplist.Comparator[K], items ...K) *SkipListSet[K] {
	if compare == nil {
		panic("set: nil comparator")
	}
	ts := &SkipListSet[K]{cmp: compare, sl: skiplist.NewFunc[K, struct{}](compare)}
	ts.AddAll(items...)
	return ts
}

// Add inserts item and reports whether it was newly added.
func (s *SkipListSet[K]) Add(item K) bool {
	_, existed := s.sl.Put(item, struct{}{})
	return !existed
}

// AddAll inserts every item.
func (s *SkipListSet[K]) AddAll(items ...K) {
	for _, item := range items {
		s.Add(item)
	}
}

// Remove deletes item and reports whether it existed.
func (s *SkipListSet[K]) Remove(item K) bool {
	_, removed := s.sl.Remove(item)
	return removed
}

// Contains reports whether item is present.
func (s *SkipListSet[K]) Contains(item K) bool {
	return s.sl.ContainsKey(item)
}

// ContainsAll reports whether every item is present according to the comparator.
func (s *SkipListSet[K]) ContainsAll(items ...K) bool {
	for _, item := range items {
		if !s.Contains(item) {
			return false
		}
	}
	return true
}

// Len returns the number of elements.
func (s *SkipListSet[K]) Len() int { return s.sl.Len() }

// IsEmpty reports whether the set contains no elements.
func (s *SkipListSet[K]) IsEmpty() bool { return s.sl.Len() == 0 }

// Clear removes all elements.
func (s *SkipListSet[K]) Clear() { s.sl.Clear() }

// First returns the smallest element.
func (s *SkipListSet[K]) First() (K, bool) {
	k, _, ok := s.sl.First()
	return k, ok
}

// Last returns the greatest element.
func (s *SkipListSet[K]) Last() (K, bool) {
	k, _, ok := s.sl.Last()
	return k, ok
}

// Floor returns the greatest element <= e.
func (s *SkipListSet[K]) Floor(e K) (K, bool) {
	k, _, ok := s.sl.Floor(e)
	return k, ok
}

// Ceiling returns the smallest element >= e.
func (s *SkipListSet[K]) Ceiling(e K) (K, bool) {
	k, _, ok := s.sl.Ceiling(e)
	return k, ok
}

// Lower returns the greatest element < e.
func (s *SkipListSet[K]) Lower(e K) (K, bool) {
	k, _, ok := s.sl.Lower(e)
	return k, ok
}

// Higher returns the smallest element > e.
func (s *SkipListSet[K]) Higher(e K) (K, bool) {
	k, _, ok := s.sl.Higher(e)
	return k, ok
}

// All iterates over the elements in ascending order.
func (s *SkipListSet[K]) All() iter.Seq[K] {
	return func(yield func(K) bool) {
		for k := range s.sl.All() {
			if !yield(k) {
				return
			}
		}
	}
}

// Range iterates over the elements in [from, to], honoring the
// inclusivity flags, in ascending order.
func (s *SkipListSet[K]) Range(from, to K, fromInclusive, toInclusive bool) iter.Seq[K] {
	return func(yield func(K) bool) {
		for k := range s.sl.Range(from, to, fromInclusive, toInclusive) {
			if !yield(k) {
				return
			}
		}
	}
}

// ToSlice returns the elements in ascending order.
func (s *SkipListSet[K]) ToSlice() []K { return s.sl.Keys() }

// Clone returns a shallow copy containing the same elements.
func (s *SkipListSet[K]) Clone() *SkipListSet[K] {
	c := NewSkipListSetFunc(s.cmp)
	for k := range s.All() {
		c.Add(k)
	}
	return c
}

// Union returns a new sorted set with the elements of s and other.
func (s *SkipListSet[K]) Union(other Set[K]) *SkipListSet[K] {
	out := s.Clone()
	for item := range other.All() {
		out.Add(item)
	}
	return out
}

// Intersection returns a new sorted set with the elements present in both sets.
func (s *SkipListSet[K]) Intersection(other Set[K]) *SkipListSet[K] {
	out := NewSkipListSetFunc(s.cmp)
	for k := range s.All() {
		if other.Contains(k) {
			out.Add(k)
		}
	}
	return out
}

// Difference returns a new sorted set with the elements of s that are not in other.
func (s *SkipListSet[K]) Difference(other Set[K]) *SkipListSet[K] {
	out := NewSkipListSetFunc(s.cmp)
	for k := range s.All() {
		if !other.Contains(k) {
			out.Add(k)
		}
	}
	return out
}

// Equal reports whether both sets contain exactly the same elements.
func (s *SkipListSet[K]) Equal(other Set[K]) bool {
	if s.Len() != other.Len() {
		return false
	}
	for k := range s.All() {
		if !other.Contains(k) {
			return false
		}
	}
	return true
}
