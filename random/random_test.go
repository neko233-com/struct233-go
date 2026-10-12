package random_test

import (
	"math/rand/v2"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/random"
)

// alnum mirrors the character set the package documents for String.
const alnum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// zeroRand scripts a source at its lowest value: IntN always returns 0 and
// Float64 always returns 0.
type zeroRand struct{}

func (zeroRand) IntN(int) int     { return 0 }
func (zeroRand) Float64() float64 { return 0 }

// maxRand scripts a source at its highest value: IntN always returns n-1 and
// Float64 always returns the largest sensible value below 1.
type maxRand struct{}

func (maxRand) IntN(n int) int   { return n - 1 }
func (maxRand) Float64() float64 { return 0.999999999 }

// seqRand replays a scripted Float64 sequence (cycling) and reports 0 for IntN.
type seqRand struct {
	floats []float64
	calls  int
}

func (s *seqRand) IntN(n int) int { return 0 }

func (s *seqRand) Float64() float64 {
	v := s.floats[s.calls%len(s.floats)]
	s.calls++
	return v
}

// onlyUses reports whether every character of s comes from alphabet.
func onlyUses(s, alphabet string) bool {
	for _, r := range s {
		if !strings.ContainsRune(alphabet, r) {
			return false
		}
	}
	return true
}

func TestNewPanicsOnNilSource(t *testing.T) {
	require.PanicsWithValue(t, "random: source must not be nil", func() { random.New(nil) })
}

func TestZeroGeneratorPanicsOnUse(t *testing.T) {
	var g *random.Generator
	require.PanicsWithValue(t, "random: nil generator", func() { g.Int(0, 1) })
	require.PanicsWithValue(t, "random: nil generator", func() { g.String(1) })
	require.PanicsWithValue(t, "random: nil generator", func() { g.Float64() })
}

func TestScriptedSourceMapsToRangeEnds(t *testing.T) {
	lo := random.New(zeroRand{})
	require.Equal(t, 10, lo.Int(10, 20), "IntN == 0 must map to min")
	require.Equal(t, int64(-5), lo.Int64(-5, 5), "IntN == 0 must map to min")
	require.Equal(t, -1, lo.Int(-1, 1), "IntN == 0 must map to min")

	hi := random.New(maxRand{})
	require.Equal(t, 20, hi.Int(10, 20), "IntN == n-1 must map to max")
	require.Equal(t, int64(5), hi.Int64(-5, 5), "IntN == n-1 must map to max")
	require.Equal(t, 7, hi.Int(7, 7), "a single-value range must still return it")
}

func TestScriptedSourceDrivesCollections(t *testing.T) {
	items := []int{10, 20, 30, 40}

	lo := random.New(zeroRand{})
	v, ok := random.Choice(lo, items)
	require.True(t, ok)
	require.Equal(t, 10, v, "Choice must take the first element at draw 0")

	hi := random.New(maxRand{})
	v, ok = random.Choice(hi, items)
	require.True(t, ok)
	require.Equal(t, 40, v, "Choice must take the last element at draw n-1")

	require.Equal(t, []int{10, 20}, random.Sample(lo, items, 2), "Sample must copy the head at draw 0")
	require.Equal(t, []int{10, 20, 30, 40}, items, "Sample must not touch the input")

	// a full shuffle with every swap aimed at index 0 rotates the slice left
	owned := []int{0, 1, 2, 3}
	random.Shuffle(random.New(zeroRand{}), owned)
	require.Equal(t, []int{1, 2, 3, 0}, owned, "Shuffle must consume one draw per position")
}

func TestIntCoversRange(t *testing.T) {
	g := random.New(rand.New(rand.NewPCG(1, 2)))
	minSeen, maxSeen := 0, 0
	for range 10_000 {
		v := g.Int(-3, 7)
		if v < minSeen {
			minSeen = v
		}
		if v > maxSeen {
			maxSeen = v
		}
	}
	require.Equal(t, -3, minSeen, "10k draws must reach the low end")
	require.Equal(t, 7, maxSeen, "10k draws must reach the high end")

	minSeen, maxSeen = 4, 4
	for range 10_000 {
		v := g.Int(4, 4)
		minSeen = min(minSeen, v)
		maxSeen = max(maxSeen, v)
	}
	require.Equal(t, 4, minSeen, "a single-value range must only ever return it")
	require.Equal(t, 4, maxSeen, "a single-value range must only ever return it")
}

