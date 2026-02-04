package geobed

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"embed"
	_ "embed"
	"encoding/gob"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	geohash "github.com/TomiHiltunen/geohash-golang"
)

// Below is the original header comment. This fork of the geobed
// contains a snapshot of the geonames data as of 2023-08-26, and an
// old version of the maxmind data set that I found by just googling
// the filename (worldcitiespop.txt.gz) until I found it cached away
// in some other github project. So, I don't know exactly how old it
// is, but you can probably get newer, better data from the maxmind
// website -- they still have a free data set, it's just now you have
// to register and log in to get it, which is inconvenient for an
// offline appllication.

// There are over 2.4 million cities in the world. The Geonames data set only contains 143,270 and the MaxMind set contains 567,382 and 3,173,959 in the other MaxMind set.
// Obviously there's a lot of overlap and the worldcitiespop.txt from MaxMind contains a lot of dupes, though it by far the most comprehensive in terms of city - lat/lng.
// It may not be possible to have information for all cities, but many of the cities are also fairly remote and likely don't have internet access anyway.
// The Geonames data is preferred because it contains additional information such as elevation, population, and more. Population is good particuarly nice because a sense for
// the city size can be understood by applications. So showing all major cities is pretty easy. Though the primary goal of this package is to geocode, the additional information
// is bonus. So after checking the Geonames set, the geocoding functions will then look at MaxMind's.
// Maybe in the future this package will even use the Geonames premium data and have functions to look up nearest airports, etc.
// I would simply use just Geonames data, but there's so many more cities in the MaxMind set despite the lack of additional details.
//
// http://download.geonames.org/export/dump/cities1000.zip
// http://geolite.maxmind.com/download/geoip/database/GeoLiteCity_CSV/GeoLiteCity-latest.zip
// http://download.maxmind.com/download/worldcities/worldcitiespop.txt.gz

//go:embed geobed-cache
var cacheData embed.FS

// dataSetFiles defines the data sources for geocoding data.
// Additional sources can be enabled by adding entries here.
var dataSetFiles = []map[string]string{
	{"url": "http://download.geonames.org/export/dump/cities1000.zip", "path": "./geobed-data/cities1000.zip", "id": "geonamesCities1000"},
	{"url": "http://download.geonames.org/export/dump/countryInfo.txt", "path": "./geobed-data/countryInfo.txt", "id": "geonamesCountryInfo"},
}

// A handy map of US state codes to full names.
var UsSateCodes = map[string]string{
	"AL": "Alabama",
	"AK": "Alaska",
	"AZ": "Arizona",
	"AR": "Arkansas",
	"CA": "California",
	"CO": "Colorado",
	"CT": "Connecticut",
	"DE": "Delaware",
	"FL": "Florida",
	"GA": "Georgia",
	"HI": "Hawaii",
	"ID": "Idaho",
	"IL": "Illinois",
	"IN": "Indiana",
	"IA": "Iowa",
	"KS": "Kansas",
	"KY": "Kentucky",
	"LA": "Louisiana",
	"ME": "Maine",
	"MD": "Maryland",
	"MA": "Massachusetts",
	"MI": "Michigan",
	"MN": "Minnesota",
	"MS": "Mississippi",
	"MO": "Missouri",
	"MT": "Montana",
	"NE": "Nebraska",
	"NV": "Nevada",
	"NH": "New Hampshire",
	"NJ": "New Jersey",
	"NM": "New Mexico",
	"NY": "New York",
	"NC": "North Carolina",
	"ND": "North Dakota",
	"OH": "Ohio",
	"OK": "Oklahoma",
	"OR": "Oregon",
	"PA": "Pennsylvania",
	"RI": "Rhode Island",
	"SC": "South Carolina",
	"SD": "South Dakota",
	"TN": "Tennessee",
	"TX": "Texas",
	"UT": "Utah",
	"VT": "Vermont",
	"VA": "Virginia",
	"WA": "Washington",
	"WV": "West Virginia",
	"WI": "Wisconsin",
	"WY": "Wyoming",
	// Territories
	"AS": "American Samoa",
	"DC": "District of Columbia",
	"FM": "Federated States of Micronesia",
	"GU": "Guam",
	"MH": "Marshall Islands",
	"MP": "Northern Mariana Islands",
	"PW": "Palau",
	"PR": "Puerto Rico",
	"VI": "Virgin Islands",
	// Armed Forces (AE includes Europe, Africa, Canada, and the Middle East)
	"AA": "Armed Forces Americas",
	"AE": "Armed Forces Europe",
	"AP": "Armed Forces Pacific",
}

