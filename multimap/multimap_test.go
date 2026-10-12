package multimap_test

import (
	"iter"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/neko233-com/struct233-go/multimap"
	"github.com/stretchr/testify/require"
)

// reader is the read side shared by both multimaps, so one invariant checker
// covers both flavours.
type reader[K comparable, V any] interface {
	Get(k K) []V
	ContainsKey(k K) bool
	ContainsEntry(k K, v V) bool
	Size() int
	KeyCount() int
	IsEmpty() bool
	Keys() []K
	Values() []V
	Entries() []multimap.Entry[K, V]
	AsMap() map[K][]V
	All() iter.Seq2[K, V]
}

// requireInvariants re-checks the size, order and uniqueness invariants through
// the public API alone. wantUnique says whether a value may appear only once per
// key.
func requireInvariants[K comparable, V comparable](t *testing.T, m reader[K, V], wantUnique bool) {
	t.Helper()
	asMap := m.AsMap()
	keys := m.Keys()
	values := m.Values()
	entries := m.Entries()

	total := 0
	for k, vs := range asMap {
		require.NotEmptyf(t, vs, "AsMap key %v maps to an empty value list", k)
		total += len(vs)
		require.Equalf(t, vs, m.Get(k), "Get(%v) must match AsMap", k)
		require.Truef(t, m.ContainsKey(k), "ContainsKey(%v) must hold", k)
		if wantUnique {
			seen := make(map[V]struct{}, len(vs))
			for _, v := range vs {
				_, dup := seen[v]
				require.Falsef(t, dup, "value %v is duplicated under key %v", v, k)
				seen[v] = struct{}{}
			}
		}
	}
	require.Equal(t, total, m.Size(), "Size must be the sum of the value lists")
	require.Equal(t, len(asMap), m.KeyCount(), "KeyCount must count the keys holding values")
	require.Equal(t, len(asMap), len(keys), "Keys must list every value-carrying key once")
	require.Equal(t, m.Size(), len(values), "Values must hold every value")
	require.Equal(t, m.Size(), len(entries), "Entries must hold every pair")
	require.Equal(t, m.Size() == 0, m.IsEmpty(), "IsEmpty must agree with Size")

	require.Equal(t, keys, keysOf(entries), "Keys must follow the entry order")
	require.Equal(t, values, valuesOf(entries), "Values must follow the entry order")
	require.Equal(t, entries, entriesOf(m), "All must yield the entries in order")

	seenKeys := make(map[K]struct{}, len(keys))
	for _, k := range keys {
		_, dup := seenKeys[k]
		require.Falsef(t, dup, "key %v is listed twice", k)
		seenKeys[k] = struct{}{}
	}
	for _, e := range entries {
		require.Truef(t, m.ContainsEntry(e.Key, e.Value), "ContainsEntry(%v, %v)", e.Key, e.Value)
	}
}

// requireListInvariants re-checks the invariants of a ListMultimap.
func requireListInvariants[K comparable, V comparable](t *testing.T, m *multimap.ListMultimap[K, V]) {
	t.Helper()
	var r reader[K, V] = m
	requireInvariants(t, r, false)
}

// requireSetInvariants re-checks the invariants of a SetMultimap, including
// per-key uniqueness.
func requireSetInvariants[K comparable, V comparable](t *testing.T, m *multimap.SetMultimap[K, V]) {
	t.Helper()
	var r reader[K, V] = m
	requireInvariants(t, r, true)
}

// keysOf returns the distinct entry keys in first-appearance order.
func keysOf[K comparable, V any](entries []multimap.Entry[K, V]) []K {
	out := make([]K, 0, len(entries))
	for i, e := range entries {
		if i == 0 || out[len(out)-1] != e.Key {
			out = append(out, e.Key)
		}
	}
	return out
}

// valuesOf returns the entry values in order.
func valuesOf[K comparable, V any](entries []multimap.Entry[K, V]) []V {
	out := make([]V, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Value)
	}
	return out
}

// entriesOf materialises All into the same shape as Entries.
func entriesOf[K comparable, V any](m reader[K, V]) []multimap.Entry[K, V] {
	out := make([]multimap.Entry[K, V], 0, m.Size())
	for k, v := range m.All() {
		out = append(out, multimap.Entry[K, V]{Key: k, Value: v})
	}
	return out
}

// wantOrEmpty keeps table literals terse: an omitted expectation means "empty".
func wantOrEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// pairs builds entries from "key=value" strings.
func pairs(kv ...string) []multimap.Entry[string, string] {
	out := make([]multimap.Entry[string, string], 0, len(kv))
	for _, s := range kv {
		k, v, _ := strings.Cut(s, "=")
		out = append(out, multimap.Entry[string, string]{Key: k, Value: v})
	}
	return out
}

// allStrings renders All as "key=value" strings.
func allStrings(m *multimap.ListMultimap[string, string]) []string {
	out := make([]string, 0, m.Size())
	for k, v := range m.All() {
		out = append(out, k+"="+v)
	}
	return out
}

func TestListBuildCases(t *testing.T) {
	tests := []struct {
		name        string
		build       func(m *multimap.ListMultimap[string, string])
		wantSize    int
		wantKeys    []string
		wantValues  []string
		wantEntries []multimap.Entry[string, string]
		wantMap     map[string][]string
	}{
		{
			name:  "empty multimap",
			build: func(m *multimap.ListMultimap[string, string]) {},
		},
		{
			name:       "one value under one key",
			build:      func(m *multimap.ListMultimap[string, string]) { m.Put("a", "1") },
			wantSize:   1,
			wantKeys:   []string{"a"},
			wantValues: []string{"1"},
			wantEntries: []multimap.Entry[string, string]{
				{Key: "a", Value: "1"},
			},
			wantMap: map[string][]string{"a": {"1"}},
		},
		{
			name: "many values under one key keep value order",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.PutAll("a", "3", "1", "2")
			},
			wantSize:   3,
			wantKeys:   []string{"a"},
			wantValues: []string{"3", "1", "2"},
			wantMap:    map[string][]string{"a": {"3", "1", "2"}},
		},
		{
			name: "many keys keep insertion order",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.Put("b", "1")
				m.Put("a", "2")
				m.Put("c", "3")
			},
			wantSize:   3,
			wantKeys:   []string{"b", "a", "c"},
			wantValues: []string{"1", "2", "3"},
			wantMap:    map[string][]string{"b": {"1"}, "a": {"2"}, "c": {"3"}},
		},
		{
			name: "the same value under several keys",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.Put("a", "x")
				m.Put("b", "x")
			},
			wantSize:   2,
			wantKeys:   []string{"a", "b"},
			wantValues: []string{"x", "x"},
			wantMap:    map[string][]string{"a": {"x"}, "b": {"x"}},
		},
		{
			name: "duplicate values under one key are kept",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.PutAll("a", "x", "x", "x")
			},
			wantSize:   3,
			wantKeys:   []string{"a"},
			wantValues: []string{"x", "x", "x"},
			wantMap:    map[string][]string{"a": {"x", "x", "x"}},
		},
		{
			name: "interleaved puts",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.Put("a", "1")
				m.Put("b", "1")
				m.Put("a", "2")
				m.Put("b", "2")
				m.Put("a", "3")
			},
			wantSize:   5,
			wantKeys:   []string{"a", "b"},
			wantValues: []string{"1", "2", "3", "1", "2"},
			wantMap:    map[string][]string{"a": {"1", "2", "3"}, "b": {"1", "2"}},
		},
		{
			name: "putall with no values only reserves nothing",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.PutAll("a")
				m.PutAll("b", "1")
				m.PutAll("b")
			},
			wantSize:   1,
			wantKeys:   []string{"b"},
			wantValues: []string{"1"},
			wantMap:    map[string][]string{"b": {"1"}},
		},
		{
			name: "removing the last value drops the key and a later put appends it",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.Put("a", "1")
				m.Put("b", "2")
				m.Put("c", "3")
				m.Remove("a", "1")
				m.Put("a", "4")
				m.RemoveAll("b")
			},
			wantSize:   2,
			wantKeys:   []string{"c", "a"},
			wantValues: []string{"3", "4"},
			wantMap:    map[string][]string{"c": {"3"}, "a": {"4"}},
		},
		{
			name: "removing one duplicate keeps the others",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.PutAll("a", "x", "y", "x")
				m.Remove("a", "x")
			},
			wantSize:   2,
			wantKeys:   []string{"a"},
			wantValues: []string{"y", "x"},
			wantMap:    map[string][]string{"a": {"y", "x"}},
		},
		{
			name: "remove of an absent value and an absent key is a no-op",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.Put("a", "1")
				m.Remove("a", "2")
				m.Remove("zz", "1")
				m.RemoveAll("zz")
			},
			wantSize:   1,
			wantKeys:   []string{"a"},
			wantValues: []string{"1"},
			wantMap:    map[string][]string{"a": {"1"}},
		},
		{
			name: "replace keeps the key position",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.Put("a", "1")
				m.Put("b", "2")
				m.Put("c", "3")
				m.Replace("a", "9")
			},
			wantSize:   3,
			wantKeys:   []string{"a", "b", "c"},
			wantValues: []string{"9", "2", "3"},
			wantMap:    map[string][]string{"a": {"9"}, "b": {"2"}, "c": {"3"}},
		},
		{
			name: "replace of an absent key appends it",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.Put("a", "1")
				m.Replace("z", "7", "8")
			},
			wantSize:   3,
			wantKeys:   []string{"a", "z"},
			wantValues: []string{"1", "7", "8"},
			wantMap:    map[string][]string{"a": {"1"}, "z": {"7", "8"}},
		},
		{
			name: "replace with no values drops the key",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.Put("a", "1")
				m.Put("b", "2")
				m.Replace("a")
				m.Replace("zz")
			},
			wantSize:   1,
			wantKeys:   []string{"b"},
			wantValues: []string{"2"},
			wantMap:    map[string][]string{"b": {"2"}},
		},
		{
			name: "clear empties the multimap",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.PutAll("a", "1", "2")
				m.Put("b", "3")
				m.Clear()
			},
		},
		{
			name: "clear then rebuild keeps insertion order",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.Put("a", "1")
				m.Put("b", "2")
				m.Clear()
				m.Put("b", "9")
			},
			wantSize:   1,
			wantKeys:   []string{"b"},
			wantValues: []string{"9"},
			wantMap:    map[string][]string{"b": {"9"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewList[string, string]()
			tt.build(m)

			require.Equal(t, tt.wantSize, m.Size(), "Size")
			require.Equal(t, len(tt.wantKeys), m.KeyCount(), "KeyCount")
			require.Equal(t, tt.wantSize == 0, m.IsEmpty(), "IsEmpty")
			// the expected entries follow from the key order and the value lists
			wantEntries := tt.wantEntries
			if wantEntries == nil {
				wantEntries = make([]multimap.Entry[string, string], 0, tt.wantSize)
				for _, k := range tt.wantKeys {
					for _, v := range tt.wantMap[k] {
						wantEntries = append(wantEntries, multimap.Entry[string, string]{Key: k, Value: v})
					}
				}
			}
			require.Equal(t, wantOrEmpty(tt.wantKeys), m.Keys(), "Keys")
			require.Equal(t, wantOrEmpty(tt.wantValues), m.Values(), "Values")
			require.Equal(t, wantEntries, m.Entries(), "Entries")
			if tt.wantMap == nil {
				require.Empty(t, m.AsMap(), "AsMap")
			} else {
				require.Equal(t, tt.wantMap, m.AsMap(), "AsMap")
			}
			for _, k := range tt.wantKeys {
				require.Equal(t, tt.wantMap[k], m.Get(k), "Get(%q)", k)
				require.True(t, m.ContainsKey(k), "ContainsKey(%q)", k)
				for _, v := range tt.wantMap[k] {
					require.True(t, m.ContainsEntry(k, v), "ContainsEntry(%q, %q)", k, v)
				}
			}
			requireListInvariants(t, m)
		})
	}
}

