package structs

import "time"

type EventType string

const (
	EventOrderPatch     EventType = "order.patch"     // eng yengil: faqat status/paymentStatus
	EventOrderUpsert    EventType = "order.upsert"    // ixtiyoriy: full order
	EventOrdersSnapshot EventType = "orders.snapshot" // ixtiyoriy: connect bo‘lganda
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
