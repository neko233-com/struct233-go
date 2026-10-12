package skiplist

import "testing"

func TestClearReleasesPredecessors(t *testing.T) {
	s := New[int, *int]()
	for i := range 10 {
		s.Put(i, new(int))
	}
	s.Clear()
	for _, n := range s.update {
		if n != nil && n != s.head {
			t.Fatal("Clear retained a predecessor and its linked entries")
		}
	}
	s.Put(1, new(int))
	if s.Len() != 1 {
		t.Fatal("skip list is not reusable after Clear")
	}
}
