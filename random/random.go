// Package random provides the random helpers game and server code keeps
// re-implementing: bounded ints, weighted picks, ids, and shuffles.
//
//	import "github.com/neko233-com/struct233-go/random"
//
//	// package-level helpers draw from the process-wide generator
//	damage := random.Int(10, 20)
//	id := random.String(16)
//	crit := random.Bool()
//
//	// a Generator owns its source, so a seeded one repeats a run exactly
//	g := random.New(rand.New(rand.NewPCG(seed, 0)))
//	drop := random.Choice(g, loot)
//	random.Shuffle(g, deck)
//
// Every draw from a Generator takes a mutex, so one Generator shared by many
// goroutines is safe even when its source is not; Default is backed by
// math/rand/v2's goroutine-safe global source. The zero Generator value is not
// usable: call New or Default first, otherwise the methods panic.
package random

import (
	"math"
	"math/rand/v2"
	"sort"
	"sync"
)

const (
	// alnum is the character set behind String.
	alnum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// digits is the character set behind Digits.
	digits = "0123456789"
)

// Rand is the random source this package needs. *rand.Rand and the
// math/rand/v2 package-level functions both satisfy it.
type Rand interface {
	// IntN returns a value in [0, n); n must be positive.
	IntN(n int) int
	// Float64 returns a value in [0, 1).
	Float64() float64
}

// Generator draws random values from a source, serialising access with a mutex
// so it is safe for concurrent use even when the source is not.
type Generator struct {
	mu  sync.Mutex // guards src
	src Rand       // nil only on the unusable zero value
}

// New wraps the given source. It panics when src is nil.
func New(src Rand) *Generator {
	if src == nil {
		panic("random: source must not be nil")
	}
	return &Generator{src: src}
}

// globalSource adapts math/rand/v2's package-level functions, which are
// goroutine-safe on their own, to Rand.
type globalSource struct{}

// IntN returns the global source's next value in [0, n).
func (globalSource) IntN(n int) int { return rand.IntN(n) }

// Float64 returns the global source's next value in [0, 1).
func (globalSource) Float64() float64 { return rand.Float64() }

// defaultGenerator is the shared generator behind Default.
var defaultGenerator = &Generator{src: globalSource{}}

// Default returns the process-wide generator, backed by math/rand/v2's
// goroutine-safe global source.
func Default() *Generator { return defaultGenerator }

// orDefault returns g, or Default when g is nil.
func orDefault(g *Generator) *Generator {
	if g == nil {
		return Default()
	}
	return g
}

// begin rejects the zero value and then takes the source lock.
func (g *Generator) begin() {
	if g == nil || g.src == nil {
		panic("random: nil generator")
	}
	g.mu.Lock()
}

// Int returns a uniform int in [min, max]. It panics when max < min.
func (g *Generator) Int(min, max int) int {
	if max < min {
		panic("random: max must not be less than min")
	}
	// the span wraps for the full int range, which is exactly what uintN wants;
	// uint64 arithmetic also keeps the final add correct for negative min.
	return int(uint64(min) + g.uintN(uint64(max)-uint64(min)+1))
}

// Int64 returns a uniform int64 in [min, max]. It panics when max < min.
func (g *Generator) Int64(min, max int64) int64 {
	if max < min {
		panic("random: max must not be less than min")
	}
	return int64(uint64(min) + g.uintN(uint64(max)-uint64(min)+1))
}

// Float64 returns a uniform float64 in [0, 1).
func (g *Generator) Float64() float64 {
	g.begin()
	defer g.mu.Unlock()
	return g.src.Float64()
}

// Bool returns true half of the time.
func (g *Generator) Bool() bool {
	return g.Float64() < 0.5
}

// String returns n random alphanumeric characters ([0-9A-Za-z]). It panics
// when n < 0.
func (g *Generator) String(n int) string { return g.stringFrom(alnum, n) }

// StringFrom returns n random characters drawn from alphabet. It panics when
// alphabet is empty or n < 0.
func (g *Generator) StringFrom(alphabet string, n int) string {
	return g.stringFrom(alphabet, n)
}

