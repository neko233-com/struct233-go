package timewheel

import (
	"testing"
	"time"
)

func TestCancelledTimersReleaseBucketAndCallback(t *testing.T) {
	w := New(time.Millisecond, 8)
	for _, delay := range []time.Duration{0, time.Hour} {
		timer := w.Schedule(delay, func() {})
		if !timer.Cancel() || timer.fn != nil || timer.bucket != nil {
			t.Fatal("cancelled timer retained its callback or bucket")
		}
	}
	for _, level := range w.buckets {
		for _, bucket := range level {
			if len(bucket) != 0 {
				t.Fatal("bucket retained cancelled timers")
			}
		}
	}
	if len(w.immediate) != 0 {
		t.Fatal("immediate queue retained cancelled timers")
	}
}

func TestStoppedAndFiredTimersReleaseCallbacks(t *testing.T) {
	w := New(time.Millisecond, 8)
	fired := w.Schedule(0, func() {})
	w.AdvanceBy(0)
	stopped := w.Schedule(time.Hour, func() {})
	w.Stop()
	if fired.fn != nil || stopped.fn != nil {
		t.Fatal("completed timer retained its callback")
	}
}
