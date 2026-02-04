// Command update-cache regenerates the geobed cache files from raw data.
//
// Usage:
//
//	go run ./cmd/update-cache
//
// This reads from ./geobed-data/ and writes to ./geobed-cache/.
// After running, compress the cache files:
//
//	bzip2 -f geobed-cache/*.dmp
package main

import (
	"fmt"
	"os"

	"github.com/andreiashu/geobed"
)

func main() {
	fmt.Println("Regenerating geobed cache from raw data...")

	if err := geobed.RegenerateCache(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Cache regenerated successfully.")
	fmt.Println("Run 'bzip2 -f geobed-cache/*.dmp' to compress the cache files.")
}
