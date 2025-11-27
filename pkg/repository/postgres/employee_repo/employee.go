package employeerepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sushitana/internal/structs"
	"sushitana/pkg/db"
	"sushitana/pkg/logger"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
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
		Create(ctx context.Context, req structs.CreateEmployee) (structs.Employee, error)
		GetById(ctx context.Context, id int64) (structs.Employee, error)
		GetAll(ctx context.Context, req structs.GetListEmployeeRequest) (structs.GetListEmployeeResponse, error)
		Delete(ctx context.Context, id int64) error
		Patch(ctx context.Context, req structs.PatchEmployee) (int64, error)
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

func (r repo) Create(ctx context.Context, req structs.CreateEmployee) (structs.Employee, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return structs.Employee{}, err
	}
	defer tx.Rollback(ctx)
	var (
		pgErr = &pgconn.PgError{}
		query = `
			INSERT INTO "employees"(
				name,
				surname,
				username,
				password,
				is_active,
				phone_number,
				salary_amount,
				kpi,
				branch_id,
				role_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id
		`
		id int64
	)

	var (
		queryAdmin = `
			INSERT INTO "admins"(
				id,
				username,
				password_hash,
				role_id
			) VALUES ($1, $2, $3, $4)
		`
		adminId = uuid.NewString()
	)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return structs.Employee{}, err
	}

	err = tx.QueryRow(ctx, query,
		req.Name,
		req.Surname,
		req.Username,
		string(hashedPassword),
		req.IsActive,
		req.PhoneNumber,
		req.RoleId,
	).Scan(&id)
	if err != nil {
		errors.As(err, &pgErr)
		if pgerrcode.UniqueViolation == pgErr.Code {
			return structs.Employee{}, err
		}
		return structs.Employee{}, err
	}

	_, err = tx.Exec(ctx, queryAdmin, adminId, req.Username, string(hashedPassword), req.RoleId)
	if err != nil {
		return structs.Employee{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return structs.Employee{}, err
	}

	return r.GetById(ctx, id)
}

func (r repo) GetById(ctx context.Context, id int64) (structs.Employee, error) {
	var (
		pgErr = &pgconn.PgError{}
		resp  structs.Employee
		query = `
			SELECT 
				id,
				name,
				surname,
				username,
				is_active,
				phone_number,
				salary_amount,
				kpi,
				branch_id,
				role_id,
				created_at,
				updated_at
			FROM employees
			WHERE id = $1
		`
	)

	err := r.db.QueryRow(ctx, query, id).Scan(
		&resp.Id,
		&resp.Name,
		&resp.Surname,
		&resp.Username,
		&resp.IsActive,
		&resp.PhoneNumber,
		&resp.RoleId,
		&resp.CreatedAt,
		&resp.UpdatedAt,
	)
	if err != nil {
		errors.As(err, &pgErr)
		if pgerrcode.UniqueViolation == pgErr.Code {
			return structs.Employee{}, err
		}
	}

	return resp, nil
}

func (r repo) GetAll(ctx context.Context, req structs.GetListEmployeeRequest) (structs.GetListEmployeeResponse, error) {

	var (
		query = `
			SELECT
				COUNT(*) OVER(), 
				e.id,
				e.name,
				e.surname,
				e.username,
				e.is_active,
				e.phone_number,
				e.salary_amount,
				e.kpi,
				e.branch_id,
				e.role_id,
				r.role_name,
				e.created_at,
				e.updated_at
			FROM employees as e
			JOIN roles r on e.role_id = r.id
		`

		resp   structs.GetListEmployeeResponse
		where  = " WHERE TRUE"
		offset = " OFFSET 0"
		limit  = " LIMIT 10"
		sort   = " ORDER BY created_at DESC"
	)

	if req.Offset > 0 {
		offset = fmt.Sprintf(" OFFSET %d", req.Offset)
	}
	if req.Limit > 0 {
		limit = fmt.Sprintf(" LIMIT %d", req.Limit)
	}
	if len(req.Search) > 0 {
		where += " AND e.name ILIKE" + " '%" + req.Search + "%'" + " OR e.surname ILIKE" + " '%" + req.Search + "%'" + " OR e.username ILIKE" + " '%" + req.Search + "%'"
	}

	query += where + sort + offset + limit

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return structs.GetListEmployeeResponse{}, err
	}

	for rows.Next() {
		var (
			employee structs.Employee
		)
		err = rows.Scan(
			&resp.Count,
			&employee.Id,
			&employee.Name,
			&employee.Surname,
			&employee.Username,
			&employee.IsActive,
			&employee.PhoneNumber,
			&employee.RoleId,
			&employee.RoleName,
			&employee.CreatedAt,
			&employee.UpdatedAt,
		)
		if err != nil {
			return structs.GetListEmployeeResponse{}, err
		}

		resp.Employees = append(resp.Employees, employee)
	}
	return resp, nil
}

func (r repo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM "employees" WHERE "id" = $1`, id)
	return err
}

func (r repo) Patch(ctx context.Context, req structs.PatchEmployee) (int64, error) {
	var (
		updateFields []string
		args         []interface{}
		argCounter   = 1
	)

	args = append(args, req.Id)

	addFiled := func(fieldName string, value interface{}, paramName string) {
		if value != nil {
			updateFields = append(updateFields, fmt.Sprintf("%s = $%d", fieldName, argCounter+1))
			args = append(args, value)
			argCounter++
		}
	}

	if req.Name != nil {
		addFiled("name", *req.Name, "name")
	}
	if req.Surname != nil {
		addFiled("surname", *req.Surname, "surname")
	}
	if req.Username != nil {
		addFiled("username", *req.Username, "username")
	}
	if req.Password != nil {
		addFiled("password", *req.Password, "password")
	}
	if req.IsActive != nil {
		addFiled("is_active", *req.IsActive, "is_active")
	}
	if req.PhoneNumber != nil {
		addFiled("phone_number", *req.PhoneNumber, "phone_number")
	}
	if req.RoleId != nil {
		addFiled("role_id", *req.RoleId, "role_id")
	}
	updateFields = append(updateFields, "updated_at = NOW()")
	query := fmt.Sprintf(`
		UPDATE "employees"
		SET %s
		WHERE id = $1
	`, strings.Join(updateFields, ", "))
	rowsAffected, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	return rowsAffected.RowsAffected(), nil
}

func (r repo) Update(ctx context.Context, req structs.UpdateEmployee) (int64, error) {
	var (
		setParts   []string
		args       []interface{}
		argIdx     = 1
		userFields = 0
	)
	args = append(args, req.Id)

	addField := func(col string, val interface{}) {
		setParts = append(setParts, fmt.Sprintf(`%s = $%d`, col, argIdx+1))
		args = append(args, val)
		argIdx++
		userFields++
	}

	if req.Name != nil {
		addField("name", *req.Name)
	}
	if req.Surname != nil {
		addField("surname", *req.Surname)
	}
	if req.Username != nil {
		addField("username", *req.Username)
	}
	if req.PhoneNumber != nil {
		addField("phone_number", *req.PhoneNumber)
	}
	if userFields == 0 {
		return 0, errors.New("no fields to update")
	}
	setParts = append(setParts, "updated_at = NOW()")
	query := fmt.Sprintf(`
		UPDATE employees
		SET %s
		WHERE id = $1
	`, strings.Join(setParts, ", "))

	res, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	ra := res.RowsAffected()
	if ra == 0 {
		return 0, sql.ErrNoRows
	}
	return ra, nil
}