func TestInt64CoversRange(t *testing.T) {
	g := random.New(rand.New(rand.NewPCG(3, 4)))

	// a narrow range so that 10k draws can actually reach both ends
	minSeen, maxSeen := int64(0), int64(0)
	for range 10_000 {
		v := g.Int64(-3, 7)
		if v < minSeen {
			minSeen = v
		}
		if v > maxSeen {
			maxSeen = v
		}
	}
	require.Equal(t, int64(-3), minSeen, "10k draws must reach the low end")
	require.Equal(t, int64(7), maxSeen, "10k draws must reach the high end")

	const (
		lo = int64(-1 << 40)
		hi = int64(1 << 40)
	)
	minSeen, maxSeen = 0, 0
	for range 10_000 {
		v := g.Int64(lo, hi)
		minSeen = min(minSeen, v)
		maxSeen = max(maxSeen, v)
	}
	require.GreaterOrEqual(t, minSeen, lo, "no draw may leave the range downwards")
	require.LessOrEqual(t, maxSeen, hi, "no draw may leave the range upwards")
	require.Negative(t, minSeen, "a symmetric range must produce negative draws")
	require.Positive(t, maxSeen, "a symmetric range must produce positive draws")
}

func TestIntPanicsWhenRangeIsInverted(t *testing.T) {
	g := random.Default()
	require.PanicsWithValue(t, "random: max must not be less than min", func() { g.Int(2, 1) })
	require.PanicsWithValue(t, "random: max must not be less than min", func() { g.Int64(2, 1) })
	require.PanicsWithValue(t, "random: max must not be less than min", func() { random.Int(0, -1) })
}

func TestFloat64StaysInUnitInterval(t *testing.T) {
	g := random.New(rand.New(rand.NewPCG(5, 6)))
	require.Zero(t, random.New(zeroRand{}).Float64(), "the source value must be forwarded as-is")
	for range 1000 {
		v := g.Float64()
		require.GreaterOrEqual(t, v, 0.0, "Float64 must not go below 0")
		require.Less(t, v, 1.0, "Float64 must stay below 1")
	}
}

func TestBoolSplitsAtHalf(t *testing.T) {
	require.True(t, random.New(zeroRand{}).Bool(), "0.0 is below the threshold")
	require.False(t, random.New(maxRand{}).Bool(), "a value near 1 is above the threshold")

	g := random.New(rand.New(rand.NewPCG(7, 8)))
	trueCount := 0
	for range 10_000 {
		if g.Bool() {
			trueCount++
		}
	}
	require.Greater(t, trueCount, 4_000, "half of 10k draws must come out true")
	require.Less(t, trueCount, 6_000, "half of 10k draws must come out false")
}

func TestStringUsesOnlyAlphanumericCharacters(t *testing.T) {
	g := random.New(rand.New(rand.NewPCG(9, 10)))
	for range 200 {
		s := g.String(32)
		require.Len(t, s, 32, "String must return exactly n characters")
		require.True(t, onlyUses(s, alnum), "String must only use [0-9A-Za-z], got %q", s)
	}
	require.Equal(t, "0000000000", random.New(zeroRand{}).String(10), "String must draw from position 0")
}

func TestStringFromUsesOnlyTheGivenAlphabet(t *testing.T) {
	const alphabet = "xyz"
	g := random.New(rand.New(rand.NewPCG(11, 12)))
	for range 200 {
		s := g.StringFrom(alphabet, 16)
		require.Len(t, s, 16, "StringFrom must return exactly n characters")
		require.True(t, onlyUses(s, alphabet), "StringFrom must only use the alphabet, got %q", s)
	}
	require.Equal(t, "aaa", g.StringFrom("a", 3), "a single-character alphabet must repeat it")
}

