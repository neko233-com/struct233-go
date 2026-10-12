// Package rangeset provides intervals and interval sets over ordered types,
// modelled after com.google.common.collect.Range and RangeSet.
//
// A Range is a single interval whose ends are closed, open or unbounded. A
// RangeSet holds a set of values as disjoint, ascending, coalesced ranges, so
// membership and set algebra stay cheap for huge spans.
//
//	s := rangeset.NewSet[int]()
//	s.Add(rangeset.Closed(1, 3))
//	s.Add(rangeset.Closed(3, 6)) // closed-touching: coalesces with [1,3]
//	fmt.Println(s.Spans())       // [[1,6]]
//	s.Remove(rangeset.Singleton(2))
//	fmt.Println(s.Spans())       // [[1,2) (2,6]]
//	fmt.Println(s.Contains(2))   // false
//
// Both types are single-threaded; they are not safe for concurrent use.
package rangeset

import (
	"cmp"
	"fmt"
	"iter"
	"slices"
	"sort"
	"strings"
)

// bound is one end of a Range. set reports whether the end is bounded; an
// unset lower end means -inf and an unset upper end +inf. Unbounded ends are
// flags rather than sentinel values of T, so every ordered type (strings
// included) works.
type bound[T cmp.Ordered] struct {
	val    T
	set    bool
	closed bool
}

// flip returns the end that excludes exactly the values beyond b: the outer
// half of a range split at b.
func flip[T cmp.Ordered](b bound[T]) bound[T] {
	return bound[T]{val: b.val, set: true, closed: !b.closed}
}

// minLower returns the lower of two lower ends. An unset end is -inf and
// therefore always the lower one; a tie prefers the closed end so that the
// result encloses both ranges.
func minLower[T cmp.Ordered](a, b bound[T]) bound[T] {
	if !a.set {
		return a
	}
	if !b.set {
		return b
	}
	if a.val < b.val {
		return a
	}
	if b.val < a.val {
		return b
	}
	return bound[T]{val: a.val, set: true, closed: a.closed || b.closed}
}

// maxUpper returns the upper of two upper ends. An unset end is +inf and
// therefore always the upper one; a tie prefers the closed end so that the
// result encloses both ranges.
func maxUpper[T cmp.Ordered](a, b bound[T]) bound[T] {
	if !a.set {
		return a
	}
	if !b.set {
		return b
	}
	if a.val > b.val {
		return a
	}
	if b.val > a.val {
		return b
	}
	return bound[T]{val: a.val, set: true, closed: a.closed || b.closed}
}

// maxLower returns the higher of two lower ends. A tie prefers the open end:
// only values in both ranges survive an intersection.
func maxLower[T cmp.Ordered](a, b bound[T]) bound[T] {
	if !a.set {
		return b
	}
	if !b.set {
		return a
	}
	if a.val > b.val {
		return a
	}
	if b.val > a.val {
		return b
	}
	return bound[T]{val: a.val, set: true, closed: a.closed && b.closed}
}

// minUpper returns the lower of two upper ends. A tie prefers the open end.
func minUpper[T cmp.Ordered](a, b bound[T]) bound[T] {
	if !a.set {
		return b
	}
	if !b.set {
		return a
	}
	if a.val < b.val {
		return a
	}
	if b.val < a.val {
		return b
	}
	return bound[T]{val: a.val, set: true, closed: a.closed && b.closed}
}

// gapped reports whether a lies strictly below b with at least one value of
// the order between them. Matching ends that are both open are gapped, which
// is what keeps (1,3) and (3,6) separate while [1,3] and (3,6] merge.
func gapped[T cmp.Ordered](a, b Range[T]) bool {
	if !a.hi.set || !b.lo.set {
		return false
	}
	if a.hi.val != b.lo.val {
		return a.hi.val < b.lo.val
	}
	return !a.hi.closed && !b.lo.closed
}

// Range is an interval over an ordered type whose ends are closed, open or
// unbounded (com.google.common.collect.Range).
//
// A range is empty when it holds no value at all: [3,1] and (1,1) are empty
// because their ends cross or coincide with an open end, while (-inf,+inf] is
// never empty. The zero value is All(), the range covering every value.
type Range[T cmp.Ordered] struct {
	lo bound[T]
	hi bound[T]
}

// Closed returns the range [lo, hi]. It is empty when lo > hi.
func Closed[T cmp.Ordered](lo, hi T) Range[T] {
	return Range[T]{
		lo: bound[T]{val: lo, set: true, closed: true},
		hi: bound[T]{val: hi, set: true, closed: true},
	}
}

