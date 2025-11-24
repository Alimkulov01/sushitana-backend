package user

import (
	"errors"
	"net/http"

	"sushitana/internal/control/user"
	"sushitana/internal/responses"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	"sushitana/pkg/reply"
	"sushitana/pkg/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Handler interface {
		LogIn(c *gin.Context)
		LogOut(c *gin.Context)
		Profile(c *gin.Context)

		GetAll(c *gin.Context)
		Create(c *gin.Context)
		GetUserById(c *gin.Context)
		Delete(c *gin.Context)
	}
	Params struct {
		fx.In
		Logger      logger.Logger
		UserService user.Service
	}

	handler struct {
		logger      logger.Logger
		userService user.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:      p.Logger,
		userService: p.UserService,
	}
}

func (h *handler) LogIn(c *gin.Context) {
	var (
		response structs.Response
		request  structs.AuthRequest
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	token, user, err := h.userService.LogIn(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrBadRequest) {
			response = responses.Unauthorized
			return
		}

		if errors.Is(err, structs.ErrUserBlocked) {
			response = responses.UserBlocked
		}

		h.logger.Error(ctx, " err on h.userService.LogIn", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = struct {
		Token string       `json:"token"`
		User  structs.User `json:"user"`
	}{
		Token: token,
		User:  user,
	}
}

func (h *handler) LogOut(c *gin.Context) {
	var (
		token    = c.GetHeader("Authorization")
		response structs.Response
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := h.userService.LogOut(c, token)
	if err != nil {
		h.logger.Error(ctx, " err on h.userService.LogOut", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}

func (h *handler) Profile(c *gin.Context) {
	var (
		ctx      = c.Request.Context()
		response structs.Response
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	adminUser, ok := c.Value("user").(structs.User)
	if !ok {
		h.logger.Warn(ctx, " can't get user from ctx")
		response = responses.Unauthorized
		return
	}

	response = responses.Success
	response.Payload = adminUser
}

func (h *handler) GetAll(c *gin.Context) {
	var (
		response structs.Response
		search   = c.Query("search")
		limit    = c.Query("limit")
		offset   = c.Query("offset")
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	users, err := h.userService.GetAll(c, structs.Filter{
		Search: search,
		Limit:  utils.StrToInt(limit),
		Offset: utils.StrToInt(offset),
	})
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.userService.Users", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = users
}

func (h *handler) Create(c *gin.Context) {
	var (
		response = responses.InternalErr
		request  structs.User
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	id, err := h.userService.Create(c, request)
	switch {
	case errors.Is(err, structs.ErrBadRequest):
		response = responses.BadRequest
		return
	case err != nil:
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = id
}

func (h *handler) GetUserById(c *gin.Context) {
	var (
		response structs.Response
		userID   = c.Param("id")
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	user, err := h.userService.GetByID(c, utils.StrToInt(userID))
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.userService.GetUserByID", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = user
}

func (h *handler) Delete(c *gin.Context) {
	var (
		response structs.Response
		userID   = c.Param("id")
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := h.userService.Delete(c, utils.StrToInt(userID))
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.userService.Delete", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
}
