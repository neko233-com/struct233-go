package table_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/table"
)

// tblOp is one scripted mutation replayed against the table and the reference
// model.
type tblOp struct {
	kind   string // "put", "remove", "removeRow", "removeColumn", "clear"
	row    int64
	column int64
	value  int64
}

func putOp(row, column, value int64) tblOp {
	return tblOp{kind: "put", row: row, column: column, value: value}
}

func removeOp(row, column int64) tblOp {
	return tblOp{kind: "remove", row: row, column: column}
}

func removeRowOp(row int64) tblOp { return tblOp{kind: "removeRow", row: row} }

func removeColumnOp(column int64) tblOp { return tblOp{kind: "removeColumn", column: column} }

func clearOp() tblOp { return tblOp{kind: "clear"} }

// tableModel is the brute-force reference implementation: a nested
// map[row]map[col]V plus the insertion-ordered cell slice.
type tableModel struct {
	rows  map[int64]map[int64]int64
	order []table.Cell[int64, int64, int64]
}

func newTableModel() *tableModel { return &tableModel{rows: map[int64]map[int64]int64{}} }

func (m *tableModel) put(row, column, value int64) {
	columns, ok := m.rows[row]
	if !ok {
		columns = map[int64]int64{}
		m.rows[row] = columns
	}
	if _, exists := columns[column]; exists {
		columns[column] = value
		for i := range m.order {
			if m.order[i].RowKey == row && m.order[i].ColumnKey == column {
				m.order[i].Value = value
				break
			}
		}
		return
	}
	columns[column] = value
	m.order = append(m.order, table.Cell[int64, int64, int64]{
		RowKey: row, ColumnKey: column, Value: value,
	})
}

