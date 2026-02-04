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

// loadAdminDivisions loads admin1 codes from geobed-data/admin1CodesASCII.txt
// Format: CC.CODE<tab>Name<tab>AsciiName<tab>GeonameId
func loadAdminDivisions() {
	adminDivisionsOnce.Do(func() {
		adminDivisions = make(map[string]map[string]AdminDivision)

		// Try to load from file
		fi, err := os.Open("./geobed-data/admin1CodesASCII.txt")
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

// getAdminDivisionCountry returns the country code if the given code is a known admin division.
// For example, "TX" -> "US", "NSW" -> "AU" (if NSW were a code)
func getAdminDivisionCountry(code string) string {
	loadAdminDivisions()
	code = toUpper(code)
	for countryCode, divisions := range adminDivisions {
		if _, ok := divisions[code]; ok {
			return countryCode
		}
	}
	return ""
}
