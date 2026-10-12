package hash_test

import (
	"fmt"
	"hash/adler32"
	"hash/crc32"
	"hash/fnv"
	"math"
	"math/bits"
	"math/rand"
	"testing"

	"github.com/neko233-com/struct233-go/hash"
	"github.com/stretchr/testify/require"
)

// boundaryLengths covers the empty input, the shortest tails, the exact
// block/tail boundaries of both MurmurHash3 variants and one long input.
var boundaryLengths = []int{0, 1, 3, 4, 7, 8, 15, 16, 17, 31, 32, 1000}

// newRand returns a deterministic source so every run and every -count uses the
// same inputs.
func newRand(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}

// randBytes returns n pseudo-random bytes from r.
func randBytes(r *rand.Rand, n int) []byte {
	b := make([]byte, n)
	r.Read(b)
	return b
}

// fixedInputs are hand-written inputs plus all 256 byte values.
func fixedInputs() map[string][]byte {
	all := make([]byte, 256)
	for i := range all {
		all[i] = byte(i)
	}
	return map[string][]byte{
		"empty":      {},
		"nil":        nil,
		"one":        []byte("a"),
		"hello":      []byte("hello"),
		"sentence":   []byte("The quick brown fox jumps over the lazy dog"),
		"zeros16":    make([]byte, 16),
		"all-bytes":  all,
		"zeros1000":  make([]byte, 1000),
		"utf8":       []byte("数据流233-hash-循环一致性"),
		"repeat-255": []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	}
}

// TestStdlibCrossCheck compares the stdlib-backed helpers with hash/fnv,
// hash/crc32 and hash/adler32 over every boundary length and fixed input.
func TestStdlibCrossCheck(t *testing.T) {
	r := newRand(20261012)
	for _, n := range boundaryLengths {
		t.Run(fmt.Sprintf("len=%d", n), func(t *testing.T) {
			checkStdlib(t, randBytes(r, n))
		})
	}
	for name, in := range fixedInputs() {
		t.Run("fixed/"+name, func(t *testing.T) {
			checkStdlib(t, in)
		})
	}
}

// checkStdlib asserts the five stdlib-backed helpers against the standard
// library for one input.
func checkStdlib(t *testing.T, in []byte) {
	t.Helper()
	w64 := fnv.New64a()
	w64.Write(in)
	require.Equal(t, w64.Sum64(), hash.FNV1a64(in), "FNV1a64")

	w32 := fnv.New32a()
	w32.Write(in)
	require.Equal(t, w32.Sum32(), hash.FNV1a32(in), "FNV1a32")

	require.Equal(t, crc32.ChecksumIEEE(in), hash.CRC32(in), "CRC32")
	require.Equal(t, crc32.Checksum(in, crc32.MakeTable(crc32.Castagnoli)), hash.CRC32C(in), "CRC32C")
	require.Equal(t, adler32.Checksum(in), hash.Adler32(in), "Adler32")
}

// TestStdlibRandomInputs cross-checks the same helpers over many random inputs
// with random lengths.
func TestStdlibRandomInputs(t *testing.T) {
	r := newRand(7)
	for i := range 64 {
		t.Run(fmt.Sprintf("iter%03d", i), func(t *testing.T) {
			checkStdlib(t, randBytes(r, r.Intn(300)))
		})
	}
}

// TestEmptyInputLiterals pins the definitional values for the empty input
// (FNV offset basis, CRC-32 of nothing, Adler-32 of nothing).
func TestEmptyInputLiterals(t *testing.T) {
	cases := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"fnv1a64-offset-basis", hash.FNV1a64(nil), 14695981039346656037},
		{"fnv1a32-offset-basis", uint64(hash.FNV1a32(nil)), 2166136261},
		{"crc32-empty", uint64(hash.CRC32(nil)), 0},
		{"crc32c-empty", uint64(hash.CRC32C(nil)), 0},
		{"adler32-empty", uint64(hash.Adler32(nil)), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.got)
		})
	}
}