func TestListGet(t *testing.T) {
	tests := []struct {
		name  string
		build func(m *multimap.ListMultimap[string, int])
		key   string
		want  []int
	}{
		{
			name:  "missing key on an empty multimap",
			build: func(m *multimap.ListMultimap[string, int]) {},
			key:   "a",
		},
		{
			name:  "missing key on a filled multimap",
			build: func(m *multimap.ListMultimap[string, int]) { m.Put("a", 1) },
			key:   "zz",
		},
		{
			name:  "one value",
			build: func(m *multimap.ListMultimap[string, int]) { m.Put("a", 1) },
			key:   "a",
			want:  []int{1},
		},
		{
			name:  "many values keep insertion order",
			build: func(m *multimap.ListMultimap[string, int]) { m.PutAll("a", 3, 1, 2) },
			key:   "a",
			want:  []int{3, 1, 2},
		},
		{
			name: "key emptied by remove",
			build: func(m *multimap.ListMultimap[string, int]) {
				m.Put("a", 1)
				m.Remove("a", 1)
			},
			key: "a",
		},
		{
			name: "key emptied by replace",
			build: func(m *multimap.ListMultimap[string, int]) {
				m.Put("a", 1)
				m.Replace("a")
			},
			key: "a",
		},
		{
			name: "duplicates are all returned",
			build: func(m *multimap.ListMultimap[string, int]) {
				m.PutAll("a", 7, 7, 7)
			},
			key:  "a",
			want: []int{7, 7, 7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewList[string, int]()
			tt.build(m)
			require.Equal(t, tt.want, m.Get(tt.key))
			requireListInvariants(t, m)
		})
	}
}

func TestListRemoveResults(t *testing.T) {
	tests := []struct {
		name     string
		build    func(m *multimap.ListMultimap[string, int])
		key      string
		value    int
		want     bool
		wantKeys []string
		wantSize int
	}{
		{
			name:  "absent key",
			build: func(m *multimap.ListMultimap[string, int]) {},
			key:   "a", value: 1,
			want: false,
		},
		{
			name:  "absent value under an existing key",
			build: func(m *multimap.ListMultimap[string, int]) { m.Put("a", 1) },
			key:   "a", value: 2,
			want:     false,
			wantKeys: []string{"a"},
			wantSize: 1,
		},
		{
			name:  "present value",
			build: func(m *multimap.ListMultimap[string, int]) { m.PutAll("a", 1, 2) },
			key:   "a", value: 1,
			want:     true,
			wantKeys: []string{"a"},
			wantSize: 1,
		},
		{
			name: "last value drops the key",
			build: func(m *multimap.ListMultimap[string, int]) {
				m.Put("a", 1)
				m.Put("b", 2)
			},
			key: "a", value: 1,
			want:     true,
			wantKeys: []string{"b"},
			wantSize: 1,
		},
		{
			name: "only the first duplicate goes",
			build: func(m *multimap.ListMultimap[string, int]) {
				m.PutAll("a", 5, 6, 5)
			},
			key: "a", value: 5,
			want:     true,
			wantKeys: []string{"a"},
			wantSize: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewList[string, int]()
			tt.build(m)
			require.Equal(t, tt.want, m.Remove(tt.key, tt.value))
			require.Equal(t, wantOrEmpty(tt.wantKeys), m.Keys())
			require.Equal(t, tt.wantSize, m.Size())
			requireListInvariants(t, m)
		})
	}
}

func TestListRemoveAllAndReplaceReturnDetachedValues(t *testing.T) {
	tests := []struct {
		name      string
		build     func(m *multimap.ListMultimap[string, int])
		removeAll string
		wantAll   []int
		replace   string
		with      []int
		wantOld   []int
		wantKeys  []string
	}{
		{
			name:      "removeall on a missing key",
			build:     func(m *multimap.ListMultimap[string, int]) {},
			removeAll: "a",
		},
		{
			name:      "removeall returns every value of the key",
			build:     func(m *multimap.ListMultimap[string, int]) { m.PutAll("a", 1, 1, 2) },
			removeAll: "a",
			wantAll:   []int{1, 1, 2},
		},
		{
			name: "replace returns the previous values",
			build: func(m *multimap.ListMultimap[string, int]) {
				m.PutAll("a", 1, 2)
				m.Put("b", 3)
			},
			replace:  "a",
			with:     []int{4},
			wantOld:  []int{1, 2},
			wantKeys: []string{"a", "b"},
		},
		{
			name: "replace of an absent key returns nothing",
			build: func(m *multimap.ListMultimap[string, int]) {
				m.Put("a", 1)
			},
			replace:  "z",
			with:     []int{9},
			wantKeys: []string{"a", "z"},
		},
		{
			name: "replace of an absent key without values returns nothing",
			build: func(m *multimap.ListMultimap[string, int]) {
				m.Put("a", 1)
			},
			replace:  "z",
			wantKeys: []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewList[string, int]()
			tt.build(m)

			got := m.RemoveAll(tt.removeAll)
			require.Equal(t, tt.wantAll, got)
			require.Nil(t, m.Get(tt.removeAll), "the key must be gone after RemoveAll")

			got = m.Replace(tt.replace, tt.with...)
			require.Equal(t, tt.wantOld, got)
			require.Equal(t, wantOrEmpty(tt.wantKeys), m.Keys())

			// the returned slices are detached: writing to them must not be visible
			if len(got) > 0 {
				got[0] = -1
			}
			require.NotEqual(t, -1, m.Get(tt.replace))
			requireListInvariants(t, m)
		})
	}
}

func TestListContains(t *testing.T) {
	tests := []struct {
		name      string
		build     func(m *multimap.ListMultimap[string, int])
		key       string
		value     int
		wantKey   bool
		wantEntry bool
	}{
		{
			name:  "empty multimap",
			build: func(m *multimap.ListMultimap[string, int]) {},
			key:   "a", value: 1,
		},
		{
			name:  "key with values",
			build: func(m *multimap.ListMultimap[string, int]) { m.PutAll("a", 1, 2) },
			key:   "a", value: 1,
			wantKey: true, wantEntry: true,
		},
		{
			name:  "value of another key",
			build: func(m *multimap.ListMultimap[string, int]) { m.Put("a", 1) },
			key:   "a", value: 2,
			wantKey: true, wantEntry: false,
		},
		{
			name:  "missing key",
			build: func(m *multimap.ListMultimap[string, int]) { m.Put("a", 1) },
			key:   "zz", value: 1,
			wantKey: false, wantEntry: false,
		},
		{
			name: "key emptied by remove",
			build: func(m *multimap.ListMultimap[string, int]) {
				m.Put("a", 1)
				m.Remove("a", 1)
			},
			key: "a", value: 1,
		},
		{
			name:  "the same value under several keys",
			build: func(m *multimap.ListMultimap[string, int]) { m.Put("a", 1); m.Put("b", 1) },
			key:   "b", value: 1,
			wantKey: true, wantEntry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewList[string, int]()
			tt.build(m)
			require.Equal(t, tt.wantKey, m.ContainsKey(tt.key))
			require.Equal(t, tt.wantEntry, m.ContainsEntry(tt.key, tt.value))
		})
	}
}

