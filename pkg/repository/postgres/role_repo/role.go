package rolerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sushitana/internal/structs"
	"sushitana/pkg/db"
	"sushitana/pkg/logger"
	"sushitana/pkg/utils"

	"github.com/google/uuid"
	"go.uber.org/fx"
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
		Create(ctx context.Context, req structs.CreateRole) error
		GetById(ctx context.Context, req structs.RolePrimaryKey) (structs.Role, error)
		GetAll(ctx context.Context, req structs.GetListRoleRequest) (structs.GetListRoleResponse, error)
		Delete(ctx context.Context, req structs.RolePrimaryKey) error
		Patch(ctx context.Context, req structs.PatchRole) (int64, error)
		GetAccessScope(ctx context.Context) ([]structs.AccessScope, error)
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

func (r repo) Create(ctx context.Context, req structs.CreateRole) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}

	var (
		role_id = uuid.NewString()
		query   = `
			INSERT INTO roles(
				id,
				role_name,
				role_description
			) VALUES ($1, $2, $3)
		`

		queryAccessScope = `
			INSERT INTO "role_access_scopes"(
				"role_id",
				"access_scope_id"
			) VALUES($1, $2)
		`
	)

	_, err = tx.Exec(ctx, query,
		role_id,
		req.RoleName,
		req.RoleDescription,
	)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	for _, access_scope := range req.AccessScopes {
		_, err = tx.Exec(ctx, queryAccessScope,
			role_id,
			access_scope,
		)
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r repo) GetById(ctx context.Context, req structs.RolePrimaryKey) (structs.Role, error) {
	var (
		query = `
			SELECT 
				"id",
				"role_name",
				"role_description",
				"created_at",
				"updated_at"
			FROM "roles"
			WHERE "id" = $1
		`
		queryAccessScope = `
			SELECT
				s."id",
				s."name",
				s."description"
			FROM "access_scopes" as s
			JOIN role_access_scopes as ras ON ras.access_scope_id = s.id
			WHERE ras."role_id" = $1 
		`

		id               sql.NullString
		role_name        sql.NullString
		role_description sql.NullString
		created_at       sql.NullString
		updated_at       sql.NullString
		access_scopes    []structs.AccessScope
	)

	err := r.db.QueryRow(ctx, query, req.Id).Scan(
		&id,
		&role_name,
		&role_description,
		&created_at,
		&updated_at,
	)
	if err != nil {
		return structs.Role{}, err
	}

	rows, err := r.db.Query(ctx, queryAccessScope, req.Id)
	if err != nil {
		return structs.Role{}, err
	}

	for rows.Next() {
		var (
			access_scope structs.AccessScope
		)

		err = rows.Scan(
			&access_scope.Id,
			&access_scope.Name,
			&access_scope.Description,
		)
		if err != nil {
			return structs.Role{}, err
		}

		access_scopes = append(access_scopes, access_scope)
	}

	return structs.Role{
		Id:              id.String,
		RoleName:        role_name.String,
		RoleDescription: role_description.String,
		AccessScopes:    access_scopes,
		CreatedAt:       created_at.String,
		UpdatedAt:       updated_at.String,
	}, nil
}

func (r repo) GetAll(ctx context.Context, req structs.GetListRoleRequest) (structs.GetListRoleResponse, error) {
	var (
		query = `
			SELECT 
				COUNT(*) OVER(),
				r.id,
				r.role_name,
				r.role_description,
				r.created_at,
				r.updated_at,
				COALESCE(json_agg(
					json_build_object(
						'id', a.id,
						'name', a.name,
						'description', a.description
					)
				) FILTER (WHERE a.id IS NOT NULL), '[]') as access_scopes,
				COUNT(a.id) FILTER (WHERE a.id IS NOT NULL) as access_scope_count
			FROM roles r
			LEFT JOIN role_access_scopes ra ON ra.role_id = r.id
			LEFT JOIN access_scopes a ON a.id = ra.access_scope_id
		`

		resp   structs.GetListRoleResponse
		where  = " WHERE TRUE"
		offset = " OFFSET 0"
		limit  = " LIMIT 10"
		sort   = " GROUP BY r.id ORDER BY r.created_at DESC"
	)

	if req.Offset > 0 {
		offset = fmt.Sprintf(" OFFSET %d", req.Offset)
	}
	if req.Limit > 0 {
		limit = fmt.Sprintf(" LIMIT %d", req.Limit)
	}
	if len(req.Search) > 0 {
		where += fmt.Sprintf(` AND (r.role_name ILIKE '%%%s%%' OR r.role_description ILIKE '%%%s%%')`, req.Search, req.Search)
	}

	query += where + sort + limit + offset

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return structs.GetListRoleResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			role             structs.Role
			id               sql.NullString
			roleName         sql.NullString
			roleDescription  sql.NullString
			createdAt        sql.NullString
			updatedAt        sql.NullString
			accessScopesRaw  []byte
			accessScopeCount sql.NullInt64
		)

		err = rows.Scan(
			&resp.Count,
			&id,
			&roleName,
			&roleDescription,
			&createdAt,
			&updatedAt,
			&accessScopesRaw,
			&accessScopeCount,
		)
		if err != nil {
			return structs.GetListRoleResponse{}, err
		}

		var accessScopes []structs.AccessScope
		if err := json.Unmarshal(accessScopesRaw, &accessScopes); err != nil {
			return structs.GetListRoleResponse{}, err
		}

		role = structs.Role{
			Id:               id.String,
			RoleName:         roleName.String,
			RoleDescription:  roleDescription.String,
			AccessScopes:     accessScopes,
			AccessScopeCount: accessScopeCount.Int64,
			CreatedAt:        createdAt.String,
			UpdatedAt:        updatedAt.String,
		}

		resp.Roles = append(resp.Roles, role)
	}
	return resp, nil
}

