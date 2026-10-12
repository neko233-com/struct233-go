package rangemap_test

import (
	"math/rand/v2"
	"testing"

	"github.com/neko233-com/struct233-go/rangemap"
	"github.com/neko233-com/struct233-go/rangeset"
	"github.com/stretchr/testify/require"
)

func TestRangeMapPutSplitsAndOverwrites(t *testing.T) {
	m := rangemap.New[int, string]()
	m.Put(rangeset.Closed(1, 10), "old")
	m.Put(rangeset.ClosedOpen(4, 7), "new")
	require.Equal(t, []rangemap.Entry[int, string]{
		{Range: rangeset.ClosedOpen(1, 4), Value: "old"},
		{Range: rangeset.ClosedOpen(4, 7), Value: "new"},
		{Range: rangeset.Closed(7, 10), Value: "old"},
	}, m.Entries())
	for _, tc := range []struct {
		key  int
		want string
		ok   bool
	}{{0, "", false}, {1, "old", true}, {3, "old", true}, {4, "new", true}, {6, "new", true}, {7, "old", true}, {10, "old", true}, {11, "", false}} {
		got, ok := m.Get(tc.key)
		require.Equal(t, tc.want, got)
		require.Equal(t, tc.ok, ok)
	}
	entry, ok := m.GetEntry(7)
	require.True(t, ok)
	require.Equal(t, rangeset.Closed(7, 10), entry.Range)
}

func TestRangeMapOpenAndUnboundedBounds(t *testing.T) {
	m := rangemap.New[int, string]()
	m.Put(rangeset.All[int](), "all")
	m.Put(rangeset.OpenClosed(-2, 2), "middle")
	require.Equal(t, []rangemap.Entry[int, string]{
		{Range: rangeset.AtMost(-2), Value: "all"},
		{Range: rangeset.OpenClosed(-2, 2), Value: "middle"},
		{Range: rangeset.GreaterThan(2), Value: "all"},
	}, m.Entries())
	for _, tc := range []struct {
		key  int
		want string
	}{{-3, "all"}, {-2, "all"}, {-1, "middle"}, {2, "middle"}, {3, "all"}} {
		got, ok := m.Get(tc.key)
		require.True(t, ok)
		require.Equal(t, tc.want, got)
	}

	// Open boundaries can leave a point unmapped without creating overlap.
	m.Clear()
	m.Put(rangeset.ClosedOpen(0, 1), "left")
	m.Put(rangeset.OpenClosed(1, 2), "right")
	_, ok := m.Get(1)
	require.False(t, ok)
}

func TestRangeMapRemoveSubRangeAndSnapshots(t *testing.T) {
	m := rangemap.New[int, string]()
	m.Put(rangeset.Closed(0, 20), "x")
	m.Remove(rangeset.Closed(5, 15))
	require.Equal(t, []rangemap.Entry[int, string]{
		{Range: rangeset.ClosedOpen(0, 5), Value: "x"},
		{Range: rangeset.OpenClosed(15, 20), Value: "x"},
	}, m.Entries())

	sub := m.SubRangeMap(rangeset.Closed(3, 17))
	require.Equal(t, []rangemap.Entry[int, string]{
		{Range: rangeset.ClosedOpen(3, 5), Value: "x"},
		{Range: rangeset.OpenClosed(15, 17), Value: "x"},
	}, sub.Entries())
	sub.Put(rangeset.Singleton(4), "sub")
	got, ok := m.Get(4)
	require.True(t, ok)
	require.Equal(t, "x", got, "subrange map must not share mutable storage")

	span, ok := m.Span()
	require.True(t, ok)
	require.Equal(t, rangeset.Closed(0, 20), span)
	entries := m.Entries()
	entries[0].Value = "changed"
	require.Equal(t, "x", m.AsMapOfRanges()[rangeset.ClosedOpen(0, 5)])
	require.Equal(t, 2, m.Len())

	m.Remove(rangeset.All[int]())
	require.True(t, m.IsEmpty())
	_, ok = m.Span()
	require.False(t, ok)
	m.Remove(rangeset.All[int]())
	m.Put(rangeset.Open(1, 1), "empty")
	require.True(t, m.IsEmpty())
}

func TestRangeMapPutAll(t *testing.T) {
	a := rangemap.New[int, string]()
	a.Put(rangeset.Closed(0, 10), "a")
	b := rangemap.New[int, string]()
	b.Put(rangeset.Closed(3, 7), "b")
	a.PutAll(b)
	a.PutAll(nil)
	require.Equal(t, []rangemap.Entry[int, string]{
		{Range: rangeset.ClosedOpen(0, 3), Value: "a"},
		{Range: rangeset.Closed(3, 7), Value: "b"},
		{Range: rangeset.OpenClosed(7, 10), Value: "a"},
	}, a.Entries())
}

func TestRangeMapRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(4, 9))
	m := rangemap.New[int, int]()
	model := make(map[int]int)
	for step := 0; step < 400; step++ {
		lo := rng.IntN(17) - 8
		hi := rng.IntN(17) - 8
		if lo > hi {
			lo, hi = hi, lo
		}
		var r rangeset.Range[int]
		switch rng.IntN(4) {
		case 0:
			r = rangeset.Closed(lo, hi)
		case 1:
			r = rangeset.Open(lo, hi)
		case 2:
			r = rangeset.ClosedOpen(lo, hi)
		default:
			r = rangeset.OpenClosed(lo, hi)
		}
		if rng.IntN(3) == 0 {
			m.Remove(r)
			for x := -12; x <= 12; x++ {
				if r.Contains(x) {
					delete(model, x)
				}
			}
		} else {
			value := rng.IntN(9)
			m.Put(r, value)
			for x := -12; x <= 12; x++ {
				if r.Contains(x) {
					model[x] = value
				}
			}
		}
		for x := -12; x <= 12; x++ {
			got, ok := m.Get(x)
			want, wantOK := model[x]
			require.Equalf(t, wantOK, ok, "step %d, key %d, entries %#v", step, x, m.Entries())
			if wantOK {
				require.Equalf(t, want, got, "step %d, key %d", step, x)
			}
		}
		assertInvariants(t, m)
	}
}

func assertInvariants(t *testing.T, m *rangemap.RangeMap[int, int]) {
	t.Helper()
	entries := m.Entries()
	for i, entry := range entries {
		require.False(t, entry.Range.IsEmpty())
		if i > 0 {
			_, intersects := entries[i-1].Range.Intersection(entry.Range)
			require.Falsef(t, intersects, "overlapping entries: %v and %v", entries[i-1].Range, entry.Range)
			prevLo, _, prevOK := entries[i-1].Range.LowerBound()
			lo, _, ok := entry.Range.LowerBound()
			if prevOK && ok {
				require.LessOrEqual(t, prevLo, lo)
			}
		}
	}
}
