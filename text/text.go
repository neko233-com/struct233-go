// Package text provides string helpers: joining, splitting, padding,
// truncation and word-case conversion.
//
// The plain functions have no room for policy, so the two fluent types carry
// it. A Joiner remembers the separator and what to do with empty values, which
// removes the "filter the slice first" pre-pass:
//
//	j := text.NewJoiner(", ").SkipEmpty()
//	j.Join("ada", "", "grace") // "ada, grace"
//
//	text.NewJoiner(", ").UseForEmpty("-").Join("ada", "", "grace") // "ada, -, grace"
//
// A Splitter is the mirror image: it trims and/or drops empty parts while it
// cuts, and Limit keeps the tail glued to the last part:
//
//	s := text.NewSplitter(",").Trim().OmitEmpty()
//	s.Split(" ada , , grace ") // ["ada", "grace"]
//
//	text.NewSplitter(",").Limit(2).Split("a,b,c") // ["a", "b,c"]
//
// The case conversions split words on explicit separators ("_", "-", white
// space), on a lower-to-upper transition ("userID" -> "user" "ID") and inside
// acronym runs ("HTTPServer" -> "HTTP" "Server"). Digits glue: a digit is never
// a boundary and is never the lower-case rune that triggers one, so "user2FA"
// is the single word "user2fa" and "v2_api" is "v2" "api". Keeping the digit
// glued to the left word is the one rule applied everywhere, which makes the
// conversions idempotent for their own format.
//
// Repeat, Truncate, PadStart and PadEnd count bytes, not runes: they may cut a
// UTF-8 sequence in half. Callers who care about runes must pre-encode.
//
// The case conversions work on runes, so invalid UTF-8 bytes become U+FFFD.
// They are idempotent for their own output on ordinary input; a rune that is
// upper case but has no lower-case mapping (or a replacement character) can
// still expose one more boundary, so the conversion converges after a second
// pass rather than in one.
package text

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Join joins elems with sep (com.google.common.base.Joiner.on(sep).join).
func Join(sep string, elems ...string) string {
	return strings.Join(elems, sep)
}

// JoinSlice joins a slice with sep.
func JoinSlice(sep string, elems []string) string {
	return strings.Join(elems, sep)
}

// JoinAny joins the fmt.Sprint form of elems with sep.
func JoinAny(sep string, elems ...any) string {
	parts := make([]string, len(elems))
	for i, e := range elems {
		parts[i] = fmt.Sprint(e)
	}
	return strings.Join(parts, sep)
}

// JoinMap joins a map as "key<kvSep>value" pairs separated by entrySep, with the
// entries in sorted key order so the result is deterministic.
func JoinMap(m map[string]string, entrySep, kvSep string) string {
	keys := slices.Sorted(maps.Keys(m))
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(entrySep)
		}
		sb.WriteString(k)
		sb.WriteString(kvSep)
		sb.WriteString(m[k])
	}
	return sb.String()
}

// Joiner joins strings with skippable or substituted empty values
// (com.google.common.base.Joiner).
//
// The zero value is not usable; build one with NewJoiner. The option methods
// mutate the receiver and return it, so they are not safe for concurrent use
// while another goroutine is joining.
type Joiner struct {
	sep        string
	skipEmpty  bool
	replace    string
	hasReplace bool
}

// NewJoiner creates a joiner that puts sep between the values.
func NewJoiner(sep string) *Joiner {
	return &Joiner{sep: sep}
}

// SkipEmpty drops empty values instead of writing them. It returns the same
// joiner so calls can be chained, and it clears any replacement set by
// UseForEmpty.
func (j *Joiner) SkipEmpty() *Joiner {
	j.skipEmpty = true
	j.replace = ""
	j.hasReplace = false
	return j
}

// UseForEmpty writes replacement instead of an empty value. It returns the same
// joiner so calls can be chained, and it clears SkipEmpty.
func (j *Joiner) UseForEmpty(replacement string) *Joiner {
	j.replace = replacement
	j.hasReplace = true
	j.skipEmpty = false
	return j
}