// Open returns the range (lo, hi). It is empty when lo >= hi.
func Open[T cmp.Ordered](lo, hi T) Range[T] {
	return Range[T]{
		lo: bound[T]{val: lo, set: true},
		hi: bound[T]{val: hi, set: true},
	}
}

// OpenClosed returns the range (lo, hi]. It is empty when lo >= hi.
func OpenClosed[T cmp.Ordered](lo, hi T) Range[T] {
	return Range[T]{
		lo: bound[T]{val: lo, set: true},
		hi: bound[T]{val: hi, set: true, closed: true},
	}
}

// ClosedOpen returns the range [lo, hi). It is empty when lo >= hi.
func ClosedOpen[T cmp.Ordered](lo, hi T) Range[T] {
	return Range[T]{
		lo: bound[T]{val: lo, set: true, closed: true},
		hi: bound[T]{val: hi, set: true},
	}
}

// AtLeast returns the range [lo, +inf), which is never empty.
func AtLeast[T cmp.Ordered](lo T) Range[T] {
	return Range[T]{lo: bound[T]{val: lo, set: true, closed: true}}
}

// AtMost returns the range (-inf, hi], which is never empty.
func AtMost[T cmp.Ordered](hi T) Range[T] {
	return Range[T]{hi: bound[T]{val: hi, set: true, closed: true}}
}

// GreaterThan returns the range (lo, +inf), which is never empty.
func GreaterThan[T cmp.Ordered](lo T) Range[T] {
	return Range[T]{lo: bound[T]{val: lo, set: true}}
}

// LessThan returns the range (-inf, hi), which is never empty.
func LessThan[T cmp.Ordered](hi T) Range[T] {
	return Range[T]{hi: bound[T]{val: hi, set: true}}
}

// Singleton returns the range [v, v], which holds exactly v.
func Singleton[T cmp.Ordered](v T) Range[T] { return Closed(v, v) }

// All returns the range (-inf, +inf), which holds every value.
func All[T cmp.Ordered]() Range[T] { return Range[T]{} }

// IsEmpty reports whether r holds no value.
func (r Range[T]) IsEmpty() bool {
	if !r.lo.set || !r.hi.set {
		return false // an unbounded end always leaves something inside
	}
	if r.lo.val != r.hi.val {
		return r.lo.val > r.hi.val
	}
	return !r.lo.closed || !r.hi.closed
}

// Contains reports whether v is one of the values of r. An empty range
// contains nothing.
func (r Range[T]) Contains(v T) bool {
	if r.IsEmpty() {
		return false
	}
	if r.lo.set && (v < r.lo.val || (v == r.lo.val && !r.lo.closed)) {
		return false
	}
	if r.hi.set && (v > r.hi.val || (v == r.hi.val && !r.hi.closed)) {
		return false
	}
	return true
}

// coversLower reports whether r's lower end reaches at or below other's.
func (r Range[T]) coversLower(other Range[T]) bool {
	if !r.lo.set {
		return true
	}
	if !other.lo.set {
		return false
	}
	if r.lo.val != other.lo.val {
		return r.lo.val < other.lo.val
	}
	return r.lo.closed || !other.lo.closed
}

// coversUpper reports whether r's upper end reaches at or above other's.
func (r Range[T]) coversUpper(other Range[T]) bool {
	if !r.hi.set {
		return true
	}
	if !other.hi.set {
		return false
	}
	if r.hi.val != other.hi.val {
		return r.hi.val > other.hi.val
	}
	return r.hi.closed || !other.hi.closed
}

// Encloses reports whether r contains every value of other. Every range
// encloses an empty range.
func (r Range[T]) Encloses(other Range[T]) bool {
	if other.IsEmpty() {
		return true
	}
	if r.IsEmpty() {
		return false
	}
	return r.coversLower(other) && r.coversUpper(other)
}

// IsConnected reports whether r and other overlap or touch, so that their
// union is again a single range: [1,4) and [4,7) are connected, but (1,4) and
// (4,7) are not. Empty ranges are connected to nothing.
func (r Range[T]) IsConnected(other Range[T]) bool {
	if r.IsEmpty() || other.IsEmpty() {
		return false
	}
	return !gapped(r, other) && !gapped(other, r)
}

// Intersection returns the values held by both r and other. The returned range
// is empty and ok is false when the two ranges share no value.
func (r Range[T]) Intersection(other Range[T]) (Range[T], bool) {
	out := Range[T]{lo: maxLower(r.lo, other.lo), hi: minUpper(r.hi, other.hi)}
	// out is empty exactly when the tighter ends cross, or coincide with an
	// open end that was asked to be part of the result.
	return out, !out.IsEmpty()
}

