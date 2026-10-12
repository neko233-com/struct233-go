// This file holds the facade aliases for the utility packages: the
// Guava-inspired helpers (multimap, multiset, bidimap, text, verify, order,
// rangeset, stopwatch, memoize, atomiclongmap, striped) and the hash
// functions. They are kept out of struct233.go so the container facade stays
// readable.
package struct233

import (
	"cmp"
	"time"

	"github.com/neko233-com/struct233-go/atomiclongmap"
	"github.com/neko233-com/struct233-go/bidimap"
	"github.com/neko233-com/struct233-go/hash"
	"github.com/neko233-com/struct233-go/hashring"
	"github.com/neko233-com/struct233-go/memoize"
	"github.com/neko233-com/struct233-go/multimap"
	"github.com/neko233-com/struct233-go/multiset"
	"github.com/neko233-com/struct233-go/order"
	"github.com/neko233-com/struct233-go/rangemap"
	"github.com/neko233-com/struct233-go/rangeset"
	"github.com/neko233-com/struct233-go/stopwatch"
	"github.com/neko233-com/struct233-go/striped"
	"github.com/neko233-com/struct233-go/table"
	"github.com/neko233-com/struct233-go/text"
	"github.com/neko233-com/struct233-go/verify"
)

// Natural returns the comparator for the natural ordering of T.
func Natural[T cmp.Ordered]() Comparator[T] { return order.Natural[T]() }

// Reverse returns the comparator that orders values the other way round.
func Reverse[T any](cmp Comparator[T]) Comparator[T] { return order.Reverse(cmp) }

// Then returns a lexicographic comparator: the first one decides and the next
// breaks its ties.
func Then[T any](cmps ...Comparator[T]) Comparator[T] { return order.Then(cmps...) }

// NullsFirst orders nil pointers before every non-nil value.
func NullsFirst[T any](cmp Comparator[T]) Comparator[*T] { return order.NullsFirst(cmp) }

// NullsLast orders nil pointers after every non-nil value.
func NullsLast[T any](cmp Comparator[T]) Comparator[*T] { return order.NullsLast(cmp) }

// ByKey compares two values by a key extracted from each one.
func ByKey[T any, K any](key func(T) K, cmp Comparator[K]) Comparator[T] {
	return order.ByKey(key, cmp)
}

// By adapts a less function to a Comparator.
func By[T any](less func(a, b T) bool) Comparator[T] { return order.By(less) }

// Max returns the greater of a and b, preferring a on a tie.
func Max[T any](cmp Comparator[T], a, b T) T { return order.Max(cmp, a, b) }

// Min returns the smaller of a and b, preferring a on a tie.
func Min[T any](cmp Comparator[T], a, b T) T { return order.Min(cmp, a, b) }

// MaxOf returns the greatest element of items; ok is false when it is empty.
func MaxOf[T any](cmp Comparator[T], items []T) (T, bool) { return order.MaxOf(cmp, items) }

// MinOf returns the smallest element of items; ok is false when it is empty.
func MinOf[T any](cmp Comparator[T], items []T) (T, bool) { return order.MinOf(cmp, items) }

// Sorted returns a stable, sorted copy of items, leaving the input untouched.
func Sorted[T any](cmp Comparator[T], items []T) []T { return order.Sorted(cmp, items) }

// IsSorted reports whether items are already in the order cmp describes.
func IsSorted[T any](cmp Comparator[T], items []T) bool { return order.IsSorted(cmp, items) }

// MultimapEntry is one key/value pair of a multimap.
type MultimapEntry[K comparable, V any] = multimap.Entry[K, V]

// ListMultimap maps each key to a list of values, keeping key and value
// insertion order (com.google.common.collect.ListMultimap).
type ListMultimap[K comparable, V comparable] = multimap.ListMultimap[K, V]

// SetMultimap maps each key to a set of values, keeping key and value insertion
// order (com.google.common.collect.SetMultimap).
type SetMultimap[K comparable, V comparable] = multimap.SetMultimap[K, V]

// NewListMultimap creates an empty ListMultimap (a key may repeat a value).
func NewListMultimap[K comparable, V comparable]() *ListMultimap[K, V] {
	return multimap.NewList[K, V]()
}

