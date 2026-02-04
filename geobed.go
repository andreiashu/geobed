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
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/agnivade/levenshtein"
	"github.com/golang/geo/s2"
)

//go:embed geobed-cache
var cacheData embed.FS

// DataSourceID identifies a data source type.
type DataSourceID string

const (
	DataSourceGeonamesCities   DataSourceID = "geonamesCities1000"
	DataSourceGeonamesCountry  DataSourceID = "geonamesCountryInfo"
	DataSourceGeonamesAdmin1   DataSourceID = "geonamesAdmin1Codes"
	DataSourceMaxMindCities    DataSourceID = "maxmindWorldCities"
)

// DataSource defines a data source for geocoding data.
type DataSource struct {
	URL  string       // Download URL
	Path string       // Local file path
	ID   DataSourceID // Identifier for processing logic
}

// dataSetFiles defines the data sources for geocoding data.
var dataSetFiles = []DataSource{
	{URL: "https://download.geonames.org/export/dump/cities1000.zip", Path: "./geobed-data/cities1000.zip", ID: DataSourceGeonamesCities},
	{URL: "https://download.geonames.org/export/dump/countryInfo.txt", Path: "./geobed-data/countryInfo.txt", ID: DataSourceGeonamesCountry},
	{URL: "https://download.geonames.org/export/dump/admin1CodesASCII.txt", Path: "./geobed-data/admin1CodesASCII.txt", ID: DataSourceGeonamesAdmin1},
}

// UsStateCodes maps US state abbreviations to full names.
var UsStateCodes = map[string]string{
	"AL": "Alabama", "AK": "Alaska", "AZ": "Arizona", "AR": "Arkansas",
	"CA": "California", "CO": "Colorado", "CT": "Connecticut", "DE": "Delaware",
	"FL": "Florida", "GA": "Georgia", "HI": "Hawaii", "ID": "Idaho",
	"IL": "Illinois", "IN": "Indiana", "IA": "Iowa", "KS": "Kansas",
	"KY": "Kentucky", "LA": "Louisiana", "ME": "Maine", "MD": "Maryland",
	"MA": "Massachusetts", "MI": "Michigan", "MN": "Minnesota", "MS": "Mississippi",
	"MO": "Missouri", "MT": "Montana", "NE": "Nebraska", "NV": "Nevada",
	"NH": "New Hampshire", "NJ": "New Jersey", "NM": "New Mexico", "NY": "New York",
	"NC": "North Carolina", "ND": "North Dakota", "OH": "Ohio", "OK": "Oklahoma",
	"OR": "Oregon", "PA": "Pennsylvania", "RI": "Rhode Island", "SC": "South Carolina",
	"SD": "South Dakota", "TN": "Tennessee", "TX": "Texas", "UT": "Utah",
	"VT": "Vermont", "VA": "Virginia", "WA": "Washington", "WV": "West Virginia",
	"WI": "Wisconsin", "WY": "Wyoming",
	// Territories
	"AS": "American Samoa", "DC": "District of Columbia",
	"FM": "Federated States of Micronesia", "GU": "Guam",
	"MH": "Marshall Islands", "MP": "Northern Mariana Islands",
	"PW": "Palau", "PR": "Puerto Rico", "VI": "Virgin Islands",
	// Armed Forces
	"AA": "Armed Forces Americas", "AE": "Armed Forces Europe", "AP": "Armed Forces Pacific",
}

// s2CellLevel determines the granularity of the S2 spatial index for reverse geocoding.
//
// S2 cells are a hierarchical spatial indexing system (see https://s2geometry.io/).
// Level 10 provides approximately 10km x 10km cells at the equator, which offers
// a good balance between:
//   - Precision: Cells are small enough to group nearby cities effectively
//   - Performance: Not too many cells to search for nearby cities
//   - Memory: Reasonable number of cells in the index
//
// Lower levels (e.g., 8) would give ~40km cells - faster but less precise.
// Higher levels (e.g., 12) would give ~2.5km cells - more precise but more memory.
const s2CellLevel = 10

