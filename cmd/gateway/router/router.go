package router

import (
	"context"
	"sushitana/apps/gateway/handlers/category"
	"sushitana/apps/gateway/handlers/client"
	"sushitana/apps/gateway/handlers/control/user"
	"sushitana/apps/gateway/handlers/product"

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
}

func NewRouter(params Params) {
	r := gin.New()

	baseUrl := "/api/v1"
	out := r.Group(baseUrl)
	out.Use(params.Ctx(), gin.Logger(), gin.Recovery())

	{
		out.POST("/user/login", params.User.LogIn)
	}

	adminGroup := out.Group("/admin",
		params.CheckAuth(),
	)
	{
		adminGroup.DELETE("/user/logout", params.User.LogOut)
		adminGroup.GET("/user/profile", params.User.Profile)
		adminGroup.POST("/user", params.User.Create)
		adminGroup.DELETE("/user/:id", params.User.Delete)
		adminGroup.GET("/user/:id", params.User.GetUserById)
		adminGroup.GET("/user/list", params.User.GetAll)
	}
	categoryGroup := out.Group("/category",
		params.CheckAuth(),
	)
	{
		categoryGroup.POST("/", params.Category.CreateCategory)
		categoryGroup.GET("/:id", params.Category.GetByIDCategory)
		categoryGroup.GET("/", params.Category.GetListCategory)
		categoryGroup.PATCH("/:id", params.Category.PatchCategory)
		categoryGroup.DELETE("/:id", params.Category.DeleteCategory)
	}
	productGroup := out.Group("/product") // params.CheckAuth(),

	{
		productGroup.POST("/", params.Product.CreateProduct)
		productGroup.GET("/:id", params.Product.GetByIDProduct)
		productGroup.GET("/", params.Product.GetListProduct)
		productGroup.PATCH("/:id", params.Product.PatchProduct)
		productGroup.DELETE("/:id", params.Product.DeleteProduct)
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
