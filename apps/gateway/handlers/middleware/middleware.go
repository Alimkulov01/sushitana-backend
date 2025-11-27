package middleware

import (
	"context"
	"net/http"
	"strings"

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
		Perm(requiredPermission string) gin.HandlerFunc
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
		claims, err := utils.ParseJWT(authToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}
		userID, ok := claims["id"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token"})
			c.Abort()
			return
		}

		employeeID, _ := claims["employee_id"].(float64)

		c.Set("user_id", userID)
		c.Set("employee_id", int(employeeID))

		c.Next()
	}
}

func (m *mw) Perm(requiredPermission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			response structs.Response
			ctx      = c.Request.Context()
		)
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Missing token",
			})
			return
		}
		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)
		resp, err := m.userSvc.GetMe(context.Background(), tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Token related error: " + err.Error(),
			})
			return
		}
		hasPermission := false
		for _, scope := range resp.Role.AccessScopes {
			if scope.Name == requiredPermission {
				hasPermission = true
				break
			}
		}
		if !hasPermission {
			m.logger.Warn(ctx, " user does not have permission",
				zap.String("permission", requiredPermission))
			response = responses.Forbidden

			c.Abort()
			reply.Json(c.Writer, responses.ForbiddenCode, &response)
			return
		}
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

func EndpointPermissionMiddleware(m Middleware) gin.HandlerFunc {
	return func(c *gin.Context) {
		requiredPermission := getRequiredPermission(c.FullPath(), c.Request.Method)
		if requiredPermission == "" {
			c.Next()
			return
		}
		handler := m.Perm(requiredPermission)
		handler(c)
	}
}

func getRequiredPermission(endpoint string, method string) string {
	var resource string

	if strings.Contains(endpoint, "/product") {
		resource = "product"
	} else if strings.Contains(endpoint, "/category") {
		resource = "category"
	} else if strings.Contains(endpoint, "/file") {
		resource = "file"
	} else if strings.Contains(endpoint, "/role") {
		resource = "role"
	} else if strings.Contains(endpoint, "/employee") {
		resource = "employee"
	} else {
		return ""
	}

	var action string
	switch method {
	case "GET":
		action = "read"
	case "POST", "PUT", "PATCH", "DELETE":
		action = "write"
	default:
		return ""
	}

	return resource + "-" + action
}