// NewListMultimapWithCapacity creates an empty ListMultimap pre-sized for
// capacity keys; the extra keys stay reserved, value-less slots.
func NewListMultimapWithCapacity[K comparable, V comparable](capacity int, keys ...K) *ListMultimap[K, V] {
	return multimap.NewListWithCapacity[K, V](capacity, keys...)
}

// NewSetMultimap creates an empty SetMultimap (values are unique per key).
func NewSetMultimap[K comparable, V comparable]() *SetMultimap[K, V] {
	return multimap.NewSet[K, V]()
}

// NewSetMultimapWithCapacity creates an empty SetMultimap pre-sized for
// capacity keys; the extra keys stay reserved, value-less slots.
func NewSetMultimapWithCapacity[K comparable, V comparable](capacity int, keys ...K) *SetMultimap[K, V] {
	return multimap.NewSetWithCapacity[K, V](capacity, keys...)
}

// Multiset counts how many times each element occurs
// (com.google.common.collect.Multiset), keeping first-insertion order.
type Multiset[T comparable] = multiset.Multiset[T]

// NewMultiset creates a counting multiset, optionally pre-filled with elems
// (each counted once).
func NewMultiset[T comparable](elems ...T) *Multiset[T] {
	return multiset.New(elems...)
}

// NewMultisetWithCapacity creates an empty counting multiset whose map is
// pre-sized for capacity distinct elements.
func NewMultisetWithCapacity[T comparable](capacity int, elems ...T) *Multiset[T] {
	return multiset.NewWithCapacity(capacity, elems...)
}

// TableCell is one row, column and value tuple of a Table.
type TableCell[R comparable, C comparable, V any] = table.Cell[R, C, V]

// Table maps a row key and a column key pair to one value, keeping cells in
// insertion order (com.google.common.collect.Table).
type Table[R comparable, C comparable, V any] = table.Table[R, C, V]

// NewTable creates an empty two-dimensional Table.
func NewTable[R comparable, C comparable, V any]() *Table[R, C, V] {
	return table.New[R, C, V]()
}

// NewTableWithCapacity creates an empty Table pre-sized for capacity cells.
func NewTableWithCapacity[R comparable, C comparable, V any](capacity int) *Table[R, C, V] {
	return table.NewWithCapacity[R, C, V](capacity)
}

// Range is an interval over an ordered type whose ends are closed, open or
// unbounded (com.google.common.collect.Range). The zero value is the interval
// holding every value.
type Range[T cmp.Ordered] = rangeset.Range[T]

// RangeSet holds a set of values as disjoint, ascending, coalesced ranges
// (com.google.common.collect.RangeSet).
type RangeSet[T cmp.Ordered] = rangeset.RangeSet[T]

// NewRangeSet creates an empty set of ranges.
func NewRangeSet[T cmp.Ordered]() *RangeSet[T] { return rangeset.NewSet[T]() }

// Generic functions cannot be aliased, so the interval constructors are
// wrapped with a Range prefix instead.

// RangeClosed returns the interval [lo, hi], empty when lo > hi.
func RangeClosed[T cmp.Ordered](lo, hi T) Range[T] { return rangeset.Closed(lo, hi) }

// RangeOpen returns the interval (lo, hi), empty when lo >= hi.
func RangeOpen[T cmp.Ordered](lo, hi T) Range[T] { return rangeset.Open(lo, hi) }

// RangeOpenClosed returns the interval (lo, hi], empty when lo >= hi.
func RangeOpenClosed[T cmp.Ordered](lo, hi T) Range[T] { return rangeset.OpenClosed(lo, hi) }

// RangeClosedOpen returns the interval [lo, hi), empty when lo >= hi.
func RangeClosedOpen[T cmp.Ordered](lo, hi T) Range[T] { return rangeset.ClosedOpen(lo, hi) }

// RangeAtLeast returns the interval [lo, +inf), which is never empty.
func RangeAtLeast[T cmp.Ordered](lo T) Range[T] { return rangeset.AtLeast(lo) }

// RangeAtMost returns the interval (-inf, hi], which is never empty.
func RangeAtMost[T cmp.Ordered](hi T) Range[T] { return rangeset.AtMost(hi) }

