package random_test

import (
	"math"
	"testing"

	"github.com/neko233-com/struct233-go/random"
	"github.com/stretchr/testify/require"
)

func TestWeightedRejectsNonFiniteWeights(t *testing.T) {
	for name, bad := range map[string]float64{"NaN": math.NaN(), "+Inf": math.Inf(1), "-Inf": math.Inf(-1)} {
		t.Run(name, func(t *testing.T) {
			items := []float64{1, bad, 0}
			weight := func(w float64) float64 { return w }
			require.PanicsWithValue(t, "random: weight must be finite", func() {
				random.Weighted(nil, items, weight)
			})
			require.PanicsWithValue(t, "random: weight must be finite", func() {
				random.NewWeightedTable(nil, items, weight)
			})
		})
	}
}

func TestWeightedRejectsTotalOverflow(t *testing.T) {
	items := []float64{math.MaxFloat64, math.MaxFloat64}
	weight := func(w float64) float64 { return w }
	require.PanicsWithValue(t, "random: total weight must be finite", func() {
		random.Weighted(nil, items, weight)
	})
	require.PanicsWithValue(t, "random: total weight must be finite", func() {
		random.NewWeightedTable(nil, items, weight)
	})
}

func TestWeightedSubnormalTotalNeverSelectsZeroWeight(t *testing.T) {
	items := []drop{{"leading zero", 0}, {"positive", math.SmallestNonzeroFloat64}, {"trailing zero", 0}}
	g := random.New(&seqRand{floats: []float64{0, 0.5, math.Nextafter(1, 0)}})
	weight := func(d drop) float64 { return d.weight }
	table := random.NewWeightedTable(g, items, weight)
	for range 9 {
		got, ok := table.Pick()
		require.True(t, ok)
		require.Equal(t, "positive", got.name)
		got, ok = random.Weighted(g, items, weight)
		require.True(t, ok)
		require.Equal(t, "positive", got.name)
	}
}