func TestDigitsUsesOnlyDecimals(t *testing.T) {
	g := random.New(rand.New(rand.NewPCG(13, 14)))
	for range 200 {
		s := g.Digits(20)
		require.Len(t, s, 20, "Digits must return exactly n characters")
		require.True(t, onlyUses(s, "0123456789"), "Digits must only use [0-9], got %q", s)
	}
	require.Equal(t, "0000", random.New(zeroRand{}).Digits(4), "Digits must draw from position 0")
}

func TestStringBuildersPanicOnBadArguments(t *testing.T) {
	g := random.Default()
	require.PanicsWithValue(t, "random: n must not be negative", func() { g.String(-1) })
	require.PanicsWithValue(t, "random: n must not be negative", func() { g.Digits(-1) })
	require.PanicsWithValue(t, "random: n must not be negative", func() { g.StringFrom("ab", -1) })
	require.PanicsWithValue(t, "random: alphabet must not be empty", func() { g.StringFrom("", 1) })
	require.PanicsWithValue(t, "random: n must not be negative", func() { random.String(-1) })
}

func TestStringBuildersReturnEmptyForZeroLength(t *testing.T) {
	g := random.Default()
	require.Empty(t, g.String(0))
	require.Empty(t, g.StringFrom("ab", 0))
	require.Empty(t, g.Digits(0))
}

func TestShuffleIsAPermutation(t *testing.T) {
	g := random.New(rand.New(rand.NewPCG(15, 16)))

	items := make([]int, 50)
	for i := range items {
		items[i] = i
	}
	random.Shuffle(g, items)

	require.Len(t, items, 50, "Shuffle must not change the length")
	require.ElementsMatch(t, intRange(50), items, "Shuffle must keep every element exactly once")

	moved := false
	for i, v := range items {
		if v != i {
			moved = true
			break
		}
	}
	require.True(t, moved, "shuffling 50 elements must reorder at least one of them")
}

func TestShuffleHandlesShortInputs(t *testing.T) {
	g := random.Default()
	random.Shuffle[int](g, nil)

	single := []int{1}
	random.Shuffle(g, single)
	require.Equal(t, []int{1}, single, "single-element input must survive")
}

func TestShuffleWithNilGeneratorUsesDefault(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8}
	random.Shuffle(nil, items)
	require.ElementsMatch(t, []int{1, 2, 3, 4, 5, 6, 7, 8}, items, "a nil generator must fall back to Default")
}

func TestChoiceOnEmptySliceMisses(t *testing.T) {
	v, ok := random.Choice[int](random.Default(), nil)
	require.False(t, ok, "Choice on a nil slice must miss")
	require.Zero(t, v)

	v, ok = random.Choice(random.Default(), []int{})
	require.False(t, ok, "Choice on an empty slice must miss")
	require.Zero(t, v)
}

func TestSampleIsASubsetWithoutRepeats(t *testing.T) {
	g := random.New(rand.New(rand.NewPCG(17, 18)))
	source := intRange(10)

	for range 500 {
		got := random.Sample(g, source, 3)
		require.Len(t, got, 3, "Sample must return exactly n elements")
		seen := make(map[int]bool, len(got))
		for _, v := range got {
			require.Contains(t, source, v, "Sample must only return source elements")
			require.False(t, seen[v], "Sample must not repeat an element")
			seen[v] = true
		}
	}
}

func TestSampleClampsToLength(t *testing.T) {
	g := random.New(rand.New(rand.NewPCG(19, 20)))
	source := intRange(4)

	got := random.Sample(g, source, 100)
	require.Len(t, got, 4, "Sample must clamp n to len(items)")
	require.ElementsMatch(t, source, got, "Sample of everything returns every element")
}

func TestSampleOfZeroReturnsEmptySlice(t *testing.T) {
	got := random.Sample(random.Default(), intRange(5), 0)
	require.NotNil(t, got, "Sample must return an empty slice, not nil")
	require.Empty(t, got)
}

func TestSamplePanicsOnNegativeN(t *testing.T) {
	require.PanicsWithValue(t, "random: n must not be negative", func() {
		random.Sample(random.Default(), intRange(3), -1)
	})
}

