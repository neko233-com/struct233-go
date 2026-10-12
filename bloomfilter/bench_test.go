package bloomfilter_test

import (
	"fmt"
	"testing"

	"github.com/neko233-com/struct233-go/bloomfilter"
)

// sink keeps benchmark results alive so the compiler cannot drop the work.
var sink any

func BenchmarkBloomFilterAddString(b *testing.B) {
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = fmt.Sprintf("player-%d", i)
	}
	f := bloomfilter.New(1<<20, 0.01)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		f.AddString(keys[i&1023])
	}
	sink = f.Count()
}

func BenchmarkBloomFilterTestStringHit(b *testing.B) {
	keys := make([]string, 1024)
	f := bloomfilter.New(1024, 0.01)
	for i := range keys {
		keys[i] = fmt.Sprintf("player-%d", i)
		f.AddString(keys[i])
	}
	hit := false
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		hit = f.TestString(keys[i&1023])
	}
	sink = hit
}

func BenchmarkBloomFilterTestStringMiss(b *testing.B) {
	f := bloomfilter.New(1024, 0.01)
	for i := range 1024 {
		f.AddString(fmt.Sprintf("player-%d", i))
	}
	hit := true
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		hit = f.TestString(fmt.Sprintf("absent-%d", i&1023))
	}
	sink = hit
}

func BenchmarkBloomFilterAddUint64(b *testing.B) {
	f := bloomfilter.New(1<<20, 0.01)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		f.AddUint64(uint64(i))
	}
	sink = f.Count()
}

func BenchmarkBloomFilterTestUint64Hit(b *testing.B) {
	f := bloomfilter.New(4096, 0.01)
	for i := range 2048 {
		f.AddUint64(uint64(i))
	}
	hit := false
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		hit = f.TestUint64(uint64(i & 2047))
	}
	sink = hit
}

func BenchmarkBloomFilterAddBytes(b *testing.B) {
	bufs := make([][]byte, 256)
	for i := range bufs {
		bufs[i] = []byte(fmt.Sprintf("token-%04d", i))
	}
	f := bloomfilter.New(1<<16, 0.01)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		f.AddBytes(bufs[i&255])
	}
	sink = f.Count()
}

func BenchmarkBloomFilterTestBytesHit(b *testing.B) {
	f := bloomfilter.New(256, 0.01)
	bufs := make([][]byte, 256)
	for i := range bufs {
		bufs[i] = []byte(fmt.Sprintf("token-%04d", i))
		f.AddBytes(bufs[i])
	}
	hit := false
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		hit = f.TestBytes(bufs[i&255])
	}
	sink = hit
}

func BenchmarkBloomFilterClear(b *testing.B) {
	f := bloomfilter.New(4096, 0.01)
	for i := range 2048 {
		f.AddUint64(uint64(i))
	}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		f.Clear()
		f.AddUint64(uint64(i & 2047))
	}
	sink = f.PopCount()
}
