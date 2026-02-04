package geobed

import (
	"testing"
)

func TestNewYorkGeocoding(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query    string
		wantCity string
		wantState string
	}{
		{"New York", "New York", "NY"},
		{"New York, NY", "New York City", "NY"},
		{"New York City", "New York City", "NY"},
		{"Austin, TX", "Austin", "TX"},
		{"Paris", "Paris", "FR"},  // Uses region as country for non-US
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			r := g.Geocode(tt.query)
			if r.City != tt.wantCity {
				t.Errorf("Geocode(%q) city = %q, want %q", tt.query, r.City, tt.wantCity)
			}
			// Check state for US cities, country for others
			if tt.wantState == "FR" {
				if r.Country() != tt.wantState {
					t.Errorf("Geocode(%q) country = %q, want %q", tt.query, r.Country(), tt.wantState)
				}
			} else if r.Region() != tt.wantState {
				t.Errorf("Geocode(%q) region = %q, want %q", tt.query, r.Region(), tt.wantState)
			}
		})
	}
}
