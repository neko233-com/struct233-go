package rangemap_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/rangemap"
	"github.com/neko233-com/struct233-go/rangeset"
)

// rmPair is one range/value mapping fed to PutAll in the scripts below.
type rmPair struct {
	r     rangeset.Range[int64]
	value int64
}

// rmOp is one scripted mutation replayed against the range map and the
// reference model.
type rmOp struct {
	kind  string // "put", "remove", "clear", "putAll", "putNil"
	r     rangeset.Range[int64]
	value int64
	pairs []rmPair
}

func rmPut(r rangeset.Range[int64], value int64) rmOp { return rmOp{kind: "put", r: r, value: value} }

func rmRemove(r rangeset.Range[int64]) rmOp { return rmOp{kind: "remove", r: r} }

func rmClear() rmOp { return rmOp{kind: "clear"} }

func rmPutAll(pairs ...rmPair) rmOp { return rmOp{kind: "putAll", pairs: pairs} }

func rmPutNil() rmOp { return rmOp{kind: "putNil"} }

// ranges exposes every range carried by the op for sample-point collection.
func (o rmOp) ranges() []rangeset.Range[int64] {
	switch o.kind {
	case "put", "remove":
		return []rangeset.Range[int64]{o.r}
	case "putAll":
		out := make([]rangeset.Range[int64], 0, len(o.pairs))
		for _, p := range o.pairs {
			out = append(out, p.r)
		}
		return out
	default:
		return nil
	}
}

// renderRM prints the stored entries for golden strings in the tables below.
func renderRM(m *rangemap.RangeMap[int64, int64]) string {
	entries := m.Entries()
	if len(entries) == 0 {
		return "empty"
	}
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, fmt.Sprintf("%s=%d", e.Range.String(), e.Value))
	}
	return strings.Join(parts, " ")
}

// rmPointModel is the brute-force reference implementation: a point-sampled
// mapping over every endpoint neighbourhood and interior point of the ranges
// the script touches.
type rmPointModel struct {
	value  map[int64]int64
	mapped map[int64]bool
}

func newRMPointModel() *rmPointModel {
	return &rmPointModel{value: map[int64]int64{}, mapped: map[int64]bool{}}
}

func (m *rmPointModel) put(r rangeset.Range[int64], value int64, points []int64) {
	for _, p := range points {
		if r.Contains(p) {
			m.value[p] = value
			m.mapped[p] = true
		}
	}
}

func (m *rmPointModel) remove(r rangeset.Range[int64], points []int64) {
	for _, p := range points {
		if r.Contains(p) {
			delete(m.value, p)
			delete(m.mapped, p)
		}
	}
}

func (m *rmPointModel) clear() {
	m.value = map[int64]int64{}
	m.mapped = map[int64]bool{}
}

