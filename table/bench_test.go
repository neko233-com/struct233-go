package table_test

import (
	"testing"

	"github.com/neko233-com/struct233-go/table"
)

// sink keeps benchmark results alive so the compiler cannot drop the work.
var sink any

// benchTable returns a table of n cells spread over 16 columns.
func benchTable(n int) *table.Table[int64, int64, int64] {
	t := table.New[int64, int64, int64]()
	for i := int64(0); i < int64(n); i++ {
		t.Put(i, i%16, i)
	}
	return t
}

func BenchmarkTablePut(b *testing.B) {
	t := table.New[int64, int64, int64]()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		t.Put(int64(i%1000), int64(i%16), int64(i))
	}
	sink = t.Size()
}

func BenchmarkTablePutUpdate(b *testing.B) {
	t := benchTable(1000)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		t.Put(int64(i%1000), int64(i%16), int64(i))
	}
	sink = t.Size()
}

func BenchmarkTableGet(b *testing.B) {
	t := benchTable(1000)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_, _ = t.Get(int64(i%1000), int64(i%16))
	}
}

func BenchmarkTableRemove(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		t := benchTable(1000)
		_, _ = t.Remove(int64(i%1000), int64(i%16))
		sink = t.Size()
	}
}

func BenchmarkTableAll(b *testing.B) {
	t := benchTable(1000)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		n := 0
		for cell := range t.All() {
			n += int(cell.Value)
		}
		sink = n
	}
}

func BenchmarkTableCells(b *testing.B) {
	t := benchTable(1000)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		sink = len(t.Cells())
	}
}
