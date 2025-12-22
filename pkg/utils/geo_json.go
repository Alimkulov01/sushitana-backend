package utils

import (
	"encoding/json"
	"fmt"
)

type geojsonBase struct {
	Type string `json:"type"`
}

type featureCollection struct {
	Type     string    `json:"type"`
	Features []feature `json:"features"`
}

type feature struct {
	Type     string    `json:"type"`
	Geometry *geometry `json:"geometry"`
}

type geoFeature struct {
	Type     string    `json:"type"`
	Geometry *geometry `json:"geometry"`
}

type geometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

func IsPointInGeoJSON(geojsonBytes []byte, lat, lng float64) (bool, error) {
	var base geojsonBase
	if err := json.Unmarshal(geojsonBytes, &base); err != nil {
		return false, fmt.Errorf("invalid geojson: %w", err)
	}

	switch base.Type {
	case "FeatureCollection":
		var fc featureCollection
		if err := json.Unmarshal(geojsonBytes, &fc); err != nil {
			return false, fmt.Errorf("invalid FeatureCollection: %w", err)
		}
		for _, f := range fc.Features {
			if f.Geometry == nil {
				continue
			}
			ok, err := pointInGeometry(*f.Geometry, lat, lng)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil

	case "Feature":
		var ft geoFeature
		if err := json.Unmarshal(geojsonBytes, &ft); err != nil {
			return false, fmt.Errorf("invalid Feature: %w", err)
		}
		if ft.Geometry == nil {
			return false, nil
		}
		return pointInGeometry(*ft.Geometry, lat, lng)

	case "Polygon", "MultiPolygon":
		var g geometry
		if err := json.Unmarshal(geojsonBytes, &g); err != nil {
			return false, fmt.Errorf("invalid geometry: %w", err)
		}
		return pointInGeometry(g, lat, lng)

	default:
		return false, fmt.Errorf("unsupported geojson type: %s", base.Type)
	}
}

func pointInGeometry(g geometry, lat, lng float64) (bool, error) {
	switch g.Type {
	case "Polygon":
		var poly [][][]float64 
		if err := json.Unmarshal(g.Coordinates, &poly); err != nil {
			return false, fmt.Errorf("bad Polygon coordinates: %w", err)
		}
		return pointInPolygon(poly, lat, lng), nil

	case "MultiPolygon":
		var mp [][][][]float64
		if err := json.Unmarshal(g.Coordinates, &mp); err != nil {
			return false, fmt.Errorf("bad MultiPolygon coordinates: %w", err)
		}
		for _, poly := range mp {
			if pointInPolygon(poly, lat, lng) {
				return true, nil
			}
		}
		return false, nil

	default:
		return false, nil
	}
}

func pointInPolygon(rings [][][]float64, lat, lng float64) bool {
	if len(rings) == 0 {
		return false
	}
	x, y := lng, lat 

	if !pointInRing(x, y, rings[0]) {
		return false
	}
	for i := 1; i < len(rings); i++ {
		if pointInRing(x, y, rings[i]) {
			return false
		}
	}
	return true
}

func pointInRing(x, y float64, ring [][]float64) bool {
	n := len(ring)
	if n < 3 {
		return false
	}

	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := ring[i][0], ring[i][1]
		xj, yj := ring[j][0], ring[j][1]

		intersects := ((yi > y) != (yj > y)) &&
			(x < (xj-xi)*(y-yi)/(yj-yi)+xi)

		if intersects {
			inside = !inside
		}
		j = i
	}
	return inside
}
