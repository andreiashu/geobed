package geobed

import (
	"math"
	"sync"
	"testing"
)

func TestReverseGeocode_KnownCities(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name          string
		lat           float64
		lng           float64
		wantCity      string
		wantCountry   string
		allowAlts     []string // alternative cities that are acceptable
	}{
		{
			name:        "Austin_TX",
			lat:         30.267,
			lng:         -97.743,
			wantCity:    "Austin",
			wantCountry: "US",
		},
		{
			name:        "Paris_FR",
			lat:         48.857,
			lng:         2.352,
			wantCity:    "Paris",
			wantCountry: "FR",
		},
		{
			name:        "Sydney_AU",
			lat:         -33.869,
			lng:         151.209,
			wantCity:    "Sydney",
			wantCountry: "AU",
		},
		{
			name:        "Berlin_DE",
			lat:         52.520,
			lng:         13.405,
			wantCity:    "Berlin",
			wantCountry: "DE",
		},
		{
			name:        "Tokyo_area_JP",
			lat:         35.676,
			lng:         139.650,
			wantCountry: "JP",
			// Accept various Tokyo metro cities
			allowAlts: []string{"Tokyo", "Shibuya", "Shinjuku", "Chiyoda", "Minato", "Nakano"},
		},
		{
			name:        "Moscow_RU",
			lat:         55.756,
			lng:         37.617,
			wantCity:    "Moscow",
			wantCountry: "RU",
		},
		{
			name:        "Cairo_EG",
			lat:         30.044,
			lng:         31.236,
			wantCity:    "Cairo",
			wantCountry: "EG",
		},
		{
			name:        "Lagos_NG",
			lat:         6.454,
			lng:         3.395,
			wantCity:    "Lagos",
			wantCountry: "NG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.ReverseGeocode(tt.lat, tt.lng)

			if result.City == "" {
				t.Errorf("ReverseGeocode(%v, %v) returned empty city", tt.lat, tt.lng)
				return
			}

			// Check country
			country := result.Country()
			if country != tt.wantCountry {
				t.Errorf("ReverseGeocode(%v, %v) country = %q, want %q",
					tt.lat, tt.lng, country, tt.wantCountry)
			}

			// Check city
			if len(tt.allowAlts) > 0 {
				// Check if result matches any alternative
				found := false
				for _, alt := range tt.allowAlts {
					if result.City == alt {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ReverseGeocode(%v, %v) city = %q, want one of %v",
						tt.lat, tt.lng, result.City, tt.allowAlts)
				}
			} else if tt.wantCity != "" {
				if result.City != tt.wantCity {
					t.Errorf("ReverseGeocode(%v, %v) city = %q, want %q",
						tt.lat, tt.lng, result.City, tt.wantCity)
				}
			}

			// Verify coordinates are reasonable
			if math.Abs(float64(result.Latitude)-tt.lat) > 1.0 {
				t.Errorf("ReverseGeocode(%v, %v) latitude = %v, differs by more than 1 degree",
					tt.lat, tt.lng, result.Latitude)
			}
			if math.Abs(float64(result.Longitude)-tt.lng) > 1.0 {
				t.Errorf("ReverseGeocode(%v, %v) longitude = %v, differs by more than 1 degree",
					tt.lat, tt.lng, result.Longitude)
			}

			// Verify population is positive
			if result.Population <= 0 {
				t.Errorf("ReverseGeocode(%v, %v) population = %v, want > 0",
					tt.lat, tt.lng, result.Population)
			}
		})
	}
}

func TestReverseGeocode_RemoteLocations(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name string
		lat  float64
		lng  float64
	}{
		{"North_Pole", 90, 0},
		{"South_Pole", -90, 0},
		{"Mid_Pacific", 0, -160},
		{"Mid_Atlantic", 30, -40},
		{"Antarctic", -75, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.ReverseGeocode(tt.lat, tt.lng)
			if result.City != "" {
				t.Errorf("ReverseGeocode(%v, %v) city = %q, want empty for remote location",
					tt.lat, tt.lng, result.City)
			}
		})
	}
}

