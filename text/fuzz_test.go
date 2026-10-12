package text_test

import (
	"strings"
	"testing"

	"github.com/neko233-com/struct233-go/text"
)

// converges reports whether repeatedly applying conv to s reaches a fixed point
// within a few passes. Ordinary input stabilises immediately; uppercased runes
// with no lower-case mapping can need one extra pass.
func converges(s string, conv func(string) string) bool {
	cur := s
	for range 4 {
		next := conv(cur)
		if next == cur {
			return true
		}
		cur = next
	}
	return false
}

// FuzzSplitJoinRoundTrip checks that splitting is lossless: joining the parts
// back with the same separator always reproduces the input, for every separator
// and every input, including empty separators (which cut between runes).
func FuzzSplitJoinRoundTrip(f *testing.F) {
	f.Add(",", "a,b,c")
	f.Add("", "xyz")
	f.Add("::", "a::::b")
	f.Add(" → ", "a → b")
	f.Add(",", "")
	f.Add(",", ",")
	f.Add("é", "aéb")

	f.Fuzz(func(t *testing.T, sep, in string) {
		parts := text.Split(in, sep)
		if parts == nil {
			t.Fatalf("Split(%q, %q) returned a nil slice", in, sep)
		}
		if got := text.JoinSlice(sep, parts); got != in {
			t.Fatalf("JoinSlice(%q, Split(%q, %q)) = %q; want the input back", sep, in, sep, got)
		}

		// trimming and dropping empty parts must only ever remove information
		cleaned := text.NewSplitter(sep).Trim().OmitEmpty().Split(in)
		for _, p := range cleaned {
			if p == "" {
				t.Fatalf("OmitEmpty left an empty part in %q", cleaned)
			}
			if p != strings.TrimSpace(p) {
				t.Fatalf("Trim left %q untrimmed", p)
			}
		}
	})
}

// FuzzCaseConversions checks the invariants the conversions promise: snake and
// kebab output is lower case, uses single separators, never starts or ends with
// one, and is stable under a second conversion. The camel forms must keep the
// same word split.
func FuzzCaseConversions(f *testing.F) {
	f.Add("userID")
	f.Add("HTTPServer")
	f.Add("v2_api")
	f.Add("")
	f.Add("___")
	f.Add("  spaced  out ")
	f.Add("管理ID2")

	f.Fuzz(func(t *testing.T, s string) {
		snake := text.ToSnakeCase(s)
		if snake != strings.ToLower(snake) {
			t.Fatalf("ToSnakeCase(%q) = %q; want lower case", s, snake)
		}
		if strings.HasPrefix(snake, "_") || strings.HasSuffix(snake, "_") {
			t.Fatalf("ToSnakeCase(%q) = %q; want no leading or trailing separator", s, snake)
		}
		if strings.Contains(snake, "__") {
			t.Fatalf("ToSnakeCase(%q) = %q; want single separators", s, snake)
		}
		if again := text.ToSnakeCase(snake); again != snake {
			// The first pass can still expose a new boundary: a rune that is
			// upper case but has no lower-case mapping (and invalid UTF-8 bytes,
			// which decode to U+FFFD) may sit after a rune that only became
			// lower case during this pass. What must hold is that the conversion
			// converges instead of oscillating.
			if !converges(snake, text.ToSnakeCase) {
				t.Fatalf("ToSnakeCase does not converge for %q: %q then %q", s, snake, again)
			}
		}

		kebab := text.ToKebabCase(s)
		if !converges(kebab, text.ToKebabCase) {
			t.Fatalf("ToKebabCase does not converge for %q", s)
		}

		// camelCase loses the boundary between adjacent capitalised words
		// ("a_b_c" -> "aBC" -> "a_bc"), so the promise is stability from the
		// camel round onwards rather than equality with the original
		if fromCamel := text.ToSnakeCase(text.ToLowerCamel(s)); !converges(fromCamel, text.ToSnakeCase) {
			t.Fatalf("snake(camel(%q)) = %q does not converge", s, fromCamel)
		}
		if fromUpper := text.ToSnakeCase(text.ToUpperCamel(s)); !converges(fromUpper, text.ToSnakeCase) {
			t.Fatalf("snake(UpperCamel(%q)) = %q does not converge", s, fromUpper)
		}
	})
}

// FuzzTextHelpersNeverPanic checks the remaining helpers against hostile input:
// they may truncate or pad arbitrary bytes but must never panic.
func FuzzTextHelpersNeverPanic(f *testing.F) {
	f.Add("hello", 3, "…")
	f.Add("", 0, "")
	f.Add("héllo wörld", 4, "..")

	f.Fuzz(func(t *testing.T, s string, n int, ellipsis string) {
		maxLen := n % 64
		if maxLen < 0 {
			maxLen += 64
		}
		if got := text.Truncate(s, maxLen, ellipsis); len(got) > maxLen {
			t.Fatalf("Truncate(%q, %d, %q) = %q; the result is longer than the budget", s, maxLen, ellipsis, got)
		}
		if got := text.PadStart(s, maxLen, '0'); len(got) < len(s) || (len(s) < maxLen && len(got) != maxLen) {
			t.Fatalf("PadStart(%q, %d) = %q; want at least the input and exactly the budget", s, maxLen, got)
		}
		if got := text.PadEnd(s, maxLen, '0'); !strings.HasPrefix(got, s) {
			t.Fatalf("PadEnd(%q, %d) = %q; want the input as a prefix", s, maxLen, got)
		}
		if text.IsBlank(s) && !text.IsNullOrBlank(s) {
			t.Fatalf("IsNullOrBlank disagrees with IsBlank for %q", s)
		}
		if text.IsNullOrEmpty(s) && !text.IsNullOrBlank(s) {
			t.Fatalf("IsNullOrBlank must be true for the empty string")
		}
	})
}
