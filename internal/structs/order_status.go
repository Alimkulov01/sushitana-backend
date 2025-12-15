package structs

const (
	OrderStatusWaitingOperator = "WAITING_OPERATOR"
	OrderStatusWaitingPayment  = "WAITING_PAYMENT"
	OrderStatusCooking         = "COOKING"
	OrderStatusReadyForPickup  = "READY_FOR_PICKUP"
	OrderStatusOnTheWay        = "ON_THE_WAY"
	OrderStatusDelivered       = "DELIVERED"
	OrderStatusCompleted       = "COMPLETED"
	OrderStatusCancelled       = "CANCELLED"
	OrderStatusRejected        = "REJECTED"
)

const (
	DeliveryTypeDelivery = "DELIVERY" // delivery
	DeliveryTypePickup   = "PICKUP"   // pickup
)

const (
	PaymentMethodCash  = "CASH"
	PaymentMethodPayme = "PAYME"
	PaymentMethodClick = "CLICK"
)