// RangeGreaterThan returns the interval (lo, +inf), which is never empty.
func RangeGreaterThan[T cmp.Ordered](lo T) Range[T] { return rangeset.GreaterThan(lo) }

// RangeLessThan returns the interval (-inf, hi), which is never empty.
func RangeLessThan[T cmp.Ordered](hi T) Range[T] { return rangeset.LessThan(hi) }

// RangeSingleton returns the interval [v, v], which holds exactly v.
func RangeSingleton[T cmp.Ordered](v T) Range[T] { return rangeset.Singleton(v) }

// RangeAll returns the interval (-inf, +inf), which holds every value.
func RangeAll[T cmp.Ordered]() Range[T] { return rangeset.All[T]() }

// RangeMap maps disjoint ranges to values (com.google.common.collect.RangeMap).
type RangeMap[K cmp.Ordered, V any] = rangemap.RangeMap[K, V]

// RangeMapEntry is one interval and its mapped value.
type RangeMapEntry[K cmp.Ordered, V any] = rangemap.Entry[K, V]

// NewRangeMap creates an empty RangeMap.
func NewRangeMap[K cmp.Ordered, V any]() *RangeMap[K, V] { return rangemap.New[K, V]() }

// Joiner joins strings with skippable or substituted empty values.
type Joiner = text.Joiner

// Splitter splits a string with optional trimming and empty-part handling.
type Splitter = text.Splitter

// NewJoiner creates a joiner that puts sep between the values.
func NewJoiner(sep string) *Joiner {
	return text.NewJoiner(sep)
}

// NewSplitter creates a splitter that cuts on sep.
func NewSplitter(sep string) *Splitter {
	return text.NewSplitter(sep)
}

// JoinStrings joins elems with sep.
func JoinStrings(sep string, elems ...string) string {
	return text.Join(sep, elems...)
}

// JoinStringsAny joins the fmt.Sprint form of elems with sep.
func JoinStringsAny(sep string, elems ...any) string {
	return text.JoinAny(sep, elems...)
}

// JoinStringMap joins a map as "key<kvSep>value" pairs separated by entrySep, in
// sorted key order.
func JoinStringMap(m map[string]string, entrySep, kvSep string) string {
	return text.JoinMap(m, entrySep, kvSep)
}

// SplitString splits str on sep without options.
func SplitString(str, sep string) []string {
	return text.Split(str, sep)
}

// RepeatString returns s repeated n times. It panics when n is negative.
func RepeatString(s string, n int) string {
	return text.Repeat(s, n)
}

// PadStart left-pads s with pad until it is at least minLen bytes long.
func PadStart(s string, minLen int, pad byte) string {
	return text.PadStart(s, minLen, pad)
}

// PadEnd right-pads s with pad until it is at least minLen bytes long.
func PadEnd(s string, minLen int, pad byte) string {
	return text.PadEnd(s, minLen, pad)
}

// Truncate shortens s to at most maxLen bytes, replacing the tail with ellipsis
// when it did not fit.
func Truncate(s string, maxLen int, ellipsis string) string {
	return text.Truncate(s, maxLen, ellipsis)
}

// IsNullOrEmpty reports whether s is empty.
func IsNullOrEmpty(s string) bool { return text.IsNullOrEmpty(s) }

// IsBlank reports whether s is empty or holds only white space.
func IsBlank(s string) bool { return text.IsBlank(s) }

// IsNullOrBlank reports whether s is empty or holds only white space.
func IsNullOrBlank(s string) bool { return text.IsNullOrBlank(s) }

// ToLowerCamel converts words to lowerCamelCase.
func ToLowerCamel(s string) string { return text.ToLowerCamel(s) }

// ToUpperCamel converts words to UpperCamelCase.
func ToUpperCamel(s string) string { return text.ToUpperCamel(s) }

// ToSnakeCase converts words to snake_case.
func ToSnakeCase(s string) string { return text.ToSnakeCase(s) }

// ToKebabCase converts words to kebab-case.
func ToKebabCase(s string) string { return text.ToKebabCase(s) }

// Hasher hashes a key to a 64-bit value; a consistent hash ring uses it to place
// keys on nodes.
type Hasher = hash.Hasher

// FNV1a32 returns the 32-bit FNV-1a hash of b.
func FNV1a32(b []byte) uint32 { return hash.FNV1a32(b) }

