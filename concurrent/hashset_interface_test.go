package concurrent_test

import (
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/concurrent"
	"github.com/neko233-com/struct233-go/set"
	"github.com/stretchr/testify/require"
)

var _ set.Set[int] = (*concurrent.HashSet[int])(nil)

func TestHashSetSharedInterfaceAndZeroValue(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  set.Set[int]
	}{
		{"constructor", concurrent.NewHashSet[int]()},
		{"zero", new(concurrent.HashSet[int])},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.set
			require.True(t, s.IsEmpty())
			require.True(t, s.ContainsAll())
			s.AddAll()
			s.Clear()
			require.True(t, s.Add(1))
			require.False(t, s.Add(1))
			s.AddAll(2, 3, 2)
			require.Equal(t, 3, s.Len())
			require.True(t, s.ContainsAll(1, 2, 3))
			require.False(t, s.ContainsAll(1, 4))
			require.ElementsMatch(t, []int{1, 2, 3}, s.ToSlice())
			snapshot := s.All()
			require.True(t, s.Remove(2))
			require.False(t, s.Remove(2))
			s.AddAll(4, 5)
			require.ElementsMatch(t, []int{1, 2, 3}, slices.Collect(snapshot))
			s.Clear()
			require.True(t, s.IsEmpty())
			require.Empty(t, slices.Collect(s.All()))
		})
	}
}

func TestZeroHashSetAddAllInitializesStorage(t *testing.T) {
	var s concurrent.HashSet[int]
	s.AddAll(1, 2, 1)
	require.Equal(t, 2, s.Len())
	require.True(t, s.ContainsAll(1, 2))
}

func TestHashSetSnapshotDoesNotHoldLockDuringIteration(t *testing.T) {
	s := concurrent.NewHashSet(1, 2, 3)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for item := range s.All() {
			s.Remove(item)
			s.AddAll(item+10, item+20)
			_ = s.ContainsAll(item+10, item+20)
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("snapshot iteration held a lock during set mutation")
	}
	require.ElementsMatch(t, []int{11, 12, 13, 21, 22, 23}, s.ToSlice())
}

func TestHashSetAddAllIsAtomicForSnapshotReaders(t *testing.T) {
	var s concurrent.HashSet[int]
	items := make([]int, 128)
	for i := range items {
		items[i] = i
	}
	var partial atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 1000 {
			s.AddAll(items...)
			s.Clear()
		}
	}()
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				if n := len(s.ToSlice()); n != 0 && n != len(items) {
					partial.Store(true)
				}
			}
		}()
	}
	wg.Wait()
	require.False(t, partial.Load(), "reader observed a partially inserted batch")
}
