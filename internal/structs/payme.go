package structs

import (
	"database/sql"
	"time"
)

type PaymeTransaction struct {
	ID                  string
	PaycomTransactionID string
	OrderID             string
	Amount              string // NUMERIC ni string qilib ushlash xavfsiz
	State               int
	CreatedTime         int64
	PerformTime         sql.NullInt64
	CancelTime          sql.NullInt64
	Reason              sql.NullInt64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type PaymeCheckPerformParams struct {
	Amount  int64   `json:"amount"`
	Account Account `json:"account"`
}

type Account struct {
	OrderID string `json:"order_id"`
	ID      string `json:"id,omitempty"`
	Phone   string `json:"phone,omitempty"`
}

type PaymeCheckPerformResult struct {
	Allow bool `json:"allow"`
}

type PaymeCreateParams struct {
	Id      string  `json:"id"`
	Time    int64   `json:"time"`
	Amount  int64   `json:"amount"`
	Account Account `json:"account"`
}

type PaymeCreateResult struct {
	CreateTime  int64  `json:"create_time"`
	Transaction string `json:"transaction"`
	State       int    `json:"state"`
}

type PaymePerformParams struct {
	Id string `json:"id"`
}

type PaymePerformResult struct {
	Transaction string `json:"transaction"`
	PerformTime int64  `json:"perform_time"`
	State       int    `json:"state"`
}

type PaymeCancelParams struct {
	Id     string `json:"id"`
	Reason int    `json:"reason"`
}

type PaymeCancelResult struct {
	Transaction string `json:"transaction"`
	CancelTime  int64  `json:"cancel_time"`
	State       int    `json:"state"`
}

type PaymeCheckParams struct {
	Id string `json:"id"`
}

type PaymeCheckResult struct {
	Transaction string `json:"transaction"`
	State       int    `json:"state"`
	CreateTime  int64  `json:"create_time"`
	PerformTime int64  `json:"perform_time"`
	CancelTime  int64  `json:"cancel_time"`
	Reason      *int   `json:"reason"` // nil => null
}

type PaymeStatementParams struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

type PaymeStatementResult struct {
	Transactions []Transaction `json:"transactions"`
}

type Transaction struct {
	Id          string  `json:"id"`
	Time        int64   `json:"time"`
	Amount      int64   `json:"amount"`
	Account     Account `json:"account"`
	CreateTime  int64   `json:"create_time"`
	PerformTime int64   `json:"perform_time"`
	CancelTime  int64   `json:"cancel_time"`
	Transaction string  `json:"transaction"`
	State       int     `json:"state"`
	Reason *int `json:"reason"`
}
