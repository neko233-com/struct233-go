package memoize

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInvalidateReleasesCachedReference(t *testing.T) {
	s := Memoize(func() *int { return new(int) })
	require.NotNil(t, s.Get())
	s.Invalidate()
	require.Nil(t, s.value, "invalidating must release the cached object immediately")
	require.False(t, s.computed)
	require.NotNil(t, s.Get())
}

func TestExpiredSupplierPanicReleasesOldValue(t *testing.T) {
	now := time.Unix(100, 0)
	panicNext := false
	s := MemoizeExpiring(func() *int {
		if panicNext {
			panic("failed refresh")
		}
		return new(int)
	}, time.Second, WithClock(func() time.Time { return now }))
	s.Get()
	now = now.Add(time.Second)
	panicNext = true
	require.PanicsWithValue(t, "failed refresh", func() { s.Get() })
	require.Nil(t, s.value)
	require.False(t, s.computed)
	panicNext = false
	require.NotNil(t, s.Get())
}

func TestClockPanicDoesNotPoisonSupplier(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(map[bool]string{false: "publication", true: "freshness"}[cached], func(t *testing.T) {
			panicClock := false
			s := MemoizeExpiring(func() int { return 42 }, time.Second, WithClock(func() time.Time {
				if panicClock {
					panic("clock failed")
				}
				return time.Unix(100, 0)
			}))
			if cached {
				s.Get()
			}
			panicClock = true
			require.PanicsWithValue(t, "clock failed", func() { s.Get() })
			unlocked := s.mu.TryLock()
			if unlocked {
				s.mu.Unlock()
			}
			require.True(t, unlocked, "a panicking clock must release the supplier lock")
			require.Equal(t, cached, s.computed, "failed publication must not cache the value")
			panicClock = false
			require.Equal(t, 42, s.Get())
		})
	}
}