// Join joins elems.
func (j *Joiner) Join(elems ...string) string {
	return j.JoinSlice(elems)
}

// JoinSlice joins a slice.
func (j *Joiner) JoinSlice(elems []string) string {
	var sb strings.Builder
	// written counts the values actually emitted, so the separator lands between
	// survivors rather than between input slots
	written := 0
	for _, e := range elems {
		if j.skipEmpty && e == "" {
			continue
		}
		if written > 0 {
			sb.WriteString(j.sep)
		}
		if e == "" && j.hasReplace {
			sb.WriteString(j.replace)
		} else {
			sb.WriteString(e)
		}
		written++
	}
	return sb.String()
}

// Splitter splits a string with optional trimming and empty-part handling
// (com.google.common.base.Splitter).
//
// The zero value is not usable; build one with NewSplitter. The option methods
// mutate the receiver and return it, so they are not safe for concurrent use
// while another goroutine is splitting.
type Splitter struct {
	sep       string
	omitEmpty bool
	trim      bool
	limit     int // 0 means unlimited
}

// NewSplitter creates a splitter that cuts on sep. An empty sep cuts between
// UTF-8 runes, matching strings.Split.
func NewSplitter(sep string) *Splitter {
	return &Splitter{sep: sep}
}

// OmitEmpty drops empty parts. It returns the same splitter so calls can be
// chained.
func (s *Splitter) OmitEmpty() *Splitter {
	s.omitEmpty = true
	return s
}

// Trim trims the surrounding white space of every part. It returns the same
// splitter so calls can be chained.
func (s *Splitter) Trim() *Splitter {
	s.trim = true
	return s
}

// Limit caps how many parts are produced, the last one keeping the rest of the
// string. It panics when n is below 1 and returns the same splitter so calls can
// be chained.
//
// The cap is applied before Trim and OmitEmpty, so the kept tail is still
// trimmed (but never re-split) and a part dropped by OmitEmpty does not free a
// slot.
func (s *Splitter) Limit(n int) *Splitter {
	if n < 1 {
		panic("text: splitter limit must be at least 1")
	}
	s.limit = n
	return s
}

