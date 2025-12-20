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
		GetByMerchantTransId(ctx context.Context, id string) (structs.Order, error)
		GetByOrderNumber(ctx context.Context, number int64) (structs.Order, error)
		GetList(ctx context.Context, req structs.GetListOrderRequest) (structs.GetListOrderResponse, error)
		Delete(ctx context.Context, order_id string) error
		UpdateStatus(ctx context.Context, req structs.UpdateStatus) error
		AddLink(ctx context.Context, link, order_id string) error
		UpdatePaymentStatus(ctx context.Context, req structs.UpdateStatus) error
		UpdateClickInfo(ctx context.Context, orderID, requestID, transactionParam string) error
		UpdateIikoMeta(ctx context.Context, orderID, iikoOrderID, iikoPosID, corrID string) error
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
	deliveryType := strings.ToUpper(strings.TrimSpace(req.DeliveryType))

	deliveryPrice := req.DeliveryPrice
	if deliveryType == "PICKUP" {
		deliveryPrice = 0
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
		deliveryType,
		req.PaymentMethod,
		paymentStatus,
		status,
		req.Address,
		req.Comment,
		req.IIKOOrderID,
		req.IIKODeliveryID,
		deliveryPrice,
		req.Products,
	); err != nil {
		r.logger.Error(ctx, "err on r.db.Exec", zap.Error(err))
		return "", fmt.Errorf("create order failed: %w", err)
	}

	r.logger.Info(ctx, "order created", zap.Any("tg_id", req.TgID), zap.String("status", status))
	return id, err
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
        FROM orders AS o
        JOIN clients AS c ON c.tgid = o.tg_id
        WHERE o.tg_id = $1
        ORDER BY o.created_at DESC
    `

	rows, err := r.db.Query(ctx, query, tgId)
	if err != nil {
		return resp, fmt.Errorf("get orders failed: %w", err)
	}
	defer rows.Close()

	// ✅ perf: butun list uchun umumiy cache
	prodCache := map[string]structs.ProductMeta{}
	boxCache := map[string]structs.BoxMeta{}

	for rows.Next() {
		var (
			order                 structs.Order
			addrBytes, itemsBytes []byte
			phone                 string
		)

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
			&phone,
		); err != nil {
			return resp, fmt.Errorf("scan order failed: %w", err)
		}

		resp.Phone = phone
		order.Phone = phone

		// ✅ jsonb -> struct
		r.unmarshalOrderJSON(addrBytes, itemsBytes, &order)

		// ✅ GetByID bilan bir xil enrich/totals/box
		r.enrichOrder(ctx, &order, prodCache, boxCache)

		resp.Orders = append(resp.Orders, order)
	}

	if err := rows.Err(); err != nil {
		return resp, fmt.Errorf("rows iteration failed: %w", err)
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
			COALESCE(o.payment_url, '') AS payment_url,
			o.created_at,
			o.updated_at,
			c.phone
		FROM orders as o
		JOIN clients as c ON c.tgid = o.tg_id
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
		&order.PaymentUrl,
		&order.CreatedAt,
		&order.UpdateAt,
		&resp.Phone,
	); err != nil {
		r.logger.Error(ctx, "err on r.db.QueryRow.Scan", zap.Error(err))
		return structs.GetListPrimaryKeyResponse{}, fmt.Errorf("get order by ID failed: %w", err)
	}

	// box meta cache: key=boxID
	boxCache := map[string]structs.BoxMeta{}

	var (
		totalItems int64
		orderTotal int64
		boxTotal   int64
	)

	for i, p := range order.Products {
		price, name, url, boxID, err := r.getProductPriceWithBox(ctx, p.ID)
		if err != nil {
			r.logger.Warn(ctx, "Price not found for product", zap.String("productId", p.ID), zap.Error(err))
			continue
		}

		qty := p.Quantity
		orderTotal += price * qty
		totalItems += qty

		// product enrich
		order.Products[i].ProductName = name
		order.Products[i].ProductPrice = price
		order.Products[i].ProductUrl = url

		// box enrich + totals
		boxID = strings.TrimSpace(boxID)
		if boxID == "" {
			continue
		}

		meta, ok := boxCache[boxID]
		if !ok {
			bp, bn, _, _, err := r.getProductPriceWithBox(ctx, boxID)
			if err != nil {
				r.logger.Warn(ctx, "Box price/name not found", zap.String("boxId", boxID), zap.Error(err))
				continue
			}
			meta = structs.BoxMeta{
				Price: bp,
				Name:  bn,
			}
			boxCache[boxID] = meta
		}

		boxTotal += meta.Price * qty

		// ✅ response ichida ham ko‘rinsin
		order.Products[i].BoxID = boxID
		order.Products[i].BoxName = meta.Name
		order.Products[i].BoxPrice = meta.Price
	}

	order.TotalCount = totalItems

	// pickup bo‘lsa delivery bo‘lmaydi
	if strings.ToUpper(strings.TrimSpace(order.DeliveryType)) == "PICKUP" {
		order.DeliveryPrice = 0
	}

	// final totals
	order.OrderPriceForIIKO = orderTotal + boxTotal
	order.TotalPrice = orderTotal + boxTotal + order.DeliveryPrice

	r.logger.Info(ctx, "order retrieved", zap.String("id", id))
	resp.Order = order
	return resp, nil
}

func (r repo) GetByMerchantTransId(ctx context.Context, merchantTransID string) (resp structs.Order, err error) {
	r.logger.Info(ctx, "Get order by merchantTransId", zap.String("merchantTransId", merchantTransID))

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
			o.payment_url,
			o.created_at,
			o.updated_at,
			c.phone
		FROM orders AS o
		JOIN clients AS c ON c.tgid = o.tg_id
		WHERE o.click_transaction_param = $1
		LIMIT 1
	`

	var (
		order                 structs.Order
		addrBytes, itemsBytes []byte
		phone                 string
	)

	if err := r.db.QueryRow(ctx, query, merchantTransID).Scan(
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
		&order.PaymentUrl,
		&order.CreatedAt,
		&order.UpdateAt,
		&phone,
	); err != nil {
		r.logger.Error(ctx, "err on r.db.QueryRow.Scan", zap.Error(err))
		return structs.Order{}, fmt.Errorf("get order by merchantTransId failed: %w", err)
	}

	order.Phone = phone
	r.unmarshalOrderJSON(addrBytes, itemsBytes, &order)

	prodCache := map[string]structs.ProductMeta{}
	boxCache := map[string]structs.BoxMeta{}
	r.enrichOrder(ctx, &order, prodCache, boxCache)

	return order, nil
}

