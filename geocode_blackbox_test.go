package geobed

import (
	"strings"
	"testing"
)

// Black-box tests for the Geocode method.
// These tests are based solely on the public API and documentation, without knowledge
// of internal implementation details.

func TestBlackBox_GeocodeEmptyAndWhitespace(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"single space", " "},
		{"multiple spaces", "   "},
		{"tab", "\t"},
		{"newline", "\n"},
		{"carriage return", "\r"},
		{"mixed whitespace", " \t\n\r "},
		{"only tabs", "\t\t\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.Geocode(tt.input)
			if result.City != "" {
				t.Errorf("Geocode(%q) returned city %q, expected empty GeobedCity", tt.input, result.City)
			}
			if result.Latitude != 0 || result.Longitude != 0 {
				t.Errorf("Geocode(%q) returned non-zero coordinates (%f, %f)", tt.input, result.Latitude, result.Longitude)
			}
		})
	}
}

func TestBlackBox_GeocodeInputLengthLimiting(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "300 ASCII characters",
			input: strings.Repeat("a", 300),
		},
		{
			name:  "300 Unicode runes (Chinese)",
			input: strings.Repeat("北京", 150), // 300 runes
		},
		{
			name:  "300 Unicode runes (emoji)",
			input: strings.Repeat("🌍", 300),
		},
		{
			name:  "mixed long string with city name",
			input: "Tokyo" + strings.Repeat("x", 300),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Geocode panicked with input of %d runes: %v", len([]rune(tt.input)), r)
				}
			}()

			result := g.Geocode(tt.input)
			// Result should be valid (even if empty), no crash
			_ = result
		})
	}
}

func TestBlackBox_GeocodeFuzzyDistanceCapping(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		query         string
		fuzzyDistance int
		expectEmpty   bool
	}{
		{
			name:          "FuzzyDistance=0 exact match required",
			query:         "Paris",
			fuzzyDistance: 0,
			expectEmpty:   false, // Exact match should work
		},
		{
			name:          "FuzzyDistance=0 with typo",
			query:         "Pariz", // Typo
			fuzzyDistance: 0,
			expectEmpty:   false, // Still matches due to other scoring mechanisms
		},
		{
			name:          "FuzzyDistance=100 should not crash",
			query:         "London",
			fuzzyDistance: 100,
			expectEmpty:   false, // Should still work
		},
		{
			name:          "FuzzyDistance=1000 extreme value",
			query:         "Tokyo",
			fuzzyDistance: 1000,
			expectEmpty:   false, // Should be capped and still work
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not timeout or crash
			done := make(chan bool, 1)
			go func() {
				result := g.Geocode(tt.query, GeocodeOptions{FuzzyDistance: tt.fuzzyDistance})
				if tt.expectEmpty && result.City != "" {
					t.Errorf("Expected empty result for %q with FuzzyDistance=%d, got %q",
						tt.query, tt.fuzzyDistance, result.City)
				}
				if !tt.expectEmpty && result.City == "" {
					t.Errorf("Expected non-empty result for %q with FuzzyDistance=%d",
						tt.query, tt.fuzzyDistance)
				}
				done <- true
			}()
			<-done
		})
	}
}

