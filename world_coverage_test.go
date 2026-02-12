package geobed

import (
	"testing"
)

func TestWorldCoverage(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create Geobed: %v", err)
	}

	// ──────────────────────────────────────────────
	// Africa
	// ──────────────────────────────────────────────

	t.Run("Africa", func(t *testing.T) {
		tests := []struct {
			query       string
			wantCity    string
			wantCountry string
		}{
			{"Lagos", "Lagos", "NG"},
			{"Nairobi", "Nairobi", "KE"},
			{"Cairo, Egypt", "Cairo", "EG"},
			{"Johannesburg", "Johannesburg", "ZA"},
			{"Addis Ababa", "Addis Ababa", "ET"},
			{"Kinshasa", "Kinshasa", "CD"},
			{"Casablanca", "Casablanca", "MA"},
			{"Dar es Salaam", "Dar es Salaam", "TZ"},
			{"Accra", "Accra", "GH"},
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
				if result.Population <= 0 {
					t.Errorf("Geocode(%q) population = %d, want > 0", tt.query, result.Population)
				}
			})
		}

		t.Run("Cairo_without_qualifier", func(t *testing.T) {
			result := g.Geocode("Cairo")
			if result.City != "Cairo" {
				t.Errorf("city = %q, want 'Cairo'", result.City)
			}
			// Cairo exists in both EG and US
			country := result.Country()
			if country != "EG" && country != "US" {
				t.Errorf("country = %q, want 'EG' or 'US'", country)
			}
		})
	})

	// ──────────────────────────────────────────────
	// South America
	// ──────────────────────────────────────────────

	t.Run("SouthAmerica", func(t *testing.T) {
		t.Run("São Paulo", func(t *testing.T) {
			result := g.Geocode("São Paulo")
			if result.City != "São Paulo" {
				t.Errorf("city = %q, want 'São Paulo'", result.City)
			}
			if result.Country() != "BR" {
				t.Errorf("country = %q, want 'BR'", result.Country())
			}
		})

		t.Run("Buenos Aires", func(t *testing.T) {
			result := g.Geocode("Buenos Aires")
			if result.City != "Buenos Aires" {
				t.Errorf("city = %q, want 'Buenos Aires'", result.City)
			}
			if result.Country() != "AR" {
				t.Errorf("country = %q, want 'AR'", result.Country())
			}
		})

		t.Run("Lima, Peru", func(t *testing.T) {
			result := g.Geocode("Lima, Peru")
			if result.City != "Lima" {
				t.Errorf("city = %q, want 'Lima'", result.City)
			}
			if result.Country() != "PE" {
				t.Errorf("country = %q, want 'PE'", result.Country())
			}
		})

		t.Run("Lima_documents_behavior", func(t *testing.T) {
			result := g.Geocode("Lima")
			if result.City != "Lima" {
				t.Errorf("city = %q, want 'Lima'", result.City)
			}
			// Lima without qualifier may return different countries
			if result.Country() == "" {
				t.Error("expected non-empty country")
			}
		})

		t.Run("Bogota, Colombia", func(t *testing.T) {
			result := g.Geocode("Bogota, Colombia")
			if result.Country() != "CO" {
				t.Errorf("country = %q, want 'CO'", result.Country())
			}
		})

		t.Run("Santiago, Chile", func(t *testing.T) {
			result := g.Geocode("Santiago, Chile")
			if result.City != "Santiago" {
				t.Errorf("city = %q, want 'Santiago'", result.City)
			}
			if result.Country() != "CL" {
				t.Errorf("country = %q, want 'CL'", result.Country())
			}
		})

		t.Run("Caracas", func(t *testing.T) {
			result := g.Geocode("Caracas")
			if result.City != "Caracas" {
				t.Errorf("city = %q, want 'Caracas'", result.City)
			}
			if result.Country() != "VE" {
				t.Errorf("country = %q, want 'VE'", result.Country())
			}
		})

		t.Run("Quito", func(t *testing.T) {
			result := g.Geocode("Quito")
			if result.City != "Quito" {
				t.Errorf("city = %q, want 'Quito'", result.City)
			}
			if result.Country() != "EC" {
				t.Errorf("country = %q, want 'EC'", result.Country())
			}
		})
	})

	// ──────────────────────────────────────────────
	// Middle East
	// ──────────────────────────────────────────────

	t.Run("MiddleEast", func(t *testing.T) {
		tests := []struct {
			query       string
			wantCity    string
			wantCountry string
		}{
			{"Dubai", "Dubai", "AE"},
			{"Riyadh", "Riyadh", "SA"},
			{"Tehran", "Tehran", "IR"},
			{"Baghdad", "Baghdad", "IQ"},
			{"Doha", "Doha", "QA"},
			{"Amman", "Amman", "JO"},
			{"Beirut", "Beirut", "LB"},
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
				if result.Population <= 0 {
					t.Errorf("Geocode(%q) population = %d, want > 0", tt.query, result.Population)
				}
			})
		}
	})

	// ──────────────────────────────────────────────
	// Southeast Asia
	// ──────────────────────────────────────────────

	t.Run("SoutheastAsia", func(t *testing.T) {
		tests := []struct {
			query       string
			wantCity    string
			wantCountry string
		}{
			{"Jakarta", "Jakarta", "ID"},
			{"Manila", "Manila", "PH"},
			{"Hanoi", "Hanoi", "VN"},
			{"Bangkok", "Bangkok", "TH"},
			{"Singapore", "Singapore", "SG"},
			{"Kuala Lumpur", "Kuala Lumpur", "MY"},
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
				if result.Population <= 0 {
					t.Errorf("Geocode(%q) population = %d, want > 0", tt.query, result.Population)
				}
			})
		}

		t.Run("Bangkok, Thailand", func(t *testing.T) {
			result := g.Geocode("Bangkok, Thailand")
			if result.City != "Bangkok" {
				t.Errorf("city = %q, want 'Bangkok'", result.City)
			}
			if result.Country() != "TH" {
				t.Errorf("country = %q, want 'TH'", result.Country())
			}
		})
	})

	// ──────────────────────────────────────────────
	// Other regions
	// ──────────────────────────────────────────────

	t.Run("OtherRegions", func(t *testing.T) {
		tests := []struct {
			query       string
			wantCity    string
			wantCountry string
		}{
			{"Auckland", "Auckland", "NZ"},
			{"Melbourne", "Melbourne", "AU"},
			{"Almaty", "Almaty", "KZ"},
			{"Tashkent", "Tashkent", "UZ"},
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
				if result.Population <= 0 {
					t.Errorf("Geocode(%q) population = %d, want > 0", tt.query, result.Population)
				}
			})
		}
	})

	// ──────────────────────────────────────────────
	// East Asia
	// ──────────────────────────────────────────────

	t.Run("EastAsia", func(t *testing.T) {
		tests := []struct {
			query       string
			wantCity    string
			wantCountry string
		}{
			{"Osaka", "Osaka", "JP"},
			{"Shanghai", "Shanghai", "CN"},
			{"Hong Kong", "Hong Kong", "HK"},
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
				if result.Population <= 0 {
					t.Errorf("Geocode(%q) population = %d, want > 0", tt.query, result.Population)
				}
			})
		}
	})

	// ──────────────────────────────────────────────
	// Europe
	// ──────────────────────────────────────────────

	t.Run("Europe", func(t *testing.T) {
		tests := []struct {
			query       string
			wantCity    string
			wantCountry string
		}{
			{"Rome", "Rome", "IT"},
			{"Madrid", "Madrid", "ES"},
			{"Amsterdam", "Amsterdam", "NL"},
			{"Warsaw", "Warsaw", "PL"},
			{"Stockholm", "Stockholm", "SE"},
			{"Helsinki", "Helsinki", "FI"},
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
				if result.Population <= 0 {
					t.Errorf("Geocode(%q) population = %d, want > 0", tt.query, result.Population)
				}
			})
		}
	})
}
