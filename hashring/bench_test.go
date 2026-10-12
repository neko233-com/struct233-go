package hashring_test

import (
	"strconv"
	"testing"

	"github.com/neko233-com/struct233-go/hash"
	"github.com/neko233-com/struct233-go/hashring"
)

// benchKeys returns 1024 deterministic keys and the mask that indexes them.
func benchKeys() ([]string, int) {
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = "user:" + strconv.Itoa(i)
	}
	return keys, len(keys) - 1
}

// benchRing returns a ring with the usual three nodes and FNV-1a hashing.
func benchRing() *hashring.Ring {
	r := hashring.New(hashring.WithReplicas(100), hashring.WithHasher(hash.FNV1a64))
	r.Add("alpha", "beta", "gamma")
	return r
}

func BenchmarkRing_Get(b *testing.B) {
	r := benchRing()
	keys, mask := benchKeys()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		node, ok := r.Get(keys[i&mask])
		if !ok || node == "" {
			b.Fatal("empty lookup")
		}
	}
}

// BenchmarkRing_Get_AndLen checks that a lookup stays allocation free when the
// caller also reads the ring's counters.
func BenchmarkRing_Get_AndLen(b *testing.B) {
	r := benchRing()
	keys, mask := benchKeys()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		node, ok := r.Get(keys[i&mask])
		if !ok || node == "" {
			b.Fatal("empty lookup")
		}
		r.Len()
	}
}

func BenchmarkRing_GetN_Three(b *testing.B) {
	r := benchRing()
	keys, mask := benchKeys()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		out := r.GetN(keys[i&mask], 3)
		if len(out) != 3 {
			b.Fatalf("GetN returned %d nodes", len(out))
		}
	}
}

func BenchmarkRing_GetN_One(b *testing.B) {
	r := benchRing()
	keys, mask := benchKeys()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		out := r.GetN(keys[i&mask], 1)
		if len(out) != 1 {
			b.Fatalf("GetN returned %d nodes", len(out))
		}
	}
}

func BenchmarkRing_Add(b *testing.B) {
	r := benchRing()
	names := make([]string, 64)
	for i := range names {
		names[i] = "node-" + strconv.Itoa(i)
	}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		name := names[i&63]
		r.Add(name)
		if !r.Remove(name) {
			b.Fatalf("remove(%q) reported a missing node", name)
		}
	}
}

func BenchmarkRing_Remove_OnlyNode(b *testing.B) {
	r := hashring.New(hashring.WithReplicas(100), hashring.WithHasher(hash.FNV1a64))
	r.Add("solo")
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if !r.Remove("solo") {
			b.Fatal("remove reported a missing node")
		}
		r.Add("solo")
	}
}

func BenchmarkRing_Nodes(b *testing.B) {
	r := benchRing()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if len(r.Nodes()) != 3 {
			b.Fatal("unexpected node count")
		}
	}
}