func TestListKeysValuesEntriesAsMapAreCopies(t *testing.T) {
	m := multimap.NewList[string, int]()
	m.PutAll("a", 1, 2)
	m.Put("b", 3)

	got := m.Get("a")
	got[0] = 99
	require.Equal(t, []int{1, 2}, m.Get("a"), "Get must hand out a copy")

	keys := m.Keys()
	keys[0] = "zzz"
	require.Equal(t, []string{"a", "b"}, m.Keys(), "Keys must hand out a copy")

	values := m.Values()
	values[0] = 99
	require.Equal(t, []int{1, 2, 3}, m.Values(), "Values must hand out a copy")

	entries := m.Entries()
	entries[0].Value = 99
	require.Equal(t, 1, m.Entries()[0].Value, "Entries must hand out a copy")

	asMap := m.AsMap()
	asMap["a"][0] = 99
	delete(asMap, "b")
	require.Equal(t, []int{1, 2}, m.Get("a"), "AsMap must hold copied value lists")
	require.True(t, m.ContainsKey("b"))

	requireListInvariants(t, m)
}

func TestListInvert(t *testing.T) {
	tests := []struct {
		name       string
		build      func(m *multimap.ListMultimap[string, string])
		wantKeys   []string
		wantValues []string
		wantMap    map[string][]string
	}{
		{
			name:  "empty multimap",
			build: func(m *multimap.ListMultimap[string, string]) {},
		},
		{
			name: "one pair",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.Put("k", "v")
			},
			wantKeys:   []string{"v"},
			wantValues: []string{"k"},
			wantMap:    map[string][]string{"v": {"k"}},
		},
		{
			name: "one key with many values",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.PutAll("k", "v1", "v2")
			},
			wantKeys:   []string{"v1", "v2"},
			wantValues: []string{"k", "k"},
			wantMap:    map[string][]string{"v1": {"k"}, "v2": {"k"}},
		},
		{
			name: "the same value under several keys",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.Put("a", "x")
				m.Put("b", "x")
			},
			wantKeys:   []string{"x"},
			wantValues: []string{"a", "b"},
			wantMap:    map[string][]string{"x": {"a", "b"}},
		},
		{
			name: "duplicate pairs stay duplicated",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.PutAll("a", "x", "x")
			},
			wantKeys:   []string{"x"},
			wantValues: []string{"a", "a"},
			wantMap:    map[string][]string{"x": {"a", "a"}},
		},
		{
			name: "inverted key order follows first appearance",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.PutAll("k1", "v2", "v1")
				m.PutAll("k2", "v1")
			},
			wantKeys:   []string{"v2", "v1"},
			wantValues: []string{"k1", "k1", "k2"},
			wantMap:    map[string][]string{"v2": {"k1"}, "v1": {"k1", "k2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewList[string, string]()
			tt.build(m)

			inv := m.Invert()
			// inverting preserves the number of pairs but not the number of keys
			require.Equal(t, m.Size(), inv.Size())
			require.Equal(t, wantOrEmpty(tt.wantKeys), inv.Keys())
			require.Equal(t, wantOrEmpty(tt.wantValues), inv.Values())
			if tt.wantMap == nil {
				require.Empty(t, inv.AsMap())
			} else {
				require.Equal(t, tt.wantMap, inv.AsMap())
			}
			requireListInvariants(t, inv)

			// inverting twice gives the original pairs back
			back := inv.Invert()
			require.Equal(t, m.Entries(), back.Entries())
		})
	}
}

func TestListAll(t *testing.T) {
	tests := []struct {
		name  string
		build func(m *multimap.ListMultimap[string, string])
		want  []string
	}{
		{name: "empty multimap", build: func(m *multimap.ListMultimap[string, string]) {}},
		{
			name: "key order then value order",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.PutAll("a", "1", "2")
				m.Put("b", "3")
				m.PutAll("a", "4")
			},
			want: []string{"a=1", "a=2", "a=4", "b=3"},
		},
		{
			name: "duplicates are yielded as duplicates",
			build: func(m *multimap.ListMultimap[string, string]) {
				m.PutAll("a", "x", "x")
			},
			want: []string{"a=x", "a=x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewList[string, string]()
			tt.build(m)
			require.Equal(t, wantOrEmpty(tt.want), allStrings(m))

			// breaking out of the sequence stops the iteration
			count := 0
			for k, v := range m.All() {
				count++
				require.Equal(t, tt.want[0], k+"="+v)
				break
			}
			if len(tt.want) == 0 {
				require.Zero(t, count)
			}
		})
	}
}

func TestListClear(t *testing.T) {
	m := multimap.NewListWithCapacity[string, int](4, "reserved")
	m.PutAll("a", 1, 2)
	m.Put("b", 3)

	m.Clear()
	require.Zero(t, m.Size())
	require.Zero(t, m.KeyCount())
	require.True(t, m.IsEmpty())
	require.Empty(t, m.Keys())
	require.Empty(t, m.Values())
	require.Empty(t, m.Entries())
	require.Empty(t, m.AsMap())
	require.False(t, m.ContainsKey("a"))
	require.False(t, m.ContainsEntry("a", 1))
	requireListInvariants(t, m)

	// clear also drops the reserved slots, so the reserved key now appends
	m.Put("reserved", 4)
	m.Put("a", 5)
	require.Equal(t, []string{"reserved", "a"}, m.Keys())
	requireListInvariants(t, m)

	// clearing twice and rebuilding keeps working
	m.Clear()
	m.Clear()
	m.Put("z", 6)
	require.Equal(t, []int{6}, m.Get("z"))
	requireListInvariants(t, m)
}

func TestListReservedKeys(t *testing.T) {
	m := multimap.NewListWithCapacity[string, int](4, "a", "b")

	// reserved slots carry no values and stay invisible
	require.Zero(t, m.Size())
	require.Zero(t, m.KeyCount())
	require.True(t, m.IsEmpty())
	require.Empty(t, m.Keys())
	require.Nil(t, m.Get("a"))
	require.False(t, m.ContainsKey("a"))
	require.False(t, m.ContainsEntry("a", 1))
	requireListInvariants(t, m)

	m.Put("c", 1)
	require.Equal(t, []string{"c"}, m.Keys())

	// activating a reserved key keeps its reserved position in the order
	m.Put("a", 2)
	require.Equal(t, []string{"a", "c"}, m.Keys())
	require.Equal(t, []int{2}, m.Get("a"))
	require.Equal(t, 2, m.KeyCount())
	requireListInvariants(t, m)

	// b keeps the second reserved slot, which sits before the appended c
	m.Put("b", 3)
	require.Equal(t, []string{"a", "b", "c"}, m.Keys())
	require.Equal(t, []int{2, 3, 1}, m.Values())

	// dropping the last value of an activated key releases its slot for good
	m.RemoveAll("a")
	require.Equal(t, []string{"b", "c"}, m.Keys())
	m.Put("a", 4)
	require.Equal(t, []string{"b", "c", "a"}, m.Keys())
	requireListInvariants(t, m)

	// removeall on a reserved key leaves the reservation alone
	res := multimap.NewListWithCapacity[string, int](1, "r")
	require.Nil(t, res.RemoveAll("r"))
	require.False(t, res.Remove("r", 1))
	require.Nil(t, res.RemoveAll("r"))
	res.Put("q", 1)
	res.Put("r", 2)
	require.Equal(t, []string{"r", "q"}, res.Keys(), "the reserved key keeps its slot")

	// repeated reserved keys keep only their first slot
	dup := multimap.NewListWithCapacity[string, int](2, "x", "x")
	dup.Put("y", 1)
	dup.Put("x", 2)
	require.Equal(t, []string{"x", "y"}, dup.Keys())
	requireListInvariants(t, dup)

	// replace activates a reserved key in place too
	rep := multimap.NewListWithCapacity[string, int](1, "p")
	rep.Put("o", 1)
	require.Empty(t, rep.Replace("p"))
	require.Equal(t, []string{"o"}, rep.Keys())
	rep.Put("p", 2)
	require.Equal(t, []string{"p", "o"}, rep.Keys())
}

func TestListZeroValueIsUsable(t *testing.T) {
	var m multimap.ListMultimap[string, int]

	require.Zero(t, m.Size())
	require.Zero(t, m.KeyCount())
	require.True(t, m.IsEmpty())
	require.Empty(t, m.Keys())
	require.Empty(t, m.Values())
	require.Empty(t, m.Entries())
	require.Empty(t, m.AsMap())
	require.Nil(t, m.Get("a"))
	require.False(t, m.ContainsKey("a"))
	require.False(t, m.ContainsEntry("a", 1))
	require.False(t, m.Remove("a", 1))
	require.Nil(t, m.RemoveAll("a"))
	require.Nil(t, m.Replace("a"))
	requireListInvariants(t, &m)

	m.Put("a", 1)
	m.PutAll("a", 2)
	require.Equal(t, []int{1, 2}, m.Get("a"))
	m.Clear()
	require.True(t, m.IsEmpty())
	requireListInvariants(t, &m)
}

func TestListCapacityPanics(t *testing.T) {
	t.Run("negative capacity", func(t *testing.T) {
		require.PanicsWithValue(t, "multimap: negative capacity", func() {
			multimap.NewListWithCapacity[string, int](-1)
		})
	})
	t.Run("zero capacity is allowed", func(t *testing.T) {
		m := multimap.NewListWithCapacity[string, int](0)
		require.True(t, m.IsEmpty())
		m.Put("a", 1)
		require.Equal(t, []int{1}, m.Get("a"))
	})
	t.Run("reserved keys with zero capacity", func(t *testing.T) {
		m := multimap.NewListWithCapacity[string, int](0, "a")
		require.True(t, m.IsEmpty())
		m.Put("a", 1)
		require.Equal(t, []string{"a"}, m.Keys())
	})
}