func TestReverseGeocode_InvalidCoordinates(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name string
		lat  float64
		lng  float64
	}{
		{"NaN_lat", math.NaN(), 0},
		{"NaN_lng", 0, math.NaN()},
		{"PosInf_lat", math.Inf(1), 0},
		{"NegInf_lng", 0, math.Inf(-1)},
		{"Both_NaN", math.NaN(), math.NaN()},
		{"Both_PosInf", math.Inf(1), math.Inf(1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic with invalid coordinates
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ReverseGeocode(%v, %v) panicked: %v", tt.lat, tt.lng, r)
				}
			}()

			result := g.ReverseGeocode(tt.lat, tt.lng)

			// Should return empty city for invalid coordinates
			if result.City != "" {
				t.Errorf("ReverseGeocode(%v, %v) city = %q, want empty for invalid coordinates",
					tt.lat, tt.lng, result.City)
			}
		})
	}
}

func TestReverseGeocode_BoundaryCoordinates(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name string
		lat  float64
		lng  float64
	}{
		{"Date_line_positive", 0, 180},
		{"Date_line_negative", 0, -180},
		{"Extreme_south_east", -90, 180},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic with boundary coordinates
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ReverseGeocode(%v, %v) panicked: %v", tt.lat, tt.lng, r)
				}
			}()

			result := g.ReverseGeocode(tt.lat, tt.lng)

			// Boundary coordinates may or may not return a city
			// Just verify the result is valid if non-empty
			if result.City != "" {
				if result.Population <= 0 {
					t.Errorf("ReverseGeocode(%v, %v) returned city %q with invalid population %v",
						tt.lat, tt.lng, result.City, result.Population)
				}
			}
		})
	}
}

func TestReverseGeocode_NeighborhoodOverride(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	t.Run("City_of_London_returns_London", func(t *testing.T) {
		// City of London (small financial district) coordinates
		lat := 51.513
		lng := -0.092

		result := g.ReverseGeocode(lat, lng)

		if result.City == "" {
			t.Errorf("ReverseGeocode(%v, %v) returned empty city", lat, lng)
			return
		}

		// Should return "London" (the big city), not "City of London" (the small district)
		// This tests the population-based override mentioned in the docs
		if result.City == "City of London" {
			t.Errorf("ReverseGeocode(%v, %v) returned %q, expected larger city like 'London'",
				lat, lng, result.City)
		}

		// Verify it's in the UK
		country := result.Country()
		if country != "GB" {
			t.Errorf("ReverseGeocode(%v, %v) country = %q, want GB", lat, lng, country)
		}
	})
}

func TestReverseGeocode_SmallOffsets(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	tests := []struct {
		name     string
		lat1     float64
		lng1     float64
		lat2     float64
		lng2     float64
		wantCity string
	}{
		{
			name:     "Paris_center_and_1km_north",
			lat1:     48.857,
			lng1:     2.352,
			lat2:     48.867, // ~1km north
			lng2:     2.352,
			wantCity: "Paris",
		},
		{
			name:     "Berlin_center_and_1km_east",
			lat1:     52.520,
			lng1:     13.405,
			lat2:     52.520,
			lng2:     13.420, // ~1km east
			wantCity: "Berlin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result1 := g.ReverseGeocode(tt.lat1, tt.lng1)
			result2 := g.ReverseGeocode(tt.lat2, tt.lng2)

			if result1.City != tt.wantCity {
				t.Errorf("ReverseGeocode(%v, %v) city = %q, want %q",
					tt.lat1, tt.lng1, result1.City, tt.wantCity)
			}

			if result2.City != tt.wantCity {
				t.Errorf("ReverseGeocode(%v, %v) city = %q, want %q",
					tt.lat2, tt.lng2, result2.City, tt.wantCity)
			}

			// Both calls should return the same city
			if result1.City != result2.City {
				t.Errorf("Small offset changed city: (%v,%v) = %q, (%v,%v) = %q",
					tt.lat1, tt.lng1, result1.City, tt.lat2, tt.lng2, result2.City)
			}
		})
	}
}

