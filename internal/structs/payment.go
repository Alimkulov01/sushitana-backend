package structs

type CreateInvoiceRequest struct {
	ServiceId             int     `json:"service_id"`
	Amount                float64 `json:"amount"`
	PhoneNumber           string  `json:"phone_number"`
	MerchantTransactionId string  `json:"merchant_trans_id"`
}

type CreateInvoiceResponse struct {
	ErrorCode int    `json:"error_code,omitempty"`
	ErrorNote string `json:"error_note,omitempty"`
	InvoiceId int64  `json:"invoice_id,omitempty"`
}

type ClickRequest struct {
	ClickTransID      int64   `form:"click_trans_id" json:"click_trans_id"`           // ID tranzaksii v CLICK
	ServiceID         int64   `form:"service_id" json:"service_id"`                   // ID servisa
	ClickPaydocID     int64   `form:"click_paydoc_id" json:"click_paydoc_id"`         // ID plateja v CLICK
	MerchantTransID   string  `form:"merchant_trans_id" json:"merchant_trans_id"`     // Order ID siz tomonda
	MerchantPrepareID int64   `form:"merchant_prepare_id" json:"merchant_prepare_id"` // Faqat complete da keladi
	Amount            float64 `form:"amount" json:"amount"`
	Action            int     `form:"action" json:"action"` // 0 = prepare, 1 = complete
	Error             int     `form:"error" json:"error"`
	ErrorNote         string  `form:"error_note" json:"error_note"`
	SignTime          string  `form:"sign_time" json:"sign_time"` // "YYYY-MM-DD HH:mm:ss"
	SignString        string  `form:"sign_string" json:"sign_string"`
}

type ClickPrepareResponse struct {
	ClickTransID      int64  `json:"click_trans_id"`
	MerchantTransID   string `json:"merchant_trans_id"`
	MerchantPrepareID int64  `json:"merchant_prepare_id"` // sizning payment ID
	Error             int    `json:"error"`
	ErrorNote         string `json:"error_note"`
}

type ClickCompleteResponse struct {
	ClickTransID      int64  `json:"click_trans_id"`
	MerchantTransID   string `json:"merchant_trans_id"`
	MerchantConfirmID int64  `json:"merchant_confirm_id"` // sizning payment ID
	Error             int    `json:"error"`
	ErrorNote         string `json:"error_note"`
}
