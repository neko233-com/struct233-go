// Package hashring implements a consistent hash ring: keys are placed on a
// circle of virtual nodes, so adding or removing one node only re-places the
// keys that node owns instead of reshuffling every key.
//
// This is the placement layer behind sharded caches, actor routing and
// "which worker owns this shard" questions. A ring is safe for concurrent
// use; every field sits behind one RWMutex.
//
// Virtual points come from a deterministic byte encoding of
// "node + separator + replica index" hashed with the ring's Hasher. They are
// kept in a sorted point slice with a parallel owner slice and probed with a
// binary search, so a lookup is O(log n) and allocation free.
//
//	ring := hashring.New(hashring.WithReplicas(64))
//	ring.Add("cache-a", "cache-b", "cache-c")
//
//	if node, ok := ring.Get("user:42"); ok {
//		_ = node // cache-a, or whoever owns user:42
//	}
//	_ = ring.GetN("user:42", 2) // primary plus one successor
//	_ = ring.Nodes()            // sorted node names
//	_ = ring.Len()              // virtual points on the circle
package hashring

import (
	"sort"
	"strconv"
	"sync"
	"unsafe"

	"github.com/neko233-com/struct233-go/hash"
)

// Hasher hashes a key to a 64-bit value.
type Hasher func(key []byte) uint64

// defaultReplicas is the virtual node count used when no option overrides it.
const defaultReplicas = 100

// Ring is a consistent hash ring: keys sit on a circle of virtual nodes, so
// adding or removing a node only moves the keys it owns.
type Ring struct {
	mu       sync.RWMutex
	nodes    map[string]struct{}
	hasher   Hasher
	replicas int
	points   []uint64 // sorted virtual point hashes
	owners   []string // owners[i] owns points[i]
}

// Option configures a Ring.
type Option func(*Ring)

// WithReplicas sets how many virtual nodes each node contributes. Default 100;
// values below 1 panic when applied.
func WithReplicas(n int) Option {
	return func(r *Ring) {
		if n < 1 {
			panic("hashring: replicas must be positive")
		}
		r.replicas = n
	}
}

// WithHasher sets the hash used for keys and virtual nodes. Default
// hash.Sum64; a nil hasher panics when applied.
func WithHasher(h Hasher) Option {
	return func(r *Ring) {
		if h == nil {
			panic("hashring: nil hasher")
		}
		r.hasher = h
	}
}

// New creates an empty ring. It panics on invalid options.
func New(opts ...Option) *Ring {
	r := &Ring{
		nodes:    make(map[string]struct{}),
		hasher:   hash.Sum64,
		replicas: defaultReplicas,
	}
	for _, opt := range opts {
		if opt == nil {
			panic("hashring: nil option")
		}
		opt(r)
	}
	return r
}

// Add inserts nodes, ignoring names already present.
func (r *Ring) Add(nodes ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	changed := false
	for _, n := range nodes {
		if _, ok := r.nodes[n]; ok {
			continue
		}
		r.nodes[n] = struct{}{}
		changed = true
	}
	if changed {
		r.rebuild()
	}
}

// Remove deletes a node, reporting whether it was present.
func (r *Ring) Remove(node string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.nodes[node]; !ok {
		return false
	}
	delete(r.nodes, node)
	r.rebuild()
	return true
}

// Get returns the node that owns key; ok is false when the ring is empty.
func (r *Ring) Get(key string) (node string, ok bool) {
	h := r.hashKey(key)
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.points) == 0 {
		return "", false
	}
	return r.owners[r.search(h)], true
}

// GetN returns up to n distinct nodes that own key, in ring order from key's
// position.
func (r *Ring) GetN(key string, n int) []string {
	if n <= 0 {
		return nil
	}
	h := r.hashKey(key)
	r.mu.RLock()
	defer r.mu.RUnlock()
	total := len(r.points)
	if total == 0 {
		return nil
	}
	out := make([]string, 0, min(n, len(r.nodes)))
	start := r.search(h)
	for i := range total {
		if len(out) == n {
			break
		}
		owner := r.owners[(start+i)%total]
		dup := false
		for _, seen := range out {
			if seen == owner {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, owner)
		}
	}
	return out
}

// Nodes returns the nodes in sorted order.
func (r *Ring) Nodes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.nodes))
	for n := range r.nodes {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Replicas returns how many virtual nodes each node contributes.
func (r *Ring) Replicas() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.replicas
}

// Len returns how many virtual nodes are on the circle.
func (r *Ring) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.points)
}

// search returns the index of the first point >= h, wrapping around to 0 when
// h sits past the last point.
func (r *Ring) search(h uint64) int {
	lo, hi := 0, len(r.points)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if r.points[mid] < h {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == len(r.points) {
		return 0
	}
	return lo
}

// hashKey hashes key without copying it. The hasher only reads the bytes and
// must not retain the slice past the call.
func (r *Ring) hashKey(key string) uint64 {
	return r.hasher(unsafe.Slice(unsafe.StringData(key), len(key)))
}

// rebuild recomputes the circle from the current node set. The caller must
// hold the write lock.
func (r *Ring) rebuild() {
	total := len(r.nodes) * r.replicas
	if cap(r.points) < total {
		r.points = make([]uint64, 0, total)
		r.owners = make([]string, 0, total)
	}
	r.points = r.points[:0]
	r.owners = r.owners[:0]
	var scratch []byte
	for node := range r.nodes {
		for i := range r.replicas {
			scratch = appendPoint(scratch[:0], node, i)
			r.points = append(r.points, r.hasher(scratch))
			r.owners = append(r.owners, node)
		}
	}
	sort.Sort(pointSet{points: r.points, owners: r.owners})
}

// appendPoint encodes "node + 0x00 + replica" into dst without fmt, so a node
// and replica index always hash from the same bytes.
func appendPoint(dst []byte, node string, replica int) []byte {
	dst = append(dst, node...)
	dst = append(dst, 0)
	return strconv.AppendInt(dst, int64(replica), 10)
}

// pointSet sorts virtual points and their owners together. Ties break on the
// owner name, so colliding points resolve the same way on every rebuild.
type pointSet struct {
	points []uint64
	owners []string
}

// Len reports the number of points.
func (p pointSet) Len() int { return len(p.points) }

// Less orders by point hash, then by owner name.
func (p pointSet) Less(i, j int) bool {
	if p.points[i] != p.points[j] {
		return p.points[i] < p.points[j]
	}
	return p.owners[i] < p.owners[j]
}

// Swap exchanges two points and keeps the owner slice in step.
func (p pointSet) Swap(i, j int) {
	p.points[i], p.points[j] = p.points[j], p.points[i]
	p.owners[i], p.owners[j] = p.owners[j], p.owners[i]
}
