package control

import (
	"sushitana/apps/gateway/handlers/control/user"

	"go.uber.org/fx"
)

var Module = fx.Options(
	user.Module,
)
