package bloomfilter_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/bloomfilter"
)

func TestNoFalseNegatives(t *testing.T) {
	b := bloomfilter.New(10_000, 0.01)

	const n = 10_000
	for i := range n {
		b.AddUint64(uint64(i))
	}
	require.Equal(t, uint64(n), b.Count())

	for i := range n {
		require.True(t, b.TestUint64(uint64(i)), "added element %d must never be reported absent", i)
	}
}

func TestStringAndBytesShareSemantics(t *testing.T) {
	b := bloomfilter.New(1_000, 0.01)

	keys := make([]string, 500)
	for i := range keys {
		keys[i] = fmt.Sprintf("player-%d", i)
		b.AddString(keys[i])
	}
	for _, k := range keys {
		require.True(t, b.TestString(k), "added string %q must be present", k)
	}

	raw := []byte("network-session-token")
	b.AddBytes(raw)
	require.True(t, b.TestBytes(raw))
	require.False(t, b.TestBytes([]byte("another-token")), "unrelated bytes should not collide here")
}

func TestFalsePositiveRateWithinDesign(t *testing.T) {
	const (
		n    = 20_000
		rate = 0.01
	)
	b := bloomfilter.New(n, rate)
	for i := range n {
		b.AddString(fmt.Sprintf("uid:%d", i))
	}

	falsePositives := 0
	const probes = 20_000
	for i := range probes {
		if b.TestString(fmt.Sprintf("absent:%d", i)) {
			falsePositives++
		}
	}
	observed := float64(falsePositives) / float64(probes)

	// allow a generous margin: FNV + double hashing is not a perfect hash
	require.Less(t, observed, rate*5, "observed false-positive rate %.4f is far above the design rate %.4f", observed, rate)
	require.Less(t, b.FalsePositiveRate(), rate*2, "estimated rate should stay near the design rate")
}

func TestParameters(t *testing.T) {
	b := bloomfilter.New(1_000, 0.01)
	require.Greater(t, b.K(), 0, "must probe at least once")
	require.LessOrEqual(t, b.K(), 64)
	require.GreaterOrEqual(t, b.BitLen(), uint64(64), "at least one word of storage")
	require.Zero(t, b.Count())
	require.Zero(t, b.FalsePositiveRate(), "empty filter has no positives")

	// the standard formula yields 10 bits per element at 1% and k = 7
	require.InDelta(t, 9586, int(b.BitLen()), 200)
	require.Equal(t, 7, b.K())
}

func TestPopCountGrows(t *testing.T) {
	b := bloomfilter.New(100, 0.01)
	before := b.PopCount()
	for i := range 50 {
		b.AddUint64(uint64(i))
	}
	require.Greater(t, b.PopCount(), before, "adding elements must set bits")
	require.LessOrEqual(t, b.PopCount(), int(b.BitLen()))
}

func TestClear(t *testing.T) {
	b := bloomfilter.New(100, 0.01)
	b.AddString("a")
	require.True(t, b.TestString("a"))

	b.Clear()
	require.Zero(t, b.Count())
	require.Zero(t, b.PopCount(), "Clear must release all bits")
	require.False(t, b.TestString("a"), "cleared filter must forget previous elements")
}

func TestZeroHashSeedIsHandled(t *testing.T) {
	b := bloomfilter.New(64, 0.01)
	// empty string produces a deterministic pair of hashes; the seed must not
	// collapse to zero (which would probe one bit repeatedly)
	b.AddString("")
	require.True(t, b.TestString(""))
	require.False(t, b.TestString("x"))
}

func TestPanicsOnBadArguments(t *testing.T) {
	require.Panics(t, func() { bloomfilter.New(0, 0.01) })
	require.Panics(t, func() { bloomfilter.New(-1, 0.01) })
	require.Panics(t, func() { bloomfilter.New(100, 0) })
	require.Panics(t, func() { bloomfilter.New(100, 1) })
	require.Panics(t, func() { bloomfilter.New(100, -0.5) })
}

func BenchmarkBloomFilterAddString(b *testing.B) {
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = fmt.Sprintf("player-%d", i)
	}
	f := bloomfilter.New(1<<20, 0.01)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		f.AddString(keys[i&1023])
	}
}

func BenchmarkBloomFilterTestUint64(b *testing.B) {
	f := bloomfilter.New(4096, 0.01)
	for i := range 2048 {
		f.AddUint64(uint64(i))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		f.TestUint64(uint64(i & 4095))
	}
}
