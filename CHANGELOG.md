# Changelog

All notable changes to this project are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.2.0] - 2026-10-10

### Added

- `bloomfilter`: space-efficient probabilistic set with no false negatives
  (string/bytes/uint64 keys, FNV-1a + splitmix64 double hashing, tunable
  false-positive rate).
- `unionfind`: disjoint-set union with path compression and union by size
  (`Find`, `Union`, `Connected`, `Size`, `Components`).
- `ttlcache`: goroutine-safe cache that combines LRU eviction with per-entry
  time-to-live and an injectable clock (`PutWithTTL`, lazy expiry, `Cleanup`).
- `concurrent.Queue`: bounded blocking FIFO queue with `ArrayBlockingQueue`
  semantics (`Put`, `Offer` with timeout, `Take`, `Poll`, `TryPut`,
  `TryTake`, `Close`).
- Facade factories and aliases for all of the above: `NewBloomFilter`,
  `NewUnionFind`, `NewTTLCache`, `NewTTLCacheWithClock`, `NewConcurrentQueue`.
- Fuzz targets comparing `skiplist`, `bitset`, `ringbuffer` and `linkedmap`
  against reference implementations.
- Godoc coverage check: 304 exported symbols, 0 undocumented.

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

[0.2.0]: https://github.com/neko233-com/struct233-go/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/neko233-com/struct233-go/releases/tag/v0.1.0
