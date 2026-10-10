package lru_test

import (
	"slices"
	"testing"

	"github.com/neko233-com/go-data-struct-lib/lru"
)

func TestEviction(t *testing.T) {
	c := lru.New[string, int](2)
	if ek, ev, evicted := c.Put("a", 1); evicted {
		t.Fatalf("unexpected eviction: %q=%v", ek, ev)
	}
	c.Put("b", 2)
	ek, ev, evicted := c.Put("c", 3)
	if !evicted || ek != "a" || ev != 1 {
		t.Fatalf("Put(c) evicted %q=%v, %v; want a=1, true", ek, ev, evicted)
	}
	if c.Len() != 2 {
		t.Fatalf("Len = %d; want 2", c.Len())
	}
	if c.Contains("a") {
		t.Fatal("a should have been evicted")
	}
}

func TestGetPromotes(t *testing.T) {
	c := lru.New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)

	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) = %d, %v", v, ok)
	}
	// a was promoted; b is now the least recently used
	_, _, evicted := c.Put("c", 3)
	if !evicted {
		t.Fatal("expected an eviction")
	}
	if c.Contains("b") {
		t.Fatal("b should have been evicted (a was promoted by Get)")
	}
	if !c.Contains("a") || !c.Contains("c") {
		t.Fatal("a and c should remain")
	}
}

func TestPeekDoesNotPromote(t *testing.T) {
	c := lru.New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)

	if v, ok := c.Peek("a"); !ok || v != 1 {
		t.Fatalf("Peek(a) = %d, %v", v, ok)
	}
	// a was NOT promoted; it is still the least recently used
	ek, _, evicted := c.Put("c", 3)
	if !evicted || ek != "a" {
		t.Fatalf("Put(c) evicted %q, %v; want a", ek, evicted)
	}
}

func TestPutUpdateNoEvict(t *testing.T) {
	c := lru.New[string, int](1)
	if _, _, evicted := c.Put("a", 1); evicted {
		t.Fatal("first Put should not evict")
	}
	if _, _, evicted := c.Put("a", 10); evicted {
		t.Fatal("updating an existing key should never evict")
	}
	if v, ok := c.Get("a"); !ok || v != 10 {
		t.Fatalf("Get(a) = %d, %v; want 10", v, ok)
	}
	if c.Len() != 1 {
		t.Fatalf("Len = %d; want 1", c.Len())
	}
	// updating also promotes the key, without evicting anything
	c2 := lru.New[string, int](2)
	c2.Put("a", 1)
	c2.Put("b", 2)
	if _, _, evicted := c2.Put("a", 11); evicted {
		t.Fatal("update should not evict")
	}
	if !c2.Contains("b") {
		t.Fatal("b should survive when a is merely updated")
	}
	// a was promoted, so b is the eviction candidate now
	ek, _, evicted := c2.Put("c", 3)
	if !evicted || ek != "b" {
		t.Fatalf("Put(c) evicted %q, %v; want b", ek, evicted)
	}
}

func TestKeysOrder(t *testing.T) {
	c := lru.New[int, int](4)
	for i := range 4 {
		c.Put(i, i)
	}
	// order: 0,1,2,3 (least → most recent)
	c.Get(1) // promotes 1: 0,2,3,1
	c.Get(0) // promotes 0: 2,3,1,0
	if got := c.Keys(); !slices.Equal(got, []int{2, 3, 1, 0}) {
		t.Fatalf("Keys = %v; want [2 3 1 0]", got)
	}
	var iterKeys []int
	for k := range c.All() {
		iterKeys = append(iterKeys, k)
	}
	if !slices.Equal(iterKeys, []int{2, 3, 1, 0}) {
		t.Fatalf("All() = %v", iterKeys)
	}
}

func TestRemoveAndClear(t *testing.T) {
	c := lru.New[int, int](3)
	c.Put(1, 1)
	c.Put(2, 2)
	if !c.Remove(1) || c.Remove(1) {
		t.Fatal("Remove should report true exactly once")
	}
	c.Put(3, 3)
	if got := c.Keys(); !slices.Equal(got, []int{2, 3}) {
		t.Fatalf("Keys after remove = %v", got)
	}
	c.Clear()
	if c.Len() != 0 || c.Contains(2) || len(c.Keys()) != 0 {
		t.Fatal("cache should be empty after Clear")
	}
	// reusable after Clear
	c.Put(9, 9)
	if v, ok := c.Get(9); !ok || v != 9 {
		t.Fatal("cache unusable after Clear")
	}
}

func TestNewPanicsOnBadCapacity(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("New(%d) should panic", capacity)
				}
			}()
			lru.New[string, int](capacity)
		}()
	}
}
