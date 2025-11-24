package main

import (
	"sushitana/apps/bot"
	"sushitana/apps/gateway"
	"sushitana/cmd/gateway/router"
	"sushitana/internal"
	"sushitana/pkg"

	"go.uber.org/fx"
)

func main() {
	fx.New(
		gateway.Module,
		router.Module,
		pkg.Module,
		internal.Module,
		bot.Module,
	).Run()
}
