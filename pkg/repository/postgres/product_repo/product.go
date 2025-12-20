package productrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sushitana/internal/structs"
	"sushitana/pkg/db"
	"sushitana/pkg/logger"
	"sushitana/pkg/utils"

	"github.com/jackc/pgx/v5"
	"github.com/spf13/cast"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Provide(New)

type (
	Params struct {
		fx.In
		Logger logger.Logger
		DB     db.Querier
	}

	Repo interface {
		Create(ctx context.Context, req []structs.CreateProduct) error
		GetByID(ctx context.Context, id string) (structs.Product, error)
		GetByProductName(ctx context.Context, name string) (structs.Product, error)
		GetList(ctx context.Context, req structs.GetListProductRequest) (structs.GetListProductResponse, error)
		Delete(ctx context.Context, ProductID string) error
		Patch(ctx context.Context, req structs.PatchProduct) (int64, error)
		GetListCategoryName(ctx context.Context, req string) ([]structs.Product, error)
		GetBox(ctx context.Context) (resp structs.GetListProductResponse, err error)
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

func (r *repo) Create(ctx context.Context, req []structs.CreateProduct) error {
	r.logger.Info(ctx, "Create product", zap.Int("count", len(req)))

	if len(req) == 0 {
		return nil
	}

	batchSize := 1000
	totalInserted := int64(0)

	query := `
		WITH data AS (
			SELECT DISTINCT ON (id)
				id,
				group_id,
				name,
				product_category_id,
				type,
				order_item_type,
				measure_unit,
				size_prices,
				do_not_print_in_cheque,
				parent_group,
				"order",
				payment_subject,
				code,
				is_deleted,
				can_set_open_price,
				splittable,
				weight
			FROM jsonb_to_recordset($1::jsonb) AS t(
				id text,
				group_id text,
				name jsonb,
				product_category_id text,
				type text,
				order_item_type text,
				measure_unit text,
				size_prices jsonb,
				do_not_print_in_cheque boolean,
				parent_group text,
				"order" int,
				payment_subject text,
				code text,
				is_deleted boolean,
				can_set_open_price boolean,
				splittable boolean,
				weight float
			)
			ORDER BY id
		)
		INSERT INTO product (
			id,
			group_id,
			name,
			product_category_id,
			type,
			order_item_type,
			measure_unit,
			size_prices,
			do_not_print_in_cheque,
			parent_group,
			"order",
			payment_subject,
			code,
			is_deleted,
			can_set_open_price,
			splittable,
			weight
		)
		SELECT
			id::text,
			COALESCE(group_id, '') AS group_id,
			name::jsonb,
			product_category_id::text,
			type::text,
			COALESCE(order_item_type, '') AS order_item_type,
			COALESCE(measure_unit, '') AS measure_unit,
			COALESCE(size_prices, '[]'::jsonb) AS size_prices,
			do_not_print_in_cheque::boolean,
			parent_group::text,
			"order"::int,
			payment_subject::text,
			code::text,
			is_deleted::boolean,
			can_set_open_price::boolean,
			splittable::boolean,
			weight::float
		FROM data
		ON CONFLICT (id) DO UPDATE
		SET
			group_id = EXCLUDED.group_id,
			name = product.name || (
				CASE
					WHEN (EXCLUDED.name ? 'ru')
						THEN jsonb_build_object('ru', EXCLUDED.name->>'ru')
					ELSE '{}'::jsonb
				END
			),
			product_category_id = EXCLUDED.product_category_id,
			type                = EXCLUDED.type,
			order_item_type     = EXCLUDED.order_item_type,
			measure_unit        = EXCLUDED.measure_unit,
			size_prices         = EXCLUDED.size_prices,
			do_not_print_in_cheque = EXCLUDED.do_not_print_in_cheque,
			parent_group        = EXCLUDED.parent_group,
			"order"             = EXCLUDED."order",
			payment_subject     = EXCLUDED.payment_subject,
			code                = EXCLUDED.code,
			is_deleted          = EXCLUDED.is_deleted,
			can_set_open_price  = EXCLUDED.can_set_open_price,
			splittable          = EXCLUDED.splittable,
			weight              = EXCLUDED.weight,
			updated_at          = now()
		WHERE
			(
				CASE
					WHEN (EXCLUDED.name ? 'ru')
						THEN (product.name->>'ru') IS DISTINCT FROM (EXCLUDED.name->>'ru')
					ELSE false
				END
			)
			OR COALESCE(product.group_id, '') IS DISTINCT FROM COALESCE(EXCLUDED.group_id, '')
			OR COALESCE(product.product_category_id, '') IS DISTINCT FROM COALESCE(EXCLUDED.product_category_id, '')
			OR COALESCE(product.type, '') IS DISTINCT FROM COALESCE(EXCLUDED.type, '')
			OR COALESCE(product.order_item_type, '') IS DISTINCT FROM COALESCE(EXCLUDED.order_item_type, '')
			OR COALESCE(product.measure_unit, '') IS DISTINCT FROM COALESCE(EXCLUDED.measure_unit, '')
			OR COALESCE(product.parent_group, '') IS DISTINCT FROM COALESCE(EXCLUDED.parent_group, '')
			OR COALESCE(product.payment_subject, '') IS DISTINCT FROM COALESCE(EXCLUDED.payment_subject, '')
			OR COALESCE(product.code, '') IS DISTINCT FROM COALESCE(EXCLUDED.code, '')
			OR product.size_prices IS DISTINCT FROM EXCLUDED.size_prices
			OR COALESCE(product.do_not_print_in_cheque, false)
				IS DISTINCT FROM COALESCE(EXCLUDED.do_not_print_in_cheque, false)
			OR COALESCE(product.is_deleted, false)
				IS DISTINCT FROM COALESCE(EXCLUDED.is_deleted, false)
			OR COALESCE(product.can_set_open_price, false)
				IS DISTINCT FROM COALESCE(EXCLUDED.can_set_open_price, false)
			OR COALESCE(product.splittable, false)
				IS DISTINCT FROM COALESCE(EXCLUDED.splittable, false)

			-- weight (float)
			OR product.weight IS DISTINCT FROM EXCLUDED.weight;
	`

	for start := 0; start < len(req); start += batchSize {
		end := start + batchSize
		if end > len(req) {
			end = len(req)
		}

		part := req[start:end]

		b, err := json.Marshal(part)
		if err != nil {
			r.logger.Error(ctx, "failed to marshal products batch",
				zap.Error(err),
				zap.Int("start", start),
				zap.Int("end", end),
			)
			return fmt.Errorf("marshal batch: %w", err)
		}

		tx, err := r.db.Begin(ctx)
		if err != nil {
			r.logger.Error(ctx, "begin tx failed", zap.Error(err))
			return fmt.Errorf("begin tx: %w", err)
		}

		cmdTag, err := tx.Exec(ctx, query, string(b))
		if err != nil {
			_ = tx.Rollback(ctx)
			r.logger.Error(ctx, "insert batch failed", zap.Error(err))
			return fmt.Errorf("insert batch failed: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			_ = tx.Rollback(ctx)
			r.logger.Error(ctx, "tx commit failed", zap.Error(err))
			return fmt.Errorf("tx commit: %w", err)
		}

		totalInserted += cmdTag.RowsAffected()
	}

	r.logger.Info(ctx, "CreateProducts finished", zap.Int64("inserted", totalInserted))
	return nil
}

func (r *repo) GetByID(ctx context.Context, id string) (structs.Product, error) {
	var (
		resp  structs.Product
		query = `
			SELECT
				id,
				group_id,
				name,
				COALESCE(product_category_id, '') AS product_category_id,
				type,
				order_item_type,
				measure_unit,
				size_prices,
				COALESCE(do_not_print_in_cheque, false) AS do_not_print_in_cheque,
				COALESCE(parent_group, '') AS parent_group,
				"order",
				COALESCE(payment_subject, '') AS payment_subject,
				code,
				COALESCE(is_deleted, false) AS is_deleted,
				COALESCE(can_set_open_price, false) AS can_set_open_price,
				splittable,
				index,
				COALESCE(is_new, false) AS is_new,
				img_url,
				COALESCE(is_active, false) AS is_active, 
				COALESCE(box_id, '') AS box_id,
				description,
				created_at,
				updated_at,
				weight
			FROM product
			WHERE id = $1
			AND EXISTS (
				SELECT 1
				FROM jsonb_array_elements(size_prices) AS sp
				WHERE (sp->'price'->>'currentPrice')::bigint > 0
			)
		`
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&resp.ID,
		&resp.GroupID,
		&resp.Name,
		&resp.ProductCategoryID,
		&resp.Type,
		&resp.OrderItemType,
		&resp.MeasureUnit,
		&resp.SizePrices,
		&resp.DoNotPrintInCheque,
		&resp.ParentGroup,
		&resp.Order,
		&resp.PaymentSubject,
		&resp.Code,
		&resp.IsDeleted,
		&resp.CanSetOpenPrice,
		&resp.Splittable,
		&resp.Index,
		&resp.IsNew,
		&resp.ImgUrl,
		&resp.IsActive,
		&resp.BoxId,
		&resp.Description,
		&resp.CreatedAt,
		&resp.UpdatedAt,
		&resp.Weight,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Error(ctx, " err from r.db.QueryRow", zap.Error(err))
			return structs.Product{}, structs.ErrNotFound
		}
		r.logger.Error(ctx, "error querying row", zap.Error(err))
		return structs.Product{}, fmt.Errorf("error getting product item by ID: %w", err)
	}
	return resp, err
}

func (r *repo) GetByProductName(ctx context.Context, name string) (resp structs.Product, err error) {
	n := strings.TrimSpace(name)
	r.logger.Info(ctx, "Get product by name", zap.String("req", n))

	if n == "" {
		return structs.Product{}, structs.ErrNotFound
	}

	// ILIKE uchun pattern
	pattern := "%" + n + "%"

	query := `
		SELECT
			id,
			group_id,
			name,
			COALESCE(product_category_id, '') AS product_category_id,
			type,
			order_item_type,
			measure_unit,
			size_prices,
			COALESCE(do_not_print_in_cheque, false) AS do_not_print_in_cheque,
			COALESCE(parent_group, '') AS parent_group,
			"order",
			COALESCE(payment_subject, '') AS payment_subject,
			code,
			COALESCE(is_deleted, false) AS is_deleted,
			COALESCE(can_set_open_price, false) AS can_set_open_price,
			splittable,
			index,
			COALESCE(is_new, false) AS is_new,
			img_url,
			COALESCE(is_active, false) AS is_active, 
			COALESCE(box_id, '') AS box_id,
			description,
			created_at,
			updated_at,
			weight
		FROM product
		WHERE
			(
				name->>'uz' ILIKE $1 OR
				name->>'ru' ILIKE $1 OR
				name->>'en' ILIKE $1
			)
			AND EXISTS (
				SELECT 1
				FROM jsonb_array_elements(COALESCE(size_prices, '[]'::jsonb)) AS sp
				WHERE (sp->'price'->>'currentPrice')::bigint > 0
			)
		LIMIT 1;
	`

	err = r.db.QueryRow(ctx, query, pattern).Scan(
		&resp.ID,
		&resp.GroupID,
		&resp.Name,
		&resp.ProductCategoryID,
		&resp.Type,
		&resp.OrderItemType,
		&resp.MeasureUnit,
		&resp.SizePrices,
		&resp.DoNotPrintInCheque,
		&resp.ParentGroup,
		&resp.Order,
		&resp.PaymentSubject,
		&resp.Code,
		&resp.IsDeleted,
		&resp.CanSetOpenPrice,
		&resp.Splittable,
		&resp.Index,
		&resp.IsNew,
		&resp.ImgUrl,
		&resp.IsActive,
		&resp.BoxId,
		&resp.Description,
		&resp.CreatedAt,
		&resp.UpdatedAt,
		&resp.Weight,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Warn(ctx, "product not found by name", zap.String("name", n))
			return structs.Product{}, structs.ErrNotFound
		}
		r.logger.Error(ctx, "error querying product by name", zap.Error(err))
		return structs.Product{}, fmt.Errorf("error getting product by name: %w", err)
	}

	return resp, nil
}

func (r *repo) GetList(ctx context.Context, req structs.GetListProductRequest) (resp structs.GetListProductResponse, err error) {
	r.logger.Info(ctx, "GetList Product", zap.Any("req", req))

	limit := int64(100)
	offset := int64(0)

	if req.Limit > 0 {
		limit = req.Limit
	}
	if req.Offset > 0 {
		offset = req.Offset
	}

	where := "WHERE TRUE "
	args := []interface{}{limit, offset}
	argID := 3
	if req.Search != "" {
		where += fmt.Sprintf(`
        AND (
            name->>'en' ILIKE $%d OR
            name->>'ru' ILIKE $%d OR
            name->>'uz' ILIKE $%d
        )`, argID, argID, argID)
		args = append(args, "%"+req.Search+"%")
		argID++
	}

	query := fmt.Sprintf(`
		SELECT
			COUNT(*) OVER(),
			id,
			group_id,
			name,
			COALESCE(product_category_id, '') AS product_category_id,
			type,
			order_item_type,
			measure_unit,
			size_prices,
			COALESCE(do_not_print_in_cheque, false) AS do_not_print_in_cheque,
			COALESCE(parent_group, '') AS parent_group,
			"order",
			COALESCE(payment_subject, '') AS payment_subject,
			code,
			COALESCE(is_deleted, false) AS is_deleted,
			COALESCE(can_set_open_price, false) AS can_set_open_price,
			splittable,
			index,
			COALESCE(is_new, false) AS is_new,
			img_url,
			COALESCE(is_active, false) AS is_active, 
			COALESCE(box_id, '') AS box_id,
			description,
			created_at,
			updated_at,
			weight
		FROM product
		%s
		AND EXISTS (
				SELECT 1
				FROM jsonb_array_elements(size_prices) AS sp
				WHERE (sp->'price'->>'currentPrice')::bigint > 0
			)
		LIMIT $1 OFFSET $2
	`, where)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error(ctx, "err on r.db.Query", zap.Error(err))
		return resp, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var list []structs.Product

	for rows.Next() {
		var p structs.Product
		err := rows.Scan(
			&resp.Count,
			&p.ID,
			&p.GroupID,
			&p.Name,
			&p.ProductCategoryID,
			&p.Type,
			&p.OrderItemType,
			&p.MeasureUnit,
			&p.SizePrices,
			&p.DoNotPrintInCheque,
			&p.ParentGroup,
			&p.Order,
			&p.PaymentSubject,
			&p.Code,
			&p.IsDeleted,
			&p.CanSetOpenPrice,
			&p.Splittable,
			&p.Index,
			&p.IsNew,
			&p.ImgUrl,
			&p.IsActive,
			&p.BoxId,
			&p.Description,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.Weight,
		)
		if err != nil {
			r.logger.Error(ctx, "err on rows.Scan", zap.Error(err))
			return resp, fmt.Errorf("row scan failed: %w", err)
		}
		list = append(list, p)
	}

	if rows.Err() != nil {
		r.logger.Error(ctx, "err on rows iteration", zap.Error(rows.Err()))
		return resp, fmt.Errorf("rows iteration failed: %w", rows.Err())
	}

	resp.Products = list
	return resp, nil
}

func (r *repo) Patch(ctx context.Context, req structs.PatchProduct) (int64, error) {
	setValues := []string{}
	params := map[string]interface{}{
		"id": req.ID,
	}
	if req.Name != nil {
		setValues = append(setValues, "name = :name")
		params["name"] = *req.Name
	}
	if req.Index != nil {
		setValues = append(setValues, "index = :index")
		params["index"] = *req.Index
	}
	if req.IsNew != nil {
		setValues = append(setValues, "is_new = :is_new")
		params["is_new"] = *req.IsNew
	}
	if req.ImgUrl != nil {
		setValues = append(setValues, "img_url = :img_url")
		params["img_url"] = *req.ImgUrl
	}
	if req.IsActive != nil {
		setValues = append(setValues, "is_active = :is_active")
		params["is_active"] = *req.IsActive
	}
	if req.BoxId != nil {
		setValues = append(setValues, "box_id = :box_id")
		params["box_id"] = *req.BoxId
	}

	if req.Description != nil {
		setValues = append(setValues, "description = :description")
		params["description"] = *req.Description
	}
	setValues = append(setValues, "updated_At = NOW()")
	if len(setValues) == 0 {
		return 0, fmt.Errorf("no fields to update for product with ID %s", req.ID)
	}
	query := fmt.Sprintf(`
        UPDATE product
        SET %s
        WHERE id = :id
    `, strings.Join(setValues, ", "))

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("error starting transaction for product ID %s: %w", req.ID, err)
	}

	query, args := utils.ReplaceQueryParams(query, params)
	result, err := tx.Exec(ctx, query, args...)
	if err != nil {
		if errRollback := tx.Rollback(ctx); errRollback != nil {
			r.logger.Error(ctx, "error rolling back transaction", zap.Error(errRollback))
			return 0, fmt.Errorf("error rolling back transaction for product ID %s: %w", req.ID, errRollback)
		}
		r.logger.Error(ctx, "error executing update", zap.Error(err))
		return 0, fmt.Errorf("error updating product with ID %s: %w", req.ID, err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		if errRollback := tx.Rollback(ctx); errRollback != nil {
			r.logger.Error(ctx, "error rolling back transaction", zap.Error(errRollback))
			return 0, fmt.Errorf("error rolling back transaction for product ID %s: %w", req.ID, errRollback)
		}
		r.logger.Warn(ctx, "no product found with the given ID", zap.String("product_id", cast.ToString(req.ID)))
		return 0, fmt.Errorf("no product found with ID %s", req.ID)
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Error(ctx, "error committing transaction", zap.Error(err))
		return 0, fmt.Errorf("error committing transaction for product ID %s: %w", req.ID, err)
	}

	return rowsAffected, nil
}

func (r *repo) Delete(ctx context.Context, productID string) error {
	r.logger.Info(ctx, "Delete product", zap.String("product_id", cast.ToString(productID)))

	query := `
		DELETE FROM product
		WHERE id = $1
	`

	result, err := r.db.Exec(ctx, query, productID)
	if err != nil {
		r.logger.Error(ctx, "error executing delete", zap.Error(err))
		return fmt.Errorf("error deleting product with ID %s: %w", productID, err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn(ctx, "no product found with the given ID", zap.String("product_id", cast.ToString(productID)))
		return fmt.Errorf("no product found with ID %s", productID)
	}

	return nil
}

func (r *repo) GetListCategoryName(ctx context.Context, req string) (resp []structs.Product, err error) {
	r.logger.Info(ctx, "GetList Product by category name", zap.Any("req", req))

	pattern := "%" + strings.TrimSpace(req) + "%"

	query := `
		SELECT
			COUNT(*) OVER() AS total_count,
			p.id,
			p.group_id,
			p.name,
			COALESCE(p.product_category_id, '') AS product_category_id,
			p.type,
			p.order_item_type,
			p.measure_unit,
			p.size_prices,
			COALESCE(p.do_not_print_in_cheque, false) AS do_not_print_in_cheque,
			COALESCE(p.parent_group, '') AS parent_group,
			p."order",
			COALESCE(p.payment_subject, '') AS payment_subject,
			p.code,
			COALESCE(p.is_deleted, false) AS is_deleted,
			COALESCE(p.can_set_open_price, false) AS can_set_open_price,
			p.splittable,
			p.index,
			COALESCE(p.is_new, false) AS is_new,
			COALESCE(p.img_url, '') AS img_url,
			COALESCE(p.is_active, false) AS is_active,
			COALESCE(p.box_id, '') AS box_id,
			p.description,
			p.created_at,
			p.updated_at,
			p.weight
		FROM product AS p
		JOIN category AS c ON c.id = p.parent_group
		WHERE
			(
				c.name->>'uz' ILIKE $1 OR
				c.name->>'ru' ILIKE $1 OR
				c.name->>'en' ILIKE $1
			)
			AND COALESCE(p.is_deleted, false) = false
			-- xohlasangiz faqat aktiv product:
			-- AND COALESCE(p.is_active, false) = true
			AND EXISTS (
				SELECT 1
				FROM jsonb_array_elements(COALESCE(p.size_prices, '[]'::jsonb)) AS sp
				WHERE (sp->'price'->>'currentPrice')::bigint > 0
			)
		ORDER BY p.created_at DESC
	`

	rows, err := r.db.Query(ctx, query, pattern)
	if err != nil {
		r.logger.Error(ctx, "err on r.db.Query", zap.Error(err))
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	list := make([]structs.Product, 0, 32)

	for rows.Next() {
		var (
			p          structs.Product
			totalCount int64
		)

		err := rows.Scan(
			&totalCount, // COUNT(*) OVER()
			&p.ID,
			&p.GroupID,
			&p.Name,
			&p.ProductCategoryID,
			&p.Type,
			&p.OrderItemType,
			&p.MeasureUnit,
			&p.SizePrices,
			&p.DoNotPrintInCheque,
			&p.ParentGroup,
			&p.Order,
			&p.PaymentSubject,
			&p.Code,
			&p.IsDeleted,
			&p.CanSetOpenPrice,
			&p.Splittable,
			&p.Index,
			&p.IsNew,
			&p.ImgUrl,
			&p.IsActive,
			&p.BoxId,
			&p.Description,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.Weight,
		)
		if err != nil {
			r.logger.Error(ctx, "err on rows.Scan", zap.Error(err))
			return nil, fmt.Errorf("row scan failed: %w", err)
		}

		list = append(list, p)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error(ctx, "err on rows iteration", zap.Error(err))
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}

	return list, nil
}

func (r *repo) GetBox(ctx context.Context) (resp structs.GetListProductResponse, err error) {
	query := `
		SELECT
			COUNT(*) OVER(),
			id,
			group_id,
			name,
			COALESCE(product_category_id, '') AS product_category_id,
			type,
			order_item_type,
			measure_unit,
			size_prices,
			COALESCE(do_not_print_in_cheque, false) AS do_not_print_in_cheque,
			COALESCE(parent_group, '') AS parent_group,
			"order",
			COALESCE(payment_subject, '') AS payment_subject,
			code,
			COALESCE(is_deleted, false) AS is_deleted,
			COALESCE(can_set_open_price, false) AS can_set_open_price,
			splittable,
			index,
			COALESCE(is_new, false) AS is_new,
			img_url,
			COALESCE(is_active, false) AS is_active,
			COALESCE(box_id, '') AS box_id,
			description,
			created_at,
			updated_at,
			weight
		FROM product
		WHERE parent_group = '8a82292e-a027-4e69-8554-7fde17c058c8'
		  AND COALESCE(is_deleted, false) = false
		ORDER BY index ASC, created_at DESC;
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.logger.Error(ctx, "err on r.db.Query", zap.Error(err))
		return resp, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p structs.Product
		err := rows.Scan(
			&resp.Count,
			&p.ID,
			&p.GroupID,
			&p.Name,
			&p.ProductCategoryID,
			&p.Type,
			&p.OrderItemType,
			&p.MeasureUnit,
			&p.SizePrices,
			&p.DoNotPrintInCheque,
			&p.ParentGroup,
			&p.Order,
			&p.PaymentSubject,
			&p.Code,
			&p.IsDeleted,
			&p.CanSetOpenPrice,
			&p.Splittable,
			&p.Index,
			&p.IsNew,
			&p.ImgUrl,
			&p.IsActive,
			&p.BoxId,
			&p.Description, // ✅ shu yetishmayotgan edi
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.Weight,
		)
		if err != nil {
			r.logger.Error(ctx, "err on rows.Scan", zap.Error(err))
			return resp, fmt.Errorf("row scan failed: %w", err)
		}
		resp.Products = append(resp.Products, p)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error(ctx, "err on rows iteration", zap.Error(err))
		return resp, fmt.Errorf("rows iteration failed: %w", err)
	}

	return resp, nil
}
