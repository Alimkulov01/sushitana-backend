package clientrepo

import (
	"context"
	"database/sql"
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
		Create(ctx context.Context, req structs.CreateClient) (structs.Client, error)
		GetByTgID(ctx context.Context, tgid int64) (structs.Client, error)
		GetByID(ctx context.Context, id int64) (structs.Client, error)
		GetList(ctx context.Context, req structs.GetListClientRequest) (structs.GetListClientResponse, error)
		Delete(ctx context.Context, clientID int64) error
		UpdateLanguage(ctx context.Context, tgID int64, lang utils.Lang) error
		UpdatePhone(ctx context.Context, tgID int64, phone string) error
		UpdateName(ctx context.Context, tgID int64, name string) error
		GetLanguageByTgID(ctx context.Context, tgID int64) (string, error)
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
func (r *repo) Create(ctx context.Context, req structs.CreateClient) (resp structs.Client, err error) {
	r.logger.Info(ctx, "Create client", zap.Any("req", req))
	query := `
        INSERT INTO clients (tgid) VALUES ($1) ON CONFLICT (tgid) DO NOTHING
    `
	err = r.db.QueryRow(ctx, query, req.TgID).Scan(&resp.ID, &resp.Language)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			existing, getErr := r.GetByTgID(ctx, req.TgID)
			if getErr != nil {
				return structs.Client{}, fmt.Errorf("client already exists but could not fetch: %w", getErr)
			}
			return existing, nil
		}
		r.logger.Error(ctx, "err on r.db.QueryRow", zap.Error(err))
		return structs.Client{}, fmt.Errorf("create client failed: %w", err)
	}

	return resp, nil
}

func (r repo) GetByTgID(ctx context.Context, tgid int64) (structs.Client, error) {
	var (
		resp  structs.Client
		lang  sql.NullString
		query = `
            SELECT
                id,
                tgid,
                phone,
                language,
                created_at,
                updated_at
            FROM clients
            WHERE tgid = $1
        `
	)

	err := r.db.QueryRow(ctx, query, tgid).Scan(
		&resp.ID,
		&resp.TgID,
		&resp.Phone,
		&lang,
		&resp.CreatedAt,
		&resp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Error(ctx, " err from r.db.QueryRow", zap.Error(err))
			return structs.Client{}, structs.ErrNotFound
		}
		r.logger.Error(ctx, "error querying row", zap.Error(err))
		return structs.Client{}, fmt.Errorf("error getting item by ID: %w", err)
	}

	if lang.Valid && lang.String != "" {
		if parsed, ok := utils.ParseLang(lang.String); ok {
			resp.Language = parsed
		} else {
			resp.Language = ""
		}
	} else {
		resp.Language = ""
	}

	return resp, nil
}

func (r repo) GetByID(ctx context.Context, id int64) (structs.Client, error) {
	var (
		resp  structs.Client
		query = `
			SELECT
				id,
				tgid,
				phone,
				language,
				created_at, 
				updated_at
			FROM clients
			WHERE id = $1
		`
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&resp.ID,
		&resp.TgID,
		&resp.Phone,
		&resp.Language,
		&resp.CreatedAt,
		&resp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Error(ctx, " err from r.db.QueryRow", zap.Error(err))
			return structs.Client{}, structs.ErrNotFound
		}
		r.logger.Error(ctx, "error querying row", zap.Error(err))
		return structs.Client{}, fmt.Errorf("error getting item by ID: %w", err)
	}
	return resp, err
}

