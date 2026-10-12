package multiset_test

import (
	"math"
	"testing"

	"github.com/neko233-com/struct233-go/multiset"
	"github.com/stretchr/testify/require"
)

func TestCountOverflowIsRejectedWithoutMutation(t *testing.T) {
	for name, mutate := range map[string]func(*multiset.Multiset[string]){
		"Add existing": func(m *multiset.Multiset[string]) { m.Add("full") },
		"Add new":      func(m *multiset.Multiset[string]) { m.Add("new") },
		"AddN":         func(m *multiset.Multiset[string]) { m.AddN("full", 2) },
		"SetCount":     func(m *multiset.Multiset[string]) { m.SetCount("new", 1) },
	} {
		t.Run(name, func(t *testing.T) {
			m := multiset.New[string]()
			m.SetCount("full", math.MaxInt)
			require.PanicsWithValue(t, "multiset: total count overflow", func() { mutate(m) })
			require.Equal(t, math.MaxInt, m.TotalCount())
			require.Equal(t, math.MaxInt, m.Count("full"))
			require.Equal(t, []string{"full"}, m.DistinctElements())
			require.False(t, m.Contains("new"))
			m.Remove("full")
			m.Add("new")
			require.Equal(t, math.MaxInt, m.TotalCount())
			m.SetCount("full", 1)
			require.Equal(t, 2, m.TotalCount())
			m.Clear()
			require.Zero(t, m.TotalCount())
		})
	}
}

var totalSink int

func BenchmarkTotalCount1024(b *testing.B) {
	m := multiset.New[int]()
	for i := range 1024 {
		m.AddN(i, 2)
	}
	b.ReportAllocs()
	for b.Loop() {
		totalSink = m.TotalCount()
	}
}
