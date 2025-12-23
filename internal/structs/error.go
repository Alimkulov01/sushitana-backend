package structs

import (
	"errors"
	"fmt"
)

var (
	ErrBadRequest        = errors.New("bad request")
	ErrNoRowsAffected    = errors.New("no rows affected")
	ErrNotFound          = errors.New("no rows in result set")
	ErrUserBlocked       = errors.New("user blocked")
	ErrUniqueViolation   = errors.New("unique Violation error")
	ErrWhiteList         = errors.New("account in whitelist")
	ErrAlreadyBooked     = errors.New("allready booked")
	ErrOutOfDeliveryZone = errors.New("the specified location is out of delivery.")
)

type ErrMinOrder struct {
	ZoneKey string // masalan: "OHANGARON"
	Min     int64
	Current int64
}

func (e ErrMinOrder) Error() string {
	return fmt.Sprintf("min order not reached: zone=%s min=%d current=%d", e.ZoneKey, e.Min, e.Current)
}
