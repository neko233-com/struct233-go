// Package table provides a two-dimensional map, modelled after
// com.google.common.collect.Table.
//
// A Table stores at most one value for each row/column pair. Cells are
// iterated in insertion order; updating a cell keeps its position, while
// removing and re-adding it moves it to the end. Row, column and map accessors
// return detached snapshots. A Table is not safe for concurrent use.
package table

import "iter"

// Cell is one row, column and value tuple in a Table.
type Cell[R comparable, C comparable, V any] struct {
	// RowKey is the row the cell belongs to.
	RowKey R
	// ColumnKey is the column the cell belongs to.
	ColumnKey C
	// Value is the value stored at the row and column pair.
	Value V
}

// Table maps a row key and column key pair to one value.
//
// The zero value is ready to use. Cells are stored in insertion order.
type Table[R comparable, C comparable, V any] struct {
	rows  map[R]map[C]V
	cells []Cell[R, C, V]
}

// New creates an empty Table.
func New[R comparable, C comparable, V any]() *Table[R, C, V] {
	return &Table[R, C, V]{rows: make(map[R]map[C]V)}
}

// NewWithCapacity creates an empty Table with room for capacity cells.
// Negative capacity is treated as zero.
func NewWithCapacity[R comparable, C comparable, V any](capacity int) *Table[R, C, V] {
	if capacity < 0 {
		capacity = 0
	}
	return &Table[R, C, V]{rows: make(map[R]map[C]V), cells: make([]Cell[R, C, V], 0, capacity)}
}

// Put inserts or updates a cell, returning its previous value if it existed.
// Updating a cell preserves its insertion position.
func (t *Table[R, C, V]) Put(row R, column C, value V) (old V, existed bool) {
	if t.rows == nil {
		t.rows = make(map[R]map[C]V)
	}
	columns, ok := t.rows[row]
	if !ok {
		columns = make(map[C]V)
		t.rows[row] = columns
	}
	if old, existed = columns[column]; existed {
		columns[column] = value
		for i := range t.cells {
			if t.cells[i].RowKey == row && t.cells[i].ColumnKey == column {
				t.cells[i].Value = value
				break
			}
		}
		return old, true
	}
	columns[column] = value
	t.cells = append(t.cells, Cell[R, C, V]{RowKey: row, ColumnKey: column, Value: value})
	return old, false
}

// Get returns the value at row and column.
func (t *Table[R, C, V]) Get(row R, column C) (V, bool) {
	if columns := t.rows[row]; columns != nil {
		value, ok := columns[column]
		return value, ok
	}
	var zero V
	return zero, false
}

// Contains reports whether a cell exists at row and column.
func (t *Table[R, C, V]) Contains(row R, column C) bool {
	_, ok := t.Get(row, column)
	return ok
}

// ContainsRow reports whether row has at least one cell.
func (t *Table[R, C, V]) ContainsRow(row R) bool { return len(t.rows[row]) != 0 }

// ContainsColumn reports whether column has at least one cell.
func (t *Table[R, C, V]) ContainsColumn(column C) bool {
	for _, cell := range t.cells {
		if cell.ColumnKey == column {
			return true
		}
	}
	return false
}

// Remove deletes the cell at row and column and returns its previous value.
func (t *Table[R, C, V]) Remove(row R, column C) (old V, removed bool) {
	columns := t.rows[row]
	if columns == nil {
		return old, false
	}
	old, removed = columns[column]
	if !removed {
		return old, false
	}
	delete(columns, column)
	if len(columns) == 0 {
		delete(t.rows, row)
	}
	for i, cell := range t.cells {
		if cell.RowKey == row && cell.ColumnKey == column {
			copy(t.cells[i:], t.cells[i+1:])
			var zero Cell[R, C, V]
			t.cells[len(t.cells)-1] = zero
			t.cells = t.cells[:len(t.cells)-1]
			break
		}
	}
	return old, true
}

// RemoveRow deletes row and returns a detached map of its cells.
func (t *Table[R, C, V]) RemoveRow(row R) map[C]V {
	columns := t.rows[row]
	if len(columns) == 0 {
		return nil
	}
	removed := make(map[C]V, len(columns))
	for column, value := range columns {
		removed[column] = value
	}
	delete(t.rows, row)
	write := 0
	for _, cell := range t.cells {
		if cell.RowKey != row {
			t.cells[write] = cell
			write++
		}
	}
	var zero Cell[R, C, V]
	for i := write; i < len(t.cells); i++ {
		t.cells[i] = zero
	}
	t.cells = t.cells[:write]
	return removed
}

