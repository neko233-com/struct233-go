package bidimap_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/bidimap"
)

// decodeBiOps turns fuzz bytes into at most maxOps operations of three bytes
// each: one operation code, one key and one value.
func decodeBiOps(data []byte) []biOp {
	const (
		maxOps  = 24
		keySpan = 6
		valSpan = 5
	)
	ops := make([]biOp, 0, maxOps)
	for i := 0; i+2 < len(data) && len(ops) < maxOps; i += 3 {
		key := string(rune('a' + data[i+1]%keySpan))
		value := string(rune('x' + data[i+2]%valSpan))
		switch data[i] % 6 {
		case 0, 1, 2:
			ops = append(ops, biOp{kind: "put", k: key, v: value})
		case 3, 4:
			ops = append(ops, biOp{kind: "force", k: key, v: value})
		case 5:
			if data[i]%12 == 5 {
				ops = append(ops, biOp{kind: "clear"})
			} else {
				ops = append(ops, biOp{kind: "remove", k: key})
			}
		}
	}
	return ops
}

// FuzzBiMap replays random Put, ForcePut, Remove and Clear operations against
// the two-map reference model and checks both the step results and the full
// state after every step.
func FuzzBiMap(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2})
	f.Add([]byte{0, 0, 0, 0, 0, 0})
	f.Add([]byte{3, 1, 2, 4, 3, 4, 0, 5, 1})
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})
	f.Add([]byte{255, 254, 253, 252, 251, 250, 249, 248, 247})

	f.Fuzz(func(t *testing.T, data []byte) {
		b := bidimap.New[string, string]()
		ref := newRefBiMap[string, string]()
		for i, o := range decodeBiOps(data) {
			switch o.kind {
			case "put":
				got := b.Put(o.k, o.v)
				want := ref.put(o.k, o.v)
				if got != want {
					t.Fatalf("op %d: Put(%q, %q) = %v, want %v", i, o.k, o.v, got, want)
				}
			case "force":
				b.ForcePut(o.k, o.v)
				ref.forcePut(o.k, o.v)
			case "remove":
				gotV, gotOK := b.Remove(o.k)
				wantV, wantOK := ref.remove(o.k)
				if gotOK != wantOK || gotV != wantV {
					t.Fatalf("op %d: Remove(%q) = (%q, %v), want (%q, %v)", i, o.k, gotV, gotOK, wantV, wantOK)
				}
			case "clear":
				b.Clear()
				ref.clear()
			}
			if b.Len() != len(ref.fwd) {
				t.Fatalf("op %d: Len() = %d, want %d", i, b.Len(), len(ref.fwd))
			}
			for k, v := range ref.fwd {
				gotV, ok := b.Get(k)
				if !ok || gotV != v {
					t.Fatalf("op %d: Get(%q) = (%q, %v), want (%q, true)", i, k, gotV, ok, v)
				}
				gotK, ok := b.GetKeyOf(v)
				if !ok || gotK != k {
					t.Fatalf("op %d: GetKeyOf(%q) = (%q, %v), want (%q, true)", i, v, gotK, ok, k)
				}
			}
		}
	})
}
