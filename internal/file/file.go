package file

import (
	"context"
	"errors"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	fileRepo "sushitana/pkg/repository/postgres/file_repo"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Params struct {
		fx.In
		CategoryRepo fileRepo.Repo
		Logger       logger.Logger
	}

	Service interface {
		Create(ctx context.Context, req structs.CreateImage) (structs.Image, error)
		GetById(ctx context.Context, req structs.ImagePrimaryKey) (structs.Image, error)
		Delete(ctx context.Context, req structs.ImagePrimaryKey) error
		GetImage(ctx context.Context, req structs.GetImageRequest) (structs.GetImagerespones, error)
		GetAll(ctx context.Context) (structs.GetImagerespones, error)
	}
	service struct {
		fileRepo fileRepo.Repo
		logger   logger.Logger
	}
)

func New(p Params) Service {
	return &service{
		fileRepo: p.CategoryRepo,
		logger:   p.Logger,
	}
}

func (s service) Create(ctx context.Context, req structs.CreateImage) (structs.Image, error) {
	id, err := s.fileRepo.Create(ctx, req)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			return id, err
		}
		s.logger.Error(ctx, "->fileRepo.Create", zap.Error(err))
		return id, err
	}
	return id, err
}

func (s service) GetById(ctx context.Context, req structs.ImagePrimaryKey) (structs.Image, error) {

	resp, err := s.fileRepo.GetById(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->fileRepo.GetById", zap.Error(err))
		return structs.Image{}, err
	}
	return resp, err
}

func (s service) Delete(ctx context.Context, req structs.ImagePrimaryKey) error {
	err := s.fileRepo.Delete(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->fileRepo.Delete", zap.Error(err))
		return err
	}
	return err
}
func (s service) GetImage(ctx context.Context, req structs.GetImageRequest) (structs.GetImagerespones, error) {
	resp, err := s.fileRepo.GetImage(ctx, req)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			return structs.GetImagerespones{}, err
		}
		s.logger.Error(ctx, " err on s.fileRepo.GetImage", zap.Error(err))
		return structs.GetImagerespones{}, err
	}
	return resp, err
}

func (s service) GetAll(ctx context.Context) (structs.GetImagerespones, error) {
	rowsAffected, err := s.fileRepo.GetAll(ctx)
	if err != nil {
		s.logger.Error(ctx, "->fileRepo.GetAll", zap.Error(err))
		return rowsAffected, err
	}
	return rowsAffected, err
}
