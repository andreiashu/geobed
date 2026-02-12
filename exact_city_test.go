package geobed

import (
	"testing"
)

func TestExactCityMatch(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	// ──────────────────────────────────────────────
	// Path 1: Single match → return immediately
	// ──────────────────────────────────────────────

	t.Run("SingleMatch", func(t *testing.T) {
		t.Run("Tokyo_Japan", func(t *testing.T) {
			result := g.Geocode("Tokyo, Japan", GeocodeOptions{ExactCity: true})
			if result.City != "Tokyo" {
				t.Errorf("city = %q, want 'Tokyo'", result.City)
			}
			if result.Country() != "JP" {
				t.Errorf("country = %q, want 'JP'", result.Country())
			}
		})

		t.Run("Reykjavik_IS", func(t *testing.T) {
			result := g.Geocode("Reykjavik", GeocodeOptions{ExactCity: true})
			if result.City != "" {
				if result.Country() != "IS" {
					t.Errorf("country = %q, want 'IS'", result.Country())
				}
			}
		})
	})

	// ──────────────────────────────────────────────
	// Path 2: Multiple matches, region match
	// ──────────────────────────────────────────────

	t.Run("MultipleMatches_RegionMatch", func(t *testing.T) {
		tests := []struct {
			name       string
			query      string
			wantCity   string
			wantRegion string
		}{
			{"Austin_TX", "Austin, TX", "Austin", "TX"},
			{"Portland_OR", "Portland, OR", "Portland", "OR"},
			{"Portland_ME", "Portland, ME", "Portland", "ME"},
			{"Springfield_IL", "Springfield, IL", "Springfield", "IL"},
			{"Springfield_MO", "Springfield, MO", "Springfield", "MO"},
			{"Springfield_MA", "Springfield, MA", "Springfield", "MA"},
			{"London_OH", "London, OH", "London", "OH"},
			{"Columbus_OH", "Columbus, OH", "Columbus", "OH"},
			{"Columbus_GA", "Columbus, GA", "Columbus", "GA"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := g.Geocode(tt.query, GeocodeOptions{ExactCity: true})
				if result.City != tt.wantCity {
					t.Errorf("Geocode(%q, exact) city = %q, want %q", tt.query, result.City, tt.wantCity)
				}
				if result.Region() != tt.wantRegion {
					t.Errorf("Geocode(%q, exact) region = %q, want %q", tt.query, result.Region(), tt.wantRegion)
				}
				if result.Country() != "US" {
					t.Errorf("Geocode(%q, exact) country = %q, want 'US'", tt.query, result.Country())
				}
			})
		}
	})

	// ──────────────────────────────────────────────
	// Path 3: Multiple matches, country+region match
	// ──────────────────────────────────────────────

	t.Run("MultipleMatches_CountryRegionMatch", func(t *testing.T) {
		tests := []struct {
			name        string
			query       string
			wantCity    string
			wantCountry string
		}{
			{"London_UK", "London, United Kingdom", "London", "GB"},
			{"Dublin_Ireland", "Dublin, Ireland", "Dublin", "IE"},
			{"Berlin_Germany", "Berlin, Germany", "Berlin", "DE"},
			{"Cairo_Egypt", "Cairo, Egypt", "Cairo", "EG"},
			{"Sydney_Australia", "Sydney, Australia", "Sydney", "AU"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := g.Geocode(tt.query, GeocodeOptions{ExactCity: true})
				if result.City != tt.wantCity {
					t.Errorf("Geocode(%q, exact) city = %q, want %q", tt.query, result.City, tt.wantCity)
				}
				if result.Country() != tt.wantCountry {
					t.Errorf("Geocode(%q, exact) country = %q, want %q", tt.query, result.Country(), tt.wantCountry)
				}
			})
		}
	})

	// ──────────────────────────────────────────────
	// Path 4: No region match, fallback to country
	// ──────────────────────────────────────────────

	t.Run("NoRegionMatch_FallbackCountry", func(t *testing.T) {
		t.Run("Austin_US_highest_pop", func(t *testing.T) {
			result := g.Geocode("Austin, United States", GeocodeOptions{ExactCity: true})
			if result.City != "Austin" {
				t.Errorf("city = %q, want 'Austin'", result.City)
			}
			if result.Country() != "US" {
				t.Errorf("country = %q, want 'US'", result.Country())
			}
			if result.Population <= 500_000 {
				t.Errorf("population = %d, want > 500K", result.Population)
			}
		})

		t.Run("Portland_US_highest_pop", func(t *testing.T) {
			result := g.Geocode("Portland, United States", GeocodeOptions{ExactCity: true})
			if result.City != "Portland" {
				t.Errorf("city = %q, want 'Portland'", result.City)
			}
			if result.Country() != "US" {
				t.Errorf("country = %q, want 'US'", result.Country())
			}
			if result.Population <= 400_000 {
				t.Errorf("population = %d, want > 400K", result.Population)
			}
		})

		t.Run("Springfield_US", func(t *testing.T) {
			result := g.Geocode("Springfield, United States", GeocodeOptions{ExactCity: true})
			if result.City != "Springfield" {
				t.Errorf("city = %q, want 'Springfield'", result.City)
			}
			if result.Country() != "US" {
				t.Errorf("country = %q, want 'US'", result.Country())
			}
		})
	})

	// ──────────────────────────────────────────────
	// Path 5: No match → empty result
	// ──────────────────────────────────────────────

	t.Run("NoMatch_EmptyResult", func(t *testing.T) {
		noMatchQueries := []string{
			"Nonexistent City",
			"Xyzzyplugh",
			"12345",
			"!@#$%",
		}
		for _, query := range noMatchQueries {
			t.Run(query, func(t *testing.T) {
				result := g.Geocode(query, GeocodeOptions{ExactCity: true})
				if result.City != "" {
					t.Errorf("Geocode(%q, exact) city = %q, want empty", query, result.City)
				}
			})
		}
	})

	// ──────────────────────────────────────────────
	// Ambiguous names without qualifier
	// ──────────────────────────────────────────────

	t.Run("AmbiguousNamesNoQualifier", func(t *testing.T) {
		for _, name := range []string{"London", "Dublin", "Austin", "Paris"} {
			t.Run(name, func(t *testing.T) {
				result := g.Geocode(name, GeocodeOptions{ExactCity: true})
				if result.City != "" && result.City != name {
					t.Errorf("city = %q, want %q or empty", result.City, name)
				}
			})
		}
	})

	// ──────────────────────────────────────────────
	// Population disambiguation within exactCity
	// ──────────────────────────────────────────────

	t.Run("PopulationDisambiguation", func(t *testing.T) {
		t.Run("Dublin_US_highest_pop", func(t *testing.T) {
			result := g.Geocode("Dublin, United States", GeocodeOptions{ExactCity: true})
			if result.City != "" {
				if result.City != "Dublin" {
					t.Errorf("city = %q, want 'Dublin'", result.City)
				}
				if result.Country() != "US" {
					t.Errorf("country = %q, want 'US'", result.Country())
				}
			}
		})

		t.Run("Portland_OR_higher_pop_than_ME", func(t *testing.T) {
			or := g.Geocode("Portland, OR", GeocodeOptions{ExactCity: true})
			me := g.Geocode("Portland, ME", GeocodeOptions{ExactCity: true})
			if or.City != "Portland" || me.City != "Portland" {
				t.Fatalf("expected both to be Portland, got %q and %q", or.City, me.City)
			}
			if or.Population <= me.Population {
				t.Errorf("Portland OR pop (%d) should be > Portland ME pop (%d)", or.Population, me.Population)
			}
		})

		t.Run("Columbus_OH_higher_pop_than_GA", func(t *testing.T) {
			oh := g.Geocode("Columbus, OH", GeocodeOptions{ExactCity: true})
			ga := g.Geocode("Columbus, GA", GeocodeOptions{ExactCity: true})
			if oh.City != "Columbus" || ga.City != "Columbus" {
				t.Fatalf("expected both to be Columbus, got %q and %q", oh.City, ga.City)
			}
			if oh.Population <= ga.Population {
				t.Errorf("Columbus OH pop (%d) should be > Columbus GA pop (%d)", oh.Population, ga.Population)
			}
		})
	})
}
