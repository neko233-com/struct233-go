package bidimap_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/bidimap"
)

// refBiMap is a plain map[K]V + map[V]K reference model of the BiMap
// semantics, used for differential testing.
type refBiMap[K, V comparable] struct {
	fwd map[K]V
	rev map[V]K
}

// newRefBiMap returns an empty reference model.
func newRefBiMap[K, V comparable]() *refBiMap[K, V] {
	return &refBiMap[K, V]{fwd: map[K]V{}, rev: map[V]K{}}
}

// put mirrors BiMap.Put: false and no change when the value is taken.
func (r *refBiMap[K, V]) put(k K, v V) bool {
	if old, ok := r.fwd[k]; ok {
		if old == v {
			return true
		}
		if _, taken := r.rev[v]; taken {
			return false
		}
		delete(r.rev, old)
	} else if _, taken := r.rev[v]; taken {
		return false
	}
	r.fwd[k] = v
	r.rev[v] = k
	return true
}

// forcePut mirrors BiMap.ForcePut: both conflicting bindings are dropped.
func (r *refBiMap[K, V]) forcePut(k K, v V) {
	if old, ok := r.fwd[k]; ok && old != v {
		delete(r.rev, old)
	}
	if bound, taken := r.rev[v]; taken && bound != k {
		delete(r.fwd, bound)
	}
	r.fwd[k] = v
	r.rev[v] = k
}

// remove mirrors BiMap.Remove.
func (r *refBiMap[K, V]) remove(k K) (V, bool) {
	v, ok := r.fwd[k]
	if !ok {
		var zero V
		return zero, false
	}
	delete(r.fwd, k)
	delete(r.rev, v)
	return v, true
}

// clear mirrors BiMap.Clear.
func (r *refBiMap[K, V]) clear() {
	clear(r.fwd)
	clear(r.rev)
}

// requireInvariants re-checks the bijection contract: forward and reverse must
// be exact inverses of each other and of the live Inverse view.
func requireInvariants[K, V comparable](t *testing.T, b *bidimap.BiMap[K, V]) {
	t.Helper()

	keys := b.Keys()
	values := b.Values()
	require.Equal(t, b.Len(), len(keys), "Keys() must hold exactly the bound keys")
	require.Equal(t, b.Len(), len(values), "Values() must hold exactly the bound values")

	inv := b.Inverse()
	require.Equal(t, b.Len(), inv.Len(), "Inverse() must hold exactly as many pairs")
	for k, v := range b.Entries() {
		gotV, ok := b.Get(k)
		require.True(t, ok, "key %v yielded by Entries must be bound", k)
		require.Equal(t, v, gotV, "Entries must agree with Get")
		gotK, ok := b.GetKeyOf(v)
		require.True(t, ok, "value %v yielded by Entries must be bound", v)
		require.Equal(t, k, gotK, "GetKeyOf must invert Get")
		invK, ok := inv.Get(v)
		require.True(t, ok, "Inverse must know value %v", v)
		require.Equal(t, k, invK, "Inverse must invert the forward mapping")
		invV, ok := inv.GetKeyOf(k)
		require.True(t, ok, "Inverse must know key %v", k)
		require.Equal(t, v, invV, "Inverse.GetKeyOf must agree with Get")
	}
}

// requireMatchesModel asserts that b holds exactly the reference model state in
// both directions.
func requireMatchesModel[K, V comparable](t *testing.T, b *bidimap.BiMap[K, V], ref *refBiMap[K, V]) {
	t.Helper()

	require.Equal(t, len(ref.fwd), b.Len(), "Len must match the reference model")
	got := map[K]V{}
	for k, v := range b.Entries() {
		got[k] = v
	}
	require.Equal(t, ref.fwd, got, "forward mapping must match the reference model")
	for k, v := range ref.fwd {
		gotV, ok := b.Get(k)
		require.True(t, ok, "key %v must stay bound", k)
		require.Equal(t, v, gotV, "Get must match the reference model")
		gotK, ok := b.GetKeyOf(v)
		require.True(t, ok, "value %v must stay bound", v)
		require.Equal(t, k, gotK, "GetKeyOf must match the reference model")
	}
}

