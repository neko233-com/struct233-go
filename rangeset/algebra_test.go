package rangeset_test

import (
	"math/rand/v2"
	"testing"

	"github.com/neko233-com/struct233-go/rangeset"
)

func TestSetAlgebraAgainstPointModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(233, 1012))
	randomRange := func() rangeset.Range[float64] {
		lo, hi := float64(rng.IntN(17)-8), float64(rng.IntN(17)-8)
		switch rng.IntN(9) {
		case 0:
			return rangeset.Open(lo, hi)
		case 1:
			return rangeset.ClosedOpen(lo, hi)
		case 2:
			return rangeset.OpenClosed(lo, hi)
		case 3:
			return rangeset.AtLeast(lo)
		case 4:
			return rangeset.AtMost(hi)
		case 5:
			return rangeset.GreaterThan(lo)
		case 6:
			return rangeset.LessThan(hi)
		case 7:
			return rangeset.Singleton(lo)
		default:
			return rangeset.Closed(lo, hi)
		}
	}
	for round := range 300 {
		a, b := rangeset.NewSet[float64](), rangeset.NewSet[float64]()
		for range 20 {
			for _, s := range []*rangeset.RangeSet[float64]{a, b} {
				r := randomRange()
				if rng.IntN(2) == 0 {
					s.Add(r)
				} else {
					s.Remove(r)
				}
			}
		}
		union, intersection, complement := a.Union(b), a.Intersection(b), a.Complement()
		for _, s := range []*rangeset.RangeSet[float64]{union, intersection, complement} {
			spans := s.Spans()
			for i, span := range spans {
				if span.IsEmpty() || (i > 0 && spans[i-1].IsConnected(span)) {
					t.Fatalf("round %d: noncanonical result: %v", round, spans)
				}
			}
		}
		intersects := false
		// Every finite boundary is integral in [-8,8]; half-step probes plus
		// the outside points cover all distinct membership regions.
		for twiceX := -20; twiceX <= 20; twiceX++ {
			x := float64(twiceX) / 2
			inA, inB := a.Contains(x), b.Contains(x)
			intersects = intersects || (inA && inB)
			if union.Contains(x) != (inA || inB) || intersection.Contains(x) != (inA && inB) || complement.Contains(x) == inA {
				t.Fatalf("round %d at %v: algebra differs from membership model; a=%v b=%v", round, x, a.Spans(), b.Spans())
			}
		}
		if a.Intersects(b) != intersects || b.Intersects(a) != intersects {
			t.Fatalf("round %d: Intersects differs from membership model", round)
		}
	}
}
