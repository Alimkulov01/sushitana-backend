package bot

import (
	"context"
	"fmt"
	"strings"

	"sushitana/apps/bot/commands/category"
	"sushitana/apps/bot/commands/clients"
	"sushitana/apps/bot/commands/order"
	"sushitana/apps/bot/commands/product"
	"sushitana/apps/bot/middleware"
	"sushitana/internal/structs"
	"sushitana/internal/texts"
	"sushitana/pkg/config"
	"sushitana/pkg/logger"
	"sushitana/pkg/tgrouter"
	"sushitana/pkg/tgrouter/interfaces"
	"sushitana/pkg/utils/ctxman"

	tgbotapi "github.com/ilpy20/telegram-bot-api/v7"
	"go.uber.org/fx"
)

var Module = fx.Options(
	clients.Module,
	category.Module,
	product.Module,
	order.Module,

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
	OrderCmd    order.Commands
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

	// commands
	tgrouter.On(bot, tgrouter.Cmd("start"), p.ClientsCmd.Start)

	// states (clients)
	tgrouter.On(bot, tgrouter.State("show_main_menu"), p.ClientsCmd.MainMenuHandler)
	tgrouter.On(bot, tgrouter.State("waiting_change_language"), p.ClientsCmd.ChangeLanguage)
	tgrouter.On(bot, tgrouter.State("waiting_for_name"), p.ClientsCmd.SaveName)
	tgrouter.On(bot, tgrouter.State("waiting_for_phone"), p.ClientsCmd.ChangePhone)

	// cart state wrapper: ✅ Подтвердить! -> select_delivery_type, aks holda oddiy cart
	tgrouter.On(bot, tgrouter.State("get_cart"), func(ctx *tgrouter.Ctx) {
		if ctx.Update().Message != nil {
			account, ok := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
			if ok && account != nil {
				lang := account.Language
				txt := strings.TrimSpace(ctx.Update().Message.Text)

				if txt == texts.Get(lang, texts.CartConfirm) {
					_ = ctx.UpdateState("select_delivery_type", nil)
					p.OrderCmd.Confirm(ctx)
					return
				}
			}
		}

		p.ProductCmd.GetCartInfoHandler(ctx)
	})

	// states (product)
	tgrouter.On(bot, tgrouter.State("category_selected"), p.ProductCmd.CategoryByProductMenu)
	tgrouter.On(bot, tgrouter.State("product_selected"), p.ProductCmd.ProductInfoHandler)

	// select_delivery_type wrapper: ⬅️ Назад -> get_cart (1 qadam orqaga)
	tgrouter.On(bot, tgrouter.State("select_delivery_type"), func(ctx *tgrouter.Ctx) {
		if ctx.Update().Message != nil {
			account, ok := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
			if ok && account != nil {
				lang := account.Language
				txt := strings.TrimSpace(ctx.Update().Message.Text)

				if txt == texts.Get(lang, texts.BackButton) {
					_ = ctx.UpdateState("get_cart", nil)
					p.ProductCmd.GetCartInfoHandler(ctx)
					return
				}
			}
		}

		p.OrderCmd.DeliveryTypeHandler(ctx)
	})

	// delivery address (location/text)
	tgrouter.On(bot, tgrouter.State("wait_address"), p.OrderCmd.WaitAddressHandler)

	tgrouter.On(bot, tgrouter.State("wait_pickup_branch"), func(ctx *tgrouter.Ctx) {
		if ctx.Update().Message == nil {
			return
		}

		chatID := ctx.Update().FromChat().ID

		account, ok := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
		if !ok || account == nil {
			p.Logger.Error(ctx.Context, "account not found")
			return
		}

		lang := account.Language
		txt := strings.TrimSpace(ctx.Update().Message.Text)

		// 1 qadam orqaga: pickup_branch -> select_delivery_type
		if txt == texts.Get(lang, texts.BackButton) {
			_, data, _ := ctx.GetState()
			if data == nil {
				data = map[string]string{}
			}

			_ = ctx.UpdateState("select_delivery_type", data)
			p.OrderCmd.Confirm(ctx)
			return
		}

		// TODO: filial tanlash
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, "Филиал tanlash hali yozilmagan. ⬅️ Назад bosing."))
	})
	// payment step (hozircha minimal stub; keyin order package ichiga ko‘chirasiz)
	tgrouter.On(bot, tgrouter.State("wait_payment"), func(ctx *tgrouter.Ctx) {
		if ctx.Update().Message == nil {
			return
		}
		chatID := ctx.Update().FromChat().ID

		account, ok := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
		if !ok || account == nil {
			p.Logger.Error(ctx.Context, "account not found")
			return
		}
		lang := account.Language
		txt := strings.TrimSpace(ctx.Update().Message.Text)

		if txt == texts.Get(lang, texts.BackButton) {
			_ = ctx.UpdateState("wait_address", nil)
			p.OrderCmd.AskLocationOrAddress(ctx)
			return
		}

		// TODO: cash/card keyboard yoziladi
		_, _ = ctx.Bot().Send(tgbotapi.NewMessage(chatID, "To‘lov tanlash hali yozilmagan. ⬅️ Назад bosing."))
	})

	// callbacks (product)
	tgrouter.On(bot, tgrouter.Callback(""), func(ctx *tgrouter.Ctx) {
		if ctx.Update().CallbackQuery == nil {
			return
		}
		data := ctx.Update().CallbackQuery.Data

		switch {
		case strings.HasPrefix(data, "back_to_menu:"),
			strings.HasPrefix(data, "qty_inc:"),
			strings.HasPrefix(data, "qty_dec:"),
			strings.HasPrefix(data, "add_to_cart:"),
			strings.HasPrefix(data, "open_cart:"),
			strings.HasPrefix(data, "cart_inc:"),
			strings.HasPrefix(data, "cart_dec:"),
			strings.HasPrefix(data, "cart_del:"),
			strings.HasPrefix(data, "cart_clear:"),
			strings.HasPrefix(data, "cart_back:"),
			strings.HasPrefix(data, "noop:"),
			data == "noop":
			p.ProductCmd.Callback(ctx)
		}
	})

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
