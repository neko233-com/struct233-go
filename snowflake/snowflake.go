// Package snowflake generates unique, time-ordered 64-bit identifiers without
// a central coordinator, following Twitter's snowflake layout.
//
//	import "github.com/neko233-com/struct233-go/snowflake"
//
//	g := snowflake.New(7)
//	id, err := g.Next()
//	if err == nil {
//		log.Printf("%d node=%d seq=%d at %s", id.Int64(), g.NodeOf(id), g.SequenceOf(id), g.TimeOf(id))
//	}
//
// A Generator is safe for concurrent use: the IDs it issues are strictly
// increasing and unique, so they sort by creation time. When the clock jumps
// backwards Next reports ErrClockBackwards instead of repeating an ID.
//
// The layout, high to low, is 1 unused sign bit, 41 timestamp bits, nodeBits
// node bits and 22-nodeBits sequence bits; the timestamp is counted in
// milliseconds from the configured epoch.
package snowflake

import (
	"errors"
	"sync"
	"time"
)

// ID is a 64-bit snowflake identifier: 1 unused sign bit, then a configurable
// number of timestamp bits, node bits and sequence bits.
type ID int64

// Int64 returns the ID as a plain int64 (useful for storage/keys).
func (id ID) Int64() int64 { return int64(id) }

const (
	// dynamicBits is the number of bits shared by the node id and the
	// sequence; the 41 timestamp bits are the rest of the 63 usable bits.
	dynamicBits = 22
	// defaultNodeBits is the node width used when WithNodeBits is not given.
	defaultNodeBits = 10
	// waitSpin is the pause between clock reads while waiting for the next
	// millisecond to arrive.
	waitSpin = 100 * time.Microsecond
)

// defaultEpoch is the epoch used when WithEpoch is not given.
var defaultEpoch = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

// ErrClockBackwards is returned by Next when the clock moved backwards.
var ErrClockBackwards = errors.New("snowflake: clock moved backwards")

// Generator produces unique, time-ordered snowflake IDs.
type Generator struct {
	mu       sync.Mutex
	epoch    time.Time
	now      func() time.Time
	node     int64
	nodeBits uint8
	seqBits  uint8
	maxNode  int64
	maxSeq   int64
	lastMS   int64 // milliseconds of the last issued ID, -1 before the first
	seq      int64 // sequence used within lastMS
}

// Option configures a Generator.
type Option func(*Generator)

// WithEpoch sets the custom epoch that timestamps are measured from.
// Default: 2020-01-01T00:00:00Z.
func WithEpoch(t time.Time) Option {
	return func(g *Generator) { g.epoch = t }
}

// WithNodeBits sets the node-id width in bits (1..22). Default 10.
// The sequence width is 22-nodeBits, so the ID layout always stays 63 bits.
func WithNodeBits(bits uint8) Option {
	return func(g *Generator) { g.nodeBits = bits }
}

// WithClock sets the time source. Default: time.Now.
// Injecting a clock keeps tests deterministic.
func WithClock(now func() time.Time) Option {
	return func(g *Generator) { g.now = now }
}

// New creates a Generator for the given node id (0 <= node < 1<<nodeBits).
// It panics if opts are invalid (e.g. node out of range, epoch in the future,
// node bits outside 1..22).
func New(node int64, opts ...Option) *Generator {
	g := &Generator{
		epoch:    defaultEpoch,
		now:      time.Now,
		node:     node,
		nodeBits: defaultNodeBits,
		lastMS:   -1,
	}
	for _, opt := range opts {
		opt(g)
	}
	if g.nodeBits < 1 || g.nodeBits > dynamicBits {
		panic("snowflake: node bits must be in 1..22")
	}
	g.seqBits = dynamicBits - g.nodeBits
	g.maxNode = (int64(1) << g.nodeBits) - 1
	g.maxSeq = (int64(1) << g.seqBits) - 1
	if node < 0 || node > g.maxNode {
		panic("snowflake: node id out of range")
	}
	if g.now == nil {
		panic("snowflake: nil clock")
	}
	if g.epoch.After(g.now()) {
		panic("snowflake: epoch must not be in the future")
	}
	return g
}

// Node returns the configured node id.
func (g *Generator) Node() int64 { return g.node }

// NodeBits returns the number of bits reserved for the node id.
func (g *Generator) NodeBits() uint8 { return g.nodeBits }

// Epoch returns the configured epoch.
func (g *Generator) Epoch() time.Time { return g.epoch }

// Next returns the next ID, waiting for the next millisecond when the
// sequence for the current millisecond is exhausted. It returns
// ErrClockBackwards when the clock moved backwards since the last call.
//
// Next blocks while the clock does not advance, so an injected clock must
// eventually move forward for the call to return.
func (g *Generator) Next() (ID, error) {
	ms := g.sinceEpoch(g.now())
	if ms < 0 {
		return 0, ErrClockBackwards
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if ms < g.lastMS {
		// ms was read before the lock, so a concurrent caller may have
		// advanced lastMS in the meantime: re-read to tell a stale reading
		// apart from a genuine rollback.
		if ms = g.sinceEpoch(g.now()); ms < g.lastMS {
			return 0, ErrClockBackwards
		}
	}
	if ms > g.lastMS {
		g.lastMS, g.seq = ms, 0
	} else {
		g.seq++
		if g.seq > g.maxSeq {
			// the millisecond is exhausted: keep re-reading the (possibly
			// injected) clock until it moves on
			for ms <= g.lastMS {
				time.Sleep(waitSpin)
				ms = g.sinceEpoch(g.now())
			}
			g.lastMS, g.seq = ms, 0
		}
	}
	return g.compose(g.lastMS, g.seq), nil
}

// MustNext is Next, panicking on error.
func (g *Generator) MustNext() ID {
	id, err := g.Next()
	if err != nil {
		panic(err) // the error already carries the "snowflake: " prefix
	}
	return id
}

// TimeOf extracts the generation time of an ID.
func (g *Generator) TimeOf(id ID) time.Time {
	return g.epoch.Add(time.Duration(int64(id)>>dynamicBits) * time.Millisecond)
}

// NodeOf extracts the node id stored in an ID.
func (g *Generator) NodeOf(id ID) int64 {
	return (int64(id) >> g.seqBits) & g.maxNode
}

// SequenceOf extracts the sequence number stored in an ID.
func (g *Generator) SequenceOf(id ID) int64 {
	return int64(id) & g.maxSeq
}

// compose packs a millisecond timestamp, the node id and the sequence into
// one ID.
func (g *Generator) compose(ms, seq int64) ID {
	return ID(ms<<dynamicBits | g.node<<g.seqBits | seq)
}

// sinceEpoch converts a clock reading into milliseconds since the epoch.
func (g *Generator) sinceEpoch(t time.Time) int64 {
	return t.Sub(g.epoch).Milliseconds()
}
