package hashring_test

import (
	"bytes"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/hash"
	"github.com/neko233-com/struct233-go/hashring"
)

// newRing builds a ring with the default MurmurHash3 hasher, whose avalanche
// spreads the "user:N" test keys evenly around the circle. FNV-1a would cluster
// them, which would make the distribution assertions below meaningless.
func newRing(replicas int, nodes ...string) *hashring.Ring {
	r := hashring.New(hashring.WithReplicas(replicas), hashring.WithHasher(hash.Sum64))
	r.Add(nodes...)
	return r
}

// keyAt returns the deterministic test key at index i.
func keyAt(i int) string { return "user:" + strconv.Itoa(i) }

// keysOf returns n deterministic keys.
func keysOf(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = keyAt(i)
	}
	return keys
}

// ownersOf records the owner of every key so a later call can be compared
// against it.
func ownersOf(t *testing.T, r *hashring.Ring, keys []string) map[string]string {
	t.Helper()
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		node, ok := r.Get(k)
		require.True(t, ok, "Get(%q)", k)
		require.NotEmpty(t, node)
		out[k] = node
	}
	return out
}

// hashCounts returns how many keys each node owns.
func hashCounts(t *testing.T, r *hashring.Ring, keys []string) map[string]int {
	t.Helper()
	counts := make(map[string]int)
	for _, k := range keys {
		node, ok := r.Get(k)
		require.True(t, ok, "Get(%q)", k)
		counts[node]++
	}
	return counts
}

// constHasher returns the same value for every input, so every virtual point
// collides and the ring has exactly one distinct point value.
func constHasher([]byte) uint64 { return 42 }

func TestNew_Defaults(t *testing.T) {
	tests := []struct {
		name         string
		opts         []hashring.Option
		wantReplicas int
	}{
		{"no options", nil, 100},
		{"single replica", []hashring.Option{hashring.WithReplicas(1)}, 1},
		{"large replica count", []hashring.Option{hashring.WithReplicas(512)}, 512},
		{"hasher only keeps default replicas", []hashring.Option{hashring.WithHasher(hash.FNV1a64)}, 100},
		{"last replicas option wins", []hashring.Option{hashring.WithReplicas(7), hashring.WithReplicas(9)}, 9},
		{"both options", []hashring.Option{hashring.WithHasher(hash.FNV1a64), hashring.WithReplicas(3)}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := hashring.New(tt.opts...)
			require.Equal(t, tt.wantReplicas, r.Replicas())
			require.Zero(t, r.Len())
			require.Empty(t, r.Nodes())

			node, ok := r.Get("anything")
			require.False(t, ok)
			require.Empty(t, node)
			require.Empty(t, r.GetN("anything", 3))
			require.False(t, r.Remove("anything"))
		})
	}
}

func TestNew_Panics(t *testing.T) {
	tests := []struct {
		name string
		opts []hashring.Option
		want string
	}{
		{"zero replicas", []hashring.Option{hashring.WithReplicas(0)}, "hashring: replicas must be positive"},
		{"negative replicas", []hashring.Option{hashring.WithReplicas(-1)}, "hashring: replicas must be positive"},
		{"nil hasher", []hashring.Option{hashring.WithHasher(nil)}, "hashring: nil hasher"},
		{"nil option", []hashring.Option{nil}, "hashring: nil option"},
		{"bad option after a good one", []hashring.Option{hashring.WithReplicas(4), hashring.WithReplicas(-4)}, "hashring: replicas must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.PanicsWithValue(t, tt.want, func() { hashring.New(tt.opts...) })
		})
	}
}