// rmSamplePoints collects every point the differential check probes: all range
// endpoints +-2, interior points of each range, and a few far-flung sentinels.
// Because every boundary is sampled on both sides, no two distinct entry
// boundaries can hide between consecutive sample points.
func rmSamplePoints(ops []rmOp) []int64 {
	seen := map[int64]bool{}
	var out []int64
	add := func(v int64) {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	for _, o := range ops {
		for _, r := range o.ranges() {
			lo, _, loOK := r.LowerBound()
			hi, _, hiOK := r.UpperBound()
			if loOK {
				for d := int64(-2); d <= 2; d++ {
					add(lo + d)
				}
			}
			if hiOK {
				for d := int64(-2); d <= 2; d++ {
					add(hi + d)
				}
			}
			if loOK && hiOK && hi >= lo {
				add(lo + (hi-lo)/2)
			}
			if loOK {
				add(lo + 1)
			}
			if hiOK {
				add(hi - 1)
			}
		}
	}
	for _, v := range []int64{-1 << 40, -1000, 0, 1000, 1 << 40} {
		add(v)
	}
	slices.Sort(out)
	return out
}

// lessRMLower orders ranges by the documented entry order: unbounded starts
// first, then by lower value, closed ends before open ends on a tie.
func lessRMLower(a, b rangeset.Range[int64]) bool {
	av, ac, aOK := a.LowerBound()
	bv, bc, bOK := b.LowerBound()
	if !aOK || !bOK {
		return !aOK && bOK
	}
	if av != bv {
		return av < bv
	}
	return ac && !bc
}

// requireRMShape asserts the structural invariant of every RangeMap: every
// entry non-empty, ascending lower bounds and pairwise disjoint.
func requireRMShape(t *testing.T, m *rangemap.RangeMap[int64, int64], where string) {
	t.Helper()
	entries := m.Entries()
	require.Equalf(t, len(entries), m.Len(), "%s: Len", where)
	require.Equalf(t, len(entries) == 0, m.IsEmpty(), "%s: IsEmpty", where)
	for i, e := range entries {
		require.Falsef(t, e.Range.IsEmpty(), "%s: entry %d is empty: %v", where, i, e.Range)
		if i == 0 {
			continue
		}
		require.Falsef(t, lessRMLower(e.Range, entries[i-1].Range),
			"%s: entries %d and %d are out of order: %v before %v", where, i-1, i, entries[i-1].Range, e.Range)
		_, intersects := entries[i-1].Range.Intersection(e.Range)
		require.Falsef(t, intersects, "%s: entries %d and %d overlap: %v and %v",
			where, i-1, i, entries[i-1].Range, e.Range)
	}
}

// checkRM asserts that m agrees with the point model on Get and GetEntry at
// every sample point, and that the stored entries expand back to exactly the
// modelled points with the modelled values.
func checkRM(t *testing.T, m *rangemap.RangeMap[int64, int64], model *rmPointModel, points []int64, where string) {
	t.Helper()
	requireRMShape(t, m, where)

	covered := map[int64]bool{}
	for i, e := range m.Entries() {
		var inside []int64
		for _, p := range points {
			if e.Range.Contains(p) {
				inside = append(inside, p)
			}
		}
		for _, p := range inside {
			require.Truef(t, model.mapped[p], "%s: entry %d %s covers modelled-absent point %d", where, i, e.Range, p)
			require.Equalf(t, model.value[p], e.Value,
				"%s: entry %d %s has value %d but the model says %d at point %d", where, i, e.Range, e.Value, model.value[p], p)
			covered[p] = true
		}
	}

	for _, p := range points {
		got, ok := m.Get(p)
		want, wantOK := model.value[p], model.mapped[p]
		require.Equalf(t, wantOK, ok, "%s: Get(%d) ok; entries=%s", where, p, renderRM(m))
		require.Equalf(t, want, got, "%s: Get(%d) value; entries=%s", where, p, renderRM(m))

		entry, entryOK := m.GetEntry(p)
		require.Equalf(t, wantOK, entryOK, "%s: GetEntry(%d) ok; entries=%s", where, p, renderRM(m))
		if entryOK {
			require.Truef(t, entry.Range.Contains(p), "%s: GetEntry(%d) returned %s which does not contain the key", where, p, entry.Range)
			require.Equalf(t, want, entry.Value, "%s: GetEntry(%d) value", where, p)
		}

		require.Equalf(t, wantOK, covered[p],
			"%s: point %d covered by entries = %t, model mapped = %t; entries=%s", where, p, covered[p], wantOK, renderRM(m))
	}
}

// runRMScript replays ops against a real range map and the point model,
// checking the full agreement after every single mutation.
func runRMScript(t *testing.T, zeroValue bool, ops []rmOp) (*rangemap.RangeMap[int64, int64], *rmPointModel) {
	t.Helper()
	var zero rangemap.RangeMap[int64, int64]
	m := &zero
	if !zeroValue {
		m = rangemap.New[int64, int64]()
	}
	model := newRMPointModel()
	points := rmSamplePoints(ops)
	for i, o := range ops {
		switch o.kind {
		case "put":
			m.Put(o.r, o.value)
			model.put(o.r, o.value, points)
		case "remove":
			m.Remove(o.r)
			model.remove(o.r, points)
		case "clear":
			m.Clear()
			model.clear()
		case "putAll":
			other := rangemap.New[int64, int64]()
			for _, p := range o.pairs {
				other.Put(p.r, p.value)
			}
			m.PutAll(other)
			for _, p := range o.pairs {
				model.put(p.r, p.value, points)
			}
		case "putNil":
			m.PutAll(nil)
		}
		checkRM(t, m, model, points, fmt.Sprintf("op %d (%s %s)", i, o.kind, renderRM(m)))
	}
	return m, model
}

// TestRangeMapDeep_Scripts replays scripted mutations and compares the stored
// entries against golden strings while the point model is checked after every
// op. Together the cases cover every exported mutation on normal, boundary and
// empty inputs.
func TestRangeMapDeep_Scripts(t *testing.T) {
	tests := []struct {
		name string
		zero bool
		ops  []rmOp
		want string
	}{
		{"empty map", false, nil, "empty"},
		{"put single closed", false, []rmOp{rmPut(rangeset.Closed[int64](1, 10), 1)}, "[1,10]=1"},
		{"put splits middle", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 10), 1), rmPut(rangeset.ClosedOpen[int64](4, 7), 2),
		}, "[1,4)=1 [4,7)=2 [7,10]=1"},
		{"overwrite same range", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 5), 1), rmPut(rangeset.Closed[int64](1, 5), 2),
		}, "[1,5]=2"},
		{"overwrite exact middle", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 10), 1), rmPut(rangeset.Closed[int64](4, 6), 2),
		}, "[1,4)=1 [4,6]=2 (6,10]=1"},
		{"empty range put ignored", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 10), 1), rmPut(rangeset.Open[int64](3, 3), 2),
		}, "[1,10]=1"},
		{"inverted range put ignored", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 10), 1), rmPut(rangeset.Closed[int64](5, 4), 2),
		}, "[1,10]=1"},
		{"empty remove ignored", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 10), 1), rmRemove(rangeset.Closed[int64](4, 3)),
		}, "[1,10]=1"},
		{"remove middle splits", false, []rmOp{
			rmPut(rangeset.Closed[int64](0, 20), 1), rmRemove(rangeset.Closed[int64](5, 15)),
		}, "[0,5)=1 (15,20]=1"},
		{"remove open middle keeps ends", false, []rmOp{
			rmPut(rangeset.Closed[int64](0, 20), 1), rmRemove(rangeset.Open[int64](5, 15)),
		}, "[0,5]=1 [15,20]=1"},
		{"remove left trim", false, []rmOp{
			rmPut(rangeset.Closed[int64](0, 20), 1), rmRemove(rangeset.AtMost[int64](5)),
		}, "(5,20]=1"},
		{"remove right trim", false, []rmOp{
			rmPut(rangeset.Closed[int64](0, 20), 1), rmRemove(rangeset.AtLeast[int64](15)),
		}, "[0,15)=1"},
		{"remove all", false, []rmOp{
			rmPut(rangeset.Closed[int64](0, 20), 1), rmRemove(rangeset.All[int64]()),
		}, "empty"},
		{"remove disjoint no-op", false, []rmOp{
			rmPut(rangeset.Closed[int64](0, 20), 1), rmRemove(rangeset.Closed[int64](30, 40)),
		}, "[0,20]=1"},
		{"remove singleton splits", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 5), 1), rmRemove(rangeset.Singleton[int64](3)),
		}, "[1,3)=1 (3,5]=1"},
		{"put all range", false, []rmOp{rmPut(rangeset.All[int64](), 1)}, "(-inf,+inf)=1"},
		{"put open closed over all", false, []rmOp{
			rmPut(rangeset.All[int64](), 1), rmPut(rangeset.OpenClosed[int64](-2, 2), 2),
		}, "(-inf,-2]=1 (-2,2]=2 (2,+inf)=1"},
		{"open ends leave gap", false, []rmOp{
			rmPut(rangeset.ClosedOpen[int64](0, 1), 1), rmPut(rangeset.OpenClosed[int64](1, 2), 2),
		}, "[0,1)=1 (1,2]=2"},
		{"put singleton at boundary", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 5), 1), rmPut(rangeset.Singleton[int64](1), 2),
		}, "[1,1]=2 (1,5]=1"},
		{"put at most over at least", false, []rmOp{
			rmPut(rangeset.AtLeast[int64](0), 1), rmPut(rangeset.AtMost[int64](5), 2),
		}, "(-inf,5]=2 (5,+inf)=1"},
		{"three puts middle overwrite", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 10), 1), rmPut(rangeset.Closed[int64](3, 5), 2), rmPut(rangeset.Closed[int64](4, 8), 3),
		}, "[1,3)=1 [3,4)=2 [4,8]=3 (8,10]=1"},
		{"put extends over two entries", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 3), 1), rmPut(rangeset.Closed[int64](7, 9), 2), rmPut(rangeset.Closed[int64](2, 8), 3),
		}, "[1,2)=1 [2,8]=3 (8,9]=2"},
		{"put greater than", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 10), 1), rmPut(rangeset.GreaterThan[int64](5), 2),
		}, "[1,5]=1 (5,+inf)=2"},
		{"put less than", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 10), 1), rmPut(rangeset.LessThan[int64](5), 2),
		}, "(-inf,5)=2 [5,10]=1"},
		{"remove greater than from all", false, []rmOp{
			rmPut(rangeset.All[int64](), 1), rmRemove(rangeset.GreaterThan[int64](0)),
		}, "(-inf,0]=1"},
		{"remove at least from all", false, []rmOp{
			rmPut(rangeset.All[int64](), 1), rmRemove(rangeset.AtLeast[int64](0)),
		}, "(-inf,0)=1"},
		{"two values remove across", false, []rmOp{
			rmPut(rangeset.Closed[int64](0, 5), 1), rmPut(rangeset.Closed[int64](5, 10), 2), rmRemove(rangeset.Closed[int64](3, 7)),
		}, "[0,3)=1 (7,10]=2"},
		{"remove everything then put", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 3), 1), rmRemove(rangeset.All[int64]()), rmPut(rangeset.Closed[int64](5, 7), 2),
		}, "[5,7]=2"},
		{"clear then put", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 3), 1), rmClear(), rmPut(rangeset.Closed[int64](2, 4), 2),
		}, "[2,4]=2"},
		{"clear empty", false, []rmOp{rmClear()}, "empty"},
		{"zero value put", true, []rmOp{rmPut(rangeset.Closed[int64](1, 2), 1)}, "[1,2]=1"},
		{"zero value remove no-op", true, []rmOp{rmRemove(rangeset.All[int64]())}, "empty"},
		{"zero value clear", true, []rmOp{rmClear()}, "empty"},
		{"zero value split", true, []rmOp{
			rmPut(rangeset.Closed[int64](0, 10), 1), rmPut(rangeset.Closed[int64](4, 6), 2),
		}, "[0,4)=1 [4,6]=2 (6,10]=1"},
		{"singleton chain", false, []rmOp{
			rmPut(rangeset.Singleton[int64](1), 1), rmPut(rangeset.Singleton[int64](2), 2), rmPut(rangeset.Singleton[int64](3), 3),
		}, "[1,1]=1 [2,2]=2 [3,3]=3"},
		{"numerically adjacent stays split", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 3), 1), rmPut(rangeset.Closed[int64](4, 6), 2),
		}, "[1,3]=1 [4,6]=2"},
		{"closed touching split by value", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 3), 1), rmPut(rangeset.Closed[int64](3, 6), 2),
		}, "[1,3)=1 [3,6]=2"},
		{"open touching stays split", false, []rmOp{
			rmPut(rangeset.ClosedOpen[int64](1, 3), 1), rmPut(rangeset.OpenClosed[int64](3, 6), 2),
		}, "[1,3)=1 (3,6]=2"},
		{"putAll disjoint appends", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 2), 1), rmPutAll(rmPair{rangeset.Closed[int64](5, 6), 3}),
		}, "[1,2]=1 [5,6]=3"},
		{"putAll overlapping overwrites middle", false, []rmOp{
			rmPut(rangeset.Closed[int64](0, 10), 1), rmPutAll(rmPair{rangeset.Closed[int64](3, 7), 2}),
		}, "[0,3)=1 [3,7]=2 (7,10]=1"},
		{"putAll nil source", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 2), 1), rmPutNil(),
		}, "[1,2]=1"},
		{"putAll empty source", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 2), 1), rmPutAll(),
		}, "[1,2]=1"},
		{"putAll open split over all", false, []rmOp{
			rmPut(rangeset.All[int64](), 1), rmPutAll(rmPair{rangeset.ClosedOpen[int64](0, 1), 2}),
		}, "(-inf,0)=1 [0,1)=2 [1,+inf)=1"},
		{"remove open interval keeps ends", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 5), 1), rmRemove(rangeset.Open[int64](2, 3)),
		}, "[1,2]=1 [3,5]=1"},
		{"put open closed split", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 10), 1), rmPut(rangeset.OpenClosed[int64](4, 7), 2),
		}, "[1,4]=1 (4,7]=2 (7,10]=1"},
		{"put closed open split", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 10), 1), rmPut(rangeset.ClosedOpen[int64](4, 7), 2),
		}, "[1,4)=1 [4,7)=2 [7,10]=1"},
		{"put fills gap between entries", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 3), 1), rmPut(rangeset.Closed[int64](7, 9), 2), rmPut(rangeset.Closed[int64](3, 7), 3),
		}, "[1,3)=1 [3,7]=3 (7,9]=2"},
		{"remove across gaps", false, []rmOp{
			rmPut(rangeset.Closed[int64](1, 3), 1), rmPut(rangeset.Closed[int64](7, 9), 2), rmRemove(rangeset.Closed[int64](2, 8)),
		}, "[1,2)=1 (8,9]=2"},
		{"overwrite then remove middle", false, []rmOp{
			rmPut(rangeset.Closed[int64](0, 20), 1), rmPut(rangeset.Closed[int64](5, 15), 2), rmRemove(rangeset.Open[int64](8, 12)),
		}, "[0,5)=1 [5,8]=2 [12,15]=2 (15,20]=1"},
		{"put at least over at most", false, []rmOp{
			rmPut(rangeset.AtMost[int64](0), 1), rmPut(rangeset.AtLeast[int64](0), 2),
		}, "(-inf,0)=1 [0,+inf)=2"},
		{"put greater than over less than", false, []rmOp{
			rmPut(rangeset.LessThan[int64](5), 1), rmPut(rangeset.GreaterThan[int64](5), 2),
		}, "(-inf,5)=1 (5,+inf)=2"},
		{"remove singleton from all", false, []rmOp{
			rmPut(rangeset.All[int64](), 1), rmPut(rangeset.Singleton[int64](5), 2), rmRemove(rangeset.Singleton[int64](5)),
		}, "(-inf,5)=1 (5,+inf)=1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := runRMScript(t, tt.zero, tt.ops)
			require.Equal(t, tt.want, renderRM(m))
		})
	}
}