func TestSetBuildCases(t *testing.T) {
	tests := []struct {
		name        string
		build       func(m *multimap.SetMultimap[string, string])
		wantSize    int
		wantKeys    []string
		wantValues  []string
		wantEntries []multimap.Entry[string, string]
		wantMap     map[string][]string
	}{
		{
			name:  "empty multimap",
			build: func(m *multimap.SetMultimap[string, string]) {},
		},
		{
			name:       "one value under one key",
			build:      func(m *multimap.SetMultimap[string, string]) { m.Put("a", "1") },
			wantSize:   1,
			wantKeys:   []string{"a"},
			wantValues: []string{"1"},
			wantMap:    map[string][]string{"a": {"1"}},
		},
		{
			name: "duplicate puts are ignored",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.Put("a", "1")
				m.Put("a", "1")
				m.Put("a", "1")
			},
			wantSize:   1,
			wantKeys:   []string{"a"},
			wantValues: []string{"1"},
			wantMap:    map[string][]string{"a": {"1"}},
		},
		{
			name: "many values under one key keep first-seen order",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.PutAll("a", "3", "1", "3", "2")
			},
			wantSize:   3,
			wantKeys:   []string{"a"},
			wantValues: []string{"3", "1", "2"},
			wantMap:    map[string][]string{"a": {"3", "1", "2"}},
		},
		{
			name: "many keys keep insertion order",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.Put("b", "1")
				m.Put("a", "2")
				m.Put("c", "3")
			},
			wantSize:   3,
			wantKeys:   []string{"b", "a", "c"},
			wantValues: []string{"1", "2", "3"},
			wantMap:    map[string][]string{"b": {"1"}, "a": {"2"}, "c": {"3"}},
		},
		{
			name: "the same value under several keys",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.Put("a", "x")
				m.Put("b", "x")
			},
			wantSize:   2,
			wantKeys:   []string{"a", "b"},
			wantValues: []string{"x", "x"},
			wantMap:    map[string][]string{"a": {"x"}, "b": {"x"}},
		},
		{
			name: "removing the last value drops the key and a later put appends it",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.Put("a", "1")
				m.Put("b", "2")
				m.Put("c", "3")
				m.Remove("a", "1")
				m.Put("a", "4")
				m.RemoveAll("b")
			},
			wantSize:   2,
			wantKeys:   []string{"c", "a"},
			wantValues: []string{"3", "4"},
			wantMap:    map[string][]string{"c": {"3"}, "a": {"4"}},
		},
		{
			name: "remove of an absent value and an absent key is a no-op",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.Put("a", "1")
				m.Remove("a", "2")
				m.Remove("zz", "1")
				m.RemoveAll("zz")
			},
			wantSize:   1,
			wantKeys:   []string{"a"},
			wantValues: []string{"1"},
			wantMap:    map[string][]string{"a": {"1"}},
		},
		{
			name: "removing one of several values keeps the key",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.PutAll("a", "x", "y", "z")
				m.Remove("a", "y")
			},
			wantSize:   2,
			wantKeys:   []string{"a"},
			wantValues: []string{"x", "z"},
			wantMap:    map[string][]string{"a": {"x", "z"}},
		},
		{
			name: "replace deduplicates and keeps the key position",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.Put("a", "1")
				m.Put("b", "2")
				m.Put("c", "3")
				m.Replace("a", "9", "9", "8")
			},
			wantSize:    4,
			wantKeys:    []string{"a", "b", "c"},
			wantValues:  []string{"9", "8", "2", "3"},
			wantEntries: pairs("a=9", "a=8", "b=2", "c=3"),
			wantMap:     map[string][]string{"a": {"9", "8"}, "b": {"2"}, "c": {"3"}},
		},
		{
			name: "replace of an absent key appends it",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.Put("a", "1")
				m.Replace("z", "7", "7", "8")
			},
			wantSize:   3,
			wantKeys:   []string{"a", "z"},
			wantValues: []string{"1", "7", "8"},
			wantMap:    map[string][]string{"a": {"1"}, "z": {"7", "8"}},
		},
		{
			name: "replace with no values drops the key",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.Put("a", "1")
				m.Put("b", "2")
				m.Replace("a")
				m.Replace("zz")
			},
			wantSize:   1,
			wantKeys:   []string{"b"},
			wantValues: []string{"2"},
			wantMap:    map[string][]string{"b": {"2"}},
		},
		{
			name: "clear empties the multimap",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.PutAll("a", "1", "2")
				m.Put("b", "3")
				m.Clear()
			},
		},
		{
			name: "clear then rebuild keeps insertion order",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.Put("a", "1")
				m.Put("b", "2")
				m.Clear()
				m.Put("b", "9")
			},
			wantSize:   1,
			wantKeys:   []string{"b"},
			wantValues: []string{"9"},
			wantMap:    map[string][]string{"b": {"9"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewSet[string, string]()
			tt.build(m)

			require.Equal(t, tt.wantSize, m.Size(), "Size")
			require.Equal(t, len(tt.wantKeys), m.KeyCount(), "KeyCount")
			require.Equal(t, tt.wantSize == 0, m.IsEmpty(), "IsEmpty")
			// the expected entries follow from the key order and the value lists
			wantEntries := tt.wantEntries
			if wantEntries == nil {
				wantEntries = make([]multimap.Entry[string, string], 0, tt.wantSize)
				for _, k := range tt.wantKeys {
					for _, v := range tt.wantMap[k] {
						wantEntries = append(wantEntries, multimap.Entry[string, string]{Key: k, Value: v})
					}
				}
			}
			require.Equal(t, wantOrEmpty(tt.wantKeys), m.Keys(), "Keys")
			require.Equal(t, wantOrEmpty(tt.wantValues), m.Values(), "Values")
			require.Equal(t, wantEntries, m.Entries(), "Entries")
			if tt.wantMap == nil {
				require.Empty(t, m.AsMap(), "AsMap")
			} else {
				require.Equal(t, tt.wantMap, m.AsMap(), "AsMap")
			}
			for _, k := range tt.wantKeys {
				require.Equal(t, tt.wantMap[k], m.Get(k), "Get(%q)", k)
				require.True(t, m.ContainsKey(k), "ContainsKey(%q)", k)
				for _, v := range tt.wantMap[k] {
					require.True(t, m.ContainsEntry(k, v), "ContainsEntry(%q, %q)", k, v)
				}
			}
			requireSetInvariants(t, m)
		})
	}
}

func TestSetGet(t *testing.T) {
	tests := []struct {
		name  string
		build func(m *multimap.SetMultimap[string, int])
		key   string
		want  []int
	}{
		{
			name:  "missing key on an empty multimap",
			build: func(m *multimap.SetMultimap[string, int]) {},
			key:   "a",
		},
		{
			name:  "missing key on a filled multimap",
			build: func(m *multimap.SetMultimap[string, int]) { m.Put("a", 1) },
			key:   "zz",
		},
		{
			name:  "one value",
			build: func(m *multimap.SetMultimap[string, int]) { m.Put("a", 1) },
			key:   "a",
			want:  []int{1},
		},
		{
			name:  "many values keep first-seen order",
			build: func(m *multimap.SetMultimap[string, int]) { m.PutAll("a", 3, 1, 2) },
			key:   "a",
			want:  []int{3, 1, 2},
		},
		{
			name: "key emptied by remove",
			build: func(m *multimap.SetMultimap[string, int]) {
				m.Put("a", 1)
				m.Remove("a", 1)
			},
			key: "a",
		},
		{
			name: "key emptied by replace",
			build: func(m *multimap.SetMultimap[string, int]) {
				m.Put("a", 1)
				m.Replace("a")
			},
			key: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewSet[string, int]()
			tt.build(m)
			require.Equal(t, tt.want, m.Get(tt.key))
			requireSetInvariants(t, m)
		})
	}
}

func TestSetPutReportsWhetherTheValueIsNew(t *testing.T) {
	tests := []struct {
		name    string
		seed    []int
		value   int
		want    bool
		wantGet []int
	}{
		{name: "new value under a new key", value: 1, want: true, wantGet: []int{1}},
		{name: "duplicate under an existing key", seed: []int{1, 2}, value: 2, want: false, wantGet: []int{1, 2}},
		{name: "new value under an existing key", seed: []int{1}, value: 2, want: true, wantGet: []int{1, 2}},
		{name: "duplicate in a single-value key", seed: []int{1}, value: 1, want: false, wantGet: []int{1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewSet[string, int]()
			m.PutAll("a", tt.seed...)
			before := m.Size()

			require.Equal(t, tt.want, m.Put("a", tt.value))
			require.Equal(t, tt.wantGet, m.Get("a"))
			if tt.want {
				require.Equal(t, before+1, m.Size())
			} else {
				require.Equal(t, before, m.Size())
			}
			requireSetInvariants(t, m)
		})
	}
}

func TestSetPutAllReportsAddedCount(t *testing.T) {
	tests := []struct {
		name      string
		seed      []int
		values    []int
		wantAdded int
		wantGet   []int
	}{
		{name: "all new", values: []int{1, 2, 3}, wantAdded: 3, wantGet: []int{1, 2, 3}},
		{name: "all duplicates", seed: []int{1, 2}, values: []int{1, 2}, wantAdded: 0, wantGet: []int{1, 2}},
		{name: "mixed", seed: []int{1}, values: []int{1, 2, 2, 3}, wantAdded: 2, wantGet: []int{1, 2, 3}},
		{name: "no values", seed: []int{1}, wantAdded: 0, wantGet: []int{1}},
		{name: "duplicates inside the argument", values: []int{5, 5, 5}, wantAdded: 1, wantGet: []int{5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewSet[string, int]()
			m.PutAll("a", tt.seed...)
			before := m.Size()

			require.Equal(t, tt.wantAdded, m.PutAll("a", tt.values...))
			require.Equal(t, tt.wantGet, m.Get("a"))
			require.Equal(t, before+tt.wantAdded, m.Size())
			requireSetInvariants(t, m)
		})
	}
}