// GeoBed provides offline geocoding capabilities using embedded city data.
// It supports both forward geocoding (location name to coordinates) and reverse
// geocoding (coordinates to location information).
//
// GeoBed instances are safe for concurrent use after initialization.
// Initialization loads approximately 143,000+ city records into memory.
type GeoBed struct {
	c           Cities
	co          []CountryInfo
	cityNameIdx map[string]int // index of city name first characters to slice positions
}

type Cities []GeobedCity

func (c Cities) Len() int {
	return len(c)
}

func (c Cities) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c Cities) Less(i, j int) bool {
	return toLower(c[i].City) < toLower(c[j].City)
}

// A combined city struct (the various data sets have different fields, this combines what's available and keeps things smaller).
type GeobedCity struct {
	City    string
	CityAlt string
	// TODO: Think about converting this to a small int to save on memory allocation. Lookup requests can have the strings converted to the same int if there are any matches.
	// This could make lookup more accurate, easier, and faster even. IF the int uses less bytes than the two letter code string.
	Country    string
	Region     string
	Latitude   float64
	Longitude  float64
	Population int32
	Geohash    string
}

// Temporary indices used during data loading (cleared after use).
var (
	maxMindCityDedupeIdx map[string][]string
	locationDedupeIdx    map[string]bool
)

// Singleton pattern for default GeoBed instance.
var (
	defaultGeobed     *GeoBed
	defaultGeobedOnce sync.Once
	defaultGeobedErr  error
)

// GetDefaultGeobed returns a shared GeoBed instance, initializing it on first call.
// This is the recommended way to obtain a GeoBed instance as it ensures
// thread-safe initialization and reuses the same instance across calls.
//
// Returns an error if data cannot be loaded.
func GetDefaultGeobed() (*GeoBed, error) {
	defaultGeobedOnce.Do(func() {
		defaultGeobed, defaultGeobedErr = NewGeobed()
	})
	return defaultGeobed, defaultGeobedErr
}

// Information about each country from Geonames including; ISO codes, FIPS, country capital, area (sq km), population, and more.
// Particularly useful for validating a location string contains a country name which can help the search process.
// Adding to this info, a slice of partial geohashes to help narrow down reverse geocoding lookups (maps to country buckets).
type CountryInfo struct {
	Country            string
	Capital            string
	Area               int32
	Population         int32
	GeonameId          int32
	ISONumeric         int16
	ISO                string
	ISO3               string
	Fips               string
	Continent          string
	Tld                string
	CurrencyCode       string
	CurrencyName       string
	Phone              string
	PostalCodeFormat   string
	PostalCodeRegex    string
	Languages          string
	Neighbours         string
	EquivalentFipsCode string
}

// Options when geocoding. For now just an exact match on city name, but there will be potentially other options that can be set to adjust how searching/matching works.
type GeocodeOptions struct {
	ExactCity bool
}

// An index range struct that's used for narrowing down ranges over the large Cities struct.
type r struct {
	f int
	t int
}

// NewGeobed creates a new GeoBed instance with geocoding data loaded into memory.
// It first attempts to load from embedded cache files, falling back to downloading
// and parsing raw data files if the cache is unavailable.
//
// Returns an error if data cannot be loaded from either source.
// For a shared instance with thread-safe initialization, use GetDefaultGeobed() instead.
//
// Example:
//
//	g, err := NewGeobed()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	city := g.Geocode("Austin, TX")
//	fmt.Printf("%s: %f, %f\n", city.City, city.Latitude, city.Longitude)
func NewGeobed() (*GeoBed, error) {
	g := &GeoBed{}

	var err error
	g.c, err = loadGeobedCityData()
	if err == nil {
		g.co, err = loadGeobedCountryData()
	}
	if err == nil {
		g.cityNameIdx, err = loadGeobedCityNameIdx()
	}
	if err != nil || len(g.c) == 0 {
		// Cache not available, try downloading and parsing raw data
		if downloadErr := g.downloadDataSets(); downloadErr != nil {
			return nil, fmt.Errorf("failed to download data sets: %w", downloadErr)
		}
		if loadErr := g.loadDataSets(); loadErr != nil {
			return nil, fmt.Errorf("failed to load data sets: %w", loadErr)
		}
		// Store cache for next time (non-fatal if this fails)
		if storeErr := g.store(); storeErr != nil {
			log.Printf("warning: failed to store cache: %v", storeErr)
		}
	}

	return g, nil
}