// TestRangeMapDeep_SubRangeMap covers view construction on boundaries.
func TestRangeMapDeep_SubRangeMap(t *testing.T) {
	tests := []struct {
		name  string
		ops   []rmOp
		sub   rangeset.Range[int64]
		want  string
		spans int
	}{
		{
			name:  "empty subrange",
			ops:   []rmOp{rmPut(rangeset.Closed[int64](1, 10), 1)},
			sub:   rangeset.Open[int64](3, 3),
			want:  "empty",
			spans: 0,
		},
		{
			name:  "contained",
			ops:   []rmOp{rmPut(rangeset.Closed[int64](1, 10), 1)},
			sub:   rangeset.Closed[int64](3, 5),
			want:  "[3,5]=1",
			spans: 1,
		},
		{
			name: "straddling boundaries",
			ops: []rmOp{
				rmPut(rangeset.Closed[int64](1, 10), 1), rmPut(rangeset.Closed[int64](4, 7), 2),
			},
			sub:   rangeset.Closed[int64](2, 8),
			want:  "[2,4)=1 [4,7]=2 (7,8]=1",
			spans: 3,
		},
		{
			name:  "outside left",
			ops:   []rmOp{rmPut(rangeset.Closed[int64](5, 10), 1)},
			sub:   rangeset.Closed[int64](0, 3),
			want:  "empty",
			spans: 0,
		},
		{
			name:  "outside right",
			ops:   []rmOp{rmPut(rangeset.Closed[int64](5, 10), 1)},
			sub:   rangeset.Closed[int64](20, 30),
			want:  "empty",
			spans: 0,
		},
		{
			name: "unbounded sub keeps all",
			ops: []rmOp{
				rmPut(rangeset.Closed[int64](1, 3), 1), rmPut(rangeset.Closed[int64](5, 7), 2),
			},
			sub:   rangeset.All[int64](),
			want:  "[1,3]=1 [5,7]=2",
			spans: 2,
		},
		{
			name:  "open bounds excluded",
			ops:   []rmOp{rmPut(rangeset.Closed[int64](0, 10), 1)},
			sub:   rangeset.Open[int64](0, 10),
			want:  "(0,10)=1",
			spans: 1,
		},
		{
			name:  "empty source map",
			ops:   nil,
			sub:   rangeset.Closed[int64](0, 10),
			want:  "empty",
			spans: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := runRMScript(t, false, tt.ops)
			sub := m.SubRangeMap(tt.sub)
			require.Equal(t, tt.want, renderRM(sub))
			requireRMShape(t, sub, "subrange map")

			// mutating the view must not touch the source map
			before := renderRM(m)
			sub.Put(rangeset.All[int64](), 99)
			sub.Remove(rangeset.All[int64]())
			require.Equal(t, before, renderRM(m), "SubRangeMap must not share storage")
		})
	}
}

