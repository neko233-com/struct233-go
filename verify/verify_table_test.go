package verify_test

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/verify"
)

// dualTrackCase drives one check through both branches of the "returns vs
// panics" contract: a passing call must not panic, a failing call must panic
// with an error that wraps the expected sentinel and carries the rendered
// message.
type dualTrackCase struct {
	name     string
	pass     bool
	sentinel error // sentinel expected when the check panics
	want     string
	fn       func()
}

func runDualTrack(t *testing.T, cases []dualTrackCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.pass {
				require.NotPanics(t, tc.fn)
				return
			}
			requirePanicsError(t, tc.sentinel, tc.want, tc.fn)
		})
	}
}

// TestTrueDualTrack covers the passing/failing branches with argument counts
// from zero to several.
func TestTrueDualTrack(t *testing.T) {
	runDualTrack(t, []dualTrackCase{
		{name: "pass no args", pass: true, fn: func() { verify.True(1 > 0, "unreachable") }},
		{name: "pass with args", pass: true, fn: func() { verify.True(true, "unreachable %d", 1) }},
		{name: "fail no args", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: cond must hold", fn: func() { verify.True(false, "cond must hold") }},
		{name: "fail one arg", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: value -1 is bad", fn: func() { verify.True(false, "value %d is bad", -1) }},
		{name: "fail two args", sentinel: verify.ErrInvalidArgument,
			want: `invalid argument: pair ("a", 2) rejected`,
			fn:   func() { verify.True(false, "pair (%q, %d) rejected", "a", 2) }},
		{name: "fail three args", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: [1 2] between 0 and 3 is false",
			fn:   func() { verify.False(true, "%v between %d and %d is %v", []int{1, 2}, 0, 3, false) }},
	})
}

// TestFalseDualTrack mirrors TestTrueDualTrack with the inverted condition.
func TestFalseDualTrack(t *testing.T) {
	runDualTrack(t, []dualTrackCase{
		{name: "pass no args", pass: true, fn: func() { verify.False(false, "unreachable") }},
		{name: "pass with args", pass: true, fn: func() { verify.False(1 > 2, "unreachable %q", "x") }},
		{name: "fail no args", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: flag must stay clear", fn: func() { verify.False(true, "flag must stay clear") }},
		{name: "fail one arg", sentinel: verify.ErrInvalidArgument,
			want: `invalid argument: flag "debug" is set`,
			fn:   func() { verify.False(true, "flag %q is set", "debug") }},
		{name: "fail two args", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: got 1 and 2",
			fn:   func() { verify.False(true, "got %d and %d", 1, 2) }},
	})
}

// TestStateDualTrack covers the ErrInvalidState classification on both
// branches.
func TestStateDualTrack(t *testing.T) {
	runDualTrack(t, []dualTrackCase{
		{name: "pass no args", pass: true, fn: func() { verify.State(true, "unreachable") }},
		{name: "pass with args", pass: true, fn: func() { verify.State(2 > 1, "unreachable %v", 2) }},
		{name: "fail no args", sentinel: verify.ErrInvalidState,
			want: "invalid state: connection closed", fn: func() { verify.State(false, "connection closed") }},
		{name: "fail one arg", sentinel: verify.ErrInvalidState,
			want: "invalid state: closed after 3 retries",
			fn:   func() { verify.State(false, "closed after %d retries", 3) }},
		{name: "fail two args", sentinel: verify.ErrInvalidState,
			want: `invalid state: worker "w1" cannot serve 7`,
			fn:   func() { verify.State(false, "worker %q cannot serve %d", "w1", 7) }},
	})
}

// TestNotNilDualTrack covers the pointer identity contract on both branches.
func TestNotNilDualTrack(t *testing.T) {
	x := 5
	runDualTrack(t, []dualTrackCase{
		{name: "pass plain pointer", pass: true, fn: func() { verify.NotNil(&x, "unreachable") }},
		{name: "pass formatted", pass: true, fn: func() { verify.NotNil(&x, "unreachable %d", x) }},
		{
			name: "fail nil no args", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: value must not be nil",
			fn: func() {
				var p *int
				verify.NotNil(p, "value must not be nil")
			},
		},
		{
			name: "fail nil with args", sentinel: verify.ErrInvalidArgument,
			want: `invalid argument: field "id" of user 9 must not be nil`,
			fn: func() {
				var p *string
				verify.NotNil(p, "field %q of user %d must not be nil", "id", 9)
			},
		},
	})
}

// TestNilDualTrack covers the inverted pointer contract on both branches.
func TestNilDualTrack(t *testing.T) {
	x := 5
	runDualTrack(t, []dualTrackCase{
		{name: "pass nil pointer", pass: true, fn: func() {
			var p *int
			verify.Nil(p, "unreachable")
		}},
		{name: "pass nil with args", pass: true, fn: func() {
			var p *struct{}
			verify.Nil(p, "unreachable %v", 1)
		}},
		{name: "fail live pointer", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: cache must be nil",
			fn:   func() { verify.Nil(&x, "cache must be nil") }},
		{name: "fail live pointer with args", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: expected nil, found 5",
			fn:   func() { verify.Nil(&x, "expected nil, found %d", x) }},
	})
}

// TestIndexDualTrack covers the half-open bound contract on both branches.
func TestIndexDualTrack(t *testing.T) {
	runDualTrack(t, []dualTrackCase{
		{name: "pass i==0 size==1", pass: true, fn: func() { verify.Index(0, 1, "unreachable") }},
		{name: "pass middle", pass: true, fn: func() { verify.Index(2, 5, "unreachable %d", 2) }},
		{name: "pass last element", pass: true, fn: func() { verify.Index(4, 5, "unreachable") }},
		{name: "fail i==size", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: index 5 out of [0, 5)",
			fn:   func() { verify.Index(5, 5, "index %d out of [0, %d)", 5, 5) }},
		{name: "fail negative i", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: index -1 out of [0, 3)",
			fn:   func() { verify.Index(-1, 3, "index %d out of [0, %d)", -1, 3) }},
		{name: "fail empty size", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: index 0 out of [0, 0)",
			fn:   func() { verify.Index(0, 0, "index %d out of [0, %d)", 0, 0) }},
		{name: "fail negative size", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: index 0 out of [0, -2)",
			fn:   func() { verify.Index(0, -2, "index %d out of [0, %d)", 0, -2) }},
	})
}

// TestPositionDualTrack covers the closed insertion-point contract on both
// branches.
func TestPositionDualTrack(t *testing.T) {
	runDualTrack(t, []dualTrackCase{
		{name: "pass empty append", pass: true, fn: func() { verify.Position(0, 0, "unreachable") }},
		{name: "pass append at end", pass: true, fn: func() { verify.Position(5, 5, "unreachable %d", 5) }},
		{name: "pass middle", pass: true, fn: func() { verify.Position(1, 5, "unreachable") }},
		{name: "fail one past end", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: position 6 out of [0, 5]",
			fn:   func() { verify.Position(6, 5, "position %d out of [0, %d]", 6, 5) }},
		{name: "fail negative", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: position -3 out of [0, 4]",
			fn:   func() { verify.Position(-3, 4, "position %d out of [0, %d]", -3, 4) }},
		{name: "fail empty one past", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: position 1 out of [0, 0]",
			fn:   func() { verify.Position(1, 0, "position %d out of [0, %d]", 1, 0) }},
	})
}

// TestInRangeDualTrack covers closed-interval membership on both branches for
// the default int instantiation plus float edge behaviour.
func TestInRangeDualTrack(t *testing.T) {
	runDualTrack(t, []dualTrackCase{
		{name: "pass v==lo", pass: true, fn: func() { verify.InRange(0, 0, 5, "unreachable") }},
		{name: "pass v==hi", pass: true, fn: func() { verify.InRange(5, 0, 5, "unreachable") }},
		{name: "pass middle", pass: true, fn: func() { verify.InRange(3, 0, 5, "unreachable %d", 3) }},
		{name: "fail below", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: value -1 not in [0, 5]",
			fn:   func() { verify.InRange(-1, 0, 5, "value %d not in [%d, %d]", -1, 0, 5) }},
		{name: "fail above", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: value 6 not in [0, 5]",
			fn:   func() { verify.InRange(6, 0, 5, "value %d not in [%d, %d]", 6, 0, 5) }},
		{name: "fail inverted bounds", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: value 2 not in [5, 1]",
			fn:   func() { verify.InRange(2, 5, 1, "value %d not in [%d, %d]", 2, 5, 1) }},
		{name: "pass float edges", pass: true, fn: func() { verify.InRange(0.5, 0.0, 1.0, "unreachable") }},
		{name: "pass nan is documented to pass", pass: true,
			fn: func() { verify.InRange(math.NaN(), 0.0, 1.0, "unreachable") }},
		{name: "fail float below", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: value -0.25 not in [0, 1]",
			fn:   func() { verify.InRange(-0.25, 0.0, 1.0, "value %v not in [%v, %v]", -0.25, 0.0, 1.0) }},
		{name: "pass string in range", pass: true, fn: func() { verify.InRange("m", "a", "z", "unreachable") }},
		{name: "fail string below", sentinel: verify.ErrInvalidArgument,
			want: `invalid argument: value "A" not in ["a", "z"]`,
			fn:   func() { verify.InRange("A", "a", "z", "value %q not in [%q, %q]", "A", "a", "z") }},
	})
}

// TestPositiveDualTrack covers the n > 0 contract on both branches.
func TestPositiveDualTrack(t *testing.T) {
	runDualTrack(t, []dualTrackCase{
		{name: "pass one", pass: true, fn: func() { verify.Positive(1, "unreachable") }},
		{name: "pass large", pass: true, fn: func() { verify.Positive(math.MaxInt, "unreachable %d", math.MaxInt) }},
		{name: "fail zero", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: count 0 must be > 0",
			fn:   func() { verify.Positive(0, "count %d must be > 0", 0) }},
		{name: "fail minus one", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: count -1 must be > 0",
			fn:   func() { verify.Positive(-1, "count %d must be > 0", -1) }},
		{name: "fail min int", sentinel: verify.ErrInvalidArgument,
			want: fmt.Sprintf("invalid argument: count %d must be > 0", math.MinInt),
			fn:   func() { verify.Positive(math.MinInt, "count %d must be > 0", math.MinInt) }},
	})
}

// TestNonNegativeDualTrack covers the n >= 0 contract on both branches.
func TestNonNegativeDualTrack(t *testing.T) {
	runDualTrack(t, []dualTrackCase{
		{name: "pass zero", pass: true, fn: func() { verify.NonNegative(0, "unreachable") }},
		{name: "pass one", pass: true, fn: func() { verify.NonNegative(1, "unreachable %d", 1) }},
		{name: "fail minus one", sentinel: verify.ErrInvalidArgument,
			want: "invalid argument: length -1 must be >= 0",
			fn:   func() { verify.NonNegative(-1, "length %d must be >= 0", -1) }},
		{name: "fail min int", sentinel: verify.ErrInvalidArgument,
			want: fmt.Sprintf("invalid argument: length %d must be >= 0", math.MinInt),
			fn:   func() { verify.NonNegative(math.MinInt, "length %d must be >= 0", math.MinInt) }},
	})
}

// TestSentinelClassification checks that every argument check wraps only
// ErrInvalidArgument and State wraps only ErrInvalidState, through errors.Is
// after a recover.
func TestSentinelClassification(t *testing.T) {
	argChecks := map[string]func(){
		"True":     func() { verify.True(false, "x") },
		"False":    func() { verify.False(true, "x") },
		"NotNil":   func() { verify.NotNil((*int)(nil), "x") },
		"Nil":      func() { verify.Nil(new(int), "x") },
		"Index":    func() { verify.Index(2, 1, "x") },
		"Position": func() { verify.Position(2, 1, "x") },
		"InRange":  func() { verify.InRange(2, 0, 1, "x") },
		"Positive": func() { verify.Positive(0, "x") },
		"NonNegative": func() {
			verify.NonNegative(-1, "x")
		},
	}
	for name, fn := range argChecks {
		t.Run(name+" wraps ErrInvalidArgument only", func(t *testing.T) {
			rec := catchPanic(fn)
			err, ok := rec.(error)
			require.Truef(t, ok, "panic value %v has type %T, want error", rec, rec)
			require.ErrorIs(t, err, verify.ErrInvalidArgument)
			require.NotErrorIs(t, err, verify.ErrInvalidState)
		})
	}

	t.Run("State wraps ErrInvalidState only", func(t *testing.T) {
		rec := catchPanic(func() { verify.State(false, "x") })
		err, ok := rec.(error)
		require.Truef(t, ok, "panic value %v has type %T, want error", rec, rec)
		require.ErrorIs(t, err, verify.ErrInvalidState)
		require.NotErrorIs(t, err, verify.ErrInvalidArgument)
	})
}

// TestRandomParameterLegality replays random parameter combinations through
// every check and requires the model's pass/fail prediction to match the
// observed behaviour (the same differential idea as the fuzz target).
func TestRandomParameterLegality(t *testing.T) {
	for seed := uint64(0); seed < 30; seed++ {
		rng := rand.New(rand.NewPCG(seed, seed*17+3))
		i := rng.IntN(21) - 10
		size := rng.IntN(21) - 10
		n := rng.IntN(21) - 10
		v := rng.IntN(21) - 10
		lo, hi := rng.IntN(11)-5, rng.IntN(11)-5
		cond := rng.IntN(2) == 0

		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			// model predictions
			indexOK := i >= 0 && i < size
			posOK := i >= 0 && i <= size
			rangeOK := v >= lo && v <= hi
			pos := n > 0
			nonNeg := n >= 0

			require.Equal(t, indexOK, notPanics(func() { verify.Index(i, size, "i") }))
			require.Equal(t, posOK, notPanics(func() { verify.Position(i, size, "p") }))
			require.Equal(t, rangeOK, notPanics(func() { verify.InRange(v, lo, hi, "r") }))
			require.Equal(t, pos, notPanics(func() { verify.Positive(n, "pos") }))
			require.Equal(t, nonNeg, notPanics(func() { verify.NonNegative(n, "nn") }))
			require.Equal(t, cond, notPanics(func() { verify.True(cond, "t") }))
			require.Equal(t, !cond, notPanics(func() { verify.False(cond, "f") }))
			require.Equal(t, cond, notPanics(func() { verify.State(cond, "s") }))
		})
	}
}

// notPanics reports whether fn returned without panicking.
func notPanics(fn func()) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	fn()
	return true
}

