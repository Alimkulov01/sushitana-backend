package structs

import (
	"database/sql"
	"time"
)

type ClickPrepareRequest struct {
	ClickTransId    int64  `form:"click_trans_id" binding:"required"`
	ServiceId       int64  `form:"service_id" binding:"required"`
	ClickPaydocId   int64  `form:"click_paydoc_id" binding:"required"`
	MerchantTransId string `form:"merchant_trans_id"`
	Amount          string `form:"amount" binding:"required"`
	Action          *int   `form:"action" binding:"required,oneof=0 1"`
	Error           *int   `form:"error" binding:"required"`
	ErrorNote       string `form:"error_note"`
	SignTime        string `form:"sign_time" binding:"required"`
	SignString      string `form:"sign_string" binding:"required"`
}
type ClickPrepareResponse struct {
	ClickTransId      int64  `json:"click_trans_id"`
	MerchantTransId   string `form:"merchant_trans_id"`
	MerchantPrepareId int64  `json:"merchant_prepare_id"`
	Error             int    `json:"error"`
	ErrorNote         string `json:"error_note"`
}

type ClickCompleteRequest struct {
	ClickTransId      int64  `form:"click_trans_id" binding:"required"`
	ServiceId         int64  `form:"service_id" binding:"required"`
	ClickPaydocId     int64  `form:"click_paydoc_id" binding:"required"`
	MerchantTransId   string `form:"merchant_trans_id"`
	MerchantPrepareId int64  `form:"merchant_prepare_id" binding:"required"`
	Amount            string `form:"amount" binding:"required"`
	Action            *int   `form:"action" binding:"required,oneof=1"` // 1
	Error             *int   `form:"error" binding:"required"`
	ErrorNote         string `form:"error_note"`
	SignTime          string `form:"sign_time" binding:"required"`
	SignString        string `form:"sign_string" binding:"required"`
}

type ClickCompleteResponse struct {
	ClickTransId      int64  `json:"click_trans_id"`
	MerchantTransId   string `form:"merchant_trans_id"`
	MerchantConfirmId int64  `json:"merchant_confirm_id"`
	Error             int    `json:"error"`
	ErrorNote         string `json:"error_note"`
}

type Invoice struct {
	ID                int64
	ClickInvoiceID    int64
	ClickTransID      int64
	ClickPaydocID     int64
	MerchantPrepareID int64
	MerchantTransID   string
	OrderID           sql.NullString
	TgID              sql.NullInt64
	CustomerPhone     sql.NullString
	Amount            string
	Currency          string
	Status            string
	Comment           sql.NullString
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CheckoutPrepareRequest struct {
	ServiceID        string      `json:"service_id"`
	MerchantID       string      `json:"merchant_id"`
	TransactionParam string      `json:"transaction_param"`
	Amount           float64     `json:"amount"`
	ReturnUrl        string      `json:"return_url,omitempty"`
	Description      string      `json:"description,omitempty"`
	Items            interface{} `json:"items,omitempty"`
}

type CheckoutPrepareResponse struct {
	RequestId string `json:"request_id"`
	ErrorCode int    `json:"error_code"`
	ErrorNote string `json:"error_note"`
}

type CheckoutInvoiceRequest struct {
	RequestId   string `json:"request_id"`
	PhoneNumber string `json:"phone_number"`
}

type CheckoutInvoiceResponse struct {
	InvoiceId int64  `json:"invoice_id"`
	ErrorCode int    `json:"error_code"`
	ErrorNote string `json:"error_note"`
}

type RetrieveResponse struct {
	ErrorCode int    `json:"error_code"`
	ErrorNote string `json:"error_note"`
}

type CreateInvoiceRequest struct {
	ServiceID       int64   `json:"service_id"`
	MerchantTransId string  `json:"merchant_trans_id"`
	Amount          float64 `json:"amount"`
	PhoneNumber     string  `json:"phone_number"`
}

type CreateInvoiceResponse struct {
	InvoiceId int64  `json:"invoice_id"`
	ErrorCode int    `json:"error_code"`
	ErrorNote string `json:"error_note"`
}

type InvoiceStatusResponse struct {
	ErrorCode         int    `json:"error_code"`
	ErrorNote         string `json:"error_note"`
	InvoiceStatus     int64  `json:"invoice_status"`
	InvoiceStatusNote string `json:"invoice_status_note"`
}

type ClickInvoiceStatusResponse struct {
	ErrorCode         int    `json:"error_code"`
	ErrorNote         string `json:"error_note"`
	InvoiceStatus     int    `json:"invoice_status"`
	InvoiceStatusNote string `json:"invoice_status_note"`
}

type UpdatePaymentStatus struct {
	OrderId string `json:"orderId"`
	Status  string `json:"status"` // PAID / UNPAID / PENDING ...
}

type ClickPaymentAttempt struct {
	ID                int64 // shu ID -> merchant_prepare_id bo‘ladi
	MerchantTransID   string
	ClickTransID      int64
	ClickPaydocID     int64
	Amount            string
	State             string // "PENDING" | "PAID" | "CANCELLED"
	MerchantConfirmID int64
	Error             int
	ErrorNote         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
