package structs

import (
	"sushitana/pkg/utils"
	"time"
)

type Client struct {
	ID                  int64      `json:"id"`
	TgID                int64      `json:"tgid"`
	Phone               string     `json:"phone"`
	Language            utils.Lang `json:"language"`
	Name                string     `json:"name"`
	OrderCount          int64      `json:"order_count"`
	CompletedOrderCount int64      `json:"completed_order_count"`
	CanceledOrderCount  int64      `json:"canceled_order_count"`
	IsActive            bool       `json:"is_active"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

type CreateClient struct {
	TgID  int64  `json:"tgid"`
	Phone string `json:"phone"`
}

type GetListClientRequest struct {
	Offset      int64  `json:"offset"`
	Limit       int64  `json:"limit"`
	PhoneNumber string `json:"phone_number"`
	Name        string `json:"name"`
}

type GetListClientResponse struct {
	Count   int64    `json:"count"`
	Clients []Client `json:"clients"`
}
