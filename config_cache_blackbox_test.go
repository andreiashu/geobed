package geobed

import (
	"sync"
	"testing"
)

// TestNewGeobed_Basic verifies that NewGeobed returns a valid GeoBed instance
// with properly populated data structures.
func TestNewGeobed_Basic(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("NewGeobed() error = %v, want nil", err)
	}
	if g == nil {
		t.Fatal("NewGeobed() returned nil GeoBed")
	}

	// Verify Cities is populated
	if g.Cities == nil {
		t.Error("Cities is nil")
	}
	if len(g.Cities) == 0 {
		t.Error("Cities is empty")
	}
	if len(g.Cities) < minCityCount {
		t.Errorf("Cities count = %d, want >= %d", len(g.Cities), minCityCount)
	}

	// Verify Countries is populated
	if g.Countries == nil {
		t.Error("Countries is nil")
	}
	if len(g.Countries) == 0 {
		t.Error("Countries is empty")
	}
	if len(g.Countries) < minCountryCount {
		t.Errorf("Countries count = %d, want >= %d", len(g.Countries), minCountryCount)
	}
}

// TestNewGeobed_WithDataDir verifies that WithDataDir option can be passed
// without causing errors, even with nonexistent directories (embedded cache should be used).
func TestNewGeobed_WithDataDir(t *testing.T) {
	tests := []struct {
		name    string
		dataDir string
	}{
		{
			name:    "nonexistent directory",
			dataDir: "/nonexistent",
		},
		{
			name:    "empty string",
			dataDir: "",
		},
		{
			name:    "invalid path",
			dataDir: "/tmp/does/not/exist/geobed-data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := NewGeobed(WithDataDir(tt.dataDir))
			if err != nil {
				t.Fatalf("NewGeobed(WithDataDir(%q)) error = %v, want nil (should use embedded cache)", tt.dataDir, err)
			}
			if g == nil {
				t.Fatal("NewGeobed() returned nil GeoBed")
			}
			if len(g.Cities) < minCityCount {
				t.Errorf("Cities count = %d, want >= %d", len(g.Cities), minCityCount)
			}
		})
	}
}

// TestNewGeobed_WithCacheDir verifies that WithCacheDir option can be passed
// and that the embedded cache is used when the cache directory is empty.
func TestNewGeobed_WithCacheDir(t *testing.T) {
	// Use a temporary directory that won't have cache files
	tempDir := t.TempDir()

	g, err := NewGeobed(WithCacheDir(tempDir))
	if err != nil {
		t.Fatalf("NewGeobed(WithCacheDir(%q)) error = %v, want nil (should use embedded cache)", tempDir, err)
	}
	if g == nil {
		t.Fatal("NewGeobed() returned nil GeoBed")
	}
	if len(g.Cities) < minCityCount {
		t.Errorf("Cities count = %d, want >= %d", len(g.Cities), minCityCount)
	}
	if len(g.Countries) < minCountryCount {
		t.Errorf("Countries count = %d, want >= %d", len(g.Countries), minCountryCount)
	}
}

// TestNewGeobed_MultipleOptions verifies that multiple options can be combined
// without causing panics or errors.
func TestNewGeobed_MultipleOptions(t *testing.T) {
	tests := []struct {
		name    string
		options []Option
	}{
		{
			name:    "both data and cache dirs",
			options: []Option{WithDataDir("x"), WithCacheDir("y")},
		},
		{
			name:    "cache dir twice",
			options: []Option{WithCacheDir("a"), WithCacheDir("b")},
		},
		{
			name:    "data dir twice",
			options: []Option{WithDataDir("a"), WithDataDir("b")},
		},
		{
			name:    "all options",
			options: []Option{WithDataDir("/tmp/x"), WithCacheDir("/tmp/y")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("NewGeobed with multiple options panicked: %v", r)
				}
			}()

			g, err := NewGeobed(tt.options...)
			if err != nil {
				t.Fatalf("NewGeobed() error = %v, want nil", err)
			}
			if g == nil {
				t.Fatal("NewGeobed() returned nil GeoBed")
			}
		})
	}
}

// TestGetDefaultGeobed_Singleton verifies that GetDefaultGeobed returns the same
// instance on multiple calls and that the instance is functional.
func TestGetDefaultGeobed_Singleton(t *testing.T) {
	g1, err := GetDefaultGeobed()
	if err != nil {
		t.Fatalf("GetDefaultGeobed() first call error = %v", err)
	}
	if g1 == nil {
		t.Fatal("GetDefaultGeobed() first call returned nil")
	}

	g2, err := GetDefaultGeobed()
	if err != nil {
		t.Fatalf("GetDefaultGeobed() second call error = %v", err)
	}
	if g2 == nil {
		t.Fatal("GetDefaultGeobed() second call returned nil")
	}

	// Verify same instance (pointer equality)
	if g1 != g2 {
		t.Error("GetDefaultGeobed() returned different instances, want same pointer")
	}

	// Verify instance is functional by performing a basic geocode
	if len(g1.Cities) == 0 {
		t.Error("Default GeoBed has no cities")
	}

	// Try a basic geocode operation to ensure it works
	result := g1.Geocode("London")
	if result.City == "" {
		t.Error("Geocode on default GeoBed returned empty result")
	}
}

