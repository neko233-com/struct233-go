// Package bidimap provides a bidirectional map
// (com.google.common.collect.BiMap) that keeps a key→value mapping and its
// value→key inverse in sync: every key is bound to at most one value and every
// value to at most one key, so both directions are functions.
//
//	b := bidimap.New[string, int]()
//	b.Put("one", 1)          // true: value 1 is still free
//	b.Put("uno", 1)          // false: 1 is already bound to "one"
//	b.ForcePut("uno", 1)     // rebinds 1 to "uno", dropping "one"
//	v, ok := b.Get("uno")    // 1, true
//	k, ok := b.GetKeyOf(1)   // "uno", true
//	inv := b.Inverse()       // live view over the same storage
//	inv.Get(1)               // "uno", true
//
// Put never breaks the bijection: when the value is already bound to another
// key it reports false and changes nothing (Guava's put throws instead).
// ForcePut overwrites both conflicting bindings. Inverse returns a live view
// sharing the same storage as its origin, so a write through either side is
// immediately visible through the other one. Keys, Values and Entries iterate
// in unspecified order.
//
// The zero value is an empty, ready-to-use bimap. A BiMap must not be copied
// after first use. Not safe for concurrent use: the concurrent package owns
// the goroutine-safe variants.
package bidimap

import "iter"

// BiMap is a bidirectional map: forward maps each key to its value, reverse
// maps each value back to its key, and inverse is the live swapped view of the
// two. The three fields always describe one single bijection.
type BiMap[K comparable, V comparable] struct {
	forward map[K]V      // key to value
	reverse map[V]K      // value to key, always the exact inverse of forward
	inverse *BiMap[V, K] // live swapped view sharing forward and reverse
}

// New creates an empty bimap.
func New[K comparable, V comparable]() *BiMap[K, V] {
	b := &BiMap[K, V]{}
	b.init()
	return b
}

// NewWithCapacity creates an empty bimap pre-sized for capacity bindings. It
// panics with "bidimap: negative capacity" when capacity is negative.
func NewWithCapacity[K comparable, V comparable](capacity int) *BiMap[K, V] {
	if capacity < 0 {
		panic("bidimap: negative capacity")
	}
	b := &BiMap[K, V]{
		forward: make(map[K]V, capacity),
		reverse: make(map[V]K, capacity),
	}
	b.inverse = &BiMap[V, K]{forward: b.reverse, reverse: b.forward, inverse: b}
	return b
}

// Put binds key to value and returns true. It returns false without changing
// anything when value is already bound to another key (Guava's put throws;
// the collision is reported as false here). Re-putting a key releases its
// previous value, keeping the bijection exact.
func (b *BiMap[K, V]) Put(key K, value V) bool {
	if old, ok := b.forward[key]; ok {
		if old == value {
			return true
		}
		// old != value, so a bound value always belongs to another key
		if _, taken := b.reverse[value]; taken {
			return false
		}
		delete(b.reverse, old)
	} else if _, taken := b.reverse[value]; taken {
		return false
	}
	b.init()
	b.forward[key] = value
	b.reverse[value] = key
	return true
}

// ForcePut binds key to value even when that breaks existing bindings: both
// the key's previous value and the value's previous key are released.
func (b *BiMap[K, V]) ForcePut(key K, value V) {
	b.init()
	if old, ok := b.forward[key]; ok && old != value {
		delete(b.reverse, old)
	}
	if bound, taken := b.reverse[value]; taken && bound != key {
		delete(b.forward, bound)
	}
	b.forward[key] = value
	b.reverse[value] = key
}

// Get returns the value bound to key; ok is false when key is unbound.
func (b *BiMap[K, V]) Get(key K) (V, bool) {
	value, ok := b.forward[key]
	return value, ok
}

// GetKeyOf returns the key bound to value (the reverse lookup); ok is false
// when value is unbound.
func (b *BiMap[K, V]) GetKeyOf(value V) (K, bool) {
	key, ok := b.reverse[value]
	return key, ok
}

// Inverse returns the live reverse view over the same storage: every write
// through either side is immediately visible through the other one, and
// Inverse of the inverse is the original bimap.
func (b *BiMap[K, V]) Inverse() *BiMap[V, K] {
	b.init()
	return b.inverse
}

// Remove unbinds key and returns the value that was bound to it; ok is false
// when key was unbound and nothing changed.
func (b *BiMap[K, V]) Remove(key K) (V, bool) {
	value, ok := b.forward[key]
	if !ok {
		var zero V
		return zero, false
	}
	delete(b.forward, key)
	delete(b.reverse, value)
	return value, true
}

// Len returns the number of bound pairs.
func (b *BiMap[K, V]) Len() int {
	return len(b.forward)
}

// Clear unbinds every pair.
func (b *BiMap[K, V]) Clear() {
	if b.forward == nil {
		return
	}
	clear(b.forward)
	clear(b.reverse)
}

// Keys returns the bound keys in unspecified order.
func (b *BiMap[K, V]) Keys() []K {
	out := make([]K, 0, len(b.forward))
	for key := range b.forward {
		out = append(out, key)
	}
	return out
}

// Values returns the bound values in unspecified order.
func (b *BiMap[K, V]) Values() []V {
	out := make([]V, 0, len(b.reverse))
	for value := range b.reverse {
		out = append(out, value)
	}
	return out
}

// Entries iterates the bound key→value pairs in unspecified order; returning
// false from yield stops the iteration early.
func (b *BiMap[K, V]) Entries() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for key, value := range b.forward {
			if !yield(key, value) {
				return
			}
		}
	}
}

// init lazily builds the backing maps and the live inverse view, which is what
// makes the zero value ready to use.
func (b *BiMap[K, V]) init() {
	if b.forward != nil {
		return
	}
	b.forward = make(map[K]V)
	b.reverse = make(map[V]K)
	b.inverse = &BiMap[V, K]{forward: b.reverse, reverse: b.forward, inverse: b}
}
