package internal

import (
	"sushitana/internal/cart"
	category "sushitana/internal/category"
	client "sushitana/internal/client"
	control "sushitana/internal/control"
	"sushitana/internal/employee"
	"sushitana/internal/file"
	"sushitana/internal/iiko"
	"sushitana/internal/menu"
	"sushitana/internal/order"
	"sushitana/internal/product"
	"sushitana/internal/role"

	"go.uber.org/fx"
)

var Module = fx.Options(
	client.Module,
	control.Module,
	category.Module,
	product.Module,
	file.Module,
	role.Module,
	employee.Module,
	cart.Module,
	iiko.Module,
	menu.Module,
	order.Module,
)
