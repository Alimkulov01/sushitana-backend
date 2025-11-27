package userRepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sushitana/internal/structs"
	"sushitana/pkg/db"
	"sushitana/pkg/logger"
	"sushitana/pkg/utils"

	"go.uber.org/fx"
	"golang.org/x/crypto/bcrypt"
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
		LoginAdmin(ctx context.Context, req structs.AdminLogin) (structs.AuthResponse, error)
		GetMe(ctx context.Context, token string) (structs.GetMeResponse, error)
		GetUserPermissions(ctx context.Context, role_id string) ([]string, error)
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

func (r repo) LoginAdmin(ctx context.Context, req structs.AdminLogin) (structs.AuthResponse, error) {

	var (
		query = `
            SELECT
                id,
                password_hash,
				role_id
            FROM admins
            WHERE username = $1
        `
		id       sql.NullString
		password sql.NullString
		role_id  sql.NullString
	)
	err := r.db.QueryRow(ctx, query, req.Username).Scan(
		&id,
		&password,
		&role_id,
	)
	if err != nil {
		return structs.AuthResponse{}, fmt.Errorf("login or password error: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(password.String), []byte(req.Password)); err != nil {
		return structs.AuthResponse{}, fmt.Errorf("invalid credentials: %w", err)
	}

	token, err := utils.GenerateJWT(id.String, role_id.String)
	if err != nil {
		return structs.AuthResponse{}, fmt.Errorf("generate jwt token error: %w", err)
	}
	_, err = r.db.Exec(ctx, `UPDATE admins SET last_login = NOW() WHERE id = $1`, id.String)
	if err != nil {
		return structs.AuthResponse{}, fmt.Errorf("failed to update last login: %w", err)
	}

	return structs.AuthResponse{
		Token: token,
	}, nil
}

func (r repo) GetMe(ctx context.Context, token string) (structs.GetMeResponse, error) {
	token = strings.TrimPrefix(token, "Bearer ")
	claims, err := utils.ParseJWT(token)
	if err != nil {
		return structs.GetMeResponse{}, err
	}
	adminID, _ := claims["id"].(string)

	const query = `
		SELECT
			COALESCE(e.id::text, a.id::text)                             AS id,
			COALESCE(e.username, a.username)                             AS username,
			e.surname                                                    AS last_name,
			e.name                                                       AS first_name,
			e.phone_number                                               AS phone_number,
			a.last_login,
			a.is_superuser,
			r.id::text                                                   AS role_id,
			r.role_name,
			r.role_description,
			r.created_at,
			r.updated_at
		FROM admins a
		LEFT JOIN employees e ON e.username = a.username
		JOIN roles r ON r.id = a.role_id
		WHERE a.id = $1
	`

	const queryAccessScope = `
		SELECT s.id, s.name, s.description
		FROM access_scopes s
		JOIN role_access_scopes ras ON ras.access_scope_id = s.id
		WHERE ras.role_id = $1
	`

	var (
		id, username, lastName, firstName, phoneNumber sql.NullString
		lastLogin, roleID, roleName, roleDesc          sql.NullString
		roleCreatedAt, roleUpdatedAt                   sql.NullString
		isSuperuser                                    sql.NullBool
	)

	if err := r.db.QueryRow(ctx, query, adminID).Scan(
		&id, &username, &lastName, &firstName, &phoneNumber,
		&lastLogin, &isSuperuser,
		&roleID, &roleName, &roleDesc, &roleCreatedAt, &roleUpdatedAt,
	); err != nil {
		return structs.GetMeResponse{}, err
	}

	rows, err := r.db.Query(ctx, queryAccessScope, roleID.String)
	if err != nil {
		return structs.GetMeResponse{}, err
	}
	defer rows.Close()

	var accessScopes []structs.AccessScope
	for rows.Next() {
		var s structs.AccessScope
		if err := rows.Scan(&s.Id, &s.Name, &s.Description); err != nil {
			return structs.GetMeResponse{}, err
		}
		accessScopes = append(accessScopes, s)
	}
	if err := rows.Err(); err != nil {
		return structs.GetMeResponse{}, err
	}

	return structs.GetMeResponse{
		ID:          id.String,
		UserName:    username.String,
		FirstName:   firstName.String,
		LastName:    lastName.String,
		Phone:       phoneNumber.String,
		LastLogin:   lastLogin.String,
		IsSuperUser: isSuperuser.Bool,
		Role: structs.Role{
			Id:              roleID.String,
			RoleName:        roleName.String,
			RoleDescription: roleDesc.String,
			CreatedAt:       roleCreatedAt.String,
			UpdatedAt:       roleUpdatedAt.String,
			AccessScopes:    accessScopes,
		},
	}, nil
}

func (r repo) GetUserPermissions(ctx context.Context, token string) ([]string, error) {
	token = strings.TrimPrefix(token, "Bearer ")
	claims, err := utils.ParseJWT(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	roleID, ok := claims["role_id"].(string)
	if !ok || roleID == "" {
		return nil, fmt.Errorf("role_id not found in token")
	}
	var (
		query = `
			SELECT 
				access_scopes.name 
			FROM role_access_scopes
			JOIN access_scopes ON role_access_scopes.access_scope_id = access_scopes.id
			WHERE role_access_scopes.role_id = $1
		`
	)
	rows, err := r.db.Query(ctx, query, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var permission string
		if err := rows.Scan(&permission); err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}

	return permissions, nil
}
