package ttlcache_test

import (
	"fmt"
	"time"

	"github.com/neko233-com/struct233-go/ttlcache"
)

func ExampleCache() {
	c := ttlcache.NewWithClock[string, int](2, time.Minute, newFakeClock().Now)
	c.Put("a", 1)
	c.Put("b", 2)
	v, ok := c.Get("a")
	fmt.Println(v, ok, c.Len())
	c.Put("c", 3) // capacity 2: b is least recently used and gets evicted
	fmt.Println(c.Contains("b"), c.Contains("c"), c.Keys())
	// Output:
	// 1 true 2
	// false true [a c]
}

func ExampleCache_PutWithTTL() {
	clock := newFakeClock()
	c := ttlcache.NewWithClock[string, int](4, time.Minute, clock.Now)
	c.PutWithTTL("short", 1, 5*time.Second)
	c.PutWithTTL("long", 2, time.Hour)
	clock.Advance(6 * time.Second)
	_, shortOK := c.Get("short")
	long, longOK := c.Get("long")
	fmt.Println(shortOK, long, longOK)
	// a non-positive TTL removes the entry instead of storing it
	c.PutWithTTL("long", 3, 0)
	fmt.Println(c.Contains("long"))
	// Output:
	// false 2 true
	// false
}

func ExampleCache_Cleanup() {
	clock := newFakeClock()
	c := ttlcache.NewWithClock[string, int](4, 10*time.Second, clock.Now)
	c.Put("a", 1)
	c.Put("b", 2)
	clock.Advance(11 * time.Second) // both entries are now expired
	fmt.Println(c.Cleanup(), c.Len())
	// Output:
	// 2 0
}
