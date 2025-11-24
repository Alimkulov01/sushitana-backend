package middleware

import (
	"net/http"

	"sushitana/internal/control/user"
	"sushitana/internal/responses"
	"sushitana/internal/structs"
	"sushitana/pkg/config"
	"sushitana/pkg/logger"
	"sushitana/pkg/reply"
	"sushitana/pkg/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	Module = fx.Provide(NewMiddleware)
)

type (
	Middleware interface {
		CheckAuth() gin.HandlerFunc
		Ctx() gin.HandlerFunc
	}

	Params struct {
		fx.In

		Logger  logger.Logger
		Config  config.IConfig
		UserSvc user.Service
	}

	mw struct {
		logger  logger.Logger
		config  config.IConfig
		userSvc user.Service
	}
)

func NewMiddleware(params Params) Middleware {
	return &mw{
		logger:  params.Logger,
		config:  params.Config,
		userSvc: params.UserSvc,
	}
}

func (m *mw) CheckAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			response structs.Response
			ctx      = c.Request.Context()
		)

		authToken := c.GetHeader("Authorization")
		if utils.StrEmpty(authToken) {
			m.logger.Warn(ctx, " empty auth token")
			response = responses.Unauthorized

			c.Abort()
			reply.Json(c.Writer, responses.UnauthorizedCode, &response)
			return
		}

		adminUser, err := m.userSvc.CheckAuthToken(ctx, authToken)
		if err != nil {
			m.logger.Warn(ctx, " err on m.userService.CheckAuthToken", zap.Error(err))
			response = responses.Unauthorized

			c.Abort()
			reply.Json(c.Writer, responses.UnauthorizedCode, &response)
			return
		}

		adminUser.Ip = getIp(c.Request)
		c.Set("user", adminUser)
		c.Set("userToken", authToken)
		c.Next()
	}
}

func (m *mw) Ctx() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := m.logger.Context(c.Request.Context())
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func getIp(request *http.Request) string {
	forwarded := request.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		return forwarded
	}

	return request.RemoteAddr
}
