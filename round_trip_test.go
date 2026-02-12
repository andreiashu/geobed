package geobed

import (
	"testing"
)

func TestRoundTrip(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	// ──────────────────────────────────────────────
	// Round-trip: geocode → reverseGeocode → same city
	// ──────────────────────────────────────────────

	t.Run("GeocodeThenReverseGeocode", func(t *testing.T) {
		queries := []string{
			"Tokyo",
			"Paris",
			"Berlin",
			"Sydney",
			"New York, NY",
			"Lagos",
			"Cairo",
			"Moscow",
			"Nairobi",
			"Austin, TX",
			"San Francisco, CA",
			"London",
			"Seoul",
			"Mumbai",
			"Beijing",
		}
		for _, query := range queries {
			t.Run(query, func(t *testing.T) {
				fwd := g.Geocode(query)
				if fwd.City == "" {
					t.Fatalf("Geocode(%q) returned empty city", query)
				}

				rev := g.ReverseGeocode(float64(fwd.Latitude), float64(fwd.Longitude))
				if rev.City != fwd.City {
					t.Errorf("round-trip: Geocode(%q)=%q → ReverseGeocode=%q, want same city",
						query, fwd.City, rev.City)
				}
				if rev.Country() != fwd.Country() {
					t.Errorf("round-trip: Geocode(%q) country=%q → ReverseGeocode country=%q, want same",
						query, fwd.Country(), rev.Country())
				}
			})
		}
	})

	// ──────────────────────────────────────────────
	// Coordinate reasonableness checks
	// ──────────────────────────────────────────────

	t.Run("CoordinateReasonableness", func(t *testing.T) {
		tests := []struct {
			name                         string
			query                        string
			minLat, maxLat, minLng, maxLng float64
		}{
			{"Tokyo", "Tokyo", 35, 36, 139, 140},
			{"Sydney", "Sydney", -34, -33, 151, 152},
			{"Lagos", "Lagos", 6, 7, 3, 4},
			{"Paris", "Paris", 48, 49, 2, 3},
			{"Berlin", "Berlin", 52, 53, 13, 14},
			{"Moscow", "Moscow", 55, 56, 37, 38},
			{"Buenos Aires", "Buenos Aires", -35, -34, -59, -58},
			{"Nairobi", "Nairobi", -2, -1, 36, 37},
			{"Austin, TX", "Austin, TX", 30, 31, -98, -97},
			{"San Francisco", "San Francisco", 37, 38, -123, -122},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := g.Geocode(tt.query)
				lat := float64(result.Latitude)
				lng := float64(result.Longitude)
				if lat < tt.minLat || lat > tt.maxLat {
					t.Errorf("Geocode(%q) lat = %v, want [%v, %v]", tt.query, lat, tt.minLat, tt.maxLat)
				}
				if lng < tt.minLng || lng > tt.maxLng {
					t.Errorf("Geocode(%q) lng = %v, want [%v, %v]", tt.query, lng, tt.minLng, tt.maxLng)
				}
			})
		}
	})

	// ──────────────────────────────────────────────
	// Deterministic: same query → same result
	// ──────────────────────────────────────────────

	t.Run("Deterministic", func(t *testing.T) {
		t.Run("geocode_Paris_consistent", func(t *testing.T) {
			r1 := g.Geocode("Paris")
			r2 := g.Geocode("Paris")
			if r1.City != r2.City {
				t.Errorf("non-deterministic: %q vs %q", r1.City, r2.City)
			}
			if r1.Latitude != r2.Latitude || r1.Longitude != r2.Longitude {
				t.Error("non-deterministic: coordinates differ")
			}
			if r1.Country() != r2.Country() {
				t.Errorf("non-deterministic: country %q vs %q", r1.Country(), r2.Country())
			}
		})

		t.Run("reverseGeocode_consistent", func(t *testing.T) {
			r1 := g.ReverseGeocode(48.8566, 2.3522)
			r2 := g.ReverseGeocode(48.8566, 2.3522)
			if r1.City != r2.City {
				t.Errorf("non-deterministic: %q vs %q", r1.City, r2.City)
			}
			if r1.Latitude != r2.Latitude || r1.Longitude != r2.Longitude {
				t.Error("non-deterministic: coordinates differ")
			}
		})
	})
}
