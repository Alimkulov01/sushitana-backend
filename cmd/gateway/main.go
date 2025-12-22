package main

import (
	"sushitana/apps/bot"
	"sushitana/apps/gateway"
	"sushitana/cmd/gateway/router"
	"sushitana/internal"
	"sushitana/pkg"
	"sushitana/pkg/utils"

	"go.uber.org/fx"
)

func main() {
	fx.New(
		fx.Provide(func() (*utils.ZoneChecker, error) {
			return utils.NewZoneCheckerFromFiles(
				"./olmaliq.json",
				"./ohongoron.json",
			)
		}),
		gateway.Module,
		router.Module,
		pkg.Module,
		internal.Module,
		bot.Module,
	).Run()
}