// Package-level lookup tables for memory-efficient string storage.
//
// Architecture Note: These tables are global (not per-instance) because GeobedCity
// methods like Country() and Region() cannot access instance data - they're called
// on value types that don't have a reference back to the GeoBed instance. This design
// allows the memory-efficient indexed storage while maintaining a clean API.
//
// Thread Safety: Each stringInterner has its own RWMutex protecting all access:
//   - Writes (interning new values) acquire the write lock
//   - Reads (lookup by index) acquire the read lock
//   - Initialization uses sync.Once for one-time setup
//
// Memory Efficiency: By storing string indexes (uint8/uint16) instead of strings
// in each GeobedCity, we save ~24 bytes per city (two string headers). With ~145K
// cities, this saves ~3.5MB of memory.

// stringInterner provides thread-safe string interning with integer indexes.
// T must be an unsigned integer type (uint8 or uint16).
type stringInterner[T ~uint8 | ~uint16] struct {
	mu     sync.RWMutex
	lookup []string     // index -> string
	index  map[string]T // string -> index
}

// newStringInterner creates a new string interner with the given initial capacity.
// Index 0 is reserved for the empty string.
func newStringInterner[T ~uint8 | ~uint16](capacity int) *stringInterner[T] {
	si := &stringInterner[T]{
		lookup: make([]string, 1, capacity), // index 0 = ""
		index:  make(map[string]T, capacity),
	}
	si.lookup[0] = ""
	si.index[""] = 0
	return si
}

// intern returns the index for a string, creating it if needed.
// Thread-safe: uses double-checked locking pattern.
func (si *stringInterner[T]) intern(s string) T {
	// Fast path: check with read lock
	si.mu.RLock()
	if idx, ok := si.index[s]; ok {
		si.mu.RUnlock()
		return idx
	}
	si.mu.RUnlock()

	// Slow path: acquire write lock and check again
	si.mu.Lock()
	defer si.mu.Unlock()
	if idx, ok := si.index[s]; ok {
		return idx
	}
	idx := T(len(si.lookup))
	si.lookup = append(si.lookup, s)
	si.index[s] = idx
	return idx
}

// get returns the string for an index, or empty string if out of bounds.
func (si *stringInterner[T]) get(idx T) string {
	si.mu.RLock()
	defer si.mu.RUnlock()
	if int(idx) < len(si.lookup) {
		return si.lookup[idx]
	}
	return ""
}

// count returns the number of interned strings.
func (si *stringInterner[T]) count() int {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return len(si.lookup)
}

var (
	countryInterner *stringInterner[uint8]
	regionInterner  *stringInterner[uint16]
	lookupOnce      sync.Once
)

// GeobedConfig contains configuration options for GeoBed initialization.
type GeobedConfig struct {
	DataDir  string // Directory for raw data files (default: "./geobed-data")
	CacheDir string // Directory for cache files (default: "./geobed-cache")
}

// Option is a functional option for configuring GeoBed.
type Option func(*GeobedConfig)

// WithDataDir sets the directory for raw data files.
func WithDataDir(dir string) Option {
	return func(c *GeobedConfig) {
		c.DataDir = dir
	}
}

// WithCacheDir sets the directory for cache files.
func WithCacheDir(dir string) Option {
	return func(c *GeobedConfig) {
		c.CacheDir = dir
	}
}

// defaultConfig returns the default configuration.
func defaultConfig() *GeobedConfig {
	return &GeobedConfig{
		DataDir:  "./geobed-data",
		CacheDir: "./geobed-cache",
	}
}

// GeoBed provides offline geocoding using embedded city data.
// Safe for concurrent use after initialization.
type GeoBed struct {
	Cities      Cities              // All loaded cities, sorted by name
	Countries   []CountryInfo       // Country metadata from Geonames
	cityNameIdx map[string]int      // index for city name search ranges
	cellIndex   map[s2.CellID][]int // S2 cell index for reverse geocoding
	config      *GeobedConfig       // Configuration options
}

// Cities is a sortable slice of GeobedCity.
type Cities []GeobedCity

func (c Cities) Len() int           { return len(c) }
func (c Cities) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c Cities) Less(i, j int) bool { return toLower(c[i].City) < toLower(c[j].City) }

