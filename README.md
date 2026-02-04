# Geobed

A high-performance, offline geocoding library for Go. Geocode city names to coordinates and reverse geocode coordinates to city names without any external API calls.

## Features

- **Offline**: All data embedded in the binary - no network requests after import
- **Fast reverse geocoding**: S2 spatial index delivers ~8μs per query (~150,000 queries/sec)
- **Forward geocoding**: Fuzzy matching with scoring for city name lookups
- **2.38 million cities**: Comprehensive global coverage from Geonames dataset
- **Thread-safe**: Safe for concurrent use from multiple goroutines
- **Zero configuration**: Works out of the box with `NewGeobed()`

## Installation

```bash
go get github.com/andreiashu/geobed
```

Requires Go 1.24 or later.

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    "github.com/andreiashu/geobed"
)

func main() {
    g, err := geobed.NewGeobed()
    if err != nil {
        log.Fatal(err)
    }

    // Forward geocoding: city name -> coordinates
    city := g.Geocode("Austin, TX")
    fmt.Printf("%s: %.4f, %.4f\n", city.City, city.Latitude, city.Longitude)
    // Output: Austin: 30.2672, -97.7431

    // Reverse geocoding: coordinates -> city
    result := g.ReverseGeocode(51.5074, -0.1278)
    fmt.Printf("%s, %s\n", result.City, result.Country())
    // Output: City of London, GB
}
```

## API

### Creating a GeoBed Instance

```go
// Create a new instance (loads ~218MB into memory)
g, err := geobed.NewGeobed()

// Or use a shared singleton (thread-safe, initialized once)
g, err := geobed.GetDefaultGeobed()
```

### Forward Geocoding

```go
// Basic lookup
city := g.Geocode("Paris")

// With region qualifier
city := g.Geocode("Paris, TX")      // Paris, Texas
city := g.Geocode("Paris, France")  // Paris, France

// Access result fields
fmt.Println(city.City)        // "Paris"
fmt.Println(city.Country())   // "FR"
fmt.Println(city.Region())    // "" (or state code for US cities)
fmt.Println(city.Latitude)    // 48.8566
fmt.Println(city.Longitude)   // 2.3522
fmt.Println(city.Population)  // 2138551
```

### Reverse Geocoding

```go
// Find nearest city to coordinates
city := g.ReverseGeocode(37.7749, -122.4194)
fmt.Printf("%s, %s, %s\n", city.City, city.Region(), city.Country())
// Output: San Francisco, CA, US
```

### GeobedCity Struct

```go
type GeobedCity struct {
    City       string  // City name
    CityAlt    string  // Alternate names (comma-separated)
    Latitude   float32 // Latitude in degrees
    Longitude  float32 // Longitude in degrees
    Population int32   // Population count
}

// Methods
func (c GeobedCity) Country() string  // ISO 3166-1 alpha-2 country code
func (c GeobedCity) Region() string   // State/province code (e.g., "TX", "CA")
```

## Performance

| Operation | Time | Throughput |
|-----------|------|------------|
| Reverse geocode | ~8μs | ~150,000/sec |
| Forward geocode | ~12ms | ~80/sec |
| Initial load | ~2s | - |

Benchmarked on Apple M1. Forward geocoding is slower due to fuzzy string matching across 2.38M cities.

## Memory Usage

- **Runtime memory**: ~218MB
- **Binary size**: ~56MB (embedded compressed data)

The library loads all city data into memory on initialization. This enables fast lookups but requires adequate RAM.

## How It Works

### Forward Geocoding
Uses a scored fuzzy matching algorithm that considers:
- Exact city name matches (highest priority)
- Region/state matches
- Country matches
- Alternate city names
- Partial matches
- Population (as tiebreaker)

### Reverse Geocoding
Uses Google's S2 Geometry library with a cell-based spatial index:
1. Divides Earth into hierarchical cells at level 10 (~10km)
2. Maps each city to its containing cell
3. On query, checks the target cell plus 8 neighbors
4. Returns the closest city by spherical distance

This achieves O(k) complexity where k ≈ 100-500 cities, compared to O(n) for naive scanning.

## Data Sources

City data comes from [Geonames](http://download.geonames.org/export/dump):
- `cities500.txt`: Cities with population > 500
- `countryInfo.txt`: Country metadata

Data snapshot: August 2023

## Limitations

- City-level precision only (no street addresses)
- Forward geocoding works best with well-known city names
- No typo correction (yet)
- US-centric region support (state codes work best for US)

## License

MIT License - see LICENSE file.

## Credits

- **Tom Maiaroto** ([@tmaiaroto](https://github.com/tmaiaroto)) - Original author of geobed
- **jvmatl** ([@jvmatl](https://github.com/jvmatl)) - Added embedded data files and offline capability

## Acknowledgments

- [Geonames](https://www.geonames.org/) for the open geographic database
- [Google S2 Geometry](https://github.com/golang/geo) for spatial indexing