func TestAddRemove_Sequence(t *testing.T) {
	tests := []struct {
		name       string
		replicas   int
		script     []string // "+name" adds, "-name" removes
		wantRemove []bool   // one entry per '-' step
		wantNodes  []string
	}{
		{
			name:      "add a single node",
			script:    []string{"+alpha"},
			wantNodes: []string{"alpha"},
		},
		{
			name:      "add three nodes",
			script:    []string{"+alpha", "+beta", "+gamma"},
			wantNodes: []string{"alpha", "beta", "gamma"},
		},
		{
			name:      "duplicate names in one call count once",
			script:    []string{"+alpha", "+alpha", "+alpha"},
			wantNodes: []string{"alpha"},
		},
		{
			name:      "duplicate names across calls count once",
			script:    []string{"+alpha", "+alpha", "+beta", "+alpha"},
			wantNodes: []string{"alpha", "beta"},
		},
		{
			name:      "nodes come back sorted whatever the add order",
			script:    []string{"+gamma", "+beta", "+alpha"},
			wantNodes: []string{"alpha", "beta", "gamma"},
		},
		{
			name:       "remove a present node",
			script:     []string{"+a", "+b", "-a"},
			wantRemove: []bool{true},
			wantNodes:  []string{"b"},
		},
		{
			name:       "remove an absent node",
			script:     []string{"+a", "+b", "-zzz"},
			wantRemove: []bool{false},
			wantNodes:  []string{"a", "b"},
		},
		{
			name:       "remove twice",
			script:     []string{"+a", "-a", "-a"},
			wantRemove: []bool{true, false},
			wantNodes:  []string{},
		},
		{
			name:       "re-add after remove",
			script:     []string{"+a", "-a", "+a"},
			wantRemove: []bool{true},
			wantNodes:  []string{"a"},
		},
		{
			name:       "remove every node",
			script:     []string{"+a", "+b", "+c", "-a", "-b", "-c"},
			wantRemove: []bool{true, true, true},
			wantNodes:  []string{},
		},
		{
			name:       "interleaved add and remove",
			script:     []string{"+a", "+b", "-b", "+c", "-a"},
			wantRemove: []bool{true, true},
			wantNodes:  []string{"c"},
		},
		{
			name:       "swap nodes",
			script:     []string{"+x", "-x", "+y", "-y", "+z"},
			wantRemove: []bool{true, true},
			wantNodes:  []string{"z"},
		},
		{
			name:      "many nodes",
			script:    []string{"+n1", "+n2", "+n3", "+n4", "+n5", "+n6", "+n7", "+n8", "+n9", "+n10"},
			wantNodes: []string{"n1", "n10", "n2", "n3", "n4", "n5", "n6", "n7", "n8", "n9"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replicas := tt.replicas
			if replicas == 0 {
				replicas = 16
			}
			r := hashring.New(hashring.WithReplicas(replicas), hashring.WithHasher(hash.FNV1a64))
			removed := make([]bool, 0, len(tt.wantRemove))
			for _, step := range tt.script {
				name := step[1:]
				switch step[0] {
				case '+':
					r.Add(name)
				case '-':
					removed = append(removed, r.Remove(name))
				default:
					t.Fatalf("bad script step %q", step)
				}
				require.Equal(t, len(r.Nodes())*replicas, r.Len())
			}
			if tt.wantRemove == nil {
				require.Empty(t, removed)
			} else {
				require.Equal(t, tt.wantRemove, removed)
			}
			require.Equal(t, tt.wantNodes, r.Nodes())
			require.Equal(t, len(tt.wantNodes)*replicas, r.Len())
		})
	}
}

func TestReplicas_Honoured(t *testing.T) {
	nodes := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	tests := []struct {
		name     string
		replicas int
		count    int
	}{
		{"one virtual node per node", 1, 1},
		{"two virtual nodes per node", 2, 2},
		{"seven virtual nodes per node", 7, 3},
		{"default replicas", 100, 4},
		{"many virtual nodes per node", 256, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRing(tt.replicas, nodes[:tt.count]...)
			require.Equal(t, tt.replicas, r.Replicas())
			require.Equal(t, tt.count*tt.replicas, r.Len())

			require.True(t, r.Remove(nodes[0]))
			require.Equal(t, (tt.count-1)*tt.replicas, r.Len())

			r.Add(nodes[0])
			require.Equal(t, tt.count*tt.replicas, r.Len())
		})
	}
}

