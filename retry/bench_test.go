package retry_test

import (
	"context"
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/retry"
)

// noSleep skips the backoff wait while still honouring a done context, so a
// benchmark measures the retry bookkeeping instead of the timer.
func noSleep(ctx context.Context, _ time.Duration) error { return ctx.Err() }

// BenchmarkPolicyDoFirstAttempt measures the happy path of a reusable policy:
// fn succeeds immediately, so no backoff is ever taken.
func BenchmarkPolicyDoFirstAttempt(b *testing.B) {
	p := retry.New(retry.WithSleep(noSleep))
	ctx := context.Background()
	attempt := func(context.Context) error { return nil }

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if err := p.Do(ctx, attempt); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDoValue measures the value-returning helper on the happy path,
// including building the throwaway policy from its options.
func BenchmarkDoValue(b *testing.B) {
	ctx := context.Background()
	attempt := func(context.Context) (int, error) { return 42, nil }

	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		v, err := retry.DoValue(ctx, attempt, retry.WithSleep(noSleep))
		if err != nil {
			b.Fatal(err)
		}
		if v != 42 {
			b.Fatalf("got %d, want 42", v)
		}
	}
}