func (m *tableModel) remove(row, column int64) {
	columns, ok := m.rows[row]
	if !ok {
		return
	}
	if _, exists := columns[column]; !exists {
		return
	}
	delete(columns, column)
	if len(columns) == 0 {
		delete(m.rows, row)
	}
	for i := range m.order {
		if m.order[i].RowKey == row && m.order[i].ColumnKey == column {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
}

func (m *tableModel) removeRow(row int64) {
	if _, ok := m.rows[row]; !ok {
		return
	}
	delete(m.rows, row)
	m.order = slices.DeleteFunc(m.order, func(c table.Cell[int64, int64, int64]) bool {
		return c.RowKey == row
	})
}

func (m *tableModel) removeColumn(column int64) {
	found := false
	for _, c := range m.order {
		if c.ColumnKey == column {
			found = true
			break
		}
	}
	if !found {
		return
	}
	for row, columns := range m.rows {
		delete(columns, column)
		if len(columns) == 0 {
			delete(m.rows, row)
		}
	}
	m.order = slices.DeleteFunc(m.order, func(c table.Cell[int64, int64, int64]) bool {
		return c.ColumnKey == column
	})
}

func (m *tableModel) clear() {
	m.rows = map[int64]map[int64]int64{}
	m.order = nil
}

// putAll copies other's cells in its insertion order, mirroring Table.PutAll.
func (m *tableModel) putAll(other *tableModel) {
	for _, c := range slices.Clone(other.order) {
		m.put(c.RowKey, c.ColumnKey, c.Value)
	}
}

// rowKeys returns row keys in the order their first cell was inserted.
func (m *tableModel) rowKeys() []int64 {
	out := []int64{}
	seen := map[int64]bool{}
	for _, c := range m.order {
		if !seen[c.RowKey] {
			seen[c.RowKey] = true
			out = append(out, c.RowKey)
		}
	}
	return out
}

// columnKeys returns column keys in the order their first cell was inserted.
func (m *tableModel) columnKeys() []int64 {
	out := []int64{}
	seen := map[int64]bool{}
	for _, c := range m.order {
		if !seen[c.ColumnKey] {
			seen[c.ColumnKey] = true
			out = append(out, c.ColumnKey)
		}
	}
	return out
}

// columnMap derives the expected column-first nested map from the model.
func (m *tableModel) columnMap() map[int64]map[int64]int64 {
	out := map[int64]map[int64]int64{}
	for _, c := range m.order {
		rows := out[c.ColumnKey]
		if rows == nil {
			rows = map[int64]int64{}
			out[c.ColumnKey] = rows
		}
		rows[c.RowKey] = c.Value
	}
	return out
}

// renderTable prints the cell order for golden strings in the tables below.
func renderTable(tb *table.Table[int64, int64, int64]) string {
	cells := tb.Cells()
	if len(cells) == 0 {
		return "empty"
	}
	parts := make([]string, 0, len(cells))
	for _, c := range cells {
		parts = append(parts, fmt.Sprintf("%d:%d=%d", c.RowKey, c.ColumnKey, c.Value))
	}
	return strings.Join(parts, " ")
}

// checkTableAgainstModel asserts that tb agrees with the model on every
// observable: size, cell order, snapshots, key orders and probing.
func checkTableAgainstModel(t *testing.T, tb *table.Table[int64, int64, int64], m *tableModel, where string) {
	t.Helper()
	require.Equalf(t, len(m.order), tb.Size(), "%s: Size", where)
	require.Equalf(t, len(m.order), tb.Len(), "%s: Len", where)
	require.Equalf(t, len(m.order) == 0, tb.IsEmpty(), "%s: IsEmpty", where)
	require.Truef(t, slices.Equal(m.order, tb.Cells()), "%s: Cells = %v, model = %v", where, tb.Cells(), m.order)

	require.Equalf(t, m.rows, tb.RowMap(), "%s: RowMap", where)
	require.Equalf(t, m.columnMap(), tb.ColumnMap(), "%s: ColumnMap", where)

	wantRows, wantColumns := m.rowKeys(), m.columnKeys()
	require.Truef(t, slices.Equal(wantRows, tb.RowKeys()), "%s: RowKeys = %v, model = %v", where, tb.RowKeys(), wantRows)
	require.Truef(t, slices.Equal(wantColumns, tb.ColumnKeys()), "%s: ColumnKeys = %v, model = %v", where, tb.ColumnKeys(), wantColumns)

	wantValues := make([]int64, 0, len(m.order))
	for _, c := range m.order {
		wantValues = append(wantValues, c.Value)
	}
	require.Truef(t, slices.Equal(wantValues, tb.Values()), "%s: Values = %v, model = %v", where, tb.Values(), wantValues)

	// probe every key the model knows plus two outsiders
	probeRows := append(slices.Clone(wantRows), -2, 50)
	probeColumns := append(slices.Clone(wantColumns), -2, 50)
	for _, row := range probeRows {
		require.Equalf(t, len(m.rows[row]) != 0, tb.ContainsRow(row), "%s: ContainsRow(%d)", where, row)
		wantRow := m.rows[row]
		if len(wantRow) == 0 {
			wantRow = nil
		}
		require.Equalf(t, wantRow, tb.Row(row), "%s: Row(%d)", where, row)
		for _, column := range probeColumns {
			want, wantOK := m.rows[row][column]
			got, gotOK := tb.Get(row, column)
			require.Equalf(t, wantOK, gotOK, "%s: Get(%d, %d) ok; cells=%v", where, row, column, tb.Cells())
			require.Equalf(t, want, got, "%s: Get(%d, %d) value; cells=%v", where, row, column, tb.Cells())
			require.Equalf(t, wantOK, tb.Contains(row, column), "%s: Contains(%d, %d)", where, row, column)
		}
	}
	for _, column := range probeColumns {
		wantColumn := map[int64]int64{}
		for _, c := range m.order {
			if c.ColumnKey == column {
				wantColumn[c.RowKey] = c.Value
			}
		}
		if len(wantColumn) == 0 {
			wantColumn = nil
		}
		require.Equalf(t, wantColumn, tb.Column(column), "%s: Column(%d)", where, column)
		require.Equalf(t, len(wantColumn) != 0, tb.ContainsColumn(column), "%s: ContainsColumn(%d)", where, column)
	}

	var got []table.Cell[int64, int64, int64]
	for cell := range tb.All() {
		got = append(got, cell)
	}
	require.Truef(t, slices.Equal(m.order, got), "%s: All() = %v, model = %v", where, got, m.order)
}

// runTableScript replays ops against a real table and the model, checking the
// full agreement after every single mutation.
func runTableScript(t *testing.T, zeroValue bool, ops []tblOp) *table.Table[int64, int64, int64] {
	t.Helper()
	var zero table.Table[int64, int64, int64]
	tb := &zero
	if !zeroValue {
		tb = table.New[int64, int64, int64]()
	}
	m := newTableModel()
	for i, o := range ops {
		switch o.kind {
		case "put":
			tb.Put(o.row, o.column, o.value)
			m.put(o.row, o.column, o.value)
		case "remove":
			tb.Remove(o.row, o.column)
			m.remove(o.row, o.column)
		case "removeRow":
			tb.RemoveRow(o.row)
			m.removeRow(o.row)
		case "removeColumn":
			tb.RemoveColumn(o.column)
			m.removeColumn(o.column)
		case "clear":
			tb.Clear()
			m.clear()
		}
		checkTableAgainstModel(t, tb, m, fmt.Sprintf("op %d (%s %d:%d)", i, o.kind, o.row, o.column))
	}
	return tb
}

// TestTableDeep_Scripts replays scripted mutations and compares the resulting
// cell order against golden strings while the model is checked after every op.
func TestTableDeep_Scripts(t *testing.T) {
	tests := []struct {
		name string
		zero bool
		ops  []tblOp
		want string
	}{
		{"empty table", false, nil, "empty"},
		{"single put", false, []tblOp{putOp(1, 1, 10)}, "1:1=10"},
		{"two cells same row", false, []tblOp{putOp(1, 1, 10), putOp(1, 2, 20)}, "1:1=10 1:2=20"},
		{"two cells same column", false, []tblOp{putOp(1, 1, 10), putOp(2, 1, 20)}, "1:1=10 2:1=20"},
		{"update keeps position", false, []tblOp{putOp(1, 1, 10), putOp(2, 1, 20), putOp(1, 1, 11)}, "1:1=11 2:1=20"},
		{"repeated updates keep first position", false, []tblOp{
			putOp(1, 1, 1), putOp(1, 2, 2), putOp(1, 1, 3), putOp(1, 1, 4),
		}, "1:1=4 1:2=2"},
		{"remove then re-add goes last", false, []tblOp{
			putOp(1, 1, 1), putOp(1, 2, 2), removeOp(1, 1), putOp(1, 1, 3),
		}, "1:2=2 1:1=3"},
		{"remove middle compacts", false, []tblOp{
			putOp(1, 1, 1), putOp(1, 2, 2), putOp(1, 3, 3), removeOp(1, 2),
		}, "1:1=1 1:3=3"},
		{"remove first", false, []tblOp{putOp(1, 1, 1), putOp(2, 2, 2), removeOp(1, 1)}, "2:2=2"},
		{"remove last", false, []tblOp{putOp(1, 1, 1), putOp(2, 2, 2), removeOp(2, 2)}, "1:1=1"},
		{"remove missing cell", false, []tblOp{putOp(1, 1, 1), removeOp(1, 2), removeOp(2, 1)}, "1:1=1"},
		{"remove row multi cells", false, []tblOp{
			putOp(1, 1, 1), putOp(1, 2, 2), putOp(2, 1, 3), removeRowOp(1),
		}, "2:1=3"},
		{"remove row keeps others order", false, []tblOp{
			putOp(2, 1, 1), putOp(1, 1, 2), putOp(3, 1, 3), removeRowOp(1),
		}, "2:1=1 3:1=3"},
		{"remove missing row", false, []tblOp{putOp(1, 1, 1), removeRowOp(9)}, "1:1=1"},
		{"remove column multi rows", false, []tblOp{
			putOp(1, 1, 1), putOp(2, 1, 2), putOp(1, 2, 3), removeColumnOp(1),
		}, "1:2=3"},
		{"remove column keeps others order", false, []tblOp{
			putOp(1, 2, 1), putOp(1, 1, 2), putOp(1, 3, 3), removeColumnOp(1),
		}, "1:2=1 1:3=3"},
		{"remove missing column", false, []tblOp{putOp(1, 1, 1), removeColumnOp(5)}, "1:1=1"},
		{"remove row then column", false, []tblOp{
			putOp(1, 1, 1), putOp(1, 2, 2), putOp(2, 1, 3), putOp(2, 2, 4),
			removeRowOp(1), removeColumnOp(2),
		}, "2:1=3"},
		{"remove column then row", false, []tblOp{
			putOp(1, 1, 1), putOp(1, 2, 2), putOp(2, 1, 3), putOp(2, 2, 4),
			removeColumnOp(1), removeRowOp(2),
		}, "1:2=2"},
		{"clear then reuse", false, []tblOp{putOp(1, 1, 1), clearOp(), putOp(2, 2, 2)}, "2:2=2"},
		{"clear empty", false, []tblOp{clearOp()}, "empty"},
		{"zero value put", true, []tblOp{putOp(1, 1, 10)}, "1:1=10"},
		{"zero value update", true, []tblOp{putOp(1, 1, 1), putOp(1, 1, 2)}, "1:1=2"},
		{"zero value remove no-op", true, []tblOp{removeOp(1, 1)}, "empty"},
		{"zero value clear", true, []tblOp{clearOp()}, "empty"},
		{"zero value removeRow no-op", true, []tblOp{removeRowOp(1)}, "empty"},
		{"zero value removeColumn no-op", true, []tblOp{removeColumnOp(1)}, "empty"},
		{"removeRow empties table", false, []tblOp{putOp(1, 1, 1), removeRowOp(1)}, "empty"},
		{"removeColumn empties table", false, []tblOp{putOp(1, 1, 1), removeColumnOp(1)}, "empty"},
		{"row keys order after re-add", false, []tblOp{
			putOp(1, 1, 1), putOp(2, 1, 2), removeRowOp(1), putOp(1, 2, 3),
		}, "2:1=2 1:2=3"},
		{"column keys order after re-add", false, []tblOp{
			putOp(1, 1, 1), putOp(1, 2, 2), removeColumnOp(1), putOp(2, 1, 3),
		}, "1:2=2 2:1=3"},
		{"update does not reorder", false, []tblOp{
			putOp(1, 1, 1), putOp(2, 2, 2), putOp(1, 1, 9), putOp(2, 2, 8),
		}, "1:1=9 2:2=8"},
		{"matrix 2x2 remove diagonal", false, []tblOp{
			putOp(1, 1, 1), putOp(1, 2, 2), putOp(2, 1, 3), putOp(2, 2, 4),
			removeOp(1, 1), removeOp(2, 2),
		}, "1:2=2 2:1=3"},
		{"same value different cells", false, []tblOp{
			putOp(1, 1, 5), putOp(1, 2, 5), putOp(2, 1, 5),
		}, "1:1=5 1:2=5 2:1=5"},
		{"put over removed row", false, []tblOp{
			putOp(1, 1, 1), removeRowOp(1), putOp(1, 1, 2),
		}, "1:1=2"},
		{"put over removed column", false, []tblOp{
			putOp(1, 1, 1), removeColumnOp(1), putOp(1, 1, 2),
		}, "1:1=2"},
		{"interleaved row removals", false, []tblOp{
			putOp(1, 1, 1), putOp(2, 2, 2), putOp(3, 3, 3), removeRowOp(2), removeRowOp(1), putOp(4, 4, 4),
		}, "3:3=3 4:4=4"},
		{"interleaved column removals", false, []tblOp{
			putOp(1, 1, 1), putOp(2, 2, 2), putOp(3, 3, 3), removeColumnOp(3), removeColumnOp(1),
		}, "2:2=2"},
		{"remove middle of row then whole row", false, []tblOp{
			putOp(1, 1, 1), putOp(1, 2, 2), putOp(1, 3, 3), removeOp(1, 2), removeRowOp(1),
		}, "empty"},
		{"single row sequence", false, []tblOp{
			putOp(1, 1, 1), putOp(1, 2, 2), putOp(1, 3, 3), removeOp(1, 1), putOp(1, 1, 4),
		}, "1:2=2 1:3=3 1:1=4"},
		{"single column sequence", false, []tblOp{
			putOp(1, 1, 1), putOp(2, 1, 2), putOp(3, 1, 3), removeOp(2, 1), putOp(2, 1, 4),
		}, "1:1=1 3:1=3 2:1=4"},
		{"negative keys", false, []tblOp{
			putOp(-1, -1, 1), putOp(-2, 1, 2), removeOp(-1, -1),
		}, "-2:1=2"},
		{"update across rows same column", false, []tblOp{
			putOp(1, 1, 1), putOp(2, 1, 2), putOp(1, 1, 3), putOp(2, 1, 4),
		}, "1:1=3 2:1=4"},
		{"remove row other row survives", false, []tblOp{
			putOp(1, 1, 1), putOp(2, 1, 2), removeRowOp(2),
		}, "1:1=1"},
		{"clear then put keeps fresh order", false, []tblOp{
			putOp(1, 1, 1), putOp(2, 2, 2), clearOp(), putOp(2, 2, 20), putOp(1, 1, 10),
		}, "2:2=20 1:1=10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := runTableScript(t, tt.zero, tt.ops)
			require.Equal(t, tt.want, renderTable(tb))
		})
	}
}

// TestTableDeep_NewWithCapacity covers constructor edge cases: a negative
// capacity is treated as zero and the table stays fully usable.
func TestTableDeep_NewWithCapacity(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		ops      []tblOp
		want     string
	}{
		{"negative capacity", -8, []tblOp{putOp(1, 1, 1), putOp(1, 2, 2)}, "1:1=1 1:2=2"},
		{"zero capacity", 0, []tblOp{putOp(2, 2, 2)}, "2:2=2"},
		{"positive capacity", 64, []tblOp{putOp(1, 1, 1)}, "1:1=1"},
		{"capacity then remove all", -1, []tblOp{putOp(1, 1, 1), removeOp(1, 1)}, "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := table.NewWithCapacity[int64, int64, int64](tt.capacity)
			m := newTableModel()
			for i, o := range tt.ops {
				switch o.kind {
				case "put":
					tb.Put(o.row, o.column, o.value)
					m.put(o.row, o.column, o.value)
				case "remove":
					tb.Remove(o.row, o.column)
					m.remove(o.row, o.column)
				case "removeRow":
					tb.RemoveRow(o.row)
					m.removeRow(o.row)
				case "removeColumn":
					tb.RemoveColumn(o.column)
					m.removeColumn(o.column)
				case "clear":
					tb.Clear()
					m.clear()
				}
				checkTableAgainstModel(t, tb, m, fmt.Sprintf("op %d", i))
			}
			require.Equal(t, tt.want, renderTable(tb))
		})
	}
}