func TestSetRemoveResults(t *testing.T) {
	tests := []struct {
		name     string
		build    func(m *multimap.SetMultimap[string, int])
		key      string
		value    int
		want     bool
		wantKeys []string
		wantSize int
	}{
		{
			name:  "absent key",
			build: func(m *multimap.SetMultimap[string, int]) {},
			key:   "a", value: 1,
			want: false,
		},
		{
			name:  "absent value under an existing key",
			build: func(m *multimap.SetMultimap[string, int]) { m.Put("a", 1) },
			key:   "a", value: 2,
			want:     false,
			wantKeys: []string{"a"},
			wantSize: 1,
		},
		{
			name:  "present value",
			build: func(m *multimap.SetMultimap[string, int]) { m.PutAll("a", 1, 2) },
			key:   "a", value: 1,
			want:     true,
			wantKeys: []string{"a"},
			wantSize: 1,
		},
		{
			name: "last value drops the key",
			build: func(m *multimap.SetMultimap[string, int]) {
				m.Put("a", 1)
				m.Put("b", 2)
			},
			key: "a", value: 1,
			want:     true,
			wantKeys: []string{"b"},
			wantSize: 1,
		},
		{
			name:  "removing twice reports false the second time",
			build: func(m *multimap.SetMultimap[string, int]) { m.Put("a", 1) },
			key:   "a", value: 1,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewSet[string, int]()
			tt.build(m)

			require.Equal(t, tt.want, m.Remove(tt.key, tt.value), "Remove")
			require.Equal(t, wantOrEmpty(tt.wantKeys), m.Keys())
			require.Equal(t, tt.wantSize, m.Size())
			require.False(t, m.Remove(tt.key, tt.value), "Remove must not report the same value twice")
			requireSetInvariants(t, m)
		})
	}
}

func TestSetRemoveAllResults(t *testing.T) {
	tests := []struct {
		name     string
		build    func(m *multimap.SetMultimap[string, int])
		key      string
		wantAll  []int
		wantKeys []string
		wantSize int
	}{
		{
			name:     "missing key",
			build:    func(m *multimap.SetMultimap[string, int]) { m.Put("b", 3) },
			key:      "zz",
			wantKeys: []string{"b"},
			wantSize: 1,
		},
		{
			name: "returns every value of the key",
			build: func(m *multimap.SetMultimap[string, int]) {
				m.PutAll("a", 1, 2)
				m.Put("b", 3)
			},
			key:      "a",
			wantAll:  []int{1, 2},
			wantKeys: []string{"b"},
			wantSize: 1,
		},
		{
			name:  "empty key after a remove",
			build: func(m *multimap.SetMultimap[string, int]) { m.Put("a", 1) },
			key:   "a", wantAll: []int{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewSet[string, int]()
			tt.build(m)

			got := m.RemoveAll(tt.key)
			require.Equal(t, tt.wantAll, got)
			require.Equal(t, wantOrEmpty(tt.wantKeys), m.Keys())
			require.Equal(t, tt.wantSize, m.Size())
			require.Nil(t, m.Get(tt.key), "the key must be gone after RemoveAll")

			// the returned slice is detached from the multimap
			if len(got) > 0 {
				got[0] = -1
			}
			require.Nil(t, m.Get(tt.key))
			requireSetInvariants(t, m)
		})
	}
}

func TestSetReplaceReturnedValues(t *testing.T) {
	m := multimap.NewSet[string, int]()
	m.PutAll("a", 1, 2)
	m.Put("b", 3)

	require.Equal(t, []int{1, 2}, m.Replace("a", 9, 9, 8))
	require.Equal(t, []int{9, 8}, m.Get("a"))
	require.Equal(t, []string{"a", "b"}, m.Keys())
	require.Nil(t, m.Replace("c", 7), "an absent key has no previous values")
	require.Equal(t, []string{"a", "b", "c"}, m.Keys())
	require.Equal(t, []int{3}, m.Replace("b"), "replacing with nothing returns the old values")
	require.Equal(t, []string{"a", "c"}, m.Keys())
	requireSetInvariants(t, m)
}

func TestSetContains(t *testing.T) {
	tests := []struct {
		name      string
		build     func(m *multimap.SetMultimap[string, int])
		key       string
		value     int
		wantKey   bool
		wantEntry bool
	}{
		{
			name:  "empty multimap",
			build: func(m *multimap.SetMultimap[string, int]) {},
			key:   "a", value: 1,
		},
		{
			name:  "key with values",
			build: func(m *multimap.SetMultimap[string, int]) { m.PutAll("a", 1, 2) },
			key:   "a", value: 2,
			wantKey: true, wantEntry: true,
		},
		{
			name:  "value never stored",
			build: func(m *multimap.SetMultimap[string, int]) { m.Put("a", 1) },
			key:   "a", value: 2,
			wantKey: true,
		},
		{
			name:  "missing key",
			build: func(m *multimap.SetMultimap[string, int]) { m.Put("a", 1) },
			key:   "zz", value: 1,
		},
		{
			name: "key emptied by remove",
			build: func(m *multimap.SetMultimap[string, int]) {
				m.Put("a", 1)
				m.Remove("a", 1)
			},
			key: "a", value: 1,
		},
		{
			name:  "the same value under several keys",
			build: func(m *multimap.SetMultimap[string, int]) { m.Put("a", 1); m.Put("b", 1) },
			key:   "b", value: 1,
			wantKey: true, wantEntry: true,
		},
		{
			name: "value removed from one key stays under the other",
			build: func(m *multimap.SetMultimap[string, int]) {
				m.Put("a", 1)
				m.Put("b", 1)
				m.Remove("a", 1)
			},
			key: "b", value: 1,
			wantKey: true, wantEntry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewSet[string, int]()
			tt.build(m)
			require.Equal(t, tt.wantKey, m.ContainsKey(tt.key))
			require.Equal(t, tt.wantEntry, m.ContainsEntry(tt.key, tt.value))
		})
	}
}

func TestSetKeysValuesEntriesAsMapAreCopies(t *testing.T) {
	m := multimap.NewSet[string, int]()
	m.PutAll("a", 1, 2)
	m.Put("b", 3)

	got := m.Get("a")
	got[0] = 99
	require.Equal(t, []int{1, 2}, m.Get("a"), "Get must hand out a copy")
	require.False(t, m.ContainsEntry("a", 99))

	keys := m.Keys()
	keys[0] = "zzz"
	require.Equal(t, []string{"a", "b"}, m.Keys(), "Keys must hand out a copy")

	values := m.Values()
	values[0] = 99
	require.Equal(t, []int{1, 2, 3}, m.Values(), "Values must hand out a copy")

	entries := m.Entries()
	entries[0].Value = 99
	require.Equal(t, 1, m.Entries()[0].Value, "Entries must hand out a copy")

	asMap := m.AsMap()
	asMap["a"][0] = 99
	delete(asMap, "b")
	require.Equal(t, []int{1, 2}, m.Get("a"), "AsMap must hold copied value lists")
	require.True(t, m.ContainsKey("b"))

	// RemoveAll hands out a detached slice
	removed := m.RemoveAll("a")
	removed[0] = 99
	require.False(t, m.ContainsKey("a"))
	require.False(t, m.ContainsEntry("a", 1))

	requireSetInvariants(t, m)
}

func TestSetInvert(t *testing.T) {
	tests := []struct {
		name       string
		build      func(m *multimap.SetMultimap[string, string])
		wantKeys   []string
		wantValues []string
		wantMap    map[string][]string
	}{
		{
			name:  "empty multimap",
			build: func(m *multimap.SetMultimap[string, string]) {},
		},
		{
			name: "one pair",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.Put("k", "v")
			},
			wantKeys:   []string{"v"},
			wantValues: []string{"k"},
			wantMap:    map[string][]string{"v": {"k"}},
		},
		{
			name: "the same value under several keys",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.Put("a", "x")
				m.Put("b", "x")
			},
			wantKeys:   []string{"x"},
			wantValues: []string{"a", "b"},
			wantMap:    map[string][]string{"x": {"a", "b"}},
		},
		{
			name: "colliding pairs merge",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.PutAll("a", "x", "y")
				m.PutAll("b", "y")
			},
			wantKeys:   []string{"x", "y"},
			wantValues: []string{"a", "a", "b"},
			wantMap:    map[string][]string{"x": {"a"}, "y": {"a", "b"}},
		},
		{
			name: "inverted key order follows first appearance",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.PutAll("k1", "v2", "v1", "v2")
				m.PutAll("k2", "v1", "v3")
			},
			wantKeys:   []string{"v2", "v1", "v3"},
			wantValues: []string{"k1", "k1", "k2", "k2"},
			wantMap:    map[string][]string{"v2": {"k1"}, "v1": {"k1", "k2"}, "v3": {"k2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewSet[string, string]()
			tt.build(m)

			inv := m.Invert()
			// inverting preserves the number of pairs but not the number of keys
			require.Equal(t, m.Size(), inv.Size())
			require.Equal(t, wantOrEmpty(tt.wantKeys), inv.Keys())
			require.Equal(t, wantOrEmpty(tt.wantValues), inv.Values())
			if tt.wantMap == nil {
				require.Empty(t, inv.AsMap())
			} else {
				require.Equal(t, tt.wantMap, inv.AsMap())
			}
			requireSetInvariants(t, inv)
		})
	}
}

