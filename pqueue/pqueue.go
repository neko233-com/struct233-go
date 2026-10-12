// Package pqueue provides a priority queue backed by a binary heap,
// similar to java.util.HeapPriorityQueue.
//
// By default the smallest element pops first (min-heap); pass a custom
// comparator to change the ordering. Not safe for concurrent use.
package pqueue

import (
	"cmp"
	"slices"
)

// HeapPriorityQueue pops elements according to the comparator
// (same contract as java.util.HeapPriorityQueue).
type HeapPriorityQueue[T any] struct {
	less func(a, b T) int
	heap []T
}

// New creates a min-queue ordered by the natural ordering of T.
func New[T cmp.Ordered]() *HeapPriorityQueue[T] {
	return NewFunc[T](cmp.Compare[T])
}

// NewFunc creates a queue ordered by a custom comparator.
// It panics if compare is nil.
func NewFunc[T any](compare func(a, b T) int) *HeapPriorityQueue[T] {
	if compare == nil {
		panic("pqueue: nil comparator")
	}
	return &HeapPriorityQueue[T]{less: compare}
}

// NewFromSlice builds a queue containing all items, in O(n).
// The input slice is not retained.
func NewFromSlice[T any](items []T, compare func(a, b T) int) *HeapPriorityQueue[T] {
	q := NewFunc[T](compare)
	q.heap = slices.Clone(items)
	for i := len(q.heap)/2 - 1; i >= 0; i-- {
		q.siftDown(i)
	}
	return q
}

// Len returns the number of elements.
func (q *HeapPriorityQueue[T]) Len() int { return len(q.heap) }

// Clear removes all elements.
func (q *HeapPriorityQueue[T]) Clear() { clear(q.heap); q.heap = q.heap[:0] }

// Push adds an element.
func (q *HeapPriorityQueue[T]) Push(v T) {
	q.heap = append(q.heap, v)
	q.siftUp(len(q.heap) - 1)
}

// Pop removes and returns the smallest element per the comparator.
func (q *HeapPriorityQueue[T]) Pop() (T, bool) {
	if len(q.heap) == 0 {
		var zero T
		return zero, false
	}
	top := q.heap[0]
	last := len(q.heap) - 1
	q.heap[0] = q.heap[last]
	clear(q.heap[last:])
	q.heap = q.heap[:last]
	if last > 0 {
		q.siftDown(0)
	}
	return top, true
}

// Peek returns the smallest element without removing it.
func (q *HeapPriorityQueue[T]) Peek() (T, bool) {
	if len(q.heap) == 0 {
		var zero T
		return zero, false
	}
	return q.heap[0], true
}

// ToSlice returns a copy of the underlying heap; the order is unspecified.
func (q *HeapPriorityQueue[T]) ToSlice() []T { return slices.Clone(q.heap) }

func (q *HeapPriorityQueue[T]) siftUp(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if q.less(q.heap[i], q.heap[parent]) < 0 {
			q.heap[i], q.heap[parent] = q.heap[parent], q.heap[i]
			i = parent
		} else {
			return
		}
	}
}

func (q *HeapPriorityQueue[T]) siftDown(i int) {
	n := len(q.heap)
	for {
		left, right, smallest := 2*i+1, 2*i+2, i
		if left < n && q.less(q.heap[left], q.heap[smallest]) < 0 {
			smallest = left
		}
		if right < n && q.less(q.heap[right], q.heap[smallest]) < 0 {
			smallest = right
		}
		if smallest == i {
			return
		}
		q.heap[i], q.heap[smallest] = q.heap[smallest], q.heap[i]
		i = smallest
	}
}
