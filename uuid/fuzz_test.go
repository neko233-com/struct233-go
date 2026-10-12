package uuid_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/neko233-com/struct233-go/uuid"
)

// FuzzParseRoundTrip checks the two invariants of the text codec for arbitrary
// input: Parse either rejects the string with the documented sentinel error, or
// it returns a UUID whose canonical form parses back to the same value.
func FuzzParseRoundTrip(f *testing.F) {
	f.Add(canonical)
	f.Add(strings.ToUpper(canonical))
	f.Add(strings.ReplaceAll(canonical, "-", ""))
	f.Add("urn:uuid:" + canonical)
	f.Add("{" + canonical + "}")
	f.Add("")
	f.Add("0198e0a1")
	f.Add("zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz")

	f.Fuzz(func(t *testing.T, s string) {
		u, err := uuid.Parse(s)
		if err != nil {
			if !errors.Is(err, uuid.ErrInvalidFormat) {
				t.Fatalf("Parse(%q) returned %v; want uuid.ErrInvalidFormat", s, err)
			}
			return
		}

		text := u.String()
		if len(text) != 36 {
			t.Fatalf("String() of %q = %q; want 36 characters", s, text)
		}
		for _, i := range []int{8, 13, 18, 23} {
			if text[i] != '-' {
				t.Fatalf("String() of %q = %q; want a hyphen at index %d", s, text, i)
			}
		}
		if text != strings.ToLower(text) {
			t.Fatalf("String() of %q = %q; want lowercase", s, text)
		}

		again, err := uuid.Parse(text)
		if err != nil || again != u {
			t.Fatalf("Parse(%q) = %v -> %q; parsing that back gave %v, %v", s, u, text, again, err)
		}

		// the equivalent bare and upper-case spellings must decode identically
		for _, alias := range []string{strings.ReplaceAll(text, "-", ""), strings.ToUpper(text)} {
			got, err := uuid.Parse(alias)
			if err != nil || got != u {
				t.Fatalf("Parse(%q) = %v, %v; want %v", alias, got, err, u)
			}
		}
	})
}

// FuzzUnmarshalTextAgreesWithParse keeps the encoding.TextUnmarshaler path and
// Parse in lockstep, including which inputs they reject.
func FuzzUnmarshalTextAgreesWithParse(f *testing.F) {
	f.Add([]byte(canonical))
	f.Add([]byte(strings.ReplaceAll(canonical, "-", "")))
	f.Add([]byte{})
	f.Add([]byte("urn:uuid:" + canonical))

	f.Fuzz(func(t *testing.T, b []byte) {
		want, wantErr := uuid.Parse(string(b))

		var got uuid.UUID
		gotErr := got.UnmarshalText(b)

		if (gotErr != nil) != (wantErr != nil) {
			t.Fatalf("UnmarshalText(%q) error = %v; Parse error = %v", b, gotErr, wantErr)
		}
		if gotErr != nil {
			if !errors.Is(gotErr, uuid.ErrInvalidFormat) {
				t.Fatalf("UnmarshalText(%q) = %v; want uuid.ErrInvalidFormat", b, gotErr)
			}
			return
		}
		if got != want {
			t.Fatalf("UnmarshalText(%q) = %v; Parse gave %v", b, got, want)
		}
	})
}

// FuzzBinaryRoundTrip checks that any 16 bytes survive a binary round trip and
// that values of the wrong width are refused.
func FuzzBinaryRoundTrip(f *testing.F) {
	f.Add([]byte(canonical))
	f.Add(make([]byte, 16))
	f.Add(make([]byte, 15))

	f.Fuzz(func(t *testing.T, b []byte) {
		var u uuid.UUID
		if err := u.UnmarshalBinary(b); err != nil {
			if len(b) == 16 {
				t.Fatalf("UnmarshalBinary(%d bytes) = %v; want success", len(b), err)
			}
			if !errors.Is(err, uuid.ErrInvalidFormat) {
				t.Fatalf("UnmarshalBinary(%d bytes) = %v; want uuid.ErrInvalidFormat", len(b), err)
			}
			return
		}
		if len(b) != 16 {
			t.Fatalf("UnmarshalBinary(%d bytes) succeeded; want a width error", len(b))
		}

		out, err := u.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary() = %v", err)
		}
		if string(out) != string(b) {
			t.Fatalf("binary round trip changed the bytes: %x != %x", out, b)
		}
	})
}
