package state

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/fx"

	"github.com/jackc/pgx/v5"

	"sushitana/internal/structs"
	"sushitana/pkg/db"
	"sushitana/pkg/logger"
	"sushitana/pkg/tgrouter/interfaces"
)

var Module = fx.Provide(New)

type Params struct {
	fx.In
	Logger logger.Logger
	DB     db.Querier
}

type state struct {
	logger logger.Logger
	db     db.Querier
}

func New(params Params) interfaces.State {
	return &state{
		logger: params.Logger,
		db:     params.DB,
	}
}

func pgxErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil
	default:
		return err
	}
}

func (s *state) Get(ctx context.Context, userId, chatId int) (string, map[string]string, error) {
	var (
		state string
		data  map[string]string
	)
	err := s.db.QueryRow(ctx, "SELECT state, data FROM state WHERE user_id = $1 AND chat_id = $2", userId, chatId).Scan(
		&state,
		&data)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, structs.ErrNotFound
		}
		return "", nil, fmt.Errorf("repo: failed get state: %w", pgxErr(err))
	}
	return state, data, nil
}

func (s *state) Set(ctx context.Context, userId, chatId int, state string, data map[string]string) error {
	_, err := s.db.Exec(ctx, `INSERT INTO state (user_id, chat_id, state, data) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, chat_id) DO UPDATE SET state = $3, data = $4`,
		userId, chatId, state, data)

	if err != nil {
		return fmt.Errorf("repo: failed update state: %w", err)
	}
	return nil
}

func (s *state) Delete(ctx context.Context, userId, chatId int) error {
	_, err := s.db.Exec(ctx, "DELETE FROM state WHERE user_id = $1 AND chat_id = $2", userId, chatId)
	if err != nil {
		return fmt.Errorf("repo: failed delete state: %w", err)
	}
	return nil
}

func (s *state) GetData(ctx context.Context, userId, chatId int, key string) (string, error) {
	var data string
	err := s.db.QueryRow(ctx, "SELECT coalesce(data->>$3, '') FROM state WHERE user_id = $1 AND chat_id = $2",
		userId, chatId, key).Scan(&data)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", structs.ErrNotFound
		}
		return "", fmt.Errorf("repo: failed get data: %w", pgxErr(err))
	}
	return data, nil
}

func (s *state) UpdateData(ctx context.Context, userId, chatId int, data map[string]string) error {
	_, err := s.db.Exec(ctx, `UPDATE state SET data = data || $3 WHERE user_id = $1 AND chat_id = $2`,
		userId, chatId, data)

	if err != nil {
		return fmt.Errorf("repo: failed update data: %w", err)
	}
	return nil
}
