// Package bloomfilter provides a probabilistic set with no false negatives,
// the classic space-efficient membership filter.
//
// Test may report an element as present although it was never added (the
// false-positive rate is bounded by the value passed to New), but Add always
// guarantees that Test returns true afterwards. Not safe for concurrent use.
package bloomfilter

import (
	"math"

	"github.com/neko233-com/struct233-go/bitset"
)

// BloomFilter is a Bloom filter over bitset.BitSet storage.
type BloomFilter struct {
	words *bitset.BitSet
	k     int    // number of hash probes
	m     uint64 // number of bits
	count uint64 // number of Add calls
}

// New creates a filter sized for expectedItems elements with the given
// false-positive rate (0 < rate < 1). It panics on a non-positive item count
// or a rate outside (0, 1).
func New(expectedItems int, falsePositiveRate float64) *BloomFilter {
	if expectedItems <= 0 {
		panic("bloomfilter: expectedItems must be positive")
	}
	if falsePositiveRate <= 0 || falsePositiveRate >= 1 {
		panic("bloomfilter: falsePositiveRate must be within (0, 1)")
	}
	// m = -n*ln(p) / (ln2)^2, k = m/n * ln2
	ln2 := math.Ln2
	m := uint64(math.Ceil(-float64(expectedItems) * math.Log(falsePositiveRate) / (ln2 * ln2)))
	if m < 64 {
		m = 64
	}
	k := int(math.Round(float64(m) / float64(expectedItems) * ln2))
	if k < 1 {
		k = 1
	}
	if k > 64 {
		k = 64
	}
	return &BloomFilter{
		words: bitset.New(),
		k:     k,
		m:     m,
	}
}

// K returns the number of hash probes per element.
func (b *BloomFilter) K() int { return b.k }

// BitLen returns the number of bits in the underlying bit vector.
func (b *BloomFilter) BitLen() uint64 { return b.m }

// Count returns how many elements have been added.
func (b *BloomFilter) Count() uint64 { return b.count }

// FalsePositiveRate returns the design false-positive rate for the current
// element count (it degrades as the filter fills beyond its design size).
func (b *BloomFilter) FalsePositiveRate() float64 {
	if b.count == 0 {
		return 0
	}
	// (1 - e^(-k*n/m))^k
	return math.Pow(1-math.Exp(-float64(b.k)*float64(b.count)/float64(b.m)), float64(b.k))
}

// Clear resets the filter to empty.
func (b *BloomFilter) Clear() {
	b.words = bitset.New()
	b.count = 0
}

// AddString adds s.
func (b *BloomFilter) AddString(s string) {
	b.probe(hashString(s), hashStringSeed(s))
	b.count++
}

// TestString reports whether s may be present.
func (b *BloomFilter) TestString(s string) bool {
	return b.probeTest(hashString(s), hashStringSeed(s))
}

// AddBytes adds p.
func (b *BloomFilter) AddBytes(p []byte) {
	b.probe(hashBytes(p), hashBytesSeed(p))
	b.count++
}

// TestBytes reports whether p may be present.
func (b *BloomFilter) TestBytes(p []byte) bool {
	return b.probeTest(hashBytes(p), hashBytesSeed(p))
}

// AddUint64 adds v.
func (b *BloomFilter) AddUint64(v uint64) {
	b.probe(mix64(v), mix64(v^0x9E3779B97F4A7C15))
	b.count++
}

// TestUint64 reports whether v may be present.
func (b *BloomFilter) TestUint64(v uint64) bool {
	return b.probeTest(mix64(v), mix64(v^0x9E3779B97F4A7C15))
}

// probe sets the k bits derived from the two base hashes
// (Kirsch-Mitzenmacher double hashing).
func (b *BloomFilter) probe(h1, h2 uint64) {
	if h2 == 0 {
		h2 = 0x9E3779B97F4A7C15
	}
	for i := range b.k {
		b.words.Set(int((h1 + uint64(i)*h2) % b.m))
	}
}

// probeTest reports whether all k bits derived from the two base hashes are set.
func (b *BloomFilter) probeTest(h1, h2 uint64) bool {
	if h2 == 0 {
		h2 = 0x9E3779B97F4A7C15
	}
	for i := range b.k {
		if !b.words.Get(int((h1 + uint64(i)*h2) % b.m)) {
			return false
		}
	}
	return true
}

const (
	fnvOffset64 = 14695981039346656037
	fnvPrime64  = 1099511628211
)

func hashString(s string) uint64 {
	h := uint64(fnvOffset64)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime64
	}
	return mix64(h)
}

func hashStringSeed(s string) uint64 {
	h := uint64(fnvOffset64) ^ 0xCBF29CE484222325
	for i := len(s) - 1; i >= 0; i-- {
		h ^= uint64(s[i])
		h *= fnvPrime64
	}
	return mix64(h)
}

func hashBytes(p []byte) uint64 {
	h := uint64(fnvOffset64)
	for _, c := range p {
		h ^= uint64(c)
		h *= fnvPrime64
	}
	return mix64(h)
}

func hashBytesSeed(p []byte) uint64 {
	h := uint64(fnvOffset64) ^ 0x9E3779B97F4A7C15
	for i := len(p) - 1; i >= 0; i-- {
		h ^= uint64(p[i])
		h *= fnvPrime64
	}
	return mix64(h)
}

// mix64 is the splitmix64 finalizer: it spreads the low-entropy FNV output.
func mix64(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xBF58476D1CE4E5B9
	x ^= x >> 27
	x *= 0x94D049BB133111EB
	x ^= x >> 31
	return x
}

// PopCount returns the number of set bits; useful for occupancy reporting.
func (b *BloomFilter) PopCount() int { return b.words.Cardinality() }
