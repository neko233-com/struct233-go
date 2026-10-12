package rangeset

import "testing"

func TestCoalescingAndClearReleaseBounds(t *testing.T) {
	s := NewSet[string]()
	s.Add(Closed("a", "b"))
	s.Add(Closed("d", "e"))
	s.Add(Closed("g", "h"))
	s.Add(All[string]())
	for _, r := range s.spans[len(s.spans):cap(s.spans)] {
		if r.lo.val != "" || r.hi.val != "" {
			t.Fatal("coalescing retained obsolete string bounds")
		}
	}
	s = NewSet[string]()
	s.Add(Closed("a", "z"))
	s.Clear()
	for _, r := range s.spans[:cap(s.spans)] {
		if r.lo.val != "" || r.hi.val != "" {
			t.Fatal("Clear retained string bounds")
		}
	}
}
