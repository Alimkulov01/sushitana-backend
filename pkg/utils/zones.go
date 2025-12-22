package utils

import (
	"fmt"
	"os"
)

type ZoneChecker struct {
	zones [][]byte
}

func NewZoneCheckerFromFiles(paths ...string) (*ZoneChecker, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no geojson files provided")
	}

	z := make([][]byte, 0, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read geojson %q: %w", p, err)
		}
		z = append(z, b)
	}
	return &ZoneChecker{zones: z}, nil
}

func (c *ZoneChecker) ContainsAny(lat, lng float64) (bool, error) {
	for _, b := range c.zones {
		ok, err := IsPointInGeoJSON(b, lat, lng)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func (c *ZoneChecker) ContainsAnyWithIndex(lat, lng float64) (bool, int, error) {
	for i, b := range c.zones {
		ok, err := IsPointInGeoJSON(b, lat, lng)
		if err != nil {
			return false, -1, err
		}
		if ok {
			return true, i, nil
		}
	}
	return false, -1, nil
}
