package rangeset_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/rangeset"
)

func TestRange_Bounds(t *testing.T) {
	tests := []struct {
		name           string
		r              rangeset.Range[int]
		empty          bool
		loVal          int
		loClosed, loOK bool
		hiVal          int
		hiClosed, hiOK bool
	}{
		{"closed", rangeset.Closed(1, 3), false, 1, true, true, 3, true, true},
		{"open", rangeset.Open(1, 3), false, 1, false, true, 3, false, true},
		{"open closed", rangeset.OpenClosed(1, 3), false, 1, false, true, 3, true, true},
		{"closed open", rangeset.ClosedOpen(1, 3), false, 1, true, true, 3, false, true},
		{"at least", rangeset.AtLeast(2), false, 2, true, true, 0, false, false},
		{"at most", rangeset.AtMost(2), false, 0, false, false, 2, true, true},
		{"greater than", rangeset.GreaterThan(2), false, 2, false, true, 0, false, false},
		{"less than", rangeset.LessThan(2), false, 0, false, false, 2, false, true},
		{"singleton", rangeset.Singleton(5), false, 5, true, true, 5, true, true},
		{"all", rangeset.All[int](), false, 0, false, false, 0, false, false},
		{"inverted closed", rangeset.Closed(3, 1), true, 0, false, false, 0, false, false},
		{"degenerate open", rangeset.Open(1, 1), true, 0, false, false, 0, false, false},
		{"degenerate closed open", rangeset.ClosedOpen(2, 2), true, 0, false, false, 0, false, false},
		{"degenerate open closed", rangeset.OpenClosed(2, 2), true, 0, false, false, 0, false, false},
		{"single point closed", rangeset.Closed(1, 1), false, 1, true, true, 1, true, true},
		{"negative", rangeset.Closed(-5, -1), false, -5, true, true, -1, true, true},
		{"inverted open", rangeset.Open(4, 2), true, 0, false, false, 0, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.empty, tt.r.IsEmpty())
			gotLo, gotLoClosed, gotLoOK := tt.r.LowerBound()
			require.Equal(t, tt.loVal, gotLo)
			require.Equal(t, tt.loClosed, gotLoClosed)
			require.Equal(t, tt.loOK, gotLoOK)
			gotHi, gotHiClosed, gotHiOK := tt.r.UpperBound()
			require.Equal(t, tt.hiVal, gotHi)
			require.Equal(t, tt.hiClosed, gotHiClosed)
			require.Equal(t, tt.hiOK, gotHiOK)
		})
	}
}

func TestRange_Contains(t *testing.T) {
	tests := []struct {
		name string
		r    rangeset.Range[int]
		v    int
		want bool
	}{
		{"closed lower", rangeset.Closed(1, 3), 1, true},
		{"closed middle", rangeset.Closed(1, 3), 2, true},
		{"closed upper", rangeset.Closed(1, 3), 3, true},
		{"closed below", rangeset.Closed(1, 3), 0, false},
		{"closed above", rangeset.Closed(1, 3), 4, false},
		{"open lower", rangeset.Open(1, 3), 1, false},
		{"open middle", rangeset.Open(1, 3), 2, true},
		{"open upper", rangeset.Open(1, 3), 3, false},
		{"open closed lower", rangeset.OpenClosed(1, 3), 1, false},
		{"open closed upper", rangeset.OpenClosed(1, 3), 3, true},
		{"closed open lower", rangeset.ClosedOpen(1, 3), 1, true},
		{"closed open upper", rangeset.ClosedOpen(1, 3), 3, false},
		{"at least lower", rangeset.AtLeast(2), 2, true},
		{"at least below", rangeset.AtLeast(2), 1, false},
		{"at least far above", rangeset.AtLeast(2), 1 << 20, true},
		{"at most upper", rangeset.AtMost(2), 2, true},
		{"at most above", rangeset.AtMost(2), 3, false},
		{"at most far below", rangeset.AtMost(2), -1 << 20, true},
		{"greater than equal", rangeset.GreaterThan(2), 2, false},
		{"greater than above", rangeset.GreaterThan(2), 3, true},
		{"less than equal", rangeset.LessThan(2), 2, false},
		{"less than below", rangeset.LessThan(2), 1, true},
		{"singleton hit", rangeset.Singleton(5), 5, true},
		{"singleton miss below", rangeset.Singleton(5), 4, false},
		{"singleton miss above", rangeset.Singleton(5), 6, false},
		{"all far below", rangeset.All[int](), -1 << 20, true},
		{"all zero", rangeset.All[int](), 0, true},
		{"all far above", rangeset.All[int](), 1 << 20, true},
		{"empty inverted", rangeset.Closed(3, 1), 2, false},
		{"empty inverted end", rangeset.Closed(3, 1), 1, false},
		{"empty open", rangeset.Open(1, 1), 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.r.Contains(tt.v))
		})
	}
}

