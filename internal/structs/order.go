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
	OrderNumber    int64          `json:"order_number"`
	TotalPrice     int64          `json:"totalPrice"`
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

type IikoCreateSettings struct {
	TransportToFrontTimeout int  `json:"transportToFrontTimeout,omitempty"`
	CheckStopList           bool `json:"checkStopList,omitempty"`
}

// Root request
type IikoCreateDeliveryRequest struct {
	OrganizationId      string              `json:"organizationId"`
	TerminalGroupId     string              `json:"terminalGroupId"`
	CreateOrderSettings *IikoCreateSettings `json:"createOrderSettings,omitempty"`
	Order               IikoOrder           `json:"order"`
}

type IikoOrder struct {
	Phone          string `json:"phone"`
	ExternalNumber string `json:"externalNumber,omitempty"`
	OrderTypeId    string `json:"orderTypeId"`

	Comment  string          `json:"comment,omitempty"`
	Items    []IikoOrderItem `json:"items"`
	Payments []IikoPayment   `json:"payments,omitempty"`
}

type IikoOrderItem struct {
	Type      string  `json:"type"`      // "Product"
	ProductId string  `json:"productId"` // iiko nomenclature GUID bo‘lishi kerak
	Amount    float64 `json:"amount"`
}

type IikoPayment struct {
	PaymentTypeId         string  `json:"paymentTypeId"`
	PaymentTypeKind       string  `json:"paymentTypeKind"` // Cash / External / Card (iiko settingga bog‘liq)
	Sum                   float64 `json:"sum"`
	IsProcessedExternally bool    `json:"isProcessedExternally,omitempty"`
}

type IikoCreateDeliveryResponse struct {
	CorrelationId string        `json:"correlationId"`
	OrderInfo     IikoOrderInfo `json:"orderInfo"`
}

type IikoOrderInfo struct {
	ID             string         `json:"id"`
	PosID          string         `json:"posId"`
	ExternalNumber string         `json:"externalNumber"`
	OrganizationId string         `json:"organizationId"`
	Timestamp      int64          `json:"timestamp"`
	CreationStatus string         `json:"creationStatus"`
	ErrorInfo      *IikoErrorInfo `json:"errorInfo"`
}

type IikoErrorInfo struct {
	Code        string `json:"code,omitempty"`
	Description string `json:"description,omitempty"`
}