func TestSampleCoversEveryElement(t *testing.T) {
	g := random.New(rand.New(rand.NewPCG(21, 22)))
	source := intRange(5)

	seen := make(map[int]bool, len(source))
	for range 2000 {
		seen[random.Sample(g, source, 1)[0]] = true
	}
	require.Len(t, seen, 5, "sampling one of five 2k times must have reached every element")
}

type drop struct {
	name   string
	weight float64
}

func drops() []drop {
	return []drop{{"common", 1}, {"uncommon", 3}, {"rare", 0}, {"negative", -5}}
}

func TestWeightedPicksTheFirstPositiveWeightAtZero(t *testing.T) {
	items := drops()
	got, ok := random.Weighted(random.New(zeroRand{}), items, func(d drop) float64 { return d.weight })
	require.True(t, ok)
	require.Equal(t, "common", got.name, "a draw of 0 must land on the first positive weight")
}

func TestWeightedIgnoresZeroAndNegativeWeights(t *testing.T) {
	items := []drop{{"zero", 0}, {"negative", -3}, {"only", 2}}
	// 0.9 * 2 = 1.8 inside the only positive bucket
	g := random.New(&seqRand{floats: []float64{0.9}})
	got, ok := random.Weighted(g, items, func(d drop) float64 { return d.weight })
	require.True(t, ok)
	require.Equal(t, "only", got.name, "zero and negative weights must never be picked")
}

func TestWeightedReportsFalseWhenNothingIsPositive(t *testing.T) {
	items := []drop{{"a", 0}, {"b", -1}}
	got, ok := random.Weighted(random.Default(), items, func(d drop) float64 { return d.weight })
	require.False(t, ok, "all-zero weights must report failure")
	require.Zero(t, got)

	got, ok = random.Weighted[drop](random.Default(), nil, func(d drop) float64 { return d.weight })
	require.False(t, ok, "an empty item list must report failure")
	require.Zero(t, got)
}

func TestWeightedPanicsOnNilWeight(t *testing.T) {
	require.PanicsWithValue(t, "random: weight function must not be nil", func() {
		random.Weighted[drop](random.Default(), drops(), nil)
	})
}

func TestWeightedSpreadAcrossBuckets(t *testing.T) {
	g := random.New(rand.New(rand.NewPCG(23, 24)))
	items := []drop{{"light", 1}, {"heavy", 3}}

	counts := map[string]int{}
	for range 10_000 {
		got, ok := random.Weighted(g, items, func(d drop) float64 { return d.weight })
		require.True(t, ok)
		counts[got.name]++
	}
	require.Greater(t, counts["heavy"], 6_500, "a 3:1 weight must dominate over 10k picks")
	require.Less(t, counts["heavy"], 8_500, "a 3:1 weight must still leave room for the light bucket")
}

func TestWeightedTablePicksAtCumulativeBoundaries(t *testing.T) {
	items := []drop{{"a", 1}, {"b", 3}, {"c", 4}} // cumulative 1, 4, 8
	g := random.New(&seqRand{floats: []float64{0, 0.125, 0.5, 0.99}})
	table := random.NewWeightedTable(g, items, func(d drop) float64 { return d.weight })

	for _, want := range []string{"a", "b", "c", "c"} {
		got, ok := table.Pick()
		require.True(t, ok)
		require.Equal(t, want, got.name, "boundary draw %v must land in its bucket", want)
	}
}

func TestWeightedTableSkipsZeroWeightItems(t *testing.T) {
	items := []drop{{"zero", 0}, {"picked", 2}, {"later", 3}}
	table := random.NewWeightedTable(random.New(zeroRand{}), items, func(d drop) float64 { return d.weight })

	for range 100 {
		got, ok := table.Pick()
		require.True(t, ok)
		require.Equal(t, "picked", got.name, "the leading zero weight must never be drawn")
	}
}

