package iikorepo

import (
	"context"
	"database/sql"
	"fmt"
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
		CreateIIKO(ctx context.Context, token string) error
		UpdateIIKO(ctx context.Context, id int64, token string) (int64, error)
		GetTokenIIKO(ctx context.Context, id int64) (token string, err error)
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

func (r repo) CreateIIKO(ctx context.Context, token string) error {
	query := `
	INSERT INTO iiko_tokens (id, token, created_at, updated_at)
	VALUES (1, $1, now(), now())
	ON CONFLICT (id) DO UPDATE
	  SET token = EXCLUDED.token,
	      updated_at = now();
	`
	_, err := r.db.Exec(ctx, query, token)
	if err != nil {
		r.logger.Error(ctx, "Failed to upsert IIKO token", zap.Error(err))
		return fmt.Errorf("create/upsert iiko token: %w", err)
	}
	return nil
}

func (r repo) GetTokenIIKO(ctx context.Context, id int64) (string, error) {
	var token string
	query := `SELECT token FROM iiko_tokens WHERE id = $1 LIMIT 1`
	if err := r.db.QueryRow(ctx, query, id).Scan(&token); err != nil {
		if err == sql.ErrNoRows {
			// no token stored yet
			return "", sql.ErrNoRows
		}
		r.logger.Error(ctx, "Failed to get IIKO token", zap.Error(err))
		return "", fmt.Errorf("get iiko token: %w", err)
	}
	return token, nil
}

func (r repo) UpdateIIKO(ctx context.Context, id int64, token string) (int64, error) {
	query := `UPDATE iiko_tokens SET token = $1, updated_at = NOW() WHERE id = $2`
	cmd, err := r.db.Exec(ctx, query, token, id)
	if err != nil {
		r.logger.Error(ctx, "Failed to update IIKO token", zap.Error(err))
		return 0, fmt.Errorf("update iiko token: %w", err)
	}
	// try to get rows affected if supported
	if ra := cmd.RowsAffected(); ra >= 0 {
		if ra == 0 {
			return 0, sql.ErrNoRows
		}
		return ra, nil
	}
	return 1, nil // fallback — assume one row updated
}
