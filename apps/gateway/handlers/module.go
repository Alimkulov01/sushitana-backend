package handlers

import (
	"sushitana/apps/gateway/handlers/category"
	"sushitana/apps/gateway/handlers/client"
	"sushitana/apps/gateway/handlers/control"
	"sushitana/apps/gateway/handlers/file"
	"sushitana/apps/gateway/handlers/middleware"
	"sushitana/apps/gateway/handlers/product"

	"go.uber.org/fx"
)

var Module = fx.Options(
	middleware.Module,
	control.Module,
	client.Module,
	category.Module,
	product.Module,
	file.Module,
)