// TestRangeMapDeep_Span covers the enclosing-interval query.
func TestRangeMapDeep_Span(t *testing.T) {
	tests := []struct {
		name string
		ops  []rmOp
		want string
		ok   bool
	}{
		{"empty map", nil, "()", false},
		{"single entry", []rmOp{rmPut(rangeset.Closed[int64](3, 5), 1)}, "[3,5]", true},
		{"two entries with gap", []rmOp{
			rmPut(rangeset.Closed[int64](1, 3), 1), rmPut(rangeset.Closed[int64](5, 10), 2),
		}, "[1,10]", true},
		{"unbounded entries", []rmOp{
			rmPut(rangeset.LessThan[int64](0), 1), rmPut(rangeset.AtLeast[int64](5), 2),
		}, "(-inf,+inf)", true},
		{"open ends widen", []rmOp{
			rmPut(rangeset.Open[int64](1, 3), 1), rmPut(rangeset.Open[int64](7, 9), 2),
		}, "(1,9)", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := runRMScript(t, false, tt.ops)
			span, ok := m.Span()
			require.Equal(t, tt.ok, ok)
			if tt.ok {
				require.Equal(t, tt.want, span.String())
			}
		})
	}
}

// TestRangeMapDeep_GetEntryBoundaries pins the binary-search boundary
// behaviour on open, closed and unbounded ends.
func TestRangeMapDeep_GetEntryBoundaries(t *testing.T) {
	// entries: [1,4)=a [4,7]=b (7,10]=c ... plus a detached point via open ends
	ops := []rmOp{
		rmPut(rangeset.Closed[int64](1, 10), 1),
		rmPut(rangeset.Closed[int64](4, 7), 2),
		rmPut(rangeset.OpenClosed[int64](20, 21), 3),
		rmPut(rangeset.AtLeast[int64](30), 4),
	}
	tests := []struct {
		name string
		key  int64
		want string // empty string means "not mapped"
	}{
		{"below everything", 0, ""},
		{"first entry closed lower", 1, "[1,4)"},
		{"first entry interior", 3, "[1,4)"},
		{"second entry closed lower", 4, "[4,7]"},
		{"second entry interior", 6, "[4,7]"},
		{"second entry closed upper", 7, "[4,7]"}, // 7 belongs to the closed upper end
		{"third entry interior", 8, "(7,10]"},
		{"third entry closed upper", 10, "(7,10]"},
		{"gap between entries", 15, ""},
		{"open lower excluded", 20, ""}, // 20 is the open bound itself
		{"open lower entry interior", 21, "(20,21]"},
		{"after short entry", 22, ""},
		{"unbounded lower entry", 30, "[30,+inf)"},
		{"unbounded interior", 1 << 30, "[30,+inf)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := runRMScript(t, false, ops)
			entry, ok := m.GetEntry(tt.key)
			if tt.want == "" {
				require.False(t, ok, "GetEntry(%d) must miss", tt.key)
				_, gotOK := m.Get(tt.key)
				require.False(t, gotOK)
				return
			}
			require.True(t, ok)
			require.Equal(t, tt.want, entry.Range.String())
			value, gotOK := m.Get(tt.key)
			require.True(t, gotOK)
			require.Equal(t, entry.Value, value)
		})
	}
}

