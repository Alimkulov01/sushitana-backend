package repository

import (
	"go.uber.org/fx"

	"sushitana/pkg/repository/postgres"
	"sushitana/pkg/repository/state"
)

var Module = fx.Options(
	postgres.Module,
	state.Module,
)