// Span returns the smallest range enclosing both r and other. It panics if
// either range is empty.
func (r Range[T]) Span(other Range[T]) Range[T] {
	if r.IsEmpty() || other.IsEmpty() {
		panic("rangeset: span of empty range")
	}
	return Range[T]{lo: minLower(r.lo, other.lo), hi: maxUpper(r.hi, other.hi)}
}

// LowerBound returns the lower end of r: value is its position and closed
// reports whether value is part of r. ok is false when the end is unbounded or
// r is empty.
func (r Range[T]) LowerBound() (value T, closed bool, ok bool) {
	if r.IsEmpty() || !r.lo.set {
		return value, false, false
	}
	return r.lo.val, r.lo.closed, true
}

// UpperBound returns the upper end of r, in the same terms as LowerBound.
func (r Range[T]) UpperBound() (value T, closed bool, ok bool) {
	if r.IsEmpty() || !r.hi.set {
		return value, false, false
	}
	return r.hi.val, r.hi.closed, true
}

// Equal reports whether r and other hold the same values. Two empty ranges are
// always equal.
func (r Range[T]) Equal(other Range[T]) bool {
	if r.IsEmpty() || other.IsEmpty() {
		return r.IsEmpty() && other.IsEmpty()
	}
	return r == other
}

// String renders r in interval notation, e.g. "[1,3)", "(1,+inf)" or "()" for
// an empty range. Unbounded ends print as -inf and +inf.
func (r Range[T]) String() string {
	if r.IsEmpty() {
		return "()"
	}
	var b strings.Builder
	if r.lo.set && r.lo.closed {
		b.WriteByte('[')
	} else {
		b.WriteByte('(')
	}
	if r.lo.set {
		fmt.Fprintf(&b, "%v", r.lo.val)
	} else {
		b.WriteString("-inf")
	}
	b.WriteByte(',')
	if r.hi.set {
		fmt.Fprintf(&b, "%v", r.hi.val)
	} else {
		b.WriteString("+inf")
	}
	// the closing bracket follows the end it belongs to, so an unbounded upper
	// end always prints as a parenthesis
	if r.hi.set && r.hi.closed {
		b.WriteByte(']')
	} else {
		b.WriteByte(')')
	}
	return b.String()
}

// RangeSet is a set of values represented as disjoint, ascending, coalesced
// ranges (com.google.common.collect.RangeSet).
//
// Every stored span is non-empty, strictly ascending and separate from its
// neighbours, so a value belongs to at most one span and unordered ranges
// inserted in any order produce the same set. The zero value is an empty set.
// Not safe for concurrent use.
type RangeSet[T cmp.Ordered] struct {
	spans []Range[T]
}

// NewSet returns a new empty set.
func NewSet[T cmp.Ordered]() *RangeSet[T] { return &RangeSet[T]{} }

// Add adds every value of r. Empty ranges are ignored. A range overlapping or
// closed-touching a stored span coalesces with it, so [1,3] + [3,6] yield
// [1,6], while (1,3) + (3,6) stay split. Ranges that are merely adjacent, such
// as [1,3] and [4,6], also stay split: T may be dense, so the values between 3
// and 4 can exist and merging would claim them.
func (s *RangeSet[T]) Add(r Range[T]) {
	if r.IsEmpty() {
		return
	}
	// spans strictly below r keep their place, the run of spans that overlap
	// or touch r collapses into r's span, the rest is copied over. Because
	// stored spans are already coalesced and ascending, only a prefix lies
	// below r and only a suffix above it.
	i := 0
	for i < len(s.spans) && gapped(s.spans[i], r) {
		i++
	}
	j := i
	merged := r
	for j < len(s.spans) && !gapped(r, s.spans[j]) {
		merged = merged.Span(s.spans[j]) // both sides are non-empty here
		j++
	}
	tail := slices.Clone(s.spans[j:])
	s.spans = append(s.spans[:i], merged)
	s.spans = append(s.spans, tail...)
}

// Remove removes every value of r. Empty ranges and values not in the set are
// ignored; removing a middle range splits a span and trimming either end
// shortens it.
func (s *RangeSet[T]) Remove(r Range[T]) {
	if r.IsEmpty() || s.IsEmpty() {
		return
	}
	out := make([]Range[T], 0, len(s.spans)+1)
	for _, sp := range s.spans {
		out = append(out, subtract(sp, r)...)
	}
	s.spans = out
}