// Digits returns n random decimal digits. It panics when n < 0.
func (g *Generator) Digits(n int) string { return g.stringFrom(digits, n) }

// stringFrom builds the result in one byte slice and converts it once, so a
// call allocates the buffer and the string and nothing else.
func (g *Generator) stringFrom(alphabet string, n int) string {
	if alphabet == "" {
		panic("random: alphabet must not be empty")
	}
	if n < 0 {
		panic("random: n must not be negative")
	}
	if n == 0 {
		return ""
	}
	g.begin()
	defer g.mu.Unlock()
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = alphabet[int(g.uintNLocked(uint64(len(alphabet))))]
	}
	return string(buf)
}

// uintN returns a uniform uint64 in [0, span); span == 0 means the full range.
func (g *Generator) uintN(span uint64) uint64 {
	g.begin()
	defer g.mu.Unlock()
	return g.uintNLocked(span)
}

// uintNLocked draws a uniform uint64 in [0, span); span == 0 means the full
// 64-bit range. Callers must hold g.mu.
func (g *Generator) uintNLocked(span uint64) uint64 {
	switch {
	case span == 0:
		return g.uint64Locked()
	case span <= math.MaxInt:
		// WHY: IntN is already modulo-free, it rejects the tail of its stream
		// instead of taking a remainder, so the common ranges stay unbiased.
		return uint64(g.src.IntN(int(span)))
	default:
		// Spans wider than IntN accepts cannot be drawn in one call: reject the
		// tail here so the accepted count is a whole multiple of span and the
		// final remainder cannot skew the result.
		rem := (math.MaxUint64%span + 1) % span // 2^64 mod span
		if rem == 0 {
			return g.uint64Locked() % span
		}
		cutoff := uint64(0) - rem // 2^64-rem, the largest uniform prefix
		for {
			if v := g.uint64Locked(); v < cutoff {
				return v % span
			}
		}
	}
}

// uint64Locked draws a uniform uint64 from four 16-bit draws: 16*4 bits is
// exact on every platform, unlike a 32-bit chunk, whose constant does not fit
// an int on 32-bit targets. Callers must hold g.mu.
func (g *Generator) uint64Locked() uint64 {
	a := uint64(g.src.IntN(1 << 16))
	b := uint64(g.src.IntN(1 << 16))
	c := uint64(g.src.IntN(1 << 16))
	d := uint64(g.src.IntN(1 << 16))
	return a<<48 | b<<32 | c<<16 | d
}

// Int is Default().Int.
func Int(min, max int) int { return Default().Int(min, max) }

// Int64 is Default().Int64.
func Int64(min, max int64) int64 { return Default().Int64(min, max) }

// Float64 is Default().Float64.
func Float64() float64 { return Default().Float64() }

// Bool is Default().Bool.
func Bool() bool { return Default().Bool() }

// String is Default().String.
func String(n int) string { return Default().String(n) }

// StringFrom is Default().StringFrom.
func StringFrom(alphabet string, n int) string { return Default().StringFrom(alphabet, n) }

// Digits is Default().Digits.
func Digits(n int) string { return Default().Digits(n) }

// Shuffle shuffles items in place. A nil g means Default().
func Shuffle[T any](g *Generator, items []T) {
	g = orDefault(g)
	g.begin()
	defer g.mu.Unlock()
	// Fisher-Yates from the back: every permutation is equally likely.
	for i := len(items) - 1; i > 0; i-- {
		j := int(g.uintNLocked(uint64(i + 1)))
		items[i], items[j] = items[j], items[i]
	}
}

// Choice returns a uniform element of items; ok is false when items is empty.
// A nil g means Default().
func Choice[T any](g *Generator, items []T) (T, bool) {
	if len(items) == 0 {
		var zero T
		return zero, false
	}
	g = orDefault(g)
	g.begin()
	defer g.mu.Unlock()
	return items[int(g.uintNLocked(uint64(len(items))))], true
}

