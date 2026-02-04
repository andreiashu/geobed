package geobed

import (
	"bufio"
	"os"
	"strings"
	"sync"
)

// AdminDivision represents a first-level administrative division (state, province, etc.)
type AdminDivision struct {
	Code string // Admin1 code (e.g., "TX", "08")
	Name string // Full name (e.g., "Texas", "Ontario")
}

// adminDivisionsCache caches loaded admin divisions per data directory.
// This avoids re-loading from disk for each GeoBed instance with the same directory.
var (
	adminDivisionsCache   = make(map[string]map[string]map[string]AdminDivision) // dataDir -> country -> code -> division
	adminDivisionsCacheMu sync.RWMutex
)

// loadAdminDivisionsForDir loads admin1 codes from the specified data directory.
// Returns a map of country code -> division code -> AdminDivision.
// Thread-safe: uses a per-directory cache to avoid redundant loading.
// Format: CC.CODE<tab>Name<tab>AsciiName<tab>GeonameId
func loadAdminDivisionsForDir(dataDir string) map[string]map[string]AdminDivision {
	// Fast path: check cache with read lock
	adminDivisionsCacheMu.RLock()
	if cached, ok := adminDivisionsCache[dataDir]; ok {
		adminDivisionsCacheMu.RUnlock()
		return cached
	}
	adminDivisionsCacheMu.RUnlock()

	// Slow path: load from disk and cache
	adminDivisionsCacheMu.Lock()
	defer adminDivisionsCacheMu.Unlock()

	// Double-check after acquiring write lock
	if cached, ok := adminDivisionsCache[dataDir]; ok {
		return cached
	}

	divisions := make(map[string]map[string]AdminDivision)

	// Try to load from file
	fi, err := os.Open(dataDir + "/admin1CodesASCII.txt")
	if err != nil {
		// Cache the empty result to avoid repeated failed attempts
		adminDivisionsCache[dataDir] = divisions
		return divisions
	}
	defer fi.Close()

	scanner := bufio.NewScanner(fi)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Split by tab: CC.CODE\tName\tAsciiName\tGeonameId
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}

		// Parse country.code from first field
		parts := strings.SplitN(fields[0], ".", 2)
		if len(parts) != 2 {
			continue
		}

		countryCode := parts[0]
		divisionCode := parts[1]
		divisionName := fields[1]

		if divisions[countryCode] == nil {
			divisions[countryCode] = make(map[string]AdminDivision)
		}

		divisions[countryCode][divisionCode] = AdminDivision{
			Code: divisionCode,
			Name: divisionName,
		}
	}

	adminDivisionsCache[dataDir] = divisions
	return divisions
}

// isAdminDivision checks if a code is a valid admin division for a specific country.
// Returns true if the code exists for that country.
func (g *GeoBed) isAdminDivision(countryCode, divisionCode string) bool {
	divisions := loadAdminDivisionsForDir(g.config.DataDir)
	divisionCode = toUpper(divisionCode)
	if countryDivisions, ok := divisions[countryCode]; ok {
		_, exists := countryDivisions[divisionCode]
		return exists
	}
	return false
}

// getAdminDivisionCountry returns the country code if the given code is a known admin division.
// For ambiguous codes (existing in multiple countries), it returns empty string.
// Use isAdminDivision with a known country for precise matching.
// Examples: "TX" -> "US", "ON" -> "CA", "NSW" -> "AU"
func (g *GeoBed) getAdminDivisionCountry(code string) string {
	divisions := loadAdminDivisionsForDir(g.config.DataDir)
	code = toUpper(code)

	// Collect all countries that have this division code
	var matches []string
	for countryCode, countryDivisions := range divisions {
		if _, ok := countryDivisions[code]; ok {
			matches = append(matches, countryCode)
		}
	}

	// Only return if unambiguous (exactly one country has this code)
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}

// getAdminDivisionName returns the name of an admin division given country and division code.
func (g *GeoBed) getAdminDivisionName(countryCode, divisionCode string) string {
	divisions := loadAdminDivisionsForDir(g.config.DataDir)
	divisionCode = toUpper(divisionCode)
	if countryDivisions, ok := divisions[countryCode]; ok {
		if div, exists := countryDivisions[divisionCode]; exists {
			return div.Name
		}
	}
	return ""
}
