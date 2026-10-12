package unionfind_test

import (
	"fmt"
	"sort"

	"github.com/neko233-com/struct233-go/unionfind"
)

func ExampleSet() {
	s := unionfind.New(5)
	s.Union(0, 1)
	s.Union(1, 2)
	fmt.Println(s.Count(), s.Connected(0, 2), s.Connected(0, 3))
	fmt.Println(s.Size(2), s.Find(2) == s.Find(0))
	// Output:
	// 3 true false
	// 3 true
}

func ExampleSet_Components() {
	s := unionfind.New(6)
	s.Union(0, 1)
	s.Union(3, 4)
	comps := s.Components()
	for _, c := range comps {
		sort.Ints(c) // members are already ascending; sort makes output order stable
	}
	sort.Slice(comps, func(i, j int) bool { return comps[i][0] < comps[j][0] })
	fmt.Println(comps)
	// Output:
	// [[0 1] [2] [3 4] [5]]
}