func TestRange_Encloses(t *testing.T) {
	tests := []struct {
		name string
		r    rangeset.Range[int]
		arg  rangeset.Range[int]
		want bool
	}{
		{"wider", rangeset.Closed(1, 5), rangeset.Closed(2, 4), true},
		{"same", rangeset.Closed(1, 5), rangeset.Closed(1, 5), true},
		{"strictly wider ends", rangeset.Closed(1, 5), rangeset.Open(1, 5), true},
		{"open cannot hold closed", rangeset.Open(1, 5), rangeset.Closed(1, 5), false},
		{"half open upper", rangeset.ClosedOpen(1, 5), rangeset.Closed(1, 5), false},
		{"half open lower", rangeset.OpenClosed(1, 5), rangeset.Closed(1, 5), false},
		{"half open holds own shape", rangeset.OpenClosed(1, 5), rangeset.Open(1, 5), true},
		{"shifted", rangeset.Closed(1, 5), rangeset.Closed(5, 9), false},
		{"unbounded holder", rangeset.AtLeast(0), rangeset.Closed(1, 5), true},
		{"unbounded holder misses below", rangeset.AtLeast(1), rangeset.Closed(0, 5), false},
		{"bounded cannot hold unbounded", rangeset.Closed(1, 5), rangeset.AtMost(5), false},
		{"all holds all", rangeset.All[int](), rangeset.All[int](), true},
		{"empty argument", rangeset.Closed(1, 5), rangeset.Closed(2, 2), true},
		{"empty argument open", rangeset.Closed(1, 5), rangeset.Open(2, 2), true},
		{"empty holder", rangeset.Closed(3, 1), rangeset.Closed(2, 2), false},
		{"empty to empty", rangeset.Closed(3, 1), rangeset.Open(2, 2), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.r.Encloses(tt.arg))
		})
	}
}

func TestRange_IsConnected(t *testing.T) {
	tests := []struct {
		name string
		a    rangeset.Range[int]
		b    rangeset.Range[int]
		want bool
	}{
		{"closed touching", rangeset.Closed(1, 4), rangeset.Closed(4, 7), true},
		{"open lower end touches closed", rangeset.ClosedOpen(1, 4), rangeset.Closed(4, 7), true},
		{"closed meets open upper", rangeset.Closed(1, 4), rangeset.Open(4, 7), true},
		{"open touching open", rangeset.ClosedOpen(1, 4), rangeset.Open(4, 7), false},
		{"separated", rangeset.Closed(1, 4), rangeset.Closed(5, 7), false},
		{"far apart", rangeset.Closed(1, 4), rangeset.Closed(9, 10), false},
		{"overlapping", rangeset.Closed(1, 5), rangeset.Closed(2, 3), true},
		{"nested", rangeset.Closed(1, 5), rangeset.Closed(0, 9), true},
		{"open gap at a point", rangeset.Open(1, 2), rangeset.Open(2, 3), false},
		{"empty argument", rangeset.Closed(1, 5), rangeset.Open(3, 3), false},
		{"empty receiver", rangeset.Closed(3, 1), rangeset.Closed(1, 5), false},
		{"unbounded overlap", rangeset.All[int](), rangeset.Closed(1, 2), true},
		{"unbounded split", rangeset.LessThan(5), rangeset.GreaterThan(5), false},
		{"unbounded closed split", rangeset.AtMost(5), rangeset.AtLeast(5), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.a.IsConnected(tt.b))
			require.Equal(t, tt.want, tt.b.IsConnected(tt.a), "IsConnected must be symmetric")
		})
	}
}

func TestRange_Intersection(t *testing.T) {
	tests := []struct {
		name string
		a    rangeset.Range[int]
		b    rangeset.Range[int]
		want string
		ok   bool
	}{
		{"overlap", rangeset.Closed(1, 5), rangeset.Closed(3, 7), "[3,5]", true},
		{"nested", rangeset.Closed(1, 9), rangeset.Closed(3, 7), "[3,7]", true},
		{"closed touching", rangeset.Closed(1, 3), rangeset.Closed(3, 5), "[3,3]", true},
		{"open touching", rangeset.ClosedOpen(1, 3), rangeset.OpenClosed(3, 5), "()", false},
		{"open end survives", rangeset.Open(1, 5), rangeset.ClosedOpen(3, 5), "[3,5)", true},
		{"open wins over closed", rangeset.Open(1, 5), rangeset.Closed(3, 5), "[3,5)", true},
		{"disjoint", rangeset.Closed(1, 2), rangeset.Closed(5, 6), "()", false},
		{"unbounded lower", rangeset.AtLeast(0), rangeset.Closed(-5, 5), "[0,5]", true},
		{"unbounded both", rangeset.LessThan(5), rangeset.GreaterThan(0), "(0,5)", true},
		{"all with closed", rangeset.All[int](), rangeset.Closed(1, 2), "[1,2]", true},
		{"empty operand", rangeset.Closed(3, 1), rangeset.Closed(1, 5), "()", false},
		{"empty receiver", rangeset.Open(1, 1), rangeset.All[int](), "()", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.a.Intersection(tt.b)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, got.String())
			other, _ := tt.b.Intersection(tt.a)
			require.Equal(t, tt.want, other.String(), "Intersection must be symmetric")
		})
	}
}

