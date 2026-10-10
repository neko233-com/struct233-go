// Package struct233 is the unified facade of struct233-go: all containers,
// single-threaded and goroutine-safe, are reachable from a single import.
//
//	import "github.com/neko233-com/struct233-go"
//
//	s := struct233.NewSet(1, 2, 3)
//	q := struct233.NewPriorityQueue[int]()
//	c := struct233.NewConcurrentSkipList[string, int]()
//
// The types below are aliases of the implementations in the subpackages
// (set, skiplist, pqueue, deque, bitset, lru, linkedmap, ringbuffer,
// concurrent), so the facade adds no indirection: values returned by the
// factories are the concrete types and all of their methods stay available.
//
// The plain containers are not safe for concurrent use (like java.util);
// use the Concurrent* types for cross-goroutine sharing.
package struct233

import (
	"cmp"

	"github.com/neko233-com/struct233-go/bitset"
	"github.com/neko233-com/struct233-go/concurrent"
	"github.com/neko233-com/struct233-go/deque"
	"github.com/neko233-com/struct233-go/linkedmap"
	"github.com/neko233-com/struct233-go/lru"
	"github.com/neko233-com/struct233-go/pqueue"
	"github.com/neko233-com/struct233-go/ringbuffer"
	"github.com/neko233-com/struct233-go/set"
	"github.com/neko233-com/struct233-go/skiplist"
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
	// SkipList is an ordered map with NavigableMap operations
	// (java.util.concurrent.ConcurrentSkipListMap, single-threaded variant).
	SkipList[K any, V any] = skiplist.SkipList[K, V]
	// TreeMap is the JDK name for the same ordered map as SkipList.
	TreeMap[K any, V any] = skiplist.SkipList[K, V]
	// LinkedMap preserves insertion order (java.util.LinkedHashMap).
	LinkedMap[K comparable, V any] = linkedmap.Map[K, V]
	// PriorityQueue is a binary-heap priority queue (java.util.PriorityQueue).
	PriorityQueue[T any] = pqueue.PriorityQueue[T]
	// Deque is a circular-array double-ended queue (java.util.ArrayDeque).
	Deque[T any] = deque.ArrayDeque[T]
	// RingBuffer keeps the newest N elements, overwriting the oldest.
	RingBuffer[T any] = ringbuffer.RingBuffer[T]
	// BitSet is a growable bit vector (java.util.BitSet).
	BitSet = bitset.BitSet
	// LRU is a fixed-capacity least-recently-used cache.
	LRU[K comparable, V any] = lru.LRU[K, V]

	// ConcurrentSet is a goroutine-safe unordered set.
	ConcurrentSet[T comparable] = concurrent.Set[T]
	// ConcurrentMap is a goroutine-safe map with an atomic GetOrPut.
	ConcurrentMap[K comparable, V any] = concurrent.Map[K, V]
	// ConcurrentSkipList is a goroutine-safe ordered map
	// (java.util.concurrent.ConcurrentSkipListMap).
	ConcurrentSkipList[K any, V any] = concurrent.SkipList[K, V]
	// ConcurrentLRU is a goroutine-safe LRU cache.
	ConcurrentLRU[K comparable, V any] = concurrent.LRU[K, V]
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

// NewTreeMap creates an ordered map ordered by the natural ordering of K
// (JDK name for NewSkipList).
func NewTreeMap[K cmp.Ordered, V any]() *TreeMap[K, V] {
	return skiplist.New[K, V]()
}

// NewTreeMapFunc creates an ordered map ordered by a custom comparator.
func NewTreeMapFunc[K any, V any](compare Comparator[K]) *TreeMap[K, V] {
	return skiplist.NewFunc[K, V](compare)
}

// NewLinkedMap creates a map that preserves insertion order.
func NewLinkedMap[K comparable, V any]() *LinkedMap[K, V] {
	return linkedmap.New[K, V]()
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

// NewRingBuffer creates a ring buffer of the given capacity (> 0) that keeps
// the newest elements and overwrites the oldest once full.
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	return ringbuffer.New[T](capacity)
}

// NewBitSet creates an empty bit vector.
func NewBitSet() *BitSet {
	return bitset.New()
}

// NewLRU creates an LRU cache with the given capacity (> 0).
func NewLRU[K comparable, V any](capacity int) *LRU[K, V] {
	return lru.New[K, V](capacity)
}

// NewConcurrentSet creates a goroutine-safe set, optionally pre-filled.
func NewConcurrentSet[T comparable](items ...T) *ConcurrentSet[T] {
	return concurrent.NewSet(items...)
}

// NewConcurrentSetWithCapacity creates an empty goroutine-safe set whose map
// is pre-sized.
func NewConcurrentSetWithCapacity[T comparable](capacity int) *ConcurrentSet[T] {
	return concurrent.NewSetWithCapacity[T](capacity)
}

// NewConcurrentMap creates an empty goroutine-safe map.
func NewConcurrentMap[K comparable, V any]() *ConcurrentMap[K, V] {
	return concurrent.NewMap[K, V]()
}

// NewConcurrentMapWithCapacity creates an empty goroutine-safe map pre-sized
// for capacity entries.
func NewConcurrentMapWithCapacity[K comparable, V any](capacity int) *ConcurrentMap[K, V] {
	return concurrent.NewMapWithCapacity[K, V](capacity)
}

// NewConcurrentSkipList creates a goroutine-safe ordered map using the
// natural ordering of K.
func NewConcurrentSkipList[K cmp.Ordered, V any]() *ConcurrentSkipList[K, V] {
	return concurrent.NewSkipList[K, V]()
}

// NewConcurrentSkipListFunc creates a goroutine-safe ordered map using a
// custom comparator.
func NewConcurrentSkipListFunc[K any, V any](compare Comparator[K]) *ConcurrentSkipList[K, V] {
	return concurrent.NewSkipListFunc[K, V](compare)
}

// NewConcurrentLRU creates a goroutine-safe LRU cache with the given
// capacity (> 0).
func NewConcurrentLRU[K comparable, V any](capacity int) *ConcurrentLRU[K, V] {
	return concurrent.NewLRU[K, V](capacity)
}
