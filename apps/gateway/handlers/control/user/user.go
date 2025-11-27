package user

import (
	"errors"
	"net/http"

	"sushitana/internal/control/user"
	"sushitana/internal/responses"
	"sushitana/internal/structs"
	"sushitana/pkg/logger"
	"sushitana/pkg/reply"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(New)
)

type (
	Handler interface {
		LoginAdmin(c *gin.Context)
		GetMe(c *gin.Context)
		GetUserPermissions(c *gin.Context)
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

func (h *handler) LoginAdmin(c *gin.Context) {
	var (
		response structs.Response
		request  structs.AdminLogin
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	err := c.ShouldBindJSON(&request)
	if err != nil {
		h.logger.Warn(ctx, " error parse request", zap.Error(err))
		response = responses.BadRequest
		return
	}

	token, err := h.userService.LoginAdmin(c, request)
	if err != nil {
		if errors.Is(err, structs.ErrBadRequest) {
			response = responses.Unauthorized
			return
		}

		if errors.Is(err, structs.ErrUserBlocked) {
			response = responses.UserBlocked
		}

		h.logger.Error(ctx, " err on h.userService.LoginAdmin", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = token
}

func (h *handler) GetMe(c *gin.Context) {
	var (
		token    = c.GetHeader("Authorization")
		response structs.Response
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)

	resp, err := h.userService.GetMe(c, token)
	if err != nil {
		h.logger.Error(ctx, " err on h.userService.GetMe", zap.Error(err))
		response = responses.InternalErr
		return
	}

	response = responses.Success
	response.Payload = resp
}

func (h *handler) GetUserPermissions(c *gin.Context) {
	var (
		response structs.Response
		token    = c.GetHeader("Authorization")
		ctx      = c.Request.Context()
	)

	defer reply.Json(c.Writer, http.StatusOK, &response)
	permissions, err := h.userService.GetUserPermissions(c, token)
	if err != nil {
		if errors.Is(err, structs.ErrNotFound) {
			response = responses.NotFound
			return
		}
		h.logger.Error(ctx, " err on h.userService.GetUserPermissions", zap.Error(err))
		response = responses.InternalErr
		return
	}
	response = responses.Success
	response.Payload = permissions
}
