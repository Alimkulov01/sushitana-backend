package handlers

import (
	"sushitana/apps/gateway/handlers/cart"
	"sushitana/apps/gateway/handlers/category"
	"sushitana/apps/gateway/handlers/client"
	"sushitana/apps/gateway/handlers/control"
	"sushitana/apps/gateway/handlers/employee"
	"sushitana/apps/gateway/handlers/file"
	"sushitana/apps/gateway/handlers/iiko"
	"sushitana/apps/gateway/handlers/middleware"
	"sushitana/apps/gateway/handlers/product"
	"sushitana/apps/gateway/handlers/role"

	"go.uber.org/fx"
)

var Module = fx.Options(
	middleware.Module,
	control.Module,
	client.Module,
	category.Module,
	product.Module,
	file.Module,
	role.Module,
	employee.Module,
	cart.Module,
	iiko.Module,
)
