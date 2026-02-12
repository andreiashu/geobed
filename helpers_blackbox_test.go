package geobed

import (
	"sort"
	"testing"
)

// TestToLower tests the toLower function with various inputs
func TestToLower(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple uppercase",
			input: "NYC",
			want:  "nyc",
		},
		{
			name:  "mixed case with spaces",
			input: "Hello World",
			want:  "hello world",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "unicode - German umlaut",
			input: "Zürich",
			want:  "zürich",
		},
		{
			name:  "unicode - German city",
			input: "MÜNCHEN",
			want:  "münchen",
		},
		{
			name:  "CJK characters - no case conversion",
			input: "東京",
			want:  "東京",
		},
		{
			name:  "already lowercase",
			input: "already lowercase",
			want:  "already lowercase",
		},
		{
			name:  "mixed unicode and ASCII",
			input: "CAFÉ",
			want:  "café",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toLower(tt.input)
			if got != tt.want {
				t.Errorf("toLower(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestToUpper tests the toUpper function with various inputs
func TestToUpper(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple lowercase",
			input: "nyc",
			want:  "NYC",
		},
		{
			name:  "simple word",
			input: "hello",
			want:  "HELLO",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "unicode - German umlaut",
			input: "zürich",
			want:  "ZÜRICH",
		},
		{
			name:  "already uppercase",
			input: "ALREADY UPPERCASE",
			want:  "ALREADY UPPERCASE",
		},
		{
			name:  "mixed case",
			input: "MiXeD",
			want:  "MIXED",
		},
		{
			name:  "unicode - French accents",
			input: "café",
			want:  "CAFÉ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toUpper(tt.input)
			if got != tt.want {
				t.Errorf("toUpper(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestCompareCaseInsensitive tests case-insensitive string comparison
func TestCompareCaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		wantSign string // "negative", "zero", "positive"
	}{
		{
			name:     "a less than b",
			a:        "a",
			b:        "b",
			wantSign: "negative",
		},
		{
			name:     "b greater than a",
			a:        "b",
			b:        "a",
			wantSign: "positive",
		},
		{
			name:     "equal case-insensitive",
			a:        "abc",
			b:        "ABC",
			wantSign: "zero",
		},
		{
			name:     "both empty strings",
			a:        "",
			b:        "",
			wantSign: "zero",
		},
		{
			name:     "unicode equal case-insensitive",
			a:        "Zürich",
			b:        "zürich",
			wantSign: "zero",
		},
		{
			name:     "apple less than banana",
			a:        "apple",
			b:        "Banana",
			wantSign: "negative",
		},
		{
			name:     "exact same string",
			a:        "identical",
			b:        "identical",
			wantSign: "zero",
		},
		{
			name:     "prefix comparison",
			a:        "test",
			b:        "testing",
			wantSign: "negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareCaseInsensitive(tt.a, tt.b)

			switch tt.wantSign {
			case "negative":
				if got >= 0 {
					t.Errorf("compareCaseInsensitive(%q, %q) = %d, want negative", tt.a, tt.b, got)
				}
			case "zero":
				if got != 0 {
					t.Errorf("compareCaseInsensitive(%q, %q) = %d, want 0", tt.a, tt.b, got)
				}
			case "positive":
				if got <= 0 {
					t.Errorf("compareCaseInsensitive(%q, %q) = %d, want positive", tt.a, tt.b, got)
				}
			}
		})
	}
}

// TestFuzzyMatchDistance0 tests fuzzyMatch with exact matching (distance 0)
func TestFuzzyMatchDistance0(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		candidate string
		want      bool
	}{
		{
			name:      "exact match case-insensitive",
			query:     "London",
			candidate: "london",
			want:      true,
		},
		{
			name:      "one character different",
			query:     "London",
			candidate: "Londx",
			want:      false,
		},
		{
			name:      "exact match same case",
			query:     "Paris",
			candidate: "Paris",
			want:      true,
		},
		{
			name:      "completely different",
			query:     "Tokyo",
			candidate: "Berlin",
			want:      false,
		},
		{
			name:      "empty strings",
			query:     "",
			candidate: "",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fuzzyMatch(tt.query, tt.candidate, 0)
			if got != tt.want {
				t.Errorf("fuzzyMatch(%q, %q, 0) = %v, want %v", tt.query, tt.candidate, got, tt.want)
			}
		})
	}
}

// TestFuzzyMatchDistance1 tests fuzzyMatch with distance 1
func TestFuzzyMatchDistance1(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		candidate string
		want      bool
	}{
		{
			name:      "missing one character",
			query:     "Londn",
			candidate: "London",
			want:      true,
		},
		{
			name:      "extra one character",
			query:     "Paris",
			candidate: "Pairis",
			want:      true,
		},
		{
			name:      "substitution one character",
			query:     "Berlin",
			candidate: "Berlan",
			want:      true,
		},
		{
			name:      "completely different strings",
			query:     "ABC",
			candidate: "London",
			want:      false,
		},
		{
			name:      "exact match still works",
			query:     "Tokyo",
			candidate: "Tokyo",
			want:      true,
		},
		{
			name:      "distance 2 should fail",
			query:     "Lndn",
			candidate: "London",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fuzzyMatch(tt.query, tt.candidate, 1)
			if got != tt.want {
				t.Errorf("fuzzyMatch(%q, %q, 1) = %v, want %v", tt.query, tt.candidate, got, tt.want)
			}
		})
	}
}

