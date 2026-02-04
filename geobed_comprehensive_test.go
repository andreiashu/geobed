package geobed

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// ============================================================================
// Edge Case Tests
// ============================================================================

// TestGeocodeEdgeCases tests edge case inputs for the Geocode function.
func TestGeocodeEdgeCases(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name        string
		input       string
		expectEmpty bool
	}{
		{
			name:        "empty string",
			input:       "",
			expectEmpty: true,
		},
		{
			name:        "whitespace only - spaces",
			input:       "   ",
			expectEmpty: true,
		},
		{
			name:        "whitespace only - tabs",
			input:       "\t\t",
			expectEmpty: true,
		},
		{
			name:        "whitespace only - mixed",
			input:       " \t \n ",
			expectEmpty: true,
		},
		{
			name:        "numeric string - zip code",
			input:       "12345",
			expectEmpty: false, // May match something or return empty
		},
		{
			name:        "numeric string - long number",
			input:       "1234567890",
			expectEmpty: false,
		},
		{
			name:        "special characters only",
			input:       "!@#$%^&*()",
			expectEmpty: false, // Let the function handle it
		},
		{
			name:        "very long string",
			input:       strings.Repeat("a", 1000),
			expectEmpty: false, // Should not panic
		},
		{
			name:        "string with newlines",
			input:       "New\nYork",
			expectEmpty: false,
		},
		{
			name:        "single character",
			input:       "A",
			expectEmpty: false,
		},
		{
			name:        "two characters",
			input:       "NY",
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			result := g.Geocode(tt.input)

			if tt.expectEmpty {
				if result.City != "" {
					t.Errorf("Geocode(%q) expected empty City, got %q", tt.input, result.City)
				}
			}
			// For non-empty expected results, we just verify no panic occurred
		})
	}
}

// ============================================================================
// Unicode/International Tests
// ============================================================================

// TestGeocodeUnicodeInternational tests unicode and international city names.
func TestGeocodeUnicodeInternational(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name        string
		input       string
		wantCity    string
		wantCountry string
	}{
		{
			name:        "Munich - English name",
			input:       "Munich",
			wantCity:    "Munich",
			wantCountry: "DE",
		},
		{
			name:        "Sao Paulo",
			input:       "Sao Paulo",
			wantCity:    "Sao Paulo",
			wantCountry: "BR",
		},
		{
			name:        "Beijing",
			input:       "Beijing",
			wantCity:    "Beijing",
			wantCountry: "CN",
		},
		{
			name:        "Moscow",
			input:       "Moscow",
			wantCity:    "Moscow",
			wantCountry: "RU",
		},
		{
			name:        "Vienna",
			input:       "Vienna",
			wantCity:    "Vienna",
			wantCountry: "AT",
		},
		{
			name:        "Prague",
			input:       "Prague",
			wantCity:    "Prague",
			wantCountry: "CZ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.Geocode(tt.input)

			// For cities with alternate names, check if the result matches
			if result.City == "" {
				t.Errorf("Geocode(%q) returned empty city, expected %q", tt.input, tt.wantCity)
				return
			}

			// Check country matches
			if tt.wantCountry != "" && result.Country() != tt.wantCountry {
				t.Errorf("Geocode(%q) country = %q, want %q", tt.input, result.Country(), tt.wantCountry)
			}
		})
	}
}

// TestGeocodeJapaneseChineseNames tests Asian city names.
func TestGeocodeJapaneseChineseNames(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name        string
		input       string
		wantCountry string
	}{
		{
			name:        "Tokyo romanized",
			input:       "Tokyo",
			wantCountry: "JP",
		},
		{
			name:        "Osaka romanized",
			input:       "Osaka",
			wantCountry: "JP",
		},
		{
			name:        "Shanghai romanized",
			input:       "Shanghai",
			wantCountry: "CN",
		},
		{
			name:        "Seoul romanized",
			input:       "Seoul",
			wantCountry: "KR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.Geocode(tt.input)

			if result.City == "" {
				t.Errorf("Geocode(%q) returned empty city", tt.input)
				return
			}

			if result.Country() != tt.wantCountry {
				t.Errorf("Geocode(%q) country = %q, want %q", tt.input, result.Country(), tt.wantCountry)
			}
		})
	}
}

// ============================================================================
// Ambiguous Names Tests
// ============================================================================

