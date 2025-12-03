package menu

import (
	"context"
	"errors"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	menurepo "sushitana/pkg/repository/postgres/menu_repo"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Params struct {
		fx.In
		MenuRepo menurepo.Repo
		Logger   logger.Logger
	}

	Service interface {
		GetMenu(ctx context.Context) ([]structs.Menu, error)
	}
	service struct {
		menurepo menurepo.Repo
		logger   logger.Logger
	}
)

func New(p Params) Service {
	return &service{
		menurepo: p.MenuRepo,
		logger:   p.Logger,
	}
}

func (s service) GetMenu(ctx context.Context) ([]structs.Menu, error) {
	resp, err := s.menurepo.GetMenu(ctx)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			return nil, err
		}
		s.logger.Error(ctx, "->menurepo.GetMenu", zap.Error(err))
		return nil, err
	}
	return resp, err
}
