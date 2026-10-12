// Package uuid generates and parses RFC 9562 universally unique identifiers.
//
// It depends on the standard library only. Version 4 UUIDs are random, version
// 7 UUIDs embed a millisecond timestamp so they sort by creation time, and
// UUID is a comparable array type: it works as a map key, as a sort key and as
// a JSON string without any wrapper.
//
//	id := uuid.NewV7()
//	fmt.Println(id, id.Time())
//	parsed, err := uuid.Parse(id.String())
//
// Prefer NewV7 over NewV4 for database keys: successive v7 UUIDs are strictly
// increasing, which keeps B-tree inserts local instead of scattering them over
// the whole index.
package uuid

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

// UUID is a 128-bit universally unique identifier (RFC 9562).
type UUID [16]byte

// Nil is the all-zero UUID, the smallest value in the RFC 9562 sort order.
var Nil UUID

// Max is the all-ones UUID, the largest value in the RFC 9562 sort order.
var Max = UUID{
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
}

// ErrInvalidFormat is returned by Parse and the unmarshal methods when the
// input is not a UUID in any of the accepted text or binary forms.
var ErrInvalidFormat = errors.New("uuid: invalid UUID format")

const (
	// textLen is the length of the canonical form, hyphens included.
	textLen = 36
	// maxSeq is the largest rand_a counter that fits in a version 7 UUID: the
	// 12 bits shared by the version nibble of byte 6.
	maxSeq = 0x0fff
)

// New returns a random version 4 UUID, the common case.
func New() UUID { return NewV4() }

// NewV4 returns a random version 4 UUID: 122 random bits from crypto/rand, with
// the version and variant bits set. Entropy is buffered under a mutex to avoid
// per-call allocation on all supported Go versions. Generation cannot fail on
// a supported platform.
func NewV4() UUID {
	var u UUID
	fillRandom(u[:])
	u[6] = u[6]&0x0f | 0x40 // version 4
	u[8] = u[8]&0x3f | 0x80 // RFC 9562 variant
	return u
}

// NewV7 returns a strictly increasing version 7 UUID for the current time. It
// shares a package-level Generator, so concurrent callers still receive ordered
// IDs even when the clock has millisecond resolution.
func NewV7() UUID { return defaultGenerator.Next() }

// NewV7At returns a version 7 UUID carrying the given timestamp. The generator
// state is untouched, so successive calls are only as ordered as their
// timestamps; use a Generator when monotonicity matters. Times before the Unix
// epoch are clamped to it.
func NewV7At(t time.Time) UUID {
	var seq [2]byte
	fillRandom(seq[:])
	return buildV7(t.UnixMilli(), uint16(seq[0]&0x0f)<<8|uint16(seq[1]))
}

// Generator issues version 7 UUIDs that are strictly increasing, using the
// 12-bit rand_a field as a counter and borrowing the next millisecond when it
// overflows. It is safe for concurrent use.
type Generator struct {
	mu     sync.Mutex
	now    func() time.Time
	lastMS int64 // timestamp of the last issued UUID
	seq    uint16
}

// Option configures a Generator.
type Option func(*Generator)

// WithClock sets the time source of a Generator. Default: time.Now. Injecting a
// clock keeps tests deterministic.
func WithClock(now func() time.Time) Option {
	return func(g *Generator) { g.now = now }
}

// NewGenerator creates a version 7 generator. It panics when the injected clock
// is nil.
func NewGenerator(opts ...Option) *Generator {
	g := &Generator{now: time.Now}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(g)
	}
	if g.now == nil {
		panic("uuid: nil clock")
	}
	return g
}

// Next returns the next strictly increasing version 7 UUID. Repeating
// timestamps (and a clock that moves backwards) advance the counter instead of
// repeating the timestamp, so IDs never go backwards.
func (g *Generator) Next() UUID {
	ms := g.now().UnixMilli()

	g.mu.Lock()
	defer g.mu.Unlock()
	switch {
	case ms > g.lastMS:
		g.lastMS, g.seq = ms, 0
	case g.seq < maxSeq:
		g.seq++
	default:
		// the counter is exhausted: borrow the next millisecond, which still
		// sorts after every UUID issued so far
		g.lastMS++
		g.seq = 0
	}
	return buildV7(g.lastMS, g.seq)
}

// defaultGenerator backs NewV7.
var defaultGenerator = NewGenerator()

// String returns the canonical 36-character lowercase form,
// "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx".
func (u UUID) String() string {
	var b [textLen]byte
	encodeText(b[:], u)
	return string(b[:])
}

// AppendText appends the canonical form to dst and returns the extended slice.
// It implements encoding.TextAppender, so an encoder that supports it can
// serialise a UUID into a caller-owned buffer without an intermediate
// allocation.
func (u UUID) AppendText(dst []byte) ([]byte, error) {
	return appendText(dst, u), nil
}

// MarshalText implements encoding.TextMarshaler, so UUID values encode as JSON
// strings and can be used as JSON object keys.
func (u UUID) MarshalText() ([]byte, error) {
	return appendText(nil, u), nil
}

// UnmarshalText implements encoding.TextUnmarshaler, so UUID values decode from
// JSON strings. It returns ErrInvalidFormat for anything Parse rejects.
func (u *UUID) UnmarshalText(b []byte) error {
	parsed, err := Parse(string(b))
	if err != nil {
		return err
	}
	*u = parsed
	return nil
}