// TestTableDeep_PutAll checks the merge semantics: existing cell positions are
// preserved, new cells append in the source's iteration order.
func TestTableDeep_PutAll(t *testing.T) {
	tests := []struct {
		name string
		self bool
		a    []tblOp
		b    []tblOp
		want string
	}{
		{
			name: "appends new cells in source order",
			a:    []tblOp{putOp(1, 1, 1)},
			b:    []tblOp{putOp(2, 2, 2), putOp(3, 1, 3)},
			want: "1:1=1 2:2=2 3:1=3",
		},
		{
			name: "overwrite keeps position",
			a:    []tblOp{putOp(1, 1, 1), putOp(2, 2, 2)},
			b:    []tblOp{putOp(2, 2, 20), putOp(1, 1, 10)},
			want: "1:1=10 2:2=20",
		},
		{
			name: "mixed overwrite and append",
			a:    []tblOp{putOp(1, 1, 1), putOp(1, 2, 2)},
			b:    []tblOp{putOp(1, 2, 20), putOp(2, 1, 30), putOp(2, 2, 40)},
			want: "1:1=1 1:2=20 2:1=30 2:2=40",
		},
		{
			name: "empty source",
			a:    []tblOp{putOp(1, 1, 1)},
			b:    nil,
			want: "1:1=1",
		},
		{
			name: "into empty table",
			a:    nil,
			b:    []tblOp{putOp(1, 1, 1), putOp(2, 2, 2)},
			want: "1:1=1 2:2=2",
		},
		{
			name: "self merge keeps order",
			self: true,
			a:    []tblOp{putOp(1, 1, 1), putOp(2, 2, 2)},
			want: "1:1=1 2:2=2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := runTableScript(t, false, tt.a)
			modelA := newTableModel()
			for _, o := range tt.a {
				modelA.put(o.row, o.column, o.value)
			}
			var b *table.Table[int64, int64, int64]
			modelB := newTableModel()
			if tt.self {
				b = a
				modelB = modelA
			} else {
				b = runTableScript(t, false, tt.b)
				for _, o := range tt.b {
					modelB.put(o.row, o.column, o.value)
				}
			}
			a.PutAll(b)
			modelA.putAll(modelB)
			checkTableAgainstModel(t, a, modelA, "after PutAll")
			require.Equal(t, tt.want, renderTable(a))

			// a nil source is ignored
			a.PutAll(nil)
			checkTableAgainstModel(t, a, modelA, "after PutAll(nil)")
		})
	}
}

