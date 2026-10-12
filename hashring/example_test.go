package hashring_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/hash"
	"github.com/neko233-com/struct233-go/hashring"
)

// ExampleNew shows the defaults, the options and the ring's own counters.
// hash.Sum64 is the default hasher; it is passed explicitly to keep the output
// stable even if that default ever changes.
func ExampleNew() {
	ring := hashring.New(
		hashring.WithReplicas(64),
		hashring.WithHasher(hash.Sum64),
	)
	ring.Add("cache-a", "cache-b")

	fmt.Println("replicas:", ring.Replicas())
	fmt.Println("points:", ring.Len())
	fmt.Println("nodes:", ring.Nodes())

	// Output:
	// replicas: 64
	// points: 128
	// nodes: [cache-a cache-b]
}

// ExampleRing_Get shows that a key always lands on the same node while the node
// set is unchanged.
func ExampleRing_Get() {
	ring := hashring.New(hashring.WithHasher(hash.Sum64))
	ring.Add("cache-a", "cache-b", "cache-c")

	for _, key := range []string{"user:1", "user:2", "user:3"} {
		node, ok := ring.Get(key)
		fmt.Printf("%s -> %s (%v)\n", key, node, ok)
	}

	// Output:
	// user:1 -> cache-c (true)
	// user:2 -> cache-b (true)
	// user:3 -> cache-b (true)
}

// ExampleRing_GetN shows the primary node plus its ring successors, which is
// what a replicated lookup walks.
func ExampleRing_GetN() {
	ring := hashring.New(hashring.WithReplicas(8), hashring.WithHasher(hash.Sum64))
	ring.Add("cache-a", "cache-b", "cache-c")

	fmt.Println("one:", ring.GetN("user:7", 1))
	fmt.Println("two:", ring.GetN("user:7", 2))
	fmt.Println("all:", ring.GetN("user:7", 3))
	fmt.Println("clamped:", ring.GetN("user:7", 99))
	fmt.Println("none:", ring.GetN("user:7", 0))

	// Output:
	// one: [cache-a]
	// two: [cache-a cache-b]
	// all: [cache-a cache-b cache-c]
	// clamped: [cache-a cache-b cache-c]
	// none: []
}

// ExampleRing_Add shows the property that motivates a hash ring: adding a node
// takes over part of the keyspace, and removing it again is a no-op for every
// other key.
func ExampleRing_Add() {
	ring := hashring.New(hashring.WithReplicas(64), hashring.WithHasher(hash.Sum64))
	ring.Add("cache-a", "cache-b", "cache-c")

	const keys = 1000
	before := make(map[string]string, keys)
	for i := range keys {
		key := fmt.Sprintf("user:%d", i)
		node, _ := ring.Get(key)
		before[key] = node
	}

	ring.Add("cache-d")

	moved, toNew := 0, 0
	for key, old := range before {
		node, _ := ring.Get(key)
		if node == old {
			continue
		}
		moved++
		if node == "cache-d" {
			toNew++
		}
	}
	fmt.Println("moved:", moved)
	fmt.Println("all moves went to the new node:", moved == toNew)

	ring.Remove("cache-d")
	same := true
	for key, old := range before {
		if node, _ := ring.Get(key); node != old {
			same = false
		}
	}
	fmt.Println("removing cache-d restored every key:", same)
	fmt.Println("nodes again:", ring.Nodes())

	// Output:
	// moved: 228
	// all moves went to the new node: true
	// removing cache-d restored every key: true
	// nodes again: [cache-a cache-b cache-c]
}