// TestNilNotNilPointerIdentityModel checks the pointer-only semantics of
// NotNil/Nil against the model "nil pointer vs non-nil pointer".
func TestNilNotNilPointerIdentityModel(t *testing.T) {
	x := 0
	var nilP *int
	cases := []struct {
		name     string
		p        *int
		notNilOK bool
	}{
		{"live pointer", &x, true},
		{"nil pointer", nilP, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.notNilOK, notPanics(func() { verify.NotNil(tc.p, "p") }))
			require.Equal(t, !tc.notNilOK, notPanics(func() { verify.Nil(tc.p, "p") }))
		})
	}
}

// TestErrorMessageIsRenderedOnce checks that format strings embedded in
// arguments are never interpreted twice (injection safety of the message).
func TestErrorMessageIsRenderedOnce(t *testing.T) {
	rec := catchPanic(func() { verify.True(false, "bad input %q", "%s %d") })
	err, ok := rec.(error)
	require.Truef(t, ok, "panic value %v has type %T, want error", rec, rec)
	require.Equal(t, `invalid argument: bad input "%s %d"`, err.Error(),
		"the argument's verbs must be escaped by %q, never rendered")
}

// TestWrappedSentinelSurvivesErrorsIs checks the recover-based boundary
// contract end to end.
func TestWrappedSentinelSurvivesErrorsIs(t *testing.T) {
	defer func() {
		rec := recover()
		err, ok := rec.(error)
		require.Truef(t, ok, "panic value %v has type %T, want error", rec, rec)
		require.True(t, errors.Is(err, verify.ErrInvalidArgument))
		require.Equal(t, "invalid argument: guard failed", err.Error())
	}()
	verify.True(false, "guard failed")
}