func TestSetAll(t *testing.T) {
	tests := []struct {
		name  string
		build func(m *multimap.SetMultimap[string, string])
		want  []string
	}{
		{name: "empty multimap", build: func(m *multimap.SetMultimap[string, string]) {}},
		{
			name: "key order then value order",
			build: func(m *multimap.SetMultimap[string, string]) {
				m.PutAll("a", "1", "2")
				m.Put("b", "3")
				m.PutAll("a", "2", "4")
			},
			want: []string{"a=1", "a=2", "a=4", "b=3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := multimap.NewSet[string, string]()
			tt.build(m)

			got := make([]string, 0, m.Size())
			for k, v := range m.All() {
				got = append(got, k+"="+v)
			}
			require.Equal(t, wantOrEmpty(tt.want), got)

			count := 0
			for k, v := range m.All() {
				count++
				require.Equal(t, tt.want[0], k+"="+v)
				break
			}
			if len(tt.want) == 0 {
				require.Zero(t, count)
			}
		})
	}
}

func TestSetClear(t *testing.T) {
	m := multimap.NewSetWithCapacity[string, int](4, "reserved")
	m.PutAll("a", 1, 2)
	m.Put("b", 3)

	m.Clear()
	require.Zero(t, m.Size())
	require.Zero(t, m.KeyCount())
	require.True(t, m.IsEmpty())
	require.Empty(t, m.Keys())
	require.Empty(t, m.Values())
	require.Empty(t, m.Entries())
	require.Empty(t, m.AsMap())
	require.False(t, m.ContainsKey("a"))
	require.False(t, m.ContainsEntry("a", 1))
	requireSetInvariants(t, m)

	// a cleared value is new again
	require.True(t, m.Put("a", 1))
	require.False(t, m.Put("a", 1))
	require.Equal(t, []int{1}, m.Get("a"))
	requireSetInvariants(t, m)
}

func TestSetReservedKeys(t *testing.T) {
	m := multimap.NewSetWithCapacity[string, int](4, "a", "b")

	require.Zero(t, m.Size())
	require.Zero(t, m.KeyCount())
	require.True(t, m.IsEmpty())
	require.Empty(t, m.Keys())
	require.Nil(t, m.Get("a"))
	require.False(t, m.ContainsKey("a"))
	require.False(t, m.ContainsEntry("a", 1))
	requireSetInvariants(t, m)

	m.Put("c", 1)
	require.Equal(t, []string{"c"}, m.Keys())

	// activating a reserved key keeps its reserved position in the order
	require.True(t, m.Put("a", 2))
	require.False(t, m.Put("a", 2))
	require.Equal(t, []string{"a", "c"}, m.Keys())
	require.Equal(t, []int{2}, m.Get("a"))
	require.Equal(t, 2, m.KeyCount())
	requireSetInvariants(t, m)

	// b keeps the second reserved slot, which sits before the appended c
	m.Put("b", 3)
	require.Equal(t, []string{"a", "b", "c"}, m.Keys())
	require.Equal(t, []int{2, 3, 1}, m.Values())

	// dropping the last value of an activated key releases its slot for good
	m.RemoveAll("a")
	require.Equal(t, []string{"b", "c"}, m.Keys())
	m.Put("a", 4)
	require.Equal(t, []string{"b", "c", "a"}, m.Keys())
	requireSetInvariants(t, m)

	// removeall on a reserved key leaves the reservation alone
	res := multimap.NewSetWithCapacity[string, int](1, "r")
	require.Nil(t, res.RemoveAll("r"))
	require.False(t, res.Remove("r", 1))
	res.Put("q", 1)
	res.Put("r", 2)
	require.Equal(t, []string{"r", "q"}, res.Keys(), "the reserved key keeps its slot")

	// repeated reserved keys keep only their first slot
	dup := multimap.NewSetWithCapacity[string, int](2, "x", "x")
	dup.Put("y", 1)
	dup.Put("x", 2)
	require.Equal(t, []string{"x", "y"}, dup.Keys())
	requireSetInvariants(t, dup)
}

func TestSetZeroValueIsUsable(t *testing.T) {
	var m multimap.SetMultimap[string, int]

	require.Zero(t, m.Size())
	require.Zero(t, m.KeyCount())
	require.True(t, m.IsEmpty())
	require.Empty(t, m.Keys())
	require.Empty(t, m.Values())
	require.Empty(t, m.Entries())
	require.Empty(t, m.AsMap())
	require.Nil(t, m.Get("a"))
	require.False(t, m.ContainsKey("a"))
	require.False(t, m.ContainsEntry("a", 1))
	require.False(t, m.Remove("a", 1))
	require.Nil(t, m.RemoveAll("a"))
	require.Nil(t, m.Replace("a"))
	requireSetInvariants(t, &m)

	require.True(t, m.Put("a", 1))
	require.Equal(t, 1, m.PutAll("a", 1, 2))
	require.Equal(t, []int{1, 2}, m.Get("a"))
	m.Clear()
	require.True(t, m.IsEmpty())
	requireSetInvariants(t, &m)
}

func TestSetCapacityPanics(t *testing.T) {
	t.Run("negative capacity", func(t *testing.T) {
		require.PanicsWithValue(t, "multimap: negative capacity", func() {
			multimap.NewSetWithCapacity[string, int](-1)
		})
	})
	t.Run("zero capacity is allowed", func(t *testing.T) {
		m := multimap.NewSetWithCapacity[string, int](0)
		require.True(t, m.IsEmpty())
		m.Put("a", 1)
		require.Equal(t, []int{1}, m.Get("a"))
	})
	t.Run("reserved keys with zero capacity", func(t *testing.T) {
		m := multimap.NewSetWithCapacity[string, int](0, "a")
		require.True(t, m.IsEmpty())
		m.Put("a", 1)
		require.Equal(t, []string{"a"}, m.Keys())
	})
}

// TestListInvariantsAfterEveryMutation re-checks the size, order and uniqueness
// invariants after each single mutation.
func TestListInvariantsAfterEveryMutation(t *testing.T) {
	m := multimap.NewList[string, int]()
	steps := []struct {
		name string
		do   func()
	}{
		{"put on the empty map", func() { m.Put("a", 1) }},
		{"duplicate value under one key", func() { m.Put("a", 1) }},
		{"many values under one key", func() { m.PutAll("b", 2, 3, 4) }},
		{"the same value under several keys", func() { m.Put("c", 1) }},
		{"remove the first duplicate", func() { m.Remove("a", 1) }},
		{"remove an absent value", func() { m.Remove("a", 9) }},
		{"remove under an absent key", func() { m.Remove("zz", 1) }},
		{"removeall", func() { m.RemoveAll("a") }},
		{"removeall an absent key", func() { m.RemoveAll("zz") }},
		{"replace an existing key", func() { m.Replace("b", 5, 5) }},
		{"replace a new key", func() { m.Replace("d", 6, 7) }},
		{"replace an existing key with nothing", func() { m.Replace("d") }},
		{"replace an absent key with nothing", func() { m.Replace("zz") }},
		{"putall with no values", func() { m.PutAll("c") }},
		{"clear", func() { m.Clear() }},
		{"rebuild after clear", func() { m.PutAll("e", 8) }},
	}

	for i, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			step.do()
			requireListInvariants(t, m)
			t.Logf("step %d: keys=%v size=%d", i, m.Keys(), m.Size())
		})
	}
}

// TestSetInvariantsAfterEveryMutation re-checks the size, order and uniqueness
// invariants after each single mutation.
func TestSetInvariantsAfterEveryMutation(t *testing.T) {
	m := multimap.NewSet[string, int]()
	steps := []struct {
		name string
		do   func()
	}{
		{"put on the empty map", func() { m.Put("a", 1) }},
		{"duplicate put", func() { m.Put("a", 1) }},
		{"many values under one key", func() { m.PutAll("b", 2, 3, 3) }},
		{"the same value under several keys", func() { m.Put("c", 1) }},
		{"remove a value", func() { m.Remove("b", 2) }},
		{"remove an absent value", func() { m.Remove("b", 9) }},
		{"remove under an absent key", func() { m.Remove("zz", 1) }},
		{"removeall", func() { m.RemoveAll("a") }},
		{"replace an existing key with duplicates", func() { m.Replace("b", 5, 5, 6) }},
		{"replace a new key", func() { m.Replace("d", 7, 7) }},
		{"replace with nothing", func() { m.Replace("d") }},
		{"putall with no values", func() { m.PutAll("c") }},
		{"clear", func() { m.Clear() }},
		{"rebuild after clear", func() { m.PutAll("e", 8) }},
	}

	for i, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			step.do()
			requireSetInvariants(t, m)
			t.Logf("step %d: keys=%v size=%d", i, m.Keys(), m.Size())
		})
	}
}

// listOpKind is one replayed operation of the differential and fuzz tests.
type listOpKind uint8

const (
	listPut listOpKind = iota
	listPutAll
	listRemove
	listRemoveAll
	listReplace
	listClear
)

// op is one replayed operation with its key and values.
type op[K comparable, V any] struct {
	kind   listOpKind
	key    K
	values []V
}

// listModel mirrors ListMultimap with a plain map plus an insertion-ordered key
// slice: the two structures the differential tests compare against.
type listModel[K comparable, V comparable] struct {
	order  []K
	values map[K][]V
}

func newListModel[K comparable, V comparable]() *listModel[K, V] {
	return &listModel[K, V]{values: make(map[K][]V)}
}

func (mo *listModel[K, V]) Put(k K, v V) {
	if _, ok := mo.values[k]; !ok {
		mo.order = append(mo.order, k)
	}
	mo.values[k] = append(mo.values[k], v)
}

func (mo *listModel[K, V]) Remove(k K, v V) bool {
	vs, ok := mo.values[k]
	if !ok {
		return false
	}
	i := slices.Index(vs, v)
	if i < 0 {
		return false
	}
	vs = slices.Delete(vs, i, i+1)
	if len(vs) == 0 {
		delete(mo.values, k)
		mo.dropKey(k)
		return true
	}
	mo.values[k] = vs
	return true
}