// TestFuzzyMatchDistance2 tests fuzzyMatch with distance 2
func TestFuzzyMatchDistance2(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		candidate string
		want      bool
	}{
		{
			name:      "transposition",
			query:     "Londno",
			candidate: "London",
			want:      true,
		},
		{
			name:      "missing two characters",
			query:     "Lndn",
			candidate: "London",
			want:      true,
		},
		{
			name:      "two substitutions",
			query:     "Lonxxx",
			candidate: "London",
			want:      false, // "xxx" vs "don" is 3 changes
		},
		{
			name:      "exact match still works",
			query:     "Sydney",
			candidate: "Sydney",
			want:      true,
		},
		{
			name:      "distance 1 still works",
			query:     "Sydne",
			candidate: "Sydney",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fuzzyMatch(tt.query, tt.candidate, 2)
			if got != tt.want {
				t.Errorf("fuzzyMatch(%q, %q, 2) = %v, want %v", tt.query, tt.candidate, got, tt.want)
			}
		})
	}
}

// TestFuzzyMatchBoundary tests boundary conditions for fuzzyMatch
func TestFuzzyMatchBoundary(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		candidate string
		maxDist   int
		want      bool
	}{
		{
			name:      "exact match with any distance",
			query:     "London",
			candidate: "London",
			maxDist:   5,
			want:      true,
		},
		{
			name:      "exact match with zero distance",
			query:     "Paris",
			candidate: "Paris",
			maxDist:   0,
			want:      true,
		},
		{
			name:      "completely different - always false",
			query:     "ABC",
			candidate: "WXYZ",
			maxDist:   2,
			want:      false,
		},
		{
			name:      "empty query and candidate",
			query:     "",
			candidate: "",
			maxDist:   0,
			want:      true,
		},
		{
			name:      "case insensitive exact match",
			query:     "BERLIN",
			candidate: "berlin",
			maxDist:   0,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fuzzyMatch(tt.query, tt.candidate, tt.maxDist)
			if got != tt.want {
				t.Errorf("fuzzyMatch(%q, %q, %d) = %v, want %v",
					tt.query, tt.candidate, tt.maxDist, got, tt.want)
			}
		})
	}
}

// TestCitiesSort tests sorting of Cities slice
func TestCitiesSort(t *testing.T) {
	cities := Cities{
		{City: "Berlin"},
		{City: "Austin"},
		{City: "Cairo"},
	}

	sort.Sort(cities)

	expected := []string{"Austin", "Berlin", "Cairo"}
	for i, city := range cities {
		if city.City != expected[i] {
			t.Errorf("After sort, cities[%d].City = %q, want %q", i, city.City, expected[i])
		}
	}
}

// TestCitiesSortCaseInsensitive tests case-insensitive sorting
func TestCitiesSortCaseInsensitive(t *testing.T) {
	cities := Cities{
		{City: "berlin"},
		{City: "Austin"},
		{City: "CAIRO"},
	}

	sort.Sort(cities)

	expected := []string{"Austin", "berlin", "CAIRO"}
	for i, city := range cities {
		if city.City != expected[i] {
			t.Errorf("After sort, cities[%d].City = %q, want %q", i, city.City, expected[i])
		}
	}
}

// TestCitiesSortUnicode tests sorting with Unicode city names
func TestCitiesSortUnicode(t *testing.T) {
	cities := Cities{
		{City: "Zürich"},
		{City: "Berlin"},
		{City: "Ålesund"},
		{City: "München"},
	}

	sort.Sort(cities)

	// After sorting, verify the order is maintained consistently
	// We just check that sorting completes without panic and order is stable
	if len(cities) != 4 {
		t.Errorf("Expected 4 cities after sort, got %d", len(cities))
	}

	// Verify that Berlin comes before München and Zürich (alphabetically)
	berlinIdx := -1
	münchenIdx := -1
	zürichIdx := -1

	for i, city := range cities {
		switch city.City {
		case "Berlin":
			berlinIdx = i
		case "München":
			münchenIdx = i
		case "Zürich":
			zürichIdx = i
		}
	}

	if berlinIdx < 0 || münchenIdx < 0 || zürichIdx < 0 {
		t.Errorf("Not all cities found after sort")
	}

	// Berlin should come before München
	if berlinIdx >= münchenIdx {
		t.Errorf("Expected Berlin before München, got Berlin at %d, München at %d", berlinIdx, münchenIdx)
	}
}

