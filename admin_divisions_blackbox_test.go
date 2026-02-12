package geobed

import (
	"sync"
	"testing"
)

// TestIsAdminDivisionValid tests that known valid admin divisions return true.
func TestIsAdminDivisionValid(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create GeoBed: %v", err)
	}

	tests := []struct {
		name         string
		countryCode  string
		divisionCode string
		want         bool
	}{
		{"US Texas", "US", "TX", true},
		{"US California", "US", "CA", true},
		{"US New York", "US", "NY", true},
		{"Canada Ontario", "CA", "08", true},
		{"Australia NSW", "AU", "02", true},
		{"Germany Berlin", "DE", "16", true},
		{"UK England", "GB", "ENG", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.isAdminDivision(tt.countryCode, tt.divisionCode)
			if got != tt.want {
				t.Errorf("isAdminDivision(%q, %q) = %v, want %v", tt.countryCode, tt.divisionCode, got, tt.want)
			}
		})
	}
}

// TestIsAdminDivisionInvalid tests that invalid admin divisions return false.
func TestIsAdminDivisionInvalid(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create GeoBed: %v", err)
	}

	tests := []struct {
		name         string
		countryCode  string
		divisionCode string
		want         bool
	}{
		{"US invalid division", "US", "ZZ", false},
		{"Invalid country with valid division code", "XX", "TX", false},
		{"Empty strings", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.isAdminDivision(tt.countryCode, tt.divisionCode)
			if got != tt.want {
				t.Errorf("isAdminDivision(%q, %q) = %v, want %v", tt.countryCode, tt.divisionCode, got, tt.want)
			}
		})
	}
}

// TestIsAdminDivisionCaseInsensitive tests that the function handles case insensitivity for division codes.
// Note: Based on black-box testing, only division codes are case-insensitive, not country codes.
func TestIsAdminDivisionCaseInsensitive(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create GeoBed: %v", err)
	}

	tests := []struct {
		name         string
		countryCode  string
		divisionCode string
		want         bool
	}{
		{"US Texas lowercase division", "US", "tx", true},
		{"US Texas mixed case division", "US", "Tx", true},
		{"US New York lowercase division", "US", "ny", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.isAdminDivision(tt.countryCode, tt.divisionCode)
			if got != tt.want {
				t.Errorf("isAdminDivision(%q, %q) = %v, want %v", tt.countryCode, tt.divisionCode, got, tt.want)
			}
		})
	}
}

// TestGetAdminDivisionCountryUnique tests unique admin division codes that map to one country.
func TestGetAdminDivisionCountryUnique(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create GeoBed: %v", err)
	}

	tests := []struct {
		name string
		code string
		want string
	}{
		{"Texas", "TX", "US"},
		{"New York", "NY", "US"},
		{"England", "ENG", "GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.getAdminDivisionCountry(tt.code)
			if got != tt.want {
				t.Errorf("getAdminDivisionCountry(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// TestGetAdminDivisionCountryAmbiguous tests ambiguous codes that exist in multiple countries.
func TestGetAdminDivisionCountryAmbiguous(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create GeoBed: %v", err)
	}

	// Numeric codes like "01", "02", "03", "08" exist in many countries
	ambiguousCodes := []string{"01", "02", "03", "08"}

	for _, code := range ambiguousCodes {
		t.Run("Code_"+code, func(t *testing.T) {
			got := g.getAdminDivisionCountry(code)
			if got != "" {
				t.Errorf("getAdminDivisionCountry(%q) = %q, want empty string for ambiguous code", code, got)
			}
		})
	}
}

// TestGetAdminDivisionCountryEmpty tests empty input.
func TestGetAdminDivisionCountryEmpty(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create GeoBed: %v", err)
	}

	got := g.getAdminDivisionCountry("")
	if got != "" {
		t.Errorf("getAdminDivisionCountry(\"\") = %q, want \"\"", got)
	}
}

// TestGetAdminDivisionNameValid tests retrieving names for valid divisions.
func TestGetAdminDivisionNameValid(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create GeoBed: %v", err)
	}

	tests := []struct {
		name         string
		countryCode  string
		divisionCode string
		want         string
	}{
		{"US Texas", "US", "TX", "Texas"},
		{"US California", "US", "CA", "California"},
		{"UK England", "GB", "ENG", "England"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.getAdminDivisionName(tt.countryCode, tt.divisionCode)
			if got != tt.want {
				t.Errorf("getAdminDivisionName(%q, %q) = %q, want %q", tt.countryCode, tt.divisionCode, got, tt.want)
			}
		})
	}
}

// TestGetAdminDivisionNameInvalid tests that invalid divisions return empty string.
func TestGetAdminDivisionNameInvalid(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create GeoBed: %v", err)
	}

	tests := []struct {
		name         string
		countryCode  string
		divisionCode string
		want         string
	}{
		{"US invalid division", "US", "ZZ", ""},
		{"Invalid country", "XX", "TX", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.getAdminDivisionName(tt.countryCode, tt.divisionCode)
			if got != tt.want {
				t.Errorf("getAdminDivisionName(%q, %q) = %q, want %q", tt.countryCode, tt.divisionCode, got, tt.want)
			}
		})
	}
}