func TestGet_EmptyRing(t *testing.T) {
	tests := []struct {
		name string
		key  string
		n    int
	}{
		{"plain key", "user:42", 3},
		{"empty key", "", 1},
		{"zero n", "user:42", 0},
		{"negative n", "user:42", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := hashring.New(hashring.WithHasher(hash.FNV1a64))
			node, ok := r.Get(tt.key)
			require.False(t, ok)
			require.Empty(t, node)
			require.Empty(t, r.GetN(tt.key, tt.n))
			require.Empty(t, r.Nodes())
			require.Zero(t, r.Len())
			require.False(t, r.Remove("alpha"))
		})
	}
}

func TestGet_Deterministic(t *testing.T) {
	keys := keysOf(500)
	tests := []struct {
		name     string
		replicas int
		nodes    []string
	}{
		{"three nodes", 100, []string{"alpha", "beta", "gamma"}},
		{"one node owns everything", 100, []string{"solo"}},
		{"single replica", 1, []string{"alpha", "beta", "gamma"}},
		{"many replicas", 512, []string{"alpha", "beta", "gamma", "delta"}},
		{"five nodes", 64, []string{"n1", "n2", "n3", "n4", "n5"}},
		{"names sharing a prefix", 32, []string{"cache-1", "cache-10", "cache-2", "cache-20"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forward := newRing(tt.replicas, tt.nodes...)
			reversed := hashring.New(hashring.WithReplicas(tt.replicas), hashring.WithHasher(hash.Sum64))
			for i := len(tt.nodes) - 1; i >= 0; i-- {
				reversed.Add(tt.nodes[i])
			}

			want := ownersOf(t, forward, keys)
			require.Equal(t, want, ownersOf(t, reversed, keys), "add order must not change the mapping")
			require.Equal(t, want, ownersOf(t, forward, keys), "repeated lookups must agree")

			if len(tt.nodes) == 1 {
				for _, node := range want {
					require.Equal(t, tt.nodes[0], node)
				}
			}

			forward.Add("transient")
			if tt.replicas*len(tt.nodes) >= 100 {
				// A ring with a single point per node has sectors coarse enough
				// that the new point may legitimately own no key at all.
				require.NotEqual(t, want, ownersOf(t, forward, keys), "sanity: adding a node moves keys")
			}
			require.True(t, forward.Remove("transient"))
			require.Equal(t, want, ownersOf(t, forward, keys), "add+remove must restore the mapping")
		})
	}
}

func TestGet_PointOrderAndWrap(t *testing.T) {
	// Points of nodes "a", "b", "c" hash to 9700, 9800, 9900; keys "k:<n>" hash
	// to n. That makes the binary search and its wraparound observable.
	hasher := func(b []byte) uint64 {
		if i := bytes.IndexByte(b, 0); i >= 0 { // virtual point encoding
			return uint64(b[0]) * 100
		}
		if i := bytes.IndexByte(b, ':'); i >= 0 {
			n, err := strconv.Atoi(string(b[i+1:]))
			require.NoError(t, err)
			return uint64(n)
		}
		return 0
	}
	tests := []struct {
		name string
		key  string
		want string
	}{
		{"below every point", "k:0", "a"},
		{"just below the first point", "k:9699", "a"},
		{"exactly on the first point", "k:9700", "a"},
		{"between the first and the second picks the second", "k:9799", "b"},
		{"exactly on the second point", "k:9800", "b"},
		{"between the second and the third picks the third", "k:9899", "c"},
		{"exactly on the third point", "k:9900", "c"},
		{"just past the last point wraps to the first", "k:9901", "a"},
		{"far past the last point wraps to the first", "k:100000", "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := hashring.New(hashring.WithReplicas(1), hashring.WithHasher(hasher))
			r.Add("c", "b", "a") // added out of order on purpose
			require.Equal(t, 3, r.Len())
			require.Equal(t, []string{"a", "b", "c"}, r.Nodes())

			// the key "k:<n>" hashes to n, so the expected node is fixed
			node, ok := r.Get(tt.key)
			require.True(t, ok)
			require.Equal(t, tt.want, node)

			// the same position must be reachable through GetN
			require.Equal(t, tt.want, r.GetN(tt.key, 3)[0])
		})
	}
}