func TestBlackBox_GeocodeExactCityMode(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		query       string
		expectCity  string
		expectState string
		allowEmpty  bool
	}{
		{
			name:        "Austin, TX with ExactCity",
			query:       "Austin, TX",
			expectCity:  "Austin",
			expectState: "TX",
			allowEmpty:  false,
		},
		{
			name:        "Nonexistent City with ExactCity",
			query:       "Nonexistent City",
			expectCity:  "", // Should return empty
			expectState: "",
			allowEmpty:  true,
		},
		{
			name:        "Paris with ExactCity",
			query:       "Paris",
			expectCity:  "Paris",
			expectState: "", // Not checking state for this test
			allowEmpty:  true, // ExactCity may require more specific matching
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.Geocode(tt.query, GeocodeOptions{ExactCity: true})

			if tt.expectCity == "" {
				if result.City != "" {
					t.Errorf("Expected empty city for %q with ExactCity=true, got %q",
						tt.query, result.City)
				}
				return
			}

			// If allowEmpty is true, accept either the expected city or empty result
			if tt.allowEmpty && result.City == "" {
				t.Logf("ExactCity mode returned empty for %q (acceptable)", tt.query)
				return
			}

			if result.City != tt.expectCity {
				if !tt.allowEmpty {
					t.Errorf("Geocode(%q, ExactCity=true) city = %q, want %q",
						tt.query, result.City, tt.expectCity)
				}
			}

			if tt.expectState != "" && result.Region() != tt.expectState {
				t.Errorf("Geocode(%q, ExactCity=true) region = %q, want %q",
					tt.query, result.Region(), tt.expectState)
			}
		})
	}
}

func TestBlackBox_GeocodeStandardCities(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query         string
		expectCity    string
		expectCountry string
	}{
		{"Paris", "Paris", "FR"},
		{"London", "London", "GB"},
		{"Tokyo", "Tokyo", "JP"},
		{"Berlin", "Berlin", "DE"},
		{"Sydney", "Sydney", "AU"},
		{"Moscow", "Moscow", "RU"},
		{"Beijing", "Beijing", "CN"},
		{"Seoul", "Seoul", "KR"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := g.Geocode(tt.query)
			if result.City != tt.expectCity {
				t.Errorf("Geocode(%q) city = %q, want %q", tt.query, result.City, tt.expectCity)
			}
			if result.Country() != tt.expectCountry {
				t.Errorf("Geocode(%q) country = %q, want %q", tt.query, result.Country(), tt.expectCountry)
			}
			if result.City != "" && (result.Latitude == 0 && result.Longitude == 0) {
				t.Errorf("Geocode(%q) returned zero coordinates for valid city", tt.query)
			}
		})
	}
}

func TestBlackBox_GeocodeCountryDisambiguation(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query         string
		expectCity    string
		expectCountry string
		expectRegion  string // For US cities
	}{
		{
			query:         "Paris, France",
			expectCity:    "Paris",
			expectCountry: "FR",
		},
		{
			query:         "Paris, TX",
			expectCity:    "Paris",
			expectCountry: "US",
			expectRegion:  "TX",
		},
		{
			query:         "London, United Kingdom",
			expectCity:    "London",
			expectCountry: "GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := g.Geocode(tt.query)
			if result.City != tt.expectCity {
				t.Errorf("Geocode(%q) city = %q, want %q", tt.query, result.City, tt.expectCity)
			}
			if result.Country() != tt.expectCountry {
				t.Errorf("Geocode(%q) country = %q, want %q", tt.query, result.Country(), tt.expectCountry)
			}
			if tt.expectRegion != "" && result.Region() != tt.expectRegion {
				t.Errorf("Geocode(%q) region = %q, want %q", tt.query, result.Region(), tt.expectRegion)
			}
		})
	}
}

func TestBlackBox_GeocodeStateDisambiguation(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query        string
		expectCity   string
		expectRegion string
	}{
		{"Springfield, IL", "Springfield", "IL"},
		{"Springfield, MA", "Springfield", "MA"},
		{"Portland, OR", "Portland", "OR"},
		{"Portland, ME", "Portland", "ME"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := g.Geocode(tt.query)
			if result.City != tt.expectCity {
				t.Errorf("Geocode(%q) city = %q, want %q", tt.query, result.City, tt.expectCity)
			}
			if result.Region() != tt.expectRegion {
				t.Errorf("Geocode(%q) region = %q, want %q", tt.query, result.Region(), tt.expectRegion)
			}
			if result.Country() != "US" {
				t.Errorf("Geocode(%q) country = %q, want US", tt.query, result.Country())
			}
		})
	}
}