func TestReverseGeocode_Determinism(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	lat := 48.857
	lng := 2.352

	// Get initial result
	firstResult := g.ReverseGeocode(lat, lng)
	if firstResult.City == "" {
		t.Fatalf("ReverseGeocode(%v, %v) returned empty city", lat, lng)
	}

	// Call 100 times and verify we get the same result
	for i := 0; i < 100; i++ {
		result := g.ReverseGeocode(lat, lng)

		if result.City != firstResult.City {
			t.Errorf("Call %d: ReverseGeocode(%v, %v) city = %q, want %q (first result)",
				i+1, lat, lng, result.City, firstResult.City)
		}

		if result.Latitude != firstResult.Latitude {
			t.Errorf("Call %d: ReverseGeocode(%v, %v) latitude = %v, want %v (first result)",
				i+1, lat, lng, result.Latitude, firstResult.Latitude)
		}

		if result.Longitude != firstResult.Longitude {
			t.Errorf("Call %d: ReverseGeocode(%v, %v) longitude = %v, want %v (first result)",
				i+1, lat, lng, result.Longitude, firstResult.Longitude)
		}

		if result.Population != firstResult.Population {
			t.Errorf("Call %d: ReverseGeocode(%v, %v) population = %v, want %v (first result)",
				i+1, lat, lng, result.Population, firstResult.Population)
		}
	}
}

func TestReverseGeocode_ConcurrentSafety(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	// Test coordinates from various world cities
	coords := []struct {
		lat float64
		lng float64
	}{
		{30.267, -97.743},  // Austin
		{48.857, 2.352},    // Paris
		{-33.869, 151.209}, // Sydney
		{52.520, 13.405},   // Berlin
		{35.676, 139.650},  // Tokyo
		{55.756, 37.617},   // Moscow
		{30.044, 31.236},   // Cairo
		{6.454, 3.395},     // Lagos
	}

	var wg sync.WaitGroup
	numGoroutines := 50

	// Recover from any panics in goroutines
	panics := make(chan interface{}, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics <- r
				}
			}()

			// Each goroutine calls ReverseGeocode multiple times
			for j := 0; j < 10; j++ {
				coord := coords[(id+j)%len(coords)]
				result := g.ReverseGeocode(coord.lat, coord.lng)

				// Basic validation
				if result.City != "" && result.Population <= 0 {
					t.Errorf("Goroutine %d: invalid population for city %q", id, result.City)
				}
			}
		}(i)
	}

	wg.Wait()
	close(panics)

	// Check if any goroutine panicked
	for p := range panics {
		t.Errorf("Goroutine panicked during concurrent access: %v", p)
	}
}

func TestReverseGeocode_EmptyResult(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	// Test that remote location returns a zero-value GeobedCity
	result := g.ReverseGeocode(90, 0) // North Pole

	if result.City != "" {
		t.Errorf("Empty result: City = %q, want empty", result.City)
	}
	if result.CityAlt != "" {
		t.Errorf("Empty result: CityAlt = %q, want empty", result.CityAlt)
	}
	if result.Latitude != 0 {
		t.Errorf("Empty result: Latitude = %v, want 0", result.Latitude)
	}
	if result.Longitude != 0 {
		t.Errorf("Empty result: Longitude = %v, want 0", result.Longitude)
	}
	if result.Population != 0 {
		t.Errorf("Empty result: Population = %v, want 0", result.Population)
	}
}

func TestReverseGeocode_ResultFields(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	// Test that a known city returns all expected fields
	result := g.ReverseGeocode(48.857, 2.352) // Paris

	if result.City == "" {
		t.Fatal("Expected non-empty City field")
	}

	// Latitude should be close to input
	if math.Abs(float64(result.Latitude)-48.857) > 1.0 {
		t.Errorf("Latitude = %v, expected close to 48.857", result.Latitude)
	}

	// Longitude should be close to input
	if math.Abs(float64(result.Longitude)-2.352) > 1.0 {
		t.Errorf("Longitude = %v, expected close to 2.352", result.Longitude)
	}

	// Population should be positive
	if result.Population <= 0 {
		t.Errorf("Population = %v, want > 0", result.Population)
	}

	// Country method should return a non-empty string
	country := result.Country()
	if country == "" {
		t.Error("Country() returned empty string")
	}

	// Region method should not panic (may be empty)
	region := result.Region()
	_ = region // Region can be empty, just verify it doesn't panic
}
