package order

import (
	"context"
	"errors"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	orderrepo "sushitana/pkg/repository/postgres/order_repo"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Params struct {
		fx.In
		OrderRepo orderrepo.Repo
		Logger    logger.Logger
	}

	Service interface {
		Create(ctx context.Context, req structs.CreateOrder) error
		GetByTgId(ctx context.Context, tgId int64) ([]structs.Order, error)
		GetByID(ctx context.Context, id string) (structs.Order, error)
		GetList(ctx context.Context, req structs.GetListOrderRequest) (structs.GetListOrderResponse, error)
		Delete(ctx context.Context, order_id string) error
		UpdateStatus(ctx context.Context, req structs.UpdateStatus) error
	}
	service struct {
		orderRepo orderrepo.Repo
		logger    logger.Logger
	}
)

func New(p Params) Service {
	return &service{
		orderRepo: p.OrderRepo,
		logger:    p.Logger,
	}
}

func (s *service) Create(ctx context.Context, req structs.CreateOrder) error {
	err := s.orderRepo.Create(ctx, req)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			return err
		}
		s.logger.Error(ctx, "->orderRepo.Create", zap.Error(err))
		return err
	}
	return err
}

func (s *service) GetByTgId(ctx context.Context, tgId int64) ([]structs.Order, error) {
	orders, err := s.orderRepo.GetByTgId(ctx, tgId)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.GetByTgId", zap.Error(err))
		return nil, err
	}
	return orders, nil
}

func (s *service) GetByID(ctx context.Context, id string) (structs.Order, error) {
	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.GetByID", zap.Error(err))
		return structs.Order{}, err
	}
	return order, nil
}

func (s *service) GetList(ctx context.Context, req structs.GetListOrderRequest) (structs.GetListOrderResponse, error) {
	resp, err := s.orderRepo.GetList(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.GetList", zap.Error(err))
		return structs.GetListOrderResponse{}, err
	}
	return resp, nil
}

func (s *service) Delete(ctx context.Context, order_id string) error {
	err := s.orderRepo.Delete(ctx, order_id)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.Delete", zap.Error(err))
		return err
	}
	return nil
}

func (s *service) UpdateStatus(ctx context.Context, req structs.UpdateStatus) error{
	err := s.orderRepo.UpdateStatus(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->orderRepo.UpdateStatus", zap.Error(err))
		return err
	}
	return nil
}
