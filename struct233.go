// Package struct233 is the unified facade of go-data-struct-lib: all
// containers are reachable from a single import.
//
//	import "github.com/neko233-com/go-data-struct-lib"
//
//	s := struct233.NewSet(1, 2, 3)
//	q := struct233.NewPriorityQueue[int]()
//
// The types below are aliases of the implementations in the subpackages
// (set, skiplist, pqueue, deque, bitset, lru), so the facade adds no
// indirection: values returned by the factories are the concrete types and
// all of their methods stay available.
//
// None of the containers are safe for concurrent use.
package struct233

import (
	"cmp"

	"github.com/neko233-com/go-data-struct-lib/bitset"
	"github.com/neko233-com/go-data-struct-lib/deque"
	"github.com/neko233-com/go-data-struct-lib/lru"
	"github.com/neko233-com/go-data-struct-lib/pqueue"
	"github.com/neko233-com/go-data-struct-lib/set"
	"github.com/neko233-com/go-data-struct-lib/skiplist"
)

// Comparator orders two keys: negative if a < b, zero if a == b, and
// positive if a > b (same contract as java.util.Comparator).
type Comparator[K any] = skiplist.Comparator[K]

// Container aliases, mirroring the JDK collection names.
type (
	// Set is an unordered hash set (java.util.HashSet).
	Set[T comparable] = set.HashSet[T]
	// SortedSet is a skip-list backed sorted set (java.util.TreeSet).
	SortedSet[T any] = set.TreeSet[T]
	// LinkedSet keeps insertion order (java.util.LinkedHashSet).
	LinkedSet[T comparable] = set.LinkedHashSet[T]
	// SkipList is an ordered map with NavigableMap operations.
	SkipList[K any, V any] = skiplist.SkipList[K, V]
	// PriorityQueue is a binary-heap priority queue (java.util.PriorityQueue).
	PriorityQueue[T any] = pqueue.PriorityQueue[T]
	// Deque is a circular-array double-ended queue (java.util.ArrayDeque).
	Deque[T any] = deque.ArrayDeque[T]
	// BitSet is a growable bit vector (java.util.BitSet).
	BitSet = bitset.BitSet
	// LRU is a fixed-capacity least-recently-used cache.
	LRU[K comparable, V any] = lru.LRU[K, V]
)

// NewSet creates an unordered hash set, optionally pre-filled.
func NewSet[T comparable](items ...T) *Set[T] {
	return set.NewHashSet(items...)
}

// NewSetWithCapacity creates an unordered hash set whose map is pre-sized
// for capacity elements, avoiding rehashing while filling it.
func NewSetWithCapacity[T comparable](capacity int, items ...T) *Set[T] {
	return set.NewHashSetWithCapacity(capacity, items...)
}

// NewSortedSet creates a sorted set ordered by the natural ordering of T.
func NewSortedSet[T cmp.Ordered](items ...T) *SortedSet[T] {
	return set.NewTreeSet(items...)
}

// NewSortedSetFunc creates a sorted set ordered by a custom comparator.
func NewSortedSetFunc[T any](compare Comparator[T], items ...T) *SortedSet[T] {
	return set.NewTreeSetFunc(compare, items...)
}

// NewLinkedSet creates a set that keeps insertion order.
func NewLinkedSet[T comparable](items ...T) *LinkedSet[T] {
	return set.NewLinkedHashSet(items...)
}

// NewSkipList creates an ordered map ordered by the natural ordering of K.
func NewSkipList[K cmp.Ordered, V any]() *SkipList[K, V] {
	return skiplist.New[K, V]()
}

// NewSkipListFunc creates an ordered map ordered by a custom comparator.
func NewSkipListFunc[K any, V any](compare Comparator[K]) *SkipList[K, V] {
	return skiplist.NewFunc[K, V](compare)
}

// NewPriorityQueue creates a min-queue ordered by the natural ordering of T.
func NewPriorityQueue[T cmp.Ordered]() *PriorityQueue[T] {
	return pqueue.New[T]()
}

// NewPriorityQueueFunc creates a queue ordered by a custom comparator.
func NewPriorityQueueFunc[T any](compare func(a, b T) int) *PriorityQueue[T] {
	return pqueue.NewFunc(compare)
}

// NewPriorityQueueFromSlice builds a queue with all items in O(n).
func NewPriorityQueueFromSlice[T any](items []T, compare func(a, b T) int) *PriorityQueue[T] {
	return pqueue.NewFromSlice(items, compare)
}

// NewDeque creates an empty double-ended queue. An optional initial
// capacity may be given.
func NewDeque[T any](initialCapacity ...int) *Deque[T] {
	return deque.New[T](initialCapacity...)
}

// NewBitSet creates an empty bit vector.
func NewBitSet() *BitSet {
	return bitset.New()
}

// NewLRU creates an LRU cache with the given capacity (> 0).
func NewLRU[K comparable, V any](capacity int) *LRU[K, V] {
	return lru.New[K, V](capacity)
}
