package set

import (
	"cmp"
	"iter"

	"github.com/neko233-com/struct233-go/skiplist"
)

// TreeSet is a set that keeps its elements sorted,
// similar to java.util.TreeSet. It is backed by a skip list.
type TreeSet[K any] struct {
	cmp skiplist.Comparator[K]
	sl  *skiplist.SkipList[K, struct{}]
}

// NewTreeSet creates a TreeSet ordered by the natural ordering of K.
func NewTreeSet[K cmp.Ordered](items ...K) *TreeSet[K] {
	ts := &TreeSet[K]{cmp: cmp.Compare[K], sl: skiplist.New[K, struct{}]()}
	ts.AddAll(items...)
	return ts
}

// NewTreeSetFunc creates a TreeSet ordered by a custom comparator.
// It panics if compare is nil.
func NewTreeSetFunc[K any](compare skiplist.Comparator[K], items ...K) *TreeSet[K] {
	if compare == nil {
		panic("set: nil comparator")
	}
	ts := &TreeSet[K]{cmp: compare, sl: skiplist.NewFunc[K, struct{}](compare)}
	ts.AddAll(items...)
	return ts
}

// Add inserts item and reports whether it was newly added.
func (s *TreeSet[K]) Add(item K) bool {
	_, existed := s.sl.Put(item, struct{}{})
	return !existed
}

// AddAll inserts every item.
func (s *TreeSet[K]) AddAll(items ...K) {
	for _, item := range items {
		s.Add(item)
	}
}

// Remove deletes item and reports whether it existed.
func (s *TreeSet[K]) Remove(item K) bool {
	_, removed := s.sl.Remove(item)
	return removed
}

// Contains reports whether item is present.
func (s *TreeSet[K]) Contains(item K) bool {
	return s.sl.ContainsKey(item)
}

// Len returns the number of elements.
func (s *TreeSet[K]) Len() int { return s.sl.Len() }

// Clear removes all elements.
func (s *TreeSet[K]) Clear() { s.sl.Clear() }

// First returns the smallest element.
func (s *TreeSet[K]) First() (K, bool) {
	k, _, ok := s.sl.First()
	return k, ok
}

// Last returns the greatest element.
func (s *TreeSet[K]) Last() (K, bool) {
	k, _, ok := s.sl.Last()
	return k, ok
}

// Floor returns the greatest element <= e.
func (s *TreeSet[K]) Floor(e K) (K, bool) {
	k, _, ok := s.sl.Floor(e)
	return k, ok
}

// Ceiling returns the smallest element >= e.
func (s *TreeSet[K]) Ceiling(e K) (K, bool) {
	k, _, ok := s.sl.Ceiling(e)
	return k, ok
}

// Lower returns the greatest element < e.
func (s *TreeSet[K]) Lower(e K) (K, bool) {
	k, _, ok := s.sl.Lower(e)
	return k, ok
}

// Higher returns the smallest element > e.
func (s *TreeSet[K]) Higher(e K) (K, bool) {
	k, _, ok := s.sl.Higher(e)
	return k, ok
}

// All iterates over the elements in ascending order.
func (s *TreeSet[K]) All() iter.Seq[K] {
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
func (s *TreeSet[K]) Range(from, to K, fromInclusive, toInclusive bool) iter.Seq[K] {
	return func(yield func(K) bool) {
		for k := range s.sl.Range(from, to, fromInclusive, toInclusive) {
			if !yield(k) {
				return
			}
		}
	}
}

// ToSlice returns the elements in ascending order.
func (s *TreeSet[K]) ToSlice() []K { return s.sl.Keys() }

// Clone returns a shallow copy containing the same elements.
func (s *TreeSet[K]) Clone() *TreeSet[K] {
	c := NewTreeSetFunc(s.cmp)
	for k := range s.All() {
		c.Add(k)
	}
	return c
}

// Union returns a new sorted set with the elements of s and other.
func (s *TreeSet[K]) Union(other *TreeSet[K]) *TreeSet[K] {
	out := s.Clone()
	out.AddAll(other.ToSlice()...)
	return out
}

// Intersection returns a new sorted set with the elements present in both sets.
func (s *TreeSet[K]) Intersection(other *TreeSet[K]) *TreeSet[K] {
	out := NewTreeSetFunc(s.cmp)
	for k := range s.All() {
		if other.Contains(k) {
			out.Add(k)
		}
	}
	return out
}

// Difference returns a new sorted set with the elements of s that are not in other.
func (s *TreeSet[K]) Difference(other *TreeSet[K]) *TreeSet[K] {
	out := NewTreeSetFunc(s.cmp)
	for k := range s.All() {
		if !other.Contains(k) {
			out.Add(k)
		}
	}
	return out
}

// Equal reports whether both sets contain exactly the same elements.
func (s *TreeSet[K]) Equal(other *TreeSet[K]) bool {
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