func TestRange_Span(t *testing.T) {
	tests := []struct {
		name string
		a    rangeset.Range[int]
		b    rangeset.Range[int]
		want string
	}{
		{"overlapping", rangeset.Closed(1, 5), rangeset.Closed(3, 7), "[1,7]"},
		{"separated", rangeset.Closed(1, 2), rangeset.Closed(5, 6), "[1,6]"},
		{"mixed ends", rangeset.Open(1, 3), rangeset.ClosedOpen(3, 5), "(1,5)"},
		{"tied open lower", rangeset.Open(1, 3), rangeset.OpenClosed(1, 5), "(1,5]"},
		{"tied closed lower wins", rangeset.Open(1, 3), rangeset.ClosedOpen(1, 5), "[1,5)"},
		{"unbounded lower", rangeset.AtLeast(5), rangeset.Closed(1, 2), "[1,+inf)"},
		{"unbounded upper", rangeset.AtMost(1), rangeset.Closed(2, 5), "(-inf,5]"},
		{"both unbounded", rangeset.AtLeast(5), rangeset.AtMost(1), "(-inf,+inf)"},
		{"all", rangeset.All[int](), rangeset.Closed(1, 2), "(-inf,+inf)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.a.Span(tt.b).String())
			require.Equal(t, tt.want, tt.b.Span(tt.a).String(), "Span must be symmetric")
		})
	}
}

func TestRange_SpanOfEmptyPanics(t *testing.T) {
	require.PanicsWithValue(t, "rangeset: span of empty range", func() {
		rangeset.Closed(1, 2).Span(rangeset.Open(3, 3))
	})
	require.PanicsWithValue(t, "rangeset: span of empty range", func() {
		rangeset.Closed(3, 1).Span(rangeset.Closed(1, 2))
	})
}

func TestRange_String(t *testing.T) {
	tests := []struct {
		name string
		r    rangeset.Range[int]
		want string
	}{
		{"closed", rangeset.Closed(1, 3), "[1,3]"},
		{"open", rangeset.Open(1, 3), "(1,3)"},
		{"open closed", rangeset.OpenClosed(1, 3), "(1,3]"},
		{"closed open", rangeset.ClosedOpen(1, 3), "[1,3)"},
		{"at least", rangeset.AtLeast(1), "[1,+inf)"},
		{"at most", rangeset.AtMost(3), "(-inf,3]"},
		{"greater than", rangeset.GreaterThan(1), "(1,+inf)"},
		{"less than", rangeset.LessThan(3), "(-inf,3)"},
		{"singleton", rangeset.Singleton(7), "[7,7]"},
		{"all", rangeset.All[int](), "(-inf,+inf)"},
		{"empty inverted", rangeset.Closed(3, 1), "()"},
		{"empty degenerate", rangeset.Open(3, 3), "()"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.r.String())
		})
	}
}

func TestRange_Equal(t *testing.T) {
	tests := []struct {
		name string
		a    rangeset.Range[int]
		b    rangeset.Range[int]
		want bool
	}{
		{"same closed", rangeset.Closed(1, 3), rangeset.Closed(1, 3), true},
		{"singleton alias", rangeset.Singleton(3), rangeset.Closed(3, 3), true},
		{"different closure", rangeset.Closed(1, 3), rangeset.OpenClosed(1, 3), false},
		{"different values", rangeset.Closed(1, 3), rangeset.Closed(1, 4), false},
		{"different unbounded ends", rangeset.AtLeast(1), rangeset.GreaterThan(1), false},
		{"all equal", rangeset.All[int](), rangeset.All[int](), true},
		{"empty variants", rangeset.Closed(3, 1), rangeset.Open(2, 2), true},
		{"empty versus unbounded", rangeset.Closed(3, 1), rangeset.All[int](), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.a.Equal(tt.b))
			require.Equal(t, tt.want, tt.b.Equal(tt.a), "Equal must be symmetric")
		})
	}
}

// spanText renders the stored spans for the expected values in the tables.
func spanText(s *rangeset.RangeSet[int]) string { return fmt.Sprint(s.Spans()) }

// build returns a set holding r.
func build(rs ...rangeset.Range[int]) *rangeset.RangeSet[int] {
	s := rangeset.NewSet[int]()
	for _, r := range rs {
		s.Add(r)
	}
	return s
}