// downloadDataSets downloads the raw data files if they don't exist locally.
// Returns an error if any download fails.
func (g *GeoBed) downloadDataSets() error {
	if err := os.MkdirAll("./geobed-data", 0777); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}

	for _, f := range dataSetFiles {
		if _, err := os.Stat(f["path"]); err == nil {
			// File already exists, skip download
			continue
		}

		if err := downloadFile(f["url"], f["path"]); err != nil {
			return fmt.Errorf("downloading %s: %w", f["id"], err)
		}
	}
	return nil
}

// downloadFile downloads a URL to a local file path.
func downloadFile(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP GET %s: status %d", url, resp.StatusCode)
	}

	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", path, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		// Clean up partial file
		os.Remove(path)
		return fmt.Errorf("writing file %s: %w", path, err)
	}

	return nil
}

// loadDataSets parses the raw data files and populates the GeoBed instance.
// Returns an error if critical data files cannot be parsed.
func (g *GeoBed) loadDataSets() error {
	locationDedupeIdx = make(map[string]bool)
	tabSplitter := regexp.MustCompile("\t")

	for _, f := range dataSetFiles {
		switch f["id"] {
		case "geonamesCities1000":
			if err := g.loadGeonamesCities(f["path"], tabSplitter); err != nil {
				return fmt.Errorf("loading geonames cities: %w", err)
			}

		case "maxmindWorldCities":
			if err := g.loadMaxMindCities(f["path"]); err != nil {
				// MaxMind is optional, just log warning
				log.Printf("warning: failed to load MaxMind cities: %v", err)
			}

		case "geonamesCountryInfo":
			if err := g.loadGeonamesCountryInfo(f["path"], tabSplitter); err != nil {
				return fmt.Errorf("loading geonames country info: %w", err)
			}
		}
	}

	// Sort cities by name for binary search optimization
	sort.Sort(g.c)

	// Build city name index for faster range-based lookups
	g.cityNameIdx = make(map[string]int)
	for k, v := range g.c {
		if len(v.City) == 0 {
			continue
		}
		ik := toLower(string(v.City[0]))
		if val, ok := g.cityNameIdx[ik]; ok {
			if val < k {
				g.cityNameIdx[ik] = k
			}
		} else {
			g.cityNameIdx[ik] = k
		}
	}

	return nil
}

// loadGeonamesCities loads city data from the Geonames cities1000.zip file.
func (g *GeoBed) loadGeonamesCities(path string, tabSplitter *regexp.Regexp) error {
	rz, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("opening zip file: %w", err)
	}
	defer rz.Close()

	for _, uF := range rz.File {
		fi, err := uF.Open()
		if err != nil {
			return fmt.Errorf("opening file in zip: %w", err)
		}
		defer fi.Close()

		scanner := bufio.NewScanner(fi)
		scanner.Split(bufio.ScanLines)

		for scanner.Scan() {
			fields := tabSplitter.Split(scanner.Text(), 19)
			if len(fields) != 19 {
				continue
			}

			lat, _ := strconv.ParseFloat(fields[4], 64)
			lng, _ := strconv.ParseFloat(fields[5], 64)
			pop, _ := strconv.Atoi(fields[14])

			gh := geohash.Encode(lat, lng)
			if gh == "7zzzzzzzzzzz" {
				gh = ""
			}

			c := GeobedCity{
				City:       strings.Trim(fields[1], " "),
				CityAlt:    fields[3],
				Country:    fields[8],
				Region:     fields[10],
				Latitude:   lat,
				Longitude:  lng,
				Population: int32(pop),
				Geohash:    gh,
			}

			if len(c.City) > 0 {
				g.c = append(g.c, c)
			}
		}
	}

	return nil
}