// GeobedCity represents a city with geocoding data.
// Memory-optimized: uses indexes for Country/Region, float32 for coordinates.
type GeobedCity struct {
	City       string  // City name
	CityAlt    string  // Alternate names (comma-separated)
	country    uint8   // Index into countryLookup
	region     uint16  // Index into regionLookup
	Latitude   float32 // Latitude in degrees
	Longitude  float32 // Longitude in degrees
	Population int32   // Population count
}

// Country returns the ISO 3166-1 alpha-2 country code (e.g., "US", "FR").
func (c GeobedCity) Country() string {
	return countryInterner.get(c.country)
}

// Region returns the administrative region code (e.g., "TX", "CA").
func (c GeobedCity) Region() string {
	return regionInterner.get(c.region)
}

// CountryCount returns the number of unique country codes in the lookup table.
// Useful for testing and debugging.
func CountryCount() int {
	return countryInterner.count()
}

// RegionCount returns the number of unique region codes in the lookup table.
// Useful for testing and debugging.
func RegionCount() int {
	return regionInterner.count()
}

// geobedCityGob is used for GOB serialization (stores strings, not indexes).
type geobedCityGob struct {
	City       string
	CityAlt    string
	Country    string
	Region     string
	Latitude   float32
	Longitude  float32
	Population int32
}

// Temporary indices used during data loading.
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
func GetDefaultGeobed() (*GeoBed, error) {
	defaultGeobedOnce.Do(func() {
		defaultGeobed, defaultGeobedErr = NewGeobed()
	})
	return defaultGeobed, defaultGeobedErr
}

// CountryInfo contains metadata about a country from Geonames.
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

// GeocodeOptions configures geocoding behavior.
type GeocodeOptions struct {
	ExactCity     bool // Require exact city name match
	FuzzyDistance int  // Max edit distance for typo tolerance (0 = disabled, 1-2 recommended)
}

// r represents an index range for city searches.
type r struct {
	f int
	t int
}

// NewGeobed creates a new GeoBed instance with geocoding data loaded into memory.
//
// Options can be provided to customize data and cache directories:
//
//	g, err := NewGeobed(WithDataDir("/custom/data"), WithCacheDir("/custom/cache"))
//
// Example:
//
//	g, err := NewGeobed()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	city := g.Geocode("Austin, TX")
//	fmt.Printf("%s: %f, %f\n", city.City, city.Latitude, city.Longitude)
func NewGeobed(opts ...Option) (*GeoBed, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Configure admin divisions data path before any use
	setAdminDivisionsDataDir(cfg.DataDir)

	g := &GeoBed{config: cfg}

	// Initialize lookup tables (thread-safe, runs once)
	lookupOnce.Do(initLookupTables)

	var err error
	g.Cities, err = loadGeobedCityData()
	if err == nil {
		g.Countries, err = loadGeobedCountryData()
	}
	if err == nil {
		g.cityNameIdx, err = loadGeobedCityNameIdx()
	}
	if err != nil || len(g.Cities) == 0 {
		if downloadErr := g.downloadDataSets(); downloadErr != nil {
			return nil, fmt.Errorf("failed to download data sets: %w", downloadErr)
		}
		if loadErr := g.loadDataSets(); loadErr != nil {
			return nil, fmt.Errorf("failed to load data sets: %w", loadErr)
		}
		if storeErr := g.store(); storeErr != nil {
			log.Printf("warning: failed to store cache: %v", storeErr)
		}
	}

	g.buildCellIndex()
	return g, nil
}

// initLookupTables initializes the country and region string interners.
func initLookupTables() {
	countryInterner = newStringInterner[uint8](256)
	regionInterner = newStringInterner[uint16](8192)
}

// internCountry returns the index for a country code, creating it if needed.
func internCountry(code string) uint8 {
	return countryInterner.intern(code)
}

// internRegion returns the index for a region code, creating it if needed.
func internRegion(code string) uint16 {
	return regionInterner.intern(code)
}

// buildCellIndex creates an S2 cell-based spatial index for fast reverse geocoding.
func (g *GeoBed) buildCellIndex() {
	g.cellIndex = make(map[s2.CellID][]int)
	for i, city := range g.Cities {
		ll := s2.LatLngFromDegrees(float64(city.Latitude), float64(city.Longitude))
		cell := s2.CellIDFromLatLng(ll).Parent(s2CellLevel)
		g.cellIndex[cell] = append(g.cellIndex[cell], i)
	}
}

