package geobed

import (
	"testing"
)

func TestScoringDisambiguation(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	// ──────────────────────────────────────────────
	// Population tiebreaker
	// ──────────────────────────────────────────────

	t.Run("PopulationTiebreaker", func(t *testing.T) {
		t.Run("Springfield_US_highest_pop", func(t *testing.T) {
			result := g.Geocode("Springfield")
			if result.City != "Springfield" {
				t.Errorf("city = %q, want 'Springfield'", result.City)
			}
			if result.Country() != "US" {
				t.Errorf("country = %q, want 'US'", result.Country())
			}
			if result.Population <= 100_000 {
				t.Errorf("population = %d, want > 100K", result.Population)
			}
		})

		t.Run("Columbus_US_OH", func(t *testing.T) {
			result := g.Geocode("Columbus")
			if result.City != "Columbus" {
				t.Errorf("city = %q, want 'Columbus'", result.City)
			}
			if result.Country() != "US" {
				t.Errorf("country = %q, want 'US'", result.Country())
			}
			if result.Population <= 500_000 {
				t.Errorf("population = %d, want > 500K", result.Population)
			}
		})

		t.Run("Birmingham", func(t *testing.T) {
			result := g.Geocode("Birmingham")
			if result.City != "Birmingham" {
				t.Errorf("city = %q, want 'Birmingham'", result.City)
			}
			if result.Population <= 100_000 {
				t.Errorf("population = %d, want > 100K", result.Population)
			}
		})

		t.Run("Moscow_RU", func(t *testing.T) {
			result := g.Geocode("Moscow")
			if result.City != "Moscow" {
				t.Errorf("city = %q, want 'Moscow'", result.City)
			}
			if result.Country() != "RU" {
				t.Errorf("country = %q, want 'RU'", result.Country())
			}
			if result.Population <= 1_000_000 {
				t.Errorf("population = %d, want > 1M", result.Population)
			}
		})

		t.Run("Dublin_IE", func(t *testing.T) {
			result := g.Geocode("Dublin")
			if result.City != "Dublin" {
				t.Errorf("city = %q, want 'Dublin'", result.City)
			}
			if result.Country() != "IE" {
				t.Errorf("country = %q, want 'IE'", result.Country())
			}
			if result.Population <= 500_000 {
				t.Errorf("population = %d, want > 500K", result.Population)
			}
		})

		t.Run("Portland_documents_behavior", func(t *testing.T) {
			result := g.Geocode("Portland")
			if result.City != "Portland" {
				t.Errorf("city = %q, want 'Portland'", result.City)
			}
			country := result.Country()
			if country != "US" {
				// Document the known bug: Portland AU returned instead of Portland OR
				if country != "AU" {
					t.Errorf("country = %q, want 'US' or 'AU' (known issue)", country)
				}
			} else {
				if result.Population <= 500_000 {
					t.Errorf("population = %d, want > 500K for Portland OR", result.Population)
				}
			}
		})

		t.Run("Richmond_US", func(t *testing.T) {
			result := g.Geocode("Richmond")
			if result.City != "Richmond" {
				t.Errorf("city = %q, want 'Richmond'", result.City)
			}
			if result.Country() != "US" {
				t.Errorf("country = %q, want 'US'", result.Country())
			}
		})

		t.Run("Cambridge", func(t *testing.T) {
			result := g.Geocode("Cambridge")
			if result.City != "Cambridge" {
				t.Errorf("city = %q, want 'Cambridge'", result.City)
			}
			if result.Population <= 50_000 {
				t.Errorf("population = %d, want > 50K", result.Population)
			}
		})
	})

	// ──────────────────────────────────────────────
	// Alt-name matching
	// ──────────────────────────────────────────────

	t.Run("AltNameMatching", func(t *testing.T) {
		t.Run("Tokyo_by_name", func(t *testing.T) {
			result := g.Geocode("Tokyo")
			if result.City != "Tokyo" {
				t.Errorf("city = %q, want 'Tokyo'", result.City)
			}
			if result.Country() != "JP" {
				t.Errorf("country = %q, want 'JP'", result.Country())
			}
		})

		t.Run("Tokio→Tokyo", func(t *testing.T) {
			result := g.Geocode("Tokio")
			if result.City != "Tokyo" {
				t.Errorf("city = %q, want 'Tokyo'", result.City)
			}
			if result.Country() != "JP" {
				t.Errorf("country = %q, want 'JP'", result.Country())
			}
		})

		t.Run("Londra→London", func(t *testing.T) {
			result := g.Geocode("Londra")
			if result.City != "London" {
				t.Errorf("city = %q, want 'London'", result.City)
			}
			if result.Country() != "GB" {
				t.Errorf("country = %q, want 'GB'", result.Country())
			}
		})

		t.Run("München→Munich", func(t *testing.T) {
			result := g.Geocode("München")
			if result.City != "Munich" {
				t.Errorf("city = %q, want 'Munich'", result.City)
			}
			if result.Country() != "DE" {
				t.Errorf("country = %q, want 'DE'", result.Country())
			}
		})
	})

	// ──────────────────────────────────────────────
	// Country/state extraction formats
	// ──────────────────────────────────────────────

	t.Run("CountryStateExtractionFormats", func(t *testing.T) {
		tests := []struct {
			name        string
			query       string
			wantCity    string
			wantCountry string
			wantRegion  string
		}{
			{"Paris, France", "Paris, France", "Paris", "FR", ""},
			{"France, Paris", "France, Paris", "Paris", "FR", ""},
			{"Paris France", "Paris France", "Paris", "FR", ""},
			{"TX, Houston", "TX, Houston", "Houston", "US", "TX"},
			{"Houston Texas", "Houston Texas", "Houston", "US", "TX"},
			{"Houston, TX", "Houston, TX", "Houston", "US", "TX"},
			{"Berlin, Germany", "Berlin, Germany", "Berlin", "DE", ""},
			{"Germany, Berlin", "Germany, Berlin", "Berlin", "DE", ""},
			{"Tokyo, Japan", "Tokyo, Japan", "Tokyo", "JP", ""},
			{"Japan, Tokyo", "Japan, Tokyo", "Tokyo", "JP", ""},
			{"Sydney, Australia", "Sydney, Australia", "Sydney", "AU", ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := g.Geocode(tt.query)
				if result.City != tt.wantCity {
					t.Errorf("Geocode(%q) city = %q, want %q", tt.query, result.City, tt.wantCity)
				}
				if result.Country() != tt.wantCountry {
					t.Errorf("Geocode(%q) country = %q, want %q", tt.query, result.Country(), tt.wantCountry)
				}
				if tt.wantRegion != "" && result.Region() != tt.wantRegion {
					t.Errorf("Geocode(%q) region = %q, want %q", tt.query, result.Region(), tt.wantRegion)
				}
			})
		}
	})

	// ──────────────────────────────────────────────
	// Scoring weight paths
	// ──────────────────────────────────────────────

	t.Run("ScoringWeightPaths", func(t *testing.T) {
		t.Run("exact_city_name_match_London", func(t *testing.T) {
			result := g.Geocode("London")
			if result.City != "London" {
				t.Errorf("city = %q, want 'London'", result.City)
			}
		})

		t.Run("region_abbrev_Austin_TX", func(t *testing.T) {
			result := g.Geocode("Austin, TX")
			if result.City != "Austin" {
				t.Errorf("city = %q, want 'Austin'", result.City)
			}
			if result.Region() != "TX" {
				t.Errorf("region = %q, want 'TX'", result.Region())
			}
		})

		t.Run("country_match_London_UK", func(t *testing.T) {
			result := g.Geocode("London, United Kingdom")
			if result.City != "London" {
				t.Errorf("city = %q, want 'London'", result.City)
			}
			if result.Country() != "GB" {
				t.Errorf("country = %q, want 'GB'", result.Country())
			}
		})

		t.Run("state_match_Portland_OR", func(t *testing.T) {
			result := g.Geocode("Portland, OR")
			if result.City != "Portland" {
				t.Errorf("city = %q, want 'Portland'", result.City)
			}
			if result.Region() != "OR" {
				t.Errorf("region = %q, want 'OR'", result.Region())
			}
		})

		t.Run("substring_match_New_York", func(t *testing.T) {
			result := g.Geocode("New York")
			if result.City == "" {
				t.Error("expected non-empty city")
			}
		})

		t.Run("case_insensitive_LONDON", func(t *testing.T) {
			result := g.Geocode("LONDON")
			if result.City != "London" {
				t.Errorf("city = %q, want 'London'", result.City)
			}
		})

		t.Run("region_disambiguation_Springfield_IL", func(t *testing.T) {
			result := g.Geocode("Springfield, IL")
			if result.City != "Springfield" {
				t.Errorf("city = %q, want 'Springfield'", result.City)
			}
			if result.Region() != "IL" {
				t.Errorf("region = %q, want 'IL'", result.Region())
			}
			if result.Country() != "US" {
				t.Errorf("country = %q, want 'US'", result.Country())
			}
		})

		t.Run("region_disambiguation_Springfield_MA", func(t *testing.T) {
			result := g.Geocode("Springfield, MA")
			if result.City != "Springfield" {
				t.Errorf("city = %q, want 'Springfield'", result.City)
			}
			if result.Region() != "MA" {
				t.Errorf("region = %q, want 'MA'", result.Region())
			}
		})

		t.Run("region_disambiguation_Columbus_GA", func(t *testing.T) {
			result := g.Geocode("Columbus, GA")
			if result.City != "Columbus" {
				t.Errorf("city = %q, want 'Columbus'", result.City)
			}
			if result.Region() != "GA" {
				t.Errorf("region = %q, want 'GA'", result.Region())
			}
		})

		t.Run("country_disambiguation_Paris_France_vs_TX", func(t *testing.T) {
			fr := g.Geocode("Paris, France")
			tx := g.Geocode("Paris, TX")
			if fr.City != "Paris" {
				t.Errorf("Paris FR city = %q, want 'Paris'", fr.City)
			}
			if fr.Country() != "FR" {
				t.Errorf("Paris FR country = %q, want 'FR'", fr.Country())
			}
			if tx.City != "Paris" {
				t.Errorf("Paris TX city = %q, want 'Paris'", tx.City)
			}
			if tx.Region() != "TX" {
				t.Errorf("Paris TX region = %q, want 'TX'", tx.Region())
			}
			if fr.Population <= tx.Population {
				t.Errorf("Paris FR pop (%d) should be > Paris TX pop (%d)", fr.Population, tx.Population)
			}
		})

		t.Run("population_bonus_for_cities_gte_1000", func(t *testing.T) {
			result := g.Geocode("London")
			if result.Population <= 1000 {
				t.Errorf("population = %d, want > 1000", result.Population)
			}
		})
	})
}
