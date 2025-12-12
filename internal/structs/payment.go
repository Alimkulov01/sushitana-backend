package structs

import "time"

// CheckoutPrepareRequest — prepare endpoint uchun so'rov modeli.
// Eslatma: JSON tag lar Click hujjatlariga mos bo'lishi kerak.
// Merk: Amount int64 (tushunarliroq) — agar Click so‘rovi string talab qilsa originalga moslang.
type CheckoutPrepareRequest struct {
	ServiceID        string `json:"service_id"`
	MerchantID       string `json:"merchant_id"` // typo tuzatildi (MerchatID -> MerchantID)
	TransactionParam string `json:"transaction_param"`
	Amount           int64  `json:"amount"`
	ReturnUrl        string `json:"return_url"`
	Source           string `json:"source,omitempty"`
	Description      string `json:"description,omitempty"`
	// TotalPrice may be internal field — agar Clickga kerak bo'lsa qoldiring
	TotalPrice int64  `json:"total_price,omitempty"`
	Items      []Item `json:"items,omitempty"`
}

// Item — mahsulot haqida qisqacha ma'lumot (agar Click API itemlarni qabul qilsa)
type Item struct {
	Name  string `json:"name"`
	Price int64  `json:"price"`
	Qty   int64  `json:"qty,omitempty"`
}

// CheckoutPrepareResponse — prepare javobi
type CheckoutPrepareResponse struct {
	ErrorCode int64  `json:"error_code"`
	ErrorNote string `json:"error_note"`
	RequestId string `json:"request_id"`
}

// CheckoutInvoiceRequest — invoice yaratish uchun so'rov
type CheckoutInvoiceRequest struct {
	RequestId   string `json:"request_id"`
	PhoneNumber string `json:"phone_number"`
}

// CheckoutInvoiceResponse — invoice yaratish javobi
type CheckoutInvoiceResponse struct {
	ErrorCode int64  `json:"error_code"`
	ErrorNote string `json:"error_note"`
	InvoiceId int64  `json:"invoice_id"`
}

// RetrieveResponse — Click retrieve endpoint javobi
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

// Invoice — invoices jadvaliga mos model (DB bilan moslashgan).
type Invoice struct {
	ID string `json:"id"` // UUID

	ClickInvoiceID  int64  `json:"click_invoice_id"`
	ClickTransID    int64  `json:"click_trans_id"`
	MerchantTransID string `json:"merchant_trans_id"` // order ID yoki transaction_param

	OrderID string `json:"order_id"` // orders jadvalidagi id
	TgID    int64  `json:"tg_id"`

	CustomerPhone string  `json:"customer_phone"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"` // UZS

	Status string `json:"status"` // CREATED, PENDING, PAID, FAILED

	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CompleteCallbackPayload struct {
	ErrorCode       int64  `json:"error_code"`
	ErrorNote       string `json:"error_note"`
	RequestId       string `json:"request_id"`
	ClickTransId    int64  `json:"click_trans_id"`
	MerchantTransId string `json:"merchant_trans_id"`
	Amount          int64  `json:"amount"`
	Action          int    `json:"action"`
	Sign            string `json:"sign"`
}
