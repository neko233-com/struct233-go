// Package hash provides small, dependency-free hashes for hash tables, bloom
// filters and consistent hash rings.
//
// The MurmurHash3 implementations follow Austin Appleby's canonical algorithm
// (x86_32 and x64_128) with little-endian reads. The FNV-1a, CRC-32 and
// Adler-32 helpers return exactly the values produced by the standard library
// packages hash/fnv, hash/crc32 and hash/adler32. Every function is
// deterministic, allocation free and safe for concurrent use.
//
// Sum64 and Sum32 are the defaults used by the rest of struct233;
// Hasher adapts any 64-bit hash so a consistent hash ring can place keys on
// nodes:
//
//	ring := hash.Hasher(hash.Sum64)
//	nodes := []string{"node-a", "node-b", "node-c"}
//	owner := nodes[ring([]byte("user:42"))%uint64(len(nodes))]
//	_ = owner
package hash

import (
	"encoding/binary"
	"math/bits"
)

// Hasher hashes a key to a 64-bit value; a consistent hash ring uses it to place
// keys on nodes.
type Hasher func(key []byte) uint64

const (
	offset64 = 14695981039346656037
	prime64  = 1099511628211
	offset32 = 2166136261
	prime32  = 16777619
)

// FNV1a64 returns the 64-bit FNV-1a hash of b (identical to hash/fnv's
// New64a).
func FNV1a64(b []byte) uint64 {
	h := uint64(offset64)
	for _, c := range b {
		h ^= uint64(c)
		h *= prime64
	}
	return h
}

// FNV1a32 returns the 32-bit FNV-1a hash of b.
func FNV1a32(b []byte) uint32 {
	h := uint32(offset32)
	for _, c := range b {
		h ^= uint32(c)
		h *= prime32
	}
	return h
}

// CRC32 returns the IEEE CRC-32 of b.
func CRC32(b []byte) uint32 {
	return crc32Sum(ieeeTable, b)
}

// CRC32C returns the Castagnoli CRC-32 of b.
func CRC32C(b []byte) uint32 {
	return crc32Sum(castagnoliTable, b)
}

// ieeeTable and castagnoliTable hold the reflected CRC-32 lookup tables built
// at init time (polynomials 0x04c11db7 and 0x1edc6f41, reflected).
var (
	ieeeTable       = makeTable(0xedb88320)
	castagnoliTable = makeTable(0x82f63b78)
)

// makeTable builds the 256-entry lookup table for a reflected CRC-32 polynomial.
func makeTable(poly uint32) *[256]uint32 {
	var t [256]uint32
	for i := range t {
		c := uint32(i)
		for range 8 {
			if c&1 == 1 {
				c = poly ^ c>>1
			} else {
				c >>= 1
			}
		}
		t[i] = c
	}
	return &t
}

// crc32Sum folds b through table using the standard reflected CRC-32 loop.
func crc32Sum(table *[256]uint32, b []byte) uint32 {
	crc := ^uint32(0)
	for _, c := range b {
		crc = table[byte(crc)^c] ^ crc>>8
	}
	return ^crc
}

// Adler32 returns the Adler-32 checksum of b.
func Adler32(b []byte) uint32 {
	const (
		mod  = 65521
		nmax = 5552 // largest n with 255*n*(n+1)/2 + (n+1)*(mod-1) < 2^32
	)
	var s1, s2 uint32 = 1, 0
	for len(b) > 0 {
		n := len(b)
		if n > nmax {
			n = nmax
		}
		for _, c := range b[:n] {
			s1 += uint32(c)
			s2 += s1
		}
		s1 %= mod
		s2 %= mod
		b = b[n:]
	}
	return s2<<16 | s1
}

