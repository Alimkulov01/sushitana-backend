package utils

import (
	"math"
)

func HaversineDistance[T float64](lat1, lon1, lat2, lon2 T) T {
	R := 6371.0 // Earth radius in kilometers, you can use 3959.0 for miles

	// Convert latitude and longitude from degrees to radians
	lat1, lon1, lat2, lon2 = degToRad(lat1), degToRad(lon1), degToRad(lat2), degToRad(lon2)

	// Differences in coordinates
	dlat := lat2 - lat1
	dlon := lon2 - lon1

	// Haversine formula
	a := math.Sin(float64(dlat/2))*math.Sin(float64(dlat/2)) + math.Cos(float64(lat1))*math.Cos(float64(lat2))*math.Sin(float64(dlon/2))*math.Sin(float64(dlon/2))
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := R * c

	return T(distance)
}

func degToRad[T float64](deg T) T {
	return deg * (math.Pi / 180)
}