// loadMaxMindCities loads city data from the MaxMind worldcitiespop.txt.gz file.
func (g *GeoBed) loadMaxMindCities(path string) error {
	maxMindCityDedupeIdx = make(map[string][]string)

	fi, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer fi.Close()

	fz, err := gzip.NewReader(fi)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer fz.Close()

	scanner := bufio.NewScanner(fz)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		t := scanner.Text()
		fields := strings.Split(t, ",")
		if len(fields) == 7 {
			var b bytes.Buffer
			b.WriteString(fields[0]) // country
			b.WriteString(fields[3]) // region
			b.WriteString(fields[1]) // city
			maxMindCityDedupeIdx[b.String()] = fields
		}
	}

	// Process deduplicated entries
	for _, fields := range maxMindCityDedupeIdx {
		if fields[0] == "" || fields[0] == "0" || fields[2] == "AccentCity" {
			continue
		}

		pop, _ := strconv.Atoi(fields[4])
		lat, _ := strconv.ParseFloat(fields[5], 64)
		lng, _ := strconv.ParseFloat(fields[6], 64)

		cn := strings.Trim(fields[2], " ")
		cn = strings.Trim(cn, "( )")

		if strings.Contains(cn, "!") || strings.Contains(cn, "@") {
			continue
		}

		gh := geohash.Encode(lat, lng)
		if gh == "7zzzzzzzzzzz" {
			gh = ""
		}

		if _, ok := locationDedupeIdx[gh]; !ok {
			locationDedupeIdx[gh] = true

			c := GeobedCity{
				City:       cn,
				Country:    toUpper(fields[0]),
				Region:     fields[3],
				Latitude:   lat,
				Longitude:  lng,
				Population: int32(pop),
				Geohash:    gh,
			}

			if len(c.City) > 0 && len(c.Country) > 0 {
				g.c = append(g.c, c)
			}
		}
	}

	// Clear temporary indices for garbage collection
	maxMindCityDedupeIdx = nil
	locationDedupeIdx = nil

	return nil
}

// loadGeonamesCountryInfo loads country metadata from the Geonames countryInfo.txt file.
func (g *GeoBed) loadGeonamesCountryInfo(path string, tabSplitter *regexp.Regexp) error {
	fi, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer fi.Close()

	scanner := bufio.NewScanner(fi)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		t := scanner.Text()
		// Skip empty lines and comments (lines starting with #)
		if len(t) == 0 || t[0] == '#' {
			continue
		}

		fields := tabSplitter.Split(t, 19)
		if len(fields) != 19 || fields[0] == "" || fields[0] == "0" {
			continue
		}

		isoNumeric, _ := strconv.Atoi(fields[2])
		area, _ := strconv.Atoi(fields[6])
		pop, _ := strconv.Atoi(fields[7])
		gid, _ := strconv.Atoi(fields[16])

		ci := CountryInfo{
			ISO:                fields[0],
			ISO3:               fields[1],
			ISONumeric:         int16(isoNumeric),
			Fips:               fields[3],
			Country:            fields[4],
			Capital:            fields[5],
			Area:               int32(area),
			Population:         int32(pop),
			Continent:          fields[8],
			Tld:                fields[9],
			CurrencyCode:       fields[10],
			CurrencyName:       fields[11],
			Phone:              fields[12],
			PostalCodeFormat:   fields[13],
			PostalCodeRegex:    fields[14],
			Languages:          fields[15],
			GeonameId:          int32(gid),
			Neighbours:         fields[17],
			EquivalentFipsCode: fields[18],
		}

		g.co = append(g.co, ci)
	}

	return nil
}

// Geocode performs forward geocoding, converting a location string to coordinates.
// It returns a GeobedCity with latitude/longitude and other location metadata.
//
// By default, fuzzy matching is used which returns a "best guess" result.
// For stricter matching, pass GeocodeOptions{ExactCity: true}.
//
// Example:
//
//	city := g.Geocode("Austin, TX")
//	fmt.Printf("%s: %.4f, %.4f\n", city.City, city.Latitude, city.Longitude)
func (g *GeoBed) Geocode(n string, opts ...GeocodeOptions) GeobedCity {
	var c GeobedCity
	n = strings.TrimSpace(n)
	if n == "" {
		return c
	}
	// variadic optional argument trick
	options := GeocodeOptions{}
	if len(opts) > 0 {
		options = opts[0]
	}

	if options.ExactCity {
		c = g.exactMatchCity(n)
	} else {
		// NOTE: The downside of this (currently) is that something is basically always returned. It's a best guess.
		// There's not much chance of it returning "not found" (or an empty GeobedCity struct).
		// If you'd rather have nothing returned if not found, look at more exact matching options.
		c = g.fuzzyMatchLocation(n)
	}

	return c
}