// RemoveColumn deletes column and returns a detached map of its cells.
func (t *Table[R, C, V]) RemoveColumn(column C) map[R]V {
	removed := make(map[R]V)
	for _, cell := range t.cells {
		if cell.ColumnKey == column {
			removed[cell.RowKey] = cell.Value
		}
	}
	if len(removed) == 0 {
		return nil
	}
	for row := range removed {
		delete(t.rows[row], column)
		if len(t.rows[row]) == 0 {
			delete(t.rows, row)
		}
	}
	write := 0
	for _, cell := range t.cells {
		if cell.ColumnKey != column {
			t.cells[write] = cell
			write++
		}
	}
	var zero Cell[R, C, V]
	for i := write; i < len(t.cells); i++ {
		t.cells[i] = zero
	}
	t.cells = t.cells[:write]
	return removed
}

// Row returns a detached map of the values in row.
func (t *Table[R, C, V]) Row(row R) map[C]V {
	columns := t.rows[row]
	if len(columns) == 0 {
		return nil
	}
	out := make(map[C]V, len(columns))
	for column, value := range columns {
		out[column] = value
	}
	return out
}

// Column returns a detached map of the values in column.
func (t *Table[R, C, V]) Column(column C) map[R]V {
	var out map[R]V
	for _, cell := range t.cells {
		if cell.ColumnKey == column {
			if out == nil {
				out = make(map[R]V)
			}
			out[cell.RowKey] = cell.Value
		}
	}
	return out
}

// RowKeys returns row keys in the order their first cell was inserted.
func (t *Table[R, C, V]) RowKeys() []R {
	out := make([]R, 0, len(t.rows))
	seen := make(map[R]struct{}, len(t.rows))
	for _, cell := range t.cells {
		if _, ok := seen[cell.RowKey]; !ok {
			seen[cell.RowKey] = struct{}{}
			out = append(out, cell.RowKey)
		}
	}
	return out
}

// ColumnKeys returns column keys in the order their first cell was inserted.
func (t *Table[R, C, V]) ColumnKeys() []C {
	out := make([]C, 0)
	seen := make(map[C]struct{})
	for _, cell := range t.cells {
		if _, ok := seen[cell.ColumnKey]; !ok {
			seen[cell.ColumnKey] = struct{}{}
			out = append(out, cell.ColumnKey)
		}
	}
	return out
}

// Cells returns a detached slice of cells in insertion order.
func (t *Table[R, C, V]) Cells() []Cell[R, C, V] {
	return append([]Cell[R, C, V](nil), t.cells...)
}

// Values returns values in cell insertion order.
func (t *Table[R, C, V]) Values() []V {
	out := make([]V, 0, len(t.cells))
	for _, cell := range t.cells {
		out = append(out, cell.Value)
	}
	return out
}

// RowMap returns a detached nested map snapshot of the table.
func (t *Table[R, C, V]) RowMap() map[R]map[C]V {
	out := make(map[R]map[C]V, len(t.rows))
	for row, columns := range t.rows {
		copyColumns := make(map[C]V, len(columns))
		for column, value := range columns {
			copyColumns[column] = value
		}
		out[row] = copyColumns
	}
	return out
}

// ColumnMap returns a detached nested map snapshot keyed first by column.
func (t *Table[R, C, V]) ColumnMap() map[C]map[R]V {
	out := make(map[C]map[R]V)
	for _, cell := range t.cells {
		rows := out[cell.ColumnKey]
		if rows == nil {
			rows = make(map[R]V)
			out[cell.ColumnKey] = rows
		}
		rows[cell.RowKey] = cell.Value
	}
	return out
}

// PutAll copies all cells from other, preserving this table's existing cell
// positions and appending new cells in other's iteration order. A nil source is
// ignored.
func (t *Table[R, C, V]) PutAll(other *Table[R, C, V]) {
	if other == nil {
		return
	}
	for _, cell := range other.Cells() {
		t.Put(cell.RowKey, cell.ColumnKey, cell.Value)
	}
}

// All iterates over cells in insertion order. Mutating the table during
// iteration is unsupported.
func (t *Table[R, C, V]) All() iter.Seq[Cell[R, C, V]] {
	return func(yield func(Cell[R, C, V]) bool) {
		for _, cell := range t.cells {
			if !yield(cell) {
				return
			}
		}
	}
}

// Size returns the number of cells.
func (t *Table[R, C, V]) Size() int { return len(t.cells) }

// Len returns the number of cells.
func (t *Table[R, C, V]) Len() int { return len(t.cells) }

// IsEmpty reports whether the table has no cells.
func (t *Table[R, C, V]) IsEmpty() bool { return len(t.cells) == 0 }

// Clear removes every cell while keeping the allocated cell buffer.
func (t *Table[R, C, V]) Clear() {
	clear(t.rows)
	clear(t.cells)
	t.cells = t.cells[:0]
}