// TestHasherType asserts that Sum64 satisfies the Hasher type and that a
// converted value is usable as a ring hash.
func TestHasherType(t *testing.T) {
	var ring hash.Hasher = hash.Sum64
	require.NotNil(t, ring)
	for _, key := range [][]byte{nil, []byte("a"), []byte("user:42"), []byte("数据")} {
		require.Equal(t, hash.Sum64(key), ring(key))
		require.Equal(t, ring(key), ring(key))
	}
	converted := hash.Hasher(hash.Sum64)
	require.Equal(t, hash.Sum64([]byte("x")), converted([]byte("x")))
}

// TestHasherRingPlacement places many keys on 8 ring nodes through a Hasher and
// asserts that every node is used and no node dominates.
func TestHasherRingPlacement(t *testing.T) {
	const nodes = 8
	ring := hash.Hasher(hash.Sum64)
	counts := make([]int, nodes)
	const keys = 8000
	for i := range keys {
		counts[ring(fmt.Appendf(nil, "key:%d", i))%nodes]++
	}
	want := keys / nodes
	for i, c := range counts {
		require.NotZero(t, c, "node %d unused", i)
		require.Less(t, math.Abs(float64(c-want)), float64(want)*0.4, "node %d skewed: %d", i, c)
	}
}

// TestDeterminism asserts every exported hash returns the same value twice for
// the same input.
func TestDeterminism(t *testing.T) {
	r := newRand(99)
	for _, n := range boundaryLengths {
		t.Run(fmt.Sprintf("len=%d", n), func(t *testing.T) {
			in := randBytes(r, n)
			require.Equal(t, hash.FNV1a64(in), hash.FNV1a64(in))
			require.Equal(t, hash.FNV1a32(in), hash.FNV1a32(in))
			require.Equal(t, hash.CRC32(in), hash.CRC32(in))
			require.Equal(t, hash.CRC32C(in), hash.CRC32C(in))
			require.Equal(t, hash.Adler32(in), hash.Adler32(in))
			require.Equal(t, hash.Sum64(in), hash.Sum64(in))
			require.Equal(t, hash.Sum32(in), hash.Sum32(in))
			a1, a2 := hash.Murmur3_128(in, 12345)
			b1, b2 := hash.Murmur3_128(in, 12345)
			require.Equal(t, a1, b1)
			require.Equal(t, a2, b2)
			require.Equal(t, hash.Murmur3_32(in, 12345), hash.Murmur3_32(in, 12345))
		})
	}
}

// TestSum64Sum32Wrappers asserts the default helpers are the seed-0 seedless
// MurmurHash3 variants.
func TestSum64Sum32Wrappers(t *testing.T) {
	r := newRand(31337)
	for i := range 24 {
		t.Run(fmt.Sprintf("iter%03d", i), func(t *testing.T) {
			in := randBytes(r, r.Intn(128))
			h1, _ := hash.Murmur3_128(in, 0)
			require.Equal(t, h1, hash.Sum64(in))
			require.Equal(t, hash.Murmur3_32(in, 0), hash.Sum32(in))
		})
	}
}

// TestMurmur3EmptyVector asserts the canonical empty-input, seed-0 vector: 0
// for x86_32, and (0, 0) for x64_128 (both follow from the algorithm, which
// leaves the accumulators untouched and then finalises zero).
func TestMurmur3EmptyVector(t *testing.T) {
	t.Run("x86_32", func(t *testing.T) {
		require.EqualValues(t, 0, hash.Murmur3_32(nil, 0))
		require.EqualValues(t, 0, hash.Murmur3_32([]byte{}, 0))
	})
	t.Run("x64_128", func(t *testing.T) {
		h1, h2 := hash.Murmur3_128(nil, 0)
		require.Zero(t, h1)
		require.Zero(t, h2)
		require.EqualValues(t, 0, hash.Sum64(nil))
	})
}

