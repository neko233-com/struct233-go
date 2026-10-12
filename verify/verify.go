// Package verify provides panic-based guard checks for arguments and state, in
// the spirit of com.google.common.base.Preconditions.
//
// Every failing check panics with an error wrapping ErrInvalidArgument or
// ErrInvalidState, so a recover-based boundary can classify the failure with
// errors.Is; the message is the caller's format/args text rendered once and
// prefixed with the sentinel's text.
//
// Usage:
//
//	func area(w, h int) int {
//		verify.Positive(w, "width %d must be positive", w)
//		verify.Positive(h, "height %d must be positive", h)
//		return w * h
//	}
//
// area(-1, 2) panics with an error whose text is
// "invalid argument: width -1 must be positive" and for which
// errors.Is(err, ErrInvalidArgument) is true.
//
// A passing check costs nothing: with no formatting arguments the variadic
// slice is nil and the call does not allocate, which is asserted by
// TestSuccessDoesNotAllocate. Passing arguments boxes them, so those calls may
// allocate on the heap.
package verify

import (
	"cmp"
	"errors"
	"fmt"
)

// ErrInvalidArgument is wrapped by the error every argument check panics with,
// so callers can recognise it with errors.Is after recovering.
var ErrInvalidArgument = errors.New("invalid argument")

// ErrInvalidState is wrapped by the error State panics with.
var ErrInvalidState = errors.New("invalid state")

// fail builds the panic payload of a failed check. format is applied to args
// exactly once and only for the %s operand, so text produced by the caller's
// verbs can never be interpreted a second time as a format string.
func fail(sentinel error, format string, args ...any) error {
	return fmt.Errorf("%w: %s", sentinel, sprintf(format, args...))
}

// sprintf renders a caller format string with its arguments. It is a plain
// wrapper so the render happens in exactly one place; callers reach it only
// through the checks above.
func sprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

// True panics when cond is false (com.google.common.base.Preconditions.checkArgument).
// The panic value is an error wrapping ErrInvalidArgument whose message is the
// formatted format/args text.
func True(cond bool, format string, args ...any) {
	if !cond {
		panic(fail(ErrInvalidArgument, format, args...))
	}
}

// False panics when cond is true.
func False(cond bool, format string, args ...any) {
	if cond {
		panic(fail(ErrInvalidArgument, format, args...))
	}
}

// State panics when cond is false, wrapping ErrInvalidState
// (com.google.common.base.Preconditions.checkState).
func State(cond bool, format string, args ...any) {
	if !cond {
		panic(fail(ErrInvalidState, format, args...))
	}
}

// NotNil returns v, or panics when v is nil, wrapping ErrInvalidArgument
// (com.google.common.base.Preconditions.checkNotNull). It only sees the
// pointer itself: an interface holding a typed nil is a non-nil pointer.
func NotNil[T any](v *T, format string, args ...any) *T {
	if v == nil {
		panic(fail(ErrInvalidArgument, format, args...))
	}
	return v
}

// Nil panics when v is not nil, wrapping ErrInvalidArgument.
func Nil[T any](v *T, format string, args ...any) {
	if v != nil {
		panic(fail(ErrInvalidArgument, format, args...))
	}
}

// Index panics unless 0 <= i < size (an array index bound check).
func Index(i, size int, format string, args ...any) {
	if i < 0 || i >= size {
		panic(fail(ErrInvalidArgument, format, args...))
	}
}

// Position panics unless 0 <= i <= size (an insertion point bound check).
func Position(i, size int, format string, args ...any) {
	if i < 0 || i > size {
		panic(fail(ErrInvalidArgument, format, args...))
	}
}

// InRange panics unless lo <= v <= hi. A NaN v passes for float types because
// every comparison with NaN is false, matching checkArgument(lo <= v && v <= hi).
func InRange[T cmp.Ordered](v, lo, hi T, format string, args ...any) {
	if v < lo || v > hi {
		panic(fail(ErrInvalidArgument, format, args...))
	}
}

// Positive panics unless n > 0.
func Positive(n int, format string, args ...any) {
	if n <= 0 {
		panic(fail(ErrInvalidArgument, format, args...))
	}
}

// NonNegative panics unless n >= 0.
func NonNegative(n int, format string, args ...any) {
	if n < 0 {
		panic(fail(ErrInvalidArgument, format, args...))
	}
}