func TestWeightedTableItems(t *testing.T) {
	table := random.NewWeightedTable(random.Default(), drops(), func(d drop) float64 { return d.weight })
	require.Equal(t, 4, table.Items(), "Items must report the number of stored elements")

	var nilTable *random.WeightedTable[drop]
	require.Zero(t, nilTable.Items(), "a nil table holds nothing")
	_, ok := nilTable.Pick()
	require.False(t, ok, "a nil table must report failure")
}

func TestWeightedTableWithSingleItemAlwaysPicksIt(t *testing.T) {
	items := []drop{{"only", 1}}
	table := random.NewWeightedTable(random.New(rand.New(rand.NewPCG(25, 26))), items, func(d drop) float64 { return d.weight })

	for range 1000 {
		got, ok := table.Pick()
		require.True(t, ok)
		require.Equal(t, "only", got.name, "a one-item table must always return that item")
	}
}

func TestWeightedTableDistribution(t *testing.T) {
	g := random.New(rand.New(rand.NewPCG(27, 28)))
	items := []drop{{"light", 1}, {"heavy", 3}}
	table := random.NewWeightedTable(g, items, func(d drop) float64 { return d.weight })

	heavy := 0
	const picks = 100_000
	for range picks {
		got, ok := table.Pick()
		require.True(t, ok)
		if got.name == "heavy" {
			heavy++
		}
	}
	share := float64(heavy) / picks
	require.Greater(t, share, 0.65, "a 3:1 weight must land near 0.75, got %v", share)
	require.Less(t, share, 0.85, "a 3:1 weight must land near 0.75, got %v", share)
}

func TestWeightedTablePickDoesNotAllocate(t *testing.T) {
	items := []drop{{"a", 1}, {"b", 3}, {"c", 4}}
	table := random.NewWeightedTable(random.Default(), items, func(d drop) float64 { return d.weight })
	table.Pick() // reach steady state

	allocs := testing.AllocsPerRun(100, func() { table.Pick() })
	require.Zero(t, allocs, "Pick must binary-search the prefix without allocating")
}

func TestNewWeightedTablePanicsOnBadArguments(t *testing.T) {
	items := drops()
	require.PanicsWithValue(t, "random: items must not be empty", func() {
		random.NewWeightedTable(random.Default(), []drop{}, func(d drop) float64 { return d.weight })
	})
	require.PanicsWithValue(t, "random: items must not be empty", func() {
		random.NewWeightedTable[drop](random.Default(), nil, func(d drop) float64 { return d.weight })
	})
	require.PanicsWithValue(t, "random: weight function must not be nil", func() {
		random.NewWeightedTable[drop](random.Default(), items, nil)
	})
	require.PanicsWithValue(t, "random: total weight must be positive", func() {
		zero := []drop{{"a", 0}, {"b", 0}}
		random.NewWeightedTable(random.Default(), zero, func(d drop) float64 { return d.weight })
	})
	require.PanicsWithValue(t, "random: total weight must be positive", func() {
		negative := []drop{{"a", -1}, {"b", -2}}
		random.NewWeightedTable(random.Default(), negative, func(d drop) float64 { return d.weight })
	})
}

func TestNewWeightedTableWithNilGeneratorUsesDefault(t *testing.T) {
	table := random.NewWeightedTable[drop](nil, drops(), func(d drop) float64 { return d.weight })
	require.Equal(t, 4, table.Items(), "a nil generator must fall back to Default")

	_, ok := table.Pick()
	require.True(t, ok, "the table must still work with the default source")
}

func TestConcurrentUseIsSafe(t *testing.T) {
	// CI runs this package under -race on ubuntu; this Windows box has no C
	// toolchain, so -race is unavailable locally and only the invariants below
	// are checked here.
	g := random.New(rand.New(rand.NewPCG(29, 30)))

	var bad atomic.Int64
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 500 {
				if v := g.Int(-50, 50); v < -50 || v > 50 {
					bad.Add(1)
				}
				if v := g.Int64(-50, 50); v < -50 || v > 50 {
					bad.Add(1)
				}
				if s := g.String(8); len(s) != 8 || !onlyUses(s, alnum) {
					bad.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	require.Zero(t, bad.Load(), "concurrent draws must stay within range and be well formed")
}

// intRange returns the elements 0..n-1.
func intRange(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}