// TestMurmur3SeedsDiffer asserts that different seeds produce different results
// for the same input.
func TestMurmur3SeedsDiffer(t *testing.T) {
	seeds := []uint32{0, 1, 2, 0xffffffff, 0x9e3779b9, 0xdeadbeef}
	inputs := map[string][]byte{
		"empty":   {},
		"short":   []byte("mu"),
		"block4":  []byte("murm"),
		"block16": []byte("murmurhash3-16by"),
		"long":    []byte("a longer key that spans several blocks of data 0123456789"),
	}
	for inName, in := range inputs {
		t.Run(inName, func(t *testing.T) {
			seen32 := make(map[uint32]uint32, len(seeds))
			seen128 := make(map[string]struct{}, len(seeds))
			for _, s := range seeds {
				h32 := hash.Murmur3_32(in, s)
				if prev, ok := seen32[h32]; ok {
					require.Equal(t, prev, s, "seed collision")
				}
				seen32[h32] = s

				h1, h2 := hash.Murmur3_128(in, s)
				key := fmt.Sprintf("%d/%d", h1, h2)
				require.NotContains(t, seen128, key, "x64 seed collision")
				seen128[key] = struct{}{}
			}
			require.Len(t, seen32, len(seeds))
			require.Len(t, seen128, len(seeds))
		})
	}
}

// TestMurmur3TailLengths hashes every tail remainder (0..16 bytes beyond whole
// blocks) and asserts distinct inputs stay distinct and never panic.
func TestMurmur3TailLengths(t *testing.T) {
	r := newRand(4242)
	for _, base := range []int{0, 4, 8, 16, 32} {
		for extra := range 17 {
			n := base + extra
			t.Run(fmt.Sprintf("base%d+extra%d", base, extra), func(t *testing.T) {
				in := randBytes(r, n)
				h32 := hash.Murmur3_32(in, 1)
				h1, h2 := hash.Murmur3_128(in, 1)
				require.Equal(t, h32, hash.Murmur3_32(in, 1))
				a1, a2 := hash.Murmur3_128(in, 1)
				require.Equal(t, h1, a1)
				require.Equal(t, h2, a2)
				// An extra byte beyond the block must change the result.
				if n > 0 {
					require.NotEqual(t, hash.Murmur3_32(in[:n-1], 1), h32)
				}
			})
		}
	}
}

// TestMurmur3DistinctInputs asserts that nearby distinct inputs of the same
// length produce distinct hashes.
func TestMurmur3DistinctInputs(t *testing.T) {
	for _, n := range []int{1, 3, 4, 7, 8, 15, 16, 17, 32} {
		t.Run(fmt.Sprintf("len=%d", n), func(t *testing.T) {
			// a one-byte input space holds only 256 distinct values, so sampling
			// more than that would force a collision by pigeonhole and the
			// distinctness assertion below could never hold
			count := 512
			if n == 1 {
				count = 256
			}
			seen := make(map[uint64]int, count)
			for i := range count {
				in := make([]byte, n)
				for j := range in {
					in[j] = byte(i>>(8*(j%4))) ^ byte(j+1)
				}
				h32 := hash.Murmur3_32(in, 7)
				h64, _ := hash.Murmur3_128(in, 7)
				key := uint64(h32)<<32 | uint64(h64&0xffffffff)
				require.NotContains(t, seen, key, "collision for i=%d", i)
				seen[key] = i
			}
			require.Len(t, seen, count)
		})
	}
}

// TestMurmur3LengthSensitivity asserts that appending a zero byte changes the
// hash, i.e. length is mixed in.
func TestMurmur3LengthSensitivity(t *testing.T) {
	cases := map[string][]byte{
		"empty":     {},
		"zero1":     {0},
		"zero2":     {0, 0},
		"zero3":     {0, 0, 0},
		"zero4":     {0, 0, 0, 0},
		"zero7":     make([]byte, 7),
		"zero15":    make([]byte, 15),
		"zero16":    make([]byte, 16),
		"zero17":    make([]byte, 17),
		"ascii-a":   []byte("a"),
		"ascii-aa":  []byte("aa"),
		"ascii-aaa": []byte("aaa"),
	}
	seen32 := make(map[uint32]string, len(cases))
	seen128 := make(map[uint64]string, len(cases))
	for name, in := range cases {
		h32 := hash.Murmur3_32(in, 0)
		require.NotContains(t, seen32, h32, "32-bit collision: %s vs %s", seen32[h32], name)
		seen32[h32] = name
		h1, _ := hash.Murmur3_128(in, 0)
		require.NotContains(t, seen128, h1, "128-bit collision: %s vs %s", seen128[h1], name)
		seen128[h1] = name
	}
}

