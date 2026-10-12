// Package verify_test drives the exported checks of package verify from the
// outside, the way a caller sees them.
package verify_test

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"math"
	"testing"

	"github.com/neko233-com/struct233-go/verify"
	"github.com/stretchr/testify/require"
)

// catchPanic runs fn and returns the recovered panic value, or nil when fn
// returned normally.
func catchPanic(fn func()) (rec any) {
	defer func() { rec = recover() }()
	fn()
	return nil
}

// requirePanicsError asserts fn panics with an error that wraps sentinel and
// whose message contains want. fn is deliberately invoked twice - once for
// require.Panics and once for the recover-based capture - which is safe because
// the checks under test are pure.
func requirePanicsError(t *testing.T, sentinel error, want string, fn func()) {
	t.Helper()
	require.Panics(t, fn)
	rec := catchPanic(fn)
	err, ok := rec.(error)
	require.Truef(t, ok, "panic value %v has type %T, want error", rec, rec)
	require.ErrorIs(t, err, sentinel)
	require.Contains(t, err.Error(), want)
}

// boolCase is one row of the True/False/State tables. An empty want leaves the
// check on its passing path.
type boolCase struct {
	name    string
	cond    bool
	format  string
	args    []any
	wantErr error // sentinel expected when the check panics, nil to pass
	want    string
}

// run applies the case against check, which is one of True/False/State.
func (tc boolCase) run(t *testing.T, check func(bool, string, ...any)) {
	t.Run(tc.name, func(t *testing.T) {
		call := func() { check(tc.cond, tc.format, tc.args...) }
		if tc.wantErr == nil {
			require.NotPanics(t, call)
			return
		}
		requirePanicsError(t, tc.wantErr, tc.want, call)
	})
}

// TestTrue covers the passing path plus failing checks with 0, 1 and several
// formatting arguments.
func TestTrue(t *testing.T) {
	cases := []boolCase{
		{name: "cond true without args", cond: true, format: "unreachable"},
		{name: "cond true with args", cond: true, format: "unreachable %d %q %v", args: []any{1, "x", struct{ A int }{2}}},
		{
			name: "cond false without args", cond: false, format: "plain failure",
			wantErr: verify.ErrInvalidArgument, want: "invalid argument: plain failure",
		},
		{
			name: "cond false with one int arg", cond: false, format: "count %d must be positive", args: []any{0},
			wantErr: verify.ErrInvalidArgument, want: "invalid argument: count 0 must be positive",
		},
		{
			name: "cond false with several args", cond: false, format: "range %d not in [%d, %d]", args: []any{9, 0, 5},
			wantErr: verify.ErrInvalidArgument, want: "invalid argument: range 9 not in [0, 5]",
		},
		{
			name: "cond false with %q and %v", cond: false, format: "key %q value %v", args: []any{"k", []int{1, 2}},
			wantErr: verify.ErrInvalidArgument, want: `invalid argument: key "k" value [1 2]`,
		},
		{
			name: "cond false with a percent literal", cond: false, format: "100%% of %d checks failed", args: []any{3},
			wantErr: verify.ErrInvalidArgument, want: "invalid argument: 100% of 3 checks failed",
		},
		{
			name: "cond false with unused args", cond: false, format: "boom", args: []any{1},
			wantErr: verify.ErrInvalidArgument, want: "invalid argument: boom%!(EXTRA int=1)",
		},
	}
	for _, tc := range cases {
		tc.run(t, verify.True)
	}
}

// TestFalse covers the passing path plus failing checks with 0, 1 and several
// formatting arguments.
func TestFalse(t *testing.T) {
	cases := []boolCase{
		{name: "cond false without args", cond: false, format: "unreachable"},
		{name: "cond false with args", cond: false, format: "unreachable %d", args: []any{1}},
		{
			name: "cond true without args", cond: true, format: "flag must not be set",
			wantErr: verify.ErrInvalidArgument, want: "invalid argument: flag must not be set",
		},
		{
			name: "cond true with one args", cond: true, format: "flag %q must not be set", args: []any{"debug"},
			wantErr: verify.ErrInvalidArgument, want: `invalid argument: flag "debug" must not be set`,
		},
		{
			name: "cond true with several args", cond: true, format: "%v and %d are set", args: []any{"a", 2},
			wantErr: verify.ErrInvalidArgument, want: "invalid argument: a and 2 are set",
		},
	}
	for _, tc := range cases {
		tc.run(t, verify.False)
	}
}

