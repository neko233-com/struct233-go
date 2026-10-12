package latch_test

import (
	"context"
	"fmt"

	"github.com/neko233-com/struct233-go/latch"
)

func ExampleLatch() {
	l := latch.New(2)
	fmt.Println(l.Count(), l.Open())

	l.CountDown()
	l.CountDown() // reaches zero: the latch opens
	fmt.Println(l.Count(), l.Open())

	// extra CountDown calls past zero are no-ops
	l.CountDown()
	fmt.Println(l.Count(), l.Open())
	// Output:
	// 2 false
	// 0 true
	// 0 true
}

func ExampleLatch_Await() {
	l := latch.New(1)

	// a canceled context wins on a closed latch...
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fmt.Println(l.Await(ctx))

	// ...but the open fast path ignores the context entirely
	l.CountDown()
	fmt.Println(l.Await(ctx))
	// Output:
	// context canceled
	// <nil>
}

func ExampleLatch_AwaitTimeout() {
	l := latch.New(1)
	fmt.Println(l.AwaitTimeout(0)) // not open yet

	l.CountDown()
	fmt.Println(l.AwaitTimeout(0)) // open: returns true at once
	// Output:
	// false
	// true
}