// Returns a GeobedCity only if there is an exact city name match. A stricter match, though if state or country are missing a guess will be made.
func (g *GeoBed) exactMatchCity(n string) GeobedCity {
	var c GeobedCity
	// Ignore the `abbrevSlice` value for now. Use `nCo` and `nSt` for more accuracy.
	nCo, nSt, _, nSlice := g.extractLocationPieces(n)
	nWithoutAbbrev := strings.Join(nSlice, " ")
	ranges := g.getSearchRange(nSlice)

	matchingCities := []GeobedCity{}

	// First, get everything that matches the city exactly (case insensitive).
	for _, rng := range ranges {
		// When adjusting the range, the keys become out of sync. Start from rng.f
		currentKey := rng.f
		for _, v := range g.c[rng.f:rng.t] {
			currentKey++
			// The full string (ie. "New York" or "Las Vegas")
			if strings.EqualFold(n, v.City) {
				matchingCities = append(matchingCities, v)
			}
			// The pieces with abbreviations removed
			if strings.EqualFold(nWithoutAbbrev, v.City) {
				matchingCities = append(matchingCities, v)
			}
			// Each piece - doesn't make sense for now. May revisit this.
			// ie. "New York" or "New" and "York" ... well, "York" is going to match a different city.
			// While that might be weeded out next, who knows. It's starting to get more fuzzy than I'd like for this function.
			// for _, np := range nSlice {
			// 	if strings.EqualFold(np, v.City) {
			// 		matchingCities = append(matchingCities, v)
			// 	}
			// }
		}
	}

	// If only one was found, we can stop right here.
	if len(matchingCities) == 1 {
		return matchingCities[0]
		// If more than one was found, we need to guess.
	} else if len(matchingCities) > 1 {
		// Then range over those matching cities and try to figure out which one it is - city names are unfortunately not unique of course.
		// There shouldn't be very many so I don't mind the multiple loops.
		for _, city := range matchingCities {
			// Was the state abbreviation present? That sounds promising.
			if strings.EqualFold(nSt, city.Region) {
				c = city
			}
		}

		for _, city := range matchingCities {
			// Matches the state and country? Likely the best scenario, I'd call it the best match.
			if strings.EqualFold(nSt, city.Region) && strings.EqualFold(nCo, city.Country) {
				c = city
			}
		}

		// If we still don't have a city, maybe we have a country with the city name, ie. "New York, USA"
		// This is tougher because there's a "New York" in Florida, Kentucky, and more. Let's use population to assist if we can.
		if c.City == "" {
			matchingCountryCities := []GeobedCity{}
			for _, city := range matchingCities {
				if strings.EqualFold(nCo, city.Country) {
					matchingCountryCities = append(matchingCountryCities, city)
				}
			}

			// If someone says, "New York, USA" they most likely mean New York, NY because it's the largest city.
			// Specific locations are often implied based on size or popularity even though the names aren't unique.
			biggestCity := GeobedCity{}
			for _, city := range matchingCountryCities {
				if city.Population > biggestCity.Population {
					biggestCity = city
				}
			}
			c = biggestCity
		}
	}

	return c
}

