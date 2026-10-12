package hashring_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/hash"
	"github.com/neko233-com/struct233-go/hashring"
)

// insertNode inserts name into the sorted model when it is not present yet.
func insertNode(model []string, name string) []string {
	idx, found := indexNode(model, name)
	if found {
		return model
	}
	model = append(model, "")
	copy(model[idx+1:], model[idx:])
	model[idx] = name
	return model
}

// indexNode returns the sorted position of name and whether it is present.
func indexNode(model []string, name string) (int, bool) {
	idx := sort.SearchStrings(model, name)
	if idx < len(model) && model[idx] == name {
		return idx, true
	}
	return idx, false
}

// FuzzRing replays random Add/Remove/Get scripts. The node set is mirrored in
// a sorted model, which makes "the ring holds exactly these nodes" directly
// checkable, and the routing properties are asserted after every step.
func FuzzRing(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x01})
	f.Add([]byte{0x01, 0x02, 0x00, 0x03, 0x02, 0x00})
	f.Add([]byte{0x00, 0x00, 0x00, 0x01, 0x01, 0x01, 0x02, 0x02, 0x02})
	f.Add([]byte{0x06, 0x0f, 0x05, 0x0e, 0x02, 0x0d, 0x07, 0x0c, 0x03, 0x0b})

	const replicas = 4

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1024 {
			data = data[:1024]
		}
		r := hashring.New(hashring.WithReplicas(replicas), hashring.WithHasher(hash.FNV1a64))
		model := []string{}

		for i := 0; i+1 < len(data); i += 2 {
			name := "n" + string(rune('a'+data[i]%6))
			key := "k" + string(rune('a'+data[i+1]%16))

			if data[i]&1 == 0 {
				r.Add(name)
				model = insertNode(model, name)
			} else {
				idx, found := indexNode(model, name)
				require.Equal(t, found, r.Remove(name), "Remove(%q)", name)
				if found {
					model = append(model[:idx], model[idx+1:]...)
				}
			}

			// the ring holds exactly these nodes
			require.Equal(t, model, r.Nodes())
			require.Equal(t, len(model)*replicas, r.Len())
			require.Equal(t, replicas, r.Replicas())

			if len(model) == 0 {
				_, ok := r.Get(key)
				require.False(t, ok)
				require.Nil(t, r.GetN(key, 3))
				continue
			}

			node, ok := r.Get(key)
			require.True(t, ok)
			require.Contains(t, model, node)
			again, ok := r.Get(key)
			require.True(t, ok)
			require.Equal(t, node, again, "Get must be deterministic")

			// GetN: at most the node count, distinct, Get first, prefix stable
			prev := []string{}
			for n := 1; n <= len(model)+1; n++ {
				got := r.GetN(key, n)
				require.Len(t, got, min(n, len(model)), "GetN(%q, %d)", key, n)
				require.Equal(t, node, got[0], "GetN must start at Get's node")
				if n > 1 {
					require.Equal(t, prev, got[:len(prev)], "GetN(%d) must extend GetN(%d)", n, n-1)
				}
				seen := make(map[string]bool, len(got))
				for _, gotName := range got {
					require.False(t, seen[gotName], "node %q returned twice", gotName)
					seen[gotName] = true
					require.Contains(t, model, gotName)
				}
				prev = got
			}
			require.Nil(t, r.GetN(key, 0))
			require.Nil(t, r.GetN(key, -1))
		}

		// replaying the same script on a fresh ring must match, so the mapping
		// cannot depend on add order or on rebuild history
		replay := hashring.New(hashring.WithReplicas(replicas), hashring.WithHasher(hash.FNV1a64))
		for i := 0; i+1 < len(data); i += 2 {
			name := "n" + string(rune('a'+data[i]%6))
			if data[i]&1 == 0 {
				replay.Add(name)
			} else {
				replay.Remove(name)
			}
		}
		require.Equal(t, r.Nodes(), replay.Nodes())
		require.Equal(t, r.Len(), replay.Len())
		for i := 0; i+1 < len(data); i += 2 {
			key := "k" + string(rune('a'+data[i+1]%16))
			got, gotOK := r.Get(key)
			want, wantOK := replay.Get(key)
			require.Equal(t, want, got)
			require.Equal(t, wantOK, gotOK)
		}
	})
}
