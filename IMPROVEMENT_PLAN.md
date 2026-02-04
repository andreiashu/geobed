# Geobed Improvement Plan

**Created:** February 2026
**Updated:** February 2026
**Repository:** github.com/jvmatl/geobed (forked)
**Purpose:** Comprehensive remediation plan for identified issues

---

## Progress Summary

| Phase | Status | Commit |
|-------|--------|--------|
| Phase 1: Critical Fixes | ✅ Complete | `cf194e4` |
| Phase 2: Thread Safety & API | ✅ Complete | `cf194e4` |
| Phase 3: Performance | ✅ Complete | `7c60787` (S2 index) |
| Phase 4: Polish | 🔲 Pending | - |

---

## Table of Contents

1. [Critical Issues](#1-critical-issues)
2. [Memory & Performance Issues](#2-memory--performance-issues)
3. [Code Quality Issues](#3-code-quality-issues)
4. [Design & Architecture Issues](#4-design--architecture-issues)
5. [Testing Gaps](#5-testing-gaps)
6. [Documentation & Operational Issues](#6-documentation--operational-issues)
7. [Implementation Priority Matrix](#7-implementation-priority-matrix)

---

## 1. Critical Issues

### 1.1 ✅ Defer Before Error Check (Line 246) — FIXED

**Status:** Fixed in commit `cf194e4`

**Problem:**
```go
r, rErr := http.Get(f["url"])
defer r.Body.Close()  // ⚠️ Deferred before checking rErr
if rErr != nil {
    // ...
}
```

If `http.Get` fails, `r` is nil, and `defer r.Body.Close()` causes a nil pointer dereference panic.

**Synthesis:**

The [correct Go pattern](https://medium.com/@KeithAlpichi/go-gotcha-closing-a-nil-http-response-body-with-defer-9b7a3eb30e8c) is to always check errors before deferring cleanup. As noted in [Go defer best practices](https://dev.to/zakariachahboun/common-use-cases-for-defer-in-go-1071), "get the resource, check for any error, and only then defer the close."

**Solution Applied:**

Refactored `downloadDataSets()` into a new `downloadFile()` helper with proper error handling:
```go
func downloadFile(url, path string) error {
    resp, err := http.Get(url)
    if err != nil {
        return fmt.Errorf("HTTP GET %s: %w", url, err)
    }
    defer resp.Body.Close()  // Now safely after error check
    // ...
}
```

---

### 1.2 ✅ log.Fatal() Crashes Application — FIXED

**Status:** Fixed in commit `cf194e4`

**Problem:**

`log.Fatal()` calls `os.Exit(1)`, which:
- Terminates the entire process immediately
- Skips all deferred functions
- Provides no opportunity for graceful degradation
- Makes the library unusable in long-running services

**Synthesis:**

According to [Go error handling best practices](https://go.dev/blog/error-handling-and-go), `log.Fatal()` should only be used in `main()` for truly unrecoverable startup errors. The [JetBrains Go guide](https://www.jetbrains.com/guide/go/tutorials/handle_errors_in_go/best_practices/) states: "Crashing is not always the best option. If an error is easy to recover from, crashing the whole application is an overreaction."

**Solution Applied:**

Changed `NewGeobed()` signature to return error:
```go
func NewGeobed() (*GeoBed, error) {
    // ...
    if err != nil {
        return nil, fmt.Errorf("failed to load geobed data: %w", err)
    }
    return g, nil
}
```

All internal functions now propagate errors instead of calling `log.Fatal()`.

---

### 1.3 ✅ Missing Bounds Check (Line 438) — FIXED

**Status:** Fixed in commit `cf194e4`

**Problem:**
```go
if string(t[0]) != "#" {  // No check if t is empty
```

Empty lines in data files cause index out of bounds panic.

**Synthesis:**

Defensive programming requires validating slice bounds before access. This is a common source of production panics.

**Solution Applied:**

Refactored into `loadGeonamesCountryInfo()` with proper bounds check:
```go
if len(t) == 0 || t[0] == '#' {
    continue
}
```

---

## 2. Memory & Performance Issues

### 2.1 Full In-Memory Storage (~56MB+ Compressed)

**Problem:**

All 2.7M+ cities are loaded into RAM. The embedded GOB cache is ~56MB compressed, expanding larger in memory. This is unsuitable for resource-constrained environments.

**Synthesis:**

For [embedded files in Go](https://vincent.bernat.ch/en/blog/2025-go-embed-compressed), there are several strategies to manage binary size:

1. **ZIP compression** - Pre-compress assets before embedding (currently using bzip2, which is good)
2. **Selective loading** - Only load data when needed
3. **Memory-mapped files** - Use mmap for large datasets
4. **Binary packers** - Use UPX to compress final binary

**Solution:**

Short-term:
- Document memory requirements clearly
- Consider lazy loading of less-populated cities

Long-term:
- Implement tiered loading (load high-population cities first)
- Add build tags for "lite" version with reduced dataset
- Consider SQLite with memory-mapped access for truly constrained environments

**Files:** `geobed.go` (architecture change)

---

### 2.2 ✅ O(n) Reverse Geocoding Scan — FIXED

**Status:** Fixed in commit `7c60787`

**Problem:**

Reverse geocoding scans through all cities comparing geohash prefixes. This is O(n) where n = 2.7M, resulting in 100-180ms per query.

**Synthesis:**

Modern geospatial indexing uses hierarchical spatial structures. [Google's S2 Geometry library](https://github.com/golang/geo) (updated December 2025) provides spherical geometry with built-in spatial indexing. The [S2 library](https://s2geometry.io/) offers:
- Fast in-memory indexing of points, polylines, and polygons
- Algorithms for measuring distances and finding nearby objects
- Support for spatial indexing using discrete "S2 cells"

Alternatively, [rtreego](https://github.com/dhconnelly/rtreego) provides R-tree indexing with k-nearest-neighbor queries.

**Solution Applied:**

Implemented S2 cell-based spatial index (simpler and more elegant than full ShapeIndex):
```go
import "github.com/golang/geo/s2"

const s2CellLevel = 10  // ~10km cells

type GeoBed struct {
    c         Cities
    co        []CountryInfo
    cellIndex map[s2.CellID][]int  // S2 cell → city indices
}

func (g *GeoBed) buildCellIndex() {
    g.cellIndex = make(map[s2.CellID][]int)
    for i, city := range g.c {
        ll := s2.LatLngFromDegrees(city.Latitude, city.Longitude)
        cell := s2.CellIDFromLatLng(ll).Parent(s2CellLevel)
        g.cellIndex[cell] = append(g.cellIndex[cell], i)
    }
}

func (g *GeoBed) ReverseGeocode(lat, lng float64) GeobedCity {
    queryLL := s2.LatLngFromDegrees(lat, lng)
    queryCell := s2.CellIDFromLatLng(queryLL).Parent(s2CellLevel)

    // Check query cell + 8 neighbors, find closest
    for _, cell := range g.cellAndNeighbors(queryCell) {
        for _, idx := range g.cellIndex[cell] {
            // Compare distances using S2's spherical geometry
        }
    }
}
```

**Actual Improvement:** O(n) → O(k) where k ≈ 100-500 cities
- Before: ~100-180ms per query
- After: ~8μs per query (~12,000-22,000x faster)
- Throughput: ~150,000 queries/second

**Files:** `geobed.go` (ReverseGeocode, NewGeobed, buildCellIndex, cellAndNeighbors)

---

### 2.3 No String Interning

**Problem:**

City names like "Paris", "London", "Springfield" appear multiple times across the dataset. Each occurrence allocates separate memory.

**Synthesis:**

Go 1.23 introduced the [`unique` package](https://go.dev/blog/unique) for canonical value deduplication. As explained by [VictoriaMetrics](https://victoriametrics.com/blog/go-unique-package-intern-string/):
- "When you've got several identical values, you only store one copy"
- "Comparison of two Handle[T] values is cheap: it comes down to a pointer comparison"
- Automatic garbage collection of unused interned strings

For pre-1.23 compatibility, [go4.org/intern](https://commaok.xyz/post/intern-strings/) provides similar functionality.

**Solution:**

Use `unique.Make()` for frequently repeated strings:
```go
import "unique"

type GeobedCity struct {
    City       unique.Handle[string]  // Interned
    CityAlt    string                 // Not interned (unique per city)
    Country    unique.Handle[string]  // Interned (only ~200 values)
    Region     unique.Handle[string]  // Interned
    // ...
}

// During loading:
city.Country = unique.Make(countryCode)
city.City = unique.Make(cityName)
```

**Expected Improvement:** 15-30% memory reduction for string data

**Files:** `geobed.go` (GeobedCity struct, loading functions)

---

### 2.4 ✅ Global Mutable State (Thread Safety) — FIXED

**Status:** Fixed in commit `cf194e4`

**Problem:**

```go
var cityNameIdx = make(map[string]int)
var locationDedupeIdx = make(map[string]bool)
var maxMindCityDedupeIdx = make(map[string][]string)
```

These package-level variables are:
- Not thread-safe for concurrent `NewGeobed()` calls
- Create hidden dependencies between instances
- Make testing difficult

**Synthesis:**

[Dave Cheney's guidance](https://dave.cheney.net/2017/06/11/go-without-package-scoped-variables) on avoiding package-scoped variables: "Package-global objects can encode state and/or behavior that is hidden from external callers."

[Peter Bourgon's theory of modern Go](https://peter.bourgon.org/blog/2017/06/09/theory-of-modern-go.html) recommends: "Pass variables or dependencies as parameters to functions or methods, promoting explicitness."

For singleton initialization, use [`sync.Once`](https://leapcell.io/blog/go-sync-once-pattern) which "guarantees that the initialization function is called exactly once, even in a concurrent environment."

**Solution Applied:**

Moved `cityNameIdx` into the GeoBed struct:
```go
type GeoBed struct {
    c           Cities
    co          []CountryInfo
    cityNameIdx map[string]int  // Moved from global
}
```

Added `GetDefaultGeobed()` with `sync.Once` for thread-safe singleton:
```go
var (
    defaultGeobed     *GeoBed
    defaultGeobedOnce sync.Once
    defaultGeobedErr  error
)

func GetDefaultGeobed() (*GeoBed, error) {
    defaultGeobedOnce.Do(func() {
        defaultGeobed, defaultGeobedErr = NewGeobed()
    })
    return defaultGeobed, defaultGeobedErr
}
```

Note: `locationDedupeIdx` and `maxMindCityDedupeIdx` remain as temporary globals used only during data loading and are cleared after use.

---

## 3. Code Quality Issues

### 3.1 Repetitive Store Function (Line 967)

**Problem:**

The `store()` function repeats the same encode/write pattern three times for different data structures.

**Synthesis:**

Apply DRY (Don't Repeat Yourself) principle with a generic helper function.

**Solution:**
```go
func storeGob[T any](filename string, data T) error {
    f, err := os.Create(filename)
    if err != nil {
        return fmt.Errorf("creating %s: %w", filename, err)
    }
    defer f.Close()

    bw := bzip2.NewWriter(f)
    defer bw.Close()

    enc := gob.NewEncoder(bw)
    if err := enc.Encode(data); err != nil {
        return fmt.Errorf("encoding %s: %w", filename, err)
    }
    return nil
}

func (g *GeoBed) store() error {
    if err := storeGob("geobed-cache/g.c.dmp.bz2", g.c); err != nil {
        return err
    }
    if err := storeGob("geobed-cache/g.co.dmp.bz2", g.co); err != nil {
        return err
    }
    return storeGob("geobed-cache/cityNameIdx.dmp.bz2", cityNameIdx)
}
```

**Files:** `geobed.go:967+`

---

### 3.2 ✅ Commented-Out Code Throughout — FIXED

**Status:** Fixed in commit `cf194e4`

**Problem:**

Multiple sections of commented-out code:
- Lines 56-57: MaxMind datasets
- Lines 501-512: Alternative index strategy
- Lines 648-661: Airport code logic
- Test file: Multiple disabled test cases

**Synthesis:**

Dead code increases cognitive load and maintenance burden. If code is not needed, remove it. If it might be needed later, document why in an issue or design doc.

**Solution Applied:**

1. Removed commented-out MaxMind dataset entries from `dataSetFiles`
2. Removed commented-out airport code logic from `fuzzyMatchLocation()`
3. Removed commented-out TODO comments about string interning and mmap
4. Cleaned up test file by removing disabled test cases and updating working ones
5. Removed verbose debug logging statements

---

### 3.3 Magic Strings for Dataset Configuration

**Problem:**

```go
var dataSetFiles = []map[string]string{
    {"url": "http://...", "type": "cities"},
    {"url": "http://...", "type": "countryInfo"},
}
```

String-based keys are error-prone and lack IDE support.

**Solution:**

Use typed structs:
```go
type DataSource struct {
    URL      string
    Type     DataSourceType
    Format   DataFormat
    Enabled  bool
}

type DataSourceType int
const (
    DataSourceCities DataSourceType = iota
    DataSourceCountryInfo
    DataSourceMaxMind
)

type DataFormat int
const (
    FormatZIP DataFormat = iota
    FormatGzip
    FormatPlain
)

var dataSources = []DataSource{
    {URL: "http://download.geonames.org/...", Type: DataSourceCities, Format: FormatZIP, Enabled: true},
    // ...
}
```

**Files:** `geobed.go:53-58`

---

### 3.4 Hardcoded Paths

**Problem:**

```go
const geobedDataDir = "./geobed-data/"
const geobedCacheDir = "./geobed-cache/"
```

No configurability for different deployment scenarios.

**Solution:**

Add configuration options:
```go
type GeobedConfig struct {
    DataDir    string
    CacheDir   string
    MaxCities  int  // Optional limit
}

func NewGeobedWithConfig(cfg GeobedConfig) (*GeoBed, error) {
    if cfg.DataDir == "" {
        cfg.DataDir = "./geobed-data/"
    }
    if cfg.CacheDir == "" {
        cfg.CacheDir = "./geobed-cache/"
    }
    // ...
}

// Preserve backwards compatibility
func NewGeobed() (*GeoBed, error) {
    return NewGeobedWithConfig(GeobedConfig{})
}
```

**Files:** `geobed.go`

---

### 3.5 ✅ Logic Error in openOptionallyBzippedFile (Line 1067) — FIXED

**Status:** Fixed in commit `cf194e4`

**Problem:**

The function had incorrect return logic - `if err == nil` should have been `if err != nil`:
```go
fh, err = openOptionallyCachedFile(file)
if err == nil {  // ⚠️ Wrong condition!
    return nil, err
}
```

**Synthesis:**

Review and fix control flow to ensure proper reader return.

**Solution Applied:**

Fixed the logic error and simplified the function:
```go
func openOptionallyBzippedFile(file string) (io.Reader, error) {
    fh, err := openOptionallyCachedFile(file + ".bz2")
    if err != nil {
        // Try uncompressed version
        fh, err = openOptionallyCachedFile(file)
        if err != nil {
            return nil, fmt.Errorf("opening %s: %w", file, err)
        }
        return fh, nil
    }
    return bzip2.NewReader(fh), nil
}
```

---

## 4. Design & Architecture Issues

### 4.1 Scoring System Complexity

**Problem:**

The fuzzy matching scoring system uses magic numbers:
- +7 points: Exact city name match
- +5 points: Region match
- +5 points: Alternate name exact match
- etc.

No documentation explains the rationale.

**Solution:**

1. Extract scoring constants with documentation:
```go
// Scoring weights for fuzzy location matching.
// These weights were empirically tuned to prioritize:
// 1. Exact matches over partial matches
// 2. City names over administrative regions
// 3. US state abbreviations (common query pattern)
const (
    ScoreExactCity      = 7  // Highest priority: exact city name
    ScoreRegionMatch    = 5  // State/province match
    ScoreAltNameExact   = 5  // Alternate name (localized)
    ScoreCountryMatch   = 4  // Country name or abbreviation
    ScorePartialCity    = 2  // City name contains query
    ScorePartialPiece   = 1  // Any piece matches
)
```

2. Consider making scoring configurable for different use cases

**Files:** `geobed.go` (fuzzyMatchLocation)

---

### 4.2 Limited Fuzzy Matching

**Problem:**

No support for typo correction or phonetic matching. "Londn" won't match "London".

**Solution:**

Add optional fuzzy matching with Levenshtein distance:
```go
import "github.com/agnivade/levenshtein"

type GeocodeOptions struct {
    ExactCity     bool
    FuzzyDistance int  // Max edit distance for typo tolerance (0 = disabled)
}

func (g *GeoBed) fuzzyMatch(query, candidate string, maxDist int) bool {
    if maxDist == 0 {
        return strings.EqualFold(query, candidate)
    }
    return levenshtein.ComputeDistance(
        strings.ToLower(query),
        strings.ToLower(candidate),
    ) <= maxDist
}
```

**Files:** `geobed.go`

---

### 4.3 US-Centric Design

**Problem:**

`UsStateCodes` is hardcoded but no equivalent exists for other countries' administrative divisions.

**Solution:**

1. Generalize to support multiple countries:
```go
type AdminDivision struct {
    Code string
    Name string
}

var AdminDivisions = map[string]map[string]AdminDivision{
    "US": {"TX": {Code: "TX", Name: "Texas"}, ...},
    "CA": {"ON": {Code: "ON", Name: "Ontario"}, ...},
    "GB": {"ENG": {Code: "ENG", Name: "England"}, ...},
}
```

2. Load from Geonames admin1CodesASCII.txt for complete coverage

**Files:** `geobed.go`

---

### 4.4 Data Staleness

**Problem:**

Data snapshot from August 2023 with no update mechanism.

**Solution:**

1. Document data freshness in README
2. Add `make update-data` target to refresh from Geonames
3. Consider CI/CD job to check for updates monthly
4. Add data version metadata:
```go
type GeoBed struct {
    // ...
    DataVersion   string    // e.g., "2026.02"
    DataTimestamp time.Time // When data was fetched
}
```

**Files:** `geobed.go`, `Makefile` (new), `README.md`

---

## 5. Testing Gaps

### 5.1 Limited Test Coverage

**Problem:**

- Only 9 geocode test cases
- Mostly US-focused
- No edge case testing

**Solution:**

Add comprehensive test cases:
```go
func (s *GeobedSuite) TestGeocodeEdgeCases(c *check.C) {
    g := NewGeobed()

    // Empty/invalid input
    c.Check(g.Geocode("").City, check.Equals, "")
    c.Check(g.Geocode("   ").City, check.Equals, "")
    c.Check(g.Geocode("12345").City, check.Equals, "")

    // Unicode/international
    c.Check(g.Geocode("München").City, check.Equals, "Munich")
    c.Check(g.Geocode("東京").City, check.Equals, "Tokyo")
    c.Check(g.Geocode("São Paulo").City, check.Equals, "São Paulo")

    // Ambiguous names
    c.Check(g.Geocode("Paris, France").Country, check.Equals, "FR")
    c.Check(g.Geocode("Paris, Texas").Region, check.Equals, "TX")

    // Case insensitivity
    c.Check(g.Geocode("LONDON").City, check.Equals, "London")
    c.Check(g.Geocode("london").City, check.Equals, "London")
}
```

**Files:** `geobed_test.go`

---

### 5.2 No Concurrency Tests

**Problem:**

No tests verify thread safety of geocoding operations.

**Solution:**

```go
func (s *GeobedSuite) TestConcurrentGeocode(c *check.C) {
    g := NewGeobed()

    var wg sync.WaitGroup
    cities := []string{"London", "Paris", "Tokyo", "New York", "Sydney"}

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(city string) {
            defer wg.Done()
            result := g.Geocode(city)
            c.Check(result.City, check.Not(check.Equals), "")
        }(cities[i%len(cities)])
    }

    wg.Wait()
}
```

**Files:** `geobed_test.go`

---

## 6. Documentation & Operational Issues

### 6.1 ✅ Missing API Documentation — FIXED

**Status:** Fixed in commit `cf194e4`

**Problem:**

No Go doc comments on public functions.

**Solution Applied:**

Added comprehensive godoc comments to all public types and functions:
- `GeoBed` struct with usage description
- `NewGeobed()` with example code
- `GetDefaultGeobed()` for singleton pattern
- `Geocode()` with options documentation
- `ReverseGeocode()` with coordinate examples
- All helper functions with clear descriptions

Example:
```go
// NewGeobed creates a new GeoBed instance with geocoding data loaded into memory.
// It first attempts to load from embedded cache files, falling back to downloading
// and parsing raw data files if the cache is unavailable.
//
// Returns an error if data cannot be loaded from either source.
// For a shared instance with thread-safe initialization, use GetDefaultGeobed() instead.
//
// Example:
//
//	g, err := NewGeobed()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	city := g.Geocode("Austin, TX")
//	fmt.Printf("%s: %f, %f\n", city.City, city.Latitude, city.Longitude)
func NewGeobed() (*GeoBed, error) { ... }
```

---

### 6.2 Large Binary Size

**Problem:**

Embedded cache adds ~56MB to binary, unsuitable for some deployments.

**Synthesis:**

Per [Go embed best practices](https://vincent.bernat.ch/en/blog/2025-go-embed-compressed), options include:
- ZIP compression of assets
- Build tags for optional embedding
- Binary packers (UPX)

**Solution:**

Add build tags for lite version:
```go
//go:build !geobed_lite

//go:embed geobed-cache
var geobedCache embed.FS
```

```go
//go:build geobed_lite

// Lite version downloads data on first use
var geobedCache embed.FS // Empty
```

Build commands:
```bash
go build                          # Full version (~60MB)
go build -tags geobed_lite        # Lite version (~5MB, downloads data)
upx --best geobed                 # Compress with UPX
```

**Files:** `geobed.go`, `geobed_lite.go` (new)

---

## 7. Implementation Priority Matrix

| Priority | Issue | Impact | Effort | Category | Status |
|----------|-------|--------|--------|----------|--------|
| **P0** | Defer before error check | Critical bug | Low | Critical | ✅ Done |
| **P0** | log.Fatal() usage | Critical bug | Medium | Critical | ✅ Done |
| **P0** | Missing bounds check | Critical bug | Low | Critical | ✅ Done |
| **P1** | Thread-safe initialization | Data race | Medium | Memory/Perf | ✅ Done |
| **P1** | Return errors from NewGeobed | API breaking | Medium | Critical | ✅ Done |
| **P2** | S2 spatial index | Performance | High | Memory/Perf | ✅ Done |
| **P2** | String interning | Memory | Medium | Memory/Perf | ⏭️ Deferred |
| **P2** | API documentation | Usability | Medium | Docs | ✅ Done |
| **P3** | Remove commented code | Maintainability | Low | Code Quality | ✅ Done |
| **P3** | Type-safe data sources | Maintainability | Low | Code Quality | 🔲 Pending |
| **P3** | Configurable paths | Flexibility | Low | Code Quality | 🔲 Pending |
| **P3** | Comprehensive tests | Reliability | Medium | Testing | 🔲 Pending |
| **P4** | Fuzzy matching (Levenshtein) | Feature | Medium | Design | 🔲 Pending |
| **P4** | International admin divisions | Feature | High | Design | 🔲 Pending |
| **P4** | Build tags for lite version | Deployment | Medium | Operational |🔲 Pending |
| **P4** | Data update mechanism | Freshness | Medium | Operational | 🔲 Pending |

---

## Recommended Implementation Order

### Phase 1: Critical Fixes ✅ COMPLETE
1. ✅ Fix defer before error check
2. ✅ Fix missing bounds check
3. ✅ Change NewGeobed to return error
4. ✅ Replace log.Fatal with error returns

### Phase 2: Thread Safety & API ✅ COMPLETE
1. ✅ Move global state into struct
2. ✅ Add sync.Once for singleton pattern
3. ✅ Add API documentation
4. ✅ Remove commented-out code
5. ✅ Fix logic error in openOptionallyBzippedFile

### Phase 3: Performance ✅ COMPLETE
1. ✅ Implement S2 spatial index (commit `7c60787`) — **~12,000x speedup**
2. ⏭️ String interning — deferred (moderate benefit, adds complexity)
3. ⏭️ Comprehensive benchmarks — deferred (basic benchmarks exist)

### Phase 4: Polish (Week 5+)
1. 🔲 Add comprehensive test cases
2. 🔲 Add build tags for lite version
3. 🔲 Implement configurable paths
4. 🔲 Add data update tooling

---

## Sources

- [Go Error Handling Best Practices](https://go.dev/blog/error-handling-and-go)
- [Common Mistakes in Go Error Handling - JetBrains](https://www.jetbrains.com/guide/go/tutorials/handle_errors_in_go/common_mistakes/)
- [Go Defer Gotcha: Closing Nil HTTP Response](https://medium.com/@KeithAlpichi/go-gotcha-closing-a-nil-http-response-body-with-defer-9b7a3eb30e8c)
- [Go's unique Package for String Interning](https://victoriametrics.com/blog/go-unique-package-intern-string/)
- [New unique Package - Go Blog](https://go.dev/blog/unique)
- [S2 Geometry Library in Go](https://github.com/golang/geo)
- [S2 Geometry Documentation](https://s2geometry.io/)
- [rtreego - R-Tree Library for Go](https://github.com/dhconnelly/rtreego)
- [sync.Once Deep Dive](https://dev.to/leapcell/a-deep-dive-into-gos-synconce-3inn)
- [Go Without Package-Scoped Variables - Dave Cheney](https://dave.cheney.net/2017/06/11/go-without-package-scoped-variables)
- [Compressing Embedded Files in Go](https://vincent.bernat.ch/en/blog/2025-go-embed-compressed)
- [Using defer in Go: Best Practices](https://dev.to/zakariachahboun/common-use-cases-for-defer-in-go-1071)
