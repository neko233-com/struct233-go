package table_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/table"
)

func ExampleTable() {
	t := table.New[string, string, int]()
	t.Put("row1", "col1", 1)
	t.Put("row1", "col2", 2)
	t.Put("row2", "col1", 3)

	value, ok := t.Get("row1", "col2")
	fmt.Println(value, ok)
	fmt.Println(t.Cells())
	// Output:
	// 2 true
	// [{row1 col1 1} {row1 col2 2} {row2 col1 3}]
}

func ExampleTable_Put() {
	t := table.New[string, string, int]()
	t.Put("r", "c", 1)
	old, existed := t.Put("r", "c", 2) // updating keeps the cell position
	fmt.Println(old, existed)

	t.Remove("r", "c")
	t.Put("r", "c", 3) // re-adding moves the cell to the end
	fmt.Println(t.Values())
	// Output:
	// 1 true
	// [3]
}

func ExampleTable_RemoveColumn() {
	t := table.New[string, string, int]()
	t.Put("row1", "col1", 1)
	t.Put("row2", "col1", 2)
	t.Put("row2", "col2", 3)

	fmt.Println(t.RemoveColumn("col1"))
	fmt.Println(t.Cells())
	// Output:
	// map[row1:1 row2:2]
	// [{row2 col2 3}]
}

func ExampleTable_ColumnKeys() {
	t := table.New[string, string, int]()
	t.Put("row1", "col1", 1)
	t.Put("row2", "col1", 2)
	t.Put("row1", "col2", 3)

	fmt.Println(t.ColumnKeys())
	fmt.Println(t.RowMap())
	// Output:
	// [col1 col2]
	// map[row1:map[col1:1 col2:3] row2:map[col1:2]]
}
