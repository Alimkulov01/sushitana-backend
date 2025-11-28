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
		GetByTgID(ctx context.Context, tgID int64) (structs.GetCartByTgID, error)
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
	r.logger.Info(ctx, "Create cart", zap.Any("req", req))
	query := `
		INSERT INTO carts(tg_id, product_id, count) VALUES ($1, $2, $3)
	`
	_, err := r.db.Exec(ctx, query, req.TGID, req.ProductID, req.Count)
	if err != nil {
		r.logger.Error(ctx, "failed to create cart", zap.Error(err))
		return err
	}
	return err

}

func (r repo) Clear(ctx context.Context, tgID int64) error {
	r.logger.Info(ctx, "Clear cart", zap.Int64("tg_id", tgID))
	query := `
		DELETE FROM carts WHERE tg_id = $1
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
		DELETE FROM carts WHERE tg_id = $1 AND product_id = $2
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

func (r repo) GetByTgID(ctx context.Context, tgID int64) (structs.GetCartByTgID, error) {
	r.logger.Info(ctx, "Get cart by tgID", zap.Int64("tg_id", tgID))
	var (
		res  structs.GetCartByTgID
		cart structs.CartInfo
	)
	query := `
		SELECT
			tg_id,
			phone_number
		FROM clients 
		WHERE tg_id = $1
	`
	err := r.db.QueryRow(ctx, query, tgID).Scan(
		&res.TGID,
		&res.PhoneNumber,
	)
	if err != nil {
		r.logger.Error(ctx, "failed to get cart by tgID", zap.Error(err))
		return res, err
	}

	queryCartInfo := `
		SELECT
			COALESCE(SUM(p.price * c.count), 0) AS total_price,
			COALESCE(JSONB_AGG(
				JSONB_BUILD_OBJECT(
					'id', p.id,
					'name', p.name,
					'category_id', p.category_id,
					'img_url', p.img_url,
					'description', p.description,
					'price', p.price,
					'count', c.count,
					'is_active', p.is_active,
					'created_at', p.created_at,
					'updated_at', p.updated_at
				)
			) FILTER (WHERE p.id IS NOT NULL), '[]') AS products
		FROM carts c
		LEFT JOIN product p ON c.product_id = p.id
		WHERE c.tg_id = $1
		GROUP BY c.tg_id
	`
	err = r.db.QueryRow(ctx, queryCartInfo, tgID).Scan(
		&cart.TotalPrice,
		&cart.Products,
	)
	if err != nil {
		r.logger.Error(ctx, "failed to get cart info by tgID", zap.Error(err))
		return res, err
	}

	res.Cart = cart
	return res, nil
}