func (r repo) Update(ctx context.Context, req structs.UpdateRole) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}

	var (
		query = `
            UPDATE "roles"
                SET
                    role_name = :role_name,
                    role_description = :role_description,
                    updated_at = NOW()
                WHERE id = :id`
	)

	params := map[string]interface{}{
		"id":               req.Id,
		"role_name":        req.RoleName,
		"role_description": req.RoleDescription,
	}

	query, args := utils.ReplaceQueryParams(query, params)
	result, err := tx.Exec(ctx, query, args...)
	if err != nil {
		tx.Rollback(ctx)
		return 0, err
	}

	rowsAffected := result.RowsAffected()

	if _, err = tx.Exec(ctx, `DELETE FROM role_access_scopes WHERE role_id = $1`, req.Id); err != nil {
		tx.Rollback(ctx)
		return 0, err
	}

	if len(req.AccessScopes) > 0 {
		var queryAccessScope = `
            INSERT INTO "role_access_scopes"(
                "role_id",
                "access_scope_id"
            ) VALUES($1, $2)`

		for _, access_scope := range req.AccessScopes {
			if _, err = tx.Exec(ctx, queryAccessScope, req.Id, access_scope); err != nil {
				tx.Rollback(ctx)
				return 0, err
			}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}

	return rowsAffected, nil
}

func (r repo) Patch(ctx context.Context, req structs.PatchRole) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}

	setValues := []string{}
	params := map[string]interface{}{
		"id": req.Id,
	}

	if req.RoleName != nil {
		setValues = append(setValues, "role_name = :role_name")
		params["role_name"] = *req.RoleName
	}
	if req.RoleDescription != nil {
		setValues = append(setValues, "role_description = :role_description")
		params["role_description"] = *req.RoleDescription
	}
	setValues = append(setValues, "updated_at = NOW()")
	if len(setValues) == 1 && req.AccessScopes == nil {
		return 0, errors.New("no fields to update")
	}

	var rowsAffected int64
	if len(setValues) > 1 {
		query := fmt.Sprintf(`
			UPDATE "roles"
				SET %s
				WHERE id = :id
		`, strings.Join(setValues, ", "))

		query, args := utils.ReplaceQueryParams(query, params)
		result, err := tx.Exec(ctx, query, args...)
		if err != nil {
			tx.Rollback(ctx)
			return 0, err
		}

		rowsAffected = result.RowsAffected()
	}
	if req.AccessScopes != nil {
		if _, err = tx.Exec(ctx, `DELETE FROM role_access_scopes WHERE role_id = $1`, req.Id); err != nil {
			tx.Rollback(ctx)
			return 0, err
		}

		// Keyin yangi access scope'larni qo'shamiz
		if len(*req.AccessScopes) > 0 {
			var queryAccessScope = `
				INSERT INTO "role_access_scopes"(
					"role_id",
					"access_scope_id"
				) VALUES($1, $2)`

			for _, access_scope := range *req.AccessScopes {
				if _, err = tx.Exec(ctx, queryAccessScope, req.Id, access_scope); err != nil {
					tx.Rollback(ctx)
					return 0, err
				}
			}
		}

		if len(setValues) == 1 {
			var exists bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM roles WHERE id = $1)`, req.Id).Scan(&exists)
			if err != nil {
				tx.Rollback(ctx)
				return 0, err
			}
			if exists {
				rowsAffected = 1
			}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}

	return rowsAffected, nil
}

func (r repo) Delete(ctx context.Context, req structs.RolePrimaryKey) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	_, err = tx.Exec(ctx, `DELETE FROM "role_access_scopes" WHERE "role_id" = $1`, req.Id)
	if err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("failed to delete role access scopes: %w", err)
	}

	result, err := tx.Exec(ctx, `DELETE FROM "roles" WHERE "id" = $1`, req.Id)
	if err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("failed to delete role: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		tx.Rollback(ctx)
		return fmt.Errorf("role with id %s not found", req.Id)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r repo) GetAccessScope(ctx context.Context) ([]structs.AccessScope, error) {
	var (
		resp  []structs.AccessScope
		query = `
			SELECT 
				"id",
				"name",
				"description"
			FROM "access_scopes"
		`
	)
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var accessScope structs.AccessScope
		err := rows.Scan(
			&accessScope.Id,
			&accessScope.Name,
			&accessScope.Description,
		)
		if err != nil {
			return nil, err
		}
		resp = append(resp, accessScope)
	}
	return resp, nil
}
