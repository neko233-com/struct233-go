package ringbuffer_test

import (
	"slices"
	"testing"

	"github.com/neko233-com/struct233-go/ringbuffer"
)

// FuzzRingBufferAgainstSlice drives Push/Pop from arbitrary bytes and compares
// the buffer against a plain slice with the same capacity and overwrite-oldest
// rule.
func FuzzRingBufferAgainstSlice(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 2, 4, 6, 8, 10})
	f.Add([]byte{1, 3, 5, 7, 9})

	f.Fuzz(func(t *testing.T, ops []byte) {
		const capacity = 4
		r := ringbuffer.New[int](capacity)
		ref := make([]int, 0, capacity)

		for _, b := range ops {
			if b&1 == 0 { // push
				v := int(b)
				evicted, ok := r.Push(v)
				if len(ref) == capacity {
					if !ok {
						t.Fatalf("Push(%d) into a full buffer must report an eviction", v)
					}
					if evicted != ref[0] {
						t.Fatalf("Push(%d) evicted %d; want %d", v, evicted, ref[0])
					}
					ref = append(ref[1:], v)
				} else {
					if ok {
						t.Fatalf("Push(%d) into a non-full buffer evicted %d", v, evicted)
					}
					ref = append(ref, v)
				}
			} else { // pop
				v, ok := r.Pop()
				if len(ref) == 0 {
					if ok {
						t.Fatalf("Pop from an empty buffer returned %d", v)
					}
				} else {
					if !ok || v != ref[0] {
						t.Fatalf("Pop() = %d, %v; want %d, true", v, ok, ref[0])
					}
					ref = ref[1:]
				}
			}

			if r.Len() != len(ref) {
				t.Fatalf("Len() = %d; want %d", r.Len(), len(ref))
			}
			if got := r.ToSlice(); !slices.Equal(got, ref) {
				t.Fatalf("ToSlice() = %v; want %v", got, ref)
			}
			for i, want := range ref {
				got, ok := r.At(i)
				if !ok || got != want {
					t.Fatalf("At(%d) = %d, %v; want %d, true", i, got, ok, want)
				}
			}
			peeked, ok := r.Peek()
			if ok != (len(ref) > 0) || (ok && peeked != ref[0]) {
				t.Fatalf("Peek() = %d, %v; want %v again", peeked, ok, ref)
			}
		}
	})
}
