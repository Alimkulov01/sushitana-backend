package orderrepo

import (
	"context"
	"fmt"
	"sushitana/internal/structs"
	"sushitana/pkg/db"
	"sushitana/pkg/logger"

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
		Create(ctx context.Context, req structs.CreateOrder) error
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

func (r repo) Create(ctx context.Context, req structs.CreateOrder) error {
	r.logger.Info(ctx, "Create order", zap.Any("req", req))

	var status string
	switch req.PaymentMethod {
	case "cash":
		status = "new"
	case "click", "payme":
		status = "pending_payment"
	default:
		return fmt.Errorf("unsupported payment method: %s", req.PaymentMethod)
	}

	query := `
		INSERT INTO orders (
			tg_id,
			phone_number,
			address,
			total_price,
			delivery_type,
			payment_method,
			products,
			order_status
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	if _, err := r.db.Exec(ctx, query,
		req.TgId,
		req.PhoneNumber,
		req.Address,
		req.PaymentMethod,
		req.Products,
		status,
	); err != nil {
		r.logger.Error(ctx, "err on r.db.Exec", zap.Error(err))
		return fmt.Errorf("create order failed: %w", err)
	}

	r.logger.Info(ctx, "order created", zap.Any("tg_id", req.TgId), zap.String("status", status))
	return nil
}
