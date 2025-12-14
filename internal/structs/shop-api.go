package structs

type PayLinkParams struct {
	ServiceID        string `json:"service_id"        form:"service_id"        binding:"required"`
	MerchantID       string `json:"merchant_id"       form:"merchant_id"       binding:"required"`
	Amount           string `json:"amount"            form:"amount"            binding:"required"`
	TransactionParam string `json:"transaction_param" form:"transaction_param" binding:"required"`

	MerchantUserID string `json:"merchant_user_id,omitempty" form:"merchant_user_id"`
	ReturnURL      string `json:"return_url,omitempty"       form:"return_url"`
	CardType       string `json:"card_type,omitempty"        form:"card_type"`
}

// Click callback: Prepare (action=0)
type PrepareRequest struct {
	ClickTransID    int64  `form:"click_trans_id"  binding:"required"`
	ServiceID       int64  `form:"service_id"     binding:"required"`
	ClickPaydocID   int64  `form:"click_paydoc_id" binding:"required"`
	MerchantTransID string `form:"merchant_trans_id" binding:"required"`
	Amount          string `form:"amount"         binding:"required"`
	Action          int    `form:"action"         binding:"required"` // must be 0
	Error           int    `form:"error"          binding:"required"`
	ErrorNote       string `form:"error_note"     binding:"required"`
	SignTime        string `form:"sign_time"      binding:"required"`
	SignString      string `form:"sign_string"    binding:"required"`
}

type PrepareResponse struct {
	ClickTransID      int64  `json:"click_trans_id"`
	MerchantTransID   string `json:"merchant_trans_id"`
	MerchantPrepareID int64  `json:"merchant_prepare_id"`
	Error             int    `json:"error"`
	ErrorNote         string `json:"error_note"`
}

// Click callback: Complete (action=1)
type CompleteRequest struct {
	ClickTransID      int64  `form:"click_trans_id"  binding:"required"`
	ServiceID         int64  `form:"service_id"     binding:"required"`
	ClickPaydocID     int64  `form:"click_paydoc_id" binding:"required"`
	MerchantTransID   string `form:"merchant_trans_id" binding:"required"`
	MerchantPrepareID int64  `form:"merchant_prepare_id" binding:"required"`
	Amount            string `form:"amount"         binding:"required"`
	Action            int    `form:"action"         binding:"required"` // must be 1
	Error             int    `form:"error"          binding:"required"`
	ErrorNote         string `form:"error_note"     binding:"required"`
	SignTime          string `form:"sign_time"      binding:"required"`
	SignString        string `form:"sign_string"    binding:"required"`
}

type CompleteResponse struct {
	ClickTransID      int64  `json:"click_trans_id"`
	MerchantTransID   string `json:"merchant_trans_id"`
	MerchantConfirmID int64  `json:"merchant_confirm_id"`
	Error             int    `json:"error"`
	ErrorNote         string `json:"error_note"`
}

const (
	ErrSuccess            = 0
	ErrSignCheckFailed    = -1
	ErrIncorrectAmount    = -2
	ErrActionNotFound     = -3
	ErrAlreadyPaid        = -4
	ErrUserOrOrderMissing = -5
	ErrTransactionMissing = -6
	ErrFailedToUpdate     = -7
	ErrClickRequestError  = -8
	ErrCancelled          = -9
)

type MerchantConfig struct {
	MerchantUserID string `json:"merchant_user_id"`
	SecretKey      string `json:"secret_key"`
}

type StatusByMTIRequest struct {
	MerchantTransID string `json:"merchant_trans_id" binding:"required"`
	ServiceID       int64  `json:"service_id"        binding:"required"`
}

type StatusByMTIResponse struct {
	ErrorCode       int    `json:"error_code"`
	ErrorNote       string `json:"error_note"`
	PaymentID       int64  `json:"payment_id"`
	MerchantTransID string `json:"merchant_trans_id"`
	PaymentStatus   int    `json:"payment_status,omitempty"`
}

type PaymentStatusRequest struct {
	PaymentID int64 `json:"payment_id" binding:"required"`
	ServiceID int64 `json:"service_id" binding:"required"`
}

type PaymentStatusResponse struct {
	ErrorCode     int    `json:"error_code"`
	ErrorNote     string `json:"error_note"`
	PaymentID     int64  `json:"payment_id"`
	PaymentStatus int    `json:"payment_status"`
}

type ReversalResponse struct {
	ErrorCode int    `json:"error_code"`
	ErrorNote string `json:"error_note"`
	PaymentID int64  `json:"payment_id"`
}
