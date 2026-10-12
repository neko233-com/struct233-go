package stopwatch_test

import (
	"fmt"
	"time"

	"github.com/neko233-com/struct233-go/stopwatch"
)

// ExampleStopwatch shows the full lifecycle with the fakeClock shared with
// stopwatch_test.go, so the example runs without sleeping and prints exact
// values.
func ExampleStopwatch() {
	clock := newFakeClock()
	sw := stopwatch.New(stopwatch.WithClock(clock.Now))

	sw.Start() // measure the first burst of work
	clock.Advance(1500 * time.Millisecond)
	sw.Stop()

	fmt.Println(sw.Elapsed()) // the gap below is free: only running time counts
	clock.Advance(time.Hour)
	fmt.Println(sw.IsRunning())

	sw.Restart() // clear the total and keep measuring
	clock.Advance(250 * time.Millisecond)
	fmt.Println(sw)

	// Output:
	// 1.5s
	// false
	// 250ms (running)
}
