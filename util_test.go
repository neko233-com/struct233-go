package struct233_test

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	struct233 "github.com/neko233-com/struct233-go"
)

func TestNewMultiset(t *testing.T) {
	m := struct233.NewMultiset("a", "b", "a")
	require.Equal(t, 3, m.TotalCount())
	require.Equal(t, 2, m.Distinct())
	require.Equal(t, 2, m.Count("a"))
	require.Equal(t, []string{"a", "b"}, m.DistinctElements(), "first insertion order")
	require.Equal(t, []string{"a", "a", "b"}, m.Elements())

	m.Add("a")
	m.AddN("c", 3)
	require.Equal(t, 7, m.TotalCount())
	require.Equal(t, 3, m.Count("a"))

	elem, count, ok := m.MostCommon()
	require.True(t, ok)
	require.Equal(t, "a", elem)
	require.Equal(t, 3, count)

	require.Equal(t, 3, m.RemoveAll("a"), "RemoveAll returns how many occurrences it dropped")
	require.Equal(t, []string{"b", "c"}, m.DistinctElements())

	capped := struct233.NewMultisetWithCapacity[string](16, "x", "x")
	require.Equal(t, 2, capped.TotalCount())
}

func TestFacadeTextHelpers(t *testing.T) {
	require.Equal(t, "a, b", struct233.JoinStrings(", ", "a", "b"))
	require.Equal(t, "1 true", struct233.JoinStringsAny(" ", 1, true))
	require.Equal(t, "a=1&b=2", struct233.JoinStringMap(map[string]string{"b": "2", "a": "1"}, "&", "="))
	require.Equal(t, []string{"a", "b"}, struct233.SplitString("a,b", ","))
	require.Equal(t, "abab", struct233.RepeatString("ab", 2))
	require.Equal(t, "0042", struct233.PadStart("42", 4, '0'))
	require.Equal(t, "42..", struct233.PadEnd("42", 4, '.'))
	require.Equal(t, "hello…", struct233.Truncate("hello world", 8, "…"))
	require.True(t, struct233.IsNullOrEmpty(""))
	require.True(t, struct233.IsNullOrBlank("  "))
	require.False(t, struct233.IsBlank(" x "))
	require.Equal(t, "user_id", struct233.ToSnakeCase("userID"))
	require.Equal(t, "user-id", struct233.ToKebabCase("userID"))
	require.Equal(t, "userId", struct233.ToLowerCamel("user_id"))
	require.Equal(t, "UserId", struct233.ToUpperCamel("user_id"))
}

func TestFacadeFluentTextTypes(t *testing.T) {
	j := struct233.NewJoiner(", ").SkipEmpty()
	require.Equal(t, "ada, grace", j.Join("ada", "", "grace"))
	require.Equal(t, "ada - grace", struct233.NewJoiner(" ").UseForEmpty("-").JoinSlice([]string{"ada", "", "grace"}))

	s := struct233.NewSplitter(",").Trim().OmitEmpty()
	require.Equal(t, []string{"ada", "grace"}, s.Split(" ada , , grace "))
	require.Equal(t, []string{"a", "b,c"}, struct233.NewSplitter(",").Limit(2).Split("a,b,c"))
}

func TestFacadeHashFunctions(t *testing.T) {
	require.Equal(t, uint64(0xcbf29ce484222325), struct233.FNV1a64(nil), "the FNV-1a offset basis")
	require.Equal(t, uint32(0x811c9dc5), struct233.FNV1a32(nil), "the FNV-1a 32 offset basis")
	require.Equal(t, uint32(0xcbf43926), struct233.CRC32([]byte("123456789")))
	require.Equal(t, uint32(0xe3069283), struct233.CRC32C([]byte("123456789")))
	require.Equal(t, uint32(0x11e60398), struct233.Adler32([]byte("Wikipedia")))
	require.Zero(t, struct233.Murmur3_32(nil, 0))

	h1, _ := struct233.Murmur3_128([]byte("key"), 0)
	require.Equal(t, h1, struct233.Sum64([]byte("key")))
	require.Equal(t, struct233.Murmur3_32([]byte("key"), 0), struct233.Sum32([]byte("key")))

	var hasher struct233.Hasher = struct233.Sum64
	require.Equal(t, struct233.Sum64([]byte("x")), hasher([]byte("x")))
}