func (mo *listModel[K, V]) RemoveAll(k K) []V {
	vs, ok := mo.values[k]
	if !ok {
		return nil
	}
	delete(mo.values, k)
	mo.dropKey(k)
	return vs
}

func (mo *listModel[K, V]) Replace(k K, vs []V) []V {
	old, ok := mo.values[k]
	if !ok {
		if len(vs) == 0 {
			return nil
		}
		mo.order = append(mo.order, k)
		mo.values[k] = slices.Clone(vs)
		return nil
	}
	if len(vs) == 0 {
		delete(mo.values, k)
		mo.dropKey(k)
		return old
	}
	mo.values[k] = slices.Clone(vs)
	return old
}

func (mo *listModel[K, V]) Clear() {
	clear(mo.values)
	mo.order = mo.order[:0]
}

func (mo *listModel[K, V]) dropKey(k K) {
	mo.order = slices.DeleteFunc(mo.order, func(x K) bool { return x == k })
}

func (mo *listModel[K, V]) size() int {
	n := 0
	for _, vs := range mo.values {
		n += len(vs)
	}
	return n
}

func (mo *listModel[K, V]) keys() []K {
	out := make([]K, 0, len(mo.order))
	return append(out, mo.order...)
}

func (mo *listModel[K, V]) flat() []V {
	out := make([]V, 0, mo.size())
	for _, k := range mo.order {
		out = append(out, mo.values[k]...)
	}
	return out
}

func (mo *listModel[K, V]) entries() []multimap.Entry[K, V] {
	out := make([]multimap.Entry[K, V], 0, mo.size())
	for _, k := range mo.order {
		for _, v := range mo.values[k] {
			out = append(out, multimap.Entry[K, V]{Key: k, Value: v})
		}
	}
	return out
}

func (mo *listModel[K, V]) asMap() map[K][]V {
	out := make(map[K][]V, len(mo.order))
	for _, k := range mo.order {
		out[k] = slices.Clone(mo.values[k])
	}
	return out
}

// setModel mirrors SetMultimap with an insertion-ordered key slice, the values
// of each key in first-seen order and the membership set of each key.
type setModel[K comparable, V comparable] struct {
	order   []K
	values  map[K][]V
	members map[K]map[V]struct{}
}

func newSetModel[K comparable, V comparable]() *setModel[K, V] {
	return &setModel[K, V]{
		values:  make(map[K][]V),
		members: make(map[K]map[V]struct{}),
	}
}

func (mo *setModel[K, V]) Put(k K, v V) bool {
	if _, dup := mo.members[k][v]; dup {
		return false
	}
	if _, ok := mo.values[k]; !ok {
		mo.order = append(mo.order, k)
		mo.members[k] = make(map[V]struct{}, 1)
	}
	mo.members[k][v] = struct{}{}
	mo.values[k] = append(mo.values[k], v)
	return true
}

func (mo *setModel[K, V]) Remove(k K, v V) bool {
	if _, ok := mo.members[k][v]; !ok {
		return false
	}
	delete(mo.members[k], v)
	i := slices.Index(mo.values[k], v)
	vs := slices.Delete(mo.values[k], i, i+1)
	if len(vs) == 0 {
		delete(mo.values, k)
		delete(mo.members, k)
		mo.dropKey(k)
		return true
	}
	mo.values[k] = vs
	return true
}

func (mo *setModel[K, V]) RemoveAll(k K) []V {
	vs, ok := mo.values[k]
	if !ok {
		return nil
	}
	delete(mo.values, k)
	delete(mo.members, k)
	mo.dropKey(k)
	return vs
}

func (mo *setModel[K, V]) Replace(k K, vs []V) []V {
	unique := uniqueValues(vs)
	old, ok := mo.values[k]
	if !ok {
		if len(unique) == 0 {
			return nil
		}
		mo.order = append(mo.order, k)
		mo.values[k] = unique
		mo.members[k] = membershipOf(unique)
		return nil
	}
	if len(unique) == 0 {
		delete(mo.values, k)
		delete(mo.members, k)
		mo.dropKey(k)
		return old
	}
	mo.values[k] = unique
	mo.members[k] = membershipOf(unique)
	return old
}

func (mo *setModel[K, V]) Clear() {
	clear(mo.values)
	clear(mo.members)
	mo.order = mo.order[:0]
}

func (mo *setModel[K, V]) dropKey(k K) {
	mo.order = slices.DeleteFunc(mo.order, func(x K) bool { return x == k })
}

func (mo *setModel[K, V]) size() int {
	n := 0
	for _, vs := range mo.values {
		n += len(vs)
	}
	return n
}

func (mo *setModel[K, V]) keys() []K {
	out := make([]K, 0, len(mo.order))
	return append(out, mo.order...)
}

func (mo *setModel[K, V]) flat() []V {
	out := make([]V, 0, mo.size())
	for _, k := range mo.order {
		out = append(out, mo.values[k]...)
	}
	return out
}

func (mo *setModel[K, V]) entries() []multimap.Entry[K, V] {
	out := make([]multimap.Entry[K, V], 0, mo.size())
	for _, k := range mo.order {
		for _, v := range mo.values[k] {
			out = append(out, multimap.Entry[K, V]{Key: k, Value: v})
		}
	}
	return out
}

func (mo *setModel[K, V]) asMap() map[K][]V {
	out := make(map[K][]V, len(mo.order))
	for _, k := range mo.order {
		out[k] = slices.Clone(mo.values[k])
	}
	return out
}

