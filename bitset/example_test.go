package bitset_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/bitset"
)

func ExampleBitSet() {
	b := bitset.New()
	b.Set(1)
	b.Set(4)
	b.Set(64)
	fmt.Println(b.Get(4), b.Get(5))
	fmt.Println(b.ToSlice())
	fmt.Println(b.Cardinality(), b.Len())
	// Output:
	// true false
	// [1 4 64]
	// 3 65
}

func ExampleBitSet_Or() {
	a := bitset.New()
	a.Set(1)
	a.Set(2)
	b := bitset.New()
	b.Set(2)
	b.Set(3)
	a.Or(b)
	fmt.Println(a.ToSlice())
	a.AndNot(b)
	fmt.Println(a.ToSlice())
	// Output:
	// [1 2 3]
	// [1]
}

func ExampleBitSet_NextSetBit() {
	b := bitset.New()
	b.Set(3)
	b.Set(70)
	for i := b.NextSetBit(0); i >= 0; i = b.NextSetBit(i + 1) {
		fmt.Println(i)
	}
	fmt.Println(b.PrevSetBit(100))
	// Output:
	// 3
	// 70
	// 70
}

func ExampleBitSet_Flip() {
	b := bitset.New()
	b.Set(5)
	b.Flip(5) // toggles off
	b.Flip(6) // toggles on
	fmt.Println(b.ToSlice())
	// Output:
	// [6]
}
