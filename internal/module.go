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
	"sushitana/internal/orderflow"
	"sushitana/internal/payment/click"
	"sushitana/internal/payment/payme"
	shopapi "sushitana/internal/payment/shop-api"
	"sushitana/internal/payment/usecase"
	"sushitana/internal/product"
	"sushitana/internal/role"
	"sushitana/internal/ws"

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
	orderflow.Module,
	click.Module,
	payme.Module,
	shopapi.Module,
	usecase.Module,
	ws.Module,
)
