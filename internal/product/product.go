package product

import (
	"context"
	"errors"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	productRepo "sushitana/pkg/repository/postgres/product_repo"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Params struct {
		fx.In
		ProductRepo productRepo.Repo
		Logger      logger.Logger
	}

	Service interface {
		Create(ctx context.Context, req structs.CreateProduct) (structs.Product, error)
		GetList(ctx context.Context, req structs.GetListProductRequest) (resp structs.GetListProductResponse, err error)
		Delete(ctx context.Context, id int64) error
		GetByID(ctx context.Context, id int64) (structs.Product, error)
		GetByProductName(ctx context.Context, name string) (structs.Product, error)
		Patch(ctx context.Context, req structs.PatchProduct) (int64, error)
	}
	service struct {
		productRepo productRepo.Repo
		logger      logger.Logger
	}
)

func New(p Params) Service {
	return &service{
		productRepo: p.ProductRepo,
		logger:      p.Logger,
	}
}

func (s service) Create(ctx context.Context, req structs.CreateProduct) (structs.Product, error) {
	id, err := s.productRepo.Create(ctx, req)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			return id, err
		}
		s.logger.Error(ctx, "->productRepo.Create", zap.Error(err))
		return id, err
	}
	return id, err
}

func (s service) GetList(ctx context.Context, req structs.GetListProductRequest) (resp structs.GetListProductResponse, err error) {

	resp, err = s.productRepo.GetList(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->productRepo.GetList", zap.Error(err))
		return structs.GetListProductResponse{}, err
	}
	return resp, err
}

func (s service) Delete(ctx context.Context, id int64) error {
	err := s.productRepo.Delete(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "->productRepo.Delete", zap.Error(err))
		return err
	}
	return err
}

func (s service) GetByID(ctx context.Context, id int64) (resp structs.Product, err error) {
	resp, err = s.productRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			return structs.Product{}, err
		}
		s.logger.Error(ctx, " err on s.productRepo.GetByID", zap.Error(err))
		return structs.Product{}, err
	}
	return resp, err
}

func (s service) GetByProductName(ctx context.Context, name string) (resp structs.Product, err error) {
	resp, err = s.productRepo.GetByProductName(ctx, name)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			return structs.Product{}, err
		}
		s.logger.Error(ctx, " err on s.productRepo.GetByProductName", zap.Error(err))
		return structs.Product{}, err
	}
	return resp, err
}

func (s service) Patch(ctx context.Context, req structs.PatchProduct) (int64, error) {
	rowsAffected, err := s.productRepo.Patch(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->productRepo.Patch", zap.Error(err))
		return rowsAffected, err
	}
	return rowsAffected, err
}
