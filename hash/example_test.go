package hash_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/hash"
)

// Example shows the default hash choosing a node in a consistent hash ring.
func Example() {
	key := []byte("user:42")
	nodes := []string{"node-a", "node-b", "node-c"}
	fmt.Println(nodes[hash.Sum64(key)%uint64(len(nodes))])
	// Output:
	// node-a
}

// ExampleSum64 shows that the default 64-bit hash is the first half of the
// seedless x64 128-bit MurmurHash3.
func ExampleSum64() {
	key := []byte("user:42")
	h1, _ := hash.Murmur3_128(key, 0)
	fmt.Println(hash.Sum64(key) == h1)
	// Output:
	// true
}

// ExampleSum32 shows that the default 32-bit hash is seedless
// MurmurHash3_x86_32.
func ExampleSum32() {
	key := []byte("user:42")
	fmt.Println(hash.Sum32(key) == hash.Murmur3_32(key, 0))
	// Output:
	// true
}

// ExampleHasher shows the Hasher signature a ring can accept.
func ExampleHasher() {
	var ring hash.Hasher = hash.Sum64
	fmt.Println(ring([]byte("user:42")) == hash.Sum64([]byte("user:42")))
	// Output:
	// true
}

// ExampleFNV1a64 prints the 64-bit FNV-1a hash of "hello".
func ExampleFNV1a64() {
	fmt.Println(hash.FNV1a64([]byte("hello")))
	// Output:
	// 11831194018420276491
}

// ExampleCRC32 prints the IEEE CRC-32 check value of "123456789".
func ExampleCRC32() {
	fmt.Printf("%08x\n", hash.CRC32([]byte("123456789")))
	// Output:
	// cbf43926
}

// ExampleCRC32C prints the Castagnoli CRC-32 check value of "123456789".
func ExampleCRC32C() {
	fmt.Printf("%08x\n", hash.CRC32C([]byte("123456789")))
	// Output:
	// e3069283
}

// ExampleAdler32 prints the Adler-32 checksum of "Wikipedia".
func ExampleAdler32() {
	fmt.Printf("%08x\n", hash.Adler32([]byte("Wikipedia")))
	// Output:
	// 11e60398
}