// FNV1a64 returns the 64-bit FNV-1a hash of b.
func FNV1a64(b []byte) uint64 { return hash.FNV1a64(b) }

// CRC32 returns the IEEE CRC-32 of b.
func CRC32(b []byte) uint32 { return hash.CRC32(b) }

// CRC32C returns the Castagnoli CRC-32 of b.
func CRC32C(b []byte) uint32 { return hash.CRC32C(b) }

// Adler32 returns the Adler-32 checksum of b.
func Adler32(b []byte) uint32 { return hash.Adler32(b) }

// Murmur3_32 returns the 32-bit MurmurHash3 of b with the given seed.
func Murmur3_32(b []byte, seed uint32) uint32 { return hash.Murmur3_32(b, seed) }

// Murmur3_128 returns the two 64-bit halves of the x64 128-bit MurmurHash3 of b
// with the given seed.
func Murmur3_128(b []byte, seed uint32) (h1, h2 uint64) { return hash.Murmur3_128(b, seed) }

// Sum32 returns the default 32-bit hash of b (MurmurHash3_x86_32 with seed 0).
func Sum32(b []byte) uint32 { return hash.Sum32(b) }

// Sum64 returns the default 64-bit hash of b (the first half of the x64 128-bit
// MurmurHash3 with seed 0).
func Sum64(b []byte) uint64 { return hash.Sum64(b) }

// Precondition checks. Each one panics with an error wrapping ErrInvalidArgument
// or ErrInvalidState, so a recover boundary can classify the failure.

// RequireTrue panics when cond is false.
func RequireTrue(cond bool, format string, args ...any) { verify.True(cond, format, args...) }

// RequireFalse panics when cond is true.
func RequireFalse(cond bool, format string, args ...any) { verify.False(cond, format, args...) }

// RequireState panics when cond is false, wrapping ErrInvalidState.
func RequireState(cond bool, format string, args ...any) { verify.State(cond, format, args...) }

// RequireNotNil returns v, or panics when v is nil.
func RequireNotNil[T any](v *T, format string, args ...any) *T {
	return verify.NotNil(v, format, args...)
}

// RequireNil panics when v is not nil.
func RequireNil[T any](v *T, format string, args ...any) { verify.Nil(v, format, args...) }

// RequireIndex panics unless 0 <= i < size.
func RequireIndex(i, size int, format string, args ...any) { verify.Index(i, size, format, args...) }

// RequirePosition panics unless 0 <= i <= size.
func RequirePosition(i, size int, format string, args ...any) {
	verify.Position(i, size, format, args...)
}

// RequireInRange panics unless lo <= v <= hi.
func RequireInRange[T cmp.Ordered](v, lo, hi T, format string, args ...any) {
	verify.InRange(v, lo, hi, format, args...)
}

// RequirePositive panics unless n > 0.
func RequirePositive(n int, format string, args ...any) { verify.Positive(n, format, args...) }

// RequireNonNegative panics unless n >= 0.
func RequireNonNegative(n int, format string, args ...any) { verify.NonNegative(n, format, args...) }

// Sentinel errors of the verify package, re-exported for errors.Is.
var (
	// ErrInvalidArgument is wrapped by every failed argument check.
	ErrInvalidArgument = verify.ErrInvalidArgument
	// ErrInvalidState is wrapped by a failed RequireState.
	ErrInvalidState = verify.ErrInvalidState
)

// HashRing is a consistent hash ring: keys are placed on a circle of virtual
// nodes, so adding or removing a node only moves the keys it owns.
type HashRing = hashring.Ring

// HashRingOption configures a HashRing.
type HashRingOption = hashring.Option

// HashRingHasher hashes a key to a 64-bit value.
type HashRingHasher = hashring.Hasher

// WithHashRingReplicas sets how many virtual nodes each node contributes.
func WithHashRingReplicas(n int) HashRingOption { return hashring.WithReplicas(n) }

// WithHashRingHasher sets the hash used for keys and virtual nodes.
func WithHashRingHasher(h HashRingHasher) HashRingOption { return hashring.WithHasher(h) }

// NewHashRing creates an empty consistent hash ring.
func NewHashRing(opts ...HashRingOption) *HashRing { return hashring.New(opts...) }