// TestRangeMapDeep_EmptyRangeIsNoOp pins that empty ranges never map anything.
func TestRangeMapDeep_EmptyRangeIsNoOp(t *testing.T) {
	empties := []rangeset.Range[int64]{
		rangeset.Closed[int64](5, 4),
		rangeset.Open[int64](3, 3),
		rangeset.ClosedOpen[int64](2, 2),
		rangeset.OpenClosed[int64](2, 2),
	}
	for i, r := range empties {
		t.Run(fmt.Sprintf("empty range %d", i), func(t *testing.T) {
			require.True(t, r.IsEmpty())
			m := rangemap.New[int64, int64]()
			m.Put(r, 1)
			require.True(t, m.IsEmpty(), "Put of an empty range must be a no-op")
			m.Put(rangeset.Closed[int64](0, 10), 2)
			m.Remove(r)
			require.Equal(t, "[0,10]=2", renderRM(m), "Remove of an empty range must be a no-op")
		})
	}
}

// TestRangeMapDeep_NonPanickingBoundaries pins that no table or range map call
// path panics: the packages document no panics, so there is no panic message
// prefix to assert on.
func TestRangeMapDeep_NonPanickingBoundaries(t *testing.T) {
	require.NotPanics(t, func() {
		var zero rangemap.RangeMap[int64, int64]
		zero.Put(rangeset.Closed[int64](1, 2), 1)
		_, _ = zero.Get(1)
		_, _ = zero.GetEntry(1)
		_, _ = zero.Span()
		zero.Remove(rangeset.All[int64]())
		zero.PutAll(nil)
		zero.Clear()
		zero.Clear()
		_ = zero.Entries()
		_ = zero.AsMapOfRanges()
		_ = zero.SubRangeMap(rangeset.Open[int64](0, 0))
		for range zero.All() {
		}
	}, "every boundary call must be a no-op instead of a panic")
}