func TestFacadePreconditions(t *testing.T) {
	require.NotPanics(t, func() { struct233.RequireTrue(true, "never") })
	require.NotPanics(t, func() { struct233.RequireState(true, "never") })
	require.NotPanics(t, func() { struct233.RequirePositive(1, "never") })
	require.NotPanics(t, func() { struct233.RequireNonNegative(0, "never") })
	require.NotPanics(t, func() { struct233.RequireIndex(0, 1, "never") })
	require.NotPanics(t, func() { struct233.RequirePosition(1, 1, "never") })
	require.NotPanics(t, func() { struct233.RequireInRange(3, 1, 5, "never") })

	v := 7
	require.Equal(t, &v, struct233.RequireNotNil(&v, "never"))
	require.NotPanics(t, func() { struct233.RequireNil[int](nil, "never") })

	// the panic value is an error wrapping the sentinel, so errors.Is works
	func() {
		defer func() {
			r := recover()
			err, ok := r.(error)
			require.True(t, ok, "the panic value must be an error")
			require.ErrorIs(t, err, struct233.ErrInvalidArgument)
			require.True(t, strings.HasPrefix(err.Error(), "invalid argument: "), "got %q", err.Error())
		}()
		struct233.RequireTrue(false, "size %d is negative", -1)
	}()

	func() {
		defer func() {
			err, ok := recover().(error)
			require.True(t, ok)
			require.ErrorIs(t, err, struct233.ErrInvalidState)
			require.True(t, errors.Is(err, struct233.ErrInvalidState))
		}()
		struct233.RequireState(false, "closed")
	}()

	require.Panics(t, func() { struct233.RequirePositive(0, "zero") })
	require.Panics(t, func() { struct233.RequireNonNegative(-1, "negative") })
	require.Panics(t, func() { struct233.RequireIndex(2, 2, "past the end") })
	require.Panics(t, func() { struct233.RequirePosition(2, 1, "past the end") })
	require.Panics(t, func() { struct233.RequireInRange(9, 1, 5, "outside") })
	require.Panics(t, func() { struct233.RequireFalse(true, "true") })
	require.Panics(t, func() { struct233.RequireNotNil[int](nil, "nil") })
	require.Panics(t, func() { struct233.RequireNil(&v, "not nil") })
}

func TestNewListMultimap(t *testing.T) {
	m := struct233.NewListMultimap[string, int]()
	m.Put("a", 1)
	m.Put("a", 2) // a ListMultimap keeps duplicates
	m.PutAll("b", 3, 4)
	m.Put("a", 1)

	require.Equal(t, 5, m.Size(), "every value counts")
	require.Equal(t, 2, m.KeyCount(), "two distinct keys")
	require.Equal(t, []int{1, 2, 1}, m.Get("a"), "value insertion order is kept")
	require.True(t, m.ContainsEntry("a", 1))
	require.False(t, m.ContainsEntry("a", 9))
	require.Equal(t, []string{"a", "b"}, m.Keys(), "key insertion order is kept")
	require.Equal(t, []int{1, 2, 1, 3, 4}, m.Values())

	require.True(t, m.Remove("a", 1), "Remove drops the first match only")
	require.Equal(t, []int{2, 1}, m.Get("a"))
	require.Equal(t, []int{3, 4}, m.RemoveAll("b"))

	inverted := m.Invert()
	require.Equal(t, []string{"a"}, inverted.Get(2), "inversion swaps keys and values")

	m.Replace("a", 42)
	require.Equal(t, []int{42}, m.Get("a"))
	require.Equal(t, 1, m.Size())

	m.Clear()
	require.True(t, m.IsEmpty())

	reserved := struct233.NewListMultimapWithCapacity[string, int](8, "x")
	require.True(t, reserved.IsEmpty(), "reserved keys carry no values")
	require.Zero(t, reserved.KeyCount())
	reserved.Put("x", 1)
	require.Equal(t, []string{"x"}, reserved.Keys(), "the reserved slot activates in place")
}

func TestNewSetMultimap(t *testing.T) {
	m := struct233.NewSetMultimap[string, int]()
	require.True(t, m.Put("a", 1))
	require.False(t, m.Put("a", 1), "a SetMultimap ignores duplicates")
	require.Equal(t, 2, m.PutAll("a", 1, 2, 3), "PutAll reports how many were new")

	require.Equal(t, []int{1, 2, 3}, m.Get("a"))
	require.Equal(t, 3, m.Size())
	require.Equal(t, 1, m.KeyCount())

	require.True(t, m.Remove("a", 2))
	require.False(t, m.Remove("a", 2))
	require.Equal(t, []int{1, 3}, m.Get("a"))

	inverted := m.Invert()
	require.Equal(t, []string{"a"}, inverted.Get(1))
	require.Equal(t, 2, inverted.KeyCount())

	entries := m.Entries()
	require.Len(t, entries, 2)
	require.Equal(t, "a", entries[0].Key)
	require.Equal(t, 1, entries[0].Value)

	require.Equal(t, map[string][]int{"a": {1, 3}}, m.AsMap())

	reserved := struct233.NewSetMultimapWithCapacity[string, int](4, "a")
	require.True(t, reserved.IsEmpty(), "reserved keys are invisible until Put")
	require.Empty(t, reserved.Keys())
}

