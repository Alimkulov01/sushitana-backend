package structs

import (
	"time"
)


type CheckoutPrepareRequest struct {
	ServiceID        string `json:"service_id"`
	MerchatID        string `json:"merchant_id"`
	TransactionParam string `json:"transaction_param"`
	Amount           string `json:"amount"`
	ReturnUrl        string `json:"return_url"`
	Source           string `json:"source"`
	Description      string `json:"description"`
	TotalPrice       int64  `json:"total_price"`
}
type Item struct {
	Name  string `json:"name"`
	Price int64  `json:"price"`
}

type CheckoutPrepareResponse struct {
	ErrorCode int64  `json:"error_code"`
	ErrorNote string `json:"error_note"`
	RequestId string `json:"request_id"`
}

type CheckoutInvoiceRequest struct {
	RequestId   string `json:"request_id"`
	PhoneNumber string `json:"phone_number"`
}

type CheckoutInvoiceResponse struct {
	ErrorCode int64  `json:"error_code"`
	ErrorNote string `json:"error_note"`
	InvoiceId int64  `json:"invoice_id"`
}

type RetrieveResponse struct {
	RequestId         string  `json:"request_id"`
	ServiceId         int64   `json:"service_id"`
	MerchantId        int64   `json:"merchant_id"`
	ServiceName       string  `json:"service_name"`
	TransactionParam  string  `json:"transaction_param"`
	Amount            int64   `json:"amount"`
	Language          string  `json:"language"`
	ReturnUrl         string  `json:"return_url"`
	CommissionPercent int64   `json:"commission_percent"`
	Payment           Payment `json:"payment"`
}

type Payment struct {
	PaymentStatusDescription string `json:"payment_status_description"`
	PaymentId                string `json:"payment_id"`
	PaymentStatus            int64  `json:"payment_status"`
	IsInvoice                int64  `json:"is_invoice"`
	PhoneNumber              string `json:"phone_number"`
}

// invoices jadvaliga mos struct
type Invoice struct {
	ID string `json:"id"` // UUID

	ClickInvoiceID  int64  `json:"click_invoice_id"`  // Shop API da hozir 0 turadi
	ClickTransID    int64  `json:"click_trans_id"`    // complete callback kelganda to'ldirasan
	MerchantTransID string `json:"merchant_trans_id"` // order ID yoki transaction_param

	OrderID string `json:"order_id"` // orders jadvalidagi id
	TgID    int64  `json:"tg_id"`    // user tg_id bo'lsa

	CustomerPhone string  `json:"customer_phone"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"` // UZS

	Status string `json:"status"` // CREATED, PENDING, PAID, FAILED

	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