// BiMap is a bidirectional map keeping a key→value mapping and its
// value→key inverse in sync (com.google.common.collect.BiMap): every key is
// bound to at most one value and every value to at most one key.
type BiMap[K comparable, V comparable] = bidimap.BiMap[K, V]

// NewBiMap creates an empty bidirectional map.
func NewBiMap[K comparable, V comparable]() *BiMap[K, V] {
	return bidimap.New[K, V]()
}

// NewBiMapWithCapacity creates an empty bidirectional map pre-sized for
// capacity bindings.
func NewBiMapWithCapacity[K comparable, V comparable](capacity int) *BiMap[K, V] {
	return bidimap.NewWithCapacity[K, V](capacity)
}

// Striped is a fixed-size set of independent mutexes addressed by a
// caller-supplied key hash (com.google.common.util.concurrent.Striped), so
// two keys block each other only when their hashes collide.
type Striped[K any] = striped.Striped[K]

// StripedRW is the read-write variant of Striped: the stripes are RWMutexes,
// so readers of the same key proceed in parallel while writers stay
// exclusive.
type StripedRW[K any] = striped.StripedRW[K]

// NewStriped creates a Striped with at least stripes independent mutexes,
// rounded up to the next power of two; hash maps a key to a 64-bit hash.
func NewStriped[K any](stripes int, hash func(K) uint64) *Striped[K] {
	return striped.New[K](stripes, hash)
}

// NewStripedRW creates a StripedRW with at least stripes independent
// RWMutexes, rounded up to the next power of two; hash maps a key to a
// 64-bit hash.
func NewStripedRW[K any](stripes int, hash func(K) uint64) *StripedRW[K] {
	return striped.NewRW[K](stripes, hash)
}

// AtomicLongMap is a goroutine-safe map from keys to int64 counters
// (com.google.common.util.concurrent.AtomicLongMap): per-key updates need no
// external lock and counter values may be negative.
type AtomicLongMap[K comparable] = atomiclongmap.AtomicLongMap[K]

// NewAtomicLongMap creates an empty counter map.
func NewAtomicLongMap[K comparable]() *AtomicLongMap[K] {
	return atomiclongmap.New[K]()
}

// Stopwatch measures elapsed time across start/stop segments, ignoring the
// gaps in between (com.google.common.base.Stopwatch).
type Stopwatch = stopwatch.Stopwatch

// StopwatchOption configures a Stopwatch.
type StopwatchOption = stopwatch.Option

// WithStopwatchClock sets the time source a Stopwatch measures its segments
// with, which keeps tests deterministic.
func WithStopwatchClock(now func() time.Time) StopwatchOption {
	return stopwatch.WithClock(now)
}

// NewStopwatch creates a stopped Stopwatch with zero elapsed time.
func NewStopwatch(opts ...StopwatchOption) *Stopwatch {
	return stopwatch.New(opts...)
}

// NewStartedStopwatch creates a Stopwatch like NewStopwatch and starts its
// first measurement segment right away.
func NewStartedStopwatch(opts ...StopwatchOption) *Stopwatch {
	return stopwatch.NewStarted(opts...)
}

// Supplier is a goroutine-safe memoizing value supplier
// (com.google.common.base.Suppliers.memoize): Get computes the value exactly
// once and then reuses it.
type Supplier[T any] = memoize.Supplier[T]

// MemoizeStats is a snapshot of a Supplier's hit/miss counters.
type MemoizeStats = memoize.Stats

// WithMemoizeClock sets the time source a Supplier measures expiration with,
// which keeps tests deterministic.
func WithMemoizeClock(now func() time.Time) memoize.Option {
	return memoize.WithClock(now)
}

// Memoize returns a Supplier that runs fn exactly once: the first Get
// computes the value and every later Get returns that same cached value.
func Memoize[T any](fn func() T, opts ...memoize.Option) *Supplier[T] {
	return memoize.Memoize(fn, opts...)
}

// MemoizeExpiring returns a Supplier that caches the value of fn for ttl and
// recomputes it on the first Get after the TTL has elapsed.
func MemoizeExpiring[T any](fn func() T, ttl time.Duration, opts ...memoize.Option) *Supplier[T] {
	return memoize.MemoizeExpiring(fn, ttl, opts...)
}
