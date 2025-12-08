package orderrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"sushitana/internal/structs"
	"sushitana/pkg/db"
	"sushitana/pkg/logger"

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
		Create(ctx context.Context, req structs.CreateOrder) error
		GetByTgId(ctx context.Context, tgId int64) (structs.GetListOrderByTgIDResponse, error)
		GetByID(ctx context.Context, id string) (structs.Order, error)
		GetList(ctx context.Context, req structs.GetListOrderRequest) (structs.GetListOrderResponse, error)
		Delete(ctx context.Context, order_id string) error
		UpdateStatus(ctx context.Context, req structs.UpdateStatus) error
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

	var (
		status        string
		paymentStatus string
		id            = uuid.NewString()
	)
	switch req.PaymentMethod {
	case "CASH":
		status = "WAITING_OPERATOR"
		paymentStatus = "UNPAID"
	case "CLICK", "PAYME":
		status = "WAITING_PAYMENT"
		paymentStatus = "PENDING"
	default:
		return fmt.Errorf("unsupported payment method: %s", req.PaymentMethod)
	}

	query := `
		INSERT INTO orders (
			id,
			tg_id,
			delivery_type,
			payment_method,
			payment_status,
			order_status,
			address,
			comment,
			iiko_order_id,
			iiko_delivery_id,
			items
		) VALUES ($1, $2::bigint, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	if _, err := r.db.Exec(ctx, query,
		id,
		req.TgID,
		req.DeliveryType,
		req.PaymentMethod,
		paymentStatus,
		status,
		req.Address,
		req.Comment,
		req.IIKOOrderID,
		req.IIKODeliveryID,
		req.Products,
	); err != nil {
		r.logger.Error(ctx, "err on r.db.Exec", zap.Error(err))
		return fmt.Errorf("create order failed: %w", err)
	}

	r.logger.Info(ctx, "order created", zap.Any("tg_id", req.TgID), zap.String("status", status))
	return nil
}

func (r repo) getProductPrice(ctx context.Context, productID string) (int64, error) {
	query := `
        SELECT (size_prices->0->'price'->>'currentPrice')::bigint
        FROM product
        WHERE id = $1
    `
	var price int64
	err := r.db.QueryRow(ctx, query, productID).Scan(&price)
	if err != nil {
		return 0, err
	}
	return price, nil
}
func (r repo) GetByTgId(ctx context.Context, tgId int64) (resp structs.GetListOrderByTgIDResponse, err error) {
	r.logger.Info(ctx, "Get orders by tgId", zap.Any("tgId", tgId))

	query := `
        SELECT 
            id,
            tg_id,
            delivery_type,
            payment_method,
            payment_status,
            order_status,
            address,
            comment,
            iiko_order_id,
            iiko_delivery_id,
            items,
            delivery_price,
            created_at,
            updated_at
        FROM orders
        WHERE tg_id = $1
        ORDER BY created_at DESC
    `

	rows, err := r.db.Query(ctx, query, tgId)
	if err != nil {
		return resp, fmt.Errorf("get orders failed: %w", err)
	}
	defer rows.Close()

	var totalItems int64

	for rows.Next() {
		var order structs.Order
		var addrBytes, itemsBytes []byte

		if err := rows.Scan(
			&order.ID,
			&order.TgID,
			&order.DeliveryType,
			&order.PaymentMethod,
			&order.PaymentStatus,
			&order.Status,
			&addrBytes,
			&order.Comment,
			&order.IIKOOrderID,
			&order.IIKODeliveryID,
			&itemsBytes,
			&order.DeliveryPrice,
			&order.CreatedAt,
			&order.UpdateAt,
		); err != nil {
			return resp, fmt.Errorf("scan order failed: %w", err)
		}

		_ = json.Unmarshal(addrBytes, &order.Address)
		_ = json.Unmarshal(itemsBytes, &order.Products)
		var orderTotal int64 = 0

		for _, p := range order.Products {

			price, err := r.getProductPrice(ctx, p.ID)
			if err != nil {
				r.logger.Warn(ctx, "Price not found for product", zap.String("productId", p.ID))
				continue
			}

			orderTotal += price * p.Quantity
			totalItems += p.Quantity
		}
		order.TotalCount = totalItems
		order.TotalPrice = orderTotal + order.DeliveryPrice

		resp.Orders = append(resp.Orders, order)
	}

	return resp, nil
}

func (r repo) GetByID(ctx context.Context, id string) (structs.Order, error) {
	r.logger.Info(ctx, "Get order by ID", zap.String("id", id))

	query := `
		SELECT 
			id,
			tg_id,
			delivery_type,
			payment_method,
			payment_status,
			order_status,
			address,
			comment,
			iiko_order_id,
			iiko_delivery_id,
			items,
			created_at,
			updated_at
		FROM orders
		WHERE id = $1
	`

	var order structs.Order
	if err := r.db.QueryRow(ctx, query, id).Scan(
		&order.ID,
		&order.TgID,
		&order.DeliveryType,
		&order.PaymentMethod,
		&order.PaymentStatus,
		&order.Status,
		&order.Address,
		&order.Comment,
		&order.IIKOOrderID,
		&order.IIKODeliveryID,
		&order.Products,
		&order.CreatedAt,
		&order.UpdateAt,
	); err != nil {
		r.logger.Error(ctx, "err on r.db.QueryRow.Scan", zap.Error(err))
		return structs.Order{}, fmt.Errorf("get order by ID failed: %w", err)
	}

	r.logger.Info(ctx, "order retrieved", zap.String("id", id))
	return order, nil
}

func (r repo) GetList(ctx context.Context, req structs.GetListOrderRequest) (resp structs.GetListOrderResponse, err error) {
	r.logger.Info(ctx, "Get order list", zap.Any("req", req))

	var (
		query = `
		SELECT
			COUNT(*) OVER(),
			id,
			tg_id,
			delivery_type,
			payment_method,
			payment_status,
			order_status,
			address,
			comment,
			iiko_order_id,
			iiko_delivery_id,
			items,
			created_at,
			updated_at
		FROM orders
	`
		where  = " WHERE TRUE"
		offset = " OFFSET 0"
		limit  = " LIMIT 10"
		sort   = " ORDER BY created_at DESC"
	)

	args := []interface{}{}
	argIndex := 1

	if req.Offset > 0 {
		offset = fmt.Sprintf(" OFFSET %d", req.Offset)
	}

	if req.Limit > 0 {
		limit = fmt.Sprintf(" LIMIT %d", req.Limit)
	}

	if len(req.Status) > 0 {
		where += fmt.Sprintf(" AND order_status::text ILIKE $%d", argIndex)
		args = append(args, "%"+req.Status+"%")
		argIndex++
	}
	if len(req.DeliveryType) > 0 {
		where += fmt.Sprintf(" AND delivery_type::text ILIKE $%d", argIndex)
		args = append(args, "%"+req.DeliveryType+"%")
		argIndex++
	}
	if len(req.PaymentMethod) > 0 {
		where += fmt.Sprintf(" AND payment_method::text ILIKE $%d", argIndex)
		args = append(args, "%"+req.PaymentMethod+"%")
		argIndex++
	}
	if len(req.CreatedAt) > 0 {
		where += fmt.Sprintf(" AND created_at::text ILIKE $%d", argIndex)
		args = append(args, "%"+req.CreatedAt+"%")
		argIndex++
	}
	if len(req.PaymentStatus) > 0 {
		where += fmt.Sprintf(" AND payment_status::text ILIKE $%d", argIndex)
		args = append(args, "%"+req.PaymentStatus+"%")
		argIndex++
	}

	query += where + sort + limit + offset

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error(ctx, "err on r.db.Query", zap.Error(err))
		return structs.GetListOrderResponse{}, fmt.Errorf("get order list failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var order structs.Order
		if err := rows.Scan(
			&resp.Count,
			&order.ID,
			&order.TgID,
			&order.DeliveryType,
			&order.PaymentMethod,
			&order.PaymentStatus,
			&order.Status,
			&order.Address,
			&order.Comment,
			&order.IIKOOrderID,
			&order.IIKODeliveryID,
			&order.Products,
			&order.CreatedAt,
			&order.UpdateAt,
		); err != nil {
			r.logger.Error(ctx, "err on rows.Scan", zap.Error(err))
			return structs.GetListOrderResponse{}, fmt.Errorf("scan order failed: %w", err)
		}

		resp.Orders = append(resp.Orders, order)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error(ctx, "err on rows.Err", zap.Error(err))
		return structs.GetListOrderResponse{}, fmt.Errorf("rows error: %w", err)
	}

	r.logger.Info(ctx, "order list retrieved", zap.Int("count", len(resp.Orders)))
	return resp, nil
}

func (r repo) Delete(ctx context.Context, order_id string) error {
	r.logger.Info(ctx, "Delete orders by order_id", zap.Any("order_id", order_id))

	query := `
		DELETE FROM orders
		WHERE id = $1
	`

	if _, err := r.db.Exec(ctx, query, order_id); err != nil {
		r.logger.Error(ctx, "err on r.db.Exec", zap.Error(err))
		return fmt.Errorf("delete orders failed: %w", err)
	}

	r.logger.Info(ctx, "orders deleted", zap.Any("order_id", order_id))
	return nil
}

func (r repo) UpdateStatus(ctx context.Context, req structs.UpdateStatus) error {
	r.logger.Info(ctx, "Update order status", zap.String("orderId", req.OrderId), zap.String("status", req.Status))

	query := `
		UPDATE orders
		SET order_status = $2
		WHERE id = $1
	`
	rowsAffected, err := r.db.Exec(ctx, query, req.OrderId, req.Status)
	if err != nil {
		r.logger.Error(ctx, "err on r.db.Exec", zap.Error(err))
		return fmt.Errorf("update order status failed: %w", err)
	}

	if rowsAffected.RowsAffected() == 0 {
		r.logger.Warn(ctx, "no order found to update", zap.String("orderId", req.OrderId))
		return fmt.Errorf("no order found with id: %s", req.OrderId)
	}

	r.logger.Info(ctx, "order status updated", zap.String("orderId", req.OrderId), zap.String("status", req.Status))
	return nil
}