func TestRangeSet_Add(t *testing.T) {
	tests := []struct {
		name string
		have []rangeset.Range[int]
		add  rangeset.Range[int]
		want string
	}{
		{"closed touching coalesces", []rangeset.Range[int]{rangeset.Closed(1, 3)}, rangeset.Closed(3, 6), "[[1,6]]"},
		// the order is dense, so [1,3] and [4,6] are separated by the values
		// between 3 and 4 and must not be merged
		{"numerically adjacent stays split", []rangeset.Range[int]{rangeset.Closed(1, 3)}, rangeset.Closed(4, 6), "[[1,3] [4,6]]"},
		{"overlapping coalesces", []rangeset.Range[int]{rangeset.Closed(1, 5)}, rangeset.Closed(3, 7), "[[1,7]]"},
		{"absorbed", []rangeset.Range[int]{rangeset.Closed(1, 9)}, rangeset.Closed(3, 4), "[[1,9]]"},
		{"open touching stays split", []rangeset.Range[int]{rangeset.ClosedOpen(1, 3)}, rangeset.OpenClosed(3, 6), "[[1,3) (3,6]]"},
		{"open upper meets closed lower merges", []rangeset.Range[int]{rangeset.Open(1, 3)}, rangeset.Closed(3, 5), "[(1,5]]"},
		{"closed touching open merges", []rangeset.Range[int]{rangeset.ClosedOpen(1, 3)}, rangeset.Closed(3, 5), "[[1,5]]"},
		{"open lower closed upper merges", []rangeset.Range[int]{rangeset.Closed(1, 3)}, rangeset.Open(3, 5), "[[1,5)]"},
		{"separated stays split", []rangeset.Range[int]{rangeset.Closed(1, 3)}, rangeset.Closed(5, 6), "[[1,3] [5,6]]"},
		{"insert below", []rangeset.Range[int]{rangeset.Closed(5, 9)}, rangeset.Closed(1, 2), "[[1,2] [5,9]]"},
		{"insert between", []rangeset.Range[int]{rangeset.Closed(1, 2), rangeset.Closed(8, 9)}, rangeset.Closed(4, 5), "[[1,2] [4,5] [8,9]]"},
		{"coalescing both sides", []rangeset.Range[int]{rangeset.Closed(1, 2), rangeset.Closed(5, 6)}, rangeset.Closed(2, 5), "[[1,6]]"},
		{"empty ignored", []rangeset.Range[int]{rangeset.Closed(1, 3)}, rangeset.Closed(5, 4), "[[1,3]]"},
		{"empty open ignored", []rangeset.Range[int]{}, rangeset.Open(2, 2), "[]"},
		{"all swallows", []rangeset.Range[int]{rangeset.Closed(1, 3)}, rangeset.All[int](), "[(-inf,+inf)]"},
		{"unbounded swallows", []rangeset.Range[int]{rangeset.AtLeast(1)}, rangeset.Closed(5, 9), "[[1,+inf)]"},
		{"two unbounded become all", []rangeset.Range[int]{rangeset.AtLeast(1)}, rangeset.AtMost(9), "[(-inf,+inf)]"},
		{"looser upper wins", []rangeset.Range[int]{rangeset.LessThan(9)}, rangeset.AtMost(3), "[(-inf,9)]"},
		{"looser lower wins", []rangeset.Range[int]{rangeset.GreaterThan(0)}, rangeset.Closed(1, 3), "[(0,+inf)]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := build(tt.have...)
			s.Add(tt.add)
			require.Equal(t, tt.want, spanText(s))
			requireInvariant(t, s)
		})
	}
}

func TestRangeSet_Remove(t *testing.T) {
	tests := []struct {
		name string
		have []rangeset.Range[int]
		del  rangeset.Range[int]
		want string
	}{
		{"middle hole", []rangeset.Range[int]{rangeset.Closed(1, 10)}, rangeset.Closed(4, 6), "[[1,4) (6,10]]"},
		{"open middle keeps ends", []rangeset.Range[int]{rangeset.Closed(1, 10)}, rangeset.Open(4, 6), "[[1,4] [6,10]]"},
		{"split by singleton", []rangeset.Range[int]{rangeset.Closed(1, 5)}, rangeset.Singleton(3), "[[1,3) (3,5]]"},
		{"trim left", []rangeset.Range[int]{rangeset.Closed(1, 10)}, rangeset.AtMost(5), "[(5,10]]"},
		{"trim left open", []rangeset.Range[int]{rangeset.Closed(1, 10)}, rangeset.LessThan(5), "[[5,10]]"},
		{"trim right", []rangeset.Range[int]{rangeset.Closed(1, 10)}, rangeset.GreaterThan(5), "[[1,5]]"},
		{"trim right closed", []rangeset.Range[int]{rangeset.Closed(1, 10)}, rangeset.AtLeast(5), "[[1,5)]"},
		{"remove whole span", []rangeset.Range[int]{rangeset.Closed(1, 3)}, rangeset.Closed(1, 3), "[]"},
		{"remove all", []rangeset.Range[int]{rangeset.Closed(1, 3)}, rangeset.All[int](), "[]"},
		{"remove unbounded tail", []rangeset.Range[int]{rangeset.AtLeast(1)}, rangeset.AtLeast(5), "[[1,5)]"},
		{"remove unbounded head", []rangeset.Range[int]{rangeset.AtMost(9)}, rangeset.AtMost(5), "[(5,9]]"},
		{"remove across spans", []rangeset.Range[int]{rangeset.Closed(1, 2), rangeset.Closed(5, 6)}, rangeset.Closed(2, 5), "[[1,2) (5,6]]"},
		{"disjoint no-op", []rangeset.Range[int]{rangeset.Closed(1, 5)}, rangeset.Closed(7, 9), "[[1,5]]"},
		{"empty no-op", []rangeset.Range[int]{rangeset.Closed(1, 5)}, rangeset.Closed(9, 8), "[[1,5]]"},
		{"empty set no-op", []rangeset.Range[int]{}, rangeset.All[int](), "[]"},
		{"middle of split set", []rangeset.Range[int]{rangeset.Closed(1, 2), rangeset.Closed(5, 6)}, rangeset.Closed(5, 5), "[[1,2] (5,6]]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := build(tt.have...)
			s.Remove(tt.del)
			require.Equal(t, tt.want, spanText(s))
			requireInvariant(t, s)
		})
	}
}

