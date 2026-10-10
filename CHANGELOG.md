# Changelog

All notable changes to this project are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-10-10

First release as **struct233-go** (formerly `go-data-struct-lib`).

### Added

- Unified facade `struct233`: factory functions (`NewSet`, `NewSortedSet`,
  `NewLinkedSet`, `NewSkipList`, `NewTreeMap`, `NewLinkedMap`,
  `NewPriorityQueue`, `NewDeque`, `NewRingBuffer`, `NewBitSet`, `NewLRU`,
  `NewConcurrentSet`, `NewConcurrentMap`, `NewConcurrentSkipList`,
  `NewConcurrentLRU`) plus type aliases.
- `skiplist`: ordered map with NavigableMap operations (`First`, `Last`,
  `Floor`, `Ceiling`, `Lower`, `Higher`, `Range`), generic comparator support.
- `set`: `HashSet`, `LinkedHashSet` (insertion order), `TreeSet` (skip-list
  backed, NavigableSet operations) and set algebra.
- `pqueue`: binary-heap priority queue with custom comparators and O(n)
  heapify.
- `deque`: circular-array double-ended queue (FIFO queue + LIFO stack).
- `bitset`: growable bit vector with `Cardinality`, `NextSetBit`,
  `PrevSetBit`, `And`, `Or`, `Xor`, `AndNot`.
- `lru`: fixed-capacity least-recently-used cache (`Get` promotes, `Peek`
  does not).
- `linkedmap`: insertion-ordered map (`java.util.LinkedHashMap` analog).
- `ringbuffer`: fixed-capacity ring buffer with overwrite-oldest semantics.
- `concurrent`: goroutine-safe `Set`, `Map` (atomic `GetOrPut`), `SkipList`
  and `LRU`, iterating over snapshots.
- Performance: zero-allocation read paths, per-instance scratch buffers for
  skip-list writes, in-place bit-set growth, pre-sized set algebra results.
  Benchmarks and allocation guards ship with every package.
- testify-based test suite, runnable examples, and GitHub Actions CI
  (format, vet, test with `-race`, benchmarks).

[0.1.0]: https://github.com/neko233-com/struct233-go/releases/tag/v0.1.0