// TestGetDefaultGeobed_Concurrent verifies that GetDefaultGeobed is thread-safe
// and that all concurrent calls receive the same instance.
func TestGetDefaultGeobed_Concurrent(t *testing.T) {
	const numGoroutines = 20

	var wg sync.WaitGroup
	results := make([]*GeoBed, numGoroutines)
	errors := make([]error, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx], errors[idx] = GetDefaultGeobed()
		}(i)
	}
	wg.Wait()

	// Verify no errors
	for i, err := range errors {
		if err != nil {
			t.Errorf("Goroutine %d got error: %v", i, err)
		}
	}

	// Verify all received the same instance
	firstInstance := results[0]
	if firstInstance == nil {
		t.Fatal("First instance is nil")
	}

	for i, instance := range results {
		if instance == nil {
			t.Errorf("Goroutine %d got nil instance", i)
			continue
		}
		if instance != firstInstance {
			t.Errorf("Goroutine %d got different instance (pointer mismatch)", i)
		}
	}
}

// TestValidateCache verifies that ValidateCache returns nil when the cache is valid.
func TestValidateCache(t *testing.T) {
	err := ValidateCache()
	if err != nil {
		t.Errorf("ValidateCache() error = %v, want nil", err)
	}
}

// TestCountryCount_AfterInitialization verifies that CountryCount returns
// a positive value after GeoBed initialization.
func TestCountryCount_AfterInitialization(t *testing.T) {
	_, err := NewGeobed()
	if err != nil {
		t.Fatalf("NewGeobed() error = %v", err)
	}

	count := CountryCount()
	if count <= 0 {
		t.Errorf("CountryCount() = %d, want > 0", count)
	}
	if count < minCountryCount {
		t.Errorf("CountryCount() = %d, want >= %d", count, minCountryCount)
	}
}

// TestRegionCount_AfterInitialization verifies that RegionCount returns
// a positive value after GeoBed initialization.
func TestRegionCount_AfterInitialization(t *testing.T) {
	_, err := NewGeobed()
	if err != nil {
		t.Fatalf("NewGeobed() error = %v", err)
	}

	count := RegionCount()
	if count <= 0 {
		t.Errorf("RegionCount() = %d, want > 0", count)
	}
}

// TestDataIntegrity_CityFields verifies that cities have valid field values.
func TestDataIntegrity_CityFields(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("NewGeobed() error = %v", err)
	}

	// Sample first 1000 cities to verify data integrity without being too slow
	sampleSize := 1000
	if len(g.Cities) < sampleSize {
		sampleSize = len(g.Cities)
	}

	for i := 0; i < sampleSize; i++ {
		city := g.Cities[i]

		// City name should not be empty
		if city.City == "" {
			t.Errorf("City[%d] has empty name", i)
		}

		// Latitude should be in valid range [-90, 90]
		if city.Latitude < -90 || city.Latitude > 90 {
			t.Errorf("City[%d] (%s) has invalid latitude: %f (want -90 to 90)", i, city.City, city.Latitude)
		}

		// Longitude should be in valid range [-180, 180]
		if city.Longitude < -180 || city.Longitude > 180 {
			t.Errorf("City[%d] (%s) has invalid longitude: %f (want -180 to 180)", i, city.City, city.Longitude)
		}

		// Country code should be 2 characters (ISO 3166-1 alpha-2)
		countryCode := city.Country()
		if len(countryCode) != 2 {
			t.Errorf("City[%d] (%s) has invalid country code length: %q (want 2 chars)", i, city.City, countryCode)
		}
	}
}

// TestDataIntegrity_CountryFields verifies that countries have valid field values.
func TestDataIntegrity_CountryFields(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("NewGeobed() error = %v", err)
	}

	for i, country := range g.Countries {
		// Country name should not be empty
		if country.Country == "" {
			t.Errorf("Country[%d] has empty name", i)
		}

		// Capital can be empty for some territories, so we won't enforce it

		// GeonameId should be positive
		if country.GeonameId <= 0 {
			t.Errorf("Country[%d] (%s) has invalid GeonameId: %d (want > 0)", i, country.Country, country.GeonameId)
		}
	}
}

