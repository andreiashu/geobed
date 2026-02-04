package geobed

import (
	"testing"
)

// TestValidation runs all validation checks on the current cache.
// This is the same validation used by the update-cache tool.
func TestValidation(t *testing.T) {
	if err := ValidateCache(); err != nil {
		t.Fatalf("Validation failed: %v", err)
	}
}

// TestDataIntegrity checks that the loaded data meets minimum thresholds.
func TestDataIntegrity(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to load geobed: %v", err)
	}

	if len(g.c) < minCityCount {
		t.Errorf("City count %d is below minimum %d", len(g.c), minCityCount)
	}

	if len(g.co) < minCountryCount {
		t.Errorf("Country count %d is below minimum %d", len(g.co), minCountryCount)
	}

	// Check that we have cities on all continents (basic global coverage)
	continentCities := map[string]string{
		"North America": "New York",
		"South America": "São Paulo",
		"Europe":        "London",
		"Asia":          "Tokyo",
		"Africa":        "Cairo",
		"Oceania":       "Sydney",
	}

	for continent, city := range continentCities {
		result := g.Geocode(city)
		if result.City == "" {
			t.Errorf("Missing city for %s: %q returned empty", continent, city)
		}
	}
}

// TestKnownCitiesGeocode validates that well-known cities geocode correctly.
func TestKnownCitiesGeocode(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to load geobed: %v", err)
	}

	for _, tc := range knownCities {
		t.Run(tc.query, func(t *testing.T) {
			result := g.Geocode(tc.query)
			if result.City != tc.wantCity {
				t.Errorf("Geocode(%q) city = %q, want %q", tc.query, result.City, tc.wantCity)
			}
			if result.Country() != tc.wantCountry {
				t.Errorf("Geocode(%q) country = %q, want %q", tc.query, result.Country(), tc.wantCountry)
			}
		})
	}
}

// TestKnownCoordsReverseGeocode validates that known coordinates reverse geocode correctly.
func TestKnownCoordsReverseGeocode(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to load geobed: %v", err)
	}

	for _, tc := range knownCoords {
		name := tc.wantCity
		t.Run(name, func(t *testing.T) {
			result := g.ReverseGeocode(tc.lat, tc.lng)
			if result.City != tc.wantCity {
				t.Errorf("ReverseGeocode(%v, %v) city = %q, want %q", tc.lat, tc.lng, result.City, tc.wantCity)
			}
			if result.Country() != tc.wantCountry {
				t.Errorf("ReverseGeocode(%v, %v) country = %q, want %q", tc.lat, tc.lng, result.Country(), tc.wantCountry)
			}
		})
	}
}