func TestRangeSet_Complement(t *testing.T) {
	tests := []struct {
		name string
		have []rangeset.Range[int]
		want string
	}{
		{"empty set", nil, "[(-inf,+inf)]"},
		{"all", []rangeset.Range[int]{rangeset.All[int]()}, "[]"},
		{"closed", []rangeset.Range[int]{rangeset.Closed(1, 3)}, "[(-inf,1) (3,+inf)]"},
		{"open", []rangeset.Range[int]{rangeset.Open(1, 3)}, "[(-inf,1] [3,+inf)]"},
		{"two spans", []rangeset.Range[int]{rangeset.Closed(1, 3), rangeset.Closed(5, 6)}, "[(-inf,1) (3,5) (6,+inf)]"},
		{"singleton", []rangeset.Range[int]{rangeset.Singleton(0)}, "[(-inf,0) (0,+inf)]"},
		{"at least", []rangeset.Range[int]{rangeset.AtLeast(0)}, "[(-inf,0)]"},
		{"less than", []rangeset.Range[int]{rangeset.LessThan(0)}, "[[0,+inf)]"},
		{"at most", []rangeset.Range[int]{rangeset.AtMost(0)}, "[(0,+inf)]"},
		{"greater than", []rangeset.Range[int]{rangeset.GreaterThan(0)}, "[(-inf,0]]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := build(tt.have...)
			c := s.Complement()
			require.Equal(t, tt.want, spanText(c))
			requireInvariant(t, c)
			// complementing twice restores the set
			require.Equal(t, spanText(s), spanText(c.Complement()))
			require.Equal(t, tt.want, spanText(s.Complement()), "Complement must leave s alone")
		})
	}
}

func TestRangeSet_Algebra(t *testing.T) {
	tests := []struct {
		name        string
		left        []rangeset.Range[int]
		right       []rangeset.Range[int]
		inter       string
		union       string
		diff        string
		reverseDiff string
	}{
		{
			name: "overlapping",
			left: []rangeset.Range[int]{rangeset.Closed(1, 5)}, right: []rangeset.Range[int]{rangeset.Closed(3, 7)},
			inter: "[[3,5]]", union: "[[1,7]]", diff: "[[1,3)]", reverseDiff: "[(5,7]]",
		},
		{
			name: "spread",
			left: []rangeset.Range[int]{rangeset.Closed(1, 3), rangeset.Closed(5, 7)}, right: []rangeset.Range[int]{rangeset.Closed(2, 6)},
			inter: "[[2,3] [5,6]]", union: "[[1,7]]", diff: "[[1,2) (6,7]]", reverseDiff: "[(3,5)]",
		},
		{
			name: "disjoint",
			left: []rangeset.Range[int]{rangeset.Closed(1, 2)}, right: []rangeset.Range[int]{rangeset.Closed(5, 6)},
			inter: "[]", union: "[[1,2] [5,6]]", diff: "[[1,2]]", reverseDiff: "[[5,6]]",
		},
		{
			name: "all with span",
			left: []rangeset.Range[int]{rangeset.All[int]()}, right: []rangeset.Range[int]{rangeset.Closed(1, 3)},
			inter: "[[1,3]]", union: "[(-inf,+inf)]", diff: "[(-inf,1) (3,+inf)]", reverseDiff: "[]",
		},
		{
			name: "touching open",
			left: []rangeset.Range[int]{rangeset.ClosedOpen(1, 3)}, right: []rangeset.Range[int]{rangeset.Open(3, 5)},
			inter: "[]", union: "[[1,3) (3,5)]", diff: "[[1,3)]", reverseDiff: "[(3,5)]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left, right := build(tt.left...), build(tt.right...)
			leftBefore, rightBefore := spanText(left), spanText(right)

			inter := left.Intersection(right)
			require.Equal(t, tt.inter, spanText(inter))
			requireInvariant(t, inter)
			uni := left.Union(right)
			require.Equal(t, tt.union, spanText(uni))
			requireInvariant(t, uni)
			diff := left.Difference(right)
			require.Equal(t, tt.diff, spanText(diff))
			requireInvariant(t, diff)
			reverse := right.Difference(left)
			require.Equal(t, tt.reverseDiff, spanText(reverse))
			requireInvariant(t, reverse)

			require.Equal(t, leftBefore, spanText(left), "set algebra must not mutate operands")
			require.Equal(t, rightBefore, spanText(right))
			// union and intersection are commutative
			require.Equal(t, tt.union, spanText(right.Union(left)))
			require.Equal(t, tt.inter, spanText(right.Intersection(left)))
			// (left \ right) + (left & right) == left
			require.Equal(t, leftBefore, spanText(diff.Union(inter)))
		})
	}
}

