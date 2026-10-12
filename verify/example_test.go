package verify_test

import (
	"errors"
	"fmt"

	"github.com/neko233-com/struct233-go/verify"
)

func Example() {
	area := func(w, h int) int {
		verify.Positive(w, "width %d must be positive", w)
		verify.Positive(h, "height %d must be positive", h)
		return w * h
	}
	fmt.Println(area(3, 4))

	defer func() {
		if r := recover(); r != nil {
			err := r.(error)
			fmt.Println(err)
			fmt.Println(errors.Is(err, verify.ErrInvalidArgument))
		}
	}()
	area(-1, 2)
	// Output:
	// 12
	// invalid argument: width -1 must be positive
	// true
}

func ExampleIndex() {
	names := []string{"a", "b", "c"}

	get := func(i int) string {
		verify.Index(i, len(names), "index %d out of range [0, %d)", i, len(names))
		return names[i]
	}
	fmt.Println(get(1))

	defer func() {
		err := recover().(error)
		fmt.Println(err)
		fmt.Println(errors.Is(err, verify.ErrInvalidArgument))
	}()
	get(3)
	// Output:
	// b
	// invalid argument: index 3 out of range [0, 3)
	// true
}

func ExampleState() {
	defer func() {
		err := recover().(error)
		fmt.Println(err)
		fmt.Println(errors.Is(err, verify.ErrInvalidState))
	}()

	started := false
	verify.State(started, "server not started")
	// Output:
	// invalid state: server not started
	// true
}
