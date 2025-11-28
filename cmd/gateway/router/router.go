package router

import (
	"context"
	"sushitana/apps/gateway/handlers/category"
	"sushitana/apps/gateway/handlers/client"
	"sushitana/apps/gateway/handlers/control/user"
	"sushitana/apps/gateway/handlers/employee"
	"sushitana/apps/gateway/handlers/file"
	"sushitana/apps/gateway/handlers/product"
	"sushitana/apps/gateway/handlers/role"

	"net/http"
	"sushitana/apps/gateway/handlers/middleware"
	"sushitana/pkg/config"
	"sushitana/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/rs/cors"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Options(
	fx.Invoke(
		NewRouter,
	),
)

type Params struct {
	fx.In

	middleware.Middleware
	Lifecycle fx.Lifecycle
	Config    config.IConfig
	Logger    logger.Logger
	User      user.Handler
	Client    client.Handler
	Category  category.Handler
	Product   product.Handler
	File      file.Handler
	Role      role.Handler
	Employee  employee.Handler
}

func NewRouter(params Params) {
	r := gin.New()
	baseUrl := "/api/v1"
	out := r.Group(baseUrl)
	out.Use(params.Ctx(), gin.Logger(), gin.Recovery())
	permissionMiddleware := middleware.EndpointPermissionMiddleware(params.Middleware)

	adminGroup := out.Group("/admin")
	{
		adminGroup.POST("/login", params.User.LoginAdmin)
		adminGroup.GET("/self", params.User.GetMe)
		adminGroup.GET("/permissions", params.User.GetUserPermissions)
	}

	api := r.Group(baseUrl)
	api.Use(params.Ctx(), gin.Logger(), gin.Recovery())
	api.Use(permissionMiddleware)
	categoryGroup := api.Group("/category")
	{
		categoryGroup.POST("/", params.Category.CreateCategory)
		categoryGroup.GET("/:id", params.Category.GetByIDCategory)
		out.GET("/category", params.Category.GetListCategory)
		categoryGroup.PATCH("/:id", params.Category.PatchCategory)
		categoryGroup.DELETE("/:id", params.Category.DeleteCategory)
	}
	productGroup := api.Group("/product")
	{
		productGroup.POST("/", params.Product.CreateProduct)
		productGroup.GET("/:id", params.Product.GetByIDProduct)
		out.GET("/product", params.Product.GetListProduct)
		productGroup.PATCH("/:id", params.Product.PatchProduct)
		productGroup.DELETE("/:id", params.Product.DeleteProduct)
	}
	fileGroup := api.Group("/file")
	{
		fileGroup.POST("/", params.File.CreateFile)
		fileGroup.GET("/", params.File.GetListFile)
		out.GET("/file/:id", params.File.GetByIDFile)
		fileGroup.GET("/image", params.File.GetImage)
		fileGroup.DELETE("/:id", params.File.DeleteFile)
	}
	roleGroup := api.Group("/role")
	{
		roleGroup.POST("/", params.Role.CreateRole)
		roleGroup.GET("/", params.Role.GetListRole)
		roleGroup.GET("/:id", params.Role.GetByIDRole)
		roleGroup.DELETE("/:id", params.Role.DeleteRole)
		roleGroup.PATCH("/:id", params.Role.PatchRole)
	}
	employeeGroup := api.Group("/employee")
	{
		employeeGroup.POST("/", params.Employee.CreateEmployee)
		employeeGroup.GET("/", params.Employee.GetListEmployee)
		employeeGroup.GET("/:id", params.Employee.GetByIDEmployee)
		employeeGroup.DELETE("/:id", params.Employee.DeleteEmployee)
		employeeGroup.PATCH("/:id", params.Employee.PatchEmployee)
	}

	server := http.Server{
		Addr: params.Config.GetString("server.port"),
		Handler: cors.New(cors.Options{
			AllowedHeaders:   []string{"*"},
			AllowedOrigins:   []string{"http://localhost:5173"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
			AllowCredentials: true,
			AllowOriginVaryRequestFunc: func(r *http.Request, origin string) (bool, []string) {
				return true, []string{"*"}
			},
		}).Handler(r),
	}

	params.Lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				params.Logger.Info(ctx, "Starting application")
				go func() {
					if err := server.ListenAndServe(); err != nil {
						params.Logger.Error(ctx, "Err on ListenAndServe", zap.Error(err))
					}
				}()

				params.Logger.Info(ctx, "Application starting on port", zap.String("port", params.Config.GetString("server.port")))
				return nil
			},
			OnStop: func(ctx context.Context) error {
				params.Logger.Error(ctx, "Application stopped")
				return server.Shutdown(ctx)
			},
		},
	)
}
