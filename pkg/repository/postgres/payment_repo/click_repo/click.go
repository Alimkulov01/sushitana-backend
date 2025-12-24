package clickrepo

import (
	"context"
	"database/sql"
	"errors"
	"sushitana/internal/structs"
	"sushitana/pkg/db"
	"sushitana/pkg/logger"
	"time"

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
		Create(ctx context.Context, req structs.Invoice) (int64, error)
		GetByMerchantTransID(ctx context.Context, merchantTransID string) (structs.Invoice, error)
		GetByInvoiceID(ctx context.Context, clickInvoiceID int64) (structs.Invoice, error)
		GetByClickTransID(ctx context.Context, clickTransID int64) (structs.Invoice, error)
		GetByPrepareID(ctx context.Context, merchantPrepareID int64) (structs.Invoice, error)
		GetInvoiceByTransID(ctx context.Context, transID string) (structs.ClickInvoice, error)
		UpsertPrepare(ctx context.Context, merchantTransID string, clickTransID, clickPaydocID int64, amount string) (merchantPrepareID int64, err error)
		UpdateOnComplete(ctx context.Context, merchantTransID string, merchantPrepareID int64, clickTransID int64, status string) (invoiceID string, orderID sql.NullString, err error)
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
func (r repo) Create(ctx context.Context, req structs.Invoice) (int64, error) {
	query := `
		INSERT INTO invoices (
			click_invoice_id,
			click_trans_id,
			click_paydoc_id,
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
		) RETURNING id
	`
	var id int64
	now := time.Now()
	if req.CreatedAt.IsZero() {
		req.CreatedAt = now
	}
	if req.UpdatedAt.IsZero() {
		req.UpdatedAt = now
	}

	err := r.db.QueryRow(ctx, query,
		req.ClickInvoiceID,
		req.ClickTransID,  // sql.NullInt64
		req.ClickPaydocID, // sql.NullInt64
		req.MerchantTransID,
		req.OrderID,       // sql.NullString
		req.TgID,          // sql.NullInt64
		req.CustomerPhone, // sql.NullString
		req.Amount,
		req.Currency,
		req.Status,
		req.Comment, // sql.NullString
		req.CreatedAt,
		req.UpdatedAt,
	).Scan(
		&id,
	)
	if err != nil {
		r.logger.Error(ctx, "failed to insert invoice", zap.Error(err))
		return 0, err
	}
	return id, nil
}

func (r repo) GetByMerchantTransID(ctx context.Context, merchantTransID string) (structs.Invoice, error) {
	var resp structs.Invoice

	query := `
		SELECT
			id,
			click_invoice_id,
			click_trans_id,
			click_paydoc_id,
			merchant_prepare_id,
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
		&resp.ClickPaydocID,
		&resp.MerchantPrepareID,
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
func (r repo) UpsertPrepare(ctx context.Context, merchantTransID string, clickTransID, clickPaydocID int64, amount string) (int64, error) {

	query := `
    UPDATE invoices
    SET
      click_trans_id   = $1,
      click_paydoc_id  = $2,
      amount           = COALESCE(NULLIF($3, '')::numeric, amount),
      updated_at       = now()
    WHERE merchant_trans_id = $4
    RETURNING merchant_prepare_id
  `

	var merchantPrepareID int64
	err := r.db.QueryRow(ctx, query, clickTransID, clickPaydocID, amount, merchantTransID).Scan(&merchantPrepareID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, structs.ErrNotFound
		}
		r.logger.Error(ctx, "UpsertPrepare failed", zap.Error(err))
		return 0, err
	}

	return merchantPrepareID, nil
}

func (r repo) UpdateOnComplete(ctx context.Context, merchantTransID string, merchantPrepareID int64, clickTransID int64, status string) (string, sql.NullString, error) {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", sql.NullString{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
    UPDATE invoices
    SET
      click_trans_id = $1,
      status        = $2,
      updated_at    = now()
    WHERE merchant_trans_id = $3
      AND merchant_prepare_id = $4
    RETURNING id, order_id
  `

	var invoiceID string
	var orderID sql.NullString
	err = tx.QueryRow(ctx, query, clickTransID, status, merchantTransID, merchantPrepareID).Scan(&invoiceID, &orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", sql.NullString{}, structs.ErrNotFound
		}
		return "", sql.NullString{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", sql.NullString{}, err
	}

	return invoiceID, orderID, nil
}

func (r repo) GetByInvoiceID(ctx context.Context, clickInvoiceID int64) (structs.Invoice, error) {
	var resp structs.Invoice

	query := `
		SELECT
			id,
			click_invoice_id,
			click_trans_id,
			click_paydoc_id,
			merchant_prepare_id,
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
		WHERE click_invoice_id = $1
		LIMIT 1
	`

	err := r.db.QueryRow(ctx, query, clickInvoiceID).Scan(
		&resp.ID,
		&resp.ClickInvoiceID,
		&resp.ClickTransID,
		&resp.ClickPaydocID,
		&resp.MerchantPrepareID,
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
		r.logger.Warn(ctx, "invoice not found or failed fetching by invoice_id", zap.Error(err))
		return structs.Invoice{}, err
	}

	return resp, nil
}

func (r repo) GetByClickTransID(ctx context.Context, clickTransID int64) (structs.Invoice, error) {
	var resp structs.Invoice

	query := `
		SELECT
			id,
			click_invoice_id,
			click_trans_id,
			click_paydoc_id,
			merchant_prepare_id,
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
		WHERE click_trans_id = $1
		LIMIT 1
	`

	err := r.db.QueryRow(ctx, query, clickTransID).Scan(
		&resp.ID,
		&resp.ClickInvoiceID,
		&resp.ClickTransID,
		&resp.ClickPaydocID,
		&resp.MerchantPrepareID,
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
		r.logger.Warn(ctx, "invoice not found or failed fetching by click_trans_id", zap.Error(err))
		return structs.Invoice{}, err
	}

	return resp, nil
}

func (r repo) GetByPrepareID(ctx context.Context, merchantPrepareID int64) (structs.Invoice, error) {
	var resp structs.Invoice

	query := `
		SELECT
			id,
			click_invoice_id,
			click_trans_id,
			click_paydoc_id,
			merchant_prepare_id,
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
		WHERE merchant_prepare_id = $1
		LIMIT 1
	`

	err := r.db.QueryRow(ctx, query, merchantPrepareID).Scan(
		&resp.ID,
		&resp.ClickInvoiceID,
		&resp.ClickTransID,
		&resp.ClickPaydocID,
		&resp.MerchantPrepareID,
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
		r.logger.Warn(ctx, "invoice not found or failed fetching by click_trans_id", zap.Error(err))
		return structs.Invoice{}, err
	}

	return resp, nil
}

func (r *repo) GetInvoiceByTransID(ctx context.Context, transID string) (structs.ClickInvoice, error) {
	var inv structs.ClickInvoice

	const query = `
		SELECT
			id,
			order_id,
			click_trans_id,
			merchant_trans_id,
			amount,
			status,
			created_at,
			updated_at
		FROM invoices
		WHERE merchant_trans_id = $1
		LIMIT 1
	`

	err := r.db.QueryRow(ctx, query, transID).Scan(
		&inv.ID,
		&inv.OrderID,
		&inv.ClickTransID,
		&inv.MerchantTransID,
		&inv.Amount,
		&inv.Status,
		&inv.CreatedAt,
		&inv.UpdatedAt,
	)
	if err != nil {
		return inv, err
	}
	return inv, nil
}
