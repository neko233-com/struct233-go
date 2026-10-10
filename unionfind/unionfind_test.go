package unionfind_test

import (
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/unionfind"
)

func TestSingletonComponents(t *testing.T) {
	s := unionfind.New(5)
	require.Equal(t, 5, s.Len())
	require.Equal(t, 5, s.Count())

	for i := range 5 {
		require.Equal(t, i, s.Find(i), "each element is its own root initially")
		require.Equal(t, 1, s.Size(i))
		require.False(t, s.Connected(i, (i+1)%5))
	}
}

func TestUnionAndConnected(t *testing.T) {
	s := unionfind.New(6)
	require.Equal(t, 6, s.Count())

	require.True(t, s.Union(0, 1))
	require.False(t, s.Union(1, 0), "unioning the same components is a no-op")
	require.Equal(t, 5, s.Count())

	require.True(t, s.Union(2, 3))
	require.True(t, s.Union(3, 4))
	require.Equal(t, 3, s.Count())
	require.Equal(t, 3, s.Size(2))
	require.Equal(t, 3, s.Size(4))

	require.True(t, s.Connected(2, 4))
	require.False(t, s.Connected(0, 4))

	require.True(t, s.Union(0, 4))
	require.Equal(t, 2, s.Count())
	require.Equal(t, 5, s.Size(0))

	require.True(t, s.Union(0, 5))
	require.Equal(t, 1, s.Count())
	require.Equal(t, 6, s.Size(0))
}

func TestUnionBySizeKeepsTreeShallow(t *testing.T) {
	const n = 10_000
	s := unionfind.New(n)
	for i := 1; i < n; i++ {
		s.Union(i, 0)
	}
	require.Equal(t, 1, s.Count())
	require.Equal(t, n, s.Size(n-1))

	// with union by size + path compression the root is depth 1
	root := s.Find(n - 1)
	require.Equal(t, s.Find(0), root)
	for i := range n {
		require.Equal(t, root, s.Find(i))
	}
}

func TestComponents(t *testing.T) {
	s := unionfind.New(7)
	s.Union(0, 1)
	s.Union(1, 2)
	s.Union(4, 5)

	components := s.Components()
	require.Len(t, components, 4)

	for _, members := range components {
		sort.Ints(members)
	}
	sort.Slice(components, func(i, j int) bool { return components[i][0] < components[j][0] })

	require.Equal(t, [][]int{{0, 1, 2}, {3}, {4, 5}, {6}}, components)
}

func TestFindAgainstReferenceModel(t *testing.T) {
	const n = 500
	s := unionfind.New(n)

	// reference model: plain reachability via labels
	labels := make([]int, n)
	for i := range labels {
		labels[i] = i
	}

	rng := rand.New(rand.NewPCG(9, 9))
	for range 2000 {
		a, b := rng.IntN(n), rng.IntN(n)
		require.Equal(t, labels[a] != labels[b], s.Union(a, b),
			"Union(%d,%d) must report whether the components differed", a, b)

		la, lb := labels[a], labels[b]
		if la != lb {
			for i, l := range labels {
				if l == lb {
					labels[i] = la
				}
			}
		}

		x, y := rng.IntN(n), rng.IntN(n)
		require.Equal(t, labels[x] == labels[y], s.Connected(x, y),
			"Connected(%d,%d) must match the reference model", x, y)
	}
}

func TestPanicsOnBadIndex(t *testing.T) {
	s := unionfind.New(2)
	require.Panics(t, func() { s.Find(2) })
	require.Panics(t, func() { s.Find(-1) })
	require.Panics(t, func() { s.Union(0, 5) })
	require.Panics(t, func() { um := unionfind.New(-1); _ = um })
}

func TestEmptySet(t *testing.T) {
	s := unionfind.New(0)
	require.Zero(t, s.Len())
	require.Zero(t, s.Count())
	require.Empty(t, s.Components())
}

func BenchmarkUnionFindUnion(b *testing.B) {
	const n = 1 << 16
	s := unionfind.New(n)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		s.Union(i&(n-1), (i+1)&(n-1))
	}
}

func BenchmarkUnionFindFind(b *testing.B) {
	const n = 1 << 14
	s := unionfind.New(n)
	for i := 1; i < n; i++ {
		s.Union(i, i-1)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		s.Find(i & (n - 1))
	}
}
