package client

import (
	"context"
	"errors"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	clientRepo "sushitana/pkg/repository/postgres/client_repo"
	"sushitana/pkg/utils"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Params struct {
		fx.In
		ClientRepo clientRepo.Repo
		Logger     logger.Logger
	}

	Service interface {
		Create(ctx context.Context, req structs.CreateClient) (structs.Client, error)
		GetList(ctx context.Context, req structs.GetListClientRequest) (resp structs.GetListClientResponse, err error)
		Delete(ctx context.Context, id int64) error
		GetByTgID(ctx context.Context, tgid int64) (structs.Client, error)
		GetByID(ctx context.Context, id int64) (structs.Client, error)
		UpdateLanguage(ctx context.Context, tgID int64, lang utils.Lang) error
		GetLanguageByTgID(ctx context.Context, tgID int64) (string, error)
	}
	service struct {
		clientRepo clientRepo.Repo
		logger     logger.Logger
	}
)

func New(p Params) Service {
	return &service{
		clientRepo: p.ClientRepo,
		logger:     p.Logger,
	}
}

func (s service) Create(ctx context.Context, req structs.CreateClient) (structs.Client, error) {
	id, err := s.clientRepo.Create(ctx, req)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			return id, err
		}
		s.logger.Error(ctx, "->clientRepo.Create", zap.Error(err))
		return id, err
	}
	return id, err
}

func (s service) GetList(ctx context.Context, req structs.GetListClientRequest) (resp structs.GetListClientResponse, err error) {

	resp, err = s.clientRepo.GetList(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->clientRepo.GetList", zap.Error(err))
		return structs.GetListClientResponse{}, err
	}
	return resp, err
}

func (s service) Delete(ctx context.Context, id int64) error {
	err := s.clientRepo.Delete(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "->clientRepo.Delete", zap.Error(err))
		return err
	}
	return err
}
func (s service) GetByID(ctx context.Context, id int64) (resp structs.Client, err error) {
	resp, err = s.clientRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			return structs.Client{}, err
		}
		s.logger.Error(ctx, " err on s.clientRepo.GetByID", zap.Error(err))
		return structs.Client{}, err
	}
	return resp, err
}

func (s service) GetByTgID(ctx context.Context, tgid int64) (resp structs.Client, err error) {
	resp, err = s.clientRepo.GetByTgID(ctx, tgid)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			return structs.Client{}, err
		}
		s.logger.Error(ctx, " err on s.clientRepo.GetByTgID", zap.Error(err))
		return structs.Client{}, err
	}
	return resp, err
}
func (s service) GetLanguageByTgID(ctx context.Context, tgID int64) (string, error) {
	resp, err := s.clientRepo.GetLanguageByTgID(ctx, tgID)

	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			return "", err
		}
		s.logger.Error(ctx, " err on s.clientRepo.GetLanguageByTgID", zap.Error(err))
		return "", err
	}
	return resp, err
}

func (s service) UpdateLanguage(ctx context.Context, tgID int64, lang utils.Lang) error {
	err := s.clientRepo.UpdateLanguage(ctx, tgID, lang)

	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			return err
		}
		s.logger.Error(ctx, " err on s.clientRepo.UpdateLanguage", zap.Error(err))
		return err
	}
	return err
}
