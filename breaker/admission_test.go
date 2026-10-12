package breaker

import (
	"errors"
	"testing"
	"time"
)

func TestExecuteClockPanicDoesNotRetainAdmissionLock(t *testing.T) {
	panicClock := false
	b := New(WithFailureThreshold(1), WithClock(func() time.Time {
		if panicClock {
			panic("clock failed")
		}
		return time.Unix(0, 0)
	}))
	_ = b.Execute(func() error { return errors.New("open circuit") })
	panicClock = true
	func() {
		defer func() {
			if got := recover(); got != "clock failed" {
				t.Errorf("clock panic = %v; want clock failed", got)
			}
		}()
		_ = b.Execute(func() error {
			t.Error("callback must not run after admission panics")
			return nil
		})
	}()
	if !b.mu.TryLock() {
		t.Fatal("Execute retained the mutex after its admission clock panicked")
	}
	b.mu.Unlock()
	b.Reset()
	if b.State() != StateClosed || b.Counts() != (Counts{}) {
		t.Fatal("breaker must remain resettable after a recovered admission panic")
	}
}