func (r repo) GetByOrderNumber(ctx context.Context, number int64) (resp structs.Order, err error) {
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
			o.payment_url,
			o.created_at,
			o.updated_at,
			c.phone
		FROM orders AS o
		JOIN clients AS c ON c.tgid = o.tg_id
		WHERE o.order_number = $1
		LIMIT 1
	`

	var (
		order                 structs.Order
		addrBytes, itemsBytes []byte
		phone                 string
	)

	if err := r.db.QueryRow(ctx, query, number).Scan(
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
		&order.PaymentUrl,
		&order.CreatedAt,
		&order.UpdateAt,
		&phone,
	); err != nil {
		r.logger.Error(ctx, "err on r.db.QueryRow.Scan", zap.Error(err))
		return structs.Order{}, fmt.Errorf("get order by order_number failed: %w", err)
	}

	order.Phone = phone
	r.unmarshalOrderJSON(addrBytes, itemsBytes, &order)

	prodCache := map[string]structs.ProductMeta{}
	boxCache := map[string]structs.BoxMeta{}
	r.enrichOrder(ctx, &order, prodCache, boxCache)

	return order, nil
}

func (r repo) GetList(ctx context.Context, req structs.GetListOrderRequest) (resp structs.GetListOrderResponse, err error) {
	r.logger.Info(ctx, "Get order list", zap.Any("req", req))

	base := `
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
			o.payment_url,
			o.created_at,
			o.updated_at,
			c.phone
		FROM orders AS o
		JOIN clients AS c ON c.tgid = o.tg_id
	`

	where := " WHERE TRUE"
	args := []interface{}{}
	argIndex := 1

	if strings.TrimSpace(req.Status) != "" {
		where += fmt.Sprintf(" AND o.order_status::text ILIKE $%d", argIndex)
		args = append(args, "%"+strings.TrimSpace(req.Status)+"%")
		argIndex++
	}
	if strings.TrimSpace(req.DeliveryType) != "" {
		where += fmt.Sprintf(" AND o.delivery_type::text ILIKE $%d", argIndex)
		args = append(args, "%"+strings.TrimSpace(req.DeliveryType)+"%")
		argIndex++
	}
	if strings.TrimSpace(req.PaymentMethod) != "" {
		where += fmt.Sprintf(" AND o.payment_method::text ILIKE $%d", argIndex)
		args = append(args, "%"+strings.TrimSpace(req.PaymentMethod)+"%")
		argIndex++
	}
	if strings.TrimSpace(req.PaymentStatus) != "" {
		where += fmt.Sprintf(" AND o.payment_status::text ILIKE $%d", argIndex)
		args = append(args, strings.TrimSpace(req.PaymentStatus))
		argIndex++
	}
	if req.OrderNumber > 0 {
		where += fmt.Sprintf(" AND o.order_number = $%d", argIndex)
		args = append(args, req.OrderNumber)
		argIndex++
	}
	if strings.TrimSpace(req.PhoneNumber) != "" {
		where += fmt.Sprintf(" AND c.phone ILIKE $%d", argIndex)
		args = append(args, "%"+strings.TrimSpace(req.PhoneNumber)+"%")
		argIndex++
	}
	if strings.TrimSpace(req.CreatedAt) != "" {
		where += fmt.Sprintf(" AND o.created_at::date = $%d::date", argIndex)
		args = append(args, strings.TrimSpace(req.CreatedAt))
		argIndex++
	}

	sort := " ORDER BY o.created_at DESC"
	limit := " LIMIT 10"
	offset := " OFFSET 0"
	if req.Limit > 0 {
		limit = fmt.Sprintf(" LIMIT %d", req.Limit)
	}
	if req.Offset > 0 {
		offset = fmt.Sprintf(" OFFSET %d", req.Offset)
	}

	query := base + where + sort + limit + offset

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error(ctx, "err on r.db.Query", zap.Error(err))
		return structs.GetListOrderResponse{}, fmt.Errorf("get order list failed: %w", err)
	}
	defer rows.Close()

	// ✅ perf cache (list uchun umumiy)
	prodCache := map[string]structs.ProductMeta{}
	boxCache := map[string]structs.BoxMeta{}

	for rows.Next() {
		var (
			order                 structs.Order
			addrBytes, itemsBytes []byte
			totalCount            int64
			phone                 string
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
			&order.PaymentUrl,
			&order.CreatedAt,
			&order.UpdateAt,
			&phone,
		); err != nil {
			r.logger.Error(ctx, "err on rows.Scan", zap.Error(err))
			return structs.GetListOrderResponse{}, fmt.Errorf("scan order failed: %w", err)
		}

		resp.Count = totalCount
		order.Phone = phone

		r.unmarshalOrderJSON(addrBytes, itemsBytes, &order)
		r.enrichOrder(ctx, &order, prodCache, boxCache)

		if strings.ToUpper(strings.TrimSpace(order.PaymentStatus)) == "PAID" {
			order.PaymentUrl = ""
		}

		resp.Orders = append(resp.Orders, order)
	}

	if err := rows.Err(); err != nil {
		return resp, fmt.Errorf("rows error: %w", err)
	}

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

func (r repo) UpdatePaymentStatus(ctx context.Context, req structs.UpdateStatus) error {
	r.logger.Info(ctx, "Update order status", zap.String("orderId", req.OrderId), zap.String("status", req.Status))

	query := `
		UPDATE orders
		SET payment_status = $2
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

func (r repo) AddLink(ctx context.Context, link, order_id string) error {
	query := `
        UPDATE orders
        SET 
            payment_url = $1,
            updated_at = NOW()
        WHERE id = $2
    `
	_, err := r.db.Exec(ctx, query, link, order_id)
	return err
}

func (r repo) UpdateIikoMeta(ctx context.Context, orderID, iikoOrderID, iikoPosID, corrID string) error {
	q := `
		UPDATE orders
		SET iiko_order_id = $2,
			iiko_pos_id = $3,
			iiko_correlation_id = $4,
			updated_at = now()
		WHERE id = $1`
	_, err := r.db.Exec(ctx, q, orderID, iikoOrderID, iikoPosID, corrID)
	return err
}

// order_repo/order.go (yoki qaysi faylda turgan bo‘lsa)
func (r repo) getProductPriceWithBox(ctx context.Context, productID string) (price int64, name structs.Name, url string, boxID string, err error) {
	query := `
		SELECT
			(size_prices->0->'price'->>'currentPrice')::bigint,
			name,
			img_url,
			COALESCE(box_id, '') AS box_id
		FROM product
		WHERE id = $1
	`
	err = r.db.QueryRow(ctx, query, productID).Scan(&price, &name, &url, &boxID)
	if err != nil {
		return 0, structs.Name{}, "", "", err
	}
	return price, name, url, boxID, nil
}

func (r repo) unmarshalOrderJSON(addrBytes, itemsBytes []byte, order *structs.Order) {
	if len(addrBytes) > 0 {
		if err := json.Unmarshal(addrBytes, &order.Address); err != nil {
			r.logger.Warn(context.Background(), "failed to unmarshal address", zap.Error(err))
		}
	}
	if len(itemsBytes) > 0 {
		if err := json.Unmarshal(itemsBytes, &order.Products); err != nil {
			r.logger.Warn(context.Background(), "failed to unmarshal items", zap.Error(err))
		}
	}
}

// product + box enrich + totals (GetByID bilan bir xil logika)
func (r repo) enrichOrder(ctx context.Context, order *structs.Order, prodCache map[string]structs.ProductMeta, boxCache map[string]structs.BoxMeta) {
	// pickup bo‘lsa delivery bo‘lmaydi
	if strings.ToUpper(strings.TrimSpace(order.DeliveryType)) == "PICKUP" {
		order.DeliveryPrice = 0
	}

	var (
		totalItems int64
		orderTotal int64
		boxTotal   int64
	)

	for i := range order.Products {
		pid := strings.TrimSpace(order.Products[i].ID)
		if pid == "" {
			continue
		}

		pm, ok := prodCache[pid]
		if !ok {
			price, name, url, boxID, err := r.getProductPriceWithBox(ctx, pid)
			if err != nil {
				r.logger.Warn(ctx, "Price not found for product", zap.String("productId", pid), zap.Error(err))
				continue
			}
			pm = structs.ProductMeta{
				Price: price,
				Name:  name,
				Url:   url,
				BoxID: strings.TrimSpace(boxID),
			}
			prodCache[pid] = pm
		}

		qty := order.Products[i].Quantity
		totalItems += qty
		orderTotal += pm.Price * qty

		// product enrich
		order.Products[i].ProductName = pm.Name
		order.Products[i].ProductPrice = pm.Price
		order.Products[i].ProductUrl = pm.Url

		// box enrich + totals
		if pm.BoxID == "" {
			continue
		}

		bm, ok := boxCache[pm.BoxID]
		if !ok {
			bp, bn, _, _, err := r.getProductPriceWithBox(ctx, pm.BoxID)
			if err != nil {
				r.logger.Warn(ctx, "Box price/name not found", zap.String("boxId", pm.BoxID), zap.Error(err))
				continue
			}
			bm = structs.BoxMeta{Price: bp, Name: bn}
			boxCache[pm.BoxID] = bm
		}

		boxTotal += bm.Price * qty

		// response ichida ham ko‘rinsin
		order.Products[i].BoxID = pm.BoxID
		order.Products[i].BoxName = bm.Name
		order.Products[i].BoxPrice = bm.Price
	}

	order.TotalCount = totalItems
	order.OrderPriceForIIKO = orderTotal + boxTotal
	order.TotalPrice = orderTotal + boxTotal + order.DeliveryPrice
}
