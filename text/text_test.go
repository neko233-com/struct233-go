package text_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/text"
)

func TestJoinAndJoinSlice(t *testing.T) {
	cases := []struct {
		name string
		sep  string
		in   []string
		want string
	}{
		{"no elements", ",", nil, ""},
		{"one element", ",", []string{"a"}, "a"},
		{"two elements", ",", []string{"a", "b"}, "a,b"},
		{"empty separator", "", []string{"a", "b"}, "ab"},
		{"multi byte separator", " → ", []string{"a", "b"}, "a → b"},
		{"empty values are kept", ",", []string{"a", "", "b"}, "a,,b"},
		{"all empty", ",", []string{"", ""}, ","},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, text.Join(tc.sep, tc.in...))
			require.Equal(t, tc.want, text.JoinSlice(tc.sep, tc.in))
		})
	}
}

func TestJoinAny(t *testing.T) {
	require.Equal(t, "a=1,b=2", text.JoinAny(",", "a=1", "b=2"))
	require.Equal(t, "1 true x", text.JoinAny(" ", 1, true, "x"))
	require.Empty(t, text.JoinAny(","), "no elements means an empty string")
}

func TestJoinMapIsDeterministic(t *testing.T) {
	m := map[string]string{"b": "2", "a": "1", "c": "3"}
	for range 20 {
		require.Equal(t, "a=1&b=2&c=3", text.JoinMap(m, "&", "="), "sorted key order must be stable")
	}
	require.Empty(t, text.JoinMap(nil, "&", "="))
	require.Equal(t, "k:v", text.JoinMap(map[string]string{"k": "v"}, "&", ":"))
}

func TestJoinerOptions(t *testing.T) {
	cases := []struct {
		name  string
		build func() *text.Joiner
		in    []string
		want  string
	}{
		{"plain", func() *text.Joiner { return text.NewJoiner(", ") }, []string{"ada", "", "grace"}, "ada, , grace"},
		{"skip empty", func() *text.Joiner { return text.NewJoiner(", ").SkipEmpty() }, []string{"ada", "", "grace"}, "ada, grace"},
		{"skip empty with one survivor", func() *text.Joiner { return text.NewJoiner(",").SkipEmpty() }, []string{"", "a"}, "a"},
		{"skip empty with all empty", func() *text.Joiner { return text.NewJoiner(",").SkipEmpty() }, []string{"", ""}, ""},
		{"skip empty with no elements", func() *text.Joiner { return text.NewJoiner(",").SkipEmpty() }, nil, ""},
		{"use for empty", func() *text.Joiner { return text.NewJoiner(", ").UseForEmpty("-") }, []string{"ada", "", "grace"}, "ada, -, grace"},
		{"use for empty with empty separator", func() *text.Joiner { return text.NewJoiner("").UseForEmpty("?") }, []string{"", ""}, "??"},
		{"use for empty leaves text alone", func() *text.Joiner { return text.NewJoiner(",").UseForEmpty("-") }, []string{"a", "b"}, "a,b"},
		{"empty replacement", func() *text.Joiner { return text.NewJoiner(",").UseForEmpty("") }, []string{"a", "", "b"}, "a,,b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.build().Join(tc.in...))
			require.Equal(t, tc.want, tc.build().JoinSlice(tc.in))
		})
	}
}

func TestJoinerOptionsAreMutuallyExclusive(t *testing.T) {
	require.Equal(t, "a,-,b", text.NewJoiner(",").SkipEmpty().UseForEmpty("-").Join("a", "", "b"),
		"UseForEmpty must clear SkipEmpty")
	require.Equal(t, "a,b", text.NewJoiner(",").UseForEmpty("-").SkipEmpty().Join("a", "", "b"),
		"SkipEmpty must clear UseForEmpty")
}

func TestFluentTypesChainOnTheSameInstance(t *testing.T) {
	j := text.NewJoiner(",")
	require.Same(t, j, j.SkipEmpty())
	require.Same(t, j, j.UseForEmpty("-"))

	s := text.NewSplitter(",")
	require.Same(t, s, s.Trim())
	require.Same(t, s, s.OmitEmpty())
	require.Same(t, s, s.Limit(2))
}

