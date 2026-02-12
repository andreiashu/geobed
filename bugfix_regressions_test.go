package geobed

import (
	"testing"
)

func TestBugfixRegressions(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	// ──────────────────────────────────────────────
	// Bug 1: Alt names were split on whitespace instead of commas
	// ──────────────────────────────────────────────

	t.Run("AltNameCommaSplitFix", func(t *testing.T) {
		tests := []struct {
			query       string
			wantCity    string
			wantCountry string
		}{
			{"Bombay", "Mumbai", "IN"},
			{"Peking", "Beijing", "CN"},
			{"Constantinople", "Istanbul", "TR"},
			{"Saigon", "Ho Chi Minh City", "VN"},
			{"Rangoon", "Yangon", "MM"},
		}
		for _, tt := range tests {
			t.Run(tt.query, func(t *testing.T) {
				result := g.Geocode(tt.query)
				if result.City != tt.wantCity {
					t.Errorf("Geocode(%q) city = %q, want %q", tt.query, result.City, tt.wantCity)
				}
				if result.Country() != tt.wantCountry {
					t.Errorf("Geocode(%q) country = %q, want %q", tt.query, result.Country(), tt.wantCountry)
				}
			})
		}

		t.Run("Tokio→Tokyo", func(t *testing.T) {
			result := g.Geocode("Tokio")
			if result.City != "Tokyo" {
				t.Errorf("Geocode('Tokio') city = %q, want 'Tokyo'", result.City)
			}
			if result.Country() != "JP" {
				t.Errorf("Geocode('Tokio') country = %q, want 'JP'", result.Country())
			}
		})

		t.Run("Londra→London", func(t *testing.T) {
			result := g.Geocode("Londra")
			if result.City != "London" {
				t.Errorf("Geocode('Londra') city = %q, want 'London'", result.City)
			}
			if result.Country() != "GB" {
				t.Errorf("Geocode('Londra') country = %q, want 'GB'", result.Country())
			}
		})

		t.Run("München→Munich", func(t *testing.T) {
			result := g.Geocode("München")
			if result.City != "Munich" {
				t.Errorf("Geocode('München') city = %q, want 'Munich'", result.City)
			}
			if result.Country() != "DE" {
				t.Errorf("Geocode('München') country = %q, want 'DE'", result.Country())
			}
		})
	})

	// ──────────────────────────────────────────────
	// Bug 2: Search range only covered primary names by first character.
	// Alt names starting with a different letter than the primary name
	// were never found. The inverted index fixes this.
	// ──────────────────────────────────────────────

	t.Run("InvertedIndexCrossLetterAltNameLookup", func(t *testing.T) {
		t.Run("Bombay_B_finds_Mumbai_M", func(t *testing.T) {
			result := g.Geocode("Bombay")
			if result.City != "Mumbai" {
				t.Errorf("Geocode('Bombay') city = %q, want 'Mumbai'", result.City)
			}
		})

		t.Run("Peking_P_finds_Beijing_B", func(t *testing.T) {
			result := g.Geocode("Peking")
			if result.City != "Beijing" {
				t.Errorf("Geocode('Peking') city = %q, want 'Beijing'", result.City)
			}
		})

		t.Run("Constantinople_C_finds_Istanbul_I", func(t *testing.T) {
			result := g.Geocode("Constantinople")
			if result.City != "Istanbul" {
				t.Errorf("Geocode('Constantinople') city = %q, want 'Istanbul'", result.City)
			}
		})

		t.Run("Saigon_S_finds_HoChiMinhCity_H", func(t *testing.T) {
			result := g.Geocode("Saigon")
			if result.City != "Ho Chi Minh City" {
				t.Errorf("Geocode('Saigon') city = %q, want 'Ho Chi Minh City'", result.City)
			}
		})

		t.Run("nameIndex_contains_primary_and_alt_names", func(t *testing.T) {
			altNames := []string{"bombay", "peking", "constantinople"}
			for _, name := range altNames {
				if _, ok := g.nameIndex[name]; !ok {
					t.Errorf("nameIndex missing alt name key %q", name)
				}
			}
			primaryNames := []string{"mumbai", "beijing", "istanbul"}
			for _, name := range primaryNames {
				if _, ok := g.nameIndex[name]; !ok {
					t.Errorf("nameIndex missing primary name key %q", name)
				}
			}
		})

		t.Run("nameIndex_keys_are_all_lowercase", func(t *testing.T) {
			count := 0
			for key := range g.nameIndex {
				if key != toLower(key) {
					t.Errorf("nameIndex key %q is not lowercase", key)
				}
				count++
				if count >= 1000 {
					break
				}
			}
		})
	})

	// ──────────────────────────────────────────────
	// Bug 3: No-match returned cities[0] instead of empty.
	// ──────────────────────────────────────────────

	t.Run("NoMatchReturnsEmptyCity", func(t *testing.T) {
		nonsense := []string{
			"Zxqwvbn",
			"Xyzpdq123",
			"Qqqqqqq",
			"!@#$%^",
			"99999",
		}
		for _, query := range nonsense {
			t.Run(query, func(t *testing.T) {
				result := g.Geocode(query)
				if result.City != "" {
					t.Errorf("Geocode(%q) city = %q, want empty", query, result.City)
				}
				if result.Population != 0 {
					t.Errorf("Geocode(%q) population = %d, want 0", query, result.Population)
				}
				if result.Latitude != 0 {
					t.Errorf("Geocode(%q) latitude = %v, want 0", query, result.Latitude)
				}
				if result.Longitude != 0 {
					t.Errorf("Geocode(%q) longitude = %v, want 0", query, result.Longitude)
				}
			})
		}

		t.Run("empty_and_whitespace_return_empty", func(t *testing.T) {
			for _, q := range []string{"", "   ", "\t\n"} {
				result := g.Geocode(q)
				if result.City != "" {
					t.Errorf("Geocode(%q) city = %q, want empty", q, result.City)
				}
			}
		})

		t.Run("nonsense_does_not_return_first_city", func(t *testing.T) {
			nonsenseResult := g.Geocode("Zxqwvbn")
			firstCity := g.Cities[0]
			if nonsenseResult.City == firstCity.City && nonsenseResult.City != "" {
				t.Errorf("Geocode('Zxqwvbn') returned cities[0] = %q, should return empty", firstCity.City)
			}
		})
	})

	// ──────────────────────────────────────────────
	// Bug 4: Reverse geocode returned neighborhoods instead of cities.
	// ──────────────────────────────────────────────

	t.Run("ReverseGeocodeNeighborhoodOverride", func(t *testing.T) {
		t.Run("Berlin_not_Mitte", func(t *testing.T) {
			result := g.ReverseGeocode(52.52, 13.405)
			if result.City != "Berlin" {
				t.Errorf("ReverseGeocode(52.52, 13.405) city = %q, want 'Berlin'", result.City)
			}
			if result.Country() != "DE" {
				t.Errorf("ReverseGeocode(52.52, 13.405) country = %q, want 'DE'", result.Country())
			}
			if result.Population <= 1_000_000 {
				t.Errorf("ReverseGeocode(52.52, 13.405) population = %d, want > 1M", result.Population)
			}
		})

		t.Run("Paris_not_neighborhood", func(t *testing.T) {
			result := g.ReverseGeocode(48.8566, 2.3522)
			if result.City != "Paris" {
				t.Errorf("ReverseGeocode(48.8566, 2.3522) city = %q, want 'Paris'", result.City)
			}
			if result.Country() != "FR" {
				t.Errorf("ReverseGeocode(48.8566, 2.3522) country = %q, want 'FR'", result.Country())
			}
		})

		t.Run("Cairo_not_neighborhood", func(t *testing.T) {
			result := g.ReverseGeocode(30.0444, 31.2357)
			if result.City != "Cairo" {
				t.Errorf("ReverseGeocode(30.0444, 31.2357) city = %q, want 'Cairo'", result.City)
			}
			if result.Country() != "EG" {
				t.Errorf("ReverseGeocode(30.0444, 31.2357) country = %q, want 'EG'", result.Country())
			}
		})

		t.Run("small_offsets_near_Berlin", func(t *testing.T) {
			center := g.ReverseGeocode(52.52, 13.405)
			offset := g.ReverseGeocode(52.52, 13.419) // ~1km east
			if center.City != "Berlin" {
				t.Errorf("center city = %q, want 'Berlin'", center.City)
			}
			if offset.City != "Berlin" {
				t.Errorf("offset city = %q, want 'Berlin'", offset.City)
			}
		})

		t.Run("small_offsets_near_Paris", func(t *testing.T) {
			center := g.ReverseGeocode(48.8566, 2.3522)
			offset := g.ReverseGeocode(48.8656, 2.3522) // ~1km north
			if center.City != "Paris" {
				t.Errorf("center city = %q, want 'Paris'", center.City)
			}
			if offset.City != "Paris" {
				t.Errorf("offset city = %q, want 'Paris'", offset.City)
			}
		})
	})

	// ──────────────────────────────────────────────
	// Bug 5: Reverse geocode returned results for remote locations.
	// ──────────────────────────────────────────────

	t.Run("ReverseGeocodeMaxDistanceCutoff", func(t *testing.T) {
		remoteLocations := []struct {
			name     string
			lat, lng float64
		}{
			{"North Pole", 90, 0},
			{"South Pole", -90, 0},
			{"mid-Pacific", 0, -160},
			{"mid-Atlantic", 30, -40},
			{"Antarctic", -75, 0},
		}
		for _, loc := range remoteLocations {
			t.Run(loc.name, func(t *testing.T) {
				result := g.ReverseGeocode(loc.lat, loc.lng)
				if result.City != "" {
					t.Errorf("ReverseGeocode(%v, %v) city = %q, want empty", loc.lat, loc.lng, result.City)
				}
			})
		}

		t.Run("locations_near_cities_still_work", func(t *testing.T) {
			result := g.ReverseGeocode(30.2672, -97.7431)
			if result.City != "Austin" {
				t.Errorf("ReverseGeocode(30.2672, -97.7431) city = %q, want 'Austin'", result.City)
			}
			if result.Country() != "US" {
				t.Errorf("ReverseGeocode(30.2672, -97.7431) country = %q, want 'US'", result.Country())
			}
		})
	})

	// ──────────────────────────────────────────────
	// Deterministic results
	// ──────────────────────────────────────────────

	t.Run("DeterministicResults", func(t *testing.T) {
		t.Run("geocode_same_result_every_time", func(t *testing.T) {
			r1 := g.Geocode("Paris")
			r2 := g.Geocode("Paris")
			if r1.City != r2.City {
				t.Errorf("non-deterministic: city %q vs %q", r1.City, r2.City)
			}
			if r1.Latitude != r2.Latitude || r1.Longitude != r2.Longitude {
				t.Error("non-deterministic: coordinates differ")
			}
		})

		t.Run("reverseGeocode_same_result_every_time", func(t *testing.T) {
			r1 := g.ReverseGeocode(48.8566, 2.3522)
			r2 := g.ReverseGeocode(48.8566, 2.3522)
			if r1.City != r2.City {
				t.Errorf("non-deterministic: city %q vs %q", r1.City, r2.City)
			}
		})
	})
}