// biOp is one scripted step of a table-driven scenario.
type biOp struct {
	kind   string // "put", "force", "remove" or "clear"
	k, v   string // v doubles as the expected value of a remove
	wantOK bool   // result of Put / Remove; unused for force and clear
}

// TestBiMapScenarios replays scripted scenarios against both the bimap and the
// reference model, checking every step result and the final state.
func TestBiMapScenarios(t *testing.T) {
	scenarios := []struct {
		name  string
		steps []biOp
		want  map[string]string
	}{
		{
			name:  "put into an empty map",
			steps: []biOp{{kind: "put", k: "a", v: "x", wantOK: true}},
			want:  map[string]string{"a": "x"},
		},
		{
			name: "put several distinct pairs",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "y", wantOK: true},
				{kind: "put", k: "c", v: "z", wantOK: true},
			},
			want: map[string]string{"a": "x", "b": "y", "c": "z"},
		},
		{
			name: "put the same pair twice keeps one binding",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "a", v: "x", wantOK: true},
			},
			want: map[string]string{"a": "x"},
		},
		{
			name: "put the same key with a new value rebinds it",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "a", v: "y", wantOK: true},
			},
			want: map[string]string{"a": "y"},
		},
		{
			name: "re-put frees the previous value",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "a", v: "y", wantOK: true},
				{kind: "put", k: "b", v: "x", wantOK: true},
			},
			want: map[string]string{"a": "y", "b": "x"},
		},
		{
			name: "put rejects a value bound to another key",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "x", wantOK: false},
			},
			want: map[string]string{"a": "x"},
		},
		{
			name: "a rejected put changes nothing",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "x", wantOK: false},
				{kind: "put", k: "c", v: "y", wantOK: true},
			},
			want: map[string]string{"a": "x", "c": "y"},
		},
		{
			name: "a rejected put leaves its key free",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "x", wantOK: false},
				{kind: "put", k: "b", v: "y", wantOK: true},
			},
			want: map[string]string{"a": "x", "b": "y"},
		},
		{
			name: "put rejects a value taken by a later re-put",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "y", wantOK: true},
				{kind: "put", k: "a", v: "y", wantOK: false},
			},
			want: map[string]string{"a": "x", "b": "y"},
		},
		{
			name: "the same key can take its own freed value",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "a", v: "y", wantOK: true},
				{kind: "put", k: "a", v: "x", wantOK: true},
			},
			want: map[string]string{"a": "x"},
		},
		{
			name: "a chain of re-puts frees each value in turn",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "a", v: "y", wantOK: true},
				{kind: "put", k: "a", v: "z", wantOK: true},
			},
			want: map[string]string{"a": "z"},
		},
		{
			name: "re-put of one key keeps the other bindings",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "y", wantOK: true},
				{kind: "put", k: "c", v: "z", wantOK: true},
				{kind: "put", k: "b", v: "w", wantOK: true},
			},
			want: map[string]string{"a": "x", "b": "w", "c": "z"},
		},
		{
			name: "remove frees the value for another key",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "remove", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "x", wantOK: true},
			},
			want: map[string]string{"b": "x"},
		},
		{
			name: "remove then re-put the same key",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "remove", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "a", v: "x", wantOK: true},
			},
			want: map[string]string{"a": "x"},
		},
		{
			name:  "remove of a missing key reports false",
			steps: []biOp{{kind: "remove", k: "z", wantOK: false}},
			want:  map[string]string{},
		},
		{
			name: "a second remove of the same key reports false",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "remove", k: "a", v: "x", wantOK: true},
				{kind: "remove", k: "a", wantOK: false},
			},
			want: map[string]string{},
		},
		{
			name: "remove keeps the other bindings",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "y", wantOK: true},
				{kind: "put", k: "c", v: "z", wantOK: true},
				{kind: "remove", k: "b", v: "y", wantOK: true},
			},
			want: map[string]string{"a": "x", "c": "z"},
		},
		{
			name: "remove then rebind to a new value",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "remove", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "a", v: "y", wantOK: true},
			},
			want: map[string]string{"a": "y"},
		},
		{
			name:  "force put into an empty map",
			steps: []biOp{{kind: "force", k: "a", v: "x"}},
			want:  map[string]string{"a": "x"},
		},
		{
			name: "force put rebinds its own key",
			steps: []biOp{
				{kind: "force", k: "a", v: "x"},
				{kind: "force", k: "a", v: "y"},
			},
			want: map[string]string{"a": "y"},
		},
		{
			name: "force put on the same pair is idempotent",
			steps: []biOp{
				{kind: "force", k: "a", v: "x"},
				{kind: "force", k: "a", v: "x"},
			},
			want: map[string]string{"a": "x"},
		},
		{
			name: "force put steals a bound value",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "force", k: "b", v: "x"},
			},
			want: map[string]string{"b": "x"},
		},
		{
			name: "force put drops the stolen key entirely",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "c", v: "z", wantOK: true},
				{kind: "force", k: "b", v: "x"},
			},
			want: map[string]string{"b": "x", "c": "z"},
		},
		{
			name: "force put on an existing key frees its old value",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "force", k: "a", v: "y"},
				{kind: "put", k: "b", v: "x", wantOK: true},
			},
			want: map[string]string{"a": "y", "b": "x"},
		},
		{
			name: "force put frees the stolen key's old value",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "y", wantOK: true},
				{kind: "force", k: "b", v: "x"},
				{kind: "put", k: "c", v: "y", wantOK: true},
			},
			want: map[string]string{"b": "x", "c": "y"},
		},
		{
			name: "a put is rejected after a force steal until the value frees",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "force", k: "b", v: "x"},
				{kind: "put", k: "a", v: "x", wantOK: false},
				{kind: "put", k: "a", v: "y", wantOK: true},
			},
			want: map[string]string{"a": "y", "b": "x"},
		},
		{
			name: "force put swaps two bindings",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "y", wantOK: true},
				{kind: "force", k: "a", v: "y"},
				{kind: "force", k: "b", v: "x"},
			},
			want: map[string]string{"a": "y", "b": "x"},
		},
		{
			name: "clear empties the map",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "y", wantOK: true},
				{kind: "clear"},
			},
			want: map[string]string{},
		},
		{
			name:  "clear on an empty map",
			steps: []biOp{{kind: "clear"}},
			want:  map[string]string{},
		},
		{
			name: "the map is reusable after clear",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "clear"},
				{kind: "put", k: "b", v: "x", wantOK: true},
			},
			want: map[string]string{"b": "x"},
		},
		{
			name: "clear releases every value",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "y", wantOK: true},
				{kind: "clear"},
				{kind: "put", k: "c", v: "x", wantOK: true},
				{kind: "put", k: "d", v: "y", wantOK: true},
			},
			want: map[string]string{"c": "x", "d": "y"},
		},
		{
			name: "put after clear of the same key",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "clear"},
				{kind: "put", k: "a", v: "x", wantOK: true},
			},
			want: map[string]string{"a": "x"},
		},
		{
			name: "alternating re-puts end on the last value",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "a", v: "y", wantOK: true},
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "a", v: "y", wantOK: true},
			},
			want: map[string]string{"a": "y"},
		},
		{
			name: "a key may map to a value equal to itself",
			steps: []biOp{
				{kind: "put", k: "a", v: "a", wantOK: true},
				{kind: "put", k: "b", v: "a", wantOK: false},
			},
			want: map[string]string{"a": "a"},
		},
		{
			name: "the empty string is a valid key and value",
			steps: []biOp{
				{kind: "put", k: "", v: "", wantOK: true},
				{kind: "put", k: "x", v: "", wantOK: false},
			},
			want: map[string]string{"": ""},
		},
		{
			name: "the empty key rebinds like any other",
			steps: []biOp{
				{kind: "put", k: "", v: "x", wantOK: true},
				{kind: "put", k: "", v: "y", wantOK: true},
			},
			want: map[string]string{"": "y"},
		},
		{
			name: "removing every key empties the map",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "y", wantOK: true},
				{kind: "put", k: "c", v: "z", wantOK: true},
				{kind: "remove", k: "a", v: "x", wantOK: true},
				{kind: "remove", k: "b", v: "y", wantOK: true},
				{kind: "remove", k: "c", v: "z", wantOK: true},
			},
			want: map[string]string{},
		},
		{
			name: "bindings can be swapped through remove",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "y", wantOK: true},
				{kind: "remove", k: "a", v: "x", wantOK: true},
				{kind: "remove", k: "b", v: "y", wantOK: true},
				{kind: "put", k: "b", v: "x", wantOK: true},
				{kind: "put", k: "a", v: "y", wantOK: true},
			},
			want: map[string]string{"a": "y", "b": "x"},
		},
		{
			name: "a rejected put is accepted once its value is removed",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "x", wantOK: false},
				{kind: "remove", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "x", wantOK: true},
			},
			want: map[string]string{"b": "x"},
		},
		{
			name: "interleaved force and put keep the bijection",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "force", k: "b", v: "x"},
				{kind: "put", k: "c", v: "y", wantOK: true},
				{kind: "force", k: "c", v: "x"},
			},
			want: map[string]string{"c": "x"},
		},
		{
			name: "conflicting puts are both rejected",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "y", wantOK: true},
				{kind: "put", k: "a", v: "y", wantOK: false},
				{kind: "put", k: "b", v: "x", wantOK: false},
			},
			want: map[string]string{"a": "x", "b": "y"},
		},
		{
			name: "force put with a fresh key and value touches nothing",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "force", k: "b", v: "y"},
			},
			want: map[string]string{"a": "x", "b": "y"},
		},
		{
			name: "remove after a force steal reports absence",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "force", k: "b", v: "x"},
				{kind: "remove", k: "a", wantOK: false},
			},
			want: map[string]string{"b": "x"},
		},
		{
			name: "five distinct pairs then rebind the middle one",
			steps: []biOp{
				{kind: "put", k: "k1", v: "v1", wantOK: true},
				{kind: "put", k: "k2", v: "v2", wantOK: true},
				{kind: "put", k: "k3", v: "v3", wantOK: true},
				{kind: "put", k: "k4", v: "v4", wantOK: true},
				{kind: "put", k: "k5", v: "v5", wantOK: true},
				{kind: "put", k: "k3", v: "v6", wantOK: true},
			},
			want: map[string]string{"k1": "v1", "k2": "v2", "k3": "v6", "k4": "v4", "k5": "v5"},
		},
		{
			name: "an overwrite chain rotates three bindings",
			steps: []biOp{
				{kind: "put", k: "a", v: "x", wantOK: true},
				{kind: "put", k: "b", v: "y", wantOK: true},
				{kind: "put", k: "c", v: "z", wantOK: true},
				{kind: "force", k: "a", v: "y"},
				{kind: "force", k: "b", v: "z"},
				{kind: "force", k: "c", v: "x"},
			},
			want: map[string]string{"a": "y", "b": "z", "c": "x"},
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			b := bidimap.New[string, string]()
			ref := newRefBiMap[string, string]()
			for i, step := range sc.steps {
				switch step.kind {
				case "put":
					require.Equal(t, step.wantOK, b.Put(step.k, step.v), "step %d Put result", i)
					require.Equal(t, step.wantOK, ref.put(step.k, step.v), "step %d model Put result", i)
				case "force":
					b.ForcePut(step.k, step.v)
					ref.forcePut(step.k, step.v)
				case "remove":
					gotV, gotOK := b.Remove(step.k)
					require.Equal(t, step.wantOK, gotOK, "step %d Remove result", i)
					wantV, wantOK := ref.remove(step.k)
					require.Equal(t, wantOK, gotOK, "step %d model Remove result", i)
					require.Equal(t, wantV, gotV, "step %d Remove value", i)
					if step.wantOK {
						require.Equal(t, step.v, gotV, "step %d removed value", i)
					}
				case "clear":
					b.Clear()
					ref.clear()
				default:
					t.Fatalf("step %d: unknown kind %q", i, step.kind)
				}
				requireMatchesModel(t, b, ref)
				requireInvariants(t, b)
			}
			got := map[string]string{}
			for k, v := range b.Entries() {
				got[k] = v
			}
			require.Equal(t, sc.want, got, "final state")
		})
	}
}

