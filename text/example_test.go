package text_test

import (
	"fmt"

	"github.com/neko233-com/struct233-go/text"
)

func ExampleNewJoiner() {
	plain := text.NewJoiner(", ").Join("ada", "", "grace")
	skipped := text.NewJoiner(", ").SkipEmpty().Join("ada", "", "grace")
	replaced := text.NewJoiner(", ").UseForEmpty("-").Join("ada", "", "grace")

	fmt.Println(plain)
	fmt.Println(skipped)
	fmt.Println(replaced)
	// Output:
	// ada, , grace
	// ada, grace
	// ada, -, grace
}

func ExampleNewSplitter() {
	trimmed := text.NewSplitter(",").Trim().OmitEmpty().Split(" ada , , grace ")
	capped := text.NewSplitter(",").Limit(2).Split("a,b,c")

	fmt.Println(trimmed)
	fmt.Println(capped)
	// Output:
	// [ada grace]
	// [a b,c]
}

func ExampleToSnakeCase() {
	fmt.Println(text.ToSnakeCase("userID"))
	fmt.Println(text.ToSnakeCase("HTTPServer"))
	fmt.Println(text.ToLowerCamel("v2_api"))
	// Output:
	// user_id
	// http_server
	// v2Api
}

func ExampleTruncate() {
	fmt.Println(text.Truncate("hello world", 8, "…"))
	fmt.Println(text.Truncate("hello", 5, "…"))
	// Output:
	// hello…
	// hello
}

func ExampleJoinMap() {
	m := map[string]string{"b": "2", "a": "1"}

	fmt.Println(text.JoinMap(m, "&", "="))
	// Output:
	// a=1&b=2
}
