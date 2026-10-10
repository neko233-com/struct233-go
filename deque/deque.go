// Package deque provides a double-ended queue backed by a growable
// circular array, similar to java.util.ArrayDeque.
//
// All operations run in amortized O(1). Use PushBack/PopFront as a FIFO
// queue and PushBack/PopBack as a LIFO stack. Not safe for concurrent use.
package deque

import "iter"

const minCapacity = 8

// ArrayDeque is a double-ended queue.
type ArrayDeque[T any] struct {
	buf  []T
	head int // index of the first element within buf
	size int
}

// New creates an empty deque. An optional initial capacity may be given.
func New[T any](initialCapacity ...int) *ArrayDeque[T] {
	capacity := minCapacity
	if len(initialCapacity) > 0 && initialCapacity[0] > capacity {
		capacity = initialCapacity[0]
	}
	return &ArrayDeque[T]{buf: make([]T, capacity)}
}

// Len returns the number of elements.
func (d *ArrayDeque[T]) Len() int { return d.size }

// Cap returns the current backing-array capacity.
func (d *ArrayDeque[T]) Cap() int { return len(d.buf) }

// grow doubles the backing array with two block copies instead of a
// per-element index walk.
func (d *ArrayDeque[T]) grow() {
	nbuf := make([]T, len(d.buf)*2)
	n := copy(nbuf, d.buf[d.head:])
	copy(nbuf[n:], d.buf[:d.head])
	d.buf = nbuf
	d.head = 0
}

// backIndex returns the buffer index that follows the last element.
func (d *ArrayDeque[T]) backIndex() int {
	idx := d.head + d.size
	if idx >= len(d.buf) {
		idx -= len(d.buf)
	}
	return idx
}

// PushFront adds v to the front.
func (d *ArrayDeque[T]) PushFront(v T) {
	if d.size == len(d.buf) {
		d.grow()
	}
	if d.head == 0 {
		d.head = len(d.buf) - 1
	} else {
		d.head--
	}
	d.buf[d.head] = v
	d.size++
}

// PushBack adds v to the back.
func (d *ArrayDeque[T]) PushBack(v T) {
	if d.size == len(d.buf) {
		d.grow()
	}
	d.buf[d.backIndex()] = v
	d.size++
}

// PopFront removes and returns the front element.
func (d *ArrayDeque[T]) PopFront() (T, bool) {
	if d.size == 0 {
		var zero T
		return zero, false
	}
	v := d.buf[d.head]
	clear(d.buf[d.head : d.head+1])
	if d.head++; d.head == len(d.buf) {
		d.head = 0
	}
	d.size--
	return v, true
}

// PopBack removes and returns the back element.
func (d *ArrayDeque[T]) PopBack() (T, bool) {
	if d.size == 0 {
		var zero T
		return zero, false
	}
	idx := d.head + d.size - 1
	if idx >= len(d.buf) {
		idx -= len(d.buf)
	}
	v := d.buf[idx]
	clear(d.buf[idx : idx+1])
	d.size--
	return v, true
}

// Front returns the front element without removing it.
func (d *ArrayDeque[T]) Front() (T, bool) {
	if d.size == 0 {
		var zero T
		return zero, false
	}
	return d.buf[d.head], true
}

// Back returns the back element without removing it.
func (d *ArrayDeque[T]) Back() (T, bool) {
	if d.size == 0 {
		var zero T
		return zero, false
	}
	idx := d.head + d.size - 1
	if idx >= len(d.buf) {
		idx -= len(d.buf)
	}
	return d.buf[idx], true
}

// Clear removes all elements while keeping the allocated capacity.
func (d *ArrayDeque[T]) Clear() {
	clear(d.buf)
	d.head = 0
	d.size = 0
}

// All iterates from front to back.
func (d *ArrayDeque[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		idx := d.head
		for range d.size {
			if !yield(d.buf[idx]) {
				return
			}
			if idx++; idx == len(d.buf) {
				idx = 0
			}
		}
	}
}

// ToSlice returns the elements from front to back.
func (d *ArrayDeque[T]) ToSlice() []T {
	out := make([]T, d.size)
	idx := d.head
	for i := range d.size {
		out[i] = d.buf[idx]
		if idx++; idx == len(d.buf) {
			idx = 0
		}
	}
	return out
}