// TestRangeMapDeep_SnapshotsAreDetached mutates every returned snapshot and
// requires the map to stay untouched.
func TestRangeMapDeep_SnapshotsAreDetached(t *testing.T) {
	m := rangemap.New[int64, int64]()
	m.Put(rangeset.Closed[int64](1, 5), 1)
	m.Put(rangeset.Closed[int64](7, 9), 2)
	before := renderRM(m)

	entries := m.Entries()
	entries[0].Value = 99
	asMap := m.AsMapOfRanges()
	for k := range asMap {
		asMap[k] = 99
		delete(asMap, k)
	}
	require.Equal(t, before, renderRM(m), "no snapshot may mutate the map")

	// All agrees with Entries and supports early stop
	var got []rangemap.Entry[int64, int64]
	for r, v := range m.All() {
		got = append(got, rangemap.Entry[int64, int64]{Range: r, Value: v})
	}
	require.Equal(t, m.Entries(), got)
	seen := 0
	for range m.All() {
		seen++
		break
	}
	require.Equal(t, 1, seen, "break must stop the iteration")
}

// TestRangeMapDeep_ModelRandomized replays random Put/Remove/Get traffic
// against the point-sampling model. This is the differential test: every
// endpoint +-epsilon and every interior point is compared against Get after
// every single mutation.
func TestRangeMapDeep_ModelRandomized(t *testing.T) {
	for _, seed := range [][2]uint64{{2026, 1012}, {7, 11}, {99, 4242}, {1, 0}} {
		rng := rand.New(rand.NewPCG(seed[0], seed[1]))
		data := make([]byte, 6*maxRMOps)
		for i := range data {
			data[i] = byte(rng.IntN(256))
		}
		runRMScript(t, false, decodeRMOps(data))
	}
}

