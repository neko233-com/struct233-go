package ttlcache_test

import (
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/ttlcache"
)

func TestSweepAndKeysReadOneClockSnapshot(t *testing.T) {
	for _, operation := range []string{"Cleanup", "Keys"} {
		t.Run(operation, func(t *testing.T) {
			now := time.Unix(0, 0)
			calls := 0
			advance := false
			c := ttlcache.NewWithClock[int, int](4, time.Second, func() time.Time {
				calls++
				read := now
				if advance {
					now = now.Add(time.Second)
				}
				return read
			})
			for i := range 4 {
				c.Put(i, i)
			}
			calls = 0
			advance = true // subsequent reads would cross every entry's expiry
			if operation == "Cleanup" {
				if c.Cleanup() != 0 {
					t.Fatal("fresh entries were removed")
				}
			} else if len(c.Keys()) != 4 {
				t.Fatal("fresh entries were missing")
			}
			if calls != 1 {
				t.Fatalf("one scan called the clock %d times; want 1", calls)
			}
		})
	}
}
