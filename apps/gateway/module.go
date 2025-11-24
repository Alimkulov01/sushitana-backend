package gateway

import (
	"sushitana/apps/gateway/handlers"

	"go.uber.org/fx"
)

var Module = fx.Options(
	handlers.Module,
)