func TestGuavaAliasesAreTheImplementationTypes(t *testing.T) {
	var m *struct233.Multiset[int] = struct233.NewMultiset(1, 1, 2)
	require.Equal(t, 3, m.TotalCount())

	var j *struct233.Joiner = struct233.NewJoiner("/")
	require.Equal(t, "a/b", j.Join("a", "b"))

	var s *struct233.Splitter = struct233.NewSplitter("/")
	require.Equal(t, []string{"a", "b"}, s.Split("a/b"))

	var lm *struct233.ListMultimap[string, int] = struct233.NewListMultimap[string, int]()
	lm.Put("k", 1)
	require.Equal(t, 1, lm.Size())

	var sm *struct233.SetMultimap[string, int] = struct233.NewSetMultimap[string, int]()
	require.True(t, sm.Put("k", 1))

	var e struct233.MultimapEntry[string, int] = struct233.MultimapEntry[string, int]{Key: "k", Value: 1}
	require.Equal(t, "k", e.Key)

	var rs *struct233.RangeSet[int] = struct233.NewRangeSet[int]()
	rs.Add(struct233.RangeSingleton(1))
	require.True(t, rs.Contains(1))

	var r struct233.Range[int] = struct233.RangeClosed(1, 2)
	require.True(t, r.Contains(1))
}

func TestFacadeRangeConstructors(t *testing.T) {
	require.Equal(t, "[1,3]", struct233.RangeClosed(1, 3).String())
	require.Equal(t, "(1,3)", struct233.RangeOpen(1, 3).String())
	require.Equal(t, "(1,3]", struct233.RangeOpenClosed(1, 3).String())
	require.Equal(t, "[1,3)", struct233.RangeClosedOpen(1, 3).String())
	require.Equal(t, "[1,+inf)", struct233.RangeAtLeast(1).String())
	require.Equal(t, "(-inf,3]", struct233.RangeAtMost(3).String())
	require.Equal(t, "(1,+inf)", struct233.RangeGreaterThan(1).String())
	require.Equal(t, "(-inf,3)", struct233.RangeLessThan(3).String())
	require.Equal(t, "[1,1]", struct233.RangeSingleton(1).String())
	require.Equal(t, "(-inf,+inf)", struct233.RangeAll[int]().String())

	require.True(t, struct233.RangeClosed(3, 1).IsEmpty(), "crossed ends are empty")
	require.True(t, struct233.RangeOpen(1, 1).IsEmpty(), "a point cannot be open on both ends")
	require.False(t, struct233.RangeSingleton(1).IsEmpty())
	require.True(t, struct233.RangeAll[int]().Encloses(struct233.RangeSingleton(1)))
	require.True(t, struct233.RangeClosed(1, 4).IsConnected(struct233.RangeClosedOpen(4, 7)))
	require.False(t, struct233.RangeOpen(1, 4).IsConnected(struct233.RangeOpen(4, 7)))

	x, ok := struct233.RangeClosed(1, 3).Intersection(struct233.RangeOpen(2, 5))
	require.True(t, ok)
	require.Equal(t, "(2,3]", x.String())

	_, ok = struct233.RangeClosed(1, 2).Intersection(struct233.RangeOpen(2, 5))
	require.False(t, ok)

	require.Equal(t, "[1,5)", struct233.RangeClosed(1, 3).Span(struct233.RangeClosedOpen(3, 5)).String())
	require.True(t, struct233.RangeOpen(1, 3).Equal(struct233.RangeOpen(1, 3)))
	require.True(t, struct233.RangeClosed(3, 1).Equal(struct233.RangeOpen(1, 1)), "two empty ranges are equal")
}

func TestFacadeRangeSet(t *testing.T) {
	s := struct233.NewRangeSet[int]()
	s.Add(struct233.RangeClosed(1, 3))
	s.Add(struct233.RangeAtLeast(3))
	require.Equal(t, 1, s.Len(), "closed-touching ranges coalesce into one span")
	require.True(t, s.Contains(math.MaxInt), "an unbounded span reaches +inf")
	require.True(t, s.Encloses(struct233.RangeSingleton(2)))

	s.Remove(struct233.RangeOpenClosed(1, 2))
	require.Equal(t, "[[1,1] (2,+inf)]", fmt.Sprint(s.Spans()))
	require.False(t, s.Contains(2))

	other := struct233.NewRangeSet[int]()
	other.Add(struct233.RangeClosedOpen(5, 8))
	x := s.Intersection(other)
	require.Equal(t, 1, x.Len())
	require.True(t, x.Contains(5))
	require.False(t, x.Contains(8), "the intersection keeps the open end")

	require.True(t, s.Intersects(other))
	require.False(t, s.Intersects(struct233.NewRangeSet[int]()))
	require.True(t, s.EnclosesAll(struct233.NewRangeSet[int]()), "the empty set is enclosed by anything")

	d := struct233.NewRangeSet[int]()
	d.Add(struct233.RangeAtLeast(1))
	d.Difference(struct233.NewRangeSet[int]())
	require.Equal(t, 1, d.Len(), "difference with an empty set is a no-op")

	comp := d.Complement()
	require.True(t, comp.Contains(0))
	require.False(t, comp.Contains(1))
	require.True(t, comp.Contains(math.MinInt))

	s.Clear()
	require.True(t, s.IsEmpty())
	require.Equal(t, 0, len(s.Spans()))

	var seen []string
	for r := range s.All() {
		seen = append(seen, r.String())
	}
	require.Empty(t, seen)
}