func TestRangeSet_Queries(t *testing.T) {
	tests := []struct {
		name        string
		have        []rangeset.Range[int]
		r           rangeset.Range[int]
		encloses    bool
		other       []rangeset.Range[int]
		enclosesAll bool
		intersects  bool
	}{
		{
			name: "span inside", have: []rangeset.Range[int]{rangeset.Closed(1, 9)}, r: rangeset.Closed(3, 5),
			encloses: true, other: []rangeset.Range[int]{rangeset.Closed(2, 4)}, enclosesAll: true, intersects: true,
		},
		{
			name: "other pokes out", have: []rangeset.Range[int]{rangeset.Closed(1, 9)}, r: rangeset.Closed(3, 5),
			encloses: true, other: []rangeset.Range[int]{rangeset.Closed(9, 12)}, enclosesAll: false, intersects: true,
		},
		{
			name: "query spans two spans", have: []rangeset.Range[int]{rangeset.Closed(1, 3), rangeset.Closed(5, 6)}, r: rangeset.Closed(2, 5),
			encloses: false, other: []rangeset.Range[int]{rangeset.Closed(6, 9)}, enclosesAll: false, intersects: true,
		},
		{
			name: "touching open is not intersecting", have: []rangeset.Range[int]{rangeset.ClosedOpen(1, 3)}, r: rangeset.Closed(3, 4),
			encloses: false, other: []rangeset.Range[int]{rangeset.Open(3, 9)}, enclosesAll: false, intersects: false,
		},
		{
			name: "unbounded holder", have: []rangeset.Range[int]{rangeset.AtLeast(0)}, r: rangeset.Closed(100, 200),
			encloses: true, other: []rangeset.Range[int]{rangeset.Closed(50, 1<<20)}, enclosesAll: true, intersects: true,
		},
		{
			name: "unbounded holder misses below", have: []rangeset.Range[int]{rangeset.AtLeast(0)}, r: rangeset.Closed(100, 200),
			encloses: true, other: []rangeset.Range[int]{rangeset.Closed(-5, -1)}, enclosesAll: false, intersects: false,
		},
		{
			name: "empty set", have: nil, r: rangeset.Closed(3, 1),
			encloses: true, other: []rangeset.Range[int]{rangeset.Closed(1, 2)}, enclosesAll: false, intersects: false,
		},
		{
			name: "empty other set", have: []rangeset.Range[int]{rangeset.Closed(1, 2)}, r: rangeset.Singleton(2),
			encloses: true, other: nil, enclosesAll: true, intersects: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, other := build(tt.have...), build(tt.other...)
			require.Equal(t, tt.encloses, s.Encloses(tt.r))
			require.Equal(t, tt.enclosesAll, s.EnclosesAll(other))
			require.Equal(t, tt.intersects, s.Intersects(other))
			// a set always encloses itself and each of its own spans
			require.True(t, s.EnclosesAll(s))
			for _, sp := range s.Spans() {
				require.True(t, s.Encloses(sp), "set must enclose its span %v", sp)
			}
		})
	}
}

func TestRangeSet_ContainsAndShape(t *testing.T) {
	s := rangeset.NewSet[int]()
	require.True(t, s.IsEmpty())
	require.Equal(t, 0, s.Len())
	require.Nil(t, s.Spans())

	s.Add(rangeset.Closed(5, 6))
	s.Add(rangeset.Closed(1, 2))
	require.False(t, s.IsEmpty())
	require.Equal(t, 2, s.Len())

	for _, v := range []int{1, 2, 5, 6} {
		require.True(t, s.Contains(v), "Contains(%d)", v)
	}
	for _, v := range []int{-1, 0, 3, 4, 7, 100} {
		require.False(t, s.Contains(v), "Contains(%d)", v)
	}

	// Spans hands out a copy
	spans := s.Spans()
	spans[0] = rangeset.All[int]()
	require.Equal(t, "[[1,2] [5,6]]", spanText(s))

	// All walks the spans in ascending order and stops early
	require.Equal(t, s.Spans(), slices.Collect(s.All()))
	seen := 0
	for range s.All() {
		seen++
		break
	}
	require.Equal(t, 1, seen)

	s.Clear()
	require.True(t, s.IsEmpty())
	require.Equal(t, 0, s.Len())
	require.False(t, s.Contains(1))
}

func TestRangeSet_ZeroValue(t *testing.T) {
	var s rangeset.RangeSet[int]
	require.True(t, s.IsEmpty())
	s.Add(rangeset.Singleton(4))
	require.True(t, s.Contains(4))
	require.Equal(t, 1, s.Len())
	require.Equal(t, "[[4,4]]", spanText(&s))
}