// TestDifferentialAgainstReferenceModel replays random Put, ForcePut, Remove
// and Clear sequences against the two-map reference model and compares every
// step.
func TestDifferentialAgainstReferenceModel(t *testing.T) {
	const (
		rounds      = 120
		opsPerRound = 120
		keySpan     = 8
		valSpan     = 8
	)
	rng := rand.New(rand.NewPCG(1, 2))
	for round := range rounds {
		var zero bidimap.BiMap[int, int]
		builders := []func() *bidimap.BiMap[int, int]{
			bidimap.New[int, int],
			func() *bidimap.BiMap[int, int] { return bidimap.NewWithCapacity[int, int](4) },
			func() *bidimap.BiMap[int, int] { return &zero },
		}
		b := builders[round%len(builders)]()
		ref := newRefBiMap[int, int]()
		for i := range opsPerRound {
			key := rng.IntN(keySpan)
			val := rng.IntN(valSpan)
			switch rng.IntN(10) {
			case 0, 1, 2, 3, 4:
				require.Equal(t, ref.put(key, val), b.Put(key, val), "round %d op %d Put", round, i)
			case 5, 6, 7:
				b.ForcePut(key, val)
				ref.forcePut(key, val)
			case 8:
				gotV, gotOK := b.Remove(key)
				wantV, wantOK := ref.remove(key)
				require.Equal(t, wantOK, gotOK, "round %d op %d Remove result", round, i)
				require.Equal(t, wantV, gotV, "round %d op %d Remove value", round, i)
			case 9:
				b.Clear()
				ref.clear()
			}
			requireMatchesModel(t, b, ref)
		}
		requireInvariants(t, b)
	}
}

