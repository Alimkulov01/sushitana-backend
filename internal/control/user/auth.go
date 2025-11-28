package user

import (
	"context"
	"errors"
	userRepo "sushitana/pkg/repository/postgres/users_repo"

	"sushitana/pkg/cache"
	"sushitana/pkg/config"
	"sushitana/pkg/logger"

	"sushitana/internal/structs"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type Role = string

type (
	Auth interface {
		LoginAdmin(ctx context.Context, req structs.AdminLogin) (structs.AuthResponse, error)
		GetMe(ctx context.Context, token string) (structs.GetMeResponse, error)
		GetUserPermissions(ctx context.Context, token string) ([]string, error)
		CreateAdmin(ctx context.Context, username, password, role_id string) error
	}

	Service interface {
		Auth
	}
)

type (
	Params struct {
		fx.In

		Logger   logger.Logger
		Config   config.IConfig
		UserRepo userRepo.Repo
		Cache    cache.ICache
	}

	service struct {
		logger   logger.Logger
		config   config.IConfig
		userRepo userRepo.Repo
		cache    cache.ICache
	}
)

func New(p Params) Service {
	return &service{
		logger:   p.Logger,
		cache:    p.Cache,
		config:   p.Config,
		userRepo: p.UserRepo,
	}
}
func (s service) CreateAdmin(ctx context.Context, username, password, role_id string) error {
	return s.userRepo.CreateAdmin(ctx, username, password, role_id)
}

func (s service) LoginAdmin(ctx context.Context, req structs.AdminLogin) (structs.AuthResponse, error) {
	token, err := s.userRepo.LoginAdmin(ctx, req)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			return structs.AuthResponse{}, structs.ErrBadRequest
		}
		s.logger.Error(ctx, " err on s.userRepo.LoginAdmin", zap.Error(err))
		return structs.AuthResponse{}, err
	}

	return token, nil
}

func (s service) GetMe(ctx context.Context, token string) (structs.GetMeResponse, error) {
	resp, err := s.userRepo.GetMe(ctx, token)
	if err != nil {
		s.logger.Error(ctx, " err on s.userRepo.GetMe", zap.Error(err))
		return structs.GetMeResponse{}, err
	}

	return resp, nil
}

func (s service) GetUserPermissions(ctx context.Context, token string) ([]string, error) {
	resp, err := s.userRepo.GetUserPermissions(ctx, token)
	if err != nil {
		s.logger.Warn(ctx, " err on s.userRepo.GetUserPermissions", zap.Error(err))
		return nil, structs.ErrNotFound
	}
	return resp, nil
}
