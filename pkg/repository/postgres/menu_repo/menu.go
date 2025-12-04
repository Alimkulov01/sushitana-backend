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
			c.is_active,
			c."index", 
			c.created_at, 
			c.updated_at, 
			c.parent_id, 
			COALESCE(c.is_included_in_menu, false) AS isIncludedInMenu,
			COALESCE(c.is_group_modifier, false) AS isGroupModifier,
			COALESCE(c.is_deleted, false) AS isDeleted,
			(
				SELECT COALESCE(
					jsonb_agg(
						jsonb_build_object(
							'id', p.id,
							'group_id', p.group_id,
							'name', p.name,
							'productCategoryId', p.product_category_id,
							'type', p.type,
							'orderItemType', p.order_item_type,
							'measureUnit', p.measure_unit,
							'sizePrices', COALESCE(p.size_prices, '[]'::jsonb),
							'doNotPrintInCheque', COALESCE(p.do_not_print_in_cheque, false),
							'parentGroup', p.parent_group,
							'order', p."order",
							'paymentSubject', p.payment_subject,
							'code', p.code,
							'isDeleted', COALESCE(p.is_deleted, false),
							'canSetOpenPrice', COALESCE(p.can_set_open_price, false),
							'splittable', COALESCE(p.splittable, false),
							'index', p.index,
							'isNew', COALESCE(p.is_new, false),
							'imgUrl', p.img_url,
							'isActive', COALESCE(p.is_active, false),
							'isHaveBox', p.is_have_box,
							'boxCount', p.box_count,
							'boxPrice', p.box_price,
							'description', COALESCE(p.description, '{}'::jsonb),
							'weight', p.weight,
							'createdAt', TO_CHAR(p.created_at, 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),
							'updatedAt', TO_CHAR(p.updated_at, 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"')
						)
						ORDER BY p.index ASC
					) FILTER (WHERE p.id IS NOT NULL),
					'[]'::jsonb
				)
				FROM product p
				WHERE p.parent_group = c.id
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
			&menu.Category.IsActive,
			&menu.Category.Index,
			&menu.Category.CreatedAt,
			&menu.Category.UpdatedAt,
			&menu.Category.ParentID,
			&menu.Category.IsIncludedInMenu,
			&menu.Category.IsGroupModifier,
			&menu.Category.IsDeleted,
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
