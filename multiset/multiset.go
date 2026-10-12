// Package multiset provides a counting set
// (com.google.common.collect.Multiset) whose iteration order follows first
// insertion.
//
// A multiset stores each distinct element together with how many times it
// occurs; the order in which distinct elements are first added is preserved, so
// Elements, DistinctElements and MostCommon are deterministic.
//
//	m := multiset.New("a", "b", "a")
//	m.Add("a")             // "a" now occurs 3 times
//	m.SetCount("b", 2)     // exact counts: a=3, b=2
//	elem, n, ok := m.MostCommon()
//	fmt.Println(elem, n, ok)          // a 3 true
//	fmt.Println(m.DistinctElements()) // [a b]
//	fmt.Println(m.Elements())         // [a a a b b]
//
// Removing the last occurrence of an element drops it from the map and from the
// insertion order; adding it again re-inserts it at the end of that order.
// Counts returns a copy and never a live view, and Equal compares counts only,
// ignoring insertion order.
//
// Not safe for concurrent use: the concurrent package owns the goroutine-safe
// variants.
package multiset

import (
	"iter"
	"maps"
	"math"
)

// Multiset counts how many times each element occurs
// (com.google.common.collect.Multiset): order comes from first insertion, so
// Elements and DistinctElements are deterministic.
type Multiset[T comparable] struct {
	counts map[T]int
	order  []T // distinct elements in first-insertion order
	total  int // maintained with counts; never exceeds math.MaxInt
}

// New creates a multiset, optionally pre-filled with elems (each one counted
// once).
func New[T comparable](elems ...T) *Multiset[T] {
	m := &Multiset[T]{counts: make(map[T]int, len(elems))}
	for _, elem := range elems {
		m.Add(elem)
	}
	return m
}

// NewWithCapacity creates an empty multiset whose map is pre-sized for capacity
// distinct elements. A negative capacity is treated as zero.
func NewWithCapacity[T comparable](capacity int, elems ...T) *Multiset[T] {
	if capacity < 0 {
		capacity = 0
	}
	m := &Multiset[T]{counts: make(map[T]int, capacity)}
	for _, elem := range elems {
		m.Add(elem)
	}
	return m
}

// Add counts one more occurrence of elem and returns the new count. It panics
// without changing the multiset when the total would overflow int.
func (m *Multiset[T]) Add(elem T) int {
	m.addN(elem, 1)
	return m.counts[elem]
}

// AddN counts n more occurrences of elem and returns the new count. It panics
// when n is negative or the total would overflow int, without changing state.
func (m *Multiset[T]) AddN(elem T, n int) int {
	if n < 0 {
		panic("multiset: negative count")
	}
	if n == 0 {
		// elem stays absent: a zero count is never stored
		return m.counts[elem]
	}
	m.addN(elem, n)
	return m.counts[elem]
}

// Remove counts one occurrence of elem away (down to zero) and returns the new
// count.
func (m *Multiset[T]) Remove(elem T) int {
	return m.RemoveN(elem, 1)
}

// RemoveN counts n occurrences of elem away and returns the new count. It panics
// when n is negative.
func (m *Multiset[T]) RemoveN(elem T, n int) int {
	if n < 0 {
		panic("multiset: negative count")
	}
	cur := m.counts[elem]
	if n >= cur {
		// also covers n == 0 on an already absent element
		m.drop(elem)
		return 0
	}
	m.counts[elem] = cur - n
	m.total -= n
	return cur - n
}

// RemoveAll drops every occurrence of elem and returns the removed count.
func (m *Multiset[T]) RemoveAll(elem T) int {
	n := m.counts[elem]
	if n == 0 {
		return 0
	}
	m.drop(elem)
	return n
}

// SetCount stores an exact count for elem, dropping the element when n is zero.
// It panics without changing state when n is negative or the total would
// overflow int.
func (m *Multiset[T]) SetCount(elem T, n int) {
	if n < 0 {
		panic("multiset: negative count")
	}
	_, present := m.counts[elem]
	if n == 0 {
		if present {
			m.drop(elem)
		}
		return
	}
	delta := n - m.counts[elem]
	if delta > math.MaxInt-m.total {
		panic("multiset: total count overflow")
	}
	if !present {
		m.order = append(m.order, elem)
	}
	m.counts[elem] = n
	m.total += delta
}

// Count returns how many times elem occurs.
func (m *Multiset[T]) Count(elem T) int {
	return m.counts[elem]
}

// Contains reports whether elem occurs at least once.
func (m *Multiset[T]) Contains(elem T) bool {
	n, ok := m.counts[elem]
	return ok && n > 0
}

// TotalCount returns the sum of all counts in O(1) time.
func (m *Multiset[T]) TotalCount() int {
	return m.total
}

// Distinct returns how many different elements are stored.
func (m *Multiset[T]) Distinct() int {
	return len(m.counts)
}

// IsEmpty reports whether nothing is stored.
func (m *Multiset[T]) IsEmpty() bool {
	return len(m.counts) == 0
}

// Elements returns one entry per occurrence, grouped by first insertion order.
func (m *Multiset[T]) Elements() []T {
	out := make([]T, 0, m.TotalCount())
	for _, elem := range m.order {
		for range m.counts[elem] {
			out = append(out, elem)
		}
	}
	return out
}

// DistinctElements returns the distinct elements in first insertion order.
func (m *Multiset[T]) DistinctElements() []T {
	out := make([]T, len(m.order))
	copy(out, m.order)
	return out
}

// Counts returns a copy of the element to count mapping.
func (m *Multiset[T]) Counts() map[T]int {
	return maps.Clone(m.counts)
}

// MostCommon returns the element with the highest count (ties are broken by
// first insertion order). ok is false when the multiset is empty.
func (m *Multiset[T]) MostCommon() (elem T, count int, ok bool) {
	for _, e := range m.order {
		if n := m.counts[e]; n > count {
			elem, count, ok = e, n, true
		}
	}
	return elem, count, ok
}

// Equal reports whether both multisets hold exactly the same counts. Insertion
// order is ignored, and a nil other is never equal.
func (m *Multiset[T]) Equal(other *Multiset[T]) bool {
	if other == nil {
		return false
	}
	if len(m.counts) != len(other.counts) {
		return false
	}
	for elem, n := range m.counts {
		if other.counts[elem] != n {
			return false
		}
	}
	return true
}

// Clear removes every element.
func (m *Multiset[T]) Clear() {
	clear(m.counts)
	clear(m.order)
	m.order = m.order[:0]
	m.total = 0
}

// All iterates one element per occurrence, grouped by first insertion order.
func (m *Multiset[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, elem := range m.order {
			for range m.counts[elem] {
				if !yield(elem) {
					return
				}
			}
		}
	}
}

// addN bumps the count of elem, recording a first sighting in the order slice.
func (m *Multiset[T]) addN(elem T, n int) {
	if n > math.MaxInt-m.total {
		panic("multiset: total count overflow")
	}
	if _, ok := m.counts[elem]; !ok {
		m.order = append(m.order, elem)
	}
	m.counts[elem] += n
	m.total += n
}

// drop deletes elem from the map and compacts the insertion order in place.
func (m *Multiset[T]) drop(elem T) {
	if _, ok := m.counts[elem]; !ok {
		return
	}
	m.total -= m.counts[elem]
	delete(m.counts, elem)
	for i, e := range m.order {
		if e == elem {
			copy(m.order[i:], m.order[i+1:])
			var zero T
			m.order[len(m.order)-1] = zero
			m.order = m.order[:len(m.order)-1]
			return
		}
	}
}
