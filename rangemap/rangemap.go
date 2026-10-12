// Package rangemap provides a map from non-overlapping intervals to values,
// modelled after com.google.common.collect.RangeMap.
//
// Put replaces the value in an interval and preserves the portions of older
// mappings outside it. The stored intervals remain non-empty, sorted and
// disjoint. A RangeMap is not safe for concurrent use.
package rangemap

import (
	"cmp"
	"iter"
	"sort"

	"github.com/neko233-com/struct233-go/rangeset"
)

// Entry is one interval and the value assigned to it.
type Entry[K cmp.Ordered, V any] struct {
	// Range is the interval the value is assigned to.
	Range rangeset.Range[K]
	// Value is the value assigned to the interval.
	Value V
}

// RangeMap maps disjoint ranges to values.
//
// The zero value is ready to use. Entries are stored in ascending lower-bound
// order, and no two entries overlap.
type RangeMap[K cmp.Ordered, V any] struct {
	entries []Entry[K, V]
}

// New creates an empty RangeMap.
func New[K cmp.Ordered, V any]() *RangeMap[K, V] { return &RangeMap[K, V]{} }

// Put maps r to value, replacing any existing values in r. Existing entries
// are split at r's boundaries as needed. Empty ranges are ignored.
func (m *RangeMap[K, V]) Put(r rangeset.Range[K], value V) {
	if r.IsEmpty() {
		return
	}
	out := make([]Entry[K, V], 0, len(m.entries)+2)
	for _, entry := range m.entries {
		out = appendRemainder(out, entry, r)
	}
	i := sort.Search(len(out), func(i int) bool { return !less(out[i].Range, r) })
	out = append(out, Entry[K, V]{})
	copy(out[i+1:], out[i:])
	out[i] = Entry[K, V]{Range: r, Value: value}
	m.entries = out
}

// PutAll copies every mapping from other, replacing any overlapping mappings
// in this map. A nil source is ignored.
func (m *RangeMap[K, V]) PutAll(other *RangeMap[K, V]) {
	if other == nil {
		return
	}
	for _, entry := range other.Entries() {
		m.Put(entry.Range, entry.Value)
	}
}

// Get returns the value mapped to key.
func (m *RangeMap[K, V]) Get(key K) (V, bool) {
	entry, ok := m.GetEntry(key)
	if !ok {
		var zero V
		return zero, false
	}
	return entry.Value, true
}

// GetEntry returns the mapped interval and value containing key.
func (m *RangeMap[K, V]) GetEntry(key K) (Entry[K, V], bool) {
	// Find the first interval whose lower bound is strictly after key. The
	// preceding interval is the only possible match because entries do not
	// overlap.
	i := sort.Search(len(m.entries), func(i int) bool {
		lower, closed, ok := m.entries[i].Range.LowerBound()
		if !ok { // an unbounded lower end precedes every key
			return false
		}
		return lower > key || (lower == key && !closed)
	})
	if i == 0 {
		return Entry[K, V]{}, false
	}
	entry := m.entries[i-1]
	if !entry.Range.Contains(key) {
		return Entry[K, V]{}, false
	}
	return entry, true
}

// Remove deletes every mapping in r, keeping any old portions outside r.
func (m *RangeMap[K, V]) Remove(r rangeset.Range[K]) {
	if r.IsEmpty() || len(m.entries) == 0 {
		return
	}
	out := make([]Entry[K, V], 0, len(m.entries)+1)
	for _, entry := range m.entries {
		out = appendRemainder(out, entry, r)
	}
	m.entries = out
}

// appendRemainder preserves the ordered portions outside removed without
// constructing a temporary RangeSet and span slices for each entry.
func appendRemainder[K cmp.Ordered, V any](out []Entry[K, V], entry Entry[K, V], removed rangeset.Range[K]) []Entry[K, V] {
	if _, ok := entry.Range.Intersection(removed); !ok {
		return append(out, entry)
	}
	if lo, closed, ok := removed.LowerBound(); ok {
		left := rangeset.AtMost(lo)
		if closed {
			left = rangeset.LessThan(lo)
		}
		if part, ok := entry.Range.Intersection(left); ok {
			out = append(out, Entry[K, V]{Range: part, Value: entry.Value})
		}
	}
	if hi, closed, ok := removed.UpperBound(); ok {
		right := rangeset.AtLeast(hi)
		if closed {
			right = rangeset.GreaterThan(hi)
		}
		if part, ok := entry.Range.Intersection(right); ok {
			out = append(out, Entry[K, V]{Range: part, Value: entry.Value})
		}
	}
	return out
}

// SubRangeMap returns a detached map containing only mappings within r.
func (m *RangeMap[K, V]) SubRangeMap(r rangeset.Range[K]) *RangeMap[K, V] {
	out := New[K, V]()
	if r.IsEmpty() {
		return out
	}
	for _, entry := range m.entries {
		if intersection, ok := entry.Range.Intersection(r); ok {
			out.entries = append(out.entries, Entry[K, V]{Range: intersection, Value: entry.Value})
		}
	}
	return out
}

// Span returns the smallest interval enclosing every mapped interval. ok is
// false when the map has no entries.
func (m *RangeMap[K, V]) Span() (rangeset.Range[K], bool) {
	if len(m.entries) == 0 {
		return rangeset.Range[K]{}, false
	}
	return m.entries[0].Range.Span(m.entries[len(m.entries)-1].Range), true
}

// Entries returns a detached slice of mappings in ascending lower-bound order.
func (m *RangeMap[K, V]) Entries() []Entry[K, V] {
	return append([]Entry[K, V](nil), m.entries...)
}

// AsMapOfRanges returns a detached map keyed by the mapped intervals.
func (m *RangeMap[K, V]) AsMapOfRanges() map[rangeset.Range[K]]V {
	out := make(map[rangeset.Range[K]]V, len(m.entries))
	for _, entry := range m.entries {
		out[entry.Range] = entry.Value
	}
	return out
}

// All iterates over mappings in ascending lower-bound order. Mutating the map
// during iteration is unsupported.
func (m *RangeMap[K, V]) All() iter.Seq2[rangeset.Range[K], V] {
	return func(yield func(rangeset.Range[K], V) bool) {
		for _, entry := range m.entries {
			if !yield(entry.Range, entry.Value) {
				return
			}
		}
	}
}

// Len returns the number of disjoint mapped intervals.
func (m *RangeMap[K, V]) Len() int { return len(m.entries) }

// IsEmpty reports whether no interval is mapped.
func (m *RangeMap[K, V]) IsEmpty() bool { return len(m.entries) == 0 }

// Clear removes every mapping while keeping the allocated entry buffer.
func (m *RangeMap[K, V]) Clear() {
	clear(m.entries)
	m.entries = m.entries[:0]
}

// less orders ranges by their lower bound, with unbounded starts first and
// closed bounds before open bounds at the same value.
func less[K cmp.Ordered](a, b rangeset.Range[K]) bool {
	aValue, aClosed, aOK := a.LowerBound()
	bValue, bClosed, bOK := b.LowerBound()
	if !aOK || !bOK {
		return !aOK && bOK
	}
	if aValue != bValue {
		return aValue < bValue
	}
	return aClosed && !bClosed
}
