// Package striped provides striped locks: a fixed-size array of independent
// mutexes addressed by a caller-supplied key hash, mirroring
// com.google.common.util.concurrent.Striped.
//
// A single mutex serialises every key. A striped lock spreads contention over
// N independent stripes instead: two keys block each other only when their
// hashes collide modulo N. Pick a hash with good avalanche properties and N
// near the expected concurrency level; N is rounded up to a power of two so
// the hot path uses a bit mask instead of a modulo.
//
//	s := striped.New(16, func(k string) uint64 {
//		h := uint64(0xcbf29ce484222325)
//		for _, c := range k {
//			h = (h ^ uint64(c)) * 0x100000001b3
//		}
//		return h
//	})
//	mu := s.Get(key) // always the same mutex for the same key, zero allocs
//	mu.Lock()
//	defer mu.Unlock()
//	// ... critical section ...
//
// All methods are safe for concurrent use. Get never allocates: it returns a
// pointer into the pre-allocated stripe array. StripedRW offers the same
// guarantees with *sync.RWMutex stripes for read-mostly work.
package striped

import "sync"

// maxStripes bounds the stripe count so the power-of-two rounding below can
// never overflow and no absurd allocation is attempted.
const maxStripes = 1 << 30

// Striped is a fixed-size set of independent mutexes addressed by key hash,
// similar to a Guava Striped<Lock> with the strong (non-bulk) semantics:
// Get returns the same *sync.Mutex for the same key at any time.
// The zero value is not usable; create a Striped with New.
type Striped[K any] struct {
	stripes []sync.Mutex
	mask    uint64
	hash    func(K) uint64
}

// New creates a Striped with at least stripes independent mutexes. The actual
// number is rounded up to the next power of two (see Size) and hash maps a
// key to a 64-bit hash value. New panics with a "striped: ..." message when
// stripes <= 0, when hash is nil, or when stripes exceeds 1<<30.
func New[K any](stripes int, hash func(K) uint64) *Striped[K] {
	size := stripeCount(stripes, hash)
	return &Striped[K]{
		stripes: make([]sync.Mutex, size),
		mask:    uint64(size - 1),
		hash:    hash,
	}
}

// Get returns the mutex guarding key. It always returns the same pointer for
// the same key, is safe for concurrent use, and performs zero allocations.
func (s *Striped[K]) Get(key K) *sync.Mutex {
	return &s.stripes[s.hash(key)&s.mask]
}

// GetAt returns the i-th stripe directly, bypassing the hash. i is wrapped
// modulo Size, so any integer is valid and GetAt(i) == GetAt(i+Size) holds
// for every i.
func (s *Striped[K]) GetAt(i int) *sync.Mutex {
	return &s.stripes[uint64(i)&s.mask]
}

// Size returns the number of stripes. It is always a power of two, at least
// the value passed to New.
func (s *Striped[K]) Size() int {
	return len(s.stripes)
}

// StripedRW is the read-write variant of Striped, mirroring the Guava
// readWriteLock semantics: the stripes are *sync.RWMutex, so readers of the
// same key proceed in parallel while writers stay exclusive. The zero value
// is not usable; create a StripedRW with NewRW.
type StripedRW[K any] struct {
	stripes []sync.RWMutex
	mask    uint64
	hash    func(K) uint64
}

// NewRW creates a StripedRW with at least stripes independent RWMutexes,
// rounded up to the next power of two. NewRW panics with a "striped: ..."
// message under the same conditions as New.
func NewRW[K any](stripes int, hash func(K) uint64) *StripedRW[K] {
	size := stripeCount(stripes, hash)
	return &StripedRW[K]{
		stripes: make([]sync.RWMutex, size),
		mask:    uint64(size - 1),
		hash:    hash,
	}
}

// GetRW returns the RWMutex guarding key. It always returns the same pointer
// for the same key, is safe for concurrent use, and performs zero allocations.
func (s *StripedRW[K]) GetRW(key K) *sync.RWMutex {
	return &s.stripes[s.hash(key)&s.mask]
}

// GetAt returns the i-th stripe directly, bypassing the hash. i is wrapped
// modulo Size, so any integer is valid and GetAt(i) == GetAt(i+Size) holds
// for every i.
func (s *StripedRW[K]) GetAt(i int) *sync.RWMutex {
	return &s.stripes[uint64(i)&s.mask]
}

// Size returns the number of stripes. It is always a power of two, at least
// the value passed to NewRW.
func (s *StripedRW[K]) Size() int {
	return len(s.stripes)
}

// stripeCount validates the constructor arguments and rounds stripes up to
// the next power of two.
func stripeCount[K any](stripes int, hash func(K) uint64) int {
	if stripes <= 0 {
		panic("striped: stripes must be positive")
	}
	if hash == nil {
		panic("striped: hash must not be nil")
	}
	if stripes > maxStripes {
		panic("striped: stripes too large")
	}
	size := 1
	for size < stripes {
		size <<= 1
	}
	return size
}