// TestInverseIsALiveView checks that the origin and its Inverse share one
// storage: writes on either side are visible on the other one.
func TestInverseIsALiveView(t *testing.T) {
	t.Run("writes through the origin are visible through Inverse", func(t *testing.T) {
		b := bidimap.New[string, int]()
		inv := b.Inverse()
		require.True(t, b.Put("one", 1))
		got, ok := inv.Get(1)
		require.True(t, ok)
		require.Equal(t, "one", got)
		require.Equal(t, 1, inv.Len())
	})

	t.Run("writes through Inverse are visible through the origin", func(t *testing.T) {
		b := bidimap.New[string, int]()
		inv := b.Inverse()
		require.True(t, inv.Put(1, "one"))
		got, ok := b.Get("one")
		require.True(t, ok)
		require.Equal(t, 1, got)
		require.Equal(t, 1, b.Len())
	})

	t.Run("Put through Inverse rejects a taken key on the origin", func(t *testing.T) {
		b := bidimap.New[string, int]()
		b.Put("one", 1)
		inv := b.Inverse()
		require.False(t, inv.Put(2, "one"), "the forward key is already bound")
		require.Equal(t, 1, b.Len())
		requireInvariants(t, b)
	})

	t.Run("ForcePut through Inverse steals forward bindings", func(t *testing.T) {
		b := bidimap.New[string, int]()
		b.Put("one", 1)
		b.Put("two", 2)
		b.Inverse().ForcePut(1, "two") // "two" now maps to 1, "one" is dropped
		got, ok := b.Get("two")
		require.True(t, ok)
		require.Equal(t, 1, got)
		_, ok = b.Get("one")
		require.False(t, ok, "the stolen binding must drop the old key")
		require.Equal(t, 1, b.Len())
		requireInvariants(t, b)
	})

	t.Run("Remove through Inverse unbinds the forward key", func(t *testing.T) {
		b := bidimap.New[string, int]()
		b.Put("one", 1)
		b.Put("two", 2)
		got, ok := b.Inverse().Remove(1)
		require.True(t, ok)
		require.Equal(t, "one", got)
		_, ok = b.Get("one")
		require.False(t, ok)
		require.Equal(t, 1, b.Len())
		requireInvariants(t, b)
	})

	t.Run("Clear through Inverse empties the origin", func(t *testing.T) {
		b := bidimap.New[string, int]()
		b.Put("one", 1)
		b.Put("two", 2)
		b.Inverse().Clear()
		require.Zero(t, b.Len())
		requireInvariants(t, b)
	})

	t.Run("Inverse of Inverse is the origin", func(t *testing.T) {
		b := bidimap.New[string, int]()
		require.Same(t, b, b.Inverse().Inverse())
	})

	t.Run("both sides agree after mixed writes", func(t *testing.T) {
		b := bidimap.New[string, int]()
		inv := b.Inverse()
		require.True(t, b.Put("one", 1))
		require.True(t, inv.Put(2, "two"))
		require.True(t, inv.Put(3, "three"))
		require.True(t, b.Put("four", 4))
		require.False(t, b.Put("five", 2), "2 is bound to two")
		inv.ForcePut(2, "five") // origin key "five" now maps to 2; "two" is dropped
		require.False(t, b.Put("six", 2), "2 is bound to five")
		require.Equal(t, 4, b.Len())
		require.Equal(t, 4, inv.Len())
		requireMatchesModel(t, b, &refBiMap[string, int]{
			fwd: map[string]int{"one": 1, "five": 2, "three": 3, "four": 4},
			rev: map[int]string{1: "one", 2: "five", 3: "three", 4: "four"},
		})
		requireInvariants(t, b)
	})
}

