package category

import (
	"context"
	"errors"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	categoryRepo "sushitana/pkg/repository/postgres/category_repo"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Params struct {
		fx.In
		CategoryRepo categoryRepo.Repo
		Logger       logger.Logger
	}

	Service interface {
		Create(ctx context.Context, token string, req []structs.IikoGroup) error
		GetList(ctx context.Context, req structs.GetListCategoryRequest) (resp structs.GetListCategoryResponse, err error)
		Delete(ctx context.Context, id string) error
		GetByID(ctx context.Context, id string) (structs.Category, error)
		Patch(ctx context.Context, req structs.PatchCategory) (int64, error)
	}
	service struct {
		categoryRepo categoryRepo.Repo
		logger       logger.Logger
	}
)

func New(p Params) Service {
	return &service{
		categoryRepo: p.CategoryRepo,
		logger:       p.Logger,
	}
}

func (s service) Create(ctx context.Context, token string, req []structs.IikoGroup) error {
	categories := []structs.CreateCategory{}
	for _, group := range req {
		category := structs.CreateCategory{}
		category.ID = group.Id
		category.Name.Ru = group.Name
		category.ParentID = group.ParentGroup
		category.IsIncludedInMenu = group.IsIncludedInMenu
		category.IsGroupModifier = group.IsGroupModifier
		category.IsDeleted = group.IsDeleted
		categories = append(categories, category)
	}
	err := s.categoryRepo.Create(ctx, categories)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			return err
		}
		s.logger.Error(ctx, "->categoryRepo.Create", zap.Error(err))
		return err
	}
	return err
}

func (s service) GetList(ctx context.Context, req structs.GetListCategoryRequest) (resp structs.GetListCategoryResponse, err error) {

	resp, err = s.categoryRepo.GetList(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->categoryRepo.GetList", zap.Error(err))
		return structs.GetListCategoryResponse{}, err
	}
	return resp, err
}

func (s service) Delete(ctx context.Context, id string) error {
	err := s.categoryRepo.Delete(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "->categoryRepo.Delete", zap.Error(err))
		return err
	}
	return err
}
func (s service) GetByID(ctx context.Context, id string) (resp structs.Category, err error) {
	resp, err = s.categoryRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			return structs.Category{}, err
		}
		s.logger.Error(ctx, " err on s.categoryRepo.GetByID", zap.Error(err))
		return structs.Category{}, err
	}
	return resp, err
}

func (s service) Patch(ctx context.Context, req structs.PatchCategory) (int64, error) {
	rowsAffected, err := s.categoryRepo.Patch(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "->categoryRepo.Patch", zap.Error(err))
		return rowsAffected, err
	}
	return rowsAffected, err
}
