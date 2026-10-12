package random_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/random"
)

// benchDrop is the typical static weight table a benchmark picks from.
type benchDrop struct {
	name   string
	weight float64
}

func benchDrops() []benchDrop {
	return []benchDrop{
		{"common", 60},
		{"uncommon", 25},
		{"rare", 10},
		{"epic", 5},
	}
}

func BenchmarkInt(b *testing.B) {
	g := random.Default()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = g.Int(0, 1000)
	}
}

func BenchmarkString16(b *testing.B) {
	g := random.Default()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = g.String(16)
	}
}

func BenchmarkWeightedTablePick(b *testing.B) {
	table := random.NewWeightedTable(random.Default(), benchDrops(), func(d benchDrop) float64 { return d.weight })
	b.ReportAllocs()
	for b.Loop() {
		table.Pick()
	}
}

func BenchmarkWeightedLinear(b *testing.B) {
	g := random.Default()
	drops := benchDrops()
	weight := func(d benchDrop) float64 { return d.weight }
	b.ReportAllocs()
	for b.Loop() {
		random.Weighted(g, drops, weight)
	}
}

func BenchmarkParallelInt(b *testing.B) {
	g := random.Default()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = g.Int(0, 1000)
		}
	})
}
