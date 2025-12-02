package categoryrepo

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
		Create(ctx context.Context, req structs.CreateCategory) (structs.Category, error)
		GetByID(ctx context.Context, id int64) (structs.Category, error)
		GetList(ctx context.Context, req structs.GetListCategoryRequest) (structs.GetListCategoryResponse, error)
		Delete(ctx context.Context, categoryID int64) error
		Patch(ctx context.Context, req structs.PatchCategory) (int64, error)
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

func (r *repo) Create(ctx context.Context, req structs.CreateCategory) (resp structs.Category, err error) {
	r.logger.Info(ctx, "Create category", zap.Any("req", req))
	query := `
        INSERT INTO category (
            name,
			post_id,
			is_active,
			"index"
        ) VALUES (
            $1,
			$2,
			$3,
			$4
        )
        RETURNING id, name, post_id, is_active, "index" created_at, updated_at
    `
	err = r.db.QueryRow(ctx, query, req.Name, req.PostID).Scan(&resp.ID, &resp.Name, &resp.PostID, &resp.IsActive, &resp.Index, &resp.CreatedAt, &resp.UpdatedAt)
	if err != nil {
		r.logger.Error(ctx, "err on r.db.QueryRow", zap.Error(err))
		return structs.Category{}, fmt.Errorf("create category failed: %w", err)
	}

	return resp, nil
}

func (r *repo) GetByID(ctx context.Context, id int64) (structs.Category, error) {
	var (
		resp  structs.Category
		query = `
			SELECT
				id,
				name,
				post_id,
				is_active,
				"index",
				created_at, 
				updated_at
			FROM category
			WHERE id = $1
		`
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&resp.ID,
		&resp.Name,
		&resp.PostID,
		&resp.IsActive,
		&resp.Index,
		&resp.CreatedAt,
		&resp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Error(ctx, " err from r.db.QueryRow", zap.Error(err))
			return structs.Category{}, structs.ErrNotFound
		}
		r.logger.Error(ctx, "error querying row", zap.Error(err))
		return structs.Category{}, fmt.Errorf("error getting category item by ID: %w", err)
	}
	return resp, err
}
func (r *repo) GetList(ctx context.Context, req structs.GetListCategoryRequest) (resp structs.GetListCategoryResponse, err error) {
	r.logger.Info(ctx, "GetList Category", zap.Any("req", req))

	limit := int64(100)
	offset := int64(0)

	if req.Limit > 0 {
		limit = req.Limit
	}
	if req.Offset > 0 {
		offset = req.Offset
	}

	query := `
		SELECT
			COUNT(*) OVER(),
			id,
			name,
			post_id,
			is_active,
			"index",
			created_at,
			updated_at
		FROM category
		WHERE TRUE
		ORDER BY index ASC, created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		r.logger.Error(ctx, "err on r.db.Query", zap.Error(err))
		return resp, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var list []structs.Category

	for rows.Next() {
		var c structs.Category

		err := rows.Scan(
			&resp.Count,
			&c.ID,
			&c.Name,
			&c.PostID,
			&c.IsActive,
			&c.Index,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			r.logger.Error(ctx, "err on rows.Scan", zap.Error(err))
			return resp, fmt.Errorf("row scan failed: %w", err)
		}

		list = append(list, c)
	}

	if rows.Err() != nil {
		r.logger.Error(ctx, "err on rows iteration", zap.Error(rows.Err()))
		return resp, fmt.Errorf("rows iteration failed: %w", rows.Err())
	}

	resp.Categories = list
	return resp, nil
}

func (r *repo) Patch(ctx context.Context, req structs.PatchCategory) (int64, error) {
	setValues := []string{}
	params := map[string]interface{}{
		"id": req.ID,
	}

	if req.Name != nil {
		setValues = append(setValues, "name = :name")
		params["name"] = *req.Name
	}
	if req.PostID != nil {
		setValues = append(setValues, "post_id = :post_id")
		params["post_id"] = *req.PostID
	}
	if req.IsActive != nil {
		setValues = append(setValues, "is_active = :is_active")
		params["is_active"] = *req.IsActive
	}
	if req.Index != nil {
		setValues = append(setValues, "index = :index")
		params["index"] = *req.Index
	}
	setValues = append(setValues, "updated_At = NOW()")
	if len(setValues) == 0 {
		return 0, fmt.Errorf("no fields to update for category with ID %d", req.ID)
	}

	query := fmt.Sprintf(`
        UPDATE category
        SET %s
        WHERE id = :id
    `, strings.Join(setValues, ", "))

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("error starting transaction for category ID %d: %w", req.ID, err)
	}

	query, args := utils.ReplaceQueryParams(query, params)
	result, err := tx.Exec(ctx, query, args...)
	if err != nil {
		if errRollback := tx.Rollback(ctx); errRollback != nil {
			r.logger.Error(ctx, "error rolling back transaction", zap.Error(errRollback))
			return 0, fmt.Errorf("error rolling back transaction for category ID %d: %w", req.ID, errRollback)
		}
		r.logger.Error(ctx, "error executing update", zap.Error(err))
		return 0, fmt.Errorf("error updating category with ID %d: %w", req.ID, err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		if errRollback := tx.Rollback(ctx); errRollback != nil {
			r.logger.Error(ctx, "error rolling back transaction", zap.Error(errRollback))
			return 0, fmt.Errorf("error rolling back transaction for category ID %d: %w", req.ID, errRollback)
		}
		r.logger.Warn(ctx, "no category found with the given ID", zap.String("category_id", cast.ToString(req.ID)))
		return 0, fmt.Errorf("no category found with ID %d", req.ID)
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Error(ctx, "error committing transaction", zap.Error(err))
		return 0, fmt.Errorf("error committing transaction for category ID %d: %w", req.ID, err)
	}

	return rowsAffected, nil
}

func (r *repo) Delete(ctx context.Context, categoryID int64) error {
	r.logger.Info(ctx, "Delete category", zap.String("category_id", cast.ToString(categoryID)))

	query := `
		DELETE FROM category
		WHERE id = $1
	`

	result, err := r.db.Exec(ctx, query, categoryID)
	if err != nil {
		r.logger.Error(ctx, "error executing delete", zap.Error(err))
		return fmt.Errorf("error deleting category with ID %d: %w", categoryID, err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn(ctx, "no category found with the given ID", zap.String("category_id", cast.ToString(categoryID)))
		return fmt.Errorf("no category found with ID %d", categoryID)
	}

	return nil
}
