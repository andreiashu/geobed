package geobed

import (
	"testing"
)

func TestLoadAdminDivisions(t *testing.T) {
	loadAdminDivisions()

	// Check that we loaded some divisions
	if len(adminDivisions) == 0 {
		t.Fatal("adminDivisions map is empty")
	}

	// Check US divisions exist
	usDivs, ok := adminDivisions["US"]
	if !ok {
		t.Fatal("US divisions not found")
	}

	// Check Texas exists
	if _, ok := usDivs["TX"]; !ok {
		t.Error("Texas (TX) not found in US divisions")
	}

	// Check Canada divisions exist
	caDivs, ok := adminDivisions["CA"]
	if !ok {
		t.Fatal("Canada divisions not found")
	}

	// Check Ontario exists (code 08)
	if _, ok := caDivs["08"]; !ok {
		t.Error("Ontario (08) not found in Canada divisions")
	}

	// Check Australia divisions exist
	auDivs, ok := adminDivisions["AU"]
	if !ok {
		t.Fatal("Australia divisions not found")
	}

	// Check New South Wales exists (code 02)
	if _, ok := auDivs["02"]; !ok {
		t.Error("New South Wales (02) not found in Australia divisions")
	}
}

func TestInternationalAdminDivisions(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query       string
		wantCountry string
		wantCity    string
	}{
		{"Austin, TX", "US", "Austin"},
		{"Dallas, TX", "US", "Dallas"},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			result := g.Geocode(tc.query)
			if result.Country() != tc.wantCountry {
				t.Errorf("Geocode(%q) country = %q, want %q", tc.query, result.Country(), tc.wantCountry)
			}
			if tc.wantCity != "" && result.City != tc.wantCity {
				t.Errorf("Geocode(%q) city = %q, want %q", tc.query, result.City, tc.wantCity)
			}
		})
	}
}

func TestGetAdminDivisionCountry(t *testing.T) {
	tests := []struct {
		code        string
		wantCountry string
	}{
		{"TX", "US"},
		{"NY", "US"},
	}

	for _, tc := range tests {
		t.Run(tc.code, func(t *testing.T) {
			got := getAdminDivisionCountry(tc.code)
			if got != tc.wantCountry {
				t.Errorf("getAdminDivisionCountry(%q) = %q, want %q", tc.code, got, tc.wantCountry)
			}
		})
	}
}