// TestGeocodeAmbiguousNames tests disambiguation of cities with the same name.
func TestGeocodeAmbiguousNames(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name        string
		input       string
		wantCountry string
		wantRegion  string
	}{
		{
			name:        "Paris France - explicit",
			input:       "Paris, France",
			wantCountry: "FR",
		},
		{
			name:        "Paris Texas - with state abbreviation",
			input:       "Paris, TX",
			wantCountry: "US",
			wantRegion:  "TX",
		},
		{
			name:        "London UK",
			input:       "London, UK",
			wantCountry: "GB",
		},
		{
			name:        "London Ohio",
			input:       "London, OH",
			wantCountry: "US",
			wantRegion:  "OH",
		},
		{
			name:        "Springfield IL",
			input:       "Springfield, IL",
			wantCountry: "US",
			wantRegion:  "IL",
		},
		{
			name:        "Springfield MA",
			input:       "Springfield, MA",
			wantCountry: "US",
			wantRegion:  "MA",
		},
		{
			name:        "Portland Oregon",
			input:       "Portland, OR",
			wantCountry: "US",
			wantRegion:  "OR",
		},
		{
			name:        "Portland Maine",
			input:       "Portland, ME",
			wantCountry: "US",
			wantRegion:  "ME",
		},
		{
			name:        "Columbus Ohio",
			input:       "Columbus, OH",
			wantCountry: "US",
			wantRegion:  "OH",
		},
		{
			name:        "Columbus Georgia",
			input:       "Columbus, GA",
			wantCountry: "US",
			wantRegion:  "GA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.Geocode(tt.input)

			if result.City == "" {
				t.Errorf("Geocode(%q) returned empty city", tt.input)
				return
			}

			if tt.wantCountry != "" && result.Country() != tt.wantCountry {
				t.Errorf("Geocode(%q) country = %q, want %q", tt.input, result.Country(), tt.wantCountry)
			}

			if tt.wantRegion != "" && result.Region() != tt.wantRegion {
				t.Errorf("Geocode(%q) region = %q, want %q", tt.input, result.Region(), tt.wantRegion)
			}
		})
	}
}

// ============================================================================
// Case Insensitivity Tests
// ============================================================================

// TestGeocodeCaseInsensitivity tests that geocoding is case-insensitive.
func TestGeocodeCaseInsensitivity(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name   string
		inputs []string
	}{
		{
			name:   "London variations",
			inputs: []string{"London", "LONDON", "london", "LoNdOn", "lONDON"},
		},
		{
			name:   "New York variations",
			inputs: []string{"New York", "NEW YORK", "new york", "New york", "nEW yORK"},
		},
		{
			name:   "Paris variations",
			inputs: []string{"Paris", "PARIS", "paris", "PaRiS"},
		},
		{
			name:   "Tokyo variations",
			inputs: []string{"Tokyo", "TOKYO", "tokyo", "ToKyO"},
		},
		{
			name:   "Sydney variations",
			inputs: []string{"Sydney", "SYDNEY", "sydney", "SyDnEy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get result for first input as reference
			reference := g.Geocode(tt.inputs[0])
			if reference.City == "" {
				t.Fatalf("Geocode(%q) returned empty city", tt.inputs[0])
			}

			// All variations should return the same city
			for _, input := range tt.inputs[1:] {
				result := g.Geocode(input)
				if result.City != reference.City {
					t.Errorf("Geocode(%q) city = %q, want %q (case insensitivity failed)", input, result.City, reference.City)
				}
				if result.Country() != reference.Country() {
					t.Errorf("Geocode(%q) country = %q, want %q (case insensitivity failed)", input, result.Country(), reference.Country())
				}
			}
		})
	}
}

// ============================================================================
// Concurrency Tests
// ============================================================================

// TestGeocodeConcurrency tests that Geocode is safe for concurrent use.
func TestGeocodeConcurrency(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	queries := []string{
		"New York", "Los Angeles", "Chicago", "Houston", "Phoenix",
		"Philadelphia", "San Antonio", "San Diego", "Dallas", "San Jose",
		"Austin", "Jacksonville", "Fort Worth", "Columbus", "Charlotte",
		"San Francisco", "Indianapolis", "Seattle", "Denver", "Boston",
		"London", "Paris", "Tokyo", "Berlin", "Sydney",
		"Moscow", "Beijing", "Seoul", "Mumbai", "Cairo",
	}

	const numGoroutines = 100
	var wg sync.WaitGroup
	panicChan := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicChan <- fmt.Sprintf("goroutine %d panicked: %v", id, r)
				}
			}()

			// Each goroutine performs multiple geocode operations
			for j := 0; j < 10; j++ {
				query := queries[(id+j)%len(queries)]
				result := g.Geocode(query)
				// Verify we got a result (not checking correctness, just no crash)
				if result.City == "" && query != "" {
					// Some queries might legitimately return empty, but most shouldn't
					continue
				}
			}
		}(i)
	}

	wg.Wait()
	close(panicChan)

	for msg := range panicChan {
		t.Errorf("Concurrency error: %s", msg)
	}
}

