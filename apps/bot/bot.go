package bot

import (
	"context"
	"fmt"
	"sushitana/apps/bot/commands/category"
	"sushitana/apps/bot/commands/clients"
	"sushitana/apps/bot/commands/product"
	"sushitana/apps/bot/middleware"
	"sushitana/pkg/config"
	"sushitana/pkg/logger"
	"sushitana/pkg/tgrouter"
	"sushitana/pkg/tgrouter/interfaces"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/fx"
)

var Module = fx.Options(
	clients.Module,
	category.Module,
	product.Module,

	fx.Provide(middleware.New),

	fx.Invoke(NewBot),
)

type Params struct {
	fx.In
	fx.Lifecycle

	Logger     logger.Logger
	Config     config.IConfig
	Factory    tgrouter.RouterFactory
	State      interfaces.State
	Middleware middleware.Middleware

	ClientsCmd  clients.Commands
	CategoryCmd category.Commands
	ProductCmd  product.Commands
}

func NewBot(p Params) error {
	token := p.Config.GetString("bot_token_sushitana")
	if token == "" {
		return fmt.Errorf("telegram bot token client is not set")
	}
	tb, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return fmt.Errorf("failed to initialize bot: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	registerClientCommands(tb)

	r := p.Factory(tb, tgrouter.WithPoolSize(10), tgrouter.WithState(p.State))

	bot := r.Group()
	bot.Use(p.Middleware.AccountMw)

	tgrouter.On(bot, tgrouter.Cmd("start"), p.ClientsCmd.Start)

	tgrouter.On(bot, tgrouter.State("show_main_menu"), p.ClientsCmd.MainMenuHandler)
	tgrouter.On(bot, tgrouter.State("waiting_change_language"), p.ClientsCmd.ChangeLanguage)
	tgrouter.On(bot, tgrouter.State("category_selected"), p.CategoryCmd.MenuCategoryHandler)

	// //product
	tgrouter.On(bot, tgrouter.State("show_product"), p.ProductCmd.CategoryByMenu)
	tgrouter.On(bot, tgrouter.State("product_selected"), p.ProductCmd.MenuCategoryMenuHandler)

	go r.ListenUpdate(ctx)
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			p.Logger.Info(ctx, "bot started!")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			r.Shutdown(ctx, cancel)
			p.Logger.Info(ctx, "bot stopped!")
			return nil
		},
	})

	return nil
}

func registerClientCommands(tb *tgbotapi.BotAPI) {
	cfg := tgbotapi.NewSetMyCommands([]tgbotapi.BotCommand{
		{Command: "start", Description: "Перезапустить бота"},
	}...)

	_, _ = tb.Request(cfg)
}
