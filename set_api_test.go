package struct233_test

import (
	"fmt"
	"testing"

	struct233 "github.com/neko233-com/struct233-go"
	"github.com/stretchr/testify/require"
)

func ExampleSet() {
	var ids struct233.Set[int] = struct233.NewHashSet(3, 1, 3)
	ids.AddAll(2, 1)
	fmt.Println(ids.Len(), ids.ContainsAll(1, 2, 3))
	ids = struct233.NewLinkedHashSet(3, 1, 2)
	fmt.Println(ids.ToSlice())
	// Output:
	// 3 true
	// [3 1 2]
}

func ExampleNavigableSet() {
	var ids struct233.NavigableSet[int] = struct233.NewSkipListSet(30, 10, 20)
	fmt.Println(ids.Ceiling(15))
	fmt.Println(ids.ToSlice())
	// Output:
	// 20 true
	// [10 20 30]
}

func TestSetFacadeWorksAcrossAllImplementations(t *testing.T) {
	sets := []struct233.Set[int]{
		struct233.NewHashSet(3, 1),
		struct233.NewLinkedHashSetWithCapacity(8, 3, 1),
		struct233.NewSkipListSet(3, 1),
		struct233.NewConcurrentHashSet(3, 1),
	}
	for _, s := range sets {
		s.AddAll(2, 1)
		require.True(t, s.ContainsAll(1, 2, 3))
		require.Equal(t, 3, s.Len())
		require.True(t, struct233.NewHashSet(1, 2, 3).Equal(s))
		require.Equal(t, []int{1, 2, 3}, struct233.NewSkipListSet[int]().Union(s).ToSlice())
		require.True(t, s.Remove(2))
		s.Clear()
		require.True(t, s.IsEmpty())
	}
}