// TestCitiesLen tests the Len method of Cities
func TestCitiesLen(t *testing.T) {
	tests := []struct {
		name   string
		cities Cities
		want   int
	}{
		{
			name:   "empty slice",
			cities: Cities{},
			want:   0,
		},
		{
			name: "one city",
			cities: Cities{
				{City: "Austin"},
			},
			want: 1,
		},
		{
			name: "three cities",
			cities: Cities{
				{City: "Austin"},
				{City: "Berlin"},
				{City: "Cairo"},
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cities.Len()
			if got != tt.want {
				t.Errorf("Cities.Len() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestCitiesSwap tests the Swap method of Cities
func TestCitiesSwap(t *testing.T) {
	cities := Cities{
		{City: "First"},
		{City: "Second"},
		{City: "Third"},
	}

	cities.Swap(0, 2)

	if cities[0].City != "Third" {
		t.Errorf("After Swap(0,2), cities[0].City = %q, want %q", cities[0].City, "Third")
	}
	if cities[2].City != "First" {
		t.Errorf("After Swap(0,2), cities[2].City = %q, want %q", cities[2].City, "First")
	}
	if cities[1].City != "Second" {
		t.Errorf("After Swap(0,2), cities[1].City = %q, want %q", cities[1].City, "Second")
	}
}

// TestCitiesLess tests the Less method of Cities
func TestCitiesLess(t *testing.T) {
	cities := Cities{
		{City: "Austin"},
		{City: "Berlin"},
		{City: "Cairo"},
		{City: "austin"}, // lowercase version
	}

	tests := []struct {
		name string
		i    int
		j    int
		want bool
	}{
		{
			name: "Austin < Berlin",
			i:    0,
			j:    1,
			want: true,
		},
		{
			name: "Berlin < Cairo",
			i:    1,
			j:    2,
			want: true,
		},
		{
			name: "Cairo > Austin",
			i:    2,
			j:    0,
			want: false,
		},
		{
			name: "Austin == austin (case-insensitive)",
			i:    0,
			j:    3,
			want: false, // Equal should return false for Less
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cities.Less(tt.i, tt.j)
			if got != tt.want {
				t.Errorf("Cities.Less(%d, %d) for %q vs %q = %v, want %v",
					tt.i, tt.j, cities[tt.i].City, cities[tt.j].City, got, tt.want)
			}
		})
	}
}

// TestGeobedCityZeroValue tests the zero value of GeobedCity
func TestGeobedCityZeroValue(t *testing.T) {
	var city GeobedCity

	if city.City != "" {
		t.Errorf("Zero value GeobedCity.City = %q, want empty string", city.City)
	}

	if city.Country() != "" {
		t.Errorf("Zero value GeobedCity.Country() = %q, want empty string", city.Country())
	}

	if city.Region() != "" {
		t.Errorf("Zero value GeobedCity.Region() = %q, want empty string", city.Region())
	}

	if city.Latitude != 0 {
		t.Errorf("Zero value GeobedCity.Latitude = %f, want 0", city.Latitude)
	}

	if city.Longitude != 0 {
		t.Errorf("Zero value GeobedCity.Longitude = %f, want 0", city.Longitude)
	}

	if city.Population != 0 {
		t.Errorf("Zero value GeobedCity.Population = %d, want 0", city.Population)
	}

	if city.CityAlt != "" {
		t.Errorf("Zero value GeobedCity.CityAlt = %q, want empty string", city.CityAlt)
	}
}

// TestGeobedCityMethodsZeroValue tests Country() and Region() methods behavior on zero value
func TestGeobedCityMethodsZeroValue(t *testing.T) {
	// This is a black-box test, so we test the behavior without knowing implementation
	// We can only test zero values and that methods exist and are callable

	var city GeobedCity

	// Test that methods are callable and return strings
	country := city.Country()
	if country != "" {
		t.Errorf("Zero value city.Country() = %q, want empty string", country)
	}

	region := city.Region()
	if region != "" {
		t.Errorf("Zero value city.Region() = %q, want empty string", region)
	}

	// Test with a city that has a name but no other data
	city.City = "TestCity"

	// Should still return empty strings for zero-initialized internal fields
	country = city.Country()
	if country != "" {
		t.Errorf("City with only name set, Country() = %q, want empty string", country)
	}

	region = city.Region()
	if region != "" {
		t.Errorf("City with only name set, Region() = %q, want empty string", region)
	}
}
