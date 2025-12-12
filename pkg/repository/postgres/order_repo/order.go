package orderrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
		Create(ctx context.Context, req structs.CreateOrder) (string, error)
		GetByTgId(ctx context.Context, tgId int64) (structs.GetListOrderByTgIDResponse, error)
		GetByID(ctx context.Context, id string) (structs.GetListPrimaryKeyResponse, error)
		GetList(ctx context.Context, req structs.GetListOrderRequest) (structs.GetListOrderResponse, error)
		Delete(ctx context.Context, order_id string) error
		UpdateStatus(ctx context.Context, req structs.UpdateStatus) error
		UpdateClickInfo(ctx context.Context, orderID, requestID, transactionParam string) error
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

func (r repo) Create(ctx context.Context, req structs.CreateOrder) (id string, err error) {
	r.logger.Info(ctx, "Create order", zap.Any("req", req))

	var (
		status        string
		paymentStatus string
	)
	id = uuid.NewString()
	switch req.PaymentMethod {
	case "CASH":
		status = "WAITING_OPERATOR"
		paymentStatus = "UNPAID"
	case "CLICK", "PAYME":
		status = "WAITING_PAYMENT"
		paymentStatus = "PENDING"
	default:
		return "", fmt.Errorf("unsupported payment method: %s", req.PaymentMethod)
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
			delivery_price,
			items
		) VALUES ($1, $2::bigint, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
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
		req.DeliveryPrice,
		req.Products,
	); err != nil {
		r.logger.Error(ctx, "err on r.db.Exec", zap.Error(err))
		return "", fmt.Errorf("create order failed: %w", err)
	}

	r.logger.Info(ctx, "order created", zap.Any("tg_id", req.TgID), zap.String("status", status))
	return id, err
}

func (r repo) getProductPrice(ctx context.Context, productID string) (int64, error) {
	query := `
        SELECT (size_prices->0->'price'->>'currentPrice')::bigint
        FROM product
        WHERE id = $1
    `
	var (
		price int64
	)
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
            o.id,
            o.tg_id,
            o.delivery_type,
            o.payment_method,
            o.payment_status,
            o.order_status,
            o.address,
            o.comment,
            o.iiko_order_id,
            o.iiko_delivery_id,
            o.items,
			o.order_number,
			o.delivery_price,
            o.created_at,
            o.updated_at,
			c.phone
        FROM orders as o
		JOIN clients as c ON c.tgid = o.tg_id
        WHERE o.tg_id = $1
        ORDER BY o.created_at DESC
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
			&order.OrderNumber,
			&order.DeliveryPrice,
			&order.CreatedAt,
			&order.UpdateAt,
			&resp.Phone,
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
		totalItems = 0
		order.TotalPrice = orderTotal + order.DeliveryPrice

		resp.Orders = append(resp.Orders, order)
	}

	return resp, nil
}