// TestLoadAdminDivisionsForDirRealData tests loading from the actual data directory.
func TestLoadAdminDivisionsForDirRealData(t *testing.T) {
	result := loadAdminDivisionsForDir("./geobed-data")

	// Check that we have data for major countries
	expectedCountries := []string{"US", "CA", "AU", "DE", "GB"}
	for _, country := range expectedCountries {
		t.Run("Country_"+country, func(t *testing.T) {
			divisions, ok := result[country]
			if !ok {
				t.Errorf("loadAdminDivisionsForDir(\"./geobed-data\") missing country %q", country)
				return
			}
			if len(divisions) == 0 {
				t.Errorf("loadAdminDivisionsForDir(\"./geobed-data\") country %q has no divisions", country)
			}
		})
	}

	// Verify some specific divisions exist
	if usDiv, ok := result["US"]; ok {
		if div, ok := usDiv["TX"]; !ok {
			t.Error("Expected US/TX division not found")
		} else if div.Name != "Texas" {
			t.Errorf("US/TX name = %q, want \"Texas\"", div.Name)
		}
	}
}

// TestLoadAdminDivisionsForDirNonexistent tests loading from a nonexistent directory.
func TestLoadAdminDivisionsForDirNonexistent(t *testing.T) {
	// Should return empty map without panicking
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("loadAdminDivisionsForDir panicked on nonexistent dir: %v", r)
		}
	}()

	result := loadAdminDivisionsForDir("/nonexistent/path/that/does/not/exist")
	if result == nil {
		t.Error("loadAdminDivisionsForDir returned nil, want empty map")
	}
	if len(result) != 0 {
		t.Errorf("loadAdminDivisionsForDir(\"/nonexistent\") returned %d entries, want 0", len(result))
	}
}

// TestLoadAdminDivisionsForDirCaching tests that the function caches results.
func TestLoadAdminDivisionsForDirCaching(t *testing.T) {
	dir := "./geobed-data"

	// First call
	result1 := loadAdminDivisionsForDir(dir)
	if len(result1) == 0 {
		t.Skip("No data loaded, skipping cache test")
	}

	// Second call - should return cached data
	result2 := loadAdminDivisionsForDir(dir)

	// Verify both calls returned the same data
	if len(result1) != len(result2) {
		t.Errorf("Cache inconsistency: first call returned %d countries, second returned %d", len(result1), len(result2))
	}

	// Check a specific entry to ensure data is the same
	if us1, ok1 := result1["US"]; ok1 {
		if us2, ok2 := result2["US"]; ok2 {
			if len(us1) != len(us2) {
				t.Errorf("Cache inconsistency for US: first call %d divisions, second call %d divisions", len(us1), len(us2))
			}
		}
	}
}

// TestIsAdminDivisionConcurrent tests concurrent access to isAdminDivision.
func TestIsAdminDivisionConcurrent(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create GeoBed: %v", err)
	}

	// Test data
	testCases := []struct {
		country  string
		division string
		expected bool
	}{
		{"US", "TX", true},
		{"US", "CA", true},
		{"US", "NY", true},
		{"CA", "08", true},
		{"AU", "02", true},
		{"US", "ZZ", false},
		{"XX", "TX", false},
	}

	// Run multiple goroutines concurrently
	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	errors := make(chan error, goroutines*iterations)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				for _, tc := range testCases {
					result := g.isAdminDivision(tc.country, tc.division)
					if result != tc.expected {
						errors <- nil // Signal that we found an inconsistency
						return
					}
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	if len(errors) > 0 {
		t.Error("Concurrent access to isAdminDivision produced inconsistent results")
	}
}

// TestGetAdminDivisionCountryConcurrent tests concurrent access to getAdminDivisionCountry.
func TestGetAdminDivisionCountryConcurrent(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create GeoBed: %v", err)
	}

	// Test data
	testCases := []struct {
		code     string
		expected string
	}{
		{"TX", "US"},
		{"NY", "US"},
		{"ENG", "GB"},
		{"01", ""}, // Ambiguous
		{"02", ""}, // Ambiguous
	}

	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	errors := make(chan error, goroutines*iterations)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				for _, tc := range testCases {
					result := g.getAdminDivisionCountry(tc.code)
					if result != tc.expected {
						errors <- nil
						return
					}
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	if len(errors) > 0 {
		t.Error("Concurrent access to getAdminDivisionCountry produced inconsistent results")
	}
}

// TestGetAdminDivisionNameConcurrent tests concurrent access to getAdminDivisionName.
func TestGetAdminDivisionNameConcurrent(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatalf("Failed to create GeoBed: %v", err)
	}

	// Test data
	testCases := []struct {
		country  string
		division string
		expected string
	}{
		{"US", "TX", "Texas"},
		{"US", "CA", "California"},
		{"GB", "ENG", "England"},
		{"US", "ZZ", ""},
		{"XX", "TX", ""},
	}

	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	errors := make(chan error, goroutines*iterations)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				for _, tc := range testCases {
					result := g.getAdminDivisionName(tc.country, tc.division)
					if result != tc.expected {
						errors <- nil
						return
					}
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	if len(errors) > 0 {
		t.Error("Concurrent access to getAdminDivisionName produced inconsistent results")
	}
}

// TestLoadAdminDivisionsForDirConcurrent tests concurrent loading from multiple directories.
func TestLoadAdminDivisionsForDirConcurrent(t *testing.T) {
	const goroutines = 20
	const iterations = 10

	var wg sync.WaitGroup

	// Test concurrent access to the same directory
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				result := loadAdminDivisionsForDir("./geobed-data")
				// Just ensure it doesn't panic and returns a map
				if result == nil {
					t.Error("loadAdminDivisionsForDir returned nil")
					return
				}
			}
		}()
	}

	wg.Wait()
}