func TestGetN_Table(t *testing.T) {
	tests := []struct {
		name     string
		replicas int
		nodes    []string
		key      string
		n        int
		wantNil  bool
		wantLen  int
	}{
		{"empty ring", 16, nil, "user:1", 3, true, 0},
		{"empty ring with n=0", 16, nil, "user:1", 0, true, 0},
		{"n=0 returns nil", 16, []string{"a", "b"}, "user:1", 0, true, 0},
		{"n<0 returns nil", 16, []string{"a", "b"}, "user:1", -3, true, 0},
		{"n=1 is the primary", 16, []string{"a", "b", "c"}, "user:1", 1, false, 1},
		{"n=2 returns two distinct nodes", 16, []string{"a", "b", "c"}, "user:2", 2, false, 2},
		{"n=3 returns all three nodes", 16, []string{"a", "b", "c"}, "user:3", 3, false, 3},
		{"n larger than the node count is clamped", 16, []string{"a", "b", "c"}, "user:4", 99, false, 3},
		{"single node ring", 16, []string{"solo"}, "user:5", 3, false, 1},
		{"single replica ring still returns every node", 1, []string{"a", "b"}, "user:6", 2, false, 2},
		{"empty key routes", 16, []string{"a", "b"}, "", 2, false, 2},
		{"unknown key routes", 16, []string{"a", "b"}, "no-such-key", 2, false, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := hashring.New(hashring.WithReplicas(tt.replicas), hashring.WithHasher(hash.Sum64))
			r.Add(tt.nodes...)

			got := r.GetN(tt.key, tt.n)
			if tt.wantNil {
				require.Nil(t, got)
				return
			}
			require.Len(t, got, tt.wantLen)
			require.Equal(t, got, r.GetN(tt.key, tt.n), "GetN must be deterministic")

			seen := make(map[string]struct{}, len(got))
			nodes := r.Nodes()
			for _, node := range got {
				_, dup := seen[node]
				require.False(t, dup, "node %q returned twice", node)
				seen[node] = struct{}{}
				require.Contains(t, nodes, node)
			}

			primary, ok := r.Get(tt.key)
			require.True(t, ok)
			require.Equal(t, primary, got[0], "GetN must start at Get's node")
		})
	}
}

func TestGetN_PrefixConsistency(t *testing.T) {
	tests := []struct {
		name string
		keys []string
	}{
		{"three nodes", keysOf(60)},
		{"one node", []string{"a", "b", "c", "d"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRing(32, "alpha", "beta", "gamma")
			if tt.name == "one node" {
				require.True(t, r.Remove("beta"))
				require.True(t, r.Remove("gamma"))
			}
			nodes := len(r.Nodes())
			for _, key := range tt.keys {
				prev := r.GetN(key, 1)
				primary, ok := r.Get(key)
				require.True(t, ok)
				require.Equal(t, []string{primary}, prev)
				for n := 1; n <= nodes+2; n++ {
					cur := r.GetN(key, n)
					require.Len(t, cur, min(n, nodes), "GetN(%q, %d)", key, n)
					require.Equal(t, cur[:len(prev)], prev, "GetN(%q, %d) must extend GetN before it", key, n)
					prev = cur
				}
			}
		})
	}
}