// TestState covers the passing path and the failing path, which must wrap
// ErrInvalidState rather than ErrInvalidArgument.
func TestState(t *testing.T) {
	cases := []boolCase{
		{name: "cond true without args", cond: true, format: "unreachable"},
		{name: "cond true with args", cond: true, format: "unreachable %d", args: []any{1}},
		{
			name: "cond false without args", cond: false, format: "broker already closed",
			wantErr: verify.ErrInvalidState, want: "invalid state: broker already closed",
		},
		{
			name: "cond false with one args", cond: false, format: "closed after %d retries", args: []any{2},
			wantErr: verify.ErrInvalidState, want: "invalid state: closed after 2 retries",
		},
		{
			name: "cond false with several args", cond: false, format: "state %q cannot handle %v", args: []any{"closed", []int{1}},
			wantErr: verify.ErrInvalidState, want: `invalid state: state "closed" cannot handle [1]`,
		},
	}
	for _, tc := range cases {
		tc.run(t, verify.State)
	}

	t.Run("failing state wraps only ErrInvalidState", func(t *testing.T) {
		rec := catchPanic(func() { verify.State(false, "closed after %d retries", 2) })
		err, ok := rec.(error)
		require.Truef(t, ok, "panic value %v has type %T, want error", rec, rec)
		require.ErrorIs(t, err, verify.ErrInvalidState)
		require.NotErrorIs(t, err, verify.ErrInvalidArgument)
		require.Equal(t, verify.ErrInvalidState, errors.Unwrap(err))
	})
}

// TestNotNil covers the passing path with a non-nil pointer, the failing path
// with a nil untyped and typed pointer, and pointers to interface values.
func TestNotNil(t *testing.T) {
	x := 42

	t.Run("non nil pointer is returned unchanged", func(t *testing.T) {
		require.Same(t, &x, verify.NotNil(&x, "unreachable"))
	})
	t.Run("pointed to value is preserved", func(t *testing.T) {
		require.Equal(t, 42, *verify.NotNil(&x, "unreachable"))
	})
	t.Run("nil pointer panics", func(t *testing.T) {
		var p *int
		requirePanicsError(t, verify.ErrInvalidArgument, "invalid argument: pointer must not be nil", func() {
			verify.NotNil(p, "pointer must not be nil")
		})
	})
	t.Run("nil typed pointer panics with formatted args", func(t *testing.T) {
		var s *string
		requirePanicsError(t, verify.ErrInvalidArgument, `invalid argument: field "name" must not be nil`, func() {
			verify.NotNil(s, "field %q must not be nil", "name")
		})
	})
	t.Run("pointer to nil interface is not nil", func(t *testing.T) {
		var r io.Reader // nil interface, but &r itself is a live pointer
		require.Same(t, &r, verify.NotNil(&r, "unreachable"))
	})
	t.Run("interface holding a typed nil is not nil", func(t *testing.T) {
		var v any = (*int)(nil)
		got := verify.NotNil(&v, "unreachable")
		require.Nil(t, *got)
	})
}

// TestNil covers the passing path with nil pointers and the failing path with
// a live pointer.
func TestNil(t *testing.T) {
	t.Run("nil pointer passes", func(t *testing.T) {
		var p *int
		require.NotPanics(t, func() { verify.Nil(p, "unreachable") })
	})
	t.Run("nil typed pointer passes", func(t *testing.T) {
		var s *string
		require.NotPanics(t, func() { verify.Nil(s, "unreachable") })
	})
	t.Run("pointer to a nil interface panics", func(t *testing.T) {
		var r io.Reader
		// Nil only inspects the pointer it is given, so a pointer to a nil
		// interface is a non-nil pointer and must be rejected.
		requirePanicsError(t, verify.ErrInvalidArgument, "invalid argument: unreachable", func() {
			verify.Nil(&r, "unreachable")
		})
	})
	t.Run("non nil pointer panics", func(t *testing.T) {
		x := 1
		requirePanicsError(t, verify.ErrInvalidArgument, "invalid argument: buffer already set", func() {
			verify.Nil(&x, "buffer already set")
		})
	})
	t.Run("non nil pointer panics with one arg", func(t *testing.T) {
		x := 1
		requirePanicsError(t, verify.ErrInvalidArgument, "invalid argument: expected nil, got 1", func() {
			verify.Nil(&x, "expected nil, got %d", x)
		})
	})
}

