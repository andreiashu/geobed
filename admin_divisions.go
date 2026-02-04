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

// adminDivisions maps country code -> division code -> AdminDivision
// Loaded from admin1CodesASCII.txt
var adminDivisions = map[string]map[string]AdminDivision{}
var adminDivisionsOnce sync.Once
var adminDivisionsDataDir = "./geobed-data" // configurable via setAdminDivisionsDataDir
var adminDivisionsMu sync.RWMutex           // protects adminDivisionsDataDir

// setAdminDivisionsDataDir sets the data directory for loading admin divisions.
// Thread-safe: can be called concurrently, but the first call to loadAdminDivisions
// will use whatever value was set at that moment.
func setAdminDivisionsDataDir(dir string) {
	adminDivisionsMu.Lock()
	defer adminDivisionsMu.Unlock()
	adminDivisionsDataDir = dir
}

// getAdminDivisionsDataDir returns the current data directory.
func getAdminDivisionsDataDir() string {
	adminDivisionsMu.RLock()
	defer adminDivisionsMu.RUnlock()
	return adminDivisionsDataDir
}

// loadAdminDivisions loads admin1 codes from the configured data directory.
// Format: CC.CODE<tab>Name<tab>AsciiName<tab>GeonameId
func loadAdminDivisions() {
	adminDivisionsOnce.Do(func() {
		adminDivisions = make(map[string]map[string]AdminDivision)

		// Get the data directory under the read lock
		dataDir := getAdminDivisionsDataDir()

		// Try to load from file
		fi, err := os.Open(dataDir + "/admin1CodesASCII.txt")
		if err != nil {
			return
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

			if adminDivisions[countryCode] == nil {
				adminDivisions[countryCode] = make(map[string]AdminDivision)
			}

			adminDivisions[countryCode][divisionCode] = AdminDivision{
				Code: divisionCode,
				Name: divisionName,
			}
		}
	})
}

// isAdminDivision checks if a code is a valid admin division for a specific country.
// Returns true if the code exists for that country.
func isAdminDivision(countryCode, divisionCode string) bool {
	loadAdminDivisions()
	divisionCode = toUpper(divisionCode)
	if divisions, ok := adminDivisions[countryCode]; ok {
		_, exists := divisions[divisionCode]
		return exists
	}
	return false
}

// getAdminDivisionCountry returns the country code if the given code is a known admin division.
// For ambiguous codes (existing in multiple countries), it returns empty string.
// Use isAdminDivision with a known country for precise matching.
// Examples: "TX" -> "US", "ON" -> "CA", "NSW" -> "AU"
func getAdminDivisionCountry(code string) string {
	loadAdminDivisions()
	code = toUpper(code)

	// Collect all countries that have this division code
	var matches []string
	for countryCode, divisions := range adminDivisions {
		if _, ok := divisions[code]; ok {
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
func getAdminDivisionName(countryCode, divisionCode string) string {
	loadAdminDivisions()
	divisionCode = toUpper(divisionCode)
	if divisions, ok := adminDivisions[countryCode]; ok {
		if div, exists := divisions[divisionCode]; exists {
			return div.Name
		}
	}
	return ""
}