func TestDistribution(t *testing.T) {
	const keys = 10000
	nodes := []string{"alpha", "beta", "gamma"}
	tests := []struct {
		name     string
		replicas int
	}{
		{"50 replicas", 50},
		{"100 replicas", 100},
		{"500 replicas", 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRing(tt.replicas, nodes...)
			counts := hashCounts(t, r, keysOf(keys))
			require.Len(t, counts, len(nodes), "every node must own something")
			for _, node := range nodes {
				share := float64(counts[node]) / keys
				require.GreaterOrEqual(t, share, 0.20, "node %s owns %.1f%%", node, share*100)
				require.LessOrEqual(t, share, 0.45, "node %s owns %.1f%%", node, share*100)
			}
		})
	}
}

func TestAddNode_MovesOnlyToTheNewNode(t *testing.T) {
	const keys = 10000
	tests := []struct {
		name     string
		replicas int
	}{
		{"32 replicas", 32},
		{"100 replicas", 100},
		{"256 replicas", 256},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRing(tt.replicas, "alpha", "beta", "gamma")
			before := ownersOf(t, r, keysOf(keys))

			r.Add("delta")
			after := ownersOf(t, r, keysOf(keys))

			moved := 0
			for i := range keys {
				key := keyAt(i)
				if before[key] == after[key] {
					continue
				}
				moved++
				require.Equal(t, "delta", after[key], "key %s must move to the new node only", key)
			}
			share := float64(moved) / keys
			require.GreaterOrEqual(t, share, 0.05, "moved %.1f%% of the keys", share*100)
			require.LessOrEqual(t, share, 0.50, "moved %.1f%% of the keys", share*100)
		})
	}
}

func TestRemoveNode_MovesOnlyItsKeys(t *testing.T) {
	const keys = 10000
	tests := []struct {
		name     string
		replicas int
	}{
		{"16 replicas", 16},
		{"100 replicas", 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRing(tt.replicas, "alpha", "beta", "gamma", "delta")
			before := ownersOf(t, r, keysOf(keys))

			require.True(t, r.Remove("beta"))
			remaining := r.Nodes()
			require.Equal(t, []string{"alpha", "delta", "gamma"}, remaining)

			moved := 0
			for i := range keys {
				key := keyAt(i)
				after, ok := r.Get(key)
				require.True(t, ok)
				require.Contains(t, remaining, after)
				if before[key] != "beta" {
					require.Equal(t, before[key], after, "key %s did not belong to beta and must not move", key)
					continue
				}
				moved++
				require.NotEqual(t, "beta", after, "key %s still points at the removed node", key)
			}
			require.NotZero(t, moved, "sanity: beta owned some keys")
		})
	}

	t.Run("removing the last node empties the ring", func(t *testing.T) {
		r := newRing(16, "solo")
		require.True(t, r.Remove("solo"))
		require.Zero(t, r.Len())
		require.Empty(t, r.Nodes())
		_, ok := r.Get("user:1")
		require.False(t, ok)
	})
}

func TestCollisions_StillResolveDeterministically(t *testing.T) {
	tests := []struct {
		name     string
		replicas int
		nodes    []string
	}{
		{"every point collides", 16, []string{"alpha", "beta", "gamma"}},
		{"single replica", 1, []string{"beta", "alpha"}},
		{"one node", 4, []string{"solo"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := hashring.New(hashring.WithReplicas(tt.replicas), hashring.WithHasher(constHasher))
			r.Add(tt.nodes...)

			// colliding points are all kept
			require.Equal(t, len(tt.nodes)*tt.replicas, r.Len())
			require.Equal(t, len(tt.nodes)*tt.replicas, len(r.Nodes())*tt.replicas)

			first, ok := r.Get("user:1")
			require.True(t, ok)
			for i := 1; i < 50; i++ {
				node, ok := r.Get(keyAt(i))
				require.True(t, ok)
				require.Equal(t, first, node, "collisions must resolve the same way every time")
			}

			got := r.GetN("user:1", len(tt.nodes)+1)
			require.Equal(t, r.Nodes(), got)

			// a rebuild triggered by an unrelated add+remove keeps the mapping
			r.Add("transient")
			require.True(t, r.Remove("transient"))
			node, ok := r.Get("user:1")
			require.True(t, ok)
			require.Equal(t, first, node)
		})
	}
}