// cellAndNeighbors returns the given cell plus its neighboring cells.
func (g *GeoBed) cellAndNeighbors(cell s2.CellID) []s2.CellID {
	cells := make([]s2.CellID, 0, 9)
	cells = append(cells, cell)

	edgeNeighbors := cell.EdgeNeighbors()
	for i := 0; i < 4; i++ {
		cells = append(cells, edgeNeighbors[i])
	}

	seen := make(map[s2.CellID]bool)
	for _, c := range cells {
		seen[c] = true
	}
	for i := 0; i < 4; i++ {
		for _, corner := range edgeNeighbors[i].EdgeNeighbors() {
			if !seen[corner] {
				cells = append(cells, corner)
				seen[corner] = true
			}
		}
	}
	return cells
}

// downloadDataSets downloads the raw data files if they don't exist locally.
func (g *GeoBed) downloadDataSets() error {
	if err := os.MkdirAll(g.config.DataDir, 0777); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}

	for _, f := range dataSetFiles {
		localPath := g.config.DataDir + "/" + filepath.Base(f.Path)
		if _, err := os.Stat(localPath); err == nil {
			continue
		}
		if err := downloadFile(f.URL, localPath); err != nil {
			return fmt.Errorf("downloading %s: %w", f.ID, err)
		}
	}
	return nil
}

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
		os.Remove(path)
		return fmt.Errorf("writing file %s: %w", path, err)
	}
	return nil
}

// loadDataSets parses the raw data files and populates the GeoBed instance.
func (g *GeoBed) loadDataSets() error {
	locationDedupeIdx = make(map[string]bool)
	tabSplitter := regexp.MustCompile("\t")

	for _, f := range dataSetFiles {
		localPath := g.config.DataDir + "/" + filepath.Base(f.Path)
		switch f.ID {
		case DataSourceGeonamesCities:
			if err := g.loadGeonamesCities(localPath, tabSplitter); err != nil {
				return fmt.Errorf("loading geonames cities: %w", err)
			}
		case DataSourceMaxMindCities:
			// MaxMind is optional supplemental data; continue on error
			if err := g.loadMaxMindCities(localPath); err != nil {
				log.Printf("info: MaxMind cities not loaded (optional): %v", err)
			}
		case DataSourceGeonamesCountry:
			if err := g.loadGeonamesCountryInfo(localPath, tabSplitter); err != nil {
				return fmt.Errorf("loading geonames country info: %w", err)
			}
		}
	}

	sort.Sort(g.Cities)

	g.cityNameIdx = make(map[string]int)
	for k, v := range g.Cities {
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

			lat, _ := strconv.ParseFloat(fields[4], 32)
			lng, _ := strconv.ParseFloat(fields[5], 32)
			pop, _ := strconv.Atoi(fields[14])

			c := GeobedCity{
				City:       strings.Trim(fields[1], " "),
				CityAlt:    fields[3],
				country:    internCountry(fields[8]),
				region:     internRegion(fields[10]),
				Latitude:   float32(lat),
				Longitude:  float32(lng),
				Population: int32(pop),
			}

			if len(c.City) > 0 {
				g.Cities = append(g.Cities, c)
			}
		}
	}
	return nil
}

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
			b.WriteString(fields[0])
			b.WriteString(fields[3])
			b.WriteString(fields[1])
			maxMindCityDedupeIdx[b.String()] = fields
		}
	}

	for _, fields := range maxMindCityDedupeIdx {
		if fields[0] == "" || fields[0] == "0" || fields[2] == "AccentCity" {
			continue
		}

		pop, _ := strconv.Atoi(fields[4])
		lat, _ := strconv.ParseFloat(fields[5], 32)
		lng, _ := strconv.ParseFloat(fields[6], 32)

		cn := strings.Trim(fields[2], " ")
		cn = strings.Trim(cn, "( )")

		if strings.Contains(cn, "!") || strings.Contains(cn, "@") {
			continue
		}

		// Use lat/lng as dedup key instead of geohash
		dedupeKey := fmt.Sprintf("%.4f,%.4f", lat, lng)
		if _, ok := locationDedupeIdx[dedupeKey]; !ok {
			locationDedupeIdx[dedupeKey] = true

			c := GeobedCity{
				City:       cn,
				country:    internCountry(toUpper(fields[0])),
				region:     internRegion(fields[3]),
				Latitude:   float32(lat),
				Longitude:  float32(lng),
				Population: int32(pop),
			}

			if len(c.City) > 0 && c.country != 0 {
				g.Cities = append(g.Cities, c)
			}
		}
	}

	maxMindCityDedupeIdx = nil
	locationDedupeIdx = nil
	return nil
}

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
		g.Countries = append(g.Countries, ci)
	}
	return nil
}