// TestLookupTables exercises Get and GetKeyOf over a fixed populated bimap.
func TestLookupTables(t *testing.T) {
	b := bidimap.New[string, int]()
	b.Put("one", 1)
	b.Put("two", 2)
	b.Put("three", 3)

	t.Run("Get", func(t *testing.T) {
		cases := []struct {
			name  string
			key   string
			wantV int
			wantK bool
		}{
			{name: "first key", key: "one", wantV: 1, wantK: true},
			{name: "middle key", key: "two", wantV: 2, wantK: true},
			{name: "last key", key: "three", wantV: 3, wantK: true},
			{name: "missing key", key: "four", wantV: 0, wantK: false},
			{name: "empty key", key: "", wantV: 0, wantK: false},
			{name: "value used as key", key: "1", wantV: 0, wantK: false},
			{name: "key with different case", key: "One", wantV: 0, wantK: false},
			{name: "whitespace key", key: "one ", wantV: 0, wantK: false},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				gotV, gotK := b.Get(tc.key)
				require.Equal(t, tc.wantK, gotK)
				require.Equal(t, tc.wantV, gotV)
			})
		}
	})

	t.Run("GetKeyOf", func(t *testing.T) {
		cases := []struct {
			name    string
			value   int
			wantKey string
			wantOK  bool
		}{
			{name: "first value", value: 1, wantKey: "one", wantOK: true},
			{name: "middle value", value: 2, wantKey: "two", wantOK: true},
			{name: "last value", value: 3, wantKey: "three", wantOK: true},
			{name: "missing value", value: 4, wantKey: "", wantOK: false},
			{name: "zero value", value: 0, wantKey: "", wantOK: false},
			{name: "negative value", value: -1, wantKey: "", wantOK: false},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				gotKey, gotOK := b.GetKeyOf(tc.value)
				require.Equal(t, tc.wantOK, gotOK)
				require.Equal(t, tc.wantKey, gotKey)
			})
		}
	})
}

