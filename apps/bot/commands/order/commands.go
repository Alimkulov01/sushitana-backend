package order

import (
	"sushitana/pkg/logger"

	"go.uber.org/fx"
)

var Module = fx.Provide(New)

type (
	Params struct {
		fx.In
		Logger logger.Logger
	}

	Commands struct {
		logger logger.Logger
	}
)

func New(p Params) Commands {
	return Commands{
		logger: p.Logger,
	}
}

