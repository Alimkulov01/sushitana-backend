package paymerepo

import (
	"context"
	"database/sql"
	"errors"
	"sushitana/internal/structs"
	"sushitana/pkg/db"
	"sushitana/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Provide(New)

const (
	StateCreated           = 1
	StatePerformed         = 2
	StateCanceledCreated   = -1
	StateCanceledPerformed = -2
)

type (
	Params struct {
		fx.In
		Logger logger.Logger
		DB     db.Querier
	}

	Repo interface {
		Create(ctx context.Context, orderID string, paycomTransID string, amount string, createdTime int64) (structs.PaymeTransaction, error)
		GetByPaycomTransactionID(ctx context.Context, paycomTransID string) (structs.PaymeTransaction, error)
		MarkPerformed(ctx context.Context, paycomTransID string, performTime int64) (structs.PaymeTransaction, error)
		MarkCanceled(ctx context.Context, paycomTransID string, cancelTime int64, reason int, newState int) (structs.PaymeTransaction, error)
		GetStatement(ctx context.Context, from, to int64) ([]structs.PaymeTransaction, error)
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

func (r repo) Create(ctx context.Context, orderID string, paycomTransID string, amount string, createdTime int64) (structs.PaymeTransaction, error) {
	query := `
		INSERT INTO payme_transactions (
			id, 
			paycom_transaction_id, 
			order_id, 
			amount, 
			state, 
			created_time, 
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4::numeric, $5, $6, now(), now()
		)
		ON CONFLICT (paycom_transaction_id)
		DO UPDATE SET updated_at = now()
		RETURNING
			id, paycom_transaction_id, order_id, 
			amount::text, state, created_time,
			perform_time, cancel_time, 
			reason, created_at, updated_at
	`

	var tx structs.PaymeTransaction
	err := r.db.QueryRow(ctx, query,
		uuid.NewString(),
		paycomTransID,
		orderID,
		amount,
		StateCreated,
		createdTime,
	).Scan(
		&tx.ID,
		&tx.PaycomTransactionID,
		&tx.OrderID,
		&tx.Amount,
		&tx.State,
		&tx.CreatedTime,
		&tx.PerformTime,
		&tx.CancelTime,
		&tx.Reason,
		&tx.CreatedAt,
		&tx.UpdatedAt,
	)
	if err != nil {
		r.logger.Error(ctx, "payme Create failed", zap.Error(err))
		return structs.PaymeTransaction{}, err
	}
	return tx, nil
}

func (r repo) GetByPaycomTransactionID(ctx context.Context, paycomTransID string) (structs.PaymeTransaction, error) {
	query := `
		SELECT
			id, 
			paycom_transaction_id, 
			order_id, 
			amount::text, 
			state, 
			created_time,
			perform_time, 
			cancel_time, 
			reason, 
			created_at, 
			updated_at
		FROM payme_transactions
		WHERE paycom_transaction_id = $1
		LIMIT 1
	`

	var tx structs.PaymeTransaction
	err := r.db.QueryRow(ctx, query, paycomTransID).Scan(
		&tx.ID,
		&tx.PaycomTransactionID,
		&tx.OrderID,
		&tx.Amount,
		&tx.State,
		&tx.CreatedTime,
		&tx.PerformTime,
		&tx.CancelTime,
		&tx.Reason,
		&tx.CreatedAt,
		&tx.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return structs.PaymeTransaction{}, structs.ErrNotFound
		}
		r.logger.Error(ctx, "payme GetByPaycomTransactionID failed", zap.Error(err))
		return structs.PaymeTransaction{}, err
	}
	return tx, nil
}

func (r repo) MarkPerformed(ctx context.Context, paycomTransID string, performTime int64) (structs.PaymeTransaction, error) {
	query := `
		UPDATE payme_transactions
		SET state = $2,
		    perform_time = $3,
		    updated_at = now()
		WHERE paycom_transaction_id = $1
		RETURNING
			id, paycom_transaction_id, order_id, 
			amount::text, state, created_time,
			perform_time, cancel_time, 
			reason, created_at, updated_at
	`

	var tx structs.PaymeTransaction
	err := r.db.QueryRow(ctx, query, paycomTransID, StatePerformed, performTime).Scan(
		&tx.ID,
		&tx.PaycomTransactionID,
		&tx.OrderID,
		&tx.Amount,
		&tx.State,
		&tx.CreatedTime,
		&tx.PerformTime,
		&tx.CancelTime,
		&tx.Reason,
		&tx.CreatedAt,
		&tx.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return structs.PaymeTransaction{}, structs.ErrNotFound
		}
		return structs.PaymeTransaction{}, err
	}
	return tx, nil
}

func (r repo) MarkCanceled(ctx context.Context, paycomTransID string, cancelTime int64, reason int, newState int) (structs.PaymeTransaction, error) {
	query := `
		UPDATE payme_transactions
		SET state = $2,
		    cancel_time = $3,
		    reason = $4,
		    updated_at = now()
		WHERE paycom_transaction_id = $1
		RETURNING
			id, paycom_transaction_id, order_id, 
			amount::text, state, created_time,
			perform_time, cancel_time, 
			reason, created_at, updated_at
	`

	var tx structs.PaymeTransaction
	err := r.db.QueryRow(ctx, query, paycomTransID, newState, cancelTime, reason).Scan(
		&tx.ID,
		&tx.PaycomTransactionID,
		&tx.OrderID,
		&tx.Amount,
		&tx.State,
		&tx.CreatedTime,
		&tx.PerformTime,
		&tx.CancelTime,
		&tx.Reason,
		&tx.CreatedAt,
		&tx.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return structs.PaymeTransaction{}, structs.ErrNotFound
		}
		return structs.PaymeTransaction{}, err
	}
	return tx, nil
}

func (r repo) GetStatement(ctx context.Context, from, to int64) ([]structs.PaymeTransaction, error) {
	query := `
		SELECT
			id, 
			paycom_transaction_id, 
			order_id, 
			amount::text, 
			state, 
			created_time,
			perform_time, 
			cancel_time, 
			reason, 
			created_at, 
			updated_at
		FROM payme_transactions
		WHERE created_time BETWEEN $1 AND $2
		ORDER BY created_time ASC
	`

	rows, err := r.db.Query(ctx, query, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []structs.PaymeTransaction
	for rows.Next() {
		var tx structs.PaymeTransaction
		if err := rows.Scan(
			&tx.ID,
			&tx.PaycomTransactionID,
			&tx.OrderID,
			&tx.Amount,
			&tx.State,
			&tx.CreatedTime,
			&tx.PerformTime,
			&tx.CancelTime,
			&tx.Reason,
			&tx.CreatedAt,
			&tx.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, tx)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