// fuzzyMatch compares two strings with optional Levenshtein distance tolerance.
// If maxDist is 0, performs exact case-insensitive match.
// Otherwise, returns true if the edit distance between the strings is <= maxDist.
func fuzzyMatch(query, candidate string, maxDist int) bool {
	if maxDist == 0 {
		return strings.EqualFold(query, candidate)
	}
	dist := levenshtein.ComputeDistance(
		strings.ToLower(query),
		strings.ToLower(candidate),
	)
	return dist <= maxDist
}

// Geocode performs forward geocoding, converting a location string to coordinates.
func (g *GeoBed) Geocode(n string, opts ...GeocodeOptions) GeobedCity {
	var c GeobedCity
	n = strings.TrimSpace(n)
	if n == "" {
		return c
	}

	options := GeocodeOptions{}
	if len(opts) > 0 {
		options = opts[0]
	}

	if options.ExactCity {
		c = g.exactMatchCity(n)
	} else {
		c = g.fuzzyMatchLocation(n, options)
	}
	return c
}

func (g *GeoBed) exactMatchCity(n string) GeobedCity {
	var c GeobedCity
	nCo, nSt, _, nSlice := g.extractLocationPieces(n)
	nWithoutAbbrev := strings.Join(nSlice, " ")
	ranges := g.getSearchRange(nSlice)

	matchingCities := []GeobedCity{}

	for _, rng := range ranges {
		for _, v := range g.Cities[rng.f:rng.t] {
			if strings.EqualFold(n, v.City) {
				matchingCities = append(matchingCities, v)
			}
			if strings.EqualFold(nWithoutAbbrev, v.City) {
				matchingCities = append(matchingCities, v)
			}
		}
	}

	if len(matchingCities) == 1 {
		return matchingCities[0]
	} else if len(matchingCities) > 1 {
		for _, city := range matchingCities {
			if strings.EqualFold(nSt, city.Region()) {
				c = city
			}
		}

		for _, city := range matchingCities {
			if strings.EqualFold(nSt, city.Region()) && strings.EqualFold(nCo, city.Country()) {
				c = city
			}
		}

		if c.City == "" {
			matchingCountryCities := []GeobedCity{}
			for _, city := range matchingCities {
				if strings.EqualFold(nCo, city.Country()) {
					matchingCountryCities = append(matchingCountryCities, city)
				}
			}

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

func (g *GeoBed) fuzzyMatchLocation(n string, opts GeocodeOptions) GeobedCity {
	nCo, nSt, abbrevSlice, nSlice := g.extractLocationPieces(n)
	ranges := g.getSearchRange(nSlice)

	bestMatchingKeys := map[int]int{}
	bestMatchingKey := 0

	for _, rng := range ranges {
		for i, v := range g.Cities[rng.f:rng.t] {
			currentKey := rng.f + i // Correct index into g.Cities
			vCountry := v.Country()
			vRegion := v.Region()

			// Fast path for simple "City, ST" format
			if nSt != "" {
				if strings.EqualFold(n, v.City) && strings.EqualFold(nSt, vRegion) {
					return v
				}
			}

			for _, av := range abbrevSlice {
				lowerAv := toLower(av)
				if len(av) == 2 && strings.EqualFold(vRegion, lowerAv) {
					bestMatchingKeys[currentKey] += 5
				}
				if len(av) == 2 && strings.EqualFold(vCountry, lowerAv) {
					bestMatchingKeys[currentKey] += 3
				}
			}

			if nCo != "" && nCo == vCountry {
				bestMatchingKeys[currentKey] += 4
			}

			if nSt != "" && nSt == vRegion {
				bestMatchingKeys[currentKey] += 4
			}

			if v.CityAlt != "" {
				alts := strings.Fields(v.CityAlt)
				for _, altV := range alts {
					if strings.EqualFold(altV, n) {
						bestMatchingKeys[currentKey] += 3
					}
					if altV == n {
						bestMatchingKeys[currentKey] += 5
					}
				}
			}

			// Exact match gets highest bonus
			if strings.EqualFold(n, v.City) {
				bestMatchingKeys[currentKey] += 7
			} else if opts.FuzzyDistance > 0 {
				// Fuzzy matching with Levenshtein distance
				for _, ns := range nSlice {
					ns = strings.TrimSuffix(ns, ",")
					if len(ns) > 2 && fuzzyMatch(ns, v.City, opts.FuzzyDistance) {
						// Fuzzy match gets slightly lower bonus than exact match
						bestMatchingKeys[currentKey] += 5
					}
				}
			}

			for _, ns := range nSlice {
				ns = strings.TrimSuffix(ns, ",")
				if strings.Contains(toLower(v.City), toLower(ns)) {
					bestMatchingKeys[currentKey] += 2
				}
				if strings.EqualFold(v.City, ns) {
					bestMatchingKeys[currentKey] += 1
				}
			}
		}
	}

	if nCo == "" {
		hp := int32(0)
		hpk := 0
		for k, v := range bestMatchingKeys {
			if g.Cities[k].Population >= 1000 {
				bestMatchingKeys[k] = v + 1
			}
			if g.Cities[k].Population > hp {
				hpk = k
				hp = g.Cities[k].Population
			}
		}
		if g.Cities[hpk].Population > 0 {
			bestMatchingKeys[hpk]++
		}
	}

	m := 0
	for k, v := range bestMatchingKeys {
		if v > m {
			m = v
			bestMatchingKey = k
		}
		if v == m && g.Cities[k].Population > g.Cities[bestMatchingKey].Population {
			bestMatchingKey = k
		}
	}

	return g.Cities[bestMatchingKey]
}

func (g *GeoBed) extractLocationPieces(n string) (string, string, []string, []string) {
	re := regexp.MustCompile(`[\S]{2,3}`)
	abbrevSlice := re.FindStringSubmatch(n)

	nCo := ""
	for _, co := range g.Countries {
		re = regexp.MustCompile("(?i)^" + co.Country + ",?\\s|\\s" + co.Country + ",?\\s" + co.Country + "\\s$")
		if re.MatchString(n) {
			nCo = co.ISO
			n = re.ReplaceAllString(n, "")
		}
	}

	nSt := ""
	// First check US state codes (most common case)
	for sc := range UsStateCodes {
		re = regexp.MustCompile("(?i)^" + sc + ",?\\s|\\s" + sc + ",?\\s|\\s" + sc + "$")
		if re.MatchString(n) {
			nSt = sc
			n = re.ReplaceAllString(n, "")
			// If we matched a US state, set country to US if not already set
			if nCo == "" {
				nCo = "US"
			}
		}
	}

	// If no US state matched, check international admin divisions
	if nSt == "" {
		// Load admin divisions (lazy, thread-safe)
		loadAdminDivisions()

		// Look for 2-3 letter codes at end of query that could be admin divisions
		// Pattern: "Toronto, ON" or "Sydney NSW"
		parts := strings.Split(n, " ")
		if len(parts) >= 2 {
			lastPart := strings.Trim(parts[len(parts)-1], ", ")
			if len(lastPart) >= 2 && len(lastPart) <= 3 {
				lastPartUpper := toUpper(lastPart)
				// If we know the country, check if it's a valid division for that country
				if nCo != "" && isAdminDivision(nCo, lastPartUpper) {
					nSt = lastPartUpper
					n = strings.Join(parts[:len(parts)-1], " ")
				} else if nCo == "" {
					// Try to find an unambiguous admin division
					if country := getAdminDivisionCountry(lastPartUpper); country != "" {
						nSt = lastPartUpper
						nCo = country
						n = strings.Join(parts[:len(parts)-1], " ")
					}
				}
			}
		}
	}
	n = strings.Trim(n, " ,")

	nSlice := strings.Split(n, " ")
	return nCo, nSt, abbrevSlice, nSlice
}

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
				tk = len(g.Cities) - 1
			}
			ranges = append(ranges, r{fk, tk})
		}
	}
	return ranges
}

