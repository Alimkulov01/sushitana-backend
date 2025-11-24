package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sushitana/pkg/cache"
	"sushitana/pkg/config"
	"sushitana/pkg/logger"
	userRepo "sushitana/pkg/repository/postgres/users_repo"
	"sushitana/pkg/utils"

	"sushitana/internal/structs"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type Role = string

const (
	Admin    Role = "admin"
	Merchant Role = "merchant"
	Manager  Role = "manager"
)

type (
	Auth interface {
		LogIn(ctx context.Context, request structs.AuthRequest) (token string, user structs.User, err error)
		LogOut(ctx context.Context, token string) error
		CheckAuthToken(ctx context.Context, token string) (user structs.User, err error)
	}

	Users interface {
		GetAll(ctx context.Context, request structs.Filter) (structs.UserList, error)
		Create(ctx context.Context, request structs.User) (id int, err error)
		GetByID(ctx context.Context, id int) (user structs.User, err error)
		Delete(ctx context.Context, id int) error
	}

	Service interface {
		Auth
		Users
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

func (s service) LogIn(ctx context.Context, request structs.AuthRequest) (
	token string, user structs.User, err error,
) {
	user, err = s.userRepo.GetUserWithPolicyByUsername(ctx, request.Username)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			return "", user, structs.ErrBadRequest
		}
		s.logger.Error(ctx, " err on s.userRepo.GetUserWithPolicyByUsername", zap.Error(err))
		return "", user, err
	}

	if !utils.CompareInBcrypt(user.Password, request.Password) {
		return "", user, structs.ErrBadRequest
	}

	token, err = utils.GenerateHash(
		utils.HashSHA2,
		utils.GenKSUID(),
		request.Username,
		request.Password,
		fmt.Sprintf("%d", time.Now().UnixNano()))
	if err != nil {
		s.logger.Error(ctx, " err on utils.GenerateHash", zap.Error(err))
		return "", user, err
	}

	user.Password = "this is password :)"
	err = s.userRepo.TokenUser(ctx, user.Username, token)
	if err != nil {
		s.logger.Error(ctx, " err on s.userRepo.Token", zap.Error(err))
	}

	return token, user, nil
}

func (s service) LogOut(ctx context.Context, token string) error {
	err := s.cache.Delete(":users:" + token)
	if err != nil {
		s.logger.Error(ctx, " err on s.cache.Delete", zap.Error(err))
		return err
	}

	return nil
}

func (s service) CheckAuthToken(ctx context.Context, token string) (user structs.User, err error) {
	err = s.cache.GetObj(":users:"+token, &user)
	if err != nil {
		s.logger.Warn(ctx, " err on s.cache.FindObj", zap.Error(err))
		return user, structs.ErrNotFound
	}

	return user, nil
}

const (
	register string = "register"
	reset           = "reset"
)

func (s service) GetAll(ctx context.Context, request structs.Filter) (structs.UserList, error) {
	users, err := s.userRepo.GetUsers(ctx, request)
	if err != nil {
		s.logger.Error(ctx, " err on s.userRepo.GetUsers", zap.Error(err))
		return structs.UserList{}, err
	}

	return users, nil
}

func (s service) Create(ctx context.Context, request structs.User) (id int, err error) {
	request.Password, err = utils.GenerateHash(utils.HashBCRYPT, utils.StripSpace(request.Password))
	if err != nil {
		s.logger.Error(ctx, " err on utils.GenerateHash", zap.Error(err))
		return 0, err
	}

	id, err = s.userRepo.Create(ctx, request)
	if err != nil {
		if errors.Is(err, structs.ErrUniqueViolation) {
			return 0, structs.ErrUniqueViolation
		}
		s.logger.Error(ctx, " err on s.userRepo.Create", zap.Error(err))
		return 0, err
	}

	return id, nil
}

func (s service) GetByID(ctx context.Context, id int) (user structs.User, err error) {
	user, err = s.userRepo.GetUserByID(ctx, id)
	if err != nil {
		s.logger.Error(ctx, " err on s.userRepo.GetUserByID", zap.Error(err))
		return user, err
	}

	return user, nil
}

func (s service) Delete(ctx context.Context, id int) error {
	err := s.userRepo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			return err
		}
		s.logger.Error(ctx, " err on s.userRepo.Delete", zap.Error(err))
		return err
	}

	return nil
}