// TestZeroValueIsUsable asserts the zero BiMap is an empty, ready-to-use map.
func TestZeroValueIsUsable(t *testing.T) {
	var b bidimap.BiMap[string, int]
	require.Zero(t, b.Len())
	require.True(t, b.Put("one", 1))
	got, ok := b.Get("one")
	require.True(t, ok)
	require.Equal(t, 1, got)

	inv := b.Inverse()
	key, ok := inv.Get(1)
	require.True(t, ok)
	require.Equal(t, "one", key)

	var empty bidimap.BiMap[int, int]
	empty.Clear() // must not panic
	require.Zero(t, empty.Len())
	requireInvariants(t, &empty)
}

// TestNewWithCapacity checks the capacity constructor and its panic contract.
func TestNewWithCapacity(t *testing.T) {
	t.Run("pre-sized map holds capacity pairs", func(t *testing.T) {
		b := bidimap.NewWithCapacity[int, int](128)
		for i := range 128 {
			require.True(t, b.Put(i, i))
		}
		require.Equal(t, 128, b.Len())
		requireInvariants(t, b)
	})

	t.Run("zero capacity is allowed", func(t *testing.T) {
		b := bidimap.NewWithCapacity[int, int](0)
		require.Zero(t, b.Len())
		require.True(t, b.Put(1, 1))
	})

	t.Run("negative capacity panics", func(t *testing.T) {
		require.PanicsWithValue(t, "bidimap: negative capacity", func() {
			bidimap.NewWithCapacity[int, int](-1)
		})
	})
}

