package delivery

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Info struct {
	Available  bool    `json:"available"`
	DistanceKm float64 `json:"distanceKm"`
	Price      int64   `json:"price"`            // so'm
	Reason     string  `json:"reason,omitempty"` // agar available=false bo'lsa
}

// Sizdagi front coords
const RestaurantLat = 40.855373
const RestaurantLng = 69.615734

const MaxDistanceKm = 8.0

const Price0To5Km int64 = 7000
const Price5To7Km int64 = 7000

const OhangaronPrice int64 = 25000

func toRad(deg float64) float64 { return deg * math.Pi / 180 }

func DistanceKm(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371.0 // km
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)

	a :=
		math.Sin(dLat/2)*math.Sin(dLat/2) +
			math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*
				math.Sin(dLng/2)*math.Sin(dLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

func containsOhangaron(addressName string) bool {
	s := strings.ToLower(strings.TrimSpace(addressName))
	if s == "" {
		return false
	}
	keywords := []string{
		"ohangaron",
		"ohangarоn", 
		"охангарон",
	}
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func GetDeliveryInfo(customerLat, customerLng float64, addressName string) Info {
	d := DistanceKm(RestaurantLat, RestaurantLng, customerLat, customerLng)

	if d > MaxDistanceKm {
		return Info{
			Available:  false,
			DistanceKm: d,
			Price:      0,
			Reason:     fmt.Sprintf("Yetkazib berish %.0f km dan uzoq manzillarga mavjud emas", MaxDistanceKm),
		}
	}

	if containsOhangaron(addressName) {
		return Info{
			Available:  true,
			DistanceKm: d,
			Price:      OhangaronPrice,
		}
	}

	switch {
	case d <= 5:
		return Info{Available: true, DistanceKm: d, Price: Price0To5Km}
	case d <= 7:
		return Info{Available: true, DistanceKm: d, Price: Price5To7Km}
	default: // 7–8
		return Info{Available: true, DistanceKm: d, Price: Price5To7Km}
	}
}

func ParseCoord(s string) (float64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid coord %q: %w", s, err)
	}
	return v, nil
}