func (r repo) GetByID(ctx context.Context, id string) (resp structs.GetListPrimaryKeyResponse, err error) {
	r.logger.Info(ctx, "Get order by ID", zap.String("id", id))

	query := `
		SELECT 
			o.id,
			o.tg_id,
			o.delivery_type,
			o.payment_method,
			o.payment_status,
			o.order_status,
			o.address,
			o.comment,
			o.iiko_order_id,
			o.iiko_delivery_id,
			o.items,
			o.delivery_price,
			o.order_number,
			o.created_at,
			o.updated_at,
			c.phone
		FROM orders as o
		JOIN clients as c ON c.tgid=o.tg_id
		WHERE o.id = $1
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
		&order.DeliveryPrice,
		&order.OrderNumber,
		&order.CreatedAt,
		&order.UpdateAt,
		&resp.Phone,
	); err != nil {
		r.logger.Error(ctx, "err on r.db.QueryRow.Scan", zap.Error(err))
		return structs.GetListPrimaryKeyResponse{}, fmt.Errorf("get order by ID failed: %w", err)
	}

	var (
		totalItems int64
		orderTotal int64 = 0
	)
	for _, p := range order.Products {

		price, err := r.getProductPrice(ctx, p.ID)
		if err != nil {
			r.logger.Warn(ctx, "Price not found for product", zap.String("productId", p.ID))
			continue
		}

		orderTotal += price * p.Quantity
	}
	order.TotalCount = totalItems
	order.TotalPrice = orderTotal + order.DeliveryPrice

	r.logger.Info(ctx, "order retrieved", zap.String("id", id))
	resp.Order = order
	return resp, nil
}

func (r repo) GetList(ctx context.Context, req structs.GetListOrderRequest) (resp structs.GetListOrderResponse, err error) {
	r.logger.Info(ctx, "Get order list", zap.Any("req", req))

	query := `
		SELECT
			COUNT(o.*) OVER(),
			o.id,
			o.tg_id,
			o.delivery_type,
			o.payment_method,
			o.payment_status,
			o.order_status,
			o.address,
			o.comment,
			o.iiko_order_id,
			o.iiko_delivery_id,
			o.items,
			o.delivery_price,
			o.order_number,
			o.created_at,
			o.updated_at,
			c.phone
		FROM orders AS o
		JOIN clients AS c ON c.tgid = o.tg_id
	`

	where := " WHERE TRUE"
	offset := " OFFSET 0"
	limit := " LIMIT 10"
	sort := " ORDER BY o.created_at DESC"

	args := []interface{}{}
	argIndex := 1

	if req.Offset > 0 {
		offset = fmt.Sprintf(" OFFSET %d", req.Offset)
	}

	if req.Limit > 0 {
		limit = fmt.Sprintf(" LIMIT %d", req.Limit)
	}

	if len(req.Status) > 0 {
		where += fmt.Sprintf(" AND o.order_status::text ILIKE $%d", argIndex)
		args = append(args, "%"+req.Status+"%")
		argIndex++
	}
	if len(req.DeliveryType) > 0 {
		where += fmt.Sprintf(" AND o.delivery_type::text ILIKE $%d", argIndex)
		args = append(args, "%"+req.DeliveryType+"%")
		argIndex++
	}
	if len(req.PaymentMethod) > 0 {
		where += fmt.Sprintf(" AND o.payment_method::text ILIKE $%d", argIndex)
		args = append(args, "%"+req.PaymentMethod+"%")
		argIndex++
	}
	if len(req.CreatedAt) > 0 {
		where += fmt.Sprintf(" AND o.created_at::date = $%d::date", argIndex)
		args = append(args, req.CreatedAt)
		argIndex++
	}
	if len(strings.TrimSpace(req.PaymentStatus)) > 0 {
		where += fmt.Sprintf(" AND o.payment_status::text ILIKE $%d", argIndex)
		args = append(args, strings.TrimSpace(req.PaymentStatus))
		argIndex++
	}
	if req.OrderNumber > 0 {
		where += fmt.Sprintf(" AND o.order_number = $%d", argIndex)
		args = append(args, req.OrderNumber)
		argIndex++
	}
	if len(req.PhoneNumber) > 0 {
		where += fmt.Sprintf(" AND c.phone ILIKE $%d", argIndex)
		args = append(args, "%"+req.PhoneNumber+"%")
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
		var (
			order                 structs.Order
			addrBytes, itemsBytes []byte
			totalCount            int64
		)

		if err := rows.Scan(
			&totalCount,
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
			&order.OrderNumber,
			&order.CreatedAt,
			&order.UpdateAt,
			&order.Phone,
		); err != nil {
			r.logger.Error(ctx, "err on rows.Scan", zap.Error(err))
			return structs.GetListOrderResponse{}, fmt.Errorf("scan order failed: %w", err)
		}

		resp.Count = totalCount

		if err := json.Unmarshal(addrBytes, &order.Address); err != nil {
			r.logger.Warn(ctx, "failed to unmarshal address", zap.Error(err))
		}
		if err := json.Unmarshal(itemsBytes, &order.Products); err != nil {
			r.logger.Warn(ctx, "failed to unmarshal items", zap.Error(err))
		}

		var orderTotal int64
		var itemCount int64

		for _, p := range order.Products {
			price, err := r.getProductPrice(ctx, p.ID)
			if err != nil {
				r.logger.Warn(ctx, "Price not found for product", zap.String("productId", p.ID))
				continue
			}
			orderTotal += price * p.Quantity
			itemCount += p.Quantity
		}

		order.TotalCount = itemCount
		order.TotalPrice = orderTotal + order.DeliveryPrice

		resp.Orders = append(resp.Orders, order)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error(ctx, "err on rows.Err", zap.Error(err))
		return structs.GetListOrderResponse{}, fmt.Errorf("rows error: %w", err)
	}

	r.logger.Info(ctx, "order list retrieved",
		zap.Int("phone_groups", len(resp.Orders)),
		zap.Int64("total_orders", resp.Count),
	)

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

func (r repo) UpdateClickInfo(ctx context.Context, orderID, requestID, transactionParam string) error {
	query := `
        UPDATE orders
        SET 
            click_request_id = $1,
            click_transaction_param = $2,
            click_prepare_at = NOW()
        WHERE id = $3
    `
	_, err := r.db.Exec(ctx, query, requestID, transactionParam, orderID)
	return err
}