// Murmur3_32 returns the 32-bit MurmurHash3 of b with the given seed.
func Murmur3_32(b []byte, seed uint32) uint32 {
	const (
		c1 = 0xcc9e2d51
		c2 = 0x1b873593
	)
	h1 := seed
	nblocks := len(b) / 4
	for i := range nblocks {
		k1 := binary.LittleEndian.Uint32(b[i*4:])
		k1 *= c1
		k1 = bits.RotateLeft32(k1, 15)
		k1 *= c2
		h1 ^= k1
		h1 = bits.RotateLeft32(h1, 13)
		h1 = h1*5 + 0xe6546b64
	}
	// tail: 0..3 remaining bytes, lowest byte first.
	tail := b[nblocks*4:]
	var k1 uint32
	switch len(tail) {
	case 3:
		k1 ^= uint32(tail[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint32(tail[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint32(tail[0])
		k1 *= c1
		k1 = bits.RotateLeft32(k1, 15)
		k1 *= c2
		h1 ^= k1
	}
	h1 ^= uint32(len(b))
	return fmix32(h1)
}

// fmix32 is the canonical MurmurHash3 32-bit finaliser.
func fmix32(h uint32) uint32 {
	h ^= h >> 16
	h *= 0x85ebca6b
	h ^= h >> 13
	h *= 0xc2b2ae35
	h ^= h >> 16
	return h
}

// Murmur3_128 returns the two 64-bit halves (h1, h2) of the x64 128-bit
// MurmurHash3 of b with the given seed.
func Murmur3_128(b []byte, seed uint32) (h1, h2 uint64) {
	const (
		c1 = 0x87c37b91114253d5
		c2 = 0x4cf5ad432745937f
	)
	h1 = uint64(seed)
	h2 = uint64(seed)
	nblocks := len(b) / 16
	for i := range nblocks {
		k1 := binary.LittleEndian.Uint64(b[i*16:])
		k2 := binary.LittleEndian.Uint64(b[i*16+8:])

		k1 *= c1
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= c2
		h1 ^= k1
		h1 = bits.RotateLeft64(h1, 27)
		h1 += h2
		h1 = h1*5 + 0x52dce729

		k2 *= c2
		k2 = bits.RotateLeft64(k2, 33)
		k2 *= c1
		h2 ^= k2
		h2 = bits.RotateLeft64(h2, 31)
		h2 += h1
		h2 = h2*5 + 0x38495ab5
	}
	// tail: 0..15 remaining bytes, lowest byte first.
	tail := b[nblocks*16:]
	var k1, k2 uint64
	switch len(tail) {
	case 15:
		k2 ^= uint64(tail[14]) << 48
		fallthrough
	case 14:
		k2 ^= uint64(tail[13]) << 40
		fallthrough
	case 13:
		k2 ^= uint64(tail[12]) << 32
		fallthrough
	case 12:
		k2 ^= uint64(tail[11]) << 24
		fallthrough
	case 11:
		k2 ^= uint64(tail[10]) << 16
		fallthrough
	case 10:
		k2 ^= uint64(tail[9]) << 8
		fallthrough
	case 9:
		k2 ^= uint64(tail[8])
		k2 *= c2
		k2 = bits.RotateLeft64(k2, 33)
		k2 *= c1
		h2 ^= k2
		fallthrough
	case 8:
		k1 ^= uint64(tail[7]) << 56
		fallthrough
	case 7:
		k1 ^= uint64(tail[6]) << 48
		fallthrough
	case 6:
		k1 ^= uint64(tail[5]) << 40
		fallthrough
	case 5:
		k1 ^= uint64(tail[4]) << 32
		fallthrough
	case 4:
		k1 ^= uint64(tail[3]) << 24
		fallthrough
	case 3:
		k1 ^= uint64(tail[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint64(tail[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint64(tail[0])
		k1 *= c1
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= c2
		h1 ^= k1
	}
	h1 ^= uint64(len(b))
	h2 ^= uint64(len(b))
	h1 += h2
	h2 += h1
	h1 = fmix64(h1)
	h2 = fmix64(h2)
	h1 += h2
	h2 += h1
	return h1, h2
}

// fmix64 is the canonical MurmurHash3 x64 finaliser.
func fmix64(k uint64) uint64 {
	k ^= k >> 33
	k *= 0xff51afd7ed558ccd
	k ^= k >> 33
	k *= 0xc4ceb9fe1a85ec53
	k ^= k >> 33
	return k
}

// Sum64 returns the package's default 64-bit hash of b: the first half of the
// x64 128-bit MurmurHash3 with seed 0. It is the Hasher used by default.
func Sum64(b []byte) uint64 {
	h1, _ := Murmur3_128(b, 0)
	return h1
}

// Sum32 returns the package's default 32-bit hash of b: MurmurHash3_x86_32 with
// seed 0.
func Sum32(b []byte) uint32 {
	return Murmur3_32(b, 0)
}
