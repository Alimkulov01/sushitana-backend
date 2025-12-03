package menurepo

import (
	"context"
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
		GetMenu(ctx context.Context) ([]structs.Menu, error)
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
func (r repo) GetMenu(ctx context.Context) ([]structs.Menu, error) {
	r.logger.Info(ctx, "GetMenu called")
	query := `
		SELECT
			c.id,
			c.name,
			c.post_id,
			c.index,
			c.is_active,
			c.created_at,
			c.updated_at,
			(
				SELECT COALESCE(
					JSON_AGG(
						JSON_BUILD_OBJECT(
							'id', p.id,
							'name', p.name,
							'category_id', p.category_id,
							'img_url', p.img_url,
							'price', p.price,
							'count', p.count,
							'description', p.description,
							'is_active', p.is_active,
							'index', p.index,
							'is_new', p.is_new,
							'discount_price', p.discount_price,
							'post_id', p.post_id,
							'created_at', TO_CHAR(p.created_at, 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),
							'updated_at', TO_CHAR(p.updated_at, 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"')
						)
						ORDER BY p.index ASC
					)
					FILTER (WHERE p.id IS NOT NULL),
					'[]'
				)
				FROM product p
				WHERE p.category_id = c.id
			) AS products
		FROM category c
		WHERE c.is_active = TRUE
		ORDER BY c.index ASC;
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.logger.Error(ctx, "failed to get menu", zap.Error(err))
		return nil, err
	}
	defer rows.Close()
	var menus []structs.Menu
	for rows.Next() {
		var menu structs.Menu
		if err := rows.Scan(
			&menu.Category.ID,
			&menu.Category.Name,
			&menu.Category.PostID,
			&menu.Category.Index,
			&menu.Category.IsActive,
			&menu.Category.CreatedAt,
			&menu.Category.UpdatedAt,
			&menu.Products,
		); err != nil {
			r.logger.Error(ctx, "failed to scan menu row", zap.Error(err))
			return nil, err
		}
		menus = append(menus, menu)
	}
	if err := rows.Err(); err != nil {
		r.logger.Error(ctx, "rows error after iteration", zap.Error(err))
		return nil, err
	}
	return menus, nil

}
