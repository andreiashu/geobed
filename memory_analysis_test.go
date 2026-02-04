package geobed

import (
	"fmt"
	"runtime"
	"testing"
	"unsafe"
)

func TestMemoryFootprint(t *testing.T) {
	g, err := NewGeobed()
	if err != nil {
		t.Fatal(err)
	}

	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)

	var city GeobedCity
	structSize := unsafe.Sizeof(city)

	fmt.Printf("Cities loaded: %d\n", len(g.c))
	fmt.Printf("GeobedCity size: %d bytes\n", structSize)
	fmt.Printf("Heap in use: %d MB\n", m.Alloc/1024/1024)
	fmt.Printf("Countries indexed: %d\n", len(countryLookup))
	fmt.Printf("Regions indexed: %d\n", len(regionLookup))
}
