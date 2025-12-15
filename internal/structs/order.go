package structs

import (
	"fmt"
	"strings"
	"time"
)

const (
	DeliveryTypeDelivery = "DELIVERY"
	DeliveryTypePickup   = "PICKUP"

	PaymentMethodCash  = "CASH"
	PaymentMethodClick = "CLICK"
	PaymentMethodPayme = "PAYME"
)

func NormalizeDeliveryType(v string) (string, error) {
	s := strings.TrimSpace(strings.ToUpper(v))
	switch s {
	case "DELIVERY":
		return "DELIVERY", nil
	case "PICKUP":
		return "PICKUP", nil
	default:
		return "", fmt.Errorf("invalid deliveryType: %q", v)
	}
}

func NormalizePaymentMethod(v string) (string, error) {
	s := strings.TrimSpace(strings.ToUpper(v))
	switch s {
	case "CASH":
		return "CASH", nil
	case "PAYME":
		return "PAYME", nil
	case "CLICK":
		return "CLICK", nil
	default:
		return "", fmt.Errorf("invalid paymentMethod: %q", v)
	}
}

func ToIikoPaymentKind(method string) (string, error) {
	m, err := NormalizePaymentMethod(method)
	if err != nil {
		return "", err
	}
	if m == PaymentMethodCash {
		return IikoPaymentKindCash, nil
	}
	return IikoPaymentKindOnline, nil
}

const (
	IikoPaymentKindCash   = "CASH"
	IikoPaymentKindOnline = "ONLINE"
)

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
	OrderNumber    int64          `json:"order_number"`
	Phone          string         `json:"phone,omitempty"`
	PaymentUrl     string         `json:"payment_url"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdateAt       time.Time      `json:"updateAt"`
}

type GetListPrimaryKeyResponse struct {
	Phone string `json:"phone"`
	Order Order  `json:"order"`
}

type GetListOrderByTgIDResponse struct {
	Phone  string  `json:"phone"`
	Orders []Order `json:"orders"`
}

type Address struct {
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	Name       string  `json:"name"`
	DistanceKm float64 `json:"distanceKm"`
}

type OrderProduct struct {
	ID           string `json:"id"`
	Quantity     int64  `json:"quantity"`
	ProductName  Name   `json:"product_name"`
	ProductUrl   string `json:"product_url"`
	ProductPrice int64  `json:"product_price"`
}

type CreateOrder struct {
	TgID           int64          `json:"tgId"`
	Address        *Address       `json:"address"`
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
	PhoneNumber   string `json:"phone_number"`
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

type IikoOrderItem struct {
	ProductId string  `json:"productId"`
	Amount    float64 `json:"amount"`
}

type IikoOrder struct {
	OrganizationId string          `json:"organizationId"`
	OrderTypeId    string          `json:"orderTypeId,omitempty"` // pickup/delivery turi
	PaymentTypeId  string          `json:"paymentTypeId,omitempty"`
	Phone          string          `json:"phone,omitempty"`
	Comment        string          `json:"comment,omitempty"`
	Items          []IikoOrderItem `json:"items"`
}

type IikoDeliveryCreateRequest struct {
	OrganizationId      string `json:"organizationId"`
	CreateOrderSettings struct {
		TransportToFrontTimeout int32 `json:"transportToFrontTimeout"`
	} `json:"createOrderSettings"`
	Order IikoOrder `json:"order"`
}

type IikoDeliveryCreateResponse struct {
	CorrelationId string `json:"correlationId"`
	OrderInfo     struct {
		Id             string  `json:"id"`
		ExternalNumber string  `json:"externalNumber"`
		FullSum        float64 `json:"fullSum"`
		Status         string  `json:"status"`
	} `json:"orderInfo"`
}
