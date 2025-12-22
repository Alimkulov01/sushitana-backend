package structs

import "errors"

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
