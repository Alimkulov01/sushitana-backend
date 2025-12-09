package category

import (
	"strings"
	"sushitana/apps/bot/commands/product"
	"sushitana/internal/category"
	"sushitana/internal/structs"
	"sushitana/internal/texts"
	"sushitana/pkg/logger"
	"sushitana/pkg/tgrouter"
	"sushitana/pkg/utils"
	"sushitana/pkg/utils/ctxman"

	tgbotapi "github.com/ilpy20/telegram-bot-api/v7"
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
	chatID := ctx.Update().FromChat().ID
	text := strings.TrimSpace(ctx.Update().Message.Text)
	account := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language
	cats, err := c.CategorySvc.GetList(ctx.Context, structs.GetListCategoryRequest{
		Search: texts.Get(lang, text),
	})
	if err != nil {
		c.logger.Error(ctx.Context, "failed to get categories", zap.Error(err))
		ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Retry)))
		return
	}

	var keyboardRows [][]tgbotapi.KeyboardButton

	var row []tgbotapi.KeyboardButton

	for _, cat := range cats.Categories {
		name := getCategoryNameByLang(lang, cat.Name)
		btn := tgbotapi.NewKeyboardButton(name)
		row = append(row, btn)
		if len(row) == 2 {
			keyboardRows = append(keyboardRows, row)
			row = []tgbotapi.KeyboardButton{}
		}
	}
	if len(row) > 0 {
		keyboardRows = append(keyboardRows, row)
	}
	backText := texts.Get(lang, texts.BackButton)

	backRow := tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton(backText),
	)
	keyboardRows = append(keyboardRows, backRow)

	keyboard := tgbotapi.NewReplyKeyboard(keyboardRows...)

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.SelectFromMenu))
	msg.ReplyMarkup = keyboard
	ctx.Bot().Send(msg)
	_ = ctx.UpdateState("category_selected", map[string]string{"last_action": "show_category"})
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
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false

	_ = ctx.UpdateState("show_main_menu", map[string]string{
		"last_action": "show_main_menu",
	})

	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Welcome))
	msg.ReplyMarkup = keyboard
	_, _ = ctx.Bot().Send(msg)

	menuUrl := "https://sushitana.uz/uz/bot/home"

	btn := tgbotapi.NewInlineKeyboardButtonWebApp(
		texts.Get(lang, texts.MenuButtonWebAppUrl),
		tgbotapi.WebAppInfo{URL: menuUrl},
	)

	msgUrl := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.MenuButtonWebAppInfo))
	msgUrl.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(btn),
	)

	_, _ = ctx.Bot().Send(msgUrl)
}