// When geocoding, this provides a scored best match.
func (g *GeoBed) fuzzyMatchLocation(n string) GeobedCity {
	nCo, nSt, abbrevSlice, nSlice := g.extractLocationPieces(n)
	// Take the reamining unclassified pieces (those not likely to be abbreviations) and get our search range.
	// These pieces are likely contain the city name. Narrowing down the search range will make the lookup faster.
	ranges := g.getSearchRange(nSlice)

	bestMatchingKeys := map[int]int{}
	bestMatchingKey := 0
	for _, rng := range ranges {
		// When adjusting the range, the keys become out of sync. Start from rng.f
		currentKey := rng.f

		for _, v := range g.c[rng.f:rng.t] {
			currentKey++

			// Fast path for simple "City, ST" format queries
			if nSt != "" {
				if strings.EqualFold(n, v.City) && strings.EqualFold(nSt, v.Region) {
					return v
				}
			}

			// Abbreviations for state/country
			// Region (state/province)
			for _, av := range abbrevSlice {
				lowerAv := toLower(av)
				if len(av) == 2 && strings.EqualFold(v.Region, lowerAv) {
					if val, ok := bestMatchingKeys[currentKey]; ok {
						bestMatchingKeys[currentKey] = val + 5
					} else {
						bestMatchingKeys[currentKey] = 5
					}
				}

				// Country (worth 2 points if exact match)
				if len(av) == 2 && strings.EqualFold(v.Country, lowerAv) {
					if val, ok := bestMatchingKeys[currentKey]; ok {
						bestMatchingKeys[currentKey] = val + 3
					} else {
						bestMatchingKeys[currentKey] = 3
					}
				}
			}

			// A discovered country name converted into a country code
			if nCo != "" {
				if nCo == v.Country {
					if val, ok := bestMatchingKeys[currentKey]; ok {
						bestMatchingKeys[currentKey] = val + 4
					} else {
						bestMatchingKeys[currentKey] = 4
					}
				}
			}

			// A discovered state name converted into a region code
			if nSt != "" {
				if nSt == v.Region {
					if val, ok := bestMatchingKeys[currentKey]; ok {
						bestMatchingKeys[currentKey] = val + 4
					} else {
						bestMatchingKeys[currentKey] = 4
					}
				}
			}

			// If any alternate names can be discovered, take them into consideration.
			if v.CityAlt != "" {
				alts := strings.Fields(v.CityAlt)
				for _, altV := range alts {
					if strings.EqualFold(altV, n) {
						if val, ok := bestMatchingKeys[currentKey]; ok {
							bestMatchingKeys[currentKey] = val + 3
						} else {
							bestMatchingKeys[currentKey] = 3
						}
					}
					// Exact, a case-sensitive match means a lot.
					if altV == n {
						if val, ok := bestMatchingKeys[currentKey]; ok {
							bestMatchingKeys[currentKey] = val + 5
						} else {
							bestMatchingKeys[currentKey] = 5
						}
					}
				}
			}

			// Exact city name matches mean a lot.
			if strings.EqualFold(n, v.City) {
				if val, ok := bestMatchingKeys[currentKey]; ok {
					bestMatchingKeys[currentKey] = val + 7
				} else {
					bestMatchingKeys[currentKey] = 7
				}
			}

			for _, ns := range nSlice {
				ns = strings.TrimSuffix(ns, ",")

				// City (worth 2 points if contians part of string)
				if strings.Contains(toLower(v.City), toLower(ns)) {
					if val, ok := bestMatchingKeys[currentKey]; ok {
						bestMatchingKeys[currentKey] = val + 2
					} else {
						bestMatchingKeys[currentKey] = 2
					}
				}

				// If there's an exat match, maybe there was noise in the string so it could be the full city name, but unlikely. For example, "New" or "Los" is in many city names.
				// Still, give it a point because it could be the bulkier part of a city name (or the city name could be one word). This has helped in some cases.
				if strings.EqualFold(v.City, ns) {
					if val, ok := bestMatchingKeys[currentKey]; ok {
						bestMatchingKeys[currentKey] = val + 1
					} else {
						bestMatchingKeys[currentKey] = 1
					}
				}

			}
		}
	}

	// If no country was found, look at population as a factor. Is it obvious?
	if nCo == "" {
		hp := int32(0)
		hpk := 0
		for k, v := range bestMatchingKeys {
			// Add bonus point for having a population 1,000+
			if g.c[k].Population >= 1000 {
				bestMatchingKeys[k] = v + 1
			}
			// Now just add a bonus for having the highest population and points
			if g.c[k].Population > hp {
				hpk = k
				hp = g.c[k].Population
			}
		}
		// Add a point for having the highest population (if any of the results had population data available).
		if g.c[hpk].Population > 0 {
			bestMatchingKeys[hpk] = bestMatchingKeys[hpk] + 1
		}
	}

	m := 0
	for k, v := range bestMatchingKeys {
		if v > m {
			m = v
			bestMatchingKey = k
		}

		// If there is a tie breaker, use the city with the higher population (if known) because it's more likely to be what is meant.
		// For example, when people say "New York" they typically mean New York, NY...Though there are many New Yorks.
		if v == m {
			if g.c[k].Population > g.c[bestMatchingKey].Population {
				bestMatchingKey = k
			}
		}
	}

	// debug
	// log.Println("Possible results:")
	// log.Println(len(bestMatchingKeys))
	// for _, kv := range bestMatchingKeys {
	// 	log.Println(g.c[kv])
	// }
	// log.Println("Best match:")
	// log.Println(g.c[bestMatchingKey])
	// log.Println("Scored:")
	// log.Println(m)

	return g.c[bestMatchingKey]
}

