package structs

import "time"

type Order struct {
	ID             string         `json:"id"`
	TgID           int64          `json:"tgId"`
	Address        Address        `json:"address"`
	DeliveryType   string         `json:"deliveryType"`
	PaymentMethod  string         `json:"paymentMethod"`
	PaymentStatus  string         `json:"paymentStatus"`
	Products       []OrderProduct `json:"products"`
	DeliveryPrice  int64          `json:"deliveryPrice"`
	Status         string         `json:"status"`
	Comment        string         `json:"comment"`
	IIKOOrderID    string         `json:"iikoOrderId"`
	IIKODeliveryID string         `json:"iikDeliveryId"`
	TotalCount     int64          `json:"totalCount"`
	TotalPrice     int64          `json:"totalPrice"`
	ProductName    Name           `json:"product_name"`
	ProductUrl     string         `json:"product_url"`
	ProductPrice   int64          `json:"product_price"`
	OrderNumber    int64          `json:"order_number"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdateAt       time.Time      `json:"updateAt"`
}

type GetListOrderByTgIDResponse struct {
	Orders []Order `json:"orders"`
}

type Address struct {
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	Name       string  `json:"name"`
	DistanceKm float64 `json:"distanceKm"`
}

type OrderProduct struct {
	ID       string `json:"id"`
	Quantity int64  `json:"quantity"`
}

type CreateOrder struct {
	TgID           int64          `json:"tgId"`
	Address        Address        `json:"address"`
	DeliveryType   string         `json:"deliveryType"`
	PaymentMethod  string         `json:"paymentMethod"`
	Products       []OrderProduct `json:"products"`
	DeliveryPrice  int64          `json:"deliveryPrice"`
	Comment        string         `json:"comment"`
	IIKOOrderID    string         `json:"iikoOrderId"`
	IIKODeliveryID string         `json:"iikDeliveryId"`
}

type GetListOrderRequest struct {
	Limit         int64  `json:"limit"`
	Offset        int64  `json:"offset"`
	Status        string `json:"status"`
	PaymentStatus string `json:"paymentStatus"`
	DeliveryType  string `json:"deliveryType"`
	PaymentMethod string `json:"paymentMethod"`
	OrderNumber   int64  `json:"order_number"`
	CreatedAt     string `json:"createdAt"`
}

type GetListOrderResponse struct {
	Count  int64   `json:"count"`
	Orders []Order `json:"orders"`
}

type UpdateStatus struct {
	OrderId string `json:"orderId"`
	Status  string `json:"status"`
}