// TestCacheConsistency_MultipleCalls verifies that multiple NewGeobed calls
// produce consistent results from the embedded cache.
func TestCacheConsistency_MultipleCalls(t *testing.T) {
	g1, err := NewGeobed()
	if err != nil {
		t.Fatalf("First NewGeobed() error = %v", err)
	}

	g2, err := NewGeobed()
	if err != nil {
		t.Fatalf("Second NewGeobed() error = %v", err)
	}

	// Verify same city count
	if len(g1.Cities) != len(g2.Cities) {
		t.Errorf("City count mismatch: first=%d, second=%d", len(g1.Cities), len(g2.Cities))
	}

	// Verify same country count
	if len(g1.Countries) != len(g2.Countries) {
		t.Errorf("Country count mismatch: first=%d, second=%d", len(g1.Countries), len(g2.Countries))
	}

	// Sample a few cities to verify data consistency
	sampleIndices := []int{0, 100, 1000, 10000}
	for _, idx := range sampleIndices {
		if idx >= len(g1.Cities) || idx >= len(g2.Cities) {
			continue
		}

		c1 := g1.Cities[idx]
		c2 := g2.Cities[idx]

		if c1.City != c2.City {
			t.Errorf("City[%d] name mismatch: %q != %q", idx, c1.City, c2.City)
		}
		if c1.Latitude != c2.Latitude {
			t.Errorf("City[%d] (%s) latitude mismatch: %f != %f", idx, c1.City, c1.Latitude, c2.Latitude)
		}
		if c1.Longitude != c2.Longitude {
			t.Errorf("City[%d] (%s) longitude mismatch: %f != %f", idx, c1.City, c1.Longitude, c2.Longitude)
		}
		if c1.Country() != c2.Country() {
			t.Errorf("City[%d] (%s) country mismatch: %q != %q", idx, c1.City, c1.Country(), c2.Country())
		}
	}
}

// TestNewGeobed_OptionsDoNotPanic verifies that various option combinations
// don't cause panics even with unusual inputs.
func TestNewGeobed_OptionsDoNotPanic(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
	}{
		{"no options", nil},
		{"empty data dir", []Option{WithDataDir("")}},
		{"empty cache dir", []Option{WithCacheDir("")}},
		{"both empty", []Option{WithDataDir(""), WithCacheDir("")}},
		{"special chars in path", []Option{WithDataDir("/tmp/!@#$%")}},
		{"very long path", []Option{WithDataDir("/tmp/" + string(make([]byte, 1000)))}},
		{"nil slice", []Option{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("NewGeobed panicked with options %v: %v", tt.name, r)
				}
			}()

			// We allow errors, but not panics
			_, _ = NewGeobed(tt.opts...)
		})
	}
}

// TestGeoBed_CitiesSorted verifies that cities are sorted by name
// (as documented in the GeoBed struct comment).
// Note: The sorting may use special collation rules for international characters,
// so we just verify that some basic ordering exists rather than strict lexicographic ordering.
func TestGeoBed_CitiesSorted(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("NewGeobed() error = %v", err)
	}

	// Verify that cities starting with 'A' come before cities starting with 'Z'
	// This is a basic sanity check that some ordering exists
	firstA := -1
	lastA := -1
	firstZ := -1

	for i, city := range g.Cities {
		if len(city.City) > 0 {
			firstChar := city.City[0]
			if firstChar == 'A' || firstChar == 'a' {
				if firstA == -1 {
					firstA = i
				}
				lastA = i
			}
			if firstChar == 'Z' || firstChar == 'z' {
				if firstZ == -1 {
					firstZ = i
				}
			}
		}
	}

	// Basic ordering check: 'A' cities should come before 'Z' cities
	if firstA != -1 && firstZ != -1 && lastA > firstZ {
		t.Errorf("Cities not properly ordered: last 'A' city at index %d comes after first 'Z' city at index %d", lastA, firstZ)
	}
}

// TestCountryCount_RegionCount_Concurrent verifies that CountryCount and RegionCount
// are safe to call concurrently.
func TestCountryCount_RegionCount_Concurrent(t *testing.T) {
	_, err := NewGeobed()
	if err != nil {
		t.Fatalf("NewGeobed() error = %v", err)
	}

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// Test concurrent CountryCount calls
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			count := CountryCount()
			if count <= 0 {
				t.Errorf("CountryCount() returned %d, want > 0", count)
			}
		}()
	}

	// Test concurrent RegionCount calls
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			count := RegionCount()
			if count <= 0 {
				t.Errorf("RegionCount() returned %d, want > 0", count)
			}
		}()
	}

	wg.Wait()
}
