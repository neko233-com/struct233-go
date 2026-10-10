// Package set provides Set implementations modeled after the JDK:
// HashSet (unordered), LinkedHashSet (insertion-ordered) and
// TreeSet (sorted, backed by a skip list).
//
// None of these types are safe for concurrent use.
package set

import "iter"

// HashSet is an unordered set backed by a Go map,
// similar to java.util.HashSet.
type HashSet[K comparable] struct {
	m map[K]struct{}
}

// NewHashSet creates a HashSet pre-filled with items.
func NewHashSet[K comparable](items ...K) *HashSet[K] {
	return NewHashSetWithCapacity(len(items), items...)
}

// NewHashSetWithCapacity creates a HashSet whose map is pre-sized for
// capacity elements, avoiding rehashing while filling it.
func NewHashSetWithCapacity[K comparable](capacity int, items ...K) *HashSet[K] {
	s := &HashSet[K]{m: make(map[K]struct{}, capacity)}
	s.AddAll(items...)
	return s
}

// Add inserts item and reports whether it was newly added.
func (s *HashSet[K]) Add(item K) bool {
	if _, ok := s.m[item]; ok {
		return false
	}
	s.m[item] = struct{}{}
	return true
}

// AddAll inserts every item.
func (s *HashSet[K]) AddAll(items ...K) {
	for _, item := range items {
		s.Add(item)
	}
}

// Remove deletes item and reports whether it existed.
func (s *HashSet[K]) Remove(item K) bool {
	if _, ok := s.m[item]; !ok {
		return false
	}
	delete(s.m, item)
	return true
}

// Contains reports whether item is present.
func (s *HashSet[K]) Contains(item K) bool {
	_, ok := s.m[item]
	return ok
}

// ContainsAll reports whether every item is present.
func (s *HashSet[K]) ContainsAll(items ...K) bool {
	for _, item := range items {
		if !s.Contains(item) {
			return false
		}
	}
	return true
}

// Len returns the number of elements.
func (s *HashSet[K]) Len() int { return len(s.m) }

// Clear removes all elements.
func (s *HashSet[K]) Clear() { clear(s.m) }

// Clone returns a shallow copy containing the same elements.
func (s *HashSet[K]) Clone() *HashSet[K] {
	c := NewHashSet[K]()
	for item := range s.m {
		c.Add(item)
	}
	return c
}

// All iterates over the elements in unspecified order.
func (s *HashSet[K]) All() iter.Seq[K] {
	return func(yield func(K) bool) {
		for item := range s.m {
			if !yield(item) {
				return
			}
		}
	}
}

// ToSlice returns the elements in unspecified order.
func (s *HashSet[K]) ToSlice() []K {
	out := make([]K, 0, len(s.m))
	for item := range s.m {
		out = append(out, item)
	}
	return out
}

// Equal reports whether both sets contain exactly the same elements.
func (s *HashSet[K]) Equal(other *HashSet[K]) bool {
	if s.Len() != other.Len() {
		return false
	}
	for item := range s.m {
		if !other.Contains(item) {
			return false
		}
	}
	return true
}

// Union returns a new set with the elements of s and other.
func (s *HashSet[K]) Union(other *HashSet[K]) *HashSet[K] {
	out := NewHashSetWithCapacity[K](s.Len() + other.Len())
	for item := range s.m {
		out.Add(item)
	}
	for item := range other.m {
		out.Add(item)
	}
	return out
}

// Intersection returns a new set with the elements present in both sets.
func (s *HashSet[K]) Intersection(other *HashSet[K]) *HashSet[K] {
	out := NewHashSetWithCapacity[K](min(s.Len(), other.Len()))
	for item := range s.m {
		if other.Contains(item) {
			out.Add(item)
		}
	}
	return out
}

// Difference returns a new set with the elements of s that are not in other.
func (s *HashSet[K]) Difference(other *HashSet[K]) *HashSet[K] {
	out := NewHashSetWithCapacity[K](s.Len())
	for item := range s.m {
		if !other.Contains(item) {
			out.Add(item)
		}
	}
	return out
}

// SymmetricDifference returns a new set with the elements that are in
// exactly one of the two sets.
func (s *HashSet[K]) SymmetricDifference(other *HashSet[K]) *HashSet[K] {
	out := NewHashSetWithCapacity[K](s.Len() + other.Len())
	for item := range s.m {
		if !other.Contains(item) {
			out.Add(item)
		}
	}
	for item := range other.m {
		if !s.Contains(item) {
			out.Add(item)
		}
	}
	return out
}

// IsSubsetOf reports whether every element of s is present in other.
func (s *HashSet[K]) IsSubsetOf(other *HashSet[K]) bool {
	if s.Len() > other.Len() {
		return false
	}
	for item := range s.m {
		if !other.Contains(item) {
			return false
		}
	}
	return true
}

// IsSupersetOf reports whether every element of other is present in s.
func (s *HashSet[K]) IsSupersetOf(other *HashSet[K]) bool {
	return other.IsSubsetOf(s)
}
