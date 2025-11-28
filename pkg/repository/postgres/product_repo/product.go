package productrepo

import (
	"context"
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
		Create(ctx context.Context, req structs.CreateProduct) (structs.Product, error)
		GetByID(ctx context.Context, id int64) (structs.Product, error)
		GetByProductName(ctx context.Context, name string) (structs.Product, error)
		GetList(ctx context.Context, req structs.GetListProductRequest) (structs.GetListProductResponse, error)
		Delete(ctx context.Context, ProductID int64) error
		Patch(ctx context.Context, req structs.PatchProduct) (int64, error)
		GetListCategoryName(ctx context.Context, req string) ([]structs.Product, error)
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

func (r *repo) Create(ctx context.Context, req structs.CreateProduct) (resp structs.Product, err error) {
	r.logger.Info(ctx, "Create product", zap.Any("req", req))
	query := `
		INSERT INTO product(
			name,
			category_id,
			img_url,
			price,
			count,
			decription,
			is_active,
			index,
			is_new,
			discount_price,
			post_id
		) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, name, category_id, img_url, price, count, description, is_active, index, is_new, discount_price, post_id, created_at, updated_at
	`
	err = r.db.QueryRow(ctx, query, req.Name, req.CategoryID, req.ImgUrl, req.Price, req.Count, req.Description, req.IsActive).Scan(
		&resp.ID,
		&resp.Name,
		&resp.CategoryID,
		&resp.ImgUrl,
		&resp.Price,
		&resp.Count,
		&resp.Description,
		&resp.IsActive,
		&resp.CreatedAt,
		&resp.UpdatedAt,
	)
	if err != nil {
		r.logger.Error(ctx, "err on r.db.QueryRow", zap.Error(err))
		return structs.Product{}, fmt.Errorf("create product failed: %w", err)
	}
	return resp, nil
}

func (r *repo) GetByID(ctx context.Context, id int64) (structs.Product, error) {
	var (
		resp  structs.Product
		query = `
			SELECT
				id,
				name,
				category_id,
				img_url,
				price,
				count,
				decription,
				is_active,
				index,
				is_new,
				discount_price,
				post_id,
				created_at,
				updated_at
			FROM product
			WHERE id = $1
		`
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&resp.ID,
		&resp.Name,
		&resp.CategoryID,
		&resp.ImgUrl,
		&resp.Price,
		&resp.Count,
		&resp.Description,
		&resp.IsActive,
		&resp.Index,
		&resp.IsNew,
		&resp.DiscountPrice,
		&resp.PostID,
		&resp.CreatedAt,
		&resp.UpdatedAt,
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
	r.logger.Info(ctx, "GetList Product by name", zap.Any("req", name))

	query := `
		SELECT
			id,
			name,
			category_id,
			img_url,
			price,
			count,
			decription,
			is_active,
			index,
			is_new,
			discount_price,
			post_id,
			created_at,
			updated_at
		FROM product
		WHERE 
			name->>'uz' ILIKE $1 OR
			name->>'ru' ILIKE $1 OR
			name->>'en' ILIKE $1
		LIMIT 1
	`

	err = r.db.QueryRow(ctx, query, name).Scan(
		&resp.ID,
		&resp.Name,
		&resp.CategoryID,
		&resp.ImgUrl,
		&resp.Price,
		&resp.Count,
		&resp.Description,
		&resp.IsActive,
		&resp.Index,
		&resp.IsNew,
		&resp.DiscountPrice,
		&resp.PostID,
		&resp.CreatedAt,
		&resp.UpdatedAt,
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

	where := "WHERE TRUE"
	args := []interface{}{limit, offset}
	argID := 3

	if req.Search != "" {
		where += fmt.Sprintf(" AND name ILIKE $%d", argID)
		args = append(args, "%"+req.Search+"%")
		argID++
	}

	query := fmt.Sprintf(`
		SELECT
			COUNT(*) OVER(),
			id,
			name,
			category_id,
			img_url,
			price,
			count,
			decription,
			is_active,
			index,
			is_new,
			discount_price,
			post_id,
			created_at,
			updated_at
		FROM product
		%s
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
			&p.Name,
			&p.CategoryID,
			&p.ImgUrl,
			&p.Price,
			&p.Count,
			&p.Description,
			&p.IsActive,
			&p.Index,
			&p.IsNew,
			&p.DiscountPrice,
			&p.PostID,
			&p.CreatedAt,
			&p.UpdatedAt,
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
	if req.CategoryID != nil {
		setValues = append(setValues, "category_id = :category_id")
		params["category_id"] = *req.CategoryID
	}
	if req.ImgUrl != nil {
		setValues = append(setValues, "img_url = :img_url")
		params["img_url"] = *req.ImgUrl
	}
	if req.Price != nil {
		setValues = append(setValues, "price = :price")
		params["price"] = *req.Price
	}
	if req.Count != nil {
		setValues = append(setValues, "count = :count")
		params["count"] = *req.Count
	}
	if req.Description != nil {
		setValues = append(setValues, "description = :description")
		params["description"] = *req.Description
	}
	if req.IsActive != nil {
		setValues = append(setValues, "is_active = :is_active")
		params["is_active"] = *req.IsActive
	}
	if req.Index != nil {
		setValues = append(setValues, "index = :index")
		params["index"] = *req.Index
	}
	if req.IsNew != nil {
		setValues = append(setValues, "is_new = :is_new")
		params["is_new"] = *req.IsNew
	}
	if req.DiscountPrice != nil {
		setValues = append(setValues, "discount_price = :discount_price")
		params["discount_price"] = *req.DiscountPrice
	}
	if req.PostID != nil {
		setValues = append(setValues, "post_id = :post_id")
		params["post_id"] = *req.PostID
	}
	setValues = append(setValues, "updated_At = NOW()")
	if len(setValues) == 0 {
		return 0, fmt.Errorf("no fields to update for product with ID %d", req.ID)
	}
	query := fmt.Sprintf(`
        UPDATE product
        SET %s
        WHERE id = :id
    `, strings.Join(setValues, ", "))

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("error starting transaction for product ID %d: %w", req.ID, err)
	}

	query, args := utils.ReplaceQueryParams(query, params)
	result, err := tx.Exec(ctx, query, args...)
	if err != nil {
		if errRollback := tx.Rollback(ctx); errRollback != nil {
			r.logger.Error(ctx, "error rolling back transaction", zap.Error(errRollback))
			return 0, fmt.Errorf("error rolling back transaction for product ID %d: %w", req.ID, errRollback)
		}
		r.logger.Error(ctx, "error executing update", zap.Error(err))
		return 0, fmt.Errorf("error updating product with ID %d: %w", req.ID, err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		if errRollback := tx.Rollback(ctx); errRollback != nil {
			r.logger.Error(ctx, "error rolling back transaction", zap.Error(errRollback))
			return 0, fmt.Errorf("error rolling back transaction for product ID %d: %w", req.ID, errRollback)
		}
		r.logger.Warn(ctx, "no product found with the given ID", zap.String("product_id", cast.ToString(req.ID)))
		return 0, fmt.Errorf("no product found with ID %d", req.ID)
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Error(ctx, "error committing transaction", zap.Error(err))
		return 0, fmt.Errorf("error committing transaction for product ID %d: %w", req.ID, err)
	}

	return rowsAffected, nil
}

func (r *repo) Delete(ctx context.Context, productID int64) error {
	r.logger.Info(ctx, "Delete product", zap.String("product_id", cast.ToString(productID)))

	query := `
		DELETE FROM product
		WHERE id = $1
	`

	result, err := r.db.Exec(ctx, query, productID)
	if err != nil {
		r.logger.Error(ctx, "error executing delete", zap.Error(err))
		return fmt.Errorf("error deleting product with ID %d: %w", productID, err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn(ctx, "no product found with the given ID", zap.String("product_id", cast.ToString(productID)))
		return fmt.Errorf("no product found with ID %d", productID)
	}

	return nil
}

func (r *repo) GetListCategoryName(ctx context.Context, req string) (resp []structs.Product, err error) {
	r.logger.Info(ctx, "GetList Product by category name", zap.Any("req", req))

	pattern := "%" + req + "%"

	query := `
		SELECT
			p.id,
			p.name,
			p.category_id,
			p.img_url,
			p.price,
			p.count,
			p.decription,
			p.is_active,
			p.index,
			p.is_new,
			p.discount_price,
			p.post_id,
			p.created_at,
			p.updated_at
		FROM product AS p
		JOIN category AS c ON c.id = p.category_id
		WHERE c.name->>'uz' ILIKE $1 OR
			  c.name->>'ru' ILIKE $1 OR
			  c.name->>'en' ILIKE $1
		ORDER BY p.created_at DESC
	`

	rows, err := r.db.Query(ctx, query, pattern)
	if err != nil {
		r.logger.Error(ctx, "err on r.db.Query", zap.Error(err))
		return resp, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var list []structs.Product

	for rows.Next() {
		var p structs.Product
		err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.CategoryID,
			&p.ImgUrl,
			&p.Price,
			&p.Count,
			&p.Description,
			&p.IsActive,
			&p.Index,
			&p.IsNew,
			&p.DiscountPrice,
			&p.PostID,
			&p.CreatedAt,
			&p.UpdatedAt,
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
	return list, nil
}
