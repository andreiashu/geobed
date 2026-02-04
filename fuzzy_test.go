package geobed

import (
	"testing"
)

func TestFuzzyGeocode(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query    string
		maxDist  int
		wantCity string
	}{
		{"Londn", 1, "London"},   // Missing 'o' (distance 1)
		{"Pairis", 1, "Paris"},   // Extra 'i' (distance 1)
		{"Toky", 1, "Tokyo"},     // Missing 'o' (distance 1)
		{"Berln", 1, "Berlin"},   // Missing 'i' (distance 1)
		{"Londno", 2, "London"},  // Transposition (distance 2)
		{"Sydeny", 2, "Sydney"},  // Transposition (distance 2)
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := g.Geocode(tt.query, GeocodeOptions{FuzzyDistance: tt.maxDist})
			if result.City != tt.wantCity {
				t.Errorf("Geocode(%q, fuzzy=%d) = %q, want %q",
					tt.query, tt.maxDist, result.City, tt.wantCity)
			}
		})
	}
}

func TestFuzzyGeocodeDistance2(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query    string
		maxDist  int
		wantCity string
	}{
		{"Lnodon", 2, "London"},      // Two character swap
		{"Tokiyo", 2, "Tokyo"},       // Extra character + swap
		{"Sydnei", 2, "Sydney"},      // Two character changes
		{"Mooscow", 2, "Moscow"},     // Extra 'o'
		{"Amsterdm", 2, "Amsterdam"}, // Missing 'a'
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := g.Geocode(tt.query, GeocodeOptions{FuzzyDistance: tt.maxDist})
			if result.City != tt.wantCity {
				t.Errorf("Geocode(%q, fuzzy=%d) = %q, want %q",
					tt.query, tt.maxDist, result.City, tt.wantCity)
			}
		})
	}
}

func TestFuzzyMatchDisabled(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	// With FuzzyDistance=0, typos should NOT match
	result := g.Geocode("Londn", GeocodeOptions{FuzzyDistance: 0})
	if result.City == "London" {
		t.Errorf("Geocode(%q, fuzzy=0) = %q, expected NOT to match London",
			"Londn", result.City)
	}
}

func TestFuzzyMatchBackwardCompatibility(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	// Test that existing code without FuzzyDistance works as before
	testCases := []struct {
		query    string
		wantCity string
	}{
		{"Austin", "Austin"},
		{"Paris", "Paris"},
		{"Sydney", "Sydney"},
		{"Berlin", "Berlin"},
		{"Tokyo", "Tokyo"},
	}

	for _, tc := range testCases {
		t.Run(tc.query, func(t *testing.T) {
			// Without options (backward compatible)
			result := g.Geocode(tc.query)
			if result.City != tc.wantCity {
				t.Errorf("Geocode(%q) = %q, want %q", tc.query, result.City, tc.wantCity)
			}

			// With empty options (backward compatible)
			result = g.Geocode(tc.query, GeocodeOptions{})
			if result.City != tc.wantCity {
				t.Errorf("Geocode(%q, GeocodeOptions{}) = %q, want %q", tc.query, result.City, tc.wantCity)
			}
		})
	}
}

func TestFuzzyMatchFunction(t *testing.T) {
	tests := []struct {
		query     string
		candidate string
		maxDist   int
		want      bool
	}{
		// Exact matches (distance 0)
		{"London", "London", 0, true},
		{"london", "London", 0, true},
		{"LONDON", "london", 0, true},
		{"Londn", "London", 0, false},

		// Distance 1 matches
		{"Londn", "London", 1, true},   // Missing 'o' (distance 1)
		{"LLondon", "London", 1, true}, // Extra character (distance 1)
		{"Londnn", "London", 1, true},  // Substitution (distance 1)
		{"Londxn", "London", 1, true},  // Substitution (distance 1)
		{"Londoon", "London", 1, true}, // Extra 'o' (distance 1)

		// Distance 2 matches
		{"Londno", "London", 2, true}, // Transposition (distance 2)
		{"Lndn", "London", 2, true},   // Missing two chars (distance 2)
		{"Lnodon", "London", 2, true}, // Two swaps (distance 2)

		// Should NOT match
		{"ABC", "London", 1, false},
		{"XYZ", "London", 2, false},
		{"Londno", "London", 1, false}, // Transposition is distance 2, not 1
	}

	for _, tt := range tests {
		t.Run(tt.query+"_"+tt.candidate, func(t *testing.T) {
			got := fuzzyMatch(tt.query, tt.candidate, tt.maxDist)
			if got != tt.want {
				t.Errorf("fuzzyMatch(%q, %q, %d) = %v, want %v",
					tt.query, tt.candidate, tt.maxDist, got, tt.want)
			}
		})
	}
}

func BenchmarkFuzzyGeocode(b *testing.B) {
	g, err := NewGeobed()
	if err != nil {
		b.Fatal(err)
	}

	b.Run("NoFuzzy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			g.Geocode("London")
		}
	})

	b.Run("FuzzyDistance1", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			g.Geocode("Londn", GeocodeOptions{FuzzyDistance: 1})
		}
	})

	b.Run("FuzzyDistance2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			g.Geocode("Lndn", GeocodeOptions{FuzzyDistance: 2})
		}
	})
}
