// Command update-cache regenerates the geobed cache files from raw data
// and validates the result.
//
// Usage:
//
//	go run ./cmd/update-cache
//
// This reads from ./geobed-data/ and writes to ./geobed-cache/.
package main

import (
	"fmt"
	"os"

	"github.com/andreiashu/geobed"
)

func main() {
	fmt.Println("=== Geobed Cache Regeneration ===")
	fmt.Println()

	// Step 1: Regenerate cache
	fmt.Println("[1/2] Regenerating cache from raw data...")
	if err := geobed.RegenerateCache(); err != nil {
		fmt.Fprintf(os.Stderr, "Error regenerating cache: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("      Cache files written to ./geobed-cache/")

	// Step 2: Validate
	fmt.Println("[2/2] Validating generated cache...")
	if err := geobed.ValidateCache(); err != nil {
		fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("=== Success ===")
	fmt.Println("Cache regenerated and validated.")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. bzip2 -f geobed-cache/*.dmp")
	fmt.Println("  2. go test ./...")
	fmt.Println("  3. git add geobed-data geobed-cache")
}
