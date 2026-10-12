// Package order provides comparator combinators, the Go answer to
// com.google.common.collect.Ordering.
//
// A Comparator follows the same contract as the containers in this module
// (negative, zero, positive), and is a plain func value, so the results plug
// straight into NewSortedSetFunc, NewSkipListFunc, NewPriorityQueueFunc and
// NewConcurrentSkipListFunc.
//
//	byScore := order.Reverse(order.Natural[int]())
//	ranking := struct233.NewSortedSetFunc(byScore, 10, 30, 20) // 30, 20, 10
//
//	byNameThenAge := order.Then(
//		order.ByKey(func(p Player) string { return p.Name }, order.Natural[string]()),
//		order.ByKey(func(p Player) int { return p.Age }, order.Natural[int]()),
//	)
//
// Every combinator returns a new Comparator and never mutates its arguments, so
// they can be composed freely. A derived Comparator is safe for concurrent use
// as long as the comparators and key functions it wraps are.
package order

import (
	"cmp"
	"slices"

	"github.com/neko233-com/struct233-go/skiplist"
)

// Comparator orders two values: negative when a < b, zero when a == b and
// positive when a > b (the same contract as java.util.Comparator). It is an
// alias of the container type, so an order.Comparator can be passed to any
// container factory without conversion.
type Comparator[T any] = skiplist.Comparator[T]

// Natural returns the comparator for the natural ordering of T.
func Natural[T cmp.Ordered]() Comparator[T] {
	return cmp.Compare[T]
}

// Reverse returns the comparator that orders values the other way round.
func Reverse[T any](cmp Comparator[T]) Comparator[T] {
	return func(a, b T) int { return -sign(cmp(a, b)) }
}

// Then returns a lexicographic comparator: the first comparator decides, and the
// next one breaks its ties. With no comparators the result reports every pair as
// equal.
func Then[T any](cmps ...Comparator[T]) Comparator[T] {
	switch len(cmps) {
	case 0:
		return func(T, T) int { return 0 }
	case 1:
		return func(a, b T) int { return sign(cmps[0](a, b)) }
	}
	return func(a, b T) int {
		for _, cmp := range cmps {
			if c := sign(cmp(a, b)); c != 0 {
				return c
			}
		}
		return 0
	}
}

// NullsFirst orders nil pointers before every non-nil value and compares two
// non-nil values with cmp.
func NullsFirst[T any](cmp Comparator[T]) Comparator[*T] {
	return func(a, b *T) int {
		switch {
		case a == nil && b == nil:
			return 0
		case a == nil:
			return -1
		case b == nil:
			return 1
		}
		return sign(cmp(*a, *b))
	}
}

// NullsLast orders nil pointers after every non-nil value and compares two
// non-nil values with cmp.
func NullsLast[T any](cmp Comparator[T]) Comparator[*T] {
	return func(a, b *T) int {
		switch {
		case a == nil && b == nil:
			return 0
		case a == nil:
			return 1
		case b == nil:
			return -1
		}
		return sign(cmp(*a, *b))
	}
}

// ByKey compares two values by a key extracted from each one: the key function
// runs once per operand, so it should be cheap.
func ByKey[T any, K any](key func(T) K, cmp Comparator[K]) Comparator[T] {
	return func(a, b T) int { return sign(cmp(key(a), key(b))) }
}

// By adapts a less function (the slices.SortFunc direction) to a Comparator.
func By[T any](less func(a, b T) bool) Comparator[T] {
	return func(a, b T) int {
		switch {
		case less(a, b):
			return -1
		case less(b, a):
			return 1
		default:
			return 0
		}
	}
}

// Max returns the greater of a and b, preferring a on a tie.
func Max[T any](cmp Comparator[T], a, b T) T {
	if cmp(a, b) >= 0 {
		return a
	}
	return b
}

// Min returns the smaller of a and b, preferring a on a tie.
func Min[T any](cmp Comparator[T], a, b T) T {
	if cmp(a, b) <= 0 {
		return a
	}
	return b
}

// MaxOf returns the greatest element of items; ok is false when items is empty.
// Ties keep the first occurrence.
func MaxOf[T any](cmp Comparator[T], items []T) (max T, ok bool) {
	for i, item := range items {
		if i == 0 || cmp(item, max) > 0 {
			max, ok = item, true
		}
	}
	return max, ok
}

// MinOf returns the smallest element of items; ok is false when items is empty.
// Ties keep the first occurrence.
func MinOf[T any](cmp Comparator[T], items []T) (min T, ok bool) {
	for i, item := range items {
		if i == 0 || cmp(item, min) < 0 {
			min, ok = item, true
		}
	}
	return min, ok
}

// Sorted returns a copy of items ordered by cmp, leaving the input untouched.
// The copy is stable, so equal elements keep their relative order.
func Sorted[T any](cmp Comparator[T], items []T) []T {
	out := slices.Clone(items)
	slices.SortStableFunc(out, cmp)
	return out
}

// IsSorted reports whether items are already in the order cmp describes. Empty
// and single-element slices are sorted.
func IsSorted[T any](cmp Comparator[T], items []T) bool {
	for i := 1; i < len(items); i++ {
		if cmp(items[i-1], items[i]) > 0 {
			return false
		}
	}
	return true
}

// sign normalises a comparator result to -1, 0 or +1 so combining comparators
// can never overflow.
func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}