// TestKeysValuesEntries checks the bulk accessors and the iteration.
func TestKeysValuesEntries(t *testing.T) {
	t.Run("Keys and Values hold exactly the bound pairs", func(t *testing.T) {
		b := bidimap.New[string, int]()
		b.Put("one", 1)
		b.Put("two", 2)
		b.Put("three", 3)

		keys := b.Keys()
		slices.Sort(keys)
		require.Equal(t, []string{"one", "three", "two"}, keys)

		values := b.Values()
		slices.Sort(values)
		require.Equal(t, []int{1, 2, 3}, values)
	})

	t.Run("Keys and Values are snapshots", func(t *testing.T) {
		b := bidimap.New[string, int]()
		b.Put("one", 1)
		keys := b.Keys()
		values := b.Values()
		b.Put("two", 2)
		require.Equal(t, []string{"one"}, keys, "Keys must not observe later writes")
		require.Equal(t, []int{1}, values, "Values must not observe later writes")
	})

	t.Run("Entries yields every pair", func(t *testing.T) {
		b := bidimap.New[string, int]()
		b.Put("one", 1)
		b.Put("two", 2)
		b.Put("three", 3)

		got := map[string]int{}
		for k, v := range b.Entries() {
			got[k] = v
		}
		require.Equal(t, map[string]int{"one": 1, "two": 2, "three": 3}, got)
	})

	t.Run("Entries stops early when yield returns false", func(t *testing.T) {
		b := bidimap.New[int, int]()
		for i := range 16 {
			b.Put(i, i)
		}
		count := 0
		for range b.Entries() {
			count++
			if count == 3 {
				break
			}
		}
		require.Equal(t, 3, count, "breaking out of Entries must stop the iteration")
	})

	t.Run("empty map iterates nothing", func(t *testing.T) {
		b := bidimap.New[string, int]()
		require.Empty(t, b.Keys())
		require.Empty(t, b.Values())
		for range b.Entries() {
			t.Fatal("Entries must not yield on an empty bimap")
		}
	})
}

// TestLenAndClear checks the size and reset operations across states.
func TestLenAndClear(t *testing.T) {
	b := bidimap.New[string, int]()
	require.Zero(t, b.Len())
	require.True(t, b.Put("one", 1))
	require.True(t, b.Put("two", 2))
	require.Equal(t, 2, b.Len())
	_, ok := b.Remove("one")
	require.True(t, ok)
	require.Equal(t, 1, b.Len())
	b.Clear()
	require.Zero(t, b.Len())
	requireInvariants(t, b)
}

