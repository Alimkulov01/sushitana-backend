package clickrepo

import (
	"context"
	"database/sql"
	"errors"
	"sushitana/internal/structs"
	"sushitana/pkg/db"
	"sushitana/pkg/logger"
	"time"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Params struct {
		fx.In
		Logger logger.Logger
		DB     db.Querier
	}

	Repo interface {
		Create(ctx context.Context, req structs.Invoice) error
		GetByMerchantTransID(ctx context.Context, merchantTransID string) (structs.Invoice, error)
		UpdateStatus(ctx context.Context, id string, status string) error
		GetByRequestId(ctx context.Context, requestId string) (structs.Invoice, error)
		UpdateOnComplete(ctx context.Context, requestId string, clickTransID int64, status string) error
	}

	repo struct {
		logger logger.Logger
		db     db.Querier
	}
)

func New(p Params) Repo {
	return &repo{
		logger: p.Logger,
		db:     p.DB,
	}
}

func (r repo) Create(ctx context.Context, req structs.Invoice) error {
	query := `
		INSERT INTO invoices (
			id,
			click_invoice_id,
			click_trans_id,
			merchant_trans_id,
			order_id,
			tg_id,
			customer_phone,
			amount,
			currency,
			status,
			comment,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, 
			$6, $7, $8, $9, $10, 
			$11, $12, $13
		)
	`

	now := time.Now()
	if req.CreatedAt.IsZero() {
		req.CreatedAt = now
	}
	if req.UpdatedAt.IsZero() {
		req.UpdatedAt = now
	}
	id := uuid.NewString()
	_, err := r.db.Exec(ctx, query,
		id,                  // $1  -> id (UUID)
		req.ClickInvoiceID,  // $2  -> click_invoice_id BIGINT
		req.ClickTransID,    // $3  -> click_trans_id BIGINT (0 bo‘lishi mumkin)
		req.MerchantTransID, // $4  -> merchant_trans_id VARCHAR
		req.OrderID,         // $5  -> order_id UUID (bo‘sh bo‘lsa NULL bo‘ladi)
		req.TgID,            // $6  -> tg_id BIGINT
		req.CustomerPhone,   // $7  -> customer_phone VARCHAR
		req.Amount,          // $8  -> amount NUMERIC(12,2)
		req.Currency,        // $9  -> currency VARCHAR(3)
		req.Status,          // $10 -> status VARCHAR
		req.Comment,         // $11 -> comment TEXT
		req.CreatedAt,       // $12 -> created_at TIMESTAMP
		req.UpdatedAt,       // $13 -> updated_at TIMESTAMP
	)
	if err != nil {
		r.logger.Error(ctx, "failed to insert invoice", zap.Error(err))
		return err
	}

	return nil
}

func (r repo) GetByMerchantTransID(ctx context.Context, merchantTransID string) (structs.Invoice, error) {
	var resp structs.Invoice

	query := `
		SELECT
			id,
			click_invoice_id,
			click_trans_id,
			merchant_trans_id,
			order_id,
			tg_id,
			customer_phone,
			amount,
			currency,
			status,
			comment,
			created_at,
			updated_at
		FROM invoices
		WHERE merchant_trans_id = $1
		LIMIT 1
	`

	err := r.db.QueryRow(ctx, query, merchantTransID).Scan(
		&resp.ID,
		&resp.ClickInvoiceID,
		&resp.ClickTransID,
		&resp.MerchantTransID,
		&resp.OrderID,
		&resp.TgID,
		&resp.CustomerPhone,
		&resp.Amount,
		&resp.Currency,
		&resp.Status,
		&resp.Comment,
		&resp.CreatedAt,
		&resp.UpdatedAt,
	)

	if err != nil {
		r.logger.Warn(ctx, "invoice not found or failed fetching", zap.Error(err))
		return structs.Invoice{}, err
	}

	return resp, nil
}

func (r repo) UpdateStatus(ctx context.Context, id string, status string) error {
	query := `
		UPDATE invoices
		SET status = $2,
		    updated_at = now()
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, id, status)
	if err != nil {
		r.logger.Error(ctx, "failed to update invoice status", zap.Error(err))
		return err
	}

	return nil
}

func (r repo) GetByRequestId(ctx context.Context, requestId string) (structs.Invoice, error) {
	var resp structs.Invoice

	query := `
		SELECT
			id,
			click_invoice_id,
			click_trans_id,
			merchant_trans_id,
			order_id,
			tg_id,
			customer_phone,
			amount,
			currency,
			status,
			comment,
			created_at,
			updated_at
		FROM invoices
		WHERE click_request_id = $1
		LIMIT 1
	`

	err := r.db.QueryRow(ctx, query, requestId).Scan(
		&resp.ID,
		&resp.ClickInvoiceID,
		&resp.ClickTransID,
		&resp.MerchantTransID,
		&resp.OrderID,
		&resp.TgID,
		&resp.CustomerPhone,
		&resp.Amount,
		&resp.Currency,
		&resp.Status,
		&resp.Comment,
		&resp.CreatedAt,
		&resp.UpdatedAt,
	)

	if err != nil {
		r.logger.Warn(ctx, "invoice not found or failed fetching", zap.Error(err))
		return structs.Invoice{}, err
	}

	return resp, nil
}

func (r repo) UpdateOnComplete(ctx context.Context, requestId string, clickTransID int64, status string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	query := `
		UPDATE invoices
		SET click_trans_id = $1, status = $2, updated_at = now()
		WHERE request_id = $3
		RETURNING id, order_id
	`
	var invoiceID string
	var orderID sql.NullString
	err = tx.QueryRow(ctx, query, clickTransID, status, requestId).Scan(&invoiceID, &orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return structs.ErrNotFound
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}