// TestTableDeep_SnapshotsAreDetached mutates every returned snapshot and
// requires the table to stay untouched.
func TestTableDeep_SnapshotsAreDetached(t *testing.T) {
	tb := table.New[int64, int64, int64]()
	tb.Put(1, 1, 11)
	tb.Put(1, 2, 12)
	tb.Put(2, 1, 21)
	before := tb.Cells()

	row := tb.Row(1)
	row[1] = 999
	delete(row, 2)
	column := tb.Column(1)
	column[1] = 999
	rowMap := tb.RowMap()
	rowMap[1][1] = 999
	delete(rowMap, 2)
	columnMap := tb.ColumnMap()
	columnMap[1][1] = 999
	cells := tb.Cells()
	cells[0] = table.Cell[int64, int64, int64]{RowKey: 9, ColumnKey: 9, Value: 9}
	values := tb.Values()
	values[0] = 999
	rowKeys := tb.RowKeys()
	rowKeys[0] = 9
	columnKeys := tb.ColumnKeys()
	columnKeys[0] = 9

	require.Equal(t, before, tb.Cells(), "no snapshot may mutate the table")
	require.Equal(t, "1:1=11 1:2=12 2:1=21", renderTable(tb))

	// Row of a missing key is nil, and mutating it must not panic
	missing := tb.Row(77)
	require.Nil(t, missing)
	require.Nil(t, tb.Column(77))
	require.Nil(t, tb.RowMap()[77])
}

