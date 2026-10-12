package set

import "iter"

// Set is a collection of distinct elements. Each implementation defines its
// equality and iteration order: hash sets use Go equality, while SkipListSet
// uses its comparator. Set algebra requires operands with compatible equality.
//
// ContainsAll with no arguments returns true. ToSlice returns a detached slice;
// the elements themselves are shallow copies. Mutating a set during iteration
// is unsupported unless an implementation explicitly permits it. Concurrency
// guarantees belong to each implementation, not to this interface.
type Set[T any] interface {
	// Add inserts an element and reports whether membership changed.
	Add(T) bool
	// AddAll inserts all elements, ignoring duplicates.
	AddAll(...T)
	// Remove deletes an element and reports whether it was present.
	Remove(T) bool
	// Contains reports whether an element is present.
	Contains(T) bool
	// ContainsAll reports whether every argument is present.
	ContainsAll(...T) bool
	// Len returns the number of distinct elements.
	Len() int
	// IsEmpty reports whether the set contains no elements.
	IsEmpty() bool
	// Clear removes every element and leaves the set reusable.
	Clear()
	// All iterates in the implementation's documented order.
	All() iter.Seq[T]
	// ToSlice returns a detached slice in iteration order.
	ToSlice() []T
}

// NavigableSet is a Set whose elements follow an ordering. First and Last
// return its smallest and greatest elements; Floor/Ceiling include an equal
// element, while Lower/Higher require a strict predecessor/successor. The bool
// result reports whether an element exists. Range follows the same ordering
// and honors each boundary's inclusivity flag.
type NavigableSet[T any] interface {
	Set[T]
	// First returns the least element in comparator order.
	First() (T, bool)
	// Last returns the greatest element in comparator order.
	Last() (T, bool)
	// Floor returns the greatest element not above the argument.
	Floor(T) (T, bool)
	// Ceiling returns the least element not below the argument.
	Ceiling(T) (T, bool)
	// Lower returns the strict predecessor of the argument.
	Lower(T) (T, bool)
	// Higher returns the strict successor of the argument.
	Higher(T) (T, bool)
	// Range iterates between the specified bounds in comparator order.
	Range(from, to T, fromInclusive, toInclusive bool) iter.Seq[T]
}

var (
	_ Set[int]            = (*HashSet[int])(nil)
	_ Set[int]            = (*LinkedHashSet[int])(nil)
	_ NavigableSet[int]   = (*SkipListSet[int])(nil)
	_ NavigableSet[[]int] = (*SkipListSet[[]int])(nil)
)
