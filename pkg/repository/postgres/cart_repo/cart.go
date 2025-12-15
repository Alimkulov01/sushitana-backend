package cartrepo

import (
	"context"
	"fmt"
	"strings"
	"sushitana/internal/structs"
	"sushitana/pkg/db"
	"sushitana/pkg/logger"
	"sushitana/pkg/utils"

	"github.com/spf13/cast"
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
		Create(ctx context.Context, req structs.CreateCart) error
		Clear(ctx context.Context, tgID int64) error
		Delete(ctx context.Context, req structs.DeleteCart) error
		Patch(ctx context.Context, req structs.PatchCart) (int64, error)
		GetByUserTgID(ctx context.Context, tgID int64) (structs.GetCartByTgID, error)
		GetByTgID(ctx context.Context, tgID int64) (structs.CartInfo, error)
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

func (r repo) Create(ctx context.Context, req structs.CreateCart) error {
	r.logger.Info(ctx, "Create cart (upsert)", zap.Any("req", req))

	const upsertQuery = `
        INSERT INTO carts (tgid, product_id, count)
        VALUES ($1, $2, $3)
        ON CONFLICT (tgid, product_id)
        DO UPDATE SET count = EXCLUDED.count
    `

	if _, err := r.db.Exec(ctx, upsertQuery, req.TGID, req.ProductID, req.Count); err != nil {
		r.logger.Error(ctx, "failed to upsert cart", zap.Error(err))
		return err
	}
	return nil
}

func (r repo) Clear(ctx context.Context, tgID int64) error {
	r.logger.Info(ctx, "Clear cart", zap.Int64("tgid", tgID))
	query := `
		DELETE FROM carts WHERE tgid = $1
	`
	_, err := r.db.Exec(ctx, query, tgID)
	if err != nil {
		r.logger.Error(ctx, "failed to clear cart", zap.Error(err))
		return err
	}
	return err
}

func (r repo) Delete(ctx context.Context, req structs.DeleteCart) error {
	r.logger.Info(ctx, "Delete cart item", zap.Any("req", req))
	query := `
		DELETE FROM carts WHERE tgid = $1 AND product_id = $2
	`
	_, err := r.db.Exec(ctx, query, req.TGID, req.ProductID)
	if err != nil {
		r.logger.Error(ctx, "failed to delete cart item", zap.Error(err))
		return err
	}
	return err
}

func (r repo) Patch(ctx context.Context, req structs.PatchCart) (int64, error) {
	setValues := []string{}
	params := map[string]interface{}{
		"id": req.TGID,
	}

	if req.ProductID != nil {
		setValues = append(setValues, "product_id = :product_id")
		params["product_id"] = *req.ProductID
	}
	if req.Count != nil {
		setValues = append(setValues, "count = :count")
		params["count"] = *req.Count
	}
	setValues = append(setValues, "updated_At = NOW()")
	if len(setValues) == 0 {
		return 0, fmt.Errorf("no fields to update for cart with ID %d", *req.TGID)
	}

	query := fmt.Sprintf(`
        UPDATE carts
        SET %s
        WHERE id = :id
    `, strings.Join(setValues, ", "))

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("error starting transaction for cart ID %d: %w", req.TGID, err)
	}

	query, args := utils.ReplaceQueryParams(query, params)
	result, err := tx.Exec(ctx, query, args...)
	if err != nil {
		if errRollback := tx.Rollback(ctx); errRollback != nil {
			r.logger.Error(ctx, "error rolling back transaction", zap.Error(errRollback))
			return 0, fmt.Errorf("error rolling back transaction for cart ID %d: %w", req.TGID, errRollback)
		}
		r.logger.Error(ctx, "error executing update", zap.Error(err))
		return 0, fmt.Errorf("error updating cart with ID %d: %w", req.TGID, err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		if errRollback := tx.Rollback(ctx); errRollback != nil {
			r.logger.Error(ctx, "error rolling back transaction", zap.Error(errRollback))
			return 0, fmt.Errorf("error rolling back transaction for cart ID %d: %w", req.TGID, errRollback)
		}
		r.logger.Warn(ctx, "no cart found with the given ID", zap.String("cart_id", cast.ToString(req.TGID)))
		return 0, fmt.Errorf("no cart found with ID %d", *req.TGID)
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Error(ctx, "error committing transaction", zap.Error(err))
		return 0, fmt.Errorf("error committing transaction for cart ID %d: %w", req.TGID, err)
	}

	return rowsAffected, nil
}

func (r repo) GetByTgID(ctx context.Context, tgID int64) (structs.CartInfo, error) {
	fmt.Println(tgID)
	r.logger.Info(ctx, "Get cart by tgID", zap.Int64("tgid", tgID))
	var (
		cart structs.CartInfo
	)
	queryCartInfo := `
		WITH t AS (SELECT $1::bigint AS tgid)
		SELECT
		COALESCE(
			SUM(
			(COALESCE(
				sp.selected_price,
				(p.size_prices->0->'price'->>'currentPrice')::numeric,
				0
			)) * COALESCE(c.count, 0)
			),
			0
		) AS total_price,
		COALESCE(
			JSONB_AGG(
			JSONB_BUILD_OBJECT(
				'id', p.id,
				'name', p.name,
				'img_url', p.img_url,
				'price', (COALESCE(
							sp.selected_price,
							(p.size_prices->0->'price'->>'currentPrice')::numeric,
							0
						))::numeric,
				'count', COALESCE(c.count, 0),
				'size_id_used', sp.selected_sizeid
			)
			) FILTER (WHERE p.id IS NOT NULL),
			'[]'::jsonb
		) AS products
		FROM t
		LEFT JOIN carts c ON c.tgid = t.tgid
		LEFT JOIN product p ON c.product_id = p.id
		LEFT JOIN LATERAL (
		SELECT
			(sp->'price'->>'currentPrice')::numeric AS selected_price,
			NULLIF(sp->>'sizeId','') AS selected_sizeid
		FROM jsonb_array_elements(coalesce(p.size_prices, '[]'::jsonb)) sp
		ORDER BY
			CASE
			WHEN (sp->>'sizeId' IS NULL OR trim(sp->>'sizeId') = '') THEN 0
			ELSE 1
			END
		LIMIT 1
		) sp ON true
		GROUP BY t.tgid;
		`

	if err := r.db.QueryRow(ctx, queryCartInfo, tgID).Scan(&cart.TotalPrice, &cart.Products); err != nil {
		r.logger.Error(ctx, "failed to get cart info by tgID", zap.Error(err))
		return cart, err
	}

	return cart, nil
}

func (r repo) GetByUserTgID(ctx context.Context, tgID int64) (structs.GetCartByTgID, error) {
	r.logger.Info(ctx, "Get cart by tgID", zap.Int64("tgid", tgID))
	var (
		res  structs.GetCartByTgID
		cart structs.CartInfo
	)
	query := `
		SELECT
			tgid,
			phone,
			language,
			name
		FROM clients 
		WHERE tgid = $1
	`
	err := r.db.QueryRow(ctx, query, tgID).Scan(
		&res.TGID,
		&res.PhoneNumber,
		&res.Language,
		&res.Name,
	)
	if err != nil {
		r.logger.Error(ctx, "failed to get cart by tgID", zap.Error(err))
		return res, err
	}

	queryCartInfo := `
		WITH t AS (SELECT $1::bigint AS tgid)
		SELECT
		COALESCE(
			SUM(
			(COALESCE(
				sp.selected_price,
				(p.size_prices->0->'price'->>'currentPrice')::numeric,
				0
			)) * COALESCE(c.count, 0)
			),
			0
		) AS total_price,
		COALESCE(
			JSONB_AGG(
			JSONB_BUILD_OBJECT(
				'id', p.id,
				'name', p.name,
				'img_url', p.img_url,
				'price', (COALESCE(
							sp.selected_price,
							(p.size_prices->0->'price'->>'currentPrice')::numeric,
							0
						))::numeric,
				'count', COALESCE(c.count, 0),
				'size_id_used', sp.selected_sizeid
			)
			) FILTER (WHERE p.id IS NOT NULL),
			'[]'::jsonb
		) AS products
		FROM t
		LEFT JOIN carts c ON c.tgid = t.tgid
		LEFT JOIN product p ON c.product_id = p.id
		LEFT JOIN LATERAL (
		SELECT
			(sp->'price'->>'currentPrice')::numeric AS selected_price,
			NULLIF(sp->>'sizeId','') AS selected_sizeid
		FROM jsonb_array_elements(coalesce(p.size_prices, '[]'::jsonb)) sp
		ORDER BY
			CASE
			WHEN (sp->>'sizeId' IS NULL OR trim(sp->>'sizeId') = '') THEN 0
			ELSE 1
			END
		LIMIT 1
		) sp ON true
		GROUP BY t.tgid;
		`

	if err := r.db.QueryRow(ctx, queryCartInfo, tgID).Scan(&cart.TotalPrice, &cart.Products); err != nil {
		r.logger.Error(ctx, "failed to get cart info by tgID", zap.Error(err))
		return res, err
	}

	res.Cart = cart
	return res, nil
}
