.PHONY: help test bench update-data regenerate-cache clean

# Default target
help:
	@echo "Geobed Makefile targets:"
	@echo ""
	@echo "  test              Run all tests"
	@echo "  bench             Run benchmarks"
	@echo "  update-data       Download fresh data from Geonames and regenerate cache"
	@echo "  regenerate-cache  Regenerate cache from existing raw data files"
	@echo "  clean             Remove generated cache files"
	@echo ""

# Run tests
test:
	go test -v ./...

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Download fresh data from Geonames and regenerate cache
update-data: download-data regenerate-cache
	@echo ""
	@echo "Data updated successfully!"
	@echo "Don't forget to commit the changes:"
	@echo "  git add geobed-data geobed-cache"
	@echo "  git commit -m 'Update Geonames data to $$(date +%Y-%m)'"

# Download fresh data files from Geonames
download-data:
	@echo "Downloading fresh Geonames data..."
	@mkdir -p geobed-data
	curl -o geobed-data/cities1000.zip http://download.geonames.org/export/dump/cities1000.zip
	curl -o geobed-data/countryInfo.txt http://download.geonames.org/export/dump/countryInfo.txt
	@echo "Download complete."

# Regenerate cache from raw data files
regenerate-cache:
	@echo "Regenerating cache..."
	@rm -f geobed-cache/*.dmp geobed-cache/*.dmp.bz2
	go run ./cmd/update-cache
	@echo "Compressing cache files..."
	bzip2 -f geobed-cache/*.dmp
	@echo "Cache regenerated."

# Remove generated cache files (keeps embedded originals)
clean:
	rm -f geobed-cache/*.dmp
