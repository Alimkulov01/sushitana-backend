package userRepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sushitana/internal/structs"
	"sushitana/pkg/db"
	"sushitana/pkg/logger"
	"sushitana/pkg/utils"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
		GetUserByUsername(ctx context.Context, username string) (user structs.User, err error)
		GetUserWithPolicyByUsername(ctx context.Context, username string) (user structs.User, err error)
		GetUserByID(ctx context.Context, id int) (user structs.User, err error)
		Create(ctx context.Context, user structs.User) (id int, err error)
		GetUsers(ctx context.Context, request structs.Filter) (structs.UserList, error)
		Delete(ctx context.Context, id int) error
		TokenUser(ctx context.Context, login, token string) error
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

func (r repo) GetUserByUsername(ctx context.Context, username string) (user structs.User, err error) {
	return r.selectUser(ctx, `u.username = $1`, username)
}

func (r repo) GetUserWithPolicyByUsername(ctx context.Context, username string) (user structs.User, err error) {

	var (
		createdAt time.Time
	)

	err = r.db.QueryRow(ctx, `
	select 
		u.id,
		username,
		password_hash,
		role,
	   	u.created_at
	from users u
	where username = $1
	GROUP BY u.id`, username).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Role,
		&createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user, structs.ErrNotFound
		}
		r.logger.Error(ctx, " err from r.db.QueryRow", zap.Error(err))
		return user, err
	}

	user.CreatedAt = createdAt.Format(utils.Layout)

	return user, nil
}

func (r repo) GetUserByID(ctx context.Context, id int) (user structs.User, err error) {

	r.logger.Info(ctx, "GetUserByID", zap.Int("id", id))

	return r.selectUser(ctx, `u.id = $1`, id)
}

func (r repo) selectUser(ctx context.Context, c string, v ...interface{}) (user structs.User, err error) {
	var (
		createdAt time.Time
		updatedAt time.Time
	)

	err = r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			u.id,
			u.username,
			u.password_hash,
			u.role,
			u.created_at,
			u.updated_at
		FROM users u WHERE %s`, c), v...).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Role,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user, structs.ErrNotFound
		}
		r.logger.Error(ctx, " err from r.db.QueryRow", zap.Error(err))
		return user, err
	}

	user.CreatedAt = createdAt.Format(utils.Layout)
	user.UpdatedAt = updatedAt.Format(utils.Layout)

	return user, nil
}

func (r repo) Create(ctx context.Context, user structs.User) (id int, err error) {
	var pgErr = &pgconn.PgError{}

	err = r.db.QueryRow(ctx, `
		insert into users (
			username,
			password_hash,
			role) VALUES ($1, $2, $3) RETURNING id`,
		user.Username,
		user.Password,
		user.Role,
	).Scan(&id)
	if err != nil {
		errors.As(err, &pgErr)
		if pgerrcode.UniqueViolation == pgErr.Code {
			return 0, structs.ErrUniqueViolation
		}
		r.logger.Error(ctx, " err on r.db.Exec", zap.Error(err))
		return 0, err
	}

	return id, nil
}

func (r repo) updateUser(ctx context.Context, set, where string, v ...interface{}) error {
	exec, err := r.db.Exec(ctx, fmt.Sprintf(`update users set %s, updated_at = now() where %s;`, set, where), v...)
	if err != nil {
		r.logger.Error(ctx, " err on r.db.Exec", zap.Error(err))
		return err
	}

	if exec.RowsAffected() == 0 {
		return structs.ErrNoRowsAffected
	}

	return nil
}

func (r repo) GetUsers(ctx context.Context, filter structs.Filter) (list structs.UserList, err error) {
	var (
		inc     int
		params  []interface{}
		pages   string
		_filter = " WHERE 1=1"
		order   = " ORDER BY u.id DESC"
	)

	if !utils.StrEmpty(filter.Search) {
		_filter += fmt.Sprintf(` AND (u.username) ILIKE ('%s' || $%d || '%s')`, "%", utils.Increment(&inc), "%")
		params = append(params, filter.Search)
	}

	if filter.Limit > 0 {
		pages += fmt.Sprintf(" LIMIT $%d ", utils.Increment(&inc))
		params = append(params, filter.Limit)
	}

	if filter.Offset >= 0 {
		pages += fmt.Sprintf(" OFFSET $%d", utils.Increment(&inc))
		params = append(params, filter.Offset)
	}

	counter := fmt.Sprintf("(SELECT COUNT(u.id) FROM users u %s)", _filter)
	query := fmt.Sprintf(`
	SELECT 
		u.id,
		u.username,
		u.password_hash,
		u.role,
		u.created_at,
		u.updated_at
		%s
	 FROM users u  
	 %s GROUP BY u.id %s`, counter, _filter, order)

	rows, err := r.db.Query(ctx, query+pages, params...)
	if err != nil {
		r.logger.Error(ctx, " err on r.db.Query", zap.Error(err))
		return list, err
	}
	defer rows.Close()

	var users []structs.User

	for rows.Next() {
		user := structs.User{}
		var createdAt time.Time
		var updatedAt time.Time

		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Password,
			&user.Role,
			&createdAt,
			&updatedAt,
			&list.Count,
		)

		user.CreatedAt = createdAt.Format(utils.Layout)
		user.UpdatedAt = updatedAt.Format(utils.Layout)

		if err != nil {
			r.logger.Error(ctx, " err on rows.Scan", zap.Error(err))
			return list, err
		}

		users = append(users, user)
	}

	list.Users = users

	return list, nil
}

func (r repo) Delete(ctx context.Context, id int) error {
	exec, err := r.db.Exec(ctx, `delete from users where id = $1`, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return structs.ErrNotFound
		}
		r.logger.Error(ctx, " err on r.db.Exec", zap.Error(err))
		return err
	}

	if exec.RowsAffected() == 0 {
		return structs.ErrNoRowsAffected
	}

	return nil
}

func (r repo) TokenUser(ctx context.Context, login, token string) error {
	var (
		query = `
			UPDATE users
				SET
					token = $2
				WHERE login = $1
		`
	)
	_, err := r.db.Exec(ctx, query, login, token)
	if err != nil {
		return err
	}
	return nil
}