// subtract returns the parts of a that lie outside b.
func subtract[T cmp.Ordered](a, b Range[T]) []Range[T] {
	if _, ok := a.Intersection(b); !ok {
		return []Range[T]{a}
	}
	var out []Range[T]
	if b.lo.set {
		// left part: a up to the far side of b's lower end
		left := Range[T]{lo: a.lo, hi: flip(b.lo)}
		if !left.IsEmpty() {
			out = append(out, left)
		}
	}
	if b.hi.set {
		right := Range[T]{lo: flip(b.hi), hi: a.hi}
		if !right.IsEmpty() {
			out = append(out, right)
		}
	}
	return out
}

// Contains reports whether v is in the set.
func (s *RangeSet[T]) Contains(v T) bool {
	// spans are ascending and disjoint, so the first span whose upper end
	// reaches v is the only one that can hold it
	i := sort.Search(len(s.spans), func(i int) bool {
		hi := s.spans[i].hi
		return !hi.set || hi.val > v || (hi.val == v && hi.closed)
	})
	return i < len(s.spans) && s.spans[i].Contains(v)
}

// Encloses reports whether every value of r is in the set. An empty range is
// always enclosed. The set must cover r with a single span: spans are separate,
// so no two of them together span r.
func (s *RangeSet[T]) Encloses(r Range[T]) bool {
	if r.IsEmpty() {
		return true
	}
	for _, sp := range s.spans {
		if sp.Encloses(r) {
			return true
		}
	}
	return false
}

// EnclosesAll reports whether every value of other is in s.
func (s *RangeSet[T]) EnclosesAll(other *RangeSet[T]) bool {
	for _, sp := range other.spans {
		if !s.Encloses(sp) {
			return false
		}
	}
	return true
}

// Intersects reports whether s and other share at least one value. Spans that
// only touch do not count, since they share no value.
func (s *RangeSet[T]) Intersects(other *RangeSet[T]) bool {
	for _, a := range s.spans {
		for _, b := range other.spans {
			if _, ok := a.Intersection(b); ok {
				return true
			}
		}
	}
	return false
}

// Intersection returns the values present in both s and other. Neither set is
// modified.
func (s *RangeSet[T]) Intersection(other *RangeSet[T]) *RangeSet[T] {
	out := NewSet[T]()
	for _, a := range s.spans {
		for _, b := range other.spans {
			if x, ok := a.Intersection(b); ok {
				out.Add(x)
			}
		}
	}
	return out
}

// Union returns the values present in s or other. Neither set is modified.
func (s *RangeSet[T]) Union(other *RangeSet[T]) *RangeSet[T] {
	out := NewSet[T]()
	for _, sp := range s.spans {
		out.Add(sp)
	}
	for _, sp := range other.spans {
		out.Add(sp)
	}
	return out
}

// Difference returns the values present in s but not in other. s is not
// modified.
func (s *RangeSet[T]) Difference(other *RangeSet[T]) *RangeSet[T] {
	out := NewSet[T]()
	out.spans = append(out.spans, s.spans...)
	for _, sp := range other.spans {
		out.Remove(sp)
	}
	return out
}

// Complement returns the values not present in s. Filling the gaps between
// spans introduces unbounded ends whenever s is bounded, and the complement of
// an empty set is All().
func (s *RangeSet[T]) Complement() *RangeSet[T] {
	out := NewSet[T]()
	if s.IsEmpty() {
		out.Add(All[T]())
		return out
	}
	lower := bound[T]{} // the first gap starts at -inf
	for _, sp := range s.spans {
		if sp.lo.set {
			out.Add(Range[T]{lo: lower, hi: flip(sp.lo)})
		}
		if !sp.hi.set {
			return out // s reaches +inf, nothing lies above it
		}
		lower = flip(sp.hi)
	}
	out.Add(Range[T]{lo: lower}) // trailing gap up to +inf
	return out
}

// Spans returns a copy of the stored ranges, ascending and disjoint. It
// returns nil for an empty set, and mutating the result leaves the set alone.
func (s *RangeSet[T]) Spans() []Range[T] { return slices.Clone(s.spans) }

// Len returns the number of disjoint ranges stored, 0 for an empty set. It
// counts ranges rather than values, so it stays finite for unbounded sets.
func (s *RangeSet[T]) Len() int { return len(s.spans) }

// IsEmpty reports whether the set holds no value.
func (s *RangeSet[T]) IsEmpty() bool { return len(s.spans) == 0 }

// Clear removes every value while keeping the allocated span slice.
func (s *RangeSet[T]) Clear() { s.spans = s.spans[:0] }

// All iterates over the stored ranges in ascending order.
func (s *RangeSet[T]) All() iter.Seq[Range[T]] {
	return func(yield func(Range[T]) bool) {
		for _, sp := range s.spans {
			if !yield(sp) {
				return
			}
		}
	}
}