func TestRangeSet_InvariantAfterMutations(t *testing.T) {
	scripts := []struct {
		name string
		ops  []op
	}{
		{"coalescing adds", []op{
			{add: true, r: rangeset.Closed(1, 3)},
			{add: true, r: rangeset.Closed(4, 6)},
			{add: true, r: rangeset.Open(6, 9)},
			{add: true, r: rangeset.Singleton(0)},
			{add: true, r: rangeset.Closed(2, 2)},
		}},
		{"splitting removes", []op{
			{add: true, r: rangeset.Closed(0, 20)},
			{add: false, r: rangeset.Closed(5, 10)},
			{add: false, r: rangeset.Singleton(15)},
			{add: false, r: rangeset.Open(12, 13)},
			{add: false, r: rangeset.LessThan(2)},
			{add: false, r: rangeset.GreaterThan(18)},
		}},
		{"unbounded traffic", []op{
			{add: true, r: rangeset.AtLeast(5)},
			{add: true, r: rangeset.LessThan(5)},
			{add: false, r: rangeset.Open(0, 10)},
			{add: true, r: rangeset.AtMost(3)},
			{add: false, r: rangeset.All[int]()},
			{add: true, r: rangeset.All[int]()},
		}},
		{"alternating", []op{
			{add: true, r: rangeset.ClosedOpen(1, 4)},
			{add: false, r: rangeset.Open(1, 4)},
			{add: false, r: rangeset.Closed(2, 3)},
			{add: false, r: rangeset.Closed(2, 2)},
			{add: true, r: rangeset.OpenClosed(1, 4)},
		}},
	}
	for _, sc := range scripts {
		t.Run(sc.name, func(t *testing.T) {
			runModel(t, sc.ops)
		})
	}
}

// TestRangeSet_ModelRandomized replays random Add/Remove traffic over the
// universe 0..200 against a brute-force model of the same values.
func TestRangeSet_ModelRandomized(t *testing.T) {
	for _, seed := range [][2]uint64{{2026, 1012}, {7, 11}, {99, 4242}} {
		rng := rand.New(rand.NewPCG(seed[0], seed[1]))
		data := make([]byte, 4*maxOps)
		for i := range data {
			data[i] = byte(rng.IntN(256))
		}
		ops := decodeOps(data)
		require.Len(t, ops, maxOps)
		runModel(t, ops)
	}
}

// requireInvariant asserts the structural invariant of every RangeSet: never
// empty spans, strictly ascending, pairwise disjoint and pairwise not touching
// so that no two stored spans could coalesce.
func requireInvariant(t *testing.T, s *rangeset.RangeSet[int]) {
	t.Helper()
	spans := s.Spans()
	require.Len(t, spans, s.Len(), "Len must match the number of spans")
	for i, sp := range spans {
		require.False(t, sp.IsEmpty(), "span %d is empty: %v", i, sp)
		if i == 0 {
			continue
		}
		prev := spans[i-1]
		_, ok := prev.Intersection(sp)
		require.False(t, ok, "spans %d and %d overlap: %v %v", i-1, i, prev, sp)
		require.False(t, prev.IsConnected(sp), "spans %d and %d touch and should have coalesced: %v %v", i-1, i, prev, sp)
		prevHi, _, prevOK := prev.UpperBound()
		nextLo, _, nextOK := sp.LowerBound()
		require.True(t, prevOK, "span %d has no upper end, so span %d cannot follow it", i-1, i)
		require.True(t, nextOK, "span %d has no lower end, so it cannot follow span %d", i, i-1)
		require.LessOrEqual(t, prevHi, nextLo, "spans %d and %d are not ascending", i-1, i)
	}
}

// The reference model below embodies the same value domain as the
// implementation, but as a dumb one-bit-per-value set, so a differential test
// catches any clever coalescing mistake.
//
// The modelled domain is the dense order of the universe 0..200 with one
// position per value, one for the gap that follows it, and a sentinel for
// everything below and above the universe. Positions are ordered exactly like
// the values they stand for, which makes membership and run counting exact, and
// ranges with unbounded ends map onto the two sentinels.
const (
	modelUniverse = 200                 // largest value the model knows
	modelBelow    = 0                   // sentinel: every value below the universe
	modelAbove    = 2*modelUniverse + 2 // sentinel: every value above the universe
	modelSize     = modelAbove + 1
)

// model is a brute-force reference set: one flag per position.
type model struct {
	bits []bool
}

// newModel returns an empty model.
func newModel() *model { return &model{bits: make([]bool, modelSize)} }

// modelPos maps a value onto its position: universe values take odd positions,
// the gap after each value the even one, and out-of-universe values collapse
// onto a sentinel.
func modelPos(v int) int {
	switch {
	case v < 0:
		return modelBelow
	case v > modelUniverse:
		return modelAbove
	default:
		return 1 + 2*v
	}
}