// Splits a string up looking for potential abbreviations by matching against a shorter list of abbreviations.
// Returns country, state, a slice of strings with potential abbreviations (based on size; 2 or 3 characters), and then a slice of the remaning pieces.
// This does a good job at separating things that are clearly abbreviations from the city so that searching is faster and more accuarate.
func (g *GeoBed) extractLocationPieces(n string) (string, string, []string, []string) {
	re := regexp.MustCompile("")

	// Extract all potential abbreviations.
	re = regexp.MustCompile(`[\S]{2,3}`)
	abbrevSlice := re.FindStringSubmatch(n)

	// Convert country to country code and pull it out. We'll use it as a secondary form of validation. Remove the code from the original query.
	nCo := ""
	for _, co := range g.co {
		re = regexp.MustCompile("(?i)^" + co.Country + ",?\\s|\\s" + co.Country + ",?\\s" + co.Country + "\\s$")
		if re.MatchString(n) {
			nCo = co.ISO
			// And remove it so we have a cleaner query string for a city.
			n = re.ReplaceAllString(n, "")
		}
	}

	// Find US State codes and pull them out as well (do not convert state names, they can also easily be city names).
	nSt := ""
	for sc := range UsSateCodes {
		re = regexp.MustCompile("(?i)^" + sc + ",?\\s|\\s" + sc + ",?\\s|\\s" + sc + "$")
		if re.MatchString(n) {
			nSt = sc
			// And remove it too.
			n = re.ReplaceAllString(n, "")
		}
	}
	// Trim spaces and commas off the modified string.
	n = strings.Trim(n, " ,")

	// Now extract words (potential city names) into a slice. With this, the index will be referenced to pinpoint sections of the g.c []GeobedCity slice to scan.
	// This results in a much faster lookup. This is over a simple binary search with strings.Search() etc. because the city name may not be the first word.
	// This should not contain any known country code or US state codes.
	nSlice := strings.Split(n, " ")

	return nCo, nSt, abbrevSlice, nSlice
}

// getSearchRange returns index ranges in the city slice to search based on the first
// character of each piece in nSlice. This significantly reduces search space.
func (g *GeoBed) getSearchRange(nSlice []string) []r {
	ranges := []r{}
	for _, ns := range nSlice {
		ns = strings.TrimSuffix(ns, ",")

		if len(ns) > 0 {
			fc := toLower(string(ns[0]))
			pik := string(prev(rune(fc[0])))

			fk := 0
			tk := 0
			if val, ok := g.cityNameIdx[pik]; ok {
				fk = val
			}
			if val, ok := g.cityNameIdx[fc]; ok {
				tk = val
			}
			if tk == 0 {
				tk = len(g.c) - 1
			}
			ranges = append(ranges, r{fk, tk})
		}
	}

	return ranges
}

func prev(r rune) rune {
	return r - 1
}

// ReverseGeocode performs reverse geocoding, converting latitude/longitude
// coordinates to a city-level location.
//
// The algorithm uses geohash prefix matching to find the closest city.
// When multiple cities match equally, the one with the highest population wins.
//
// Example:
//
//	city := g.ReverseGeocode(30.26715, -97.74306)
//	fmt.Printf("%s, %s\n", city.City, city.Region) // Austin, TX
func (g *GeoBed) ReverseGeocode(lat, lng float64) GeobedCity {
	c := GeobedCity{}

	gh := geohash.Encode(lat, lng)
	if gh == "7zzzzzzzzzzz" {
		return c
	}

	mostMatched := 0
	matched := 0
	for k, v := range g.c {
		if len(v.Geohash) < 2 {
			continue
		}
		if v.Geohash[0] == gh[0] && v.Geohash[1] == gh[1] {
			matched = 2
			for i := 2; i <= len(gh); i++ {
				if v.Geohash[0:i] == gh[0:i] {
					matched++
				}
			}
			if matched == mostMatched && g.c[k].Population > c.Population {
				c = g.c[k]
			}
			if matched > mostMatched {
				c = g.c[k]
				mostMatched = matched
			}
		}
	}

	return c
}