// TestMurmur3Avalanche asserts that flipping one input bit flips about half of
// the output bits on average.
func TestMurmur3Avalanche(t *testing.T) {
	r := newRand(2024)
	const samples = 2000
	t.Run("x86_32", func(t *testing.T) {
		total := 0
		for range samples {
			in := randBytes(r, 1+r.Intn(48))
			idx := r.Intn(len(in))
			bit := uint(r.Intn(8))
			flipped := append([]byte(nil), in...)
			flipped[idx] ^= 1 << bit
			total += bits.OnesCount32(hash.Murmur3_32(in, 5) ^ hash.Murmur3_32(flipped, 5))
		}
		avg := float64(total) / samples
		require.Greater(t, avg, 12.0, "weak 32-bit avalanche: %.2f", avg)
		require.Less(t, avg, 20.0, "strong 32-bit avalanche: %.2f", avg)
	})
	t.Run("x64_128", func(t *testing.T) {
		total := 0
		for range samples {
			in := randBytes(r, 1+r.Intn(64))
			idx := r.Intn(len(in))
			bit := uint(r.Intn(8))
			flipped := append([]byte(nil), in...)
			flipped[idx] ^= 1 << bit
			a1, a2 := hash.Murmur3_128(in, 5)
			b1, b2 := hash.Murmur3_128(flipped, 5)
			total += bits.OnesCount64(a1^b1) + bits.OnesCount64(a2^b2)
		}
		avg := float64(total) / samples
		require.Greater(t, avg, 48.0, "weak 128-bit avalanche: %.2f", avg)
		require.Less(t, avg, 80.0, "strong 128-bit avalanche: %.2f", avg)
	})
}

// TestLowBitDistribution asserts Sum64 % n is roughly uniform over many keys.
func TestLowBitDistribution(t *testing.T) {
	for _, n := range []uint64{2, 8, 16, 64} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			const keys = 100000
			counts := make([]int, n)
			for i := range keys {
				counts[hash.Sum64(fmt.Appendf(nil, "key-%d", i))%n]++
			}
			want := float64(keys) / float64(n)
			for bucket, c := range counts {
				require.Greater(t, float64(c), want*0.9, "bucket %d too small: %d", bucket, c)
				require.Less(t, float64(c), want*1.1, "bucket %d too large: %d", bucket, c)
			}
		})
	}
}

// TestSum32LowBitDistribution does the same for the 32-bit default.
func TestSum32LowBitDistribution(t *testing.T) {
	const (
		keys   = 50000
		bucket = uint32(16)
	)
	counts := make([]int, bucket)
	for i := range keys {
		counts[hash.Sum32(fmt.Appendf(nil, "k%d", i))%bucket]++
	}
	want := float64(keys) / float64(bucket)
	for i, c := range counts {
		require.Greater(t, float64(c), want*0.85, "bucket %d too small: %d", i, c)
		require.Less(t, float64(c), want*1.15, "bucket %d too large: %d", i, c)
	}
}