// lockedBiMap serialises every access with one mutex, which is what makes the
// concurrency test race-clean by construction.
type lockedBiMap struct {
	mu    sync.Mutex
	inner *bidimap.BiMap[string, int]
}

// put binds key to value under the lock.
func (l *lockedBiMap) put(key string, value int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inner.Put(key, value)
}

// forcePut binds key to value under the lock, dropping conflicts.
func (l *lockedBiMap) forcePut(key string, value int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.inner.ForcePut(key, value)
}

// get looks key up under the lock.
func (l *lockedBiMap) get(key string) (int, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inner.Get(key)
}

// getKeyOf reverse-looks value up under the lock.
func (l *lockedBiMap) getKeyOf(value int) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inner.GetKeyOf(value)
}

// TestConcurrentAccess uses one mutex around every call, so the test is
// race-clean by construction; with -race it also proves the wrapper is needed.
func TestConcurrentAccess(t *testing.T) {
	const (
		goroutines = 8
		ops        = 200
		shared     = "shared"
		valueSpace = 1_000_000 // keeps shared values out of the per-key space
	)

	b := bidimap.NewWithCapacity[string, int](goroutines + 1)
	guarded := &lockedBiMap{inner: b}

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%02d", id)
			for i := range ops {
				value := id*ops + i
				if !guarded.put(key, value) {
					t.Errorf("goroutine %d: Put(%q, %d) must succeed", id, key, value)
					return
				}
				if got, ok := guarded.get(key); !ok || got != value {
					t.Errorf("goroutine %d: Get(%q) = %d, %v", id, key, got, ok)
					return
				}
				if i%8 == 0 {
					guarded.forcePut(shared, valueSpace+value)
				}
				// reading somebody else's key must never panic
				guarded.get(fmt.Sprintf("key-%02d", (id+1)%goroutines))
			}
		}(g)
	}
	wg.Wait()

	for g := range goroutines {
		key := fmt.Sprintf("key-%02d", g)
		got, ok := guarded.get(key)
		require.True(t, ok, "per-goroutine key must survive")
		require.Equal(t, g*ops+ops-1, got, "per-goroutine key must hold its last value")
		owner, ok := guarded.getKeyOf(got)
		require.True(t, ok)
		require.Equal(t, key, owner)
	}
	sharedValue, ok := guarded.get(shared)
	require.True(t, ok, "the shared key must stay bound")
	require.GreaterOrEqual(t, sharedValue, valueSpace)
	require.Equal(t, goroutines+1, b.Len())
	requireInvariants(t, b)
}

// TestReadsDoNotAllocate guards the read hot paths against unexpected
// allocations: the map is warm and never grows.
func TestReadsDoNotAllocate(t *testing.T) {
	b := bidimap.NewWithCapacity[int, int](64)
	for i := range 64 {
		b.Put(i, i)
	}
	inv := b.Inverse()

	var (
		sinkV  int
		sinkK  int
		sinkOK bool
		sinkN  int
		sinkI  *bidimap.BiMap[int, int]
	)

	allocs := testing.AllocsPerRun(1000, func() { sinkV, sinkOK = b.Get(32) })
	require.Zero(t, allocs, "Get must not allocate")

	allocs = testing.AllocsPerRun(1000, func() { sinkK, sinkOK = b.GetKeyOf(32) })
	require.Zero(t, allocs, "GetKeyOf must not allocate")

	allocs = testing.AllocsPerRun(1000, func() { sinkN = b.Len() })
	require.Zero(t, allocs, "Len must not allocate")

	allocs = testing.AllocsPerRun(1000, func() { sinkI = b.Inverse() })
	require.Zero(t, allocs, "Inverse must not allocate")

	allocs = testing.AllocsPerRun(1000, func() { sinkV, sinkOK = inv.Get(32) })
	require.Zero(t, allocs, "Inverse.Get must not allocate")

	require.True(t, sinkOK)
	require.Equal(t, 32, sinkV)
	require.Equal(t, 32, sinkK)
	require.Equal(t, 64, sinkN)
	require.Same(t, inv, sinkI)
}