func prev(r rune) rune {
	return r - 1
}

// ReverseGeocode converts lat/lng coordinates to a city location.
func (g *GeoBed) ReverseGeocode(lat, lng float64) GeobedCity {
	queryLL := s2.LatLngFromDegrees(lat, lng)
	queryCell := s2.CellIDFromLatLng(queryLL).Parent(s2CellLevel)

	var closest GeobedCity
	minDist := math.MaxFloat64

	for _, cell := range g.cellAndNeighbors(queryCell) {
		indices, ok := g.cellIndex[cell]
		if !ok {
			continue
		}

		for _, idx := range indices {
			city := g.Cities[idx]
			cityLL := s2.LatLngFromDegrees(float64(city.Latitude), float64(city.Longitude))
			dist := float64(queryLL.Distance(cityLL))

			if dist < minDist {
				minDist = dist
				closest = city
			} else if dist == minDist && city.Population > closest.Population {
				closest = city
			}
		}
	}
	return closest
}

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

// RegenerateCache forces a reload from raw data files and regenerates the cache.
// This is useful for updating the embedded cache after downloading fresh data.
// The raw data files must exist in ./geobed-data/ before calling this function.
//
// After running, compress the cache files with bzip2:
//
//	bzip2 -f geobed-cache/*.dmp
func RegenerateCache() error {
	g := &GeoBed{config: defaultConfig()}

	// Initialize lookup tables
	lookupOnce.Do(initLookupTables)

	// Load from raw data files (skip cache)
	if err := g.loadDataSets(); err != nil {
		return fmt.Errorf("failed to load data sets: %w", err)
	}

	// Store to cache
	if err := g.store(); err != nil {
		return fmt.Errorf("failed to store cache: %w", err)
	}

	return nil
}

