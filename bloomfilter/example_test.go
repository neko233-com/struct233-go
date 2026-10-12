package bloomfilter_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/bloomfilter"
)

func ExampleBloomFilter() {
	b := bloomfilter.New(1000, 0.01)
	b.AddString("player-42")
	b.AddString("player-43")
	fmt.Println(b.TestString("player-42"), b.TestString("player-99"))
	fmt.Println(b.Count(), b.BitLen() > 0, b.K() > 0)
	// Output:
	// true false
	// 2 true true
}

func ExampleBloomFilter_Clear() {
	b := bloomfilter.New(1000, 0.01)
	b.AddUint64(7)
	fmt.Println(b.TestUint64(7), b.Count())
	b.Clear()
	fmt.Println(b.TestUint64(7), b.Count())
	// Output:
	// true 1
	// false 0
}

func ExampleBloomFilter_testBytes() {
	b := bloomfilter.New(1000, 0.01)
	token := []byte("network-session-token")
	b.AddBytes(token)
	fmt.Println(b.TestBytes(token))
	fmt.Println(b.TestBytes([]byte("another-token")))
	// Output:
	// true
	// false
}