// TestTableDeep_IterationSemantics checks All against Cells and early stops.
func TestTableDeep_IterationSemantics(t *testing.T) {
	tb := table.New[int64, int64, int64]()
	var got []table.Cell[int64, int64, int64]
	for cell := range tb.All() {
		got = append(got, cell)
	}
	require.Empty(t, got, "an empty table iterates nothing")

	tb.Put(1, 1, 1)
	tb.Put(2, 2, 2)
	tb.Put(3, 3, 3)
	for cell := range tb.All() {
		got = append(got, cell)
	}
	require.Equal(t, tb.Cells(), got)

	seen := 0
	for range tb.All() {
		seen++
		break
	}
	require.Equal(t, 1, seen, "break must stop the iteration")

	// the two size accessors agree
	require.Equal(t, tb.Size(), tb.Len())
	require.False(t, tb.IsEmpty())
}

// TestTableDeep_BoundariesDoNotPanic pins the documented non-panicking
// boundary behaviours. Neither table API path panics, so there is no panic
// message prefix to assert on.
func TestTableDeep_BoundariesDoNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		tb := table.NewWithCapacity[int64, int64, int64](-8)
		tb.Put(1, 1, 1)
		_, _ = tb.Get(2, 2)
		_, _ = tb.Remove(2, 2)
		_ = tb.RemoveRow(2)
		_ = tb.RemoveColumn(2)
		tb.PutAll(nil)
		tb.PutAll(tb)
		tb.Clear()
		tb.Clear()
	}, "every boundary call must be a no-op instead of a panic")
}