func TestConcurrent_GetWithAddRemove(t *testing.T) {
	const (
		readers = 8
		writers = 2
	)
	r := hashring.New(hashring.WithReplicas(16), hashring.WithHasher(hash.Sum64))
	r.Add("base")

	stop := make(chan struct{})
	var wg sync.WaitGroup

	for g := range readers {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				key := fmt.Sprintf("g%d:key%d", g, i)
				node, ok := r.Get(key)
				if !ok {
					t.Errorf("Get(%q) reported an empty ring", key)
					return
				}
				if node == "" {
					t.Errorf("Get(%q) returned an empty node", key)
					return
				}
				// "base" is never removed, so every lookup must find a node
				// even though the other nodes churn underneath.
				out := r.GetN(key, 3)
				if len(out) == 0 || len(out) > 3 {
					t.Errorf("GetN(%q, 3) returned %d nodes", key, len(out))
					return
				}
				seen := make(map[string]bool, len(out))
				for _, name := range out {
					if seen[name] {
						t.Errorf("GetN(%q, 3) repeated node %q", key, name)
						return
					}
					seen[name] = true
				}
				if got := r.Replicas(); got != 16 {
					t.Errorf("Replicas() = %d", got)
					return
				}
				if r.Len() <= 0 {
					t.Errorf("Len() = %d", r.Len())
					return
				}
				if len(r.Nodes()) == 0 {
					t.Error("Nodes() is empty")
					return
				}
			}
		}(g)
	}

	for w := range writers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range 300 {
				name := fmt.Sprintf("w%d-node%d", w, i%7)
				r.Add(name)
				r.Remove(name)
				r.Add(name)
			}
			if w == 0 {
				close(stop)
			}
		}(w)
	}

	wg.Wait()

	require.True(t, r.Remove("base"))
	r.Add("after")
	node, ok := r.Get("user:42")
	require.True(t, ok)
	require.NotEmpty(t, node)
}

// sinkNode and sinkOK keep Get's results alive so the allocation guard cannot
// be optimised away.
var (
	sinkNode string
	sinkOK   bool
)

func TestGet_HasNoAllocations(t *testing.T) {
	r := newRing(100, "alpha", "beta", "gamma")
	keys := []string{"user:42", "user:43", "", "a-very-long-key-that-still-only-needs-hashing"}

	for _, key := range keys {
		sinkNode, sinkOK = r.Get(key)
		require.True(t, sinkOK)

		allocs := testing.AllocsPerRun(100, func() {
			sinkNode, sinkOK = r.Get(key)
		})
		require.Zero(t, allocs, "Get(%q) allocated %.0f times", key, allocs)
	}
}

func TestHasher_IsAppliedToKeysAndPoints(t *testing.T) {
	var mu sync.Mutex
	var seen int
	counting := func(b []byte) uint64 {
		mu.Lock()
		seen++
		mu.Unlock()
		return hash.FNV1a64(b)
	}

	r := hashring.New(hashring.WithReplicas(4), hashring.WithHasher(counting))
	r.Add("alpha", "beta")
	require.Equal(t, 8, seen, "one hash call per virtual point")

	before := seen
	_, ok := r.Get("user:1")
	require.True(t, ok)
	require.Equal(t, before+1, seen, "one hash call per lookup")

	require.True(t, r.Remove("beta"))
	require.Equal(t, before+1+4, seen, "rebuilding hashes the points that remain")
}
