package uuid_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/uuid"
)

// sink keeps the compiler from optimising a generated UUID away.
var sink uuid.UUID

// canonical is a fixed UUID used by the parsing and formatting tests. Its
// version nibble is 7 and its variant bits are the RFC 9562 ones.
const canonical = "0198e0a1-3f2b-7c4d-9e5f-0123456789ab"

// clock is a manually advanced time source: ordering tests stay deterministic
// instead of racing the real clock.
type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

func (c *clock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func TestNewV4SetsVersionAndVariant(t *testing.T) {
	seen := make(map[uuid.UUID]struct{}, 256)
	for range 256 {
		u := uuid.NewV4()
		require.Equal(t, 4, u.Version())
		require.Equal(t, byte(0x80), u[8]&0xc0, "the variant bits must be 0b10")
		seen[u] = struct{}{}
	}
	require.Len(t, seen, 256, "random UUIDs must not collide")
	require.Equal(t, 4, uuid.New().Version(), "New is the random v4 generator")
}

func TestNewV4DoesNotAllocate(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() { sink = uuid.NewV4() })
	require.Zero(t, allocs, "NewV4 draws from an internal random buffer and must not allocate")
}

func TestNilAndMax(t *testing.T) {
	require.True(t, uuid.Nil.IsNil())
	require.Zero(t, uuid.Nil.Version())
	require.False(t, uuid.Max.IsNil())
	require.Equal(t, 15, uuid.Max.Version())

	require.Negative(t, uuid.Nil.Compare(uuid.Max))
	require.Positive(t, uuid.Max.Compare(uuid.Nil))
	require.Zero(t, uuid.Nil.Compare(uuid.Nil))
}

func TestStringIsCanonical(t *testing.T) {
	u, err := uuid.Parse(canonical)
	require.NoError(t, err)

	got := u.String()
	require.Equal(t, canonical, got)
	require.Len(t, got, 36)
	for _, i := range []int{8, 13, 18, 23} {
		require.Equal(t, byte('-'), got[i], "hyphen at %d", i)
	}
	require.Equal(t, got, strings.ToLower(got), "the canonical form is lowercase")
}

func TestParseAcceptsEveryTextForm(t *testing.T) {
	want, err := uuid.Parse(canonical)
	require.NoError(t, err)

	forms := map[string]string{
		"canonical":      canonical,
		"upper case":     strings.ToUpper(canonical),
		"bare hex":       strings.ReplaceAll(canonical, "-", ""),
		"bare hex upper": strings.ToUpper(strings.ReplaceAll(canonical, "-", "")),
		"urn prefix":     "urn:uuid:" + canonical,
		"urn upper":      "URN:UUID:" + strings.ToUpper(canonical),
		"braces":         "{" + canonical + "}",
		"braced urn":     "{urn:uuid:" + canonical + "}",
		"whitespace":     "  " + canonical + "\t\n",
	}
	for name, form := range forms {
		got, err := uuid.Parse(form)
		require.NoError(t, err, "%s form must parse", name)
		require.Equal(t, want, got, "%s form must parse to the same UUID", name)
	}
}

func TestParseRejectsInvalidInput(t *testing.T) {
	cases := map[string]string{
		"empty":                 "",
		"blank":                 "   ",
		"words":                 "not-a-uuid",
		"short":                 canonical[:35],
		"long":                  canonical + "a",
		"bad hex digit":         strings.Replace(canonical, "0198", "g198", 1),
		"hyphen in wrong place": strings.Replace(canonical, "-", "_", 1),
		"missing hyphen":        strings.Replace(canonical, "-", "", 1),
		"extra hyphen":          canonical[:8] + "--" + canonical[9:],
		"unclosed brace":        "{" + canonical,
		"only a brace":          "{",
		"urn without uuid":      "urn:" + canonical,
		"short urn body":        "urn:uuid:" + canonical[:20],
		"32 chars with hyphens": canonical[:32],
		"non ascii":             strings.Replace(canonical, "0", "０", 1),
	}
	for name, input := range cases {
		got, err := uuid.Parse(input)
		require.ErrorIs(t, err, uuid.ErrInvalidFormat, "%s (%q) must be rejected", name, input)
		require.Equal(t, uuid.Nil, got, "%s must not return a partial UUID", name)
	}
}

func TestMustParsePanics(t *testing.T) {
	require.Equal(t, canonical, uuid.MustParse(canonical).String())
	require.PanicsWithValue(t, uuid.ErrInvalidFormat, func() { uuid.MustParse("nope") })
}

func TestStringParseRoundTrip(t *testing.T) {
	for range 128 {
		u := uuid.NewV4()
		parsed, err := uuid.Parse(u.String())
		require.NoError(t, err)
		require.Equal(t, u, parsed)
	}
}

func TestVersionAndTime(t *testing.T) {
	v7 := uuid.NewV7()
	require.Equal(t, 7, v7.Version())

	tt, ok := v7.Time()
	require.True(t, ok, "a v7 UUID carries a timestamp")
	require.WithinDuration(t, time.Now(), tt, time.Minute)

	_, ok = uuid.NewV4().Time()
	require.False(t, ok, "a v4 UUID carries no timestamp")
	_, ok = uuid.Nil.Time()
	require.False(t, ok)
}