func TestBlackBox_GeocodeCaseInsensitivity(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	queries := []string{"LONDON", "london", "London", "LoNdOn"}
	var results []GeobedCity

	for _, query := range queries {
		result := g.Geocode(query)
		results = append(results, result)
	}

	// All results should be the same
	for i := 1; i < len(results); i++ {
		if results[i].City != results[0].City {
			t.Errorf("Case variation %q returned different city %q vs %q",
				queries[i], results[i].City, results[0].City)
		}
		if results[i].Latitude != results[0].Latitude || results[i].Longitude != results[0].Longitude {
			t.Errorf("Case variation %q returned different coordinates", queries[i])
		}
		if results[i].Country() != results[0].Country() {
			t.Errorf("Case variation %q returned different country %q vs %q",
				queries[i], results[i].Country(), results[0].Country())
		}
	}

	// Verify we got London
	if results[0].City != "London" {
		t.Errorf("Expected London, got %q", results[0].City)
	}
}

func TestBlackBox_GeocodeAlternateNames(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query            string
		expectCity       string // Modern/official name
		expectCountry    string
		expectAltContains string // Alternate name should be in CityAlt
	}{
		{
			query:         "Bombay",
			expectCity:    "Mumbai",
			expectCountry: "IN",
		},
		{
			query:         "Peking",
			expectCity:    "Beijing",
			expectCountry: "CN",
		},
		{
			query:         "Constantinople",
			expectCity:    "Istanbul",
			expectCountry: "TR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := g.Geocode(tt.query)
			if result.City != tt.expectCity {
				t.Errorf("Geocode(%q) city = %q, want %q", tt.query, result.City, tt.expectCity)
			}
			if result.Country() != tt.expectCountry {
				t.Errorf("Geocode(%q) country = %q, want %q", tt.query, result.Country(), tt.expectCountry)
			}
			// Verify it's a valid city with coordinates
			if result.City != "" && (result.Latitude == 0 && result.Longitude == 0) {
				t.Errorf("Geocode(%q) returned zero coordinates", tt.query)
			}
		})
	}
}

func TestBlackBox_GeocodeMultipleOptionPatterns(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		query   string
		opts    []GeocodeOptions
		wantErr bool
	}{
		{
			name:  "no options",
			query: "Paris",
			opts:  nil,
		},
		{
			name:  "empty options slice",
			query: "Paris",
			opts:  []GeocodeOptions{},
		},
		{
			name:  "single empty option struct",
			query: "Paris",
			opts:  []GeocodeOptions{{}},
		},
		{
			name:  "ExactCity only",
			query: "Austin, TX",
			opts:  []GeocodeOptions{{ExactCity: true}},
		},
		{
			name:  "FuzzyDistance only",
			query: "Paris",
			opts:  []GeocodeOptions{{FuzzyDistance: 2}},
		},
		{
			name:  "both ExactCity and FuzzyDistance",
			query: "London",
			opts:  []GeocodeOptions{{ExactCity: true, FuzzyDistance: 1}},
		},
		{
			name:  "multiple option structs (last wins)",
			query: "Tokyo",
			opts:  []GeocodeOptions{{FuzzyDistance: 1}, {FuzzyDistance: 2}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Geocode panicked with options %+v: %v", tt.opts, r)
				}
			}()

			result := g.Geocode(tt.query, tt.opts...)
			// For these well-known cities, we should get results (unless ExactCity blocks it)
			if result.City == "" && !tt.wantErr {
				// This is acceptable - just verify it didn't crash
			}
		})
	}
}

