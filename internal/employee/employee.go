package employee

import (
	"context"
	"errors"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	employeeRepo "sushitana/pkg/repository/postgres/employee_repo"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Params struct {
		fx.In
		EmployeeRepo employeeRepo.Repo
		Logger       logger.Logger
	}

	Service interface {
		Create(ctx context.Context, req structs.CreateEmployee) (structs.Employee, error)
		GetById(ctx context.Context, id int64) (structs.Employee, error)
		GetAll(ctx context.Context, req structs.GetListEmployeeRequest) (structs.GetListEmployeeResponse, error)
		Delete(ctx context.Context, id int64) error
		Patch(ctx context.Context, req structs.PatchEmployee) (int64, error)
	}
	service struct {
		employeeRepo employeeRepo.Repo
		logger       logger.Logger
	}
)

func New(p Params) Service {
	return &service{
		employeeRepo: p.EmployeeRepo,
		logger:       p.Logger,
	}
}

func (s service) Create(ctx context.Context, req structs.CreateEmployee) (structs.Employee, error) {
	resp, err := s.employeeRepo.Create(ctx, req)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			return structs.Employee{}, err
		}
		s.logger.Error(ctx, "->employeeRepo.Create", zap.Error(err))
		return structs.Employee{}, err
	}
	return resp, err
}

func (s service) GetAll(ctx context.Context, req structs.GetListEmployeeRequest) (structs.GetListEmployeeResponse, error) {
	resp, err := s.employeeRepo.GetAll(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->employeeRepo.GetList", zap.Error(err))
		return structs.GetListEmployeeResponse{}, err
	}
	return resp, err
}

func (s service) Delete(ctx context.Context, id int64) error {
	err := s.employeeRepo.Delete(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "->employeeRepo.Delete", zap.Error(err))
		return err
	}
	return err
}
func (s service) GetById(ctx context.Context, id int64) (structs.Employee, error) {
	resp, err := s.employeeRepo.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			return structs.Employee{}, err
		}
		s.logger.Error(ctx, " err on s.employeeRepo.GetByID", zap.Error(err))
		return structs.Employee{}, err
	}
	return resp, err
}

func (s service) Patch(ctx context.Context, req structs.PatchEmployee) (int64, error) {
	rowsAffected, err := s.employeeRepo.Patch(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->employeeRepo.Patch", zap.Error(err))
		return rowsAffected, err
	}
	return rowsAffected, err
}