func TestNewV7IsStrictlyIncreasing(t *testing.T) {
	c := &clock{now: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
	g := uuid.NewGenerator(uuid.WithClock(c.Now))

	prev := g.Next()
	// 5001 IDs far exceed the 4096 ids one millisecond can hold, so this also
	// exercises the counter overflow path.
	for range 5000 {
		next := g.Next()
		require.Negative(t, prev.Compare(next), "UUIDs must increase: %s then %s", prev, next)
		prev = next
	}

	tt, ok := prev.Time()
	require.True(t, ok)
	require.Equal(t, time.Millisecond, tt.Sub(c.Now()),
		"the overflow must borrow exactly the next millisecond")
}

func TestClockRollbackKeepsIDsIncreasing(t *testing.T) {
	c := &clock{now: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
	g := uuid.NewGenerator(uuid.WithClock(c.Now))

	before := g.Next()
	c.Advance(-time.Hour) // the clock jumps backwards
	after := g.Next()

	require.Negative(t, before.Compare(after), "a backwards clock must not repeat or rewind IDs")
}

func TestConcurrentNewV7IsUnique(t *testing.T) {
	g := uuid.NewGenerator()
	const (
		goroutines = 8
		perWorker  = 500
	)

	ch := make(chan uuid.UUID, goroutines*perWorker)
	var done = make(chan struct{})
	for range goroutines {
		go func() {
			defer func() { done <- struct{}{} }()
			for range perWorker {
				ch <- g.Next()
			}
		}()
	}
	for range goroutines {
		<-done
	}
	close(ch)

	seen := make(map[uuid.UUID]struct{}, goroutines*perWorker)
	for u := range ch {
		seen[u] = struct{}{}
	}
	require.Len(t, seen, goroutines*perWorker, "concurrent generation must not repeat an ID")
}

func TestNewV7AtUsesTheGivenTimestamp(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 678*int(time.Millisecond), time.UTC)
	u := uuid.NewV7At(at)

	require.Equal(t, 7, u.Version())
	require.Equal(t, byte(0x80), u[8]&0xc0)

	tt, ok := u.Time()
	require.True(t, ok)
	require.Equal(t, at.UnixMilli(), tt.UnixMilli())
}

func TestNewV7AtClampsPreEpochTimes(t *testing.T) {
	tt, ok := uuid.NewV7At(time.Unix(-1, 0).UTC()).Time()
	require.True(t, ok)
	require.Equal(t, int64(0), tt.UnixMilli(), "the 48-bit field cannot go below the Unix epoch")
}

func TestGeneratorPanicsOnNilClock(t *testing.T) {
	require.Panics(t, func() { uuid.NewGenerator(uuid.WithClock(nil)) })
}

func TestCompareSortsTimeOrderedUUIDs(t *testing.T) {
	base := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	ids := []uuid.UUID{
		uuid.NewV7At(base.Add(2 * time.Millisecond)),
		uuid.NewV7At(base),
		uuid.NewV7At(base.Add(time.Millisecond)),
	}

	slices.SortFunc(ids, uuid.UUID.Compare)

	for i, want := range []time.Time{base, base.Add(time.Millisecond), base.Add(2 * time.Millisecond)} {
		tt, ok := ids[i].Time()
		require.True(t, ok)
		require.Equal(t, want.UnixMilli(), tt.UnixMilli(), "index %d", i)
	}
}

func TestTextMarshalling(t *testing.T) {
	u := uuid.MustParse(canonical)

	b, err := u.MarshalText()
	require.NoError(t, err)
	require.Equal(t, canonical, string(b))

	var parsed uuid.UUID
	require.NoError(t, parsed.UnmarshalText([]byte(canonical)))
	require.Equal(t, u, parsed)
	require.ErrorIs(t, parsed.UnmarshalText([]byte("nope")), uuid.ErrInvalidFormat)

	// JSON values and JSON map keys both use the text form
	jb, err := json.Marshal(u)
	require.NoError(t, err)
	require.JSONEq(t, `"`+canonical+`"`, string(jb))

	var back uuid.UUID
	require.NoError(t, json.Unmarshal(jb, &back))
	require.Equal(t, u, back)

	keys, err := json.Marshal(map[uuid.UUID]int{u: 1})
	require.NoError(t, err, "a map key needs the text form to be usable")
	require.JSONEq(t, `{"`+canonical+`":1}`, string(keys))

	require.Error(t, json.Unmarshal([]byte(`"nope"`), &back))
}

func TestAppendTextAppendsAndDoesNotAllocate(t *testing.T) {
	u := uuid.MustParse(canonical)

	prefix := []byte("id=")
	out, err := u.AppendText(prefix)
	require.NoError(t, err)
	require.Equal(t, "id="+canonical, string(out))

	buf := make([]byte, 0, 64)
	_, err = u.AppendText(buf)
	require.NoError(t, err)

	allocs := testing.AllocsPerRun(100, func() {
		buf = buf[:0]
		buf, _ = u.AppendText(buf)
	})
	require.Zero(t, allocs, "AppendText must reuse the caller's buffer")
	require.Equal(t, canonical, string(buf))
}

func TestBinaryMarshalling(t *testing.T) {
	u := uuid.MustParse(canonical)

	b, err := u.MarshalBinary()
	require.NoError(t, err)
	require.Len(t, b, 16)
	require.Equal(t, u[:], b)

	var parsed uuid.UUID
	require.NoError(t, parsed.UnmarshalBinary(b))
	require.Equal(t, u, parsed)

	require.ErrorIs(t, parsed.UnmarshalBinary(b[:15]), uuid.ErrInvalidFormat)
	require.ErrorIs(t, parsed.UnmarshalBinary(append(b, 0)), uuid.ErrInvalidFormat)
}

func TestBytesReturnsTheRawValue(t *testing.T) {
	u := uuid.MustParse(canonical)
	require.Equal(t, [16]byte(u), u.Bytes())
}