// Validation thresholds for data integrity checks.
// Based on Geonames cities1000.zip dataset (~145K cities with pop > 1000).
const (
	minCityCount    = 140000 // Expect at least 140K cities from Geonames
	minCountryCount = 200    // Expect at least 200 countries
)

// validationCity defines a known city for functional validation.
type validationCity struct {
	query       string
	wantCity    string
	wantCountry string
}

// validationCoord defines known coordinates for reverse geocoding validation.
type validationCoord struct {
	lat, lng    float64
	wantCity    string
	wantCountry string
}

// knownCities are used to validate forward geocoding works correctly.
// These are chosen to be unambiguous and match actual geocoder behavior.
var knownCities = []validationCity{
	{"Austin", "Austin", "US"},
	{"Paris", "Paris", "FR"},
	{"Sydney", "Sydney", "AU"},
	{"Berlin", "Berlin", "DE"},
	{"New York, NY", "New York City", "US"},
	{"Tokyo", "Tokyo", "JP"},
}

// knownCoords are used to validate reverse geocoding works correctly.
// Coordinates are chosen to be near city centers for reliable matching.
var knownCoords = []validationCoord{
	{30.26715, -97.74306, "Austin", "US"},     // Austin, TX (from existing tests)
	{37.44651, -122.15322, "Palo Alto", "US"}, // Palo Alto, CA (from existing tests)
	{36.9741, -122.0308, "Santa Cruz", "US"},  // Santa Cruz, CA (from existing tests)
	{-33.8688, 151.2093, "Sydney", "AU"},      // Sydney
}

