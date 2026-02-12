package geobed

import (
	"sync"
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type GeobedSuite struct {
	testLocations []map[string]string
}

var _ = Suite(&GeobedSuite{})

var g *GeoBed

func (s *GeobedSuite) SetUpSuite(c *C) {
	s.testLocations = append(s.testLocations, map[string]string{"query": "Austin", "city": "Austin", "country": "US", "region": "TX"})
	s.testLocations = append(s.testLocations, map[string]string{"query": "Paris", "city": "Paris", "country": "FR", "region": ""})
	s.testLocations = append(s.testLocations, map[string]string{"query": "Sydney", "city": "Sydney", "country": "AU", "region": ""})
	s.testLocations = append(s.testLocations, map[string]string{"query": "Berlin", "city": "Berlin", "country": "DE", "region": ""})
}

func (s *GeobedSuite) TestANewGeobed(c *C) {
	var err error
	g, err = NewGeobed()
	c.Assert(err, IsNil)
	c.Assert(g, Not(IsNil))
	c.Assert(len(g.Cities), Not(Equals), 0)
	c.Assert(len(g.Countries), Not(Equals), 0)
	c.Assert(len(g.nameIndex), Not(Equals), 0)
	c.Assert(g.Cities, FitsTypeOf, []GeobedCity(nil))
	c.Assert(g.Countries, FitsTypeOf, []CountryInfo(nil))
	c.Assert(g.nameIndex, FitsTypeOf, make(map[string][]int))
}

func (s *GeobedSuite) TestGeocode(c *C) {
	for _, v := range s.testLocations {
		r := g.Geocode(v["query"])
		c.Assert(r.City, Equals, v["city"])
		c.Assert(r.Country(), Equals, v["country"])
		if v["region"] != "" {
			c.Assert(r.Region(), Equals, v["region"])
		}
	}

	r := g.Geocode("")
	c.Assert(r.City, Equals, "")

	r = g.Geocode(" ")
	c.Assert(r.Population, Equals, int32(0))
}

func (s *GeobedSuite) TestReverseGeocode(c *C) {
	r := g.ReverseGeocode(30.26715, -97.74306)
	c.Assert(r.City, Equals, "Austin")
	c.Assert(r.Region(), Equals, "TX")
	c.Assert(r.Country(), Equals, "US")

	r = g.ReverseGeocode(37.44651, -122.15322)
	c.Assert(r.City, Equals, "Palo Alto")
	c.Assert(r.Region(), Equals, "CA")
	c.Assert(r.Country(), Equals, "US")

	// Use precise Santa Cruz city center coordinates
	r = g.ReverseGeocode(36.9741, -122.0308)
	c.Assert(r.City, Equals, "Santa Cruz")

	// Stanford campus coordinates
	r = g.ReverseGeocode(37.4275, -122.1697)
	c.Assert(r.City, Equals, "Stanford")

	// With neighborhood override, nearby major city London (pop >1M) is preferred
	// over the small "City of London" financial district (pop ~8k)
	r = g.ReverseGeocode(51.51279, -0.09184)
	c.Assert(r.City, Equals, "London")
}

func (s *GeobedSuite) TestToUpper(c *C) {
	c.Assert(toUpper("nyc"), Equals, "NYC")
}

func (s *GeobedSuite) TestToLower(c *C) {
	c.Assert(toLower("NYC"), Equals, "nyc")
}

func BenchmarkNewGeobed(b *testing.B) {
	var err error
	for n := 0; n < b.N; n++ {
		g, err = NewGeobed()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReverseGeocode(b *testing.B) {
	if g == nil {
		var err error
		g, err = NewGeobed()
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		g.ReverseGeocode(51.51279, -0.09184)
	}
}

func BenchmarkGeocode(b *testing.B) {
	if g == nil {
		var err error
		g, err = NewGeobed()
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		g.Geocode("New York")
	}
}

// TestConcurrentNewGeobed verifies that multiple goroutines can safely
// call NewGeobed simultaneously without races or panics.
func TestConcurrentNewGeobed(t *testing.T) {
	const numGoroutines = 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)
	instances := make(chan *GeoBed, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gb, err := NewGeobed()
			if err != nil {
				errors <- err
				return
			}
			instances <- gb
		}()
	}

	wg.Wait()
	close(errors)
	close(instances)

	// Check for errors
	for err := range errors {
		t.Fatalf("NewGeobed failed: %v", err)
	}

	// Verify all instances work correctly
	var allInstances []*GeoBed
	for gb := range instances {
		allInstances = append(allInstances, gb)
	}

	if len(allInstances) != numGoroutines {
		t.Fatalf("Expected %d instances, got %d", numGoroutines, len(allInstances))
	}

	// Test each instance can geocode correctly
	for i, gb := range allInstances {
		result := gb.Geocode("Austin, TX")
		if result.City != "Austin" {
			t.Errorf("Instance %d: expected Austin, got %s", i, result.City)
		}
		if result.Country() != "US" {
			t.Errorf("Instance %d: expected US, got %s", i, result.Country())
		}
	}
}