func (r repo) GetList(ctx context.Context, req structs.GetListClientRequest) (resp structs.GetListClientResponse, err error) {
	r.logger.Info(ctx, "GetList", zap.Any("req", req))

	filterSQL, args := filterClientQuery(req)

	query := `
		SELECT
			COUNT(*) OVER(), 
			id,
			tgid,
			phone,
			language,
			created_at, 
			updated_at
		FROM clients
	` + filterSQL

	rows, err := r.db.Query(ctx, query, args)
	if err != nil {
		r.logger.Error(ctx, "err on r.db.Query", zap.Error(err))
		return structs.GetListClientResponse{}, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var clients []structs.Client

	for rows.Next() {
		var client structs.Client
		if err := rows.Scan(
			&resp.Count,
			&client.ID,
			&client.TgID,
			&client.Phone,
			&client.Language,
			&client.CreatedAt,
			&client.UpdatedAt,
		); err != nil {
			r.logger.Error(ctx, "err on rows.Scan", zap.Error(err))
			return structs.GetListClientResponse{}, fmt.Errorf("row scan failed: %w", err)
		}
		clients = append(clients, client)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error(ctx, "err on rows iteration", zap.Error(err))
		return structs.GetListClientResponse{}, fmt.Errorf("rows iteration failed: %w", err)
	}
	resp.Clients = clients
	return resp, nil
}

func filterClientQuery(req structs.GetListClientRequest) (string, pgx.NamedArgs) {
	queryParams := pgx.NamedArgs{
		"limit":  100,
		"offset": 0,
	}

	var b strings.Builder
	b.WriteString(" WHERE TRUE")

	if !utils.StrEmpty(strings.TrimSpace(req.Search)) {
		search := "%" + strings.TrimSpace(req.Search) + "%"
		queryParams["search"] = search

		b.WriteString(`
			AND (
				phone ILIKE @search
			)
		`)
	}

	if req.Limit > 0 {
		queryParams["limit"] = req.Limit
	}
	if req.Offset > 0 {
		queryParams["offset"] = req.Offset
	}

	b.WriteString(" ORDER BY created_at DESC")
	b.WriteString(" LIMIT @limit OFFSET @offset")

	return b.String(), queryParams
}

func (r *repo) Delete(ctx context.Context, clientID int64) error {
	r.logger.Info(ctx, "Delete client", zap.String("client_id", cast.ToString(clientID)))

	query := `
		DELETE FROM clients
		WHERE id = $1
	`

	result, err := r.db.Exec(ctx, query, clientID)
	if err != nil {
		r.logger.Error(ctx, "error executing delete", zap.Error(err))
		return fmt.Errorf("error deleting client with ID %d: %w", clientID, err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn(ctx, "no client found with the given ID", zap.String("client_id", cast.ToString(clientID)))
		return fmt.Errorf("no client found with ID %d", clientID)
	}

	return nil
}

func (r *repo) UpdateLanguage(ctx context.Context, tgID int64, lang utils.Lang) error {
	query := `
        UPDATE clients
        SET language = $1,
            updated_at = now()
        WHERE tgid = $2
    `
	result, err := r.db.Exec(ctx, query, lang, tgID)
	if err != nil {
		r.logger.Error(ctx, "error updating lang", zap.Error(err))
		return fmt.Errorf("failed to update lang: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.logger.Warn(ctx, "no client found with given tgid", zap.Int64("tgid", tgID))
		return structs.ErrNotFound
	}

	return nil
}

func (r *repo) UpdatePhone(ctx context.Context, tgID int64, phone string) error {
	query := `
        UPDATE clients
        SET phone = $1,
            updated_at = now()
        WHERE tgid = $2
    `
	result, err := r.db.Exec(ctx, query, phone, tgID)
	if err != nil {
		r.logger.Error(ctx, "error updating phone", zap.Error(err))
		return fmt.Errorf("failed to update phone: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.logger.Warn(ctx, "no client found with given tgid", zap.Int64("tgid", tgID))
		return structs.ErrNotFound
	}

	return nil
}

func (r *repo) UpdateName(ctx context.Context, tgID int64, name string) error {
	query := `
        UPDATE clients
        SET name = $1,
            updated_at = now()
        WHERE tgid = $2
    `
	result, err := r.db.Exec(ctx, query, name, tgID)
	if err != nil {
		r.logger.Error(ctx, "error updating name", zap.Error(err))
		return fmt.Errorf("failed to update name: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.logger.Warn(ctx, "no client found with given tgid", zap.Int64("tgid", tgID))
		return structs.ErrNotFound
	}

	return nil
}

func (r *repo) GetLanguageByTgID(ctx context.Context, tgID int64) (string, error) {
	r.logger.Info(ctx, "Update client lang", zap.Int64("tgid", tgID))
	var language string
	query := `
        SELECT
			language
		FROM clients
		WHERE tgid = $1
    `
	err := r.db.QueryRow(ctx, query, tgID).Scan(
		&language,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.logger.Error(ctx, " err from r.db.QueryRow", zap.Error(err))
			return "", structs.ErrNotFound
		}
		r.logger.Error(ctx, "error querying row", zap.Error(err))
		return "", fmt.Errorf("error getting item by ID: %w", err)
	}
	return language, err
}
