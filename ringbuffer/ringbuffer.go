// Package ringbuffer provides a fixed-capacity ring buffer with
// overwrite-oldest semantics.
//
// It is the container behind "last N events" style tracking (recent damage,
// frame metrics, rolling logs): pushing into a full buffer discards the
// oldest element instead of growing. Not safe for concurrent use.
package ringbuffer

import "iter"

// RingBuffer holds at most Capacity elements in FIFO order.
type RingBuffer[T any] struct {
	buf  []T
	head int // index of the oldest element
	size int
}

// New creates a ring buffer with the given capacity. It panics if
// capacity <= 0.
func New[T any](capacity int) *RingBuffer[T] {
	if capacity <= 0 {
		panic("ringbuffer: capacity must be positive")
	}
	return &RingBuffer[T]{buf: make([]T, capacity)}
}

// Cap returns the fixed capacity.
func (r *RingBuffer[T]) Cap() int { return len(r.buf) }

// Len returns the number of elements currently stored.
func (r *RingBuffer[T]) Len() int { return r.size }

// Push appends v. When the buffer is full the oldest element is overwritten
// and returned as evicted.
func (r *RingBuffer[T]) Push(v T) (oldest T, evicted bool) {
	if r.size < len(r.buf) {
		idx := r.head + r.size
		if idx >= len(r.buf) {
			idx -= len(r.buf)
		}
		r.buf[idx] = v
		r.size++
		return oldest, false
	}
	// full: overwrite the oldest element and advance the head
	oldest = r.buf[r.head]
	r.buf[r.head] = v
	if r.head++; r.head == len(r.buf) {
		r.head = 0
	}
	return oldest, true
}

// Pop removes and returns the oldest element.
func (r *RingBuffer[T]) Pop() (T, bool) {
	if r.size == 0 {
		var zero T
		return zero, false
	}
	v := r.buf[r.head]
	clear(r.buf[r.head : r.head+1])
	if r.head++; r.head == len(r.buf) {
		r.head = 0
	}
	r.size--
	return v, true
}

// Peek returns the oldest element without removing it.
func (r *RingBuffer[T]) Peek() (T, bool) {
	if r.size == 0 {
		var zero T
		return zero, false
	}
	return r.buf[r.head], true
}

// At returns the element at logical index i (0 = oldest), i < Len.
func (r *RingBuffer[T]) At(i int) (T, bool) {
	if i < 0 || i >= r.size {
		var zero T
		return zero, false
	}
	idx := r.head + i
	if idx >= len(r.buf) {
		idx -= len(r.buf)
	}
	return r.buf[idx], true
}

// Clear removes all elements while keeping the capacity.
func (r *RingBuffer[T]) Clear() {
	clear(r.buf)
	r.head = 0
	r.size = 0
}

// ToSlice returns the elements from oldest to newest.
func (r *RingBuffer[T]) ToSlice() []T {
	out := make([]T, r.size)
	idx := r.head
	for i := range r.size {
		out[i] = r.buf[idx]
		if idx++; idx == len(r.buf) {
			idx = 0
		}
	}
	return out
}

// All iterates from oldest to newest.
func (r *RingBuffer[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		idx := r.head
		for range r.size {
			if !yield(r.buf[idx]) {
				return
			}
			if idx++; idx == len(r.buf) {
				idx = 0
			}
		}
	}
}
