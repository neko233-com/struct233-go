package atomiclongmap_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/atomiclongmap"
)

// BenchmarkAddHotPath measures Add on an already-present key, which must not
// allocate and only briefly holds the read lock.
func BenchmarkAddHotPath(b *testing.B) {
	m := atomiclongmap.New[string]()
	m.Add("k", 1)

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		m.Add("k", 1)
	}
}

// BenchmarkIncrementHotPath measures the Increment fast path on an existing
// key.
func BenchmarkIncrementHotPath(b *testing.B) {
	m := atomiclongmap.New[string]()
	m.Increment("k")

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		m.Increment("k")
	}
}

// BenchmarkGet measures reading an existing counter.
func BenchmarkGet(b *testing.B) {
	m := atomiclongmap.New[string]()
	m.Put("k", 42)

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = m.Get("k")
	}
}

// BenchmarkAddNewKey measures the cold path that inserts a new counter cell
// (one allocation per distinct key).
func BenchmarkAddNewKey(b *testing.B) {
	m := atomiclongmap.New[int]()

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		m.Add(i, 1)
	}
}

// BenchmarkAsMap measures snapshotting a 1000-entry map.
func BenchmarkAsMap(b *testing.B) {
	m := atomiclongmap.New[int]()
	for i := 0; i < 1000; i++ {
		m.Put(i, int64(i))
	}

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = m.AsMap()
	}
}