// TestNoAllocs asserts hashing allocates nothing.
func TestNoAllocs(t *testing.T) {
	in := make([]byte, 256)
	newRand(11).Read(in)
	sink := uint64(0)
	t.Run("sum64", func(t *testing.T) {
		require.Zero(t, testing.AllocsPerRun(200, func() { sink = hash.Sum64(in) }))
	})
	t.Run("murmur3_128", func(t *testing.T) {
		require.Zero(t, testing.AllocsPerRun(200, func() {
			a, b := hash.Murmur3_128(in, 3)
			sink = a ^ b
		}))
	})
	t.Run("murmur3_32", func(t *testing.T) {
		require.Zero(t, testing.AllocsPerRun(200, func() { sink = uint64(hash.Murmur3_32(in, 3)) }))
	})
	t.Run("fnv1a64", func(t *testing.T) {
		require.Zero(t, testing.AllocsPerRun(200, func() { sink = hash.FNV1a64(in) }))
	})
	t.Run("fnv1a32", func(t *testing.T) {
		require.Zero(t, testing.AllocsPerRun(200, func() { sink = uint64(hash.FNV1a32(in)) }))
	})
	t.Run("crc32", func(t *testing.T) {
		require.Zero(t, testing.AllocsPerRun(200, func() { sink = uint64(hash.CRC32(in)) }))
	})
	t.Run("crc32c", func(t *testing.T) {
		require.Zero(t, testing.AllocsPerRun(200, func() { sink = uint64(hash.CRC32C(in)) }))
	})
	t.Run("adler32", func(t *testing.T) {
		require.Zero(t, testing.AllocsPerRun(200, func() { sink = uint64(hash.Adler32(in)) }))
	})
	require.NotZero(t, sink)
}

// TestSharedPrefixHashes asserts that keys sharing a long prefix still differ,
// which matters for ring balance.
func TestSharedPrefixHashes(t *testing.T) {
	prefix := "tenant-00000000000000000000000000000000/"
	seen := make(map[uint64]int, 1000)
	for i := range 1000 {
		key := prefix + fmt.Sprint(i)
		h := hash.Sum64([]byte(key))
		require.NotContains(t, seen, h, "collision at i=%d", i)
		seen[h] = i
	}
	require.Len(t, seen, 1000)
}

// TestByteSliceAliasing asserts hashing does not modify the input.
func TestByteSliceAliasing(t *testing.T) {
	in := []byte("immutable input buffer for hashing")
	before := append([]byte(nil), in...)
	hash.Sum64(in)
	hash.Sum32(in)
	hash.Murmur3_128(in, 9)
	hash.Murmur3_32(in, 9)
	hash.FNV1a64(in)
	hash.FNV1a32(in)
	hash.CRC32(in)
	hash.CRC32C(in)
	hash.Adler32(in)
	require.Equal(t, before, in)
}

// FuzzSum64 checks arbitrary bytes: the default helpers are their seedless
// MurmurHash3 counterparts, hashing is deterministic, and the stdlib-backed
// helpers agree with hash/fnv, hash/crc32 and hash/adler32.
func FuzzSum64(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("hello"))
	f.Add([]byte("murmurhash3 x64 128 seeds and tails"))
	f.Add(make([]byte, 17))
	f.Fuzz(func(t *testing.T, b []byte) {
		h1, h2 := hash.Murmur3_128(b, 0)
		require.Equal(t, h1, hash.Sum64(b))
		require.Equal(t, hash.Murmur3_32(b, 0), hash.Sum32(b))

		a1, a2 := hash.Murmur3_128(b, 0)
		require.Equal(t, h1, a1)
		require.Equal(t, h2, a2)
		require.Equal(t, hash.Murmur3_32(b, 0), hash.Murmur3_32(b, 0))

		w64 := fnv.New64a()
		w64.Write(b)
		require.Equal(t, w64.Sum64(), hash.FNV1a64(b))

		w32 := fnv.New32a()
		w32.Write(b)
		require.Equal(t, w32.Sum32(), hash.FNV1a32(b))

		require.Equal(t, crc32.ChecksumIEEE(b), hash.CRC32(b))
		require.Equal(t, crc32.Checksum(b, crc32.MakeTable(crc32.Castagnoli)), hash.CRC32C(b))
		require.Equal(t, adler32.Checksum(b), hash.Adler32(b))
	})
}