// TestReverseGeocodeConcurrency tests that ReverseGeocode is safe for concurrent use.
func TestReverseGeocodeConcurrency(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	coords := []struct {
		lat, lng float64
	}{
		{40.7128, -74.0060},   // New York
		{34.0522, -118.2437},  // Los Angeles
		{51.5074, -0.1278},    // London
		{48.8566, 2.3522},     // Paris
		{35.6762, 139.6503},   // Tokyo
		{-33.8688, 151.2093},  // Sydney
		{55.7558, 37.6173},    // Moscow
		{39.9042, 116.4074},   // Beijing
		{37.5665, 126.9780},   // Seoul
		{19.0760, 72.8777},    // Mumbai
	}

	const numGoroutines = 100
	var wg sync.WaitGroup
	panicChan := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicChan <- fmt.Sprintf("goroutine %d panicked: %v", id, r)
				}
			}()

			for j := 0; j < 10; j++ {
				coord := coords[(id+j)%len(coords)]
				result := g.ReverseGeocode(coord.lat, coord.lng)
				// Just verify no crash
				_ = result.City
				_ = result.Country()
			}
		}(i)
	}

	wg.Wait()
	close(panicChan)

	for msg := range panicChan {
		t.Errorf("Concurrency error: %s", msg)
	}
}

// TestMixedConcurrency tests both Geocode and ReverseGeocode running concurrently.
func TestMixedConcurrency(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	queries := []string{"New York", "London", "Paris", "Tokyo", "Sydney"}
	coords := []struct {
		lat, lng float64
	}{
		{40.7128, -74.0060},
		{51.5074, -0.1278},
		{48.8566, 2.3522},
		{35.6762, 139.6503},
		{-33.8688, 151.2093},
	}

	const numGoroutines = 200
	var wg sync.WaitGroup
	panicChan := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicChan <- fmt.Sprintf("goroutine %d panicked: %v", id, r)
				}
			}()

			for j := 0; j < 20; j++ {
				if j%2 == 0 {
					query := queries[(id+j)%len(queries)]
					result := g.Geocode(query)
					_ = result.City
				} else {
					coord := coords[(id+j)%len(coords)]
					result := g.ReverseGeocode(coord.lat, coord.lng)
					_ = result.City
				}
			}
		}(i)
	}

	wg.Wait()
	close(panicChan)

	for msg := range panicChan {
		t.Errorf("Concurrency error: %s", msg)
	}
}

// ============================================================================
// Reverse Geocoding Edge Cases
// ============================================================================

// TestReverseGeocodeEdgeCases tests edge cases for reverse geocoding.
func TestReverseGeocodeEdgeCases(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name string
		lat  float64
		lng  float64
	}{
		{
			name: "origin point (0,0) - Gulf of Guinea",
			lat:  0.0,
			lng:  0.0,
		},
		{
			name: "north pole",
			lat:  90.0,
			lng:  0.0,
		},
		{
			name: "south pole",
			lat:  -90.0,
			lng:  0.0,
		},
		{
			name: "date line positive",
			lat:  0.0,
			lng:  180.0,
		},
		{
			name: "date line negative",
			lat:  0.0,
			lng:  -180.0,
		},
		{
			name: "middle of Pacific Ocean",
			lat:  0.0,
			lng:  -160.0,
		},
		{
			name: "middle of Atlantic Ocean",
			lat:  30.0,
			lng:  -40.0,
		},
		{
			name: "extreme coordinates",
			lat:  -90.0,
			lng:  180.0,
		},
		{
			name: "very precise coordinates - Austin TX",
			lat:  30.267153,
			lng:  -97.743057,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			result := g.ReverseGeocode(tt.lat, tt.lng)

			// We don't strictly check for empty results since behavior may vary
			// The main goal is to ensure no panics occur
			_ = result.City
			_ = result.Country()
			_ = result.Region()
		})
	}
}

