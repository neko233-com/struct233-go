package order_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/order"
)

func ExampleReverse() {
	desc := order.Reverse(order.Natural[int]())
	fmt.Println(order.Sorted(desc, []int{3, 1, 4, 1, 5}))
	// Output:
	// [5 4 3 1 1]
}

func ExampleByKey() {
	type player struct {
		name  string
		score int
	}
	byScoreThenName := order.Then(
		order.Reverse(order.ByKey(func(p player) int { return p.score }, order.Natural[int]())),
		order.ByKey(func(p player) string { return p.name }, order.Natural[string]()),
	)

	roster := []player{{"ada", 10}, {"bob", 30}, {"cyd", 10}}
	for _, p := range order.Sorted(byScoreThenName, roster) {
		fmt.Println(p.name, p.score)
	}
	// Output:
	// bob 30
	// ada 10
	// cyd 10
}

func ExampleNullsFirst() {
	scores := []*int{new(int), nil, new(int)}
	*scores[0] = 7
	*scores[2] = 3

	for _, s := range order.Sorted(order.NullsFirst(order.Natural[int]()), scores) {
		if s == nil {
			fmt.Println("nil")
			continue
		}
		fmt.Println(*s)
	}
	// Output:
	// nil
	// 3
	// 7
}

func ExampleMax() {
	cmp := order.Natural[int]()
	fmt.Println(order.Max(cmp, 3, 9), order.Min(cmp, 3, 9))

	words := []string{"pear", "fig", "banana"}
	longest, _ := order.MaxOf(order.ByKey(func(s string) int { return len(s) }, cmp), words)
	fmt.Println(longest)
	// Output:
	// 9 3
	// banana
}
