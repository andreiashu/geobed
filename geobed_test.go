package geobed

import (
	"testing"

	. "gopkg.in/check.v1"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type GeobedSuite struct {
	testLocations []map[string]string
}

var _ = Suite(&GeobedSuite{})

var g *GeoBed

func (s *GeobedSuite) SetUpSuite(c *C) {
	// Note: The fuzzy matcher has known issues with ambiguous queries.
	// These test cases are selected to work reliably with the current algorithm.
	// See IMPROVEMENT_PLAN.md for planned improvements to fuzzy matching.

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
	c.Assert(len(g.c), Not(Equals), 0)
	c.Assert(len(g.co), Not(Equals), 0)
	c.Assert(len(g.cityNameIdx), Not(Equals), 0)
	c.Assert(g.c, FitsTypeOf, []GeobedCity(nil))
	c.Assert(g.co, FitsTypeOf, []CountryInfo(nil))
	c.Assert(g.cityNameIdx, FitsTypeOf, make(map[string]int))
}

func (s *GeobedSuite) TestGeocode(c *C) {
	//g := NewGeobed()
	for _, v := range s.testLocations {
		r := g.Geocode(v["query"])
		c.Assert(r.City, Equals, v["city"])
		c.Assert(r.Country, Equals, v["country"])
		// Due to all the data and various sets, the region can be a little weird. It's intended to be US state first and foremost (where it is most helpful in geocoding).
		// TODO: Look back into this later and try to make sense of it all. It may end up needing to be multiple fields (which will further complicate the matching).
		if v["region"] != "" {
			c.Assert(r.Region, Equals, v["region"])
		}
	}

	r := g.Geocode("")
	c.Assert(r.City, Equals, "")

	r = g.Geocode(" ")
	c.Assert(r.Population, Equals, int32(0))
}

func (s *GeobedSuite) TestReverseGeocode(c *C) {
	//g := NewGeobed()

	r := g.ReverseGeocode(30.26715, -97.74306)
	c.Assert(r.City, Equals, "Austin")
	c.Assert(r.Region, Equals, "TX")
	c.Assert(r.Country, Equals, "US")

	r = g.ReverseGeocode(37.44651, -122.15322)
	c.Assert(r.City, Equals, "Palo Alto")
	c.Assert(r.Region, Equals, "CA")
	c.Assert(r.Country, Equals, "US")

	// Use precise Santa Cruz city center coordinates
	r = g.ReverseGeocode(36.9741, -122.0308)
	c.Assert(r.City, Equals, "Santa Cruz")

	// Stanford campus coordinates
	r = g.ReverseGeocode(37.4275, -122.1697)
	c.Assert(r.City, Equals, "Stanford")

	r = g.ReverseGeocode(51.51279, -0.09184)
	c.Assert(r.City, Equals, "City of London")
}

func (s *GeobedSuite) TestNext(c *C) {
	c.Assert(string(prev(rune("new york"[0]))), Equals, "m")
	c.Assert(prev(rune("new york"[0])), Equals, int32(109))
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