// ValidateCache loads the cache and performs integrity and functional checks.
// Returns an error if validation fails.
func ValidateCache() error {
	// Load from cache (this tests that cache files are readable)
	g, err := NewGeobed()
	if err != nil {
		return fmt.Errorf("failed to load cache: %w", err)
	}

	// Check city count
	cityCount := len(g.Cities)
	if cityCount < minCityCount {
		return fmt.Errorf("city count too low: got %d, want >= %d", cityCount, minCityCount)
	}
	fmt.Printf("      City count: %d (OK)\n", cityCount)

	// Check country count
	countryCount := len(g.Countries)
	if countryCount < minCountryCount {
		return fmt.Errorf("country count too low: got %d, want >= %d", countryCount, minCountryCount)
	}
	fmt.Printf("      Country count: %d (OK)\n", countryCount)

	// Validate forward geocoding
	fmt.Printf("      Forward geocoding: ")
	for _, tc := range knownCities {
		result := g.Geocode(tc.query)
		if result.City != tc.wantCity {
			return fmt.Errorf("geocode(%q) = %q, want %q", tc.query, result.City, tc.wantCity)
		}
		if result.Country() != tc.wantCountry {
			return fmt.Errorf("geocode(%q) country = %q, want %q", tc.query, result.Country(), tc.wantCountry)
		}
	}
	fmt.Printf("%d cities OK\n", len(knownCities))

	// Validate reverse geocoding
	fmt.Printf("      Reverse geocoding: ")
	for _, tc := range knownCoords {
		result := g.ReverseGeocode(tc.lat, tc.lng)
		if result.City != tc.wantCity {
			return fmt.Errorf("reverseGeocode(%v, %v) = %q, want %q", tc.lat, tc.lng, result.City, tc.wantCity)
		}
		if result.Country() != tc.wantCountry {
			return fmt.Errorf("reverseGeocode(%v, %v) country = %q, want %q", tc.lat, tc.lng, result.Country(), tc.wantCountry)
		}
	}
	fmt.Printf("%d coords OK\n", len(knownCoords))

	return nil
}

// store saves the Geobed data to disk cache.
func (g *GeoBed) store() error {
	cacheDir := g.config.CacheDir
	if err := os.MkdirAll(cacheDir, 0777); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	// Convert to GOB-friendly format
	gobCities := make([]geobedCityGob, len(g.Cities))
	for i, c := range g.Cities {
		gobCities[i] = geobedCityGob{
			City:       c.City,
			CityAlt:    c.CityAlt,
			Country:    c.Country(),
			Region:     c.Region(),
			Latitude:   c.Latitude,
			Longitude:  c.Longitude,
			Population: c.Population,
		}
	}

	b := new(bytes.Buffer)
	enc := gob.NewEncoder(b)

	if err := enc.Encode(gobCities); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "g.c.dmp"), b.Bytes(), 0666); err != nil {
		return err
	}

	b.Reset()
	if err := enc.Encode(g.Countries); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "g.co.dmp"), b.Bytes(), 0666); err != nil {
		return err
	}

	b.Reset()
	if err := enc.Encode(g.cityNameIdx); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "cityNameIdx.dmp"), b.Bytes(), 0666); err != nil {
		return err
	}

	return nil
}

func openOptionallyCachedFile(file string) (fs.File, error) {
	fh, err := cacheData.Open(file)
	if err != nil {
		fh, err = os.Open(file)
		if err != nil {
			return nil, err
		}
	}
	return fh, nil
}

func openOptionallyBzippedFile(file string) (io.Reader, error) {
	fh, err := openOptionallyCachedFile(file + ".bz2")
	if err != nil {
		fh, err = openOptionallyCachedFile(file)
		if err != nil {
			return nil, fmt.Errorf("opening %s: %w", file, err)
		}
		return fh, nil
	}
	return bzip2.NewReader(fh), nil
}

func loadGeobedCityData() ([]GeobedCity, error) {
	fh, err := openOptionallyBzippedFile("geobed-cache/g.c.dmp")
	if err != nil {
		return nil, err
	}

	// Try loading as new format first
	var gobCities []geobedCityGob
	dec := gob.NewDecoder(fh)
	if err := dec.Decode(&gobCities); err != nil {
		return nil, err
	}

	// Convert from GOB format to memory-efficient format
	cities := make([]GeobedCity, len(gobCities))
	for i, gc := range gobCities {
		cities[i] = GeobedCity{
			City:       gc.City,
			CityAlt:    gc.CityAlt,
			country:    internCountry(gc.Country),
			region:     internRegion(gc.Region),
			Latitude:   gc.Latitude,
			Longitude:  gc.Longitude,
			Population: gc.Population,
		}
	}
	return cities, nil
}

func loadGeobedCountryData() ([]CountryInfo, error) {
	fh, err := openOptionallyBzippedFile("geobed-cache/g.co.dmp")
	if err != nil {
		return nil, err
	}
	co := []CountryInfo{}
	dec := gob.NewDecoder(fh)
	if err := dec.Decode(&co); err != nil {
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
