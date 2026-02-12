package geobed

import (
	"testing"
)

func TestReverseGeocodeCorrectness(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	// ──────────────────────────────────────────────
	// Remote locations → empty city
	// ──────────────────────────────────────────────

	t.Run("RemoteLocations_EmptyCity", func(t *testing.T) {
		remotes := []struct {
			name     string
			lat, lng float64
		}{
			{"Middle of Pacific", 0, -160},
			{"North Pole", 90, 0},
			{"South Pole", -90, 0},
			{"Mid Atlantic", 30, -40},
			{"Antarctic", -75, 0},
		}
		for _, tt := range remotes {
			t.Run(tt.name, func(t *testing.T) {
				result := g.ReverseGeocode(tt.lat, tt.lng)
				if result.City != "" {
					t.Errorf("ReverseGeocode(%v, %v) city = %q, want empty", tt.lat, tt.lng, result.City)
				}
			})
		}

		t.Run("Gulf_of_Guinea_0_0", func(t *testing.T) {
			result := g.ReverseGeocode(0, 0)
			// May be empty or find a coastal city — document behavior
			if result.City != "" {
				if result.Population < 0 {
					t.Error("population should be >= 0")
				}
			}
		})
	})

	// ──────────────────────────────────────────────
	// World cities by coordinates
	// ──────────────────────────────────────────────

	t.Run("WorldCitiesByCoordinates", func(t *testing.T) {
		tests := []struct {
			name        string
			lat, lng    float64
			wantCity    string
			wantCountry string
		}{
			{"Paris", 48.8566, 2.3522, "Paris", "FR"},
			{"Berlin", 52.5200, 13.4050, "Berlin", "DE"},
			{"Lagos", 6.4541, 3.3947, "Lagos", "NG"},
			{"Nairobi", -1.2864, 36.8172, "Nairobi", "KE"},
			{"Buenos Aires", -34.6037, -58.3816, "Buenos Aires", "AR"},
			{"New York City", 40.7128, -74.0060, "New York City", "US"},
			{"Moscow", 55.7558, 37.6173, "Moscow", "RU"},
			{"Beijing", 39.9042, 116.4074, "Beijing", "CN"},
			{"Seoul", 37.5665, 126.9780, "Seoul", "KR"},
			{"Mumbai", 19.0760, 72.8777, "Mumbai", "IN"},
			{"Sydney", -33.8688, 151.2093, "Sydney", "AU"},
			{"Austin", 30.2672, -97.7431, "Austin", "US"},
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
	})

	// ──────────────────────────────────────────────
	// Tokyo area
	// ──────────────────────────────────────────────

	t.Run("TokyoArea", func(t *testing.T) {
		result := g.ReverseGeocode(35.6762, 139.6503)
		if result.Country() != "JP" {
			t.Errorf("country = %q, want 'JP'", result.Country())
		}
		if result.City == "" {
			t.Error("expected non-empty city near Tokyo")
		}
	})

	// ──────────────────────────────────────────────
	// Jakarta area
	// ──────────────────────────────────────────────

	t.Run("JakartaArea", func(t *testing.T) {
		result := g.ReverseGeocode(-6.2088, 106.8456)
		if result.Country() != "ID" {
			t.Errorf("country = %q, want 'ID'", result.Country())
		}
		if result.City == "" {
			t.Error("expected non-empty city near Jakarta")
		}
	})

	// ──────────────────────────────────────────────
	// Precision: ~1km offset should still return same city
	// ──────────────────────────────────────────────

	t.Run("SmallOffsets", func(t *testing.T) {
		t.Run("Paris_1km_north", func(t *testing.T) {
			center := g.ReverseGeocode(48.8566, 2.3522)
			offset := g.ReverseGeocode(48.8656, 2.3522)
			if center.City != "Paris" {
				t.Errorf("center city = %q, want 'Paris'", center.City)
			}
			if offset.City != "Paris" {
				t.Errorf("offset city = %q, want 'Paris'", offset.City)
			}
		})

		t.Run("Berlin_1km_east", func(t *testing.T) {
			center := g.ReverseGeocode(52.5200, 13.4050)
			offset := g.ReverseGeocode(52.5200, 13.4190)
			if center.City != "Berlin" {
				t.Errorf("center city = %q, want 'Berlin'", center.City)
			}
			if offset.City != "Berlin" {
				t.Errorf("offset city = %q, want 'Berlin'", offset.City)
			}
		})

		t.Run("Austin_1km_south", func(t *testing.T) {
			center := g.ReverseGeocode(30.2672, -97.7431)
			offset := g.ReverseGeocode(30.2582, -97.7431)
			if center.City != "Austin" {
				t.Errorf("center city = %q, want 'Austin'", center.City)
			}
			if offset.City != "Austin" {
				t.Errorf("offset city = %q, want 'Austin'", offset.City)
			}
		})

		t.Run("Moscow_1km_west", func(t *testing.T) {
			center := g.ReverseGeocode(55.7558, 37.6173)
			offset := g.ReverseGeocode(55.7558, 37.6023)
			if center.City != "Moscow" {
				t.Errorf("center city = %q, want 'Moscow'", center.City)
			}
			if offset.City != "Moscow" {
				t.Errorf("offset city = %q, want 'Moscow'", offset.City)
			}
		})
	})

	// ──────────────────────────────────────────────
	// Boundary coordinates
	// ──────────────────────────────────────────────

	t.Run("BoundaryCoordinates", func(t *testing.T) {
		t.Run("date_line_0_180", func(t *testing.T) {
			result := g.ReverseGeocode(0, 180)
			if result.City != "" {
				if result.Population < 0 {
					t.Error("population should be >= 0")
				}
			}
		})

		t.Run("date_line_0_neg180", func(t *testing.T) {
			result := g.ReverseGeocode(0, -180)
			if result.City != "" {
				if result.Population < 0 {
					t.Error("population should be >= 0")
				}
			}
		})

		t.Run("extreme_neg90_180", func(t *testing.T) {
			result := g.ReverseGeocode(-90, 180)
			if result.City != "" {
				t.Errorf("city = %q, want empty", result.City)
			}
		})
	})

	// ──────────────────────────────────────────────
	// Additional known locations
	// ──────────────────────────────────────────────

	t.Run("AdditionalKnownLocations", func(t *testing.T) {
		tests := []struct {
			name        string
			lat, lng    float64
			wantCity    string
			wantCountry string
		}{
			{"Palo Alto", 37.44651, -122.15322, "Palo Alto", "US"},
			{"Santa Cruz", 36.9741, -122.0308, "Santa Cruz", "US"},
			{"London", 51.5074, -0.1278, "London", "GB"},
			{"Los Angeles", 34.0522, -118.2437, "Los Angeles", "US"},
			{"Cairo", 30.0444, 31.2357, "Cairo", "EG"},
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
	})
}
