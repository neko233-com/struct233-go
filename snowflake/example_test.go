package snowflake_test

import (
	"fmt"
	"time"

	"github.com/neko233-com/struct233-go/snowflake"
)

func ExampleGenerator() {
	epoch := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	g := snowflake.New(7,
		snowflake.WithEpoch(epoch),
		snowflake.WithClock(func() time.Time { return epoch.Add(1500 * time.Millisecond) }))

	id := g.MustNext()
	fmt.Println(g.NodeBits(), g.Node())
	fmt.Println(g.NodeOf(id), g.SequenceOf(id))
	fmt.Println(g.TimeOf(id).Sub(epoch))
	// Output:
	// 10 7
	// 7 0
	// 1.5s
}

func ExampleGenerator_Next_sequenceWithinMillisecond() {
	epoch := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	// a frozen clock: every id of this millisecond shares one timestamp and
	// the sequence field advances
	g := snowflake.New(1,
		snowflake.WithEpoch(epoch),
		snowflake.WithClock(func() time.Time { return epoch.Add(time.Second) }))

	first, _ := g.Next()
	second, _ := g.Next()
	fmt.Println(g.SequenceOf(first), g.SequenceOf(second))
	fmt.Println(g.TimeOf(first).Equal(g.TimeOf(second)))
	fmt.Println(first.Int64() < second.Int64())
	// Output:
	// 0 1
	// true
	// true
}

func ExampleGenerator_clockRollback() {
	epoch := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	now := epoch.Add(time.Second)
	g := snowflake.New(3,
		snowflake.WithEpoch(epoch),
		snowflake.WithClock(func() time.Time { return now }))

	g.MustNext()
	now = epoch // the clock jumped backwards
	_, err := g.Next()
	fmt.Println(err)
	// Output:
	// snowflake: clock moved backwards
}
