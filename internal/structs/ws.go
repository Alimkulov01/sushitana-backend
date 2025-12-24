package structs

import "time"

type EventType string

const (
	EventOrderPatch     EventType = "order.patch"     // eng yengil: faqat status/paymentStatus
	EventOrderUpsert    EventType = "order.upsert"    // ixtiyoriy: full order
	EventOrdersSnapshot EventType = "orders.snapshot" // ixtiyoriy: connect bo‘lganda
	EventOrderRemove    EventType = "order.remove"
)

type Event struct {
	Type EventType `json:"type"`
	TS   time.Time `json:"ts"`
	// Payload format: type ga qarab
	Payload any `json:"payload,omitempty"`
}

// Frontga faqat status/paymentStatus yangilash uchun eng yengil payload
type OrderPatchPayload struct {
	ID            string `json:"id"`
	Status        string `json:"status,omitempty"`
	PaymentStatus string `json:"paymentStatus,omitempty"`
	UpdateAt      string `json:"updateAt,omitempty"`
	OrderNumber   int64  `json:"order_number,omitempty"`
}

type OrderDTO struct {
	ID                string         `json:"id"`
	TgID              int64          `json:"tgId"`
	Address           *Address       `json:"address"`
	DeliveryType      string         `json:"deliveryType"`
	PaymentMethod     string         `json:"paymentMethod"`
	PaymentStatus     string         `json:"paymentStatus"`
	Products          []OrderProduct `json:"products"`
	DeliveryPrice     int64          `json:"deliveryPrice"`
	Status            string         `json:"status"`
	Comment           string         `json:"comment"`
	IIKOOrderID       string         `json:"iikoOrderId"`
	IIKODeliveryID    string         `json:"iikDeliveryId"`
	TotalCount        int64          `json:"totalCount"`
	TotalPrice        int64          `json:"totalPrice"`
	OrderNumber       int64          `json:"order_number"`
	Phone             string         `json:"phone,omitempty"`
	PaymentUrl        string         `json:"payment_url"`
	OrderPriceForIIKO int64          `json:"order_price_for_iiko"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdateAt          time.Time      `json:"updateAt"`
}

type OrdersSnapshotPayload struct {
	Orders []OrderDTO `json:"orders"`
	Total  int        `json:"total,omitempty"`
}

type OrderUpsertPayload struct {
	Order OrderDTO `json:"order"`
}

type OrderRemovePayload struct {
	ID string `json:"id"`
}
