# Geobed Improvement Plan

**Created:** February 2026
**Repository:** github.com/jvmatl/geobed (forked)
**Purpose:** Comprehensive remediation plan for identified issues

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

### 1.1 Defer Before Error Check (Line 246)

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

**Solution:**
```go
r, rErr := http.Get(f["url"])
if rErr != nil {
    log.Printf("failed to download %s: %v", f["url"], rErr)
    continue
}
defer r.Body.Close()
```

**Files:** `geobed.go:246`

---

### 1.2 log.Fatal() Crashes Application (Lines 274, 281, 427)

**Problem:**

`log.Fatal()` calls `os.Exit(1)`, which:
- Terminates the entire process immediately
- Skips all deferred functions
- Provides no opportunity for graceful degradation
- Makes the library unusable in long-running services

**Synthesis:**

According to [Go error handling best practices](https://go.dev/blog/error-handling-and-go), `log.Fatal()` should only be used in `main()` for truly unrecoverable startup errors. The [JetBrains Go guide](https://www.jetbrains.com/guide/go/tutorials/handle_errors_in_go/best_practices/) states: "Crashing is not always the best option. If an error is easy to recover from, crashing the whole application is an overreaction."

The modern Go approach is to return errors up the call stack with context using `fmt.Errorf` with `%w` for error wrapping.

**Solution:**

Change `NewGeobed()` signature to return error:
```go
func NewGeobed() (*GeoBed, error) {
    // ...
    if err != nil {
        return nil, fmt.Errorf("failed to load geobed data: %w", err)
    }
    return &g, nil
}
```

For internal functions, propagate errors instead of calling `log.Fatal()`:
```go
// Before
log.Fatal("Error parsing file: ", err)

// After
return fmt.Errorf("parsing %s: %w", filename, err)
```

**Files:** `geobed.go:274, 281, 427`

---

### 1.3 Missing Bounds Check (Line 438)

**Problem:**
```go
if string(t[0]) != "#" {  // No check if t is empty
```

Empty lines in data files cause index out of bounds panic.

**Synthesis:**

Defensive programming requires validating slice bounds before access. This is a common source of production panics.

**Solution:**
```go
if len(t) > 0 && string(t[0]) != "#" {
```

**Files:** `geobed.go:438`

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

### 2.2 O(n) Reverse Geocoding Scan

**Problem:**

Reverse geocoding scans through all cities comparing geohash prefixes. This is O(n) where n = 2.7M, resulting in 100-180ms per query.

**Synthesis:**

Modern geospatial indexing uses hierarchical spatial structures. [Google's S2 Geometry library](https://github.com/golang/geo) (updated December 2025) provides spherical geometry with built-in spatial indexing. The [S2 library](https://s2geometry.io/) offers:
- Fast in-memory indexing of points, polylines, and polygons
- Algorithms for measuring distances and finding nearby objects
- Support for spatial indexing using discrete "S2 cells"

Alternatively, [rtreego](https://github.com/dhconnelly/rtreego) provides R-tree indexing with k-nearest-neighbor queries.

**Solution:**

Replace geohash linear scan with S2 spatial index:
```go
import "github.com/golang/geo/s2"

type GeoBed struct {
    c       Cities
    co      []CountryInfo
    index   *s2.ShapeIndex  // Add spatial index
}

func (g *GeoBed) buildSpatialIndex() {
    for i, city := range g.c {
        point := s2.PointFromLatLng(s2.LatLngFromDegrees(city.Latitude, city.Longitude))
        g.index.Add(point, i)
    }
}

func (g *GeoBed) ReverseGeocode(lat, lng float64) GeobedCity {
    query := s2.NewClosestPointQuery(g.index, s2.NewClosestPointQueryOptions())
    target := s2.PointFromLatLng(s2.LatLngFromDegrees(lat, lng))
    result := query.FindClosestPoint(target)
    return g.c[result.ShapeID()]
}
```

**Expected Improvement:** O(n) → O(log n), ~100-180ms → <1ms

**Files:** `geobed.go` (ReverseGeocode, NewGeobed)

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

### 2.4 Global Mutable State (Thread Safety)

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

**Solution:**

Move indices into the GeoBed struct:
```go
type GeoBed struct {
    c            Cities
    co           []CountryInfo
    cityNameIdx  map[string]int  // Move from global
    mu           sync.RWMutex    // For thread-safe access
}

// For singleton pattern with thread-safe initialization:
var (
    defaultGeobed *GeoBed
    geobedOnce    sync.Once
    geobedErr     error
)

func GetDefaultGeobed() (*GeoBed, error) {
    geobedOnce.Do(func() {
        defaultGeobed, geobedErr = NewGeobed()
    })
    return defaultGeobed, geobedErr
}
```

**Files:** `geobed.go` (struct definition, initialization)

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

### 3.2 Commented-Out Code Throughout

**Problem:**

Multiple sections of commented-out code:
- Lines 56-57: MaxMind datasets
- Lines 501-512: Alternative index strategy
- Lines 648-661: Airport code logic
- Test file: Multiple disabled test cases

**Synthesis:**

Dead code increases cognitive load and maintenance burden. If code is not needed, remove it. If it might be needed later, document why in an issue or design doc.

**Solution:**

1. Remove commented-out code entirely
2. Create GitHub issues for features that might be revisited:
   - Issue: "Consider re-enabling MaxMind dataset for additional city coverage"
   - Issue: "Evaluate airport code geocoding feature"
3. Use git history to recover code if needed later

**Files:** `geobed.go`, `geobed_test.go`

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

### 3.5 Logic Error in openOptionallyBzippedFile (Line 1067)

**Problem:**

The function has incorrect return logic - returns nil on success path.

**Synthesis:**

Review and fix control flow to ensure proper reader return.

**Solution:**

Audit the function and ensure all paths return correctly:
```go
func openOptionallyBzippedFile(path string) (io.Reader, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err  // Error case
    }

    if strings.HasSuffix(path, ".bz2") {
        return bzip2.NewReader(f), nil  // Success with bzip2
    }
    return f, nil  // Success without compression
}
```

**Files:** `geobed.go:1067`

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

### 6.1 Missing API Documentation

**Problem:**

No Go doc comments on public functions.

**Solution:**

Add comprehensive documentation:
```go
// GeoBed provides offline geocoding capabilities using embedded city data.
// It supports both forward geocoding (name → coordinates) and reverse
// geocoding (coordinates → location).
//
// GeoBed instances are safe for concurrent use after initialization.
// Initialization loads approximately 2.7 million city records into memory,
// requiring ~100MB RAM.
type GeoBed struct { ... }

// NewGeobed creates a new GeoBed instance with default configuration.
// It loads city data from embedded cache files.
//
// Returns an error if data cannot be loaded. The returned GeoBed
// instance is safe for concurrent use.
//
// Example:
//
//     g, err := NewGeobed()
//     if err != nil {
//         log.Fatal(err)
//     }
//     city := g.Geocode("Austin, TX")
//     fmt.Printf("%s: %f, %f\n", city.City, city.Latitude, city.Longitude)
func NewGeobed() (*GeoBed, error) { ... }
```

**Files:** `geobed.go`

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

| Priority | Issue | Impact | Effort | Category |
|----------|-------|--------|--------|----------|
| **P0** | Defer before error check | Critical bug | Low | Critical |
| **P0** | log.Fatal() usage | Critical bug | Medium | Critical |
| **P0** | Missing bounds check | Critical bug | Low | Critical |
| **P1** | Thread-safe initialization | Data race | Medium | Memory/Perf |
| **P1** | Return errors from NewGeobed | API breaking | Medium | Critical |
| **P2** | S2 spatial index | Performance | High | Memory/Perf |
| **P2** | String interning | Memory | Medium | Memory/Perf |
| **P2** | API documentation | Usability | Medium | Docs |
| **P3** | Remove commented code | Maintainability | Low | Code Quality |
| **P3** | Type-safe data sources | Maintainability | Low | Code Quality |
| **P3** | Configurable paths | Flexibility | Low | Code Quality |
| **P3** | Comprehensive tests | Reliability | Medium | Testing |
| **P4** | Fuzzy matching (Levenshtein) | Feature | Medium | Design |
| **P4** | International admin divisions | Feature | High | Design |
| **P4** | Build tags for lite version | Deployment | Medium | Operational |
| **P4** | Data update mechanism | Freshness | Medium | Operational |

---

## Recommended Implementation Order

### Phase 1: Critical Fixes (Week 1)
1. Fix defer before error check
2. Fix missing bounds check
3. Change NewGeobed to return error
4. Replace log.Fatal with error returns

### Phase 2: Thread Safety & API (Week 2)
1. Move global state into struct
2. Add sync.Once for singleton pattern
3. Add API documentation
4. Remove commented-out code

### Phase 3: Performance (Weeks 3-4)
1. Implement S2 spatial index
2. Add string interning with `unique` package
3. Add comprehensive benchmarks

### Phase 4: Polish (Week 5+)
1. Add comprehensive test cases
2. Add build tags for lite version
3. Implement configurable paths
4. Add data update tooling

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
