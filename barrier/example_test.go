package barrier_test

import (
	"context"
	"fmt"

	"github.com/neko233-com/struct233-go/barrier"
)

func ExampleBarrier() {
	b := barrier.New(2)
	fmt.Println(b.Parties(), b.Waiting(), b.Broken())

	// two parties arrive; the barrier is cyclic, so the next round starts
	// immediately. Wait for the first party to park so the arrival order
	// (and therefore the reported index) is deterministic.
	go b.Await(context.Background())
	for b.Waiting() != 1 {
	}
	index, err := b.Await(context.Background())
	fmt.Println(index, err)
	// Output:
	// 2 0 false
	// 1 <nil>
}

func ExampleBarrier_Await_cancelled() {
	b := barrier.New(2)

	// the first party parks; a cancelled arrival breaks the generation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := b.Await(ctx)
	fmt.Println(err == context.Canceled)

	// the broken barrier rejects arrivals until Reset
	_, err = b.Await(context.Background())
	fmt.Println(err)
	b.Reset()
	fmt.Println(b.Broken())
	// Output:
	// true
	// barrier: broken
	// false
}

func ExampleBarrier_Reset() {
	b := barrier.New(2)

	// break the generation with an already-cancelled context...
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = b.Await(ctx)

	// ...then repair it: a fresh generation rendezvous works again
	b.Reset()
	go b.Await(context.Background())
	for b.Waiting() != 1 {
	}
	index, err := b.Await(context.Background())
	fmt.Println(index, err, b.Broken())
	// Output:
	// 1 <nil> false
}
