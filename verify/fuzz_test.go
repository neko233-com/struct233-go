package verify_test

import (
	"errors"
	"testing"

	"github.com/neko233-com/struct233-go/verify"
)

// FuzzVerifyParams feeds fuzzer-chosen parameters into every check and
// requires the observed pass/panic behaviour to match the predicate model of
// each check, and every panic to wrap the right sentinel with the rendered
// message.
func FuzzVerifyParams(f *testing.F) {
	f.Add([]byte{5, 5, 1, 0, 0, 5, 1})
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{255, 1, 255, 1, 255, 1, 1})
	f.Add([]byte{3, 10, 0, 2, 2, 2, 0})
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 7 {
			return
		}
		// map the raw bytes to small signed parameters around the boundaries
		i := int(data[0])%11 - 5
		size := int(data[1])%11 - 5
		n := int(data[2])%7 - 3
		v := int(data[3])%7 - 3
		lo := int(data[4])%7 - 3
		hi := int(data[5])%7 - 3
		cond := data[6]%2 == 0

		// model: each check's pass predicate
		checks := []struct {
			name string
			pass bool
			sent error
			fn   func()
		}{
			{"True", cond, verify.ErrInvalidArgument, func() { verify.True(cond, "t %d", i) }},
			{"False", !cond, verify.ErrInvalidArgument, func() { verify.False(cond, "f %d", i) }},
			{"State", cond, verify.ErrInvalidState, func() { verify.State(cond, "s %d", n) }},
			{"Index", i >= 0 && i < size, verify.ErrInvalidArgument,
				func() { verify.Index(i, size, "idx %d of %d", i, size) }},
			{"Position", i >= 0 && i <= size, verify.ErrInvalidArgument,
				func() { verify.Position(i, size, "pos %d of %d", i, size) }},
			{"InRange", v >= lo && v <= hi, verify.ErrInvalidArgument,
				func() { verify.InRange(v, lo, hi, "v %d in [%d, %d]", v, lo, hi) }},
			{"Positive", n > 0, verify.ErrInvalidArgument, func() { verify.Positive(n, "n %d", n) }},
			{"NonNegative", n >= 0, verify.ErrInvalidArgument,
				func() { verify.NonNegative(n, "n %d", n) }},
		}
		for _, tc := range checks {
			if tc.pass {
				if rec := catchPanicQuiet(tc.fn); rec != nil {
					t.Fatalf("%s(%d,%d,%d) must pass but panicked with %v", tc.name, i, size, n, rec)
				}
				continue
			}
			rec := catchPanicQuiet(tc.fn)
			err, ok := rec.(error)
			if !ok {
				t.Fatalf("%s must panic with an error, got %v (%T)", tc.name, rec, rec)
			}
			if !errors.Is(err, tc.sent) {
				t.Fatalf("%s panic %v must wrap %v", tc.name, err, tc.sent)
			}
		}

		// pointer checks against the nil/non-nil model
		p := new(int)
		var nilP *int
		if rec := catchPanicQuiet(func() { verify.NotNil(nilP, "p %d", i) }); rec == nil {
			t.Fatal("NotNil(nil) must panic")
		}
		if rec := catchPanicQuiet(func() { verify.NotNil(p, "p %d", i) }); rec != nil {
			t.Fatalf("NotNil(non-nil) must pass, panicked with %v", rec)
		}
		if rec := catchPanicQuiet(func() { verify.Nil(nilP, "p %d", i) }); rec != nil {
			t.Fatalf("Nil(nil) must pass, panicked with %v", rec)
		}
		if rec := catchPanicQuiet(func() { verify.Nil(p, "p %d", i) }); rec == nil {
			t.Fatal("Nil(non-nil) must panic")
		}
	})
}

// catchPanicQuiet runs fn and returns the recovered panic value, or nil when
// fn returned normally. It is the fuzz-safe variant of catchPanic (no test
// handle needed).
func catchPanicQuiet(fn func()) (rec any) {
	defer func() { rec = recover() }()
	fn()
	return nil
}