// TestIndex covers the passing indices plus every bound failure: i == size,
// size == 0, a negative index and a negative size.
func TestIndex(t *testing.T) {
	cases := []struct {
		name    string
		i, size int
		want    string
	}{
		{name: "first element i==0", i: 0, size: 1},
		{name: "i==0 of a large size", i: 0, size: math.MaxInt},
		{name: "middle element", i: 2, size: 5},
		{name: "last element i==size-1", i: 4, size: 5},
		{
			name: "i==size panics", i: 5, size: 5,
			want: "invalid argument: index 5 out of range [0, 5)",
		},
		{
			name: "size==0 panics", i: 0, size: 0,
			want: "invalid argument: index 0 out of range [0, 0)",
		},
		{
			name: "negative i panics", i: -1, size: 5,
			want: "invalid argument: index -1 out of range [0, 5)",
		},
		{
			name: "negative size panics", i: 0, size: -1,
			want: "invalid argument: index 0 out of range [0, -1)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			call := func() { verify.Index(tc.i, tc.size, "index %d out of range [0, %d)", tc.i, tc.size) }
			if tc.want == "" {
				require.NotPanics(t, call)
				return
			}
			requirePanicsError(t, verify.ErrInvalidArgument, tc.want, call)
		})
	}
}

// TestPosition covers the passing insertion points plus every bound failure:
// i == size is legal here, i == size+1 is not.
func TestPosition(t *testing.T) {
	cases := []struct {
		name    string
		i, size int
		want    string
	}{
		{name: "empty i==0 size==0", i: 0, size: 0},
		{name: "front i==0", i: 0, size: 5},
		{name: "middle", i: 2, size: 5},
		{name: "append i==size", i: 5, size: 5},
		{name: "append at a large size", i: math.MaxInt, size: math.MaxInt},
		{
			name: "i==size+1 panics", i: 6, size: 5,
			want: "invalid argument: position 6 out of range [0, 5]",
		},
		{
			name: "negative i panics", i: -1, size: 5,
			want: "invalid argument: position -1 out of range [0, 5]",
		},
		{
			name: "size==0 with i==1 panics", i: 1, size: 0,
			want: "invalid argument: position 1 out of range [0, 0]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			call := func() { verify.Position(tc.i, tc.size, "position %d out of range [0, %d]", tc.i, tc.size) }
			if tc.want == "" {
				require.NotPanics(t, call)
				return
			}
			requirePanicsError(t, verify.ErrInvalidArgument, tc.want, call)
		})
	}
}

// checkRangeBounds pins the edges of InRange for one ordered type: v == lo and
// v == hi pass, one step outside either end panics.
func checkRangeBounds[T cmp.Ordered](t *testing.T, below, lo, hi, above T) {
	t.Helper()
	const format = "value %v not in [%v, %v]"

	t.Run("v==lo passes", func(t *testing.T) {
		require.NotPanics(t, func() { verify.InRange(lo, lo, hi, format, lo, lo, hi) })
	})
	t.Run("v==hi passes", func(t *testing.T) {
		require.NotPanics(t, func() { verify.InRange(hi, lo, hi, format, hi, lo, hi) })
	})
	t.Run("below lo panics", func(t *testing.T) {
		requirePanicsError(t, verify.ErrInvalidArgument,
			"invalid argument: value "+fmt.Sprint(below)+" not in [", func() {
				verify.InRange(below, lo, hi, format, below, lo, hi)
			})
	})
	t.Run("above hi panics", func(t *testing.T) {
		requirePanicsError(t, verify.ErrInvalidArgument,
			"invalid argument: value "+fmt.Sprint(above)+" not in [", func() {
				verify.InRange(above, lo, hi, format, above, lo, hi)
			})
	})
}

