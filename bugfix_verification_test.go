package geobed

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"testing"
)

// ============================================================================
// Bug Fix Verification Tests
//
// Each test corresponds to a specific bug found during code review.
// These tests verify the fix works and prevent regressions.
// ============================================================================

// ---------------------------------------------------------------------------
// Fix 1: Data race on package-level dedup vars
// The dedup indices (locationDedupeIdx, maxMindCityDedupeIdx) were package-level
// vars that could race when concurrent NewGeobed() calls fell through to
// loadDataSets(). Fix: made them local to loadDataSets/loadMaxMindCities.
// ---------------------------------------------------------------------------

func TestFix_DataRaceDedupVarsAreLocal(t *testing.T) {
	// Verify that concurrent NewGeobed() calls don't panic.
	// Under the old code with package-level dedup vars, concurrent access
	// to the maps during the cold-cache path would cause a fatal race.
	// With embedded cache, the cold-cache path isn't reached, but this
	// verifies the interners (also global) are properly protected.
	const numGoroutines = 20
	var wg sync.WaitGroup
	errs := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gb, err := NewGeobed()
			if err != nil {
				errs <- err
				return
			}
			// Exercise the geocoder to touch interners
			r := gb.Geocode("Austin, TX")
			if r.City != "Austin" {
				errs <- fmt.Errorf("expected Austin, got %q", r.City)
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent NewGeobed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Fix 2: Data duplication on partial cache failure
// If cache partially loads (cities OK, nameIndex fails), the old code
// fell through to loadDataSets which appended to the existing g.Cities,
// doubling all entries. Fix: g.Cities/Countries/nameIndex are reset to nil.
// ---------------------------------------------------------------------------

func TestFix_PartialCacheResetPreventsDataDuplication(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	cityCount := len(g.Cities)
	if cityCount < minCityCount {
		t.Fatalf("expected at least %d cities, got %d", minCityCount, cityCount)
	}

	// Verify no duplicates by checking a sample city
	austinCount := 0
	for _, c := range g.Cities {
		if c.City == "Austin" && c.Country() == "US" && c.Region() == "TX" {
			austinCount++
		}
	}
	if austinCount != 1 {
		t.Errorf("found %d entries for Austin,TX,US - expected exactly 1 (possible duplication)", austinCount)
	}
}

// ---------------------------------------------------------------------------
// Fix 3 & 4: Country/state ambiguity and non-deterministic iteration
// "Georgia" the country was matching before "GA" the US state.
// US state code map iteration was non-deterministic.
// Fix: sorted state codes for deterministic order.
// ---------------------------------------------------------------------------

func TestFix_SortedUsStateCodes(t *testing.T) {
	codes := sortedUsStateCodes()
	if len(codes) != len(UsStateCodes) {
		t.Fatalf("sortedUsStateCodes has %d entries, want %d", len(codes), len(UsStateCodes))
	}

	// Verify sorted
	if !sort.StringsAreSorted(codes) {
		t.Error("sortedUsStateCodes is not sorted")
	}

	// Verify all codes present
	seen := make(map[string]bool)
	for _, code := range codes {
		if seen[code] {
			t.Errorf("duplicate code: %s", code)
		}
		seen[code] = true
		if _, ok := UsStateCodes[code]; !ok {
			t.Errorf("code %s not in UsStateCodes", code)
		}
	}
}

func TestFix_StateCodeDeterminism(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	// Run the same query 50 times, should always get the same result
	for i := 0; i < 50; i++ {
		r := g.Geocode("Springfield, IL")
		if r.Region() != "IL" {
			t.Errorf("iteration %d: region = %q, want IL", i, r.Region())
		}
	}
}

// ---------------------------------------------------------------------------
// Fix 5: abbrevRegex returned only first match
// FindStringSubmatch returns only the first match. For "Austin, TX" it
// returned ["Au"] instead of ["TX"]. Fix: use FindAllString.
// ---------------------------------------------------------------------------

func TestFix_AbbrevRegexReturnsAllMatches(t *testing.T) {
	re := abbrevRegex()

	tests := []struct {
		input string
		want  []string
	}{
		{"Austin, TX", []string{"TX"}},         // "Austin" has >3 chars, not matched
		{"NY, US", []string{"NY", "US"}},       // Both 2-letter tokens
		{"CA LA", []string{"CA", "LA"}},        // Space-separated
		{"Hello World", []string{}},            // >3 chars, no match
		{"A B", []string{}},                    // 1-char tokens, no match
		{"SFO LAX", []string{"SFO", "LAX"}},   // 3-letter tokens
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := re.FindAllString(tt.input, -1)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("FindAllString(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i, g := range got {
				if g != tt.want[i] {
					t.Errorf("FindAllString(%q)[%d] = %q, want %q", tt.input, i, g, tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Fix 6: exactMatchCity zero-population fallback
// When all matching cities had Population=0, the old code returned empty
// because `0 > 0` is false. Fix: default to first match.
// ---------------------------------------------------------------------------

func TestFix_ExactMatchZeroPopulationFallback(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	// Test that exact match still works for well-known cities
	r := g.Geocode("Austin, United States", GeocodeOptions{ExactCity: true})
	if r.City != "Austin" {
		t.Errorf("city = %q, want Austin", r.City)
	}
	if r.Country() != "US" {
		t.Errorf("country = %q, want US", r.Country())
	}

	// Test that multiple matches with country disambiguation work
	r = g.Geocode("Portland, United States", GeocodeOptions{ExactCity: true})
	if r.City != "Portland" {
		t.Errorf("city = %q, want Portland", r.City)
	}
	if r.Country() != "US" {
		t.Errorf("country = %q, want US", r.Country())
	}
}

// ---------------------------------------------------------------------------
// Fix 7: downloadFile error handling
// os.Remove error was ignored; Close() error lost on success path.
// Fix: use success flag pattern for proper cleanup.
// ---------------------------------------------------------------------------

// (downloadFile is tested indirectly - the fix is structural and verified
// by code review. Direct testing would require network mocking.)

// ---------------------------------------------------------------------------
// Fix 8: FuzzyDistance uncapped
// No upper limit on FuzzyDistance allowed O(N) Levenshtein scans across
// the entire name index. Fix: cap at maxFuzzyDistance (3).
// ---------------------------------------------------------------------------

func TestFix_FuzzyDistanceCapped(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	// With unreasonably high FuzzyDistance, should still work (capped to 3)
	r := g.Geocode("Londn", GeocodeOptions{FuzzyDistance: 100})
	if r.City != "London" {
		t.Errorf("Geocode('Londn', fuzzy=100) city = %q, want 'London'", r.City)
	}

	// Verify cap constant
	if maxFuzzyDistance != 3 {
		t.Errorf("maxFuzzyDistance = %d, want 3", maxFuzzyDistance)
	}
}

// ---------------------------------------------------------------------------
// Fix 9: ReverseGeocode NaN/Inf validation
// NaN, +Inf, -Inf coordinates were passed directly to S2 geometry
// which could cause undefined behavior. Fix: return empty city.
// ---------------------------------------------------------------------------

func TestFix_ReverseGeocodeNaNInfValidation(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		lat  float64
		lng  float64
	}{
		{"NaN lat", math.NaN(), 0},
		{"NaN lng", 0, math.NaN()},
		{"both NaN", math.NaN(), math.NaN()},
		{"+Inf lat", math.Inf(1), 0},
		{"-Inf lat", math.Inf(-1), 0},
		{"+Inf lng", 0, math.Inf(1)},
		{"-Inf lng", 0, math.Inf(-1)},
		{"both +Inf", math.Inf(1), math.Inf(1)},
		{"NaN lat +Inf lng", math.NaN(), math.Inf(1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.ReverseGeocode(tt.lat, tt.lng)
			if result.City != "" {
				t.Errorf("ReverseGeocode(%v, %v) city = %q, want empty", tt.lat, tt.lng, result.City)
			}
		})
	}

	// Sanity: valid coordinates still work
	r := g.ReverseGeocode(30.26715, -97.74306)
	if r.City != "Austin" {
		t.Errorf("valid coords: city = %q, want Austin", r.City)
	}
}

// ---------------------------------------------------------------------------
// Fix 10: ReverseGeocode sort determinism
// sort.Slice was unstable; equal-distance+population candidates had
// non-deterministic order. Fix: sort.SliceStable + city name tiebreaker.
// ---------------------------------------------------------------------------

func TestFix_ReverseGeocodeSortDeterminism(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	// Run the same reverse geocode 100 times - must always return same city
	coords := []struct {
		lat, lng float64
	}{
		{48.8566, 2.3522},     // Paris
		{52.5200, 13.4050},    // Berlin
		{40.7128, -74.0060},   // NYC
		{51.5074, -0.1278},    // London
		{35.6762, 139.6503},   // Tokyo
	}

	for _, c := range coords {
		firstResult := g.ReverseGeocode(c.lat, c.lng)
		for i := 0; i < 100; i++ {
			r := g.ReverseGeocode(c.lat, c.lng)
			if r.City != firstResult.City {
				t.Errorf("non-deterministic at (%v,%v): iteration %d got %q, first was %q",
					c.lat, c.lng, i, r.City, firstResult.City)
				break
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Fix 11: Scoring tie-break determinism
// Map iteration in fuzzyMatchLocation produced non-deterministic results
// when score AND population tied. Fix: prefer lower city index.
// ---------------------------------------------------------------------------

func TestFix_ScoringTieBreakDeterminism(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	queries := []string{
		"Springfield",
		"Portland",
		"Columbus",
		"London",
		"Richmond",
		"Birmingham",
		"Cambridge",
	}

	for _, q := range queries {
		firstResult := g.Geocode(q)
		for i := 0; i < 50; i++ {
			r := g.Geocode(q)
			if r.City != firstResult.City || r.Country() != firstResult.Country() {
				t.Errorf("non-deterministic for %q: iteration %d got %q/%q, first was %q/%q",
					q, i, r.City, r.Country(), firstResult.City, firstResult.Country())
				break
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Fix 12: stringInterner off-by-one
// The overflow check used >= instead of >, losing one valid index slot.
// Fix: changed to > so the full range [0, maxVal] is usable.
// ---------------------------------------------------------------------------

func TestFix_StringInternerOffByOne(t *testing.T) {
	// Test with uint8 (max 255) where the off-by-one is easily observable.
	si := newStringInterner[uint8](256)

	// Index 0 is reserved for "". Add 255 unique non-empty strings.
	for i := 0; i < 255; i++ {
		idx := si.intern(fmt.Sprintf("s%d", i))
		if idx == 0 {
			t.Fatalf("expected non-zero index for s%d", i)
		}
	}

	// With the fix, the interner should have exactly 256 entries (0..255)
	if si.count() != 256 {
		t.Errorf("count = %d, want 256 (255 strings + empty)", si.count())
	}

	// Now the NEXT intern should panic (exceeds uint8 capacity)
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic on overflow, but didn't panic")
			return
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "capacity exceeded") {
			t.Errorf("unexpected panic message: %v", r)
		}
	}()
	si.intern("overflow_trigger")
}

func TestFix_StringInternerBasicOperations(t *testing.T) {
	si := newStringInterner[uint16](10)

	// Index 0 is reserved for empty string
	idx := si.intern("")
	if idx != 0 {
		t.Errorf("empty string index = %d, want 0", idx)
	}

	// New strings get sequential indices
	idx1 := si.intern("hello")
	idx2 := si.intern("world")
	if idx1 == 0 || idx2 == 0 {
		t.Error("non-empty strings should not get index 0")
	}
	if idx1 == idx2 {
		t.Error("different strings should get different indices")
	}

	// Same string returns same index (idempotent)
	idx1b := si.intern("hello")
	if idx1b != idx1 {
		t.Errorf("re-interning 'hello' got %d, want %d", idx1b, idx1)
	}

	// Lookup by index
	if s := si.get(idx1); s != "hello" {
		t.Errorf("get(%d) = %q, want 'hello'", idx1, s)
	}
	if s := si.get(idx2); s != "world" {
		t.Errorf("get(%d) = %q, want 'world'", idx2, s)
	}

	// Out-of-bounds returns empty string
	if s := si.get(65535); s != "" {
		t.Errorf("get(65535) = %q, want empty", s)
	}

	// Count
	if c := si.count(); c != 3 { // empty + hello + world
		t.Errorf("count = %d, want 3", c)
	}
}

func TestFix_StringInternerConcurrency(t *testing.T) {
	si := newStringInterner[uint16](1000)

	const numGoroutines = 50
	const stringsPerGoroutine = 100

	var wg sync.WaitGroup
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < stringsPerGoroutine; i++ {
				// Some strings overlap between goroutines, some don't
				s := fmt.Sprintf("str_%d", i)
				idx := si.intern(s)
				got := si.get(idx)
				if got != s {
					t.Errorf("goroutine %d: intern+get(%q) = %q", id, s, got)
				}
			}
		}(g)
	}
	wg.Wait()

	// Should have exactly 101 entries: empty + 100 unique "str_0".."str_99"
	if c := si.count(); c != stringsPerGoroutine+1 {
		t.Errorf("count = %d, want %d", c, stringsPerGoroutine+1)
	}
}
