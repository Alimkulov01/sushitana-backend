package click

import (
	"sushitana/internal/payment/click"
	"sushitana/pkg/logger"

	"go.uber.org/fx"
)

var Module = fx.Provide(New)

type (
	Handler interface {
	}

	Params struct {
		fx.In
		Logger       logger.Logger
		ClickService click.Service
	}

	handler struct {
		logger       logger.Logger
		clickService click.Service
	}
)

func New(p Params) Handler {
	return &handler{
		logger:       p.Logger,
		clickService: p.ClickService,
	}
}