// maxRMOps bounds the replay length so fuzzing stays fast.
const maxRMOps = 64

// decodeRMOps turns arbitrary bytes into mutations, six bytes per mutation:
// the kind, the constructor and the two endpoints plus the value. Endpoints
// are int64 values drawn from a small domain so overlaps and boundary reuse
// happen often.
func decodeRMOps(data []byte) []rmOp {
	ops := make([]rmOp, 0, maxRMOps)
	for i := 0; i+6 <= len(data) && len(ops) < maxRMOps; i += 6 {
		lo, hi := int64(int8(data[i+2])), int64(int8(data[i+3]))
		value := int64(int8(data[i+4]))
		var r rangeset.Range[int64]
		switch data[i+1] % 10 {
		case 0:
			r = rangeset.Closed[int64](lo, hi)
		case 1:
			r = rangeset.Open[int64](lo, hi)
		case 2:
			r = rangeset.OpenClosed[int64](lo, hi)
		case 3:
			r = rangeset.ClosedOpen[int64](lo, hi)
		case 4:
			r = rangeset.AtLeast[int64](lo)
		case 5:
			r = rangeset.AtMost[int64](hi)
		case 6:
			r = rangeset.GreaterThan[int64](lo)
		case 7:
			r = rangeset.LessThan[int64](hi)
		case 8:
			r = rangeset.Singleton[int64](lo)
		default:
			r = rangeset.All[int64]()
		}
		switch data[i] % 3 {
		case 0:
			ops = append(ops, rmOp{kind: "put", r: r, value: value})
		case 1:
			ops = append(ops, rmOp{kind: "remove", r: r})
		default:
			ops = append(ops, rmOp{kind: "clear"})
		}
	}
	return ops
}

// TestRangeMapDeep_GetDoesNotAllocate guards the read hot path: Get and
// GetEntry must not allocate on a populated map.
func TestRangeMapDeep_GetDoesNotAllocate(t *testing.T) {
	m := rangemap.New[int64, int64]()
	for i := int64(0); i < 512; i++ {
		m.Put(rangeset.Closed[int64](3*i, 3*i+1), i)
	}

	var value int64
	var ok bool
	allocs := testing.AllocsPerRun(100, func() {
		value, ok = m.Get(3*100 + 1)
	})
	require.Zero(t, allocs, "Get must not allocate")
	require.True(t, ok)
	require.Equal(t, int64(100), value)

	var entry rangemap.Entry[int64, int64]
	allocs = testing.AllocsPerRun(100, func() {
		entry, ok = m.GetEntry(3*100 + 1)
	})
	require.Zero(t, allocs, "GetEntry must not allocate")
	require.True(t, ok)
	require.Equal(t, rangeset.Closed[int64](300, 301), entry.Range)
}
