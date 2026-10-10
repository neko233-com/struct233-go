// Package unionfind provides a disjoint-set union structure (union-find)
// with path compression and union by size, giving near-constant amortized
// operations.
//
// Elements are identified by the indexes 0..N-1 passed to New. Not safe for
// concurrent use.
package unionfind

// Set is a partition of the indexes 0..N-1 into disjoint components.
type Set struct {
	parent []int32 // parent[x] == x for a root
	size   []int32 // component size, valid at roots
	count  int     // number of components
}

// New creates a Set with n singleton components. It panics if n is negative.
func New(n int) *Set {
	if n < 0 {
		panic("unionfind: negative element count")
	}
	s := &Set{
		parent: make([]int32, n),
		size:   make([]int32, n),
		count:  n,
	}
	for i := range s.parent {
		s.parent[i] = int32(i)
		s.size[i] = 1
	}
	return s
}

// Len returns the number of elements.
func (s *Set) Len() int { return len(s.parent) }

// Count returns the number of disjoint components.
func (s *Set) Count() int { return s.count }

// Find returns the representative (root) of the component containing x,
// compressing the path along the way.
func (s *Set) Find(x int) int {
	s.check(x)
	root := x
	for s.parent[root] != int32(root) {
		root = int(s.parent[root])
	}
	for s.parent[x] != int32(root) { // path compression
		next := int(s.parent[x])
		s.parent[x] = int32(root)
		x = next
	}
	return root
}

// Union merges the components of a and b and reports whether they were
// different.
func (s *Set) Union(a, b int) bool {
	rootA, rootB := s.Find(a), s.Find(b)
	if rootA == rootB {
		return false
	}
	if s.size[rootA] < s.size[rootB] {
		rootA, rootB = rootB, rootA
	}
	s.parent[rootB] = int32(rootA)
	s.size[rootA] += s.size[rootB]
	s.count--
	return true
}

// Connected reports whether a and b belong to the same component.
func (s *Set) Connected(a, b int) bool {
	return s.Find(a) == s.Find(b)
}

// Size returns the number of elements in the component containing x.
func (s *Set) Size(x int) int {
	return int(s.size[s.Find(x)])
}

// Components returns the component roots as a slice of slices: each inner
// slice lists the member indexes in ascending order.
func (s *Set) Components() [][]int {
	buckets := make(map[int32][]int, s.count)
	for x := range s.parent {
		root := int32(s.Find(x))
		buckets[root] = append(buckets[root], x)
	}
	out := make([][]int, 0, len(buckets))
	for _, members := range buckets {
		out = append(out, members)
	}
	return out
}

func (s *Set) check(x int) {
	if x < 0 || x >= len(s.parent) {
		panic("unionfind: index out of range")
	}
}
