package table_test

import (
	"iter"
	"slices"
	"testing"

	"github.com/neko233-com/struct233-go/table"
	"github.com/stretchr/testify/require"
)

func TestTablePutGetAndInsertionOrder(t *testing.T) {
	var m table.Table[string, string, *int] // the zero value is usable
	one, two := 1, 2
	_, existed := m.Put("r2", "c1", &one)
	require.False(t, existed)
	m.Put("r1", "c2", &two)
	old, existed := m.Put("r2", "c1", &two)
	require.True(t, existed)
	require.Same(t, &one, old)
	require.Same(t, &two, mustGet(t, &m, "r2", "c1"))
	require.Equal(t, []table.Cell[string, string, *int]{
		{RowKey: "r2", ColumnKey: "c1", Value: &two},
		{RowKey: "r1", ColumnKey: "c2", Value: &two},
	}, m.Cells(), "an update must keep its cell position")
	require.Equal(t, []string{"r2", "r1"}, m.RowKeys())
	require.Equal(t, []string{"c1", "c2"}, m.ColumnKeys())
	require.True(t, m.ContainsRow("r1"))
	require.True(t, m.ContainsColumn("c2"))
}

func mustGet[R comparable, C comparable, V any](t *testing.T, m *table.Table[R, C, V], row R, column C) V {
	t.Helper()
	value, ok := m.Get(row, column)
	require.True(t, ok)
	return value
}

func TestTableSnapshotsAndRemoval(t *testing.T) {
	m := table.NewWithCapacity[string, int, int](8)
	m.Put("r1", 1, 11)
	m.Put("r2", 1, 21)
	m.Put("r1", 2, 12)
	row := m.Row("r1")
	column := m.Column(1)
	rowMap := m.RowMap()
	columnMap := m.ColumnMap()
	row[1] = 999
	column["r1"] = 999
	rowMap["r1"][2] = 999
	columnMap[1]["r2"] = 999
	require.Equal(t, 11, mustGet(t, m, "r1", 1))
	require.Equal(t, 12, mustGet(t, m, "r1", 2))
	require.Equal(t, 21, mustGet(t, m, "r2", 1))

	removedColumn := m.RemoveColumn(1)
	require.Equal(t, map[string]int{"r1": 11, "r2": 21}, removedColumn)
	require.Equal(t, []table.Cell[string, int, int]{{RowKey: "r1", ColumnKey: 2, Value: 12}}, m.Cells())
	removedRow := m.RemoveRow("r1")
	require.Equal(t, map[int]int{2: 12}, removedRow)
	require.True(t, m.IsEmpty())
	require.Nil(t, m.RemoveRow("missing"))
	require.Nil(t, m.RemoveColumn(5))
}

func TestTableRemovePutAllAndClear(t *testing.T) {
	a := table.New[string, int, int]()
	a.Put("a", 1, 1)
	a.Put("b", 1, 2)
	a.Put("a", 2, 3)
	old, ok := a.Remove("b", 1)
	require.True(t, ok)
	require.Equal(t, 2, old)
	a.Put("b", 1, 4)
	require.Equal(t, []string{"a", "b"}, a.RowKeys())
	require.Equal(t, []int{1, 2}, a.ColumnKeys())

	b := table.New[string, int, int]()
	b.Put("a", 1, 10)
	b.Put("c", 3, 30)
	a.PutAll(b)
	a.PutAll(nil)
	require.Equal(t, 10, mustGet(t, a, "a", 1))
	require.Equal(t, []table.Cell[string, int, int]{
		{RowKey: "a", ColumnKey: 1, Value: 10},
		{RowKey: "a", ColumnKey: 2, Value: 3},
		{RowKey: "b", ColumnKey: 1, Value: 4},
		{RowKey: "c", ColumnKey: 3, Value: 30},
	}, a.Cells())
	a.Clear()
	require.True(t, a.IsEmpty())
	a.Put("z", 9, 99)
	require.Equal(t, 99, mustGet(t, a, "z", 9))
}

func TestTableAllAndCellCopies(t *testing.T) {
	m := table.New[int, int, string]()
	m.Put(1, 2, "x")
	m.Put(3, 4, "y")
	cells := m.Cells()
	cells[0].Value = "changed"
	require.Equal(t, "x", mustGet(t, m, 1, 2))
	var got []table.Cell[int, int, string]
	var seq iter.Seq[table.Cell[int, int, string]] = m.All()
	for cell := range seq {
		got = append(got, cell)
	}
	require.True(t, slices.Equal(got, m.Cells()))
}
