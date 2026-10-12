package uuid_test

import (
	"sync"
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/uuid"
	"github.com/stretchr/testify/require"
)

func TestConcurrentEntropyRefillsRemainUnique(t *testing.T) {
	const workers, perWorker = 16, 1000
	ids := make(chan uuid.UUID, workers*perWorker)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perWorker {
				if i%2 == 0 {
					ids <- uuid.NewV4()
				} else {
					ids <- uuid.NewV7At(time.UnixMilli(1234))
				}
			}
		}()
	}
	wg.Wait()
	close(ids)
	seen := make(map[uuid.UUID]struct{}, workers*perWorker)
	for id := range ids {
		require.Equal(t, byte(0x80), id[8]&0xc0)
		require.Contains(t, []int{4, 7}, id.Version())
		_, duplicate := seen[id]
		require.False(t, duplicate, "entropy must not be reused across buffer refills")
		seen[id] = struct{}{}
	}
	require.Len(t, seen, workers*perWorker)
}

func TestAllGeneratorsStayAllocationFreeAcrossRefills(t *testing.T) {
	g := uuid.NewGenerator()
	require.Zero(t, testing.AllocsPerRun(1000, func() { sink = uuid.NewV4() }))
	require.Zero(t, testing.AllocsPerRun(1000, func() { sink = g.Next() }))
	require.Zero(t, testing.AllocsPerRun(1000, func() { sink = uuid.NewV7At(time.UnixMilli(1234)) }))
}
