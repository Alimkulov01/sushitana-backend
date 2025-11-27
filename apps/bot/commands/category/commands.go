package category

import (
	"fmt"
	"strings"
	"sushitana/apps/bot/commands/product"
	"sushitana/internal/category"
	"sushitana/internal/structs"
	"sushitana/internal/texts"
	"sushitana/pkg/logger"
	"sushitana/pkg/tgrouter"
	"sushitana/pkg/utils"
	"sushitana/pkg/utils/ctxman"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Provide(New)

type Params struct {
	fx.In
	Logger      logger.Logger
	CategorySvc category.Service
	ProductCmd  product.Commands
}

type Commands struct {
	logger      logger.Logger
	CategorySvc category.Service
	ProductCmd  product.Commands
}

func New(p Params) Commands {
	return Commands{
		logger:      p.Logger,
		CategorySvc: p.CategorySvc,
		ProductCmd:  p.ProductCmd,
	}
}

func (c *Commands) MenuCategoryHandler(ctx *tgrouter.Ctx) {
	if ctx.Update().Message == nil {
		return
	}
	text := strings.TrimSpace(ctx.Update().Message.Text)
	chatID := ctx.Update().Message.Chat.ID

	account := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language
	cats, err := c.CategorySvc.GetList(ctx.Context, structs.GetListCategoryRequest{})
	if err != nil {
		c.logger.Error(ctx.Context, "failed to get categories", zap.Error(err))
		ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}
	for _, cat := range cats.Categories {
		name := getCategoryNameByLang(lang, cat.Name)
		if text == name {
			_ = ctx.UpdateState("show_product", nil)
			c.ProductCmd.CategoryByMenu(ctx)
			return
		}
	}
	switch text {
	case texts.Get(lang, texts.BackButton):
		_ = ctx.UpdateState("show_main_menu", map[string]string{"last_action": "show_category"})
		return
	default:
		ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.SelectFromMenu)))
	}
}

func getCategoryNameByLang(lang utils.Lang, name structs.Name) string {
	switch lang {
	case utils.UZ:
		return name.Uz
	case utils.RU:
		return name.Ru
	default:
		return name.En
	}
}

func (c *Commands) ShowMainMenu(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID
	account := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.MenuButton)),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.FeedbackButton)),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.InfoButton)),
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.ContactButton)),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(texts.Get(lang, texts.LanguageButton)),
		),
	)
	_ = ctx.UpdateState("show_main_menu", map[string]string{
		"last_action": "changed_language",
	})

	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Welcome))
	msg.ReplyMarkup = keyboard
	ctx.Bot().Send(msg)

	menuUrl := fmt.Sprintf("https://your-api.example/pay/mock/checkout?token=%d", chatID)
	msgUrl := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.MenuButtonWebAppInfo))
	msgUrl.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL(texts.Get(lang, texts.MenuButtonWebAppUrl), menuUrl),
		),
	)
	ctx.Bot().Send(msgUrl)
}