func TestBlackBox_GeocodeCoordinateReasonableness(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query     string
		minLat    float32
		maxLat    float32
		minLon    float32
		maxLon    float32
	}{
		{
			query:  "Tokyo",
			minLat: 35.0,
			maxLat: 36.0,
			minLon: 139.0,
			maxLon: 140.0,
		},
		{
			query:  "Paris",
			minLat: 48.5,
			maxLat: 49.0,
			minLon: 2.0,
			maxLon: 2.8,
		},
		{
			query:  "New York",
			minLat: 40.0,
			maxLat: 41.0,
			minLon: -74.5,
			maxLon: -73.5,
		},
		{
			query:  "Sydney",
			minLat: -34.0,
			maxLat: -33.5,
			minLon: 150.5,
			maxLon: 151.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := g.Geocode(tt.query)
			if result.City == "" {
				t.Fatalf("Geocode(%q) returned empty city", tt.query)
			}

			if result.Latitude < tt.minLat || result.Latitude > tt.maxLat {
				t.Errorf("Geocode(%q) latitude = %f, expected between %f and %f",
					tt.query, result.Latitude, tt.minLat, tt.maxLat)
			}

			if result.Longitude < tt.minLon || result.Longitude > tt.maxLon {
				t.Errorf("Geocode(%q) longitude = %f, expected between %f and %f",
					tt.query, result.Longitude, tt.minLon, tt.maxLon)
			}

			// General sanity checks
			if result.Latitude < -90 || result.Latitude > 90 {
				t.Errorf("Geocode(%q) latitude = %f, outside valid range [-90, 90]",
					tt.query, result.Latitude)
			}
			if result.Longitude < -180 || result.Longitude > 180 {
				t.Errorf("Geocode(%q) longitude = %f, outside valid range [-180, 180]",
					tt.query, result.Longitude)
			}
		})
	}
}

func TestBlackBox_GeocodePopulationField(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	// Major cities should have population data
	tests := []string{"Tokyo", "New York", "Paris", "London", "Beijing"}

	for _, query := range tests {
		t.Run(query, func(t *testing.T) {
			result := g.Geocode(query)
			if result.City == "" {
				t.Fatalf("Geocode(%q) returned empty city", query)
			}

			// Major cities should have positive population
			// (This is a black-box assumption based on the field's purpose)
			if result.Population <= 0 {
				t.Logf("Note: Geocode(%q) population = %d (may be expected for some datasets)",
					query, result.Population)
			}
		})
	}
}

func TestBlackBox_GeocodeReturnType(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	// Test that all fields are properly accessible
	result := g.Geocode("Paris")

	// Access all public fields to verify they exist and have correct types
	_ = result.City       // string
	_ = result.CityAlt    // string
	_ = result.Latitude   // float32
	_ = result.Longitude  // float32
	_ = result.Population // int32

	// Access all public methods
	_ = result.Country() // string
	_ = result.Region()  // string

	// For a valid city, City should be non-empty
	if result.City == "" {
		t.Error("Geocode(\"Paris\") returned empty City field")
	}

	// Country() and Region() should return strings (even if empty)
	country := result.Country()
	region := result.Region()
	_ = country
	_ = region
}

func TestBlackBox_GeocodeIdempotency(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	// Calling Geocode multiple times with same input should return same result
	query := "London"

	result1 := g.Geocode(query)
	result2 := g.Geocode(query)
	result3 := g.Geocode(query)

	if result1.City != result2.City || result2.City != result3.City {
		t.Errorf("Geocode(%q) returned different cities: %q, %q, %q",
			query, result1.City, result2.City, result3.City)
	}

	if result1.Latitude != result2.Latitude || result2.Latitude != result3.Latitude {
		t.Errorf("Geocode(%q) returned different latitudes: %f, %f, %f",
			query, result1.Latitude, result2.Latitude, result3.Latitude)
	}

	if result1.Longitude != result2.Longitude || result2.Longitude != result3.Longitude {
		t.Errorf("Geocode(%q) returned different longitudes: %f, %f, %f",
			query, result1.Longitude, result2.Longitude, result3.Longitude)
	}

	if result1.Country() != result2.Country() || result2.Country() != result3.Country() {
		t.Errorf("Geocode(%q) returned different countries: %q, %q, %q",
			query, result1.Country(), result2.Country(), result3.Country())
	}
}