func TestSplitterOptions(t *testing.T) {
	cases := []struct {
		name  string
		build func() *text.Splitter
		in    string
		want  []string
	}{
		{"plain", func() *text.Splitter { return text.NewSplitter(",") }, "a,b", []string{"a", "b"}},
		{"keeps empty parts", func() *text.Splitter { return text.NewSplitter(",") }, "a,,b", []string{"a", "", "b"}},
		{"omit empty", func() *text.Splitter { return text.NewSplitter(",").OmitEmpty() }, "a,,b,", []string{"a", "b"}},
		{"trim", func() *text.Splitter { return text.NewSplitter(",").Trim() }, " a , b ", []string{"a", "b"}},
		{"trim then omit", func() *text.Splitter { return text.NewSplitter(",").Trim().OmitEmpty() }, " a , , b ", []string{"a", "b"}},
		{"omit keeps blanks that hold text", func() *text.Splitter { return text.NewSplitter(",").OmitEmpty() }, "a, , b", []string{"a", " ", " b"}},
		{"no match", func() *text.Splitter { return text.NewSplitter(";") }, "a,b", []string{"a,b"}},
		{"empty input", func() *text.Splitter { return text.NewSplitter(",").OmitEmpty() }, "", []string{}},
		{"empty input without omit", func() *text.Splitter { return text.NewSplitter(",") }, "", []string{""}},
		{"consecutive separators", func() *text.Splitter { return text.NewSplitter("::") }, "a::::b", []string{"a", "", "b"}},
		{"multi byte separator", func() *text.Splitter { return text.NewSplitter(" → ") }, "a → b → c", []string{"a", "b", "c"}},
		{"empty separator splits runes", func() *text.Splitter { return text.NewSplitter("") }, "héllo", []string{"h", "é", "l", "l", "o"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.build().Split(tc.in))
		})
	}
}

func TestSplitterLimit(t *testing.T) {
	cases := []struct {
		name  string
		limit int
		in    string
		want  []string
	}{
		{"limit 1 keeps everything", 1, "a,b,c", []string{"a,b,c"}},
		{"limit 2", 2, "a,b,c", []string{"a", "b,c"}},
		{"limit equals the part count", 3, "a,b,c", []string{"a", "b", "c"}},
		{"limit above the part count", 9, "a,b,c", []string{"a", "b", "c"}},
		{"limit on a single part", 2, "abc", []string{"abc"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, text.NewSplitter(",").Limit(tc.limit).Split(tc.in))
		})
	}

	t.Run("the cap is applied before trim and omit empty", func(t *testing.T) {
		got := text.NewSplitter(",").Limit(3).Trim().OmitEmpty().Split(" a , , b , c ")
		require.Equal(t, []string{"a", "b , c"}, got,
			"the capped tail is trimmed as one part and a dropped part does not free a slot")
	})

	t.Run("a limit below one panics", func(t *testing.T) {
		require.Panics(t, func() { text.NewSplitter(",").Limit(0) })
		require.Panics(t, func() { text.NewSplitter(",").Limit(-1) })
	})
}

func TestBlankPredicates(t *testing.T) {
	cases := []struct {
		in           string
		empty, blank bool
	}{
		{"", true, true},
		{" ", false, true},
		{"\t\n", false, true},
		{"\u00a0", false, true}, // a non-breaking space is white space for TrimSpace
		{"x", false, false},
		{" x ", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.empty, text.IsNullOrEmpty(tc.in))
			require.Equal(t, tc.blank, text.IsBlank(tc.in))
			require.Equal(t, tc.blank, text.IsNullOrBlank(tc.in))
		})
	}
}

func TestRepeat(t *testing.T) {
	require.Equal(t, "", text.Repeat("ab", 0))
	require.Equal(t, "ab", text.Repeat("ab", 1))
	require.Equal(t, "ababab", text.Repeat("ab", 3))
	require.Equal(t, "", text.Repeat("", 10))
	require.Panics(t, func() { text.Repeat("ab", -1) })
}

func TestPadding(t *testing.T) {
	cases := []struct {
		name   string
		start  bool
		in     string
		minLen int
		pad    byte
		want   string
	}{
		{"start pads", true, "42", 5, '0', "00042"},
		{"start is long enough", true, "000042", 5, '0', "000042"},
		{"start is exactly long enough", true, "12345", 5, '0', "12345"},
		{"start with zero budget", true, "abc", 0, '0', "abc"},
		{"start with empty input", true, "", 3, '0', "000"},
		{"end pads", false, "42", 5, ' ', "42   "},
		{"end is long enough", false, "42   ", 2, ' ', "42   "},
		{"end with empty input", false, "", 3, '.', "..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			if tc.start {
				got = text.PadStart(tc.in, tc.minLen, tc.pad)
			} else {
				got = text.PadEnd(tc.in, tc.minLen, tc.pad)
			}
			require.Equal(t, tc.want, got)
		})
	}

	t.Run("byte semantics", func(t *testing.T) {
		require.Equal(t, "[é", text.PadStart("é", 3, '['),
			"é is two bytes, so a three byte budget needs one pad byte")
	})
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		maxLen   int
		ellipsis string
		want     string
	}{
		{"fits", "hello", 8, "…", "hello"},
		{"exact", "hello", 5, "…", "hello"},
		{"ellipsis appended", "hello world", 8, "…", "hello…"},
		{"ascii ellipsis", "hello world", 8, "...", "hello..."},
		{"zero budget", "hello", 0, "…", ""},
		{"ellipsis longer than the budget", "hello", 2, "...", ".."},
		{"ellipsis exactly the budget", "hello", 3, "...", "..."},
		{"empty input", "", 0, "…", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := text.Truncate(tc.in, tc.maxLen, tc.ellipsis)
			require.Equal(t, tc.want, got)
			require.LessOrEqual(t, len(got), tc.maxLen, "the result must fit the budget")
		})
	}

	require.Panics(t, func() { text.Truncate("x", -1, "…") })
}

