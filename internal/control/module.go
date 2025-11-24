package control

import (
	"sushitana/internal/control/user"

	"go.uber.org/fx"
)

var Module = fx.Options(
	user.Module,
)
