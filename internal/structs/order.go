package structs

import "time"

type Order struct {
	ID                int64          `json:"id"`
	TgId              int64          `json:"tg_id"`
	PhoneNumber       string         `json:"phone_number"`
	Address           Address        `json:"address"`
	TolalPrice        float64        `json:"total_price"`
	DeliveryType      string         `json:"delivery_type"`
	PaymentMethod     string         `json:"payment_method"`
	DeliveryPrice     int64          `json:"delivery_price"`
	Products          []ProductOrder `json:"products"`
	OrderStatus       string         `json:"order_status"`
	LinkOrderInvoices string         `json:"link_order_invoices"`
	CreatedAT         time.Time      `json:"created_at"`
	UpdatedAT         time.Time      `json:"updated_at"`
}

type Address struct {
	Langitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	Address   string  `json:"address"`
	LinkMap   string  `json:"link_map"`
}

type ProductOrder struct {
	Count     int64 `json:"count"`
	ProductID int64 `json:"product_id"`
}

type CreateOrder struct {
	TgId          int64          `json:"tg_id"`
	PhoneNumber   string         `json:"phone_number"`
	Address       string         `json:"address"`
	PaymentMethod string         `json:"payment_method"`
	Products      []ProductOrder `json:"products"`
	DeliveryType  string         `json:"delivery_type"`
}

type UpdateStatusOrder struct {
	OrderID     int64  `json:"order_id"`
	OrderStatus string `json:"order_status"`
}

type GetListOrderRequest struct {
	Limit         int64  `json:"limit"`
	Offset        int64  `json:"offset"`
	OrderStatus   string `json:"order_status"`
	PhoneNumber   string `json:"phone_number"`
	PaymentMethod string `json:"payment_method"`
}

type GetListOrderResponse struct {
	Orders []Order `json:"orders"`
	Count  int64   `json:"count"`
}