// TestReverseGeocodeKnownLocations tests reverse geocoding of well-known locations.
func TestReverseGeocodeKnownLocations(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name        string
		lat         float64
		lng         float64
		wantCity    string
		wantCountry string
	}{
		{
			name:        "Austin TX",
			lat:         30.26715,
			lng:         -97.74306,
			wantCity:    "Austin",
			wantCountry: "US",
		},
		{
			name:        "Palo Alto CA",
			lat:         37.44651,
			lng:         -122.15322,
			wantCity:    "Palo Alto",
			wantCountry: "US",
		},
		{
			name:        "Santa Cruz CA",
			lat:         36.9741,
			lng:         -122.0308,
			wantCity:    "Santa Cruz",
			wantCountry: "US",
		},
		{
			name:        "Sydney Australia",
			lat:         -33.8688,
			lng:         151.2093,
			wantCity:    "Sydney",
			wantCountry: "AU",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.ReverseGeocode(tt.lat, tt.lng)

			if result.City != tt.wantCity {
				t.Errorf("ReverseGeocode(%v, %v) city = %q, want %q", tt.lat, tt.lng, result.City, tt.wantCity)
			}
			if result.Country() != tt.wantCountry {
				t.Errorf("ReverseGeocode(%v, %v) country = %q, want %q", tt.lat, tt.lng, result.Country(), tt.wantCountry)
			}
		})
	}
}

// ============================================================================
// Additional Tests
// ============================================================================

// TestGeocodeWithOptions tests the GeocodeOptions functionality.
func TestGeocodeWithOptions(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name        string
		input       string
		exactCity   bool
		wantCity    string
		wantEmpty   bool // Some exact match queries may return empty
	}{
		{
			name:      "fuzzy match - Austin",
			input:     "Austin",
			exactCity: false,
			wantCity:  "Austin",
		},
		{
			name:      "exact match with state - Austin TX",
			input:     "Austin, TX",
			exactCity: true,
			wantCity:  "Austin",
		},
		{
			name:      "fuzzy match - New York NY",
			input:     "New York, NY",
			exactCity: false,
			wantCity:  "New York City",
		},
		{
			name:      "exact match - New York City NY (full name)",
			input:     "New York City, NY",
			exactCity: true,
			wantCity:  "New York City",
		},
		{
			name:      "fuzzy match - Paris France",
			input:     "Paris, France",
			exactCity: false,
			wantCity:  "Paris",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := GeocodeOptions{ExactCity: tt.exactCity}
			result := g.Geocode(tt.input, opts)

			if tt.wantEmpty {
				if result.City != "" {
					t.Errorf("Geocode(%q, ExactCity=%v) expected empty city, got %q", tt.input, tt.exactCity, result.City)
				}
				return
			}

			if result.City != tt.wantCity {
				t.Errorf("Geocode(%q, ExactCity=%v) city = %q, want %q", tt.input, tt.exactCity, result.City, tt.wantCity)
			}
		})
	}
}

// TestGeocodeUSStates tests geocoding with various US state formats.
func TestGeocodeUSStates(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name       string
		input      string
		wantCity   string
		wantRegion string
	}{
		{
			name:       "city with state abbreviation",
			input:      "Austin, TX",
			wantCity:   "Austin",
			wantRegion: "TX",
		},
		{
			name:       "city with lowercase state",
			input:      "Austin, tx",
			wantCity:   "Austin",
			wantRegion: "TX",
		},
		{
			name:       "city with no comma",
			input:      "Austin TX",
			wantCity:   "Austin",
			wantRegion: "TX",
		},
		{
			name:       "city with state name",
			input:      "Houston, Texas",
			wantCity:   "Houston",
			wantRegion: "TX",
		},
		{
			name:       "California city",
			input:      "San Francisco, CA",
			wantCity:   "San Francisco",
			wantRegion: "CA",
		},
		{
			name:       "New York city",
			input:      "New York, NY",
			wantCity:   "New York City",
			wantRegion: "NY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.Geocode(tt.input)

			if result.City != tt.wantCity {
				t.Errorf("Geocode(%q) city = %q, want %q", tt.input, result.City, tt.wantCity)
			}
			if result.Region() != tt.wantRegion {
				t.Errorf("Geocode(%q) region = %q, want %q", tt.input, result.Region(), tt.wantRegion)
			}
		})
	}
}

