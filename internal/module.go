package internal

import (
	category "sushitana/internal/category"
	client "sushitana/internal/client"
	control "sushitana/internal/control"
	"sushitana/internal/product"

	"go.uber.org/fx"
)

var Module = fx.Options(
	client.Module,
	control.Module,
	category.Module,
	product.Module,
)
