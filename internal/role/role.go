package role

import (
	"context"
	"errors"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	roleRepo "sushitana/pkg/repository/postgres/role_repo"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Params struct {
		fx.In
		RoleRepo roleRepo.Repo
		Logger   logger.Logger
	}

	Service interface {
		Create(ctx context.Context, req structs.CreateRole) error
		GetById(ctx context.Context, req structs.RolePrimaryKey) (structs.Role, error)
		GetAll(ctx context.Context, req structs.GetListRoleRequest) (structs.GetListRoleResponse, error)
		Delete(ctx context.Context, req structs.RolePrimaryKey) error
		Patch(ctx context.Context, req structs.PatchRole) (int64, error)
	}
	service struct {
		roleRepo roleRepo.Repo
		logger   logger.Logger
	}
)

func New(p Params) Service {
	return &service{
		roleRepo: p.RoleRepo,
		logger:   p.Logger,
	}
}

func (s service) Create(ctx context.Context, req structs.CreateRole) error {
	err := s.roleRepo.Create(ctx, req)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			return err
		}
		s.logger.Error(ctx, "->roleRepo.Create", zap.Error(err))
		return err
	}
	return err
}

func (s service) GetAll(ctx context.Context, req structs.GetListRoleRequest) (structs.GetListRoleResponse, error) {

	resp, err := s.roleRepo.GetAll(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->roleRepo.GetList", zap.Error(err))
		return structs.GetListRoleResponse{}, err
	}
	return resp, err
}

func (s service) Delete(ctx context.Context, req structs.RolePrimaryKey) error {
	err := s.roleRepo.Delete(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->roleRepo.Delete", zap.Error(err))
		return err
	}
	return err
}
func (s service) GetById(ctx context.Context, req structs.RolePrimaryKey) (structs.Role, error) {
	resp, err := s.roleRepo.GetById(ctx, req)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			return structs.Role{}, err
		}
		s.logger.Error(ctx, " err on s.roleRepo.GetByID", zap.Error(err))
		return structs.Role{}, err
	}
	return resp, err
}

func (s service) Patch(ctx context.Context, req structs.PatchRole) (int64, error) {
	rowsAffected, err := s.roleRepo.Patch(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->roleRepo.Patch", zap.Error(err))
		return rowsAffected, err
	}
	return rowsAffected, err
}