// MarshalBinary implements encoding.BinaryMarshaler as the 16 raw bytes.
func (u UUID) MarshalBinary() ([]byte, error) {
	b := make([]byte, len(u))
	copy(b, u[:])
	return b, nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler. It returns
// ErrInvalidFormat when b is not exactly 16 bytes long.
func (u *UUID) UnmarshalBinary(b []byte) error {
	if len(b) != len(u) {
		return ErrInvalidFormat
	}
	copy(u[:], b)
	return nil
}

// Bytes returns the 16 raw bytes of the UUID.
func (u UUID) Bytes() [16]byte { return u }

// Version returns the version nibble stored in byte 6: 4 for random UUIDs, 7
// for time-ordered ones, 0 for Nil.
func (u UUID) Version() int { return int(u[6] >> 4) }

// IsNil reports whether u is the nil UUID.
func (u UUID) IsNil() bool { return u == Nil }

// Compare orders u and other byte-wise, which is RFC 9562 order and therefore
// time order for version 7 UUIDs. It returns a negative number, zero or a
// positive number, and can be used directly with slices.SortFunc.
func (u UUID) Compare(other UUID) int {
	return bytes.Compare(u[:], other[:])
}

// Time returns the creation time embedded in a version 7 UUID, truncated to
// milliseconds. ok is false for every other version, which carries no
// timestamp.
func (u UUID) Time() (time.Time, bool) {
	if u.Version() != 7 {
		return time.Time{}, false
	}
	ms := int64(u[0])<<40 | int64(u[1])<<32 | int64(u[2])<<24 |
		int64(u[3])<<16 | int64(u[4])<<8 | int64(u[5])
	return time.UnixMilli(ms).UTC(), true
}

// Parse accepts the canonical 36-character form, the 32-character hex form, an
// optional "urn:uuid:" prefix (any case), optional surrounding braces and
// optional surrounding whitespace. Hex digits may be upper or lower case.
func Parse(s string) (UUID, error) {
	in := strings.TrimSpace(s)
	if len(in) >= 2 && in[0] == '{' && in[len(in)-1] == '}' {
		in = in[1 : len(in)-1]
	}
	if len(in) == 45 && strings.EqualFold(in[:9], "urn:uuid:") {
		in = in[9:]
	}
	if len(in) != 32 && len(in) != textLen {
		return Nil, ErrInvalidFormat
	}
	if len(in) == textLen && (in[8] != '-' || in[13] != '-' || in[18] != '-' || in[23] != '-') {
		return Nil, ErrInvalidFormat
	}

	var digits [32]byte
	n := 0
	for i := range len(in) {
		c := in[i]
		if c == '-' {
			continue
		}
		if n == len(digits) {
			return Nil, ErrInvalidFormat
		}
		digits[n] = c
		n++
	}
	if n != len(digits) {
		return Nil, ErrInvalidFormat
	}

	var u UUID
	if _, err := hex.Decode(u[:], digits[:]); err != nil {
		return Nil, ErrInvalidFormat
	}
	return u, nil
}

// MustParse is Parse, panicking when s is not a valid UUID.
func MustParse(s string) UUID {
	u, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// buildV7 lays out the RFC 9562 version 7 fields: 48 bits of milliseconds,
// 4 bits of version, 12 bits of seq, 2 bits of variant and 62 random bits.
func buildV7(ms int64, seq uint16) UUID {
	if ms < 0 {
		ms = 0 // the 48-bit field cannot carry a pre-epoch timestamp
	}
	var u UUID
	u[0] = byte(ms >> 40)
	u[1] = byte(ms >> 32)
	u[2] = byte(ms >> 24)
	u[3] = byte(ms >> 16)
	u[4] = byte(ms >> 8)
	u[5] = byte(ms)
	u[6] = 0x70 | byte(seq>>8)&0x0f
	u[7] = byte(seq)
	fillRandom(u[8:])
	u[8] = u[8]&0x3f | 0x80
	return u
}

// appendText writes the canonical form at the end of dst, reusing dst's spare
// capacity when there is any.
func appendText(dst []byte, u UUID) []byte {
	start := len(dst)
	var buf [textLen]byte
	dst = append(dst, buf[:]...)
	encodeText(dst[start:start+textLen], u)
	return dst
}

// encodeText writes the canonical form into dst, which must hold textLen bytes.
func encodeText(dst []byte, u UUID) {
	hex.Encode(dst[0:8], u[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], u[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], u[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], u[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], u[10:16])
}

// entropy holds a bounded batch of crypto/rand bytes. Consumed bytes are
// cleared, and the caller's stack buffer never escapes into crypto/rand.
var entropy = struct {
	sync.Mutex
	bytes [4096]byte
	next  int
}{next: 4096}

// fillRandom fills the internal UUID fields (at most 16 bytes per call).
func fillRandom(b []byte) {
	entropy.Lock()
	defer entropy.Unlock()
	if len(b) > len(entropy.bytes)-entropy.next {
		if _, err := rand.Read(entropy.bytes[:]); err != nil {
			panic("uuid: crypto/rand failed: " + err.Error())
		}
		entropy.next = 0
	}
	src := entropy.bytes[entropy.next : entropy.next+len(b)]
	copy(b, src)
	clear(src)
	entropy.next += len(b)
}