// A slightly faster lowercase function.
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range b {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// A slightly faster uppercase function.
func toUpper(s string) string {
	b := make([]byte, len(s))
	for i := range b {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// store saves the Geobed data to disk for faster subsequent loads.
func (g *GeoBed) store() error {
	b := new(bytes.Buffer)

	// Store the city info
	enc := gob.NewEncoder(b)
	err := enc.Encode(g.c)
	if err != nil {
		b.Reset()
		return err
	}

	fh, eopen := os.OpenFile("./geobed-cache/g.c.dmp", os.O_CREATE|os.O_WRONLY, 0666)
	defer fh.Close()
	if eopen != nil {
		b.Reset()
		return eopen
	}
	n, e := fh.Write(b.Bytes())
	if e != nil {
		b.Reset()
		return e
	}
	log.Printf("%d bytes successfully written to cache file\n", n)

	// Store the country info as well (this is all now repetition - refactor)
	b.Reset()
	// enc = gob.NewEncoder(b)
	err = enc.Encode(g.co)
	if err != nil {
		b.Reset()
		return err
	}

	fh, eopen = os.OpenFile("./geobed-cache/g.co.dmp", os.O_CREATE|os.O_WRONLY, 0666)
	defer fh.Close()
	if eopen != nil {
		b.Reset()
		return eopen
	}
	n, e = fh.Write(b.Bytes())
	if e != nil {
		b.Reset()
		return e
	}
	log.Printf("%d bytes successfully written to cache file\n", n)

	// Store the index info
	b.Reset()
	err = enc.Encode(g.cityNameIdx)
	if err != nil {
		b.Reset()
		return err
	}

	fh, eopen = os.OpenFile("./geobed-cache/cityNameIdx.dmp", os.O_CREATE|os.O_WRONLY, 0666)
	defer fh.Close()
	if eopen != nil {
		b.Reset()
		return eopen
	}
	n, e = fh.Write(b.Bytes())
	if e != nil {
		b.Reset()
		return e
	}
	log.Printf("%d bytes successfully written to cache file\n", n)

	b.Reset()
	return nil
}

func openOptionallyCachedFile(file string) (fs.File, error) {
	fh, err := cacheData.Open(file)
	if err != nil {
		// Try local filesystem
		fh, err = os.Open(file)
		if err != nil {
			return nil, err
		}
	}
	return fh, nil
}

// openOptionallyBzippedFile looks for filename.bz2 first and returns a bzip2 reader.
// If the compressed version isn't found, it falls back to the uncompressed version.
func openOptionallyBzippedFile(file string) (io.Reader, error) {
	fh, err := openOptionallyCachedFile(file + ".bz2")
	if err != nil {
		// Try uncompressed version
		fh, err = openOptionallyCachedFile(file)
		if err != nil {
			return nil, fmt.Errorf("opening %s: %w", file, err)
		}
		return fh, nil
	}

	return bzip2.NewReader(fh), nil
}

// Loads a GeobedCity dump, which saves a bit of time.
func loadGeobedCityData() ([]GeobedCity, error) {
	fh, err := openOptionallyBzippedFile("geobed-cache/g.c.dmp")
	if err != nil {
		return nil, err
	}

	gc := []GeobedCity{}
	dec := gob.NewDecoder(fh)
	err = dec.Decode(&gc)
	if err != nil {
		return nil, err
	}
	return gc, nil
}

func loadGeobedCountryData() ([]CountryInfo, error) {
	fh, err := openOptionallyBzippedFile("geobed-cache/g.co.dmp")
	if err != nil {
		return nil, err
	}
	co := []CountryInfo{}
	dec := gob.NewDecoder(fh)
	err = dec.Decode(&co)
	if err != nil {
		return nil, err
	}
	return co, nil
}

func loadGeobedCityNameIdx() (map[string]int, error) {
	fh, err := openOptionallyBzippedFile("geobed-cache/cityNameIdx.dmp")
	if err != nil {
		return nil, err
	}
	idx := make(map[string]int)
	dec := gob.NewDecoder(fh)
	if err := dec.Decode(&idx); err != nil {
		return nil, err
	}
	return idx, nil
}