// TestInRange covers every signed and unsigned int width, the float widths, the
// byte/rune aliases and string bounds.
func TestInRange(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"int", func(t *testing.T) { checkRangeBounds(t, -3, -2, 3, 4) }},
		{"int8", func(t *testing.T) { checkRangeBounds[int8](t, -3, -2, 3, 4) }},
		{"int16", func(t *testing.T) { checkRangeBounds[int16](t, -3, -2, 3, 4) }},
		{"int32", func(t *testing.T) { checkRangeBounds[int32](t, -3, -2, 3, 4) }},
		{"int64", func(t *testing.T) { checkRangeBounds[int64](t, -3, -2, 3, 4) }},
		{"rune alias of int32", func(t *testing.T) { checkRangeBounds[rune](t, 'a', 'b', 'y', 'z') }},
		{"uint", func(t *testing.T) { checkRangeBounds[uint](t, 0, 1, 3, 4) }},
		{"uint8", func(t *testing.T) { checkRangeBounds[uint8](t, 0, 1, 3, 4) }},
		{"byte alias of uint8", func(t *testing.T) { checkRangeBounds[byte](t, 0, 1, 3, 4) }},
		{"uint16", func(t *testing.T) { checkRangeBounds[uint16](t, 0, 1, 3, 4) }},
		{"uint32", func(t *testing.T) { checkRangeBounds[uint32](t, 0, 1, 3, 4) }},
		{"uint64", func(t *testing.T) { checkRangeBounds[uint64](t, 0, 1, 3, 4) }},
		{"uintptr", func(t *testing.T) { checkRangeBounds[uintptr](t, 0, 1, 3, 4) }},
		{"float32", func(t *testing.T) { checkRangeBounds[float32](t, -3.5, -2.5, 3.5, 4.5) }},
		{"float64", func(t *testing.T) { checkRangeBounds(t, -3.5, -2.5, 3.5, 4.5) }},
		{"string", func(t *testing.T) { checkRangeBounds(t, "a", "b", "y", "z") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t) })
	}

	t.Run("inverted bounds always panic", func(t *testing.T) {
		requirePanicsError(t, verify.ErrInvalidArgument, "invalid argument: value 3 not in [5, 1]", func() {
			verify.InRange(3, 5, 1, "value %d not in [%d, %d]", 3, 5, 1)
		})
	})
	t.Run("nan passes both comparisons", func(t *testing.T) {
		require.NotPanics(t, func() { verify.InRange(math.NaN(), 0.0, 1.0, "unreachable") })
	})
}

// TestPositive covers the passing path at 1, 2 and math.MaxInt plus 0 and the
// negative boundary.
func TestPositive(t *testing.T) {
	cases := []struct {
		name string
		n    int
		want string
	}{
		{name: "one", n: 1},
		{name: "two", n: 2},
		{name: "max int", n: math.MaxInt},
		{name: "zero panics", n: 0, want: "invalid argument: count 0 must be > 0"},
		{name: "minus one panics", n: -1, want: "invalid argument: count -1 must be > 0"},
		{name: "min int panics", n: math.MinInt, want: fmt.Sprintf("invalid argument: count %d must be > 0", math.MinInt)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			call := func() { verify.Positive(tc.n, "count %d must be > 0", tc.n) }
			if tc.want == "" {
				require.NotPanics(t, call)
				return
			}
			requirePanicsError(t, verify.ErrInvalidArgument, tc.want, call)
		})
	}
}

// TestNonNegative covers the passing path at 0, 1 and math.MaxInt plus the
// negative boundary.
func TestNonNegative(t *testing.T) {
	cases := []struct {
		name string
		n    int
		want string
	}{
		{name: "zero", n: 0},
		{name: "one", n: 1},
		{name: "max int", n: math.MaxInt},
		{name: "minus one panics", n: -1, want: "invalid argument: length -1 must be >= 0"},
		{name: "min int panics", n: math.MinInt, want: fmt.Sprintf("invalid argument: length %d must be >= 0", math.MinInt)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			call := func() { verify.NonNegative(tc.n, "length %d must be >= 0", tc.n) }
			if tc.want == "" {
				require.NotPanics(t, call)
				return
			}
			requirePanicsError(t, verify.ErrInvalidArgument, tc.want, call)
		})
	}
}

// TestPanicMessageRendering pins the exact panic text for 0, 1 and several
// arguments and for the %d, %q and %v verbs.
func TestPanicMessageRendering(t *testing.T) {
	cases := []struct {
		name string
		fn   func()
		want string
	}{
		{"no args", func() { verify.True(false, "plain") }, "invalid argument: plain"},
		{"one %d", func() { verify.True(false, "got %d", 7) }, "invalid argument: got 7"},
		{"one %q", func() { verify.True(false, "key %q", "k") }, `invalid argument: key "k"`},
		{"several %d %q %v", func() { verify.True(false, "%d %q %v", 1, "s", []int{2, 3}) }, `invalid argument: 1 "s" [2 3]`},
		{"percent literal with args", func() { verify.True(false, "%d%% %v", 50, true) }, "invalid argument: 50% true"},
		{"verb inside an argument is rendered once", func() { verify.True(false, "bad %q", "%d") }, `invalid argument: bad "%d"`},
		{"format with no verb keeps extra args visible", func() { format := "boom"; verify.True(false, format, 1, 2) }, "invalid argument: boom%!(EXTRA int=1, int=2)"},
		{"Index message", func() { verify.Index(3, 2, "index %d of %d", 3, 2) }, "invalid argument: index 3 of 2"},
		{"Position message", func() { verify.Position(3, 2, "position %d of %d", 3, 2) }, "invalid argument: position 3 of 2"},
		{"Positive message", func() { verify.Positive(0, "size %d", 0) }, "invalid argument: size 0"},
		{"NonNegative message", func() { verify.NonNegative(-2, "offset %d for %q", -2, "body") }, `invalid argument: offset -2 for "body"`},
		{"InRange message", func() { verify.InRange(9, 0, 5, "range %d not in [%d, %d]", 9, 0, 5) }, "invalid argument: range 9 not in [0, 5]"},
		{"State message", func() { verify.State(false, "closed after %d retries", 2) }, "invalid state: closed after 2 retries"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := catchPanic(tc.fn)
			err, ok := rec.(error)
			require.Truef(t, ok, "panic value %v has type %T, want error", rec, rec)
			require.Equal(t, tc.want, err.Error())
		})
	}
}