// Sample returns up to n distinct elements of items in random order (without
// replacement). It panics when n < 0. A nil g means Default().
func Sample[T any](g *Generator, items []T, n int) []T {
	if n < 0 {
		panic("random: n must not be negative")
	}
	if n > len(items) {
		n = len(items)
	}
	if n == 0 {
		return []T{}
	}
	// copy first: the caller's slice must survive the partial shuffle
	out := make([]T, len(items))
	copy(out, items)
	g = orDefault(g)
	g.begin()
	defer g.mu.Unlock()
	// a partial Fisher-Yates leaves a uniform subset in the first n positions
	for i := range n {
		j := i + int(g.uintNLocked(uint64(len(out)-i)))
		out[i], out[j] = out[j], out[i]
	}
	return out[:n]
}

// Weighted returns an element picked proportionally to weight; ok is false when
// items is empty or every weight is <= 0. Negative weights are treated as zero.
// A nil g means Default(). It is O(n) per pick: use WeightedTable for repeated
// picks.
func Weighted[T any](g *Generator, items []T, weight func(T) float64) (T, bool) {
	var zero T
	if len(items) == 0 {
		return zero, false
	}
	if weight == nil {
		panic("random: weight function must not be nil")
	}
	var total float64
	for _, item := range items {
		if w := weight(item); w > 0 {
			total += w
		}
	}
	if total <= 0 {
		return zero, false
	}
	pick := orDefault(g).Float64() * total
	var acc float64
	last, found := zero, false
	for _, item := range items {
		if w := weight(item); w > 0 {
			if acc += w; pick < acc {
				return item, true
			}
			// keep the last positive bucket for the float rounding case where
			// pick lands a hair above the accumulated total
			last, found = item, true
		}
	}
	return last, found
}

// WeightedTable is a precomputed cumulative-weight table for repeated picks
// from a fixed set (drop tables, spawn tables). Pick is O(log n) and does not
// allocate.
type WeightedTable[T any] struct {
	gen    *Generator // source for Pick, never nil
	items  []T        // the items, in the caller's order
	prefix []float64  // cumulative weight: prefix[i] = sum of weights[0..i]
	total  float64    // prefix[len-1], the sum of every weight
}

// NewWeightedTable builds a picker over items. It panics when items is empty,
// when weight is nil, or when the total weight is <= 0. A nil g means
// Default(). The items are copied, so later changes to the caller's slice are
// not seen.
func NewWeightedTable[T any](g *Generator, items []T, weight func(T) float64) *WeightedTable[T] {
	if len(items) == 0 {
		panic("random: items must not be empty")
	}
	if weight == nil {
		panic("random: weight function must not be nil")
	}
	t := &WeightedTable[T]{
		gen:    orDefault(g),
		items:  make([]T, len(items)),
		prefix: make([]float64, len(items)),
	}
	var total float64
	for i, item := range items {
		w := weight(item)
		if w < 0 {
			w = 0 // same rule as Weighted: negative weights are zero
		}
		total += w
		t.items[i] = item
		t.prefix[i] = total
	}
	if total <= 0 {
		panic("random: total weight must be positive")
	}
	t.total = total
	return t
}

// Pick returns a random element and reports success; it returns the zero value
// and false only for an empty table.
func (t *WeightedTable[T]) Pick() (T, bool) {
	var zero T
	if t == nil || len(t.items) == 0 {
		return zero, false
	}
	idx := t.pickIndex(t.gen.Float64() * t.total)
	if idx >= len(t.items) {
		// only reachable through float rounding at the very top of the range
		idx = len(t.items) - 1
	}
	return t.items[idx], true
}

// pickIndex returns the first index whose cumulative weight exceeds pick, which
// is what keeps zero-weight items unreachable.
func (t *WeightedTable[T]) pickIndex(pick float64) int {
	// WHY: > rather than sort.SearchFloat64s' >=: a zero weight repeats the
	// previous cumulative value, and >= would hand that item out at the lower
	// boundary. The closure does not escape, so Pick stays allocation-free.
	return sort.Search(len(t.prefix), func(i int) bool { return t.prefix[i] > pick })
}

// Items returns how many elements the table holds.
func (t *WeightedTable[T]) Items() int {
	if t == nil {
		return 0
	}
	return len(t.items)
}