// Split splits str.
func (s *Splitter) Split(str string) []string {
	var parts []string
	if s.limit > 0 {
		parts = strings.SplitN(str, s.sep, s.limit)
	} else {
		parts = strings.Split(str, s.sep)
	}
	if !s.trim && !s.omitEmpty {
		return parts
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s.trim {
			p = strings.TrimSpace(p)
		}
		if s.omitEmpty && p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// Split splits str on sep without options.
func Split(str, sep string) []string {
	return NewSplitter(sep).Split(str)
}

// IsNullOrEmpty reports whether s is empty (the Go analog of
// com.google.common.base.Strings.isNullOrEmpty).
func IsNullOrEmpty(s string) bool {
	return s == ""
}

// IsBlank reports whether s is empty or holds only white space.
func IsBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

// IsNullOrBlank reports whether s is empty or holds only white space.
func IsNullOrBlank(s string) bool {
	return IsBlank(s)
}

// Repeat returns s repeated n times. It panics when n is negative.
func Repeat(s string, n int) string {
	if n < 0 {
		panic("text: repeat count must not be negative")
	}
	return strings.Repeat(s, n)
}

// PadStart left-pads s with pad until it is at least minLen bytes long; it
// returns s unchanged when it is already long enough.
//
// Byte semantics: minLen and the padding are counted in bytes, so a multi-byte
// s needs fewer pad bytes than its rune count suggests.
func PadStart(s string, minLen int, pad byte) string {
	if len(s) >= minLen {
		return s
	}
	return strings.Repeat(string(pad), minLen-len(s)) + s
}

// PadEnd right-pads s with pad until it is at least minLen bytes long.
//
// Byte semantics, see PadStart.
func PadEnd(s string, minLen int, pad byte) string {
	if len(s) >= minLen {
		return s
	}
	return s + strings.Repeat(string(pad), minLen-len(s))
}

// Truncate shortens s to at most maxLen bytes, replacing the tail with ellipsis
// when it did not fit. It panics when maxLen is negative.
//
// The result never exceeds maxLen bytes. When ellipsis alone is longer than the
// budget it is itself cut, so the result may end in the middle of a UTF-8
// sequence.
func Truncate(s string, maxLen int, ellipsis string) string {
	if maxLen < 0 {
		panic("text: maxLen must not be negative")
	}
	if len(s) <= maxLen {
		return s
	}
	if len(ellipsis) >= maxLen {
		return ellipsis[:maxLen]
	}
	return s[:maxLen-len(ellipsis)] + ellipsis
}

// ToLowerCamel converts snake_case, kebab-case or space separated words to
// lowerCamelCase.
//
// Upper-case input is not preserved: "HTTPServer" becomes "httpServer". A digit
// keeps the following run glued ("user2FA" is one word), see the package doc.
func ToLowerCamel(s string) string {
	return camel(s, false)
}

// ToUpperCamel converts words to UpperCamelCase.
func ToUpperCamel(s string) string {
	return camel(s, true)
}

// ToSnakeCase converts words to snake_case.
func ToSnakeCase(s string) string {
	return joinWords(s, "_")
}

// ToKebabCase converts words to kebab-case.
func ToKebabCase(s string) string {
	return joinWords(s, "-")
}

// joinWords lower-cases every word of s and joins them with sep.
func joinWords(s, sep string) string {
	words := splitWords(s)
	lowered := make([]string, len(words))
	for i, w := range words {
		lowered[i] = strings.ToLower(w)
	}
	return strings.Join(lowered, sep)
}

// camel lower-cases every word of s and joins them in camel case, capitalizing
// every word but the first unless upperFirst.
func camel(s string, upperFirst bool) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for i, w := range splitWords(s) {
		w = strings.ToLower(w)
		if i > 0 || upperFirst {
			w = capitalize(w)
		}
		sb.WriteString(w)
	}
	return sb.String()
}

// capitalize upper-cases the first rune of s, keeping the rest as is.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if up := unicode.ToUpper(r); up != r {
		return string(up) + s[size:]
	}
	return s
}

// isWordSeparator reports whether r separates words in the case conversions.
func isWordSeparator(r rune) bool {
	return r == '_' || r == '-' || unicode.IsSpace(r)
}

// splitWords cuts s into words: separators are dropped, a lower-to-upper
// transition opens a word and the last upper-case rune of an acronym run opens
// one too ("HTTPServer" -> "HTTP" "Server"). Digits never open a word, and a
// digit does not count as the lower-case rune that would open one, so
// "user2FA" stays a single word.
func splitWords(s string) []string {
	runes := []rune(s)
	words := make([]string, 0, 4)
	start := -1 // index of the first rune of the word being built
	for i, r := range runes {
		if isWordSeparator(r) {
			if start >= 0 {
				words = append(words, string(runes[start:i]))
				start = -1
			}
			continue
		}
		if start >= 0 && opensWord(runes, i) {
			words = append(words, string(runes[start:i]))
			start = i
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		words = append(words, string(runes[start:]))
	}
	return words
}

// opensWord reports whether the rune at i opens a new word inside the word that
// already started, i.e. i > 0 and runes[i-1] belongs to the current word.
func opensWord(runes []rune, i int) bool {
	cur, prev := runes[i], runes[i-1]
	if !unicode.IsUpper(cur) {
		return false // digits and lower-case runes never open one
	}
	if unicode.IsLower(prev) {
		return true // userID -> user | ID
	}
	// acronym run: the last upper-case rune before a lower-case one opens the
	// word (HTTP|Server); a digit in front keeps the run glued to it.
	return unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1])
}
