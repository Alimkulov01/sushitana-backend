package clients

import (
	"context"
	"fmt"
	"strings"
	"sushitana/apps/bot/commands/category"
	"sushitana/internal/client"
	"sushitana/internal/keyboards"
	"sushitana/internal/structs"
	"sushitana/internal/texts"
	"sushitana/pkg/logger"
	"sushitana/pkg/tgrouter"
	"sushitana/pkg/utils"
	"sushitana/pkg/utils/ctxman"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Provide(New)

type Params struct {
	fx.In
	Logger      logger.Logger
	ClientSvc   client.Service
	CategoryCmd category.Commands
}

type Commands struct {
	logger      logger.Logger
	ClientSvc   client.Service
	CategoryCmd category.Commands
}

func New(p Params) Commands {
	return Commands{
		logger:      p.Logger,
		ClientSvc:   p.ClientSvc,
		CategoryCmd: p.CategoryCmd,
	}
}

func (c *Commands) Start(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID
	c.logger.Info(ctx.Context, "start command", zap.Int64("user_tgid", chatID))

	ctxWithTimeout, cancel := context.WithTimeout(ctx.Context, 3*time.Second)
	defer cancel()

	client, err := c.ClientSvc.Create(ctxWithTimeout, structs.CreateClient{
		TgID: chatID,
	})
	if err != nil {
		c.logger.Error(ctxWithTimeout, "failed to create client", zap.Error(err))
		ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(utils.RU, texts.Retry)))
		return
	}

	lang := client.Language
	if lang == "" {
		lang = utils.UZ
	}
	_ = ctx.UpdateState("show_main_menu", map[string]string{
		"last_action": "start_bot",
	})
	c.ShowMainMenu(ctx)
}

func (c *Commands) MainMenuHandler(ctx *tgrouter.Ctx) {
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
	switch text {
	case texts.Get(lang, texts.LanguageButton):
		_ = ctx.UpdateState("change_language", nil)
		c.ChangeLanguageInfo(ctx)
	case texts.Get(lang, texts.ContactButton):
		_ = ctx.UpdateState("contact", nil)
		c.Contact(ctx)
	case texts.Get(lang, texts.MenuButton):
		_ = ctx.UpdateState("show_category", map[string]string{"last_action": "show_main_menu"})
		c.CategoryCmd.MenuCategoryInfo(ctx)
	default:
		ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(lang, texts.SelectFromMenu)))

	}
}

func (c *Commands) ChangeLanguageInfo(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID
	account := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	lang := account.Language
	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.SelectFromMenu))
	msg.ReplyMarkup = keyboards.LanguageKeyboard(lang)
	_ = ctx.UpdateState("waiting_change_language", map[string]string{"last_action": "change_language"})
	ctx.Bot().Send(msg)
}

func (c *Commands) Contact(ctx *tgrouter.Ctx) {
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
		"last_action": "contact",
	})

	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false
	msg := tgbotapi.NewMessage(chatID, texts.Get(lang, texts.Contact))
	msg.ReplyMarkup = keyboard
	ctx.Bot().Send(msg)
}

func (c *Commands) ChangeLanguage(ctx *tgrouter.Ctx) {
	chatID := ctx.Update().FromChat().ID
	text := strings.TrimSpace(ctx.Update().Message.Text)
	account := ctx.Context.Value(ctxman.AccountKey{}).(*structs.Client)
	if account == nil {
		c.logger.Error(ctx.Context, "account not found")
		return
	}
	switch text {
	case "🇺🇿 Oʻzbekcha":
		account.Language = utils.UZ
	case "🇷🇺 Русский":
		account.Language = utils.RU
	default:
		ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(account.Language, texts.SelectFromMenu)))
		return
	}
	if err := c.ClientSvc.UpdateLanguage(ctx.Context, chatID, account.Language); err != nil {
		c.logger.Error(ctx.Context, "update language err")
		return
	}
	ctx.Context = context.WithValue(ctx.Context, ctxman.AccountKey{}, account)
	_ = ctx.UpdateState("show_main_menu", map[string]string{"last_action": "waiting_change_language"})
	ctx.Bot().Send(tgbotapi.NewMessage(chatID, texts.Get(account.Language, texts.SuccessChangeLanguage)))
	c.ShowMainMenu(ctx)
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