// TestGeocodeCountries tests geocoding with explicit country indicators.
func TestGeocodeCountries(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name        string
		input       string
		wantCountry string
	}{
		{
			name:        "London UK",
			input:       "London, United Kingdom",
			wantCountry: "GB",
		},
		{
			name:        "Paris France",
			input:       "Paris, France",
			wantCountry: "FR",
		},
		{
			name:        "Berlin Germany",
			input:       "Berlin, Germany",
			wantCountry: "DE",
		},
		{
			name:        "Tokyo Japan",
			input:       "Tokyo, Japan",
			wantCountry: "JP",
		},
		{
			name:        "Sydney Australia",
			input:       "Sydney, Australia",
			wantCountry: "AU",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.Geocode(tt.input)

			if result.City == "" {
				t.Errorf("Geocode(%q) returned empty city", tt.input)
				return
			}

			if result.Country() != tt.wantCountry {
				t.Errorf("Geocode(%q) country = %q, want %q", tt.input, result.Country(), tt.wantCountry)
			}
		})
	}
}

// TestGetDefaultGeobedSingleton tests the singleton pattern for GetDefaultGeobed.
func TestGetDefaultGeobedSingleton(t *testing.T) {
	// First call should initialize
	g1, err1 := GetDefaultGeobed()
	if err1 != nil {
		t.Fatalf("GetDefaultGeobed() first call error: %v", err1)
	}

	// Second call should return the same instance
	g2, err2 := GetDefaultGeobed()
	if err2 != nil {
		t.Fatalf("GetDefaultGeobed() second call error: %v", err2)
	}

	// Should be the same pointer
	if g1 != g2 {
		t.Error("GetDefaultGeobed() should return the same instance on subsequent calls")
	}

	// Should be functional
	result := g1.Geocode("New York")
	if result.City == "" {
		t.Error("GetDefaultGeobed() returned non-functional instance")
	}
}

// TestGetDefaultGeobedConcurrency tests concurrent access to GetDefaultGeobed.
func TestGetDefaultGeobedConcurrency(t *testing.T) {
	const numGoroutines = 50
	var wg sync.WaitGroup
	results := make([]*GeoBed, numGoroutines)
	errors := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			g, err := GetDefaultGeobed()
			results[idx] = g
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// Check for errors
	for i, err := range errors {
		if err != nil {
			t.Errorf("GetDefaultGeobed() goroutine %d error: %v", i, err)
		}
	}

	// All results should be the same pointer
	for i := 1; i < numGoroutines; i++ {
		if results[i] != results[0] {
			t.Errorf("GetDefaultGeobed() goroutine %d returned different instance", i)
		}
	}
}

// TestGeobedCityMethods tests the GeobedCity accessor methods.
func TestGeobedCityMethods(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	result := g.Geocode("Austin, TX")

	// Test City field
	if result.City == "" {
		t.Error("GeobedCity.City should not be empty for Austin, TX")
	}

	// Test Country() method
	country := result.Country()
	if country != "US" {
		t.Errorf("GeobedCity.Country() = %q, want %q", country, "US")
	}

	// Test Region() method
	region := result.Region()
	if region != "TX" {
		t.Errorf("GeobedCity.Region() = %q, want %q", region, "TX")
	}

	// Test Latitude and Longitude
	if result.Latitude == 0 && result.Longitude == 0 {
		t.Error("GeobedCity coordinates should not be (0, 0) for Austin, TX")
	}

	// Latitude should be approximately 30.26 for Austin
	if result.Latitude < 30.0 || result.Latitude > 31.0 {
		t.Errorf("GeobedCity.Latitude = %v, expected approximately 30.26", result.Latitude)
	}

	// Longitude should be approximately -97.74 for Austin
	if result.Longitude < -98.0 || result.Longitude > -97.0 {
		t.Errorf("GeobedCity.Longitude = %v, expected approximately -97.74", result.Longitude)
	}

	// Test Population
	if result.Population <= 0 {
		t.Errorf("GeobedCity.Population = %d, expected positive value for Austin", result.Population)
	}
}

// TestSpecialInputFormats tests various input format edge cases.
func TestSpecialInputFormats(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name  string
		input string
	}{
		{"leading spaces", "  Austin"},
		{"trailing spaces", "Austin  "},
		{"multiple spaces between words", "New  York"},
		{"tabs between words", "New\tYork"},
		{"mixed whitespace", " Austin , TX "},
		{"multiple commas", "Austin,,,TX"},
		{"semicolon separator", "Austin;TX"},
		{"hyphenated city", "Winston-Salem"},
		{"city with apostrophe", "O'Fallon"},
		{"city with period", "St. Louis"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			result := g.Geocode(tt.input)
			// Just verify it returns something (even empty) without crashing
			_ = result.City
		})
	}
}
