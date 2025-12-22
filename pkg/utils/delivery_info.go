package utils

import (
	"fmt"
	"math"
)

const (
	DeliveryPriceOlmaliq   int64 = 7000 
	DeliveryPriceOhangaron int64 = 25000 
)

var RestaurantLat = 40.855373
var RestaurantLng = 69.615734

type DeliveryInfo struct {
	Available  bool
	Price      int64
	DistanceKm float64
	Reason     string

	ZoneIndex int    
	ZoneName  string 
}

func DistanceKm(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLng := (lng2 - lng1) * math.Pi / 180.0

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180.0)*math.Cos(lat2*math.Pi/180.0)*
			math.Sin(dLng/2)*math.Sin(dLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

func GetDeliveryInfo(zones *ZoneChecker, lat, lng float64, addressText string) DeliveryInfo {
	if zones == nil {
		return DeliveryInfo{
			Available: false,
			Reason:    "Delivery zones not configured",
		}
	}
	if lat == 0 || lng == 0 {
		return DeliveryInfo{
			Available: false,
			Reason:    "Location is required",
		}
	}

	ok, idx, err := zones.ContainsAnyWithIndex(lat, lng)
	if err != nil {
		return DeliveryInfo{
			Available: false,
			Reason:    fmt.Sprintf("Zone check failed: %v", err),
		}
	}
	if !ok {
		return DeliveryInfo{
			Available: false,
			Reason:    "Bu hududga yetkazib berilmaydi",
			ZoneIndex: -1,
		}
	}

	info := DeliveryInfo{
		Available:  true,
		DistanceKm: DistanceKm(RestaurantLat, RestaurantLng, lat, lng),
		ZoneIndex:  idx,
	}

	switch idx {
	case 0:
		info.ZoneName = "olmaliq"
		info.Price = DeliveryPriceOlmaliq
	case 1:
		info.ZoneName = "ohangaron"
		info.Price = DeliveryPriceOhangaron
	default:
		info.Available = false
		info.Reason = "Unknown delivery zone"
	}

	return info
}