// uniqueValues returns the unique values in first-occurrence order.
func uniqueValues[V comparable](vs []V) []V {
	out := make([]V, 0, len(vs))
	for _, v := range vs {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// membershipOf returns the membership set of vs.
func membershipOf[V comparable](vs []V) map[V]struct{} {
	out := make(map[V]struct{}, len(vs))
	for _, v := range vs {
		out[v] = struct{}{}
	}
	return out
}

// applyOp replays one operation on the multimaps and their models, checking that
// the value-returning operations agree.
func applyOp[K comparable, V comparable](t *testing.T, m *multimap.ListMultimap[K, V], mo *listModel[K, V], o op[K, V]) {
	t.Helper()
	switch o.kind {
	case listPut:
		m.Put(o.key, o.values[0])
		mo.Put(o.key, o.values[0])
	case listPutAll:
		m.PutAll(o.key, o.values...)
		for _, v := range o.values {
			mo.Put(o.key, v)
		}
	case listRemove:
		require.Equal(t, mo.Remove(o.key, o.values[0]), m.Remove(o.key, o.values[0]), "Remove")
	case listRemoveAll:
		require.Equal(t, mo.RemoveAll(o.key), m.RemoveAll(o.key), "RemoveAll")
	case listReplace:
		require.Equal(t, mo.Replace(o.key, o.values), m.Replace(o.key, o.values...), "Replace")
	case listClear:
		m.Clear()
		mo.Clear()
	}
}

// applySetOp replays one operation on the set multimaps and their models,
// checking that the value-returning operations agree.
func applySetOp[K comparable, V comparable](t *testing.T, m *multimap.SetMultimap[K, V], mo *setModel[K, V], o op[K, V]) {
	t.Helper()
	switch o.kind {
	case listPut:
		require.Equal(t, mo.Put(o.key, o.values[0]), m.Put(o.key, o.values[0]), "Put")
	case listPutAll:
		added := 0
		for _, v := range o.values {
			if mo.Put(o.key, v) {
				added++
			}
		}
		require.Equal(t, added, m.PutAll(o.key, o.values...), "PutAll")
	case listRemove:
		require.Equal(t, mo.Remove(o.key, o.values[0]), m.Remove(o.key, o.values[0]), "Remove")
	case listRemoveAll:
		require.Equal(t, mo.RemoveAll(o.key), m.RemoveAll(o.key), "RemoveAll")
	case listReplace:
		require.Equal(t, mo.Replace(o.key, o.values), m.Replace(o.key, o.values...), "Replace")
	case listClear:
		m.Clear()
		mo.Clear()
	}
}

// requireListMatchesModel compares the whole observable state of m with its
// model.
func requireListMatchesModel[K comparable, V comparable](t *testing.T, m *multimap.ListMultimap[K, V], mo *listModel[K, V]) {
	t.Helper()
	require.Equal(t, mo.size(), m.Size(), "Size")
	require.Equal(t, len(mo.order), m.KeyCount(), "KeyCount")
	require.Equal(t, mo.size() == 0, m.IsEmpty(), "IsEmpty")
	require.Equal(t, mo.keys(), m.Keys(), "Keys")
	require.Equal(t, mo.flat(), m.Values(), "Values")
	require.Equal(t, mo.entries(), m.Entries(), "Entries")
	require.Equal(t, mo.asMap(), m.AsMap(), "AsMap")
	for _, k := range mo.order {
		require.Equalf(t, mo.values[k], m.Get(k), "Get(%v)", k)
		require.Truef(t, m.ContainsKey(k), "ContainsKey(%v)", k)
		for _, v := range mo.values[k] {
			require.Truef(t, m.ContainsEntry(k, v), "ContainsEntry(%v, %v)", k, v)
		}
	}
}

// requireSetMatchesModel compares the whole observable state of m with its
// model.
func requireSetMatchesModel[K comparable, V comparable](t *testing.T, m *multimap.SetMultimap[K, V], mo *setModel[K, V]) {
	t.Helper()
	require.Equal(t, mo.size(), m.Size(), "Size")
	require.Equal(t, len(mo.order), m.KeyCount(), "KeyCount")
	require.Equal(t, mo.size() == 0, m.IsEmpty(), "IsEmpty")
	require.Equal(t, mo.keys(), m.Keys(), "Keys")
	require.Equal(t, mo.flat(), m.Values(), "Values")
	require.Equal(t, mo.entries(), m.Entries(), "Entries")
	require.Equal(t, mo.asMap(), m.AsMap(), "AsMap")
	for _, k := range mo.order {
		require.Equalf(t, mo.values[k], m.Get(k), "Get(%v)", k)
		require.Truef(t, m.ContainsKey(k), "ContainsKey(%v)", k)
		require.Equalf(t, mo.members[k], membershipOf(m.Get(k)), "membership of %v", k)
		for _, v := range mo.values[k] {
			require.Truef(t, m.ContainsEntry(k, v), "ContainsEntry(%v, %v)", k, v)
		}
	}
}

// randomValues returns n random values from the small alphabet the differential
// tests use, so that collisions are frequent.
func randomValues(rng *rand.Rand, n int) []int {
	out := make([]int, 0, n)
	for range n {
		out = append(out, rng.IntN(5))
	}
	return out
}

// randomOp builds one random operation over keys 0..keySpan-1.
func randomOp(rng *rand.Rand, keySpan int) op[int, int] {
	o := op[int, int]{kind: listOpKind(rng.IntN(6)), key: rng.IntN(keySpan)}
	switch o.kind {
	case listPut, listRemove:
		o.values = randomValues(rng, 1)
	case listPutAll:
		o.values = randomValues(rng, 1+rng.IntN(4))
	case listReplace:
		o.values = randomValues(rng, rng.IntN(5))
	case listClear:
		if rng.IntN(8) != 0 {
			o.kind = listPut
			o.values = randomValues(rng, 1)
		}
	}
	return o
}

// TestListModelDifferential replays randomised operations against a plain
// map[K][]V plus an insertion-order key slice.
func TestListModelDifferential(t *testing.T) {
	const (
		keySpan = 6
		steps   = 2000
		missKey = keySpan + 1
	)
	rng := rand.New(rand.NewPCG(20261012, 11))
	m := multimap.NewList[int, int]()
	mo := newListModel[int, int]()

	for i := range steps {
		o := randomOp(rng, keySpan)
		applyOp(t, m, mo, o)
		requireListMatchesModel(t, m, mo)
		requireListInvariants(t, m)

		// a key that never existed must look empty through every reader
		require.False(t, m.ContainsKey(missKey), "step %d", i)
		require.False(t, m.ContainsEntry(missKey, 0), "step %d", i)
		require.Nil(t, m.Get(missKey), "step %d", i)
		require.Nil(t, m.RemoveAll(missKey), "step %d", i)
		require.False(t, m.Remove(missKey, 0), "step %d", i)
	}
}

// TestSetModelDifferential replays randomised operations against a plain
// map[K]map[V]struct{} plus an insertion-order key slice.
func TestSetModelDifferential(t *testing.T) {
	const (
		keySpan = 6
		steps   = 2000
		missKey = keySpan + 1
	)
	rng := rand.New(rand.NewPCG(20261012, 12))
	m := multimap.NewSet[int, int]()
	mo := newSetModel[int, int]()

	for i := range steps {
		o := randomOp(rng, keySpan)
		applySetOp(t, m, mo, o)
		requireSetMatchesModel(t, m, mo)
		requireSetInvariants(t, m)

		require.False(t, m.ContainsKey(missKey), "step %d", i)
		require.False(t, m.ContainsEntry(missKey, 0), "step %d", i)
		require.Nil(t, m.Get(missKey), "step %d", i)
		require.Nil(t, m.RemoveAll(missKey), "step %d", i)
		require.False(t, m.Remove(missKey, 0), "step %d", i)
	}
}

func TestListAllocations(t *testing.T) {
	m := multimap.NewList[string, int]()
	m.PutAll("a", 1, 2, 3)
	m.Put("b", 4)

	// ContainsEntry only looks up the key and scans the value list: no allocation.
	require.Zero(t, testing.AllocsPerRun(100, func() {
		if !m.ContainsEntry("a", 3) {
			t.Error("ContainsEntry missed an existing entry")
		}
	}), "ContainsEntry must not allocate")

	require.Zero(t, testing.AllocsPerRun(100, func() {
		if !m.ContainsKey("b") || m.Size() != 4 || m.KeyCount() != 2 || m.IsEmpty() {
			t.Error("readers changed the multimap")
		}
	}), "ContainsKey, Size, KeyCount and IsEmpty must not allocate")

	// Get hands back a copy of the value list, so each call allocates exactly one
	// backing array and nothing else: assert the exact number.
	require.Equal(t, 1.0, testing.AllocsPerRun(100, func() {
		if len(m.Get("a")) != 3 {
			t.Error("Get returned the wrong value list")
		}
	}), "Get must allocate exactly one slice")

	require.Zero(t, testing.AllocsPerRun(100, func() {
		if m.Get("missing") != nil {
			t.Error("Get must return nil for a missing key")
		}
	}), "Get on a missing key must not allocate")
}

func TestSetAllocations(t *testing.T) {
	m := multimap.NewSet[string, int]()
	m.PutAll("a", 1, 2, 3)
	m.Put("b", 4)

	require.Zero(t, testing.AllocsPerRun(100, func() {
		if !m.ContainsEntry("a", 3) {
			t.Error("ContainsEntry missed an existing entry")
		}
	}), "ContainsEntry must not allocate")

	// Get hands back a copy of the value list, so each call allocates exactly one
	// backing array and nothing else: assert the exact number.
	require.Equal(t, 1.0, testing.AllocsPerRun(100, func() {
		if len(m.Get("a")) != 3 {
			t.Error("Get returned the wrong value list")
		}
	}), "Get must allocate exactly one slice")

	require.Zero(t, testing.AllocsPerRun(100, func() {
		if m.Get("missing") != nil {
			t.Error("Get must return nil for a missing key")
		}
	}), "Get on a missing key must not allocate")
}

// TestListConcurrentInstancesAreIsolated is race-clean by construction: every
// goroutine owns its multimap, so nothing needs a lock.
func TestListConcurrentInstancesAreIsolated(t *testing.T) {
	const (
		goroutines   = 8
		perGoroutine = 512
		keySpan      = 16
	)
	sizes := make([]int, goroutines)
	keyCounts := make([]int, goroutines)
	sums := make([]int, goroutines)
	valueCounts := make([]int, goroutines)

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			m := multimap.NewList[int, int]()
			sum := 0
			for i := range perGoroutine {
				m.Put(i%keySpan, i)
				sum += i
			}
			sizes[g] = m.Size()
			keyCounts[g] = m.KeyCount()
			valueCounts[g] = len(m.Get(0))
			sums[g] = sum
		}(g)
	}
	wg.Wait()

	for g := range goroutines {
		require.Equal(t, perGoroutine, sizes[g], "goroutine %d", g)
		require.Equal(t, keySpan, keyCounts[g], "goroutine %d", g)
		require.Equal(t, perGoroutine/keySpan, valueCounts[g], "goroutine %d", g)
		require.Equal(t, perGoroutine*(perGoroutine-1)/2, sums[g], "goroutine %d", g)
	}
}

// TestListConcurrentSharedUnderOneMutex shows the documented way to share a
// multimap across goroutines: all state under one mutex.
func TestListConcurrentSharedUnderOneMutex(t *testing.T) {
	const (
		goroutines   = 8
		perGoroutine = 300
	)
	var (
		mu     sync.Mutex
		shared = multimap.NewList[int, int]()
		wg     sync.WaitGroup
	)
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range perGoroutine {
				mu.Lock()
				shared.Put(g, i)
				mu.Unlock()
			}
		}(g)
	}
	wg.Wait()

	require.Equal(t, goroutines*perGoroutine, shared.Size())
	require.Equal(t, goroutines, shared.KeyCount())
	for g := range goroutines {
		require.Equal(t, perGoroutine, len(shared.Get(g)), "key %d", g)
	}
}

// TestSetConcurrentInstancesAreIsolated is race-clean by construction: every
// goroutine owns its multimap, so nothing needs a lock.
func TestSetConcurrentInstancesAreIsolated(t *testing.T) {
	const (
		goroutines   = 8
		perGoroutine = 512
		keySpan      = 16
	)
	sizes := make([]int, goroutines)
	keyCounts := make([]int, goroutines)
	valueCounts := make([]int, goroutines)

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			m := multimap.NewSet[int, int]()
			for i := range perGoroutine {
				m.Put(i%keySpan, i)
			}
			sizes[g] = m.Size()
			keyCounts[g] = m.KeyCount()
			valueCounts[g] = len(m.Get(0))
			if !m.ContainsEntry(0, 0) || m.ContainsEntry(0, -1) {
				valueCounts[g] = -1
			}
		}(g)
	}
	wg.Wait()

	for g := range goroutines {
		require.Equal(t, perGoroutine, sizes[g], "goroutine %d", g)
		require.Equal(t, keySpan, keyCounts[g], "goroutine %d", g)
		require.Equal(t, perGoroutine/keySpan, valueCounts[g], "goroutine %d", g)
	}
}

// TestSetConcurrentSharedUnderOneMutex shows the documented way to share a
// multimap across goroutines: all state under one mutex.
func TestSetConcurrentSharedUnderOneMutex(t *testing.T) {
	const (
		goroutines   = 8
		perGoroutine = 300
	)
	var (
		mu     sync.Mutex
		shared = multimap.NewSet[int, int]()
		wg     sync.WaitGroup
	)
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range perGoroutine {
				mu.Lock()
				shared.Put(g, i)
				mu.Unlock()
			}
		}(g)
	}
	wg.Wait()

	require.Equal(t, goroutines*perGoroutine, shared.Size())
	require.Equal(t, goroutines, shared.KeyCount())
	for g := range goroutines {
		require.Equal(t, perGoroutine, len(shared.Get(g)), "key %d", g)
	}
}