// holds reports whether the position p stands for one of the values of r.
// LowerBound and UpperBound report an empty range as unbounded, so emptiness
// has to be checked first.
func holds(r rangeset.Range[int], p int) bool {
	if r.IsEmpty() {
		return false
	}
	loPos, loClosed := modelBelow, true
	if v, closed, ok := r.LowerBound(); ok {
		loPos, loClosed = modelPos(v), closed
	}
	hiPos, hiClosed := modelAbove, true
	if v, closed, ok := r.UpperBound(); ok {
		hiPos, hiClosed = modelPos(v), closed
	}
	return (p > loPos || (p == loPos && loClosed)) && (p < hiPos || (p == hiPos && hiClosed))
}

// add marks every position covered by r; empty ranges mark nothing.
func (m *model) add(r rangeset.Range[int]) {
	for p := range m.bits {
		if holds(r, p) {
			m.bits[p] = true
		}
	}
}

// remove clears every position covered by r.
func (m *model) remove(r rangeset.Range[int]) {
	for p := range m.bits {
		if holds(r, p) {
			m.bits[p] = false
		}
	}
}

// contains reports whether the model holds v.
func (m *model) contains(v int) bool { return m.bits[modelPos(v)] }

// spanCount returns the number of maximal runs of set positions, which is how
// many spans the real set must keep.
func (m *model) spanCount() int {
	n := 0
	for p := range m.bits {
		if m.bits[p] && (p == 0 || !m.bits[p-1]) {
			n++
		}
	}
	return n
}

// empty reports whether the model holds nothing.
func (m *model) empty() bool { return m.spanCount() == 0 }

// complement returns the model of the complement.
func (m *model) complement() *model {
	out := newModel()
	for p := range out.bits {
		out.bits[p] = !m.bits[p]
	}
	return out
}

// checkSpans asserts that s agrees with m on membership - over the whole
// universe plus a few values outside it - and that the stored spans expand back
// to exactly the modelled positions, with the same span count.
func checkSpans(t *testing.T, s *rangeset.RangeSet[int], m *model, where string) {
	t.Helper()
	// plain comparisons: this runs after every mutation, fuzzing included
	for v := -2; v <= modelUniverse+2; v++ {
		if got, want := s.Contains(v), m.contains(v); got != want {
			t.Fatalf("%s: Contains(%d) = %t, model says %t; spans=%v", where, v, got, want, s.Spans())
		}
	}
	var expanded [modelSize]bool
	for _, sp := range s.Spans() {
		for p := range expanded {
			expanded[p] = expanded[p] || holds(sp, p)
		}
	}
	for p := range expanded {
		if expanded[p] != m.bits[p] {
			t.Fatalf("%s: position %d covered by spans = %t, model says %t; spans=%v", where, p, expanded[p], m.bits[p], s.Spans())
		}
	}
	require.Equal(t, m.spanCount(), s.Len(), "%s: Len; spans=%v", where, s.Spans())
	require.Equal(t, m.empty(), s.IsEmpty(), "%s: IsEmpty", where)
}

// checkModel asserts the full agreement of s with the model, complement
// included.
func checkModel(t *testing.T, s *rangeset.RangeSet[int], m *model, where string) {
	t.Helper()
	checkSpans(t, s, m, where)
	checkSpans(t, s.Complement(), m.complement(), where+" complement")
}

// op is one replayable mutation of the model test.
type op struct {
	add bool
	r   rangeset.Range[int]
}

// maxOps bounds the replay length so fuzzing stays fast.
const maxOps = 128

// decodeOps turns arbitrary bytes into mutations, four bytes per mutation: the
// kind, the constructor and the two endpoints. Endpoints are drawn from the
// modelled universe so the reference model stays exact.
func decodeOps(data []byte) []op {
	ops := make([]op, 0, maxOps)
	for i := 0; i+4 <= len(data) && len(ops) < maxOps; i += 4 {
		lo, hi := int(data[i+2])%(modelUniverse+1), int(data[i+3])%(modelUniverse+1)
		var r rangeset.Range[int]
		switch data[i+1] % 10 {
		case 0:
			r = rangeset.Closed(lo, hi)
		case 1:
			r = rangeset.Open(lo, hi)
		case 2:
			r = rangeset.OpenClosed(lo, hi)
		case 3:
			r = rangeset.ClosedOpen(lo, hi)
		case 4:
			r = rangeset.AtLeast(lo)
		case 5:
			r = rangeset.AtMost(hi)
		case 6:
			r = rangeset.GreaterThan(lo)
		case 7:
			r = rangeset.LessThan(hi)
		case 8:
			r = rangeset.Singleton(lo)
		default:
			r = rangeset.All[int]()
		}
		ops = append(ops, op{add: data[i]%2 == 0, r: r})
	}
	return ops
}

// runModel replays ops against a real set and the reference model, checking the
// invariant and the model after every single mutation.
func runModel(t *testing.T, ops []op) {
	t.Helper()
	s := rangeset.NewSet[int]()
	m := newModel()
	for i, o := range ops {
		kind := "Remove"
		if o.add {
			kind = "Add"
			s.Add(o.r)
			m.add(o.r)
		} else {
			s.Remove(o.r)
			m.remove(o.r)
		}
		where := fmt.Sprintf("op %d (%s %v)", i, kind, o.r)
		requireInvariant(t, s)
		checkModel(t, s, m, where)
	}
}