func TestCaseConversions(t *testing.T) {
	cases := []struct {
		in                                   string
		lowerCamel, upperCamel, snake, kebab string
	}{
		{"userID", "userId", "UserId", "user_id", "user-id"},
		{"user_id", "userId", "UserId", "user_id", "user-id"},
		{"user-id", "userId", "UserId", "user_id", "user-id"},
		{"user id", "userId", "UserId", "user_id", "user-id"},
		{"HTTPServer", "httpServer", "HttpServer", "http_server", "http-server"},
		{"httpserver", "httpserver", "Httpserver", "httpserver", "httpserver"},
		{"user2FA", "user2fa", "User2fa", "user2fa", "user2fa"},
		{"v2_api", "v2Api", "V2Api", "v2_api", "v2-api"},
		{"", "", "", "", ""},
		{"___", "", "", "", ""},
		{"  spaced  out ", "spacedOut", "SpacedOut", "spaced_out", "spaced-out"},
		{"a", "a", "A", "a", "a"},
		{"A", "a", "A", "a", "a"},
		// a digit never opens a word, so "ID2Name" stays one word
		{"userID2Name", "userId2name", "UserId2name", "user_id2name", "user-id2name"},
		{"a_b_c", "aBC", "ABC", "a_b_c", "a-b-c"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.lowerCamel, text.ToLowerCamel(tc.in))
			require.Equal(t, tc.upperCamel, text.ToUpperCamel(tc.in))
			require.Equal(t, tc.snake, text.ToSnakeCase(tc.in))
			require.Equal(t, tc.kebab, text.ToKebabCase(tc.in))
		})
	}
}

func TestCaseConversionsAreIdempotentForTheirOwnFormat(t *testing.T) {
	inputs := []string{"userID", "HTTPServer", "v2_api", "user2FA", "a_b_c", "  spaced  out ", "a", "A", ""}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			snake := text.ToSnakeCase(in)
			require.Equal(t, snake, text.ToSnakeCase(snake))
			kebab := text.ToKebabCase(in)
			require.Equal(t, kebab, text.ToKebabCase(kebab))

			// snake(camel(x)) is not always snake(x): camelCase cannot carry the
			// word boundary between adjacent capitalised words ("a_b_c" becomes
			// "aBC", which splits back into "a" and "BC"). What must hold is that
			// the composition is a fixed point, so repeated conversion is stable.
			fromCamel := text.ToSnakeCase(text.ToLowerCamel(in))
			require.Equal(t, fromCamel, text.ToSnakeCase(fromCamel))
			fromUpper := text.ToSnakeCase(text.ToUpperCamel(in))
			require.Equal(t, fromUpper, text.ToSnakeCase(fromUpper))
		})
	}
}

func TestCamelToSnakeIsLossyForAdjacentCapitals(t *testing.T) {
	// documented consequence of the word-splitting rules: lowerCamelCase merges
	// the two single-letter words, and a digit glues to the word in front, so the
	// first camel round is the one that stabilises
	require.Equal(t, "a_bc", text.ToSnakeCase(text.ToLowerCamel("a_b_c")))
	require.Equal(t, "v2api", text.ToSnakeCase(text.ToLowerCamel("v2_api")))
	require.Equal(t, "v2api", text.ToSnakeCase("v2api"))
}

func TestCaseConversionConvergesForHostileInput(t *testing.T) {
	// found by FuzzCaseConversions: an invalid UTF-8 byte decodes to U+FFFD and
	// U+03D4 is upper case without a lower-case mapping, so the first pass still
	// exposes a boundary and the second one settles it
	hostile := "0\xc5A\u03d4"

	first := text.ToSnakeCase(hostile)
	second := text.ToSnakeCase(first)
	require.Equal(t, second, text.ToSnakeCase(second), "the second pass must be a fixed point")
	require.Equal(t, text.ToKebabCase(hostile), text.ToKebabCase(hostile), "the conversion is deterministic")

	// invalid bytes are replaced rather than kept
	require.Contains(t, first, "\uFFFD")
}

func TestEmptyInputEdgeCases(t *testing.T) {
	require.Equal(t, "ab", text.NewJoiner("").Join("a", "b"))
	require.Equal(t, []string{"a", "b"}, text.NewSplitter("").Split("ab"), "an empty separator cuts between runes")
	require.Equal(t, []string{""}, text.Split("", ","), "the helper is a plain split")
	require.Empty(t, text.Split("", ",")[0])
}
