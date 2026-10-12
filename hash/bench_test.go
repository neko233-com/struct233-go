package hash_test

import (
	"fmt"
	"testing"

	"github.com/neko233-com/struct233-go/hash"
)

// benchSizes are the input sizes measured by the benchmarks.
var benchSizes = []int{8, 64, 1024}

// benchData holds one deterministic input per size in benchSizes.
var benchData = func() map[int][]byte {
	m := make(map[int][]byte, len(benchSizes))
	for _, n := range benchSizes {
		m[n] = randBytes(newRand(1234), n)
	}
	return m
}()

// benchSink64 and benchSink32 keep benchmark results observable so the compiler
// cannot discard the hashing work.
var (
	benchSink64 uint64
	benchSink32 uint32
)

// BenchmarkSum64 measures the default 64-bit hash.
func BenchmarkSum64(b *testing.B) {
	for _, n := range benchSizes {
		data := benchData[n]
		b.Run(fmt.Sprintf("len=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n))
			for i := 0; b.Loop(); i++ {
				benchSink64 = hash.Sum64(data)
			}
		})
	}
}

// BenchmarkSum32 measures the default 32-bit hash.
func BenchmarkSum32(b *testing.B) {
	for _, n := range benchSizes {
		data := benchData[n]
		b.Run(fmt.Sprintf("len=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n))
			for i := 0; b.Loop(); i++ {
				benchSink32 = hash.Sum32(data)
			}
		})
	}
}

// BenchmarkMurmur3_128 measures the seeded x64 128-bit hash.
func BenchmarkMurmur3_128(b *testing.B) {
	for _, n := range benchSizes {
		data := benchData[n]
		b.Run(fmt.Sprintf("len=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n))
			for i := 0; b.Loop(); i++ {
				benchSink64, _ = hash.Murmur3_128(data, 42)
			}
		})
	}
}

// BenchmarkMurmur3_32 measures the seeded x86 32-bit hash.
func BenchmarkMurmur3_32(b *testing.B) {
	for _, n := range benchSizes {
		data := benchData[n]
		b.Run(fmt.Sprintf("len=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n))
			for i := 0; b.Loop(); i++ {
				benchSink32 = hash.Murmur3_32(data, 42)
			}
		})
	}
}

// BenchmarkFNV1a64 measures the 64-bit FNV-1a hash.
func BenchmarkFNV1a64(b *testing.B) {
	for _, n := range benchSizes {
		data := benchData[n]
		b.Run(fmt.Sprintf("len=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n))
			for i := 0; b.Loop(); i++ {
				benchSink64 = hash.FNV1a64(data)
			}
		})
	}
}

// BenchmarkCRC32 measures the IEEE CRC-32 checksum.
func BenchmarkCRC32(b *testing.B) {
	for _, n := range benchSizes {
		data := benchData[n]
		b.Run(fmt.Sprintf("len=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n))
			for i := 0; b.Loop(); i++ {
				benchSink32 = hash.CRC32(data)
			}
		})
	}
}

// BenchmarkCRC32C measures the Castagnoli CRC-32 checksum.
func BenchmarkCRC32C(b *testing.B) {
	for _, n := range benchSizes {
		data := benchData[n]
		b.Run(fmt.Sprintf("len=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n))
			for i := 0; b.Loop(); i++ {
				benchSink32 = hash.CRC32C(data)
			}
		})
	}
}

// BenchmarkAdler32 measures the Adler-32 checksum.
func BenchmarkAdler32(b *testing.B) {
	for _, n := range benchSizes {
		data := benchData[n]
		b.Run(fmt.Sprintf("len=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n))
			for i := 0; b.Loop(); i++ {
				benchSink32 = hash.Adler32(data)
			}
		})
	}
}

// BenchmarkHasherRing measures the Hasher facade over the same inputs.
func BenchmarkHasherRing(b *testing.B) {
	ring := hash.Hasher(hash.Sum64)
	for _, n := range benchSizes {
		data := benchData[n]
		b.Run(fmt.Sprintf("len=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n))
			for i := 0; b.Loop(); i++ {
				benchSink64 = ring(data)
			}
		})
	}
}
