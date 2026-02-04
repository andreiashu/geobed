.PHONY: help test bench update-data download-data regenerate-cache validate build clean

# Default target
help:
	@echo "Geobed Makefile targets:"
	@echo ""
	@echo "  test              Run all tests"
	@echo "  bench             Run benchmarks"
	@echo "  update-data       Full pipeline: download -> regenerate -> validate -> build -> test"
	@echo "  download-data     Download fresh data from Geonames"
	@echo "  regenerate-cache  Regenerate and validate cache from raw data"
	@echo "  validate          Validate current cache (load test + functional checks)"
	@echo "  build             Build all packages"
	@echo "  clean             Remove generated cache files"
	@echo ""

# Run tests
test:
	go test -v ./...

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Full data update pipeline
update-data: download-data regenerate-cache build test
	@echo ""
	@echo "==========================================="
	@echo "Data update complete and validated!"
	@echo "==========================================="
	@echo ""
	@echo "To commit the changes:"
	@echo "  git add geobed-data geobed-cache"
	@echo "  git commit -m 'Update Geonames data to $$(date +%Y-%m)'"

# Download fresh data files from Geonames
download-data:
	@echo ""
	@echo "=== Downloading Geonames Data ==="
	@mkdir -p geobed-data
	@echo "Downloading cities1000.zip..."
	@curl -f -o geobed-data/cities1000.zip http://download.geonames.org/export/dump/cities1000.zip
	@echo "Downloading countryInfo.txt..."
	@curl -f -o geobed-data/countryInfo.txt http://download.geonames.org/export/dump/countryInfo.txt
	@echo "Downloading admin1CodesASCII.txt..."
	@curl -f -o geobed-data/admin1CodesASCII.txt http://download.geonames.org/export/dump/admin1CodesASCII.txt
	@echo ""
	@echo "Validating downloads..."
	@test $$(stat -f%z geobed-data/cities1000.zip 2>/dev/null || stat -c%s geobed-data/cities1000.zip) -gt 5000000 \
		|| (echo "ERROR: cities1000.zip too small (download failed?)" && exit 1)
	@test $$(stat -f%z geobed-data/countryInfo.txt 2>/dev/null || stat -c%s geobed-data/countryInfo.txt) -gt 20000 \
		|| (echo "ERROR: countryInfo.txt too small (download failed?)" && exit 1)
	@test $$(stat -f%z geobed-data/admin1CodesASCII.txt 2>/dev/null || stat -c%s geobed-data/admin1CodesASCII.txt) -gt 10000 \
		|| (echo "ERROR: admin1CodesASCII.txt too small (download failed?)" && exit 1)
	@echo "Download complete and validated."

# Regenerate cache from raw data files (includes validation)
regenerate-cache:
	@echo ""
	@echo "=== Regenerating Cache ==="
	@# Keep old .bz2 files until new ones are ready (for go:embed)
	@rm -f geobed-cache/*.dmp
	@go run ./cmd/update-cache
	@echo ""
	@echo "Compressing cache files..."
	@bzip2 -f geobed-cache/*.dmp
	@echo "Validating compressed cache sizes..."
	@# Expect ~7MB for cities cache (Geonames cities1000 + optimized struct format)
	@test $$(stat -f%z geobed-cache/g.c.dmp.bz2 2>/dev/null || stat -c%s geobed-cache/g.c.dmp.bz2) -gt 5000000 \
		|| (echo "ERROR: g.c.dmp.bz2 too small (< 5MB)" && exit 1)
	@echo "Cache files:"
	@ls -lh geobed-cache/*.bz2

# Validate current cache without regenerating
validate:
	@echo ""
	@echo "=== Validating Cache ==="
	@go test -v -run "TestValidation|TestDataIntegrity|TestKnownCities|TestKnownCoords" ./...

# Build all packages
build:
	@echo ""
	@echo "=== Building ==="
	@go build ./...
	@echo "Build successful."

# Remove generated cache files (keeps embedded originals in git)
clean:
	rm -f geobed-cache/*.dmp