// TestPanicValueClassification covers the error identity contract that callers
// rely on after recover.
func TestPanicValueClassification(t *testing.T) {
	t.Run("argument panic is an error wrapping ErrInvalidArgument", func(t *testing.T) {
		rec := catchPanic(func() { verify.Index(-1, 1, "i %d", -1) })
		err, ok := rec.(error)
		require.Truef(t, ok, "panic value %v has type %T, want error", rec, rec)
		require.ErrorIs(t, err, verify.ErrInvalidArgument)
		require.NotErrorIs(t, err, verify.ErrInvalidState)
		require.Equal(t, verify.ErrInvalidArgument, errors.Unwrap(err))
	})
	t.Run("panic value is not a plain string", func(t *testing.T) {
		rec := catchPanic(func() { verify.True(false, "nope") })
		_, isString := rec.(string)
		require.False(t, isString, "panic value must be an error, got %T", rec)
	})
	t.Run("sentinels are distinct and self describing", func(t *testing.T) {
		require.NotEqual(t, verify.ErrInvalidArgument, verify.ErrInvalidState)
		require.Equal(t, "invalid argument", verify.ErrInvalidArgument.Error())
		require.Equal(t, "invalid state", verify.ErrInvalidState.Error())
	})
	t.Run("sentinels survive errors.Is through fmt.Errorf", func(t *testing.T) {
		wrapped := fmt.Errorf("outer: %w", verify.ErrInvalidArgument)
		require.ErrorIs(t, wrapped, verify.ErrInvalidArgument)
		require.False(t, errors.Is(wrapped, verify.ErrInvalidState))
	})
}

// TestSuccessDoesNotAllocate documents the cost of a passing check: with no
// formatting arguments the varargs slice stays nil and nothing escapes.
func TestSuccessDoesNotAllocate(t *testing.T) {
	p := new(int)
	var np *int

	cases := []struct {
		name string
		fn   func()
	}{
		{"True", func() { verify.True(true, "x") }},
		{"True with a longer format", func() { verify.True(true, "condition must hold for every caller") }},
		{"False", func() { verify.False(false, "x") }},
		{"State", func() { verify.State(true, "x") }},
		{"NotNil", func() { verify.NotNil(p, "x") }},
		{"Nil", func() { verify.Nil(np, "x") }},
		{"Index", func() { verify.Index(0, 1, "x") }},
		{"Position", func() { verify.Position(0, 0, "x") }},
		{"InRange", func() { verify.InRange(1, 0, 2, "x") }},
		{"Positive", func() { verify.Positive(1, "x") }},
		{"NonNegative", func() { verify.NonNegative(0, "x") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Zero(t, testing.AllocsPerRun(1000, tc.fn), "passing check must not allocate")
		})
	}

	t.Run("True with no args is the documented zero allocation shape", func(t *testing.T) {
		require.Zero(t, testing.AllocsPerRun(1000, func() { verify.True(true, "x") }))
	})
}

// TestAllocsWithArguments records, without asserting, what a passing check with
// formatting arguments costs: the arguments are boxed into a slice, so the
// compiler may have to allocate. The measurement is logged for the record.
func TestAllocsWithArguments(t *testing.T) {
	withArgs := testing.AllocsPerRun(1000, func() { verify.True(true, "value %d and %q", 1, "two") })
	noArgs := testing.AllocsPerRun(1000, func() { verify.True(true, "value") })
	t.Logf("allocs per passing check: no args=%v, three varargs=%v", noArgs, withArgs)
	require.Zero(t, noArgs)
}