// TestTableDeep_ModelRandomized replays random Put/Remove traffic against the
// map[row]map[col]V + insertion-order model. This is the differential test:
// any disagreement on values, cell order or snapshots fails with the exact op.
func TestTableDeep_ModelRandomized(t *testing.T) {
	for _, seed := range [][2]uint64{{2026, 1012}, {7, 11}, {99, 4242}, {1, 0}} {
		rng := rand.New(rand.NewPCG(seed[0], seed[1]))
		data := make([]byte, 5*maxTableOps)
		for i := range data {
			data[i] = byte(rng.IntN(256))
		}
		runTableScript(t, false, decodeTableOps(data))
	}
}

// maxTableOps bounds the replay length so fuzzing stays fast.
const maxTableOps = 64

// decodeTableOps turns arbitrary bytes into mutations, five bytes per
// mutation: the kind and the row/column/value key material. Keys are int64
// values drawn from a small domain so collisions and reuse happen often.
func decodeTableOps(data []byte) []tblOp {
	ops := make([]tblOp, 0, maxTableOps)
	for i := 0; i+5 <= len(data) && len(ops) < maxTableOps; i += 5 {
		row := int64(int8(data[i+1])) % 5
		column := int64(int8(data[i+2])) % 5
		value := int64(int8(data[i+4]))
		switch data[i] % 5 {
		case 0:
			ops = append(ops, tblOp{kind: "put", row: row, column: column, value: value})
		case 1:
			ops = append(ops, tblOp{kind: "remove", row: row, column: column})
		case 2:
			ops = append(ops, tblOp{kind: "removeRow", row: row})
		case 3:
			ops = append(ops, tblOp{kind: "removeColumn", column: column})
		default:
			ops = append(ops, tblOp{kind: "clear"})
		}
	}
	return ops
}

// TestTableDeep_GetDoesNotAllocate guards the read hot path: Get and Contains
// must not allocate on a populated table.
func TestTableDeep_GetDoesNotAllocate(t *testing.T) {
	tb := table.New[int64, int64, int64]()
	for i := int64(0); i < 1024; i++ {
		tb.Put(i, i%16, i)
	}

	var value int64
	var ok bool
	allocs := testing.AllocsPerRun(100, func() {
		value, ok = tb.Get(512, 512%16)
	})
	require.Zero(t, allocs, "Get must not allocate")
	require.True(t, ok)
	require.Equal(t, int64(512), value)

	allocs = testing.AllocsPerRun(100, func() {
		ok = tb.Contains(512, 512%16)
	})
	require.Zero(t, allocs, "Contains must not allocate")
	require.True(t, ok)

	allocs = testing.AllocsPerRun(100, func() {
		ok = tb.ContainsRow(512)
	})
	require.Zero(t, allocs, "ContainsRow must not allocate")
	require.True(t, ok)
}
