package cart

import (
	"context"
	"errors"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	cartRepo "sushitana/pkg/repository/postgres/cart_repo"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Params struct {
		fx.In
		CartRepo cartRepo.Repo
		Logger   logger.Logger
	}

	Service interface {
		Create(ctx context.Context, req structs.CreateCart) error
		Clear(ctx context.Context, tgID int64) error
		Delete(ctx context.Context, req structs.DeleteCart) error
		Patch(ctx context.Context, req structs.PatchCart) (int64, error)
		GetByUserTgID(ctx context.Context, tgID int64) (structs.GetCartByTgID, error)
		GetByTgID(ctx context.Context, tgID int64) (structs.CartInfo, error)
	}
	service struct {
		cartRepo cartRepo.Repo
		logger   logger.Logger
	}
)

func New(p Params) Service {
	return &service{
		cartRepo: p.CartRepo,
		logger:   p.Logger,
	}
}

func (s *service) Create(ctx context.Context, req structs.CreateCart) error {
	err := s.cartRepo.Create(ctx, req)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			return err
		}
		s.logger.Error(ctx, "->cartRepo.Create", zap.Error(err))
		return err
	}
	return err
}

func (s *service) Clear(ctx context.Context, tgID int64) error {
	err := s.cartRepo.Clear(ctx, tgID)
	if err != nil {
		s.logger.Error(ctx, "->cartRepo.Clear", zap.Error(err))
		return err
	}
	return err
}

func (s *service) Delete(ctx context.Context, req structs.DeleteCart) error {
	err := s.cartRepo.Delete(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->cartRepo.Delete", zap.Error(err))
		return err
	}
	return err
}

func (s *service) Patch(ctx context.Context, req structs.PatchCart) (int64, error) {
	updatedRows, err := s.cartRepo.Patch(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->cartRepo.Patch", zap.Error(err))
		return 0, err
	}
	return updatedRows, nil
}

func (s *service) GetByUserTgID(ctx context.Context, tgID int64) (structs.GetCartByTgID, error) {

	cart, err := s.cartRepo.GetByUserTgID(ctx, tgID)
	if err != nil {
		s.logger.Error(ctx, "->cartRepo.GetByTgID", zap.Error(err))
		return structs.GetCartByTgID{}, err
	}
	return cart, nil
}

func (s *service) GetByTgID(ctx context.Context, tgID int64) (structs.CartInfo, error) {
	{
		cart, err := s.cartRepo.GetByTgID(ctx, tgID)
		if err != nil {
			s.logger.Error(ctx, "->cartRepo.GetByTgID", zap.Error(err))
			return structs.CartInfo{}, err
		}
		return cart, nil
	}
}
