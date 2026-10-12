package snowflake_test

import (
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/snowflake"
)

// FuzzSnowflakeStream drives a generator with a fuzzer-chosen node width,
// node id and clock progression, then requires the produced stream to satisfy
// the formula-inverse model: every id decodes back to its node, its sequence
// budget and its timestamp, and the stream is strictly increasing.
func FuzzSnowflakeStream(f *testing.F) {
	f.Add([]byte{10, 7, 1, 1, 2, 3, 4, 5})
	f.Add([]byte{22, 63, 0, 255, 255, 0, 1, 2})
	f.Add([]byte{1, 1, 2, 3, 5, 8, 13, 21})
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{21, 42, 100, 1, 2, 3, 4, 5})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 3 {
			return
		}
		epoch := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

		// byte 0 selects the node width (1..22), byte 1 the node id
		nodeBits := uint8(1 + int(data[0])%22)
		node := int64(data[1]) % (int64(1) << nodeBits)
		// byte 2 selects how many times the clock advances, the rest select
		// the advance amount in milliseconds
		advances := 1 + int(data[2])%8
		stepMS := int64(1)
		if len(data) > 3 {
			stepMS = 1 + int64(data[3])%4
		}

		start := epoch.Add(365 * 24 * time.Hour)
		clock := newAdvancingClock(start, time.Duration(stepMS)*time.Millisecond)
		g := snowflake.New(node,
			snowflake.WithEpoch(epoch), snowflake.WithNodeBits(nodeBits),
			snowflake.WithClock(clock.Now))
		maxSeq := int64(1)<<(22-nodeBits) - 1

		var prev snowflake.ID
		for i := range 64 {
			// periodically jump the clock forward like the fuzzer asks
			if i%8 == 7 && advances > 0 {
				clock.Set(clock.Now().Add(time.Duration(stepMS) * time.Millisecond))
				advances--
			}
			id, err := g.Next()
			if err != nil {
				t.Fatalf("Next #%d: %v", i, err)
			}
			if got := g.NodeOf(id); got != node {
				t.Fatalf("NodeOf(%d) = %d, want %d", id.Int64(), got, node)
			}
			if got := g.SequenceOf(id); got < 0 || got > maxSeq {
				t.Fatalf("SequenceOf(%d) = %d, want in [0, %d]", id.Int64(), got, maxSeq)
			}
			if got := g.TimeOf(id).Sub(epoch).Milliseconds(); got != int64(id)>>22 {
				t.Fatalf("TimeOf(%d) = %d ms, want %d ms", id.Int64(), got, int64(id)>>22)
			}
			if i > 0 && id <= prev {
				t.Fatalf("ids must strictly increase: %d then %d", prev.Int64(), id.Int64())
			}
			prev = id
		}
	})
}
