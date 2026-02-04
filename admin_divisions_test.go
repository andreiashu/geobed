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

	// Check Germany divisions exist
	deDivs, ok := adminDivisions["DE"]
	if !ok {
		t.Fatal("Germany divisions not found")
	}

	// Check Berlin exists (code 16)
	if _, ok := deDivs["16"]; !ok {
		t.Error("Berlin (16) not found in Germany divisions")
	}

	// Check Great Britain divisions exist
	gbDivs, ok := adminDivisions["GB"]
	if !ok {
		t.Fatal("Great Britain divisions not found")
	}

	// Check England exists (code ENG)
	if _, ok := gbDivs["ENG"]; !ok {
		t.Error("England (ENG) not found in Great Britain divisions")
	}
}

func TestIsAdminDivision(t *testing.T) {
	tests := []struct {
		country  string
		division string
		want     bool
	}{
		{"US", "TX", true},
		{"US", "CA", true},
		{"US", "NY", true},
		{"US", "ZZ", false}, // Invalid US division
		{"CA", "08", true},  // Ontario
		{"AU", "02", true},  // NSW
		{"DE", "16", true},  // Berlin
		{"GB", "ENG", true}, // England
		{"XX", "TX", false}, // Invalid country
	}

	for _, tc := range tests {
		t.Run(tc.country+"_"+tc.division, func(t *testing.T) {
			got := isAdminDivision(tc.country, tc.division)
			if got != tc.want {
				t.Errorf("isAdminDivision(%q, %q) = %v, want %v", tc.country, tc.division, got, tc.want)
			}
		})
	}
}

func TestGetAdminDivisionCountry(t *testing.T) {
	tests := []struct {
		code        string
		wantCountry string
	}{
		{"TX", "US"},  // Unique to US
		{"NY", "US"},  // Unique to US
		{"ENG", "GB"}, // Unique to GB (England)
		// Note: Numeric codes like "08" may exist in multiple countries
		// and should return empty string (ambiguous)
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

func TestGetAdminDivisionName(t *testing.T) {
	tests := []struct {
		country  string
		division string
		wantName string
	}{
		{"US", "TX", "Texas"},
		{"US", "CA", "California"},
		{"GB", "ENG", "England"},
		{"US", "ZZ", ""}, // Invalid
	}

	for _, tc := range tests {
		t.Run(tc.country+"_"+tc.division, func(t *testing.T) {
			got := getAdminDivisionName(tc.country, tc.division)
			if got != tc.wantName {
				t.Errorf("getAdminDivisionName(%q, %q) = %q, want %q", tc.country, tc.division, got, tc.wantName)
			}
		})
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
		// US cities (should still work)
		{"Austin, TX", "US", "Austin"},
		{"Dallas, TX", "US", "Dallas"},
		{"New York, NY", "US", "New York City"},

		// The integration primarily helps when:
		// 1. We know the country and want to validate the region
		// 2. We have a unique region code like TX, NY, ENG
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

func TestAmbiguousAdminDivisionCodes(t *testing.T) {
	// Numeric codes like "01", "02", "08" exist in many countries
	// getAdminDivisionCountry should return empty for ambiguous codes

	ambiguousCodes := []string{"01", "02", "03", "08"}
	for _, code := range ambiguousCodes {
		t.Run(code, func(t *testing.T) {
			result := getAdminDivisionCountry(code)
			if result != "" {
				// Count how many countries have this code
				count := 0
				loadAdminDivisions()
				for _, divs := range adminDivisions {
					if _, ok := divs[code]; ok {
						count++
					}
				}
				if count > 1 {
					t.Errorf("getAdminDivisionCountry(%q) = %q, expected empty for ambiguous code (found in %d countries)", code, result, count)
				}
			}
		})
	}
}
